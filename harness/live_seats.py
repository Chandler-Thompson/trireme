"""Live seat adapters — headless agent-CLI sessions speaking the engine's
action vocabulary.

Seat tool allowlists come from a `harness.project.ProjectProfile`, never from
constants here: this module knows what a Worker may do in general (read, edit,
stage, commit) and nothing about what any particular repository is built with.
Pass a profile to each seat; omit it and the seat gets the generic core only,
which is enough to read and commit but cannot run a build.

model.call telemetry IS emitted in this harness: the CLI's JSON result
envelope carries machinery-measured usage (tokens, cost, API latency), which
the emission law permits — the harness reads the provider's accounting, the
agent self-reports nothing. tool.call remains unmeasurable from outside the
session and stays absent (ENVELOPE.md degraded caveat, narrowed to tool.*).

Session contracts (the seat must end with one JSON object):
  worker:   {"outcome":"done","evidence":"..."} |
            {"outcome":"clarify","question":"..."}
  verifier: {"verdict":"pass","spec_concern":0|1} |
            {"verdict":"fail","evidence":"failing-test/repro"}
  producer: {"answer":"..."} |
            {"ruling":"reissue","reason":"..."} (no lawful deliverable)
Every session's raw envelope lands in the sortie's sessions/ directory.
"""

import os

from .claude_cli import run_headless
from .project import (BASE_PRODUCER_TOOLS, BASE_VERIFIER_TOOLS,
                      BASE_WORKER_TOOLS)


WORKER_SYSTEM = """You are a Trireme fleet WORKER seat. Non-negotiable laws:
- Touch ONLY the files in your task's file-scope. Nothing else, ever.
- Acceptance tests are immutable in-task: never edit a test the TASKING marked
  as acceptance criteria.
- If the spec is ambiguous or contradicts a test, STOP and file CLARIFY —
  the unspeccable goes up, never down.
- Verify with the given test command before claiming done; your evidence line
  must reference real output.
- Commit your work to the current branch (git add <scoped files> && git commit)
  before claiming done — an uncommitted worktree merges nothing.
- Stage each declared path EXPLICITLY. Never `git add -A`, `git add .`, or
  wildcards: any file outside your declared file-scope must stay uncommitted —
  committing one is a scope violation charged to you, and build-generated
  files are the usual way it happens by accident. (Sortie 4, T-402 attempt 1:
  exactly this.){generated_artifacts}
- Every turn ends with the JSON contract below, NO exceptions. If a tool call
  is permission-denied or you are otherwise blocked, do not explain in prose
  and stop — file {"outcome":"clarify","question":"<the blocked action and
  why you need it>"}. A turn without the JSON line is burned as a seat error.
End your final message with exactly one JSON object on its own line:
{"outcome":"done","evidence":"<what you ran and saw>"} or
{"outcome":"clarify","question":"<the precise contradiction>"}"""

VERIFIER_SYSTEM = """You are a Trireme fleet VERIFIER seat. Non-negotiable laws:
- You judge; you NEVER fix code.
- A FAIL must be evidence-backed: a failing test or a concrete repro, never
  opinion or taste.
- FILE-SCOPE IS A WALL: check every changed path against the task's declared
  file-scope. An out-of-scope edit is an evidence-backed FAIL by itself (name
  the file), regardless of green tests. (Sortie 3, T-300 attempt 2: the
  worker edited a shared helper module it had not declared.)
- DIFF FROM THE MERGE-BASE, never two-dot against the base branch: use
  `git diff {main_branch}...HEAD` (three-dot) or diff from `git merge-base`. The
  base branch may have moved since the task branch was cut; a two-dot diff
  shows the base's new files as branch "deletions" and indicts the worker
  for commits it never made. (Sortie 3, SYN-M2 attempt 1: exactly this
  false charge.) You judge the work as of its branch base; integrating with
  a moved base is the merge gate's job, not yours.
- Taste/approach concerns on work that meets the contract-as-written: PASS
  with spec_concern 1 (it will be tracked and remediated forward).
- Run the acceptance tests and the full suite yourself; read the diff.
- Run the test command EXACTLY as given. Compound shell forms (appending
  `; echo $?` etc.) are denied by the permission layer; the tool result
  already reports the exit code.
End your final message with exactly one JSON object on its own line:
{"verdict":"pass","spec_concern":0} or
{"verdict":"fail","evidence":"<failing test / repro>"}"""

