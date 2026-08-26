#!/usr/bin/env python3
"""Adversarial checks — prove certification is earned, not coincidental.

1. Attribution is COMPUTED from config: with max_worker_retries=0 the first
   FAIL must charge the producer (no retry). If the ladder were hardcoded to
   the flight script, this would still charge the worker.
2. Off-script stubs fail loudly (StubExhausted), never skip.
3. Exclusive file-scope is enforced at posting time.
4. Emitter timestamps are strictly distinct (normalization mapping stability).
5-8. Synthetic-injection integrity (engine refusal, scratch guard, file-scope guard,
   retry-evidence feedback). 9. Normalizer round-trips the doctrine's own expected ledgers unchanged
   (idempotence on already-normalized input).

Exit 0 = all hold.
"""

import glob
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from harness import config
from harness.board import Board, FileScopeConflict
from harness.emitter import Emitter
from harness.lifecycle import Engine, StubMergeGate, TaskSpec
from harness.normalize import normalize
from harness.seats import StubSeatDriver, StubExhausted
from conformance.flights import WORKER_MODEL, VERIFIER_MODEL, TOOL_TESTS_GREEN, TASK_SHAPE

OUT = "/tmp/trireme-adversarial"
os.makedirs(OUT, exist_ok=True)
failures = []


def check(name, cond, detail=""):
    print(("PASS  " if cond else "FAIL  ") + name + (f" — {detail}" if detail and not cond else ""))
    if not cond:
        failures.append(name)


creds = config.board_credentials()
dhash = config.doctrine_hash()


def fresh(tag, fleet_overrides=None):
    fleet = config.fleet_config()
    fleet.update(fleet_overrides or {})
    ledger = os.path.join(OUT, f"{tag}.jsonl")
    if os.path.exists(ledger):
        os.remove(ledger)
    em = Emitter(f"adv-{tag}", dhash, ledger)
    bd = Board(creds["BOARD_API_URL"], creds["BOARD_API_KEY"], em, f"adversarial-{tag}")
    return Engine(bd, em, fleet, StubMergeGate()), bd, ledger


def cleanup(bd):
    try:
        for wft_id in bd._tasks.values():
            bd._req("DELETE", f"/tasks/{wft_id}")
        bd._req("DELETE", f"/projects/{bd.project_id}")
    except Exception:
        pass


# 1. retries=0 -> first FAIL charges producer
engine, bd, ledger = fresh("retries0", {"max_worker_retries": 0})
spec = TaskSpec("T-ADV1", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "x")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("fail", "tests red")]),
    producer=StubSeatDriver([]),
)
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
rejects = [e for e in events if e["type"] == "task.rejected"]
check("retries=0 charges producer on first fail",
      outcome == "reissued" and len(rejects) == 1
      and rejects[0]["payload"] == {"attempt": 1, "charged": "producer"},
      f"outcome={outcome} rejects={rejects}")
cleanup(bd)

