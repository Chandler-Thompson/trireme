"""Live decomposition driver (Phase 2, ROLES/PRODUCER.md 'state discipline') —
the one genuinely new piece of machinery the live-producer sortie needs.
Everything else in Phase 2 reuses Phase-1 plumbing unchanged (Board.post_task,
Engine, the per-task LiveProducer CLARIFY-ruling seat).

Drives a LiveProducerDecompose seat through:
  propose -> locally validate each task against the four claimability
  requirements -> post the clean ones (Board enforces exclusive file-scope at
  posting) -> bounce anything rejected back to the producer for revision ->
  repeat, up to MAX_REVISION_ROUNDS -> stop.

Runs under its own phase-level budget (NOT a per-task budget — decomposition
happens before any task exists), same BudgetMeter class as every task uses.
A cap trip ends decomposition gracefully: whatever was posted before the trip
flies, the cap event is journaled, nothing already posted is torn down
(Phase 2 proposal, "Cost envelope").

State discipline: every round's proposal and posting result is written to
`state_path` after the round completes, so the phase is killable and
respawnable — nothing about what has or hasn't been posted lives only in the
producer's session context.

Ledger note: `producer.decompose_bounce` is harness-level telemetry, not yet
a doctrine-blessed event (no profiler view reads it). It is emitted so the
data exists if a future doctrine PR decides it is worth measuring formally —
same posture as seat.error before PR #5. Flagged here, not decided here.
"""

import json

from .board import FileScopeConflict
from .budget import (BudgetMeter, SessionCapExceeded, TokenCapExceeded,
                     WallCapExceeded)
from .claude_cli import SeatSessionError, SeatWallTimeout
from .lifecycle import TaskSpec

MAX_REVISION_ROUNDS = 3


def _validate_local(t: dict, seen_ids: set, batch_ids: set) -> list:
    """The four claimability requirements (DECOMPOSITION.md #1-4), checked
    before a proposed task ever reaches the board. Returns problem strings
    (empty == clean)."""
    problems = []
    tid = t.get("task_id")
    if not tid:
        problems.append("missing task_id")
    if not (t.get("acceptance_criteria") or "").strip():
        problems.append("acceptance_criteria empty — prose criteria are not criteria")
    if not t.get("files"):
        problems.append("files[] empty — no file-scope declared")
    deps = t.get("deps", [])
    if not isinstance(deps, list):
        problems.append("deps must be a list")
    else:
        unknown = [d for d in deps if d not in seen_ids and d not in batch_ids]
        if unknown:
            problems.append(f"deps reference unknown task_id(s): {unknown}")
    return problems


def run_decomposition(engine, producer_seat, state_path: str, phase: str,
                      phase_budget_tokens: int, phase_budget_wall_s: int,
                      session_budget_tokens=None, synthetic: int = 0):
    """Drive one live decomposition phase to completion.

    Returns (posted: list[TaskSpec], dropped: list[dict], state: dict).
    dropped tasks are never silently lost — every one is named with its
    reason in the returned state and written to state_path.
    """
    meter = BudgetMeter(phase_budget_tokens, phase_budget_wall_s,
                        session_budget_tokens=session_budget_tokens)
    if hasattr(producer_seat, "set_budget_meter"):
        producer_seat.set_budget_meter(meter)

    posted: list = []
    dropped: list = []
    seen_ids: set = set()
    addendum = None
    state = {"rounds": [], "posted": [], "dropped": [], "cap_hit": None}

    def _save():
        with open(state_path, "w") as f:
            json.dump(state, f, indent=1, sort_keys=True)

    for round_n in range(1, MAX_REVISION_ROUNDS + 1):
        try:
            result, model_call = producer_seat.propose(addendum)
        except (SeatSessionError, SeatWallTimeout) as e:
            state["rounds"].append({"round": round_n, "seat_error": str(e)})
            _save()
            break

        engine.emitter.emit("producer", None, "model.call", model_call, synthetic)
        meter.feed(model_call)
        try:
            meter.check()
        except SessionCapExceeded:
            cap_kind = "session_tokens"
        except TokenCapExceeded:
            cap_kind = "tokens"
        except WallCapExceeded:
            cap_kind = "wall"
        else:
            cap_kind = None
        if cap_kind:
            engine.emitter.emit("board", None, "budget.cap_hit",
                               {"kind": cap_kind, "charged": "producer",
                                "phase": "decomposition"}, synthetic)
            state["rounds"].append({"round": round_n, "cap_hit": cap_kind})
            state["cap_hit"] = cap_kind
            _save()
            break

        proposed = result.get("tasks", [])
        batch_ids = {t.get("task_id") for t in proposed if t.get("task_id")}
        round_record = {"round": round_n,
                        "proposed": sorted(batch_ids),
                        "rationale": result.get("rationale", ""),
                        "posted": [], "rejected": []}

        rejects = []
        for t in proposed:
            tid = t.get("task_id")
            if tid in seen_ids:
                rejects.append((tid, "duplicate task_id — already posted"))
                continue
            problems = _validate_local(t, seen_ids, batch_ids)
            if problems:
                rejects.append((tid or "<missing-id>", "; ".join(problems)))
                continue
            spec = TaskSpec(
                tid, t.get("title", tid), t.get("description", ""),
                files=t["files"], deps=t.get("deps", []), phase=phase,
                acceptance_criteria=t.get("acceptance_criteria", ""),
                budget_tokens=t.get("budget_tokens"),
                budget_wall_s=t.get("budget_wall_s"))
            try:
                engine.post(spec, synthetic=synthetic)
            except FileScopeConflict as e:
                rejects.append((tid, str(e)))
                continue
            posted.append(spec)
            seen_ids.add(tid)
            round_record["posted"].append(tid)

        for tid, reason in rejects:
            round_record["rejected"].append({"task_id": tid, "reason": reason})
            # Journaled (see module docstring) — not yet a doctrine metric.
            engine.emitter.emit("producer", tid, "producer.decompose_bounce",
                               {"round": round_n, "reason": reason}, synthetic)

        state["rounds"].append(round_record)
        state["posted"] = [s.task_id for s in posted]
        _save()

        if not rejects:
            break
        if round_n == MAX_REVISION_ROUNDS:
            for tid, reason in rejects:
                dropped.append({"task_id": tid, "reason": reason,
                                "note": "revision rounds exhausted"})
            break
        addendum = (
            "REVISION addendum: the following proposed tasks were REJECTED at "
            "posting and must be fixed or dropped in your reply. Already-posted "
            "tasks are final — do not resubmit them, and do not repeat their "
            "task_ids or files. Return the SAME JSON shape containing ONLY "
            "corrected replacements for the rejected ids (new ids allowed if "
            "you re-cut scope), or simply omit an id to drop it (say why in "
            "rationale).\n\nRejected:\n" +
            "\n".join(f"- {tid}: {reason}" for tid, reason in rejects))

    if dropped:
        state["dropped"] = dropped
        _save()
    return posted, dropped, state
