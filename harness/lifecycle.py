"""The doctrine lifecycle engine — one code path for conformance AND live sorties.

Implements, from the pinned doctrine:
- decomposition-contract posting (via Board.post_task, file-scope enforced)
- the rejection ladder (CONTRACTS/REJECTION.md): attempt N fail charges the
  worker while N <= max_worker_retries (same worker retries); the next fail
  charges the Producer — hard stop, reissue. Attribution is COMPUTED here
  from config/fleet.toml, never scripted.
- CLARIFY escalation (unspeccable goes up, never down)
- mechanical merge (CONTRACTS/MERGE.md) behind a pluggable MergeGate,
  including gate 2: the mechanical file-scope gate (merge-base diff vs the
  task's declared files[]; ScopeViolation -> task.scope_violation, a
  worker-charged rejection-ladder rung)
- seat.error on seat INFRASTRUCTURE deaths (charged nobody; the failing
  seat is attributed on the exception by the drive helpers)
- budget caps (CONTRACTS/DECOMPOSITION.md #4, harness/budget.py): token cap
  checked at session boundaries, wall cap hard-enforced via subprocess
  timeout; either trip emits budget.cap_hit {kind, charged: "producer"} and
  reissues. In doctrine the Worker "stops work and files a SITREP" — in this
  harness the machinery IS the stop, and the ledger event is the record.

Seat drivers supply behavior; this engine supplies law.
"""

from .budget import (BudgetMeter, SessionCapExceeded, TokenCapExceeded,
                     WallCapExceeded)
from .claude_cli import SeatSessionError, SeatWallTimeout


class TaskSpec:
    def __init__(self, task_id: str, title: str, description: str, files: list,
                 deps: list, phase: str, acceptance_criteria: str = "",
                 remediation_of=None, budget_tokens: int | None = None,
                 budget_wall_s: int | None = None):
        self.task_id = task_id
        self.title = title
        self.description = description
        self.files = files
        self.deps = deps
        self.phase = phase
        self.acceptance_criteria = acceptance_criteria
        self.remediation_of = remediation_of
        self.budget_tokens = budget_tokens
        self.budget_wall_s = budget_wall_s