PRODUCER_SYSTEM = """You are the Trireme fleet TECHNICAL PRODUCER answering a
CLARIFY. Two lawful rulings:
- If a decisive, minimal answer lets the worker proceed lawfully, give it.
- If the CLARIFY exposes a task with NO lawful deliverable — acceptance
  requirements that contradict each other, or required work outside the
  declared file-scope — do NOT invent an answer downward: withdraw the task
  for re-decomposition. Never tell the worker to edit an acceptance test or
  touch an out-of-scope file; those are doctrine violations, not answers.
- Picking a side cannot resolve a contradiction INSIDE an immutable
  acceptance test: the losing assertion still runs and still fails, and
  nobody below you may edit it. Verify the claim in the task worktree (your
  cwd), then rule reissue.
End your final message with exactly one JSON object on its own line:
{"answer":"<the ruling>"} or
{"ruling":"reissue","reason":"<why no lawful deliverable exists>"}"""


class _LiveSeat:
    #: generic core for this seat kind; a project profile adds to it, never
    #: replaces it. Subclasses set BASE_TOOLS and allowlist_for().
    BASE_TOOLS = []
    MAX_SESSION_S = 3600

    def __init__(self, seat: str, system: str, cwd: str, sessions_dir: str,
                 model=None, profile=None):
        self.seat = seat
        self.cwd = cwd
        self.sessions_dir = sessions_dir
        self.model = model
        self.profile = profile
        self.system = self._render(system)
        self.allowed_tools = self.allowlist_for(profile)
        self._turn = 0
        self._budget_meter = None

    def _render(self, system: str) -> str:
        """Fill the project-shaped slots in a seat system prompt. Prompts that
        contain no slots pass through untouched.

        Deliberately `str.replace`, not `str.format`: every seat prompt ends
        with a literal JSON contract, and `format` would read those braces as
        fields and raise."""
        p = self.profile
        note = (getattr(p, "generated_artifacts", "") or "") if p else ""
        branch = (getattr(p, "main_branch", "") or "") if p else ""
        return (system
                .replace("{main_branch}", branch or "main")
                .replace("{generated_artifacts}", ("\n  " + note) if note else ""))

    def allowlist_for(self, profile) -> list:
        return list(self.BASE_TOOLS)

    def set_budget_meter(self, meter):
        """Engine-attached (duck-typed, like record_rejection). The task's
        remaining wall budget becomes the subprocess timeout, so a hung
        session dies at the cap instead of running to the flat default."""
        self._budget_meter = meter

    def _run(self, prompt: str):
        self._turn += 1
        os.makedirs(self.sessions_dir, exist_ok=True)
        transcript = os.path.join(self.sessions_dir,
                                  f"{self.seat}-turn{self._turn}.json")
        timeout_s = self.MAX_SESSION_S
        if self._budget_meter is not None:
            timeout_s = max(1, min(timeout_s,
                                   int(self._budget_meter.wall_remaining_s())))
        return run_headless(prompt, self.system, self.cwd, transcript,
                            model=self.model, timeout_s=timeout_s,
                            allowed_tools=self.allowed_tools)


class LiveWorker(_LiveSeat):
    BASE_TOOLS = BASE_WORKER_TOOLS

    def allowlist_for(self, profile):
        return profile.worker_allowlist() if profile else list(self.BASE_TOOLS)

    def __init__(self, cwd, tasking, test_cmd, sessions_dir, model=None,
                 profile=None):
        super().__init__("worker", WORKER_SYSTEM, cwd, sessions_dir, model,
                         profile)
        self.tasking = tasking
        self.test_cmd = test_cmd
        self._clarify_context = []
        self._rejection_context = []

    def next_actions(self):
        prompt = f"TASKING:\n{self.tasking}\n\nTest command: {self.test_cmd}\n"
        if self._clarify_context:
            prompt += "\nCLARIFY answers so far:\n" + "\n".join(self._clarify_context)
        if self._rejection_context:
            prompt += ("\nYour previous attempt on this branch was REJECTED "
                       "with this evidence — diagnose and fix before claiming "
                       "done again:\n" + "\n".join(self._rejection_context))
        result, model_call = self._run(prompt)
        yield ("model", model_call)
        if result.get("outcome") == "done":
            yield ("done", result.get("evidence", ""))
        elif result.get("outcome") == "clarify":
            yield ("clarify", result.get("question", "unspecified"))
        else:
            raise RuntimeError(f"worker returned unusable outcome: {result}")

    def record_answer(self, answer: str):
        self._clarify_context.append(answer)

    def record_rejection(self, evidence: str):
        self._rejection_context.append(evidence)


