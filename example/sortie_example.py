#!/usr/bin/env python3
"""The worked example — one sortie, three tasks, against `example/fixture`.

This file is the reference for what a well-formed task spec looks like under
`doctrine/CONTRACTS/DECOMPOSITION.md`. Read `example/README.md` alongside it;
the README says *why* each spec is shaped the way it is, and this file is the
shape itself.

Two modes, and the default is the safe one:

    python3 example/sortie_example.py
        Manifest validation only. Zero model spend, no board writes. Checks
        the dependency graph, checks every declared path against the fixture
        as it actually exists on disk, and checks that no two tasks open at
        the same time declare the same file.

    python3 example/sortie_example.py --launch-approved
        Live seats. Spends real tokens against a real board. Refused without
        the flag, deliberately: a sortie is an expensive thing to start by
        accident.

Bootstrap the fixture first (it is plain files in this repository, not a
nested git repo):

    bash example/bootstrap_fixture.sh          # -> /tmp/trireme-example-fixture
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from harness import config                                     # noqa: E402
from harness.board import Board                                # noqa: E402
from harness.emitter import Emitter                            # noqa: E402
from harness.lifecycle import Engine, TaskSpec                 # noqa: E402
from harness.live_seats import (LiveProducer, LiveVerifier,    # noqa: E402
                                LiveWorker)
from harness.merge_gate import GitMergeGate                    # noqa: E402
from harness.project import ProjectProfile                     # noqa: E402

FIXTURE = os.environ.get("TRIREME_EXAMPLE_FIXTURE",
                         "/tmp/trireme-example-fixture")
LEDGER_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "ledgers")

# ---------------------------------------------------------------------------
# The project profile. Everything project-shaped lives here and nowhere else —
# this is the whole of what the harness needs to know about the repository it
# is flying against. Swap this block and the same harness flies your project.
# ---------------------------------------------------------------------------
PROJECT = ProjectProfile(
    repo=FIXTURE,
    main_branch="main",
    test_cmd="bash tools/run_tests.sh",
    # The permission form of the test command, spelled out. The harness will
    # not infer it: a shell command and a permission entry are different
    # grammars, and guessing turns a blocked seat into a mystery.
    worker_tools=["Bash(bash tools/run_tests.sh)"],
    verifier_tools=["Bash(bash tools/run_tests.sh)"],
    # This fixture's build generates nothing, so there is nothing to warn
    # about. A project whose build writes lock files, sidecars or generated
    # sources names them here, and the sentence is spliced into the Worker's
    # staging law.
    generated_artifacts="",
)

SORTIE_BRIEF = (
    "Example sortie. Repo: a small closed-integer-interval library "
    "(`src/intervals.py`) with a unittest suite under `tests/`. The whole "
    "test surface runs as one command, `bash tools/run_tests.sh`, which "
    "exits 0 when green. Objective: close the containment gap, repair the "
    "overlap defect, and prove the two agree. Standing rulings for CLARIFY "
    "answers: acceptance criteria outrank the task description wherever they "
    "conflict; acceptance tests are immutable within a task and a "
    "requested-but-undeclared test edit is never authorized; where the lawful "
    "reading makes a change unnecessary, close minimal with evidence rather "
    "than inventing work."
)

# ---------------------------------------------------------------------------
# The tasks.
#
# Each one carries all four claimability requirements: acceptance criteria as
# tests-to-make-pass, an exclusive file-scope, declared dependency edges, and
# budgets (omitted here, which accepts the fleet defaults from
# doctrine/config/fleet.toml — the Producer may tighten them, never loosen).
# ---------------------------------------------------------------------------
TASKS = [
    TaskSpec(
        "T-001", "Interval membership test",
        "src/intervals.py's Interval has no membership test, so callers "
        "reach for `i.lo <= x <= i.hi` by hand and the closed-interval "
        "convention is re-derived at every call site. "
        "(1) Add `__contains__(self, value: int) -> bool` to Interval, "
        "returning True when value is one of the integers in the closed "
        "interval. Both endpoints are members: 3 in Interval(1, 3) is True. "
        "Non-integers are out of scope for this task — do not add type "
        "coercion or validation. "
        "(2) Write tests/test_contains.py covering: both endpoints are "
        "members; an interior point is a member; the two points just outside "
        "each endpoint are not; a single-point interval contains exactly its "
        "one value; membership works across zero and in fully negative "
        "intervals. "
        "Do not modify overlaps() — the defect in it is T-002's scope.",
        files=["src/intervals.py", "tests/test_contains.py"],
        deps=[], phase="example",
        acceptance_criteria=(
            "bash tools/run_tests.sh exits 0; tests/test_contains.py exists "
            "and its cases run; `3 in Interval(1, 3)` and `1 in Interval(1, 3)` "
            "are both True; `0 in Interval(1, 3)` and `4 in Interval(1, 3)` "
            "are both False; the pre-existing 9 tests still pass."),
    ),
    TaskSpec(
        "T-002", "Repair Interval.overlaps at touching endpoints",
        "Interval.overlaps uses strict comparisons on what the class "
        "documents as CLOSED intervals, so two intervals that share exactly "
        "one endpoint are reported as disjoint: Interval(1, 3).overlaps("
        "Interval(3, 5)) returns False and must return True. "
        "(1) Fix the comparison in src/intervals.py so overlaps is true "
        "exactly when the two intervals share at least one integer. "
        "(2) Replace the DEFECT paragraph in the docstring with a plain "
        "statement of the rule. "
        "(3) Write tests/test_overlaps.py covering: touching at a shared "
        "endpoint in both argument orders; adjacent-but-not-touching "
        "([1,3] and [4,6]) is False; nesting is True; identical intervals "
        "are True; single-point intervals on and off the boundary; the "
        "relation is symmetric over a fixed sweep of pairs. "
        "The existing TestOverlaps cases in tests/test_intervals.py must "
        "keep passing unchanged — that file is NOT in your file-scope.",
        # Declared as a dependency because T-001 also owns src/intervals.py,
        # and no two OPEN tasks may declare the same file. See example/README.
        files=["src/intervals.py", "tests/test_overlaps.py"],
        deps=["T-001"], phase="example",
        acceptance_criteria=(
            "bash tools/run_tests.sh exits 0; "
            "Interval(1, 3).overlaps(Interval(3, 5)) is True; "
            "Interval(3, 5).overlaps(Interval(1, 3)) is True; "
            "Interval(1, 3).overlaps(Interval(4, 6)) is False; "
            "tests/test_intervals.py is unchanged in the diff."),
    ),
    TaskSpec(
        "T-003", "Cross-check overlaps against merge and membership",
        "overlaps, merge and membership are three descriptions of the same "
        "geometry, and nothing currently forces them to agree — which is why "
        "the T-002 defect survived a green suite. Write "
        "tests/test_invariants.py asserting, over a deterministic sweep of "
        "interval pairs (both endpoints ranging over -4..4, every legal "
        "pair, so the sweep is reproducible rather than random): "
        "(a) a.overlaps(b) is True if and only if some integer is in both a "
        "and b, computed independently by membership; "
        "(b) a.overlaps(b) == b.overlaps(a) for every pair; "
        "(c) a.merge(b).length() == a.length() + b.length() - shared, where "
        "shared is the count of integers in both, again by membership; "
        "(d) every integer of a and every integer of b is in a.merge(b). "
        "This suite exists to fail if the T-002 repair is reverted — check "
        "that it does before you claim done.",
        files=["tests/test_invariants.py"],
        deps=["T-001", "T-002"], phase="example",
        acceptance_criteria=(
            "bash tools/run_tests.sh exits 0; tests/test_invariants.py "
            "sweeps every legal interval pair with endpoints in -4..4; "
            "reverting the T-002 comparison fix locally makes this suite "
            "fail (proven adversarial teeth, quoted in the evidence line)."),
    ),
]

# Waves respect the dependency edges. T-001 and T-002 could not be posted
# together anyway — they share src/intervals.py, and the board refuses the
# second at posting time.
WAVES = [["T-001"], ["T-002"], ["T-003"]]


# ------------------------------------------------------------------ dry run
def validate_manifest() -> int:
    """Everything checkable without spending a token or touching the board."""
    problems = []
    ids = [t.task_id for t in TASKS]

    if len(set(ids)) != len(ids):
        problems.append("duplicate task ids")

    for t in TASKS:
        for dep in t.deps:
            if dep not in ids:
                problems.append(f"{t.task_id}: dependency {dep} is not in this list")
        if not t.files:
            problems.append(f"{t.task_id}: empty file-scope")
        if not t.acceptance_criteria:
            problems.append(f"{t.task_id}: no acceptance criteria")

    # dependency edges must order every file-scope collision
    order = {tid: w for w, wave in enumerate(WAVES) for tid in wave}
    for i, a in enumerate(TASKS):
        for b in TASKS[i + 1:]:
            shared = set(a.files) & set(b.files)
            if not shared:
                continue
            if order.get(a.task_id) == order.get(b.task_id):
                problems.append(
                    f"{a.task_id} and {b.task_id} share {sorted(shared)} "
                    f"and are in the same wave — the board will refuse the "
                    f"second at posting")
            elif not (a.task_id in b.deps or b.task_id in a.deps):
                problems.append(
                    f"{a.task_id} and {b.task_id} share {sorted(shared)} "
                    f"with no dependency edge between them")

    # declared paths against the fixture as it actually is
    if os.path.isdir(FIXTURE):
        for t in TASKS:
            for path in t.files:
                full = os.path.join(FIXTURE, path)
                exists = os.path.exists(full)
                new = path.startswith("tests/test_")
                if not exists and not new:
                    problems.append(
                        f"{t.task_id}: declares {path}, which does not exist "
                        f"in the fixture and is not a new test file")
    else:
        problems.append(
            f"fixture not found at {FIXTURE} — run "
            f"`bash example/bootstrap_fixture.sh` first (path checks skipped)")

    for t in TASKS:
        print(f"  {t.task_id}  {t.title}")
        print(f"        files: {t.files}")
        print(f"        deps:  {t.deps or '(none)'}")

    print()
    if problems:
        for p in problems:
            print(f"  PROBLEM: {p}")
        print(f"\n{len(problems)} problem(s). Nothing was spent.")
        return 1
    print("manifest OK — dependency graph, file-scope exclusivity and "
          "declared paths all check out. Nothing was spent.")
    return 0


# -------------------------------------------------------------------- launch
def worktree_for(task_id: str) -> str:
    path = f"/tmp/trireme-example/{task_id}"
    subprocess.run(["git", "-C", FIXTURE, "worktree", "remove", "--force", path],
                   capture_output=True)
    subprocess.run(["git", "-C", FIXTURE, "branch", "-D", f"task/{task_id}"],
                   capture_output=True)
    subprocess.run(["git", "-C", FIXTURE, "worktree", "add", "-B",
                    f"task/{task_id}", path, PROJECT.main_branch], check=True,
                   capture_output=True)
    return path


def worktree_cleanup(task_id: str, path: str):
    subprocess.run(["git", "-C", FIXTURE, "worktree", "remove", "--force", path],
                   capture_output=True)
    subprocess.run(["git", "-C", FIXTURE, "branch", "-D", f"task/{task_id}"],
                   capture_output=True)


def launch() -> int:
    os.makedirs(LEDGER_DIR, exist_ok=True)
    creds = config.board_credentials()
    dhash = config.doctrine_hash()
    fleet = config.fleet_config()
    sortie_id = f"example-{int(time.time())}"
    emitter = Emitter(sortie_id, dhash,
                      os.path.join(LEDGER_DIR, f"{sortie_id}.jsonl"))
    board = Board(creds["BOARD_API_URL"], creds["BOARD_API_KEY"], emitter,
                  sortie_id)
    gate = GitMergeGate(FIXTURE, PROJECT.main_branch, PROJECT.test_cmd.split())
    engine = Engine(board, emitter, fleet, gate)
    sessions_root = os.path.join(LEDGER_DIR, sortie_id + "-sessions")

    specs = {t.task_id: t for t in TASKS}
    engine.open_sortie("example")
    try:
        for wave in WAVES:
            for tid in wave:
                spec = specs[tid]
                engine.post(spec)
                wt = worktree_for(tid)
                gate.worktrees[tid] = wt
                sessions = os.path.join(sessions_root, tid)
                tasking = (f"{spec.title}\n\n{spec.description}\n\n"
                           f"Acceptance criteria: {spec.acceptance_criteria}\n"
                           f"File-scope (touch NOTHING else): {spec.files}")
                producer = LiveProducer(FIXTURE, SORTIE_BRIEF, sessions,
                                        profile=PROJECT)
                worker = LiveWorker(wt, tasking, PROJECT.test_cmd, sessions,
                                    profile=PROJECT)
                producer.on_answer = worker.record_answer
                verifier = LiveVerifier(wt, tasking, PROJECT.test_cmd, sessions,
                                        profile=PROJECT)
                outcome = engine.run_task(spec, worker, verifier, producer)
                print(f"{tid}: {outcome}")
                if outcome == "merged":
                    worktree_cleanup(tid, wt)
                else:
                    print(f"HARD STOP at {tid}. A reissue needs Producer "
                          f"re-decomposition, which is a human decision. "
                          f"Worktree kept for inspection: {wt}")
                    return 1
    finally:
        engine.close_sortie()
        print(f"ledger: {emitter.ledger_path}")
    return 0


def main():
    if "--launch-approved" in sys.argv:
        sys.exit(launch())
    print("DRY RUN — manifest validation only. Nothing will be spent.")
    print(f"fixture: {FIXTURE}\n")
    rc = validate_manifest()
    print("\nTo fly it for real: "
          "python3 example/sortie_example.py --launch-approved")
    sys.exit(rc)


if __name__ == "__main__":
    main()