class Engine:
    def __init__(self, board, emitter, fleet_cfg: dict, merge_gate):
        self.board = board
        self.emitter = emitter
        self.cfg = fleet_cfg
        self.merge_gate = merge_gate
        # Q8 synthetic failure injection (harness/synthetic.py). Called after
        # task.completed, before the verifier — ONLY on synthetic tasks; the
        # engine refuses to let injection machinery near real work.
        self.synthetic_injector = None
        # Budget enforcement is ALWAYS ON — no off switch (a silently disabled
        # cap is the audit failure class Sortie 2's ingest bugs taught).
        # The clock is injectable only so wall-cap tests are deterministic.
        import time
        self.budget_clock = time.monotonic
        # Set when a task was reissued due to a seat infrastructure failure
        # (see run_task) — sortie scripts surface it loudly.
        self.last_infra_error = None

    def open_sortie(self, phase: str):
        self.emitter.emit("runner", None, "sortie.open", {"phase": phase})

    def close_sortie(self):
        self.emitter.emit("runner", None, "sortie.close", {})

    def post(self, spec: TaskSpec, synthetic: int = 0):
        self.board.post_task(
            spec.task_id, spec.title, spec.description, spec.files, spec.deps,
            spec.budget_tokens or self.cfg["default_task_budget_tokens"],
            spec.budget_wall_s or self.cfg["default_task_budget_wall_s"],
            spec.phase,
            acceptance_criteria=spec.acceptance_criteria,
            remediation_of=spec.remediation_of,
            synthetic=synthetic,
        )

    def _emit_model_call(self, seat, task_id, payload, synthetic, meter):
        """Every model.call goes through the meter: ledger the event (it
        happened; the cap doesn't unhappen it), then charge and check."""
        self.emitter.emit(seat, task_id, "model.call", payload, synthetic)
        if meter is not None:
            meter.feed(payload)
            meter.check()

    @staticmethod
    def _tag_seat(exc, seat):
        """Attach the failing seat to an infra exception (for seat.error)
        without clobbering a tag set closer to the failure."""
        if not hasattr(exc, "seat"):
            exc.seat = seat
        raise exc

    def _drain_worker(self, worker, worker_seat, task_id, synthetic, meter=None):
        """Run worker actions until a terminal action; emit telemetry as we go.
        Returns the terminal action tuple."""
        try:
            for action in worker.next_actions():
                kind = action[0]
                if kind == "model":
                    self._emit_model_call(worker_seat, task_id, action[1], synthetic, meter)
                elif kind == "tool":
                    self.emitter.emit(worker_seat, task_id, "tool.call", action[1], synthetic)
                elif kind in ("clarify", "done"):
                    return action
                else:
                    raise RuntimeError(f"worker yielded unexpected action {kind!r}")
        except SeatSessionError as e:
            self._tag_seat(e, worker_seat)
        raise RuntimeError("worker yielded no terminal action")

    def _judge(self, verifier, verifier_seat, task_id, synthetic, meter=None):
        try:
            for action in verifier.next_actions():
                kind = action[0]
                if kind == "model":
                    self._emit_model_call(verifier_seat, task_id, action[1], synthetic, meter)
                elif kind in ("pass", "fail"):
                    return action
                else:
                    raise RuntimeError(f"verifier yielded unexpected action {kind!r}")
        except SeatSessionError as e:
            self._tag_seat(e, verifier_seat)
        raise RuntimeError("verifier yielded no verdict")

    def _answer_clarify(self, producer, task_id, synthetic, meter=None):
        """Returns ("answer", text) or ("reissue", reason). A reissue ruling
        is the lawful terminal for a CLARIFY that exposes a task with no
        lawful deliverable (contradictory acceptance requirements, required
        work outside the declared file-scope): the Producer withdraws and
        re-decomposes rather than inventing an answer downward."""
        try:
            for action in producer.next_actions():
                kind = action[0]
                if kind == "model":
                    self._emit_model_call("producer", task_id, action[1], synthetic, meter)
                elif kind in ("answer", "reissue"):
                    return action
                else:
                    raise RuntimeError(f"producer yielded unexpected action {kind!r}")
        except SeatSessionError as e:
            self._tag_seat(e, "producer")
        raise RuntimeError("producer yielded no answer")

    def run_task(self, spec: TaskSpec, worker, verifier, producer,
                 worker_seat: str = "worker-1", verifier_seat: str = "verifier-1",
                 synthetic: int = 0) -> str:
        """Drive one posted task to a terminal state.
        Returns 'merged' or 'reissued'."""
        task_id = spec.task_id
        max_retries = self.cfg["max_worker_retries"]

        # Budget caps (DECOMPOSITION #4): stamped budgets or config defaults —
        # the Producer may tighten, never remove, so the cap always exists.
        meter = BudgetMeter(
            spec.budget_tokens or self.cfg["default_task_budget_tokens"],
            spec.budget_wall_s or self.cfg["default_task_budget_wall_s"],
            clock=self.budget_clock,
            # Fleet-level (PR #4 redline): every session stays under the
            # dumb-zone threshold. None until the key is law in fleet.toml.
            session_budget_tokens=self.cfg.get("default_session_budget_tokens"))
        for seat in (worker, verifier, producer):
            if hasattr(seat, "set_budget_meter"):
                seat.set_budget_meter(meter)

        self.board.claim(task_id, worker_seat, synthetic)
        try:
            return self._run_task_inner(
                spec, worker, verifier, producer, worker_seat, verifier_seat,
                synthetic, task_id, max_retries, meter)
        except (WallCapExceeded, SeatWallTimeout):
            # Ordered BEFORE SeatSessionError: SeatWallTimeout subclasses it,
            # but a wall kill is a budget event, not an infra failure.
            self.emitter.emit("board", task_id, "budget.cap_hit",
                              {"kind": "wall", "charged": "producer"}, synthetic)
            self.board.reissued(task_id, synthetic)
            return "reissued"
        except SeatSessionError as e:
            # Seat INFRASTRUCTURE failure (provider api_error, unparsable
            # envelope, nonzero CLI exit) — not a verdict about the work.
            # Sortie 3 flight 2 lesson: this must not be sortie-fatal. The
            # task is reissued (rerunnable), the transcript holds the
            # diagnosis, and the sortie flies on. Doctrine PR #5: emit
            # seat.error so infra deaths are labeled and charged NOBODY —
            # kept out of the blame views (m03) by construction.
            self.last_infra_error = str(e)
            self.emitter.emit("runner", task_id, "seat.error",
                              {"seat": getattr(e, "seat", "unknown"),
                               "reason": str(e)}, synthetic)
            self.board.reissued(task_id, synthetic)
            return "reissued"
        except SessionCapExceeded:
            self.emitter.emit("board", task_id, "budget.cap_hit",
                              {"kind": "session_tokens", "charged": "producer"},
                              synthetic)
            self.board.reissued(task_id, synthetic)
            return "reissued"
        except TokenCapExceeded:
            self.emitter.emit("board", task_id, "budget.cap_hit",
                              {"kind": "tokens", "charged": "producer"}, synthetic)
            self.board.reissued(task_id, synthetic)
            return "reissued"

    def _run_task_inner(self, spec, worker, verifier, producer, worker_seat,
                        verifier_seat, synthetic, task_id, max_retries, meter):
        attempt = 1
        while True:
            # Work phase (may CLARIFY mid-flight, then resume)
            while True:
                terminal = self._drain_worker(worker, worker_seat, task_id,
                                              synthetic, meter)
                if terminal[0] == "clarify":
                    self.board.clarify(worker_seat, task_id, terminal[1], synthetic)
                    if hasattr(producer, "set_question"):
                        producer.set_question(terminal[1])
                    ruling = self._answer_clarify(producer, task_id, synthetic, meter)
                    if ruling[0] == "reissue":
                        # No answering TASKING exists — the task itself is
                        # broken. No clarify_resolved (nothing resumed);
                        # straight to reissue, the CLARIFY stands as the
                        # escalation record.
                        self.board.reissued(task_id, synthetic)
                        return "reissued"
                    answer = ruling[1]
                    self.board.clarify_resolved(task_id, answer, synthetic)
                    if hasattr(worker, "record_answer"):
                        worker.record_answer(answer)
                    continue  # worker resumes with the answer
                evidence = terminal[1]
                break

            self.board.completed(worker_seat, task_id, attempt, evidence, synthetic)

            if self.synthetic_injector is not None:
                if not synthetic:
                    raise RuntimeError(
                        "integrity: synthetic_injector set on a real task run "
                        f"({task_id}, synthetic=0) — injection machinery may "
                        "only touch synthetic exercises")
                self.synthetic_injector.maybe_inject(spec, attempt)

            verdict = self._judge(verifier, verifier_seat, task_id, synthetic, meter)
            if verdict[0] == "pass":
                spec_concern = verdict[1] if len(verdict) > 1 else 0
                self.board.verdict_pass(verifier_seat, task_id, attempt,
                                        spec_concern, synthetic)
                from .merge_gate import (RebaseConflict, ScopeViolation,
                                         SuiteFailedPostRebase)
                try:
                    commit = self.merge_gate.merge(spec)
                except ScopeViolation as e:
                    # MERGE.md gate 2: undeclared path in the merge-base diff.
                    # The event always charges the worker (the diff is the
                    # evidence, machinery-emitted); the task then re-enters
                    # the rejection ladder at the current attempt.
                    self.emitter.emit("board", task_id, "task.scope_violation",
                                      {"files": e.files, "charged": "worker"},
                                      synthetic)
                    if attempt <= max_retries:
                        self.board.rejected(task_id, attempt, "worker", synthetic)
                        if hasattr(worker, "record_rejection"):
                            worker.record_rejection(
                                "scope violation (mechanical merge gate): "
                                f"undeclared paths {e.files} in your branch's "
                                "merge-base diff — revert every change outside "
                                "the task's declared files[]")
                        attempt += 1
                        continue
                    self.board.rejected(task_id, attempt, "producer", synthetic)
                    self.board.reissued(task_id, synthetic)
                    return "reissued"
                except RebaseConflict:
                    self.emitter.emit("board", task_id, "task.merge_conflict",
                                      {"charged": "producer"}, synthetic)
                    self.board.reissued(task_id, synthetic)
                    return "reissued"
                except SuiteFailedPostRebase:
                    self.emitter.emit("board", task_id, "task.suite_failed_post_rebase",
                                      {"charged": "producer"}, synthetic)
                    self.board.reissued(task_id, synthetic)
                    return "reissued"
                self.board.merged(task_id, commit, synthetic)
                return "merged"

            # FAIL — evidence-backed, per the ladder
            fail_evidence = verdict[1]
            self.board.verdict_fail(verifier_seat, task_id, attempt,
                                    fail_evidence, synthetic)
            if attempt <= max_retries:
                self.board.rejected(task_id, attempt, "worker", synthetic)
                # The retry must see WHY it failed — a rejection without its
                # evidence is a coin-flip retry. (Gap found in Sortie 2 prep;
                # Sortie 1 never hit a retry so it never bit.)
                if hasattr(worker, "record_rejection"):
                    worker.record_rejection(fail_evidence)
                attempt += 1
                continue  # same worker, one retry
            self.board.rejected(task_id, attempt, "producer", synthetic)
            self.board.reissued(task_id, synthetic)
            return "reissued"


class StubMergeGate:
    """Conformance merge gate: certification stubs the merge machinery like
    every other layer — it returns a deterministic commit hash which
    normalization maps to <COMMIT-1>."""

    def __init__(self):
        self._n = 0

    def merge(self, spec) -> str:
        self._n += 1
        return f"{self._n:040x}"