class LiveVerifier(_LiveSeat):
    # judges, never fixes: no Edit/Write, no commit
    BASE_TOOLS = BASE_VERIFIER_TOOLS

    def allowlist_for(self, profile):
        return profile.verifier_allowlist() if profile else list(self.BASE_TOOLS)

    def __init__(self, cwd, tasking, test_cmd, sessions_dir, model=None,
                 profile=None):
        super().__init__("verifier", VERIFIER_SYSTEM, cwd, sessions_dir, model,
                         profile)
        self.tasking = tasking
        self.test_cmd = test_cmd

    def next_actions(self):
        prompt = (f"Verify the claimed-done work on this branch against:\n"
                  f"{self.tasking}\n\nTest command: {self.test_cmd}\n"
                  f"Diff the branch against "
                  f"{getattr(self.profile, 'main_branch', None) or 'main'}; "
                  f"run the tests.")
        result, model_call = self._run(prompt)
        yield ("model", model_call)
        if result.get("verdict") == "pass":
            yield ("pass", int(result.get("spec_concern", 0)))
        elif result.get("verdict") == "fail":
            yield ("fail", result.get("evidence", "unspecified"))
        else:
            raise RuntimeError(f"verifier returned unusable verdict: {result}")


DECOMPOSER_SYSTEM = """You are the Trireme fleet TECHNICAL PRODUCER, opening a
sortie. Your job right now is decomposition: read the sortie brief and the
fixture repository, then propose a set of claimable tasks. Non-negotiable laws
(CONTRACTS/DECOMPOSITION.md):
- A task is claimable only when it has all four: (1) acceptance criteria as
  concrete tests-to-make-pass — prose criteria are not criteria; (2) an
  EXCLUSIVE file-scope, the exact files it may touch, with NO overlap against
  any other task in your own list; (3) declared dependency edges naming other
  task_ids in your own list, wherever one task's work depends on another's
  landing first; (4) a token budget and a wall-time budget — omit both to
  accept the fleet default, or state your own only to make them TIGHTER, never
  looser.
- Real software understanding is required: read the actual code your tasks
  touch. Do not guess at file names or invent functions that do not exist.
- The unspeccable goes up, never down: if part of the objective cannot be
  expressed as tests-to-make-pass with what you can determine from the repo,
  leave it OUT of this decomposition and say why in your rationale. Never
  hand a Worker a vague task to interpret.
- You decide how many tasks to post. There is no target count — too few
  (one unclaimable monolith) and too many (needless churn) are both your call
  to avoid, not a number handed down to you.
- You do not write or fix production code in this phase.
End your final message with exactly one JSON object on its own line, in this
shape:
{"tasks": [{"task_id": "T-6xx", "title": "...", "description": "...",
            "acceptance_criteria": "...", "files": ["..."], "deps": [],
            "budget_tokens": null, "budget_wall_s": null}, ...],
 "rationale": "<1-4 sentences on scope and sequencing choices>"}
Assign your own ascending task_ids (e.g. T-600, T-601, ...); gaps are fine.
An empty "tasks" list is lawful if the objective is genuinely unspeccable
from the current repo state — say why in rationale."""


class LiveProducerDecompose(_LiveSeat):
    """The decomposition half of the Producer seat (Phase 2) — distinct from
    LiveProducer below, which only ever answers CLARIFYs. Read-only: no
    Edit/Write/Bash, so it is safe to point directly at the fixture repo
    without a scratch clone. Driven by harness/decompose.py, which owns the
    propose -> post -> bounce-on-conflict loop and the phase-level budget."""

    BASE_TOOLS = BASE_PRODUCER_TOOLS

    def __init__(self, cwd, sortie_brief, sessions_dir, model=None,
                 profile=None):
        super().__init__("producer", DECOMPOSER_SYSTEM, cwd, sessions_dir,
                         model, profile)
        self.brief = sortie_brief

    def propose(self, addendum: str | None = None):
        """One decomposition turn. Returns (seat_json, model_call_payload) —
        the raw pair, so the caller can feed its own phase-level budget meter
        (this is not a per-task session, so it does not go through the
        engine's per-task run_task/meter plumbing)."""
        prompt = f"Sortie brief:\n{self.brief}"
        if addendum:
            prompt += "\n\n" + addendum
        return self._run(prompt)


class LiveProducer(_LiveSeat):
    BASE_TOOLS = BASE_PRODUCER_TOOLS

    def __init__(self, cwd, sortie_brief, sessions_dir, model=None,
                 profile=None):
        super().__init__("producer", PRODUCER_SYSTEM, cwd, sessions_dir,
                         model, profile)
        self.brief = sortie_brief
        self.on_answer = None
        self._question = None

    def next_actions(self):
        prompt = f"Sortie brief:\n{self.brief}\n\nWorker CLARIFY:\n{self._question}"
        result, model_call = self._run(prompt)
        yield ("model", model_call)
        if result.get("ruling") == "reissue":
            yield ("reissue", (result.get("reason") or "unspecified").strip())
            return
        answer = (result.get("answer") or "").strip()
        if not answer:
            raise RuntimeError(f"producer returned no answer: {result}")
        if self.on_answer:
            self.on_answer(answer)
        yield ("answer", answer)

    def set_question(self, q: str):
        self._question = q
