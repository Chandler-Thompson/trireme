"""Project profile — everything a fleet needs to know about the repository it
is flying against, in one place.

Why this exists: the seat tool allowlists in `live_seats.py` used to name a
specific project's build tooling directly. That made the harness describe one
repository rather than any repository. The allowlists here are the generic
core — read, edit, git — and each project adds only what its own build needs.

A profile is data, not code. Build one in your sortie script:

    from harness.project import ProjectProfile

    PROJECT = ProjectProfile(
        repo="/path/to/fixture",
        main_branch="main",
        test_cmd="python3 -m pytest -q",
        worker_tools=["Bash(python3 -m pytest:*)"],
        verifier_tools=["Bash(python3 -m pytest:*)"],
    )

`test_cmd` is what the seats are told to run. It is NOT automatically permitted
— name its permission form explicitly in `worker_tools`/`verifier_tools`, because
a permission entry and a shell command are different grammars and guessing one
from the other is how a seat ends up either over-permitted or silently blocked.

`generated_artifacts` is prose spliced into the Worker's system prompt. Use it
when the project's build writes files the worker did not author (sidecars, lock
files, generated sources). The staging law is absolute either way — the worker
stages declared paths explicitly and never `git add -A` — but the worker cannot
anticipate an artifact it has never been told about.
"""

from dataclasses import dataclass, field


# Tools every seat gets regardless of project. Read/search plus the git verbs
# the doctrine's staging law requires. Deliberately excludes any build, test,
# package or language-specific command.
BASE_WORKER_TOOLS = [
    "Read", "Edit", "Write", "Grep", "Glob",
    "Bash(git add:*)", "Bash(git commit:*)",
    "Bash(git rm:*)", "Bash(git restore:*)",
    "Bash(git status)", "Bash(git diff:*)", "Bash(git log:*)",
]

# The Verifier judges and never fixes: no Edit, no Write, no commit.
BASE_VERIFIER_TOOLS = [
    "Read", "Grep", "Glob",
    "Bash(git status)", "Bash(git diff:*)", "Bash(git log:*)",
]

# The Producer only ever reads.
BASE_PRODUCER_TOOLS = ["Read", "Grep", "Glob"]


@dataclass
class ProjectProfile:
    """One repository's flight envelope."""

    repo: str
    main_branch: str = "main"
    test_cmd: str = ""
    #: permission entries the Worker needs beyond BASE_WORKER_TOOLS
    worker_tools: list = field(default_factory=list)
    #: permission entries the Verifier needs beyond BASE_VERIFIER_TOOLS
    verifier_tools: list = field(default_factory=list)
    #: prose for the Worker prompt about build-generated files, or ""
    generated_artifacts: str = ""

    def worker_allowlist(self) -> list:
        return BASE_WORKER_TOOLS + list(self.worker_tools)

    def verifier_allowlist(self) -> list:
        return BASE_VERIFIER_TOOLS + list(self.verifier_tools)

    def producer_allowlist(self) -> list:
        return list(BASE_PRODUCER_TOOLS)
