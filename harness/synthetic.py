"""Synthetic failure injection (CONTRACTS/CONFORMANCE.md: every failure
path exercised >=3x in real sorties, tagged synthetic:1).

Design (Sortie 2, redlined by the Principal before flight):
- Synthetic exercise tasks run against a SCRATCH clone of the fixture; their
  whole lifecycle carries synthetic:1 and their merges land on scratch main.
  Real fixture history and real metrics are untouched by construction.
- Mutation injection: after the worker commits ("task.completed"), machinery
  plants a defect on the branch with an innocuous commit message. Seats fly
  BLIND — the telemetry tag satisfies the law (machinery-level bookkeeping);
  blind seats measure real verifier/worker behavior, not their reaction to a
  label. The verifier's FAIL is genuinely evidence-backed: the suite really
  fails.
- Exercised paths:
    rejection #1 (worker charged, retry, fix, merge)   -> inject_on_attempts=[1]
    rejection #2 (hard stop, producer charged, reissue) -> inject_on_attempts=[1, 2]
    CLARIFY needs no injector: the TASKING itself carries a planted
    spec-vs-acceptance-test contradiction.
- Integrity guards: the engine refuses a set injector on synthetic=0 runs
  (lifecycle.py), and the injector additionally verifies it is operating in a
  scratch worktree, never the real fixture. Every injection is recorded in
  the injector's audit list, written to sortie metadata at close (no
  unregistered telemetry event types; `exercise.injected` is a candidate
  ENVELOPE.md amendment).
"""

import subprocess


class Mutation:
    """One scripted defect: exact-string replacement in one scoped file."""

    def __init__(self, path: str, old: str, new: str, commit_msg: str):
        self.path = path          # repo-relative, must be inside task file-scope
        self.old = old
        self.new = new
        self.commit_msg = commit_msg  # innocuous — seats fly blind


class MutationInjector:
    """Plants scripted defects on a synthetic task's branch, post-completion,
    pre-verification. One injector instance per synthetic task run."""

    def __init__(self, worktree: str, task_id: str,
                 mutations_by_attempt: dict, scratch_marker: str):
        """mutations_by_attempt: {attempt_number: Mutation}
        scratch_marker: substring that MUST appear in the worktree path —
        a physical guard that injection only ever happens in scratch."""
        self.worktree = worktree
        self.task_id = task_id
        self.mutations = mutations_by_attempt
        self.scratch_marker = scratch_marker
        self.audit = []  # -> sortie metadata at close

    def maybe_inject(self, spec, attempt: int):
        mut = self.mutations.get(attempt)
        if mut is None:
            return
        if self.scratch_marker not in self.worktree:
            raise RuntimeError(
                f"integrity: injector worktree {self.worktree!r} is not a "
                f"scratch checkout (marker {self.scratch_marker!r} absent) — "
                "refusing to mutate")
        if mut.path not in spec.files:
            raise RuntimeError(
                f"integrity: mutation target {mut.path!r} outside task "
                f"file-scope {spec.files}")
        full = f"{self.worktree}/{mut.path}"
        with open(full) as f:
            text = f.read()
        if mut.old not in text:
            raise RuntimeError(
                f"injection target string not found in {mut.path} "
                f"(attempt {attempt}) — exercise script is stale")
        with open(full, "w") as f:
            f.write(text.replace(mut.old, mut.new, 1))
        subprocess.run(["git", "-C", self.worktree, "add", mut.path],
                       check=True, capture_output=True)
        subprocess.run(
            ["git", "-C", self.worktree,
             "-c", "user.name=trireme-harness",
             "-c", "user.email=harness@trireme.local",
             "commit", "-m", mut.commit_msg],
            check=True, capture_output=True)
        self.audit.append({
            "task_id": self.task_id, "attempt": attempt, "path": mut.path,
            "mutation": f"{mut.old!r} -> {mut.new!r}",
            "commit_msg": mut.commit_msg,
        })