# 2. off-script stub fails loudly
engine, bd, _ = fresh("offscript")
spec = TaskSpec("T-ADV2", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
try:
    engine.run_task(spec,
                    worker=StubSeatDriver([("model", WORKER_MODEL)]),  # no terminal
                    verifier=StubSeatDriver([]), producer=StubSeatDriver([]))
    check("off-script stub fails loudly", False, "no exception raised")
except StubExhausted:
    check("off-script stub fails loudly", True)
except Exception as e:
    check("off-script stub fails loudly", False, f"wrong exception {type(e).__name__}: {e}")
cleanup(bd)

# 3. exclusive file-scope at posting
engine, bd, _ = fresh("filescope")
engine.open_sortie("adv")
engine.post(TaskSpec("T-ADV3a", **TASK_SHAPE))
try:
    engine.post(TaskSpec("T-ADV3b", **TASK_SHAPE))  # same files
    check("file-scope conflict refused at posting", False, "second post accepted")
except FileScopeConflict:
    check("file-scope conflict refused at posting", True)
cleanup(bd)

# 4. emitter ts strictly distinct under rapid fire
em = Emitter("adv-ts", dhash, os.path.join(OUT, "ts.jsonl"))
stamps = [em.emit("runner", None, "sortie.open", {})["ts"] for _ in range(200)]
check("200 rapid-fire timestamps all distinct", len(set(stamps)) == 200)

# 5–8. Synthetic-injection machinery integrity + retry-evidence feedback
from harness.synthetic import MutationInjector, Mutation

# 5. engine refuses a set injector on a real (synthetic=0) task
engine, bd, _ = fresh("injectreal")
spec = TaskSpec("T-ADV5", **TASK_SHAPE)
engine.synthetic_injector = MutationInjector("/tmp/trireme-scratch/x", "T-ADV5",
                                             {}, "trireme-scratch")
engine.open_sortie("adv")
engine.post(spec)
try:
    engine.run_task(spec,
                    worker=StubSeatDriver([("model", WORKER_MODEL), ("done", "x")]),
                    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
                    producer=StubSeatDriver([]))
    check("injector on real task refused by engine", False, "run completed")
except RuntimeError as e:
    check("injector on real task refused by engine", "integrity" in str(e), str(e))
cleanup(bd)

# 6. injector refuses a non-scratch worktree (physical guard)
# A real directory that is emphatically NOT a scratch clone. The example
# fixture is the one such path this repository can count on; the guard must
# refuse it exactly as it would refuse a production checkout.
_NON_SCRATCH = os.path.join(os.path.dirname(os.path.dirname(
    os.path.abspath(__file__))), "example", "fixture")
inj = MutationInjector(_NON_SCRATCH, "T-X",
                       {1: Mutation("src/intervals.py", "x", "y", "m")},
                       "trireme-scratch")
try:
    inj.maybe_inject(TaskSpec("T-X", **TASK_SHAPE), 1)
    check("injector refuses non-scratch worktree", False, "mutation ran")
except RuntimeError as e:
    check("injector refuses non-scratch worktree", "integrity" in str(e), str(e))

# 7. injector refuses a mutation outside the task's file-scope
inj = MutationInjector("/tmp/trireme-scratch/x", "T-X",
                       {1: Mutation("not/in/scope.py", "x", "y", "m")},
                       "trireme-scratch")
try:
    inj.maybe_inject(TaskSpec("T-X", **TASK_SHAPE), 1)
    check("injector refuses out-of-scope mutation", False, "mutation ran")
except RuntimeError as e:
    check("injector refuses out-of-scope mutation", "file-scope" in str(e), str(e))

# 8. rejection evidence reaches the retry (engine -> worker.record_rejection)
class _RecordingWorker(StubSeatDriver):
    def __init__(self, actions):
        super().__init__(actions)
        self.rejections = []

    def record_rejection(self, evidence):
        self.rejections.append(evidence)

engine, bd, _ = fresh("retryevidence", {"max_worker_retries": 1})
spec = TaskSpec("T-ADV8", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
wk = _RecordingWorker([("model", WORKER_MODEL), ("done", "attempt1"),
                       ("model", WORKER_MODEL), ("done", "attempt2")])
outcome = engine.run_task(
    spec, worker=wk,
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("fail", "boundary test red"),
                             ("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
check("rejection evidence reaches the retry",
      outcome == "merged" and wk.rejections == ["boundary test red"],
      f"outcome={outcome} rejections={wk.rejections}")
cleanup(bd)

# 10-14. Budget-cap enforcement (harness/budget.py, DECOMPOSITION #4)

# 10. token cap trips: cumulative fresh+out over budget -> budget.cap_hit
#     {kind: tokens, charged: producer} + reissue; the tripping model.call is
#     still ledgered (it happened) and the verdict never runs.
BIG_MODEL = {"cost_usd": 0.5, "latency_ms": 9000, "model": "stub-worker",
             "provider": "stub", "tokens_in": 999999, "tokens_in_fresh": 60000,
             "tokens_out": 5000}
engine, bd, ledger = fresh("budget-tokens")
spec = TaskSpec("T-ADV10", **{**TASK_SHAPE, "budget_tokens": 100_000})
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", BIG_MODEL), ("done", "a1"),
                           ("model", BIG_MODEL), ("done", "a2")]),
    verifier=StubSeatDriver([("model", BIG_MODEL), ("fail", "never reached")]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
hits = [e for e in events if e["type"] == "budget.cap_hit"]
check("token cap trips -> cap_hit tokens, charged producer, reissued",
      outcome == "reissued" and len(hits) == 1
      and hits[0]["payload"] == {"kind": "tokens", "charged": "producer"}
      and hits[0]["seat"] == "board"
      and not any(e["type"].startswith("verdict") for e in events)
      and [e["type"] for e in events if e["type"] == "model.call"] == ["model.call"] * 2,
      f"outcome={outcome} hits={hits}")
check("token counter uses tokens_in_fresh, not raw tokens_in",
      len([e for e in events if e["type"] == "model.call"]) == 2,
      "a raw-tokens_in counter (999999) would trip on the FIRST call")
cleanup(bd)

# 11. fresh-field fallback: payloads without tokens_in_fresh count tokens_in+out
from harness.budget import (BudgetMeter, SessionCapExceeded,
                            TokenCapExceeded, WallCapExceeded)
m = BudgetMeter(10_000, 3600)
m.feed({"tokens_in": 9_000, "tokens_out": 2_000})  # no tokens_in_fresh -> 11k
try:
    m.check()
    check("fallback counter (no tokens_in_fresh) trips on tokens_in+out", False)
except TokenCapExceeded:
    check("fallback counter (no tokens_in_fresh) trips on tokens_in+out", True)

# 12. wall cap trips (deterministic injected clock) -> cap_hit wall + reissue
fake_now = [0.0]
engine, bd, ledger = fresh("budget-wall")
engine.budget_clock = lambda: fake_now[0]
spec = TaskSpec("T-ADV12", **{**TASK_SHAPE, "budget_wall_s": 600})

class _SlowWorker(StubSeatDriver):
    def next_actions(self):
        fake_now[0] += 700.0  # the session "takes" 700s of a 600s budget
        return super().next_actions()

engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec, worker=_SlowWorker([("model", WORKER_MODEL), ("done", "x")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
hits = [e for e in events if e["type"] == "budget.cap_hit"]
check("wall cap trips -> cap_hit wall, charged producer, reissued",
      outcome == "reissued" and len(hits) == 1
      and hits[0]["payload"] == {"kind": "wall", "charged": "producer"},
      f"outcome={outcome} hits={hits}")
cleanup(bd)

# 13. healthy task under both caps: no cap_hit, merges untouched
engine, bd, ledger = fresh("budget-healthy")
spec = TaskSpec("T-ADV13", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "x")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
check("healthy task never sees the meter",
      outcome == "merged" and not any(e["type"] == "budget.cap_hit" for e in events),
      f"outcome={outcome}")
cleanup(bd)

# 14. live-seat wall plumbing: remaining wall budget becomes the subprocess
#     timeout (floor 1s, ceiling MAX_SESSION_S) — captured via stubbed runner
from harness import live_seats as _ls
captured = []
_real_run_headless = _ls.run_headless
_ls.run_headless = lambda *a, **kw: (captured.append(kw.get("timeout_s")) or
                                     ({"outcome": "done"}, {}))
try:
    for wall_s, elapsed, want in [(600, 0.0, 600),      # full budget remains
                                  (600, 550.0, 50),     # partial remains
                                  (600, 650.0, 1),      # overrun -> floor 1s
                                  (99999, 0.0, 3600)]:  # ceiling = session max
        seat = _ls._LiveSeat("worker", "sys", "/tmp", "/tmp/trireme-adversarial")
        seat.set_budget_meter(BudgetMeter(100_000, wall_s, clock=lambda e=elapsed: e))
        # meter _t0 = elapsed at construction; shift clock forward is not
        # needed — remaining = wall_s - 0 here, so emulate elapsed via budget
        seat._budget_meter._t0 = 0.0
        seat._run("p")
        got = captured[-1]
        if got != want:
            check(f"live seat timeout plumbing (wall={wall_s}, elapsed={elapsed})",
                  False, f"timeout_s={got}, want {want}")
            break
    else:
        check("live seat timeout = min(remaining wall, session max), floor 1s", True)
finally:
    _ls.run_headless = _real_run_headless

# 15. per-session cap (PR #4 redline): ONE session over 150k fresh+out trips
#     kind=session_tokens even though the task-cumulative budget is fine —
#     and outranks kind=tokens when a single call trips both.
FAT_SESSION = {"cost_usd": 0.9, "latency_ms": 9000, "model": "stub-worker",
               "provider": "stub", "tokens_in": 999999,
               "tokens_in_fresh": 160_000, "tokens_out": 8_000}
engine, bd, ledger = fresh("budget-session",
                           {"default_session_budget_tokens": 150_000})
spec = TaskSpec("T-ADV15", **{**TASK_SHAPE, "budget_tokens": 168_001})
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec, worker=StubSeatDriver([("model", FAT_SESSION), ("done", "x")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
hits = [e for e in events if e["type"] == "budget.cap_hit"]
check("single fat session trips kind=session_tokens under a healthy task budget",
      outcome == "reissued" and len(hits) == 1
      and hits[0]["payload"] == {"kind": "session_tokens", "charged": "producer"},
      f"outcome={outcome} hits={hits}")
m = BudgetMeter(100_000, 3600, session_budget_tokens=150_000)
m.feed({"tokens_in_fresh": 160_000, "tokens_out": 8_000})  # trips both caps
try:
    m.check()
    check("session cap outranks cumulative cap when one call trips both", False)
except SessionCapExceeded:
    check("session cap outranks cumulative cap when one call trips both", True)
except TokenCapExceeded:
    check("session cap outranks cumulative cap when one call trips both", False,
          "cumulative kind won; per-call cause must outrank the aggregate")
cleanup(bd)

# 16. session cap absent from config = lawfully unenforced (config is law;
#     an undefined budget is not a silent off switch, it is undefined law)
fleet_nokey = config.fleet_config()
fleet_nokey.pop("default_session_budget_tokens", None)
m = BudgetMeter(fleet_nokey["default_task_budget_tokens"], 3600,
                session_budget_tokens=fleet_nokey.get("default_session_budget_tokens"))
m.feed({"tokens_in_fresh": 200_000, "tokens_out": 8_000})
try:
    m.check()
    check("absent config key -> session cap lawfully unenforced (cumulative still live)",
          m.tokens_spent == 208_000)
except SessionCapExceeded:
    check("absent config key -> session cap lawfully unenforced (cumulative still live)",
          False, "session cap fired with no law defining it")
except TokenCapExceeded:
    # fine iff the cumulative budget is genuinely smaller than 208k
    check("absent config key -> session cap lawfully unenforced (cumulative still live)",
          fleet_nokey["default_task_budget_tokens"] < 208_000)

# 17. producer reissue ruling: a CLARIFY answered with ("reissue", reason)
#     reissues the task — no clarify_resolved (nothing resumed), no merge,
#     the CLARIFY event stands as the escalation record.
engine, bd, ledger = fresh("reissue-ruling")
spec = TaskSpec("T-ADV17", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", WORKER_MODEL),
                           ("clarify", "acceptance asserts q==6 and q==5 for the same captured value")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([("model", VERIFIER_MODEL),
                             ("reissue", "contradictory acceptance, no lawful deliverable")]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
types = [e["type"] for e in events if e["task_id"] == "T-ADV17"]
check("producer reissue ruling -> clarify stands, task reissued, nothing resumed",
      outcome == "reissued"
      and types.count("task.clarify") == 1
      and types.count("task.clarify_resolved") == 0
      and types.count("task.reissued") == 1
      and types.count("task.merged") == 0,
      f"outcome={outcome} types={types}")
cleanup(bd)

# 18. seat infrastructure failure (api_error, nonzero CLI exit) is NOT
#     sortie-fatal: the task reissues, the wall-timeout subclass still maps
#     to its budget event, and the engine records the infra diagnosis.
from harness.claude_cli import SeatSessionError, SeatWallTimeout

class _DyingSeat(StubSeatDriver):
    def __init__(self, exc):
        super().__init__([])
        self.exc = exc

    def next_actions(self):
        raise self.exc
        yield  # pragma: no cover

engine, bd, ledger = fresh("infra-fail")
spec = TaskSpec("T-ADV18", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec, worker=_DyingSeat(SeatSessionError("session exited 1", "/tmp/t.json")),
    verifier=StubSeatDriver([]), producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
check("seat infra failure -> reissued, not sortie-fatal, no cap_hit",
      outcome == "reissued"
      and engine.last_infra_error is not None
      and not any(e["type"] == "budget.cap_hit" for e in events)
      and any(e["type"] == "task.reissued" for e in events),
      f"outcome={outcome} infra={engine.last_infra_error}")
cleanup(bd)

engine, bd, ledger = fresh("walltimeout-order")
spec = TaskSpec("T-ADV18B", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec, worker=_DyingSeat(SeatWallTimeout("killed at wall cap", "/tmp/t.json")),
    verifier=StubSeatDriver([]), producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
hits = [e for e in events if e["type"] == "budget.cap_hit"]
check("SeatWallTimeout still maps to cap_hit wall (subclass ordering)",
      outcome == "reissued" and len(hits) == 1
      and hits[0]["payload"]["kind"] == "wall",
      f"outcome={outcome} hits={hits}")
cleanup(bd)

# 19-21. Mechanical file-scope gate (doctrine PR #5, MERGE.md gate 2)
from harness.merge_gate import GitMergeGate, ScopeViolation


class _ScopeOnceGate(StubMergeGate):
    """Raises ScopeViolation on the first merge attempt, passes after —
    models a worker whose retry reverts the undeclared edit."""

    def __init__(self, files):
        super().__init__()
        self.files = files
        self.calls = 0

    def merge(self, spec):
        self.calls += 1
        if self.calls == 1:
            raise ScopeViolation(list(self.files))
        return super().merge(spec)


class _RecordingWorker(StubSeatDriver):
    def __init__(self, script):
        super().__init__(script)
        self.rejections = []

    def record_rejection(self, evidence):
        self.rejections.append(evidence)


# 19. scope violation -> event {files, charged: worker} + worker-charged
#     rejection rung + the SAME worker's retry (with the diff as evidence)
#     merges clean.
engine, bd, ledger = fresh("scopegate")
engine.merge_gate = _ScopeOnceGate(["src/shared_helpers.py"])
spec = TaskSpec("T-ADV19", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
w = _RecordingWorker([
    ("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "attempt 1"),
    ("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "reverted undeclared edit"),
])
outcome = engine.run_task(
    spec, worker=w,
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0),
                             ("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
sv = [e for e in events if e["type"] == "task.scope_violation"]
rejects = [e for e in events if e["type"] == "task.rejected"]
check("scope violation -> typed event + worker rejection rung + retry merges",
      outcome == "merged" and len(sv) == 1
      and sv[0]["payload"] == {"files": ["src/shared_helpers.py"], "charged": "worker"}
      and len(rejects) == 1 and rejects[0]["payload"]["charged"] == "worker"
      and len(w.rejections) == 1 and "shared_helpers.py" in w.rejections[0]
      and any(e["type"] == "task.merged" for e in events),
      f"outcome={outcome} sv={sv} rejects={rejects} fed={w.rejections}")
cleanup(bd)

# 20. ladder still computed from config: retries=0 -> scope violation is the
#     hard stop (producer-charged rejection, reissue, no merge).
engine, bd, ledger = fresh("scopegate0", {"max_worker_retries": 0})
engine.merge_gate = _ScopeOnceGate(["src/shared_helpers.py"])
spec = TaskSpec("T-ADV20", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "x")]),
    verifier=StubSeatDriver([("model", VERIFIER_MODEL), ("pass", 0)]),
    producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
rejects = [e for e in events if e["type"] == "task.rejected"]
check("retries=0: scope violation is the hard stop, producer-charged",
      outcome == "reissued"
      and any(e["type"] == "task.scope_violation" for e in events)
      and len(rejects) == 1 and rejects[0]["payload"]["charged"] == "producer"
      and not any(e["type"] == "task.merged" for e in events),
      f"outcome={outcome} rejects={rejects}")
cleanup(bd)

# 21. GitMergeGate diffs from the MERGE-BASE: after main moves, the branch is
#     not indicted for main's own commits (the SYN-M2 false-charge class) —
#     but a genuine undeclared edit on the branch IS refused.
import subprocess, tempfile, shutil


def _sh(cwd, *args):
    return subprocess.run(args, cwd=cwd, capture_output=True, text=True, check=True)


scratch = tempfile.mkdtemp(prefix="adv-scopegate-")
try:
    _sh(scratch, "git", "init", "-q", "-b", "master")
    _sh(scratch, "git", "config", "user.email", "adv@test")
    _sh(scratch, "git", "config", "user.name", "adv")
    open(os.path.join(scratch, "declared.py"), "w").write("a\n")
    open(os.path.join(scratch, "other.py"), "w").write("b\n")
    _sh(scratch, "git", "add", "-A")
    _sh(scratch, "git", "commit", "-qm", "base")
    # branch: edit ONLY the declared file
    _sh(scratch, "git", "checkout", "-qb", "task/T-ADV21")
    open(os.path.join(scratch, "declared.py"), "w").write("a2\n")
    _sh(scratch, "git", "commit", "-aqm", "declared edit")
    # master moves underneath (touches other.py) — the SYN-M2 setup
    _sh(scratch, "git", "checkout", "-q", "master")
    open(os.path.join(scratch, "other.py"), "w").write("b2\n")
    _sh(scratch, "git", "commit", "-aqm", "master moved")
    _sh(scratch, "git", "checkout", "-q", "task/T-ADV21")

    gate = GitMergeGate(scratch, "master", ["true"])

    class _S:
        task_id = "T-ADV21"
        files = ["declared.py"]

    commit = gate.merge(_S())
    ok_innocent = bool(commit)

    # now a branch with a genuine undeclared edit must be refused, with the
    # undeclared path (and ONLY it) as evidence
    _sh(scratch, "git", "checkout", "-qb", "task/T-ADV21B", "master")
    open(os.path.join(scratch, "declared.py"), "w").write("a3\n")
    open(os.path.join(scratch, "other.py"), "w").write("b3\n")
    _sh(scratch, "git", "commit", "-aqm", "one declared, one undeclared")

    class _S2:
        task_id = "T-ADV21B"
        files = ["declared.py"]

    try:
        gate.merge(_S2())
        ok_guilty, detail21 = False, "undeclared edit merged"
    except ScopeViolation as e:
        ok_guilty, detail21 = e.files == ["other.py"], f"files={e.files}"
    check("merge gate diffs from merge-base: innocent branch passes after "
          "master moves; undeclared edit refused with exact paths",
          ok_innocent and ok_guilty, detail21)
finally:
    shutil.rmtree(scratch, ignore_errors=True)

# 22. seat.error (doctrine PR #5): infra death emits the typed event with the
#     FAILING seat attributed, charged nobody (no task.rejected, no cap_hit).
engine, bd, ledger = fresh("seaterror")
spec = TaskSpec("T-ADV22", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec,
    worker=StubSeatDriver([("model", WORKER_MODEL), ("tool", TOOL_TESTS_GREEN), ("done", "x")]),
    verifier=_DyingSeat(SeatSessionError("session exited 1", "/tmp/t.json")),
    producer=StubSeatDriver([]), verifier_seat="verifier-1")
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
se = [e for e in events if e["type"] == "seat.error"]
check("seat.error emitted with failing seat, charged nobody",
      outcome == "reissued" and len(se) == 1
      and se[0]["payload"]["seat"] == "verifier-1"
      and "session exited 1" in se[0]["payload"]["reason"]
      and "charged" not in se[0]["payload"]
      and not any(e["type"] in ("task.rejected", "budget.cap_hit") for e in events),
      f"outcome={outcome} se={se}")
cleanup(bd)

# 23. wall timeout is a BUDGET event, not an infra one: no seat.error beside
#     the cap_hit (subclass ordering holds under the new emission).
engine, bd, ledger = fresh("wall-not-infra")
spec = TaskSpec("T-ADV23", **TASK_SHAPE)
engine.open_sortie("adv")
engine.post(spec)
outcome = engine.run_task(
    spec, worker=_DyingSeat(SeatWallTimeout("killed at wall cap", "/tmp/t.json")),
    verifier=StubSeatDriver([]), producer=StubSeatDriver([]))
engine.close_sortie()
events = [json.loads(l) for l in open(ledger)]
check("wall kill emits cap_hit wall and NO seat.error",
      outcome == "reissued"
      and any(e["type"] == "budget.cap_hit" and e["payload"]["kind"] == "wall"
              for e in events)
      and not any(e["type"] == "seat.error" for e in events),
      f"outcome={outcome}")
cleanup(bd)

# 9. normalizer idempotent on the doctrine's expected ledgers
ok = True
for path in glob.glob(os.path.join(config.DOCTRINE_REPO, "CONTRACTS", "conformance",
                                   "*", "expected-ledger.jsonl")):
    expected = open(path).read()
    if normalize(expected) != expected:
        ok = False
        print(f"  normalizer changed {path}")
check("normalizer idempotent on expected ledgers", ok)

print("\nALL HOLD" if not failures else f"\n{len(failures)} FAILED: {failures}")
sys.exit(0 if not failures else 1)
