"""Mechanical merge gate (CONTRACTS/MERGE.md) — nobody's judgment.

Serial queue: rebase the task branch onto main, run the FULL suite post-rebase,
fast-forward main, return the commit. Rebase conflict and post-rebase suite
failure are Producer-charged decomposition failures (raised as typed errors so
the engine can emit the right event). Invariant: main is always green.

Task branches live in per-task worktrees; a checked-out branch can only be
rebased from inside its own worktree, so the gate runs the rebase and the
post-rebase suite there, then fast-forwards main in the primary checkout.
"""

import subprocess


class ScopeViolation(Exception):
    """Worker edited an undeclared path (MERGE.md gate 2). The diff is the
    evidence; the fix (reverting the undeclared edit) is the Worker's, so
    this re-enters the rejection ladder charged to the worker."""

    def __init__(self, files: list):
        self.files = files
        super().__init__("undeclared paths in merge-base diff: " + ", ".join(files))


class RebaseConflict(Exception):
    """Producer decomposition failure: two merged scopes weren't independent."""


class SuiteFailedPostRebase(Exception):
    """Producer decomposition failure: semantic coupling survived file-scope."""


class GitMergeGate:
    def __init__(self, repo: str, main_branch: str, suite_cmd: list):
        self.repo = repo
        self.main = main_branch
        self.suite_cmd = suite_cmd
        self.worktrees = {}  # task_id -> worktree path (set by the runner)

    def _git(self, cwd, *args, check=True):
        return subprocess.run(["git", "-C", cwd, *args],
                              capture_output=True, text=True, check=check)

    def merge(self, spec) -> str:
        branch = f"task/{spec.task_id}"
        wt = self.worktrees.get(spec.task_id, self.repo)

        # Gate 2 (MERGE.md): mechanical file-scope check, BEFORE the rebase.
        # Three-dot = diff from the merge-base — never two-dot against a main
        # that may have moved (Sortie 3 SYN-M2: that indicts the Worker for
        # main's own commits).
        diff = self._git(wt, "diff", "--name-only", f"{self.main}...HEAD")
        changed = [p for p in diff.stdout.splitlines() if p.strip()]
        undeclared = sorted(set(changed) - set(spec.files))
        if undeclared:
            raise ScopeViolation(undeclared)

        rebase = self._git(wt, "rebase", self.main, check=False)
        if rebase.returncode != 0:
            self._git(wt, "rebase", "--abort", check=False)
            raise RebaseConflict(f"{branch} onto {self.main}: {rebase.stderr[-800:]}")

        suite = subprocess.run(self.suite_cmd, cwd=wt,
                               capture_output=True, text=True)
        if suite.returncode != 0:
            raise SuiteFailedPostRebase(suite.stdout[-800:] + suite.stderr[-800:])

        self._git(self.repo, "checkout", self.main)
        self._git(self.repo, "merge", "--ff-only", branch)
        return self._git(self.repo, "rev-parse", "HEAD").stdout.strip()
