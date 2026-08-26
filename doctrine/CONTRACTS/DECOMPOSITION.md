# Contract — Decomposition (Q4)

A task is **claimable** only when all four hold. The board's mechanical layer (3b)
refuses claims on tasks missing any of them.

## The four claimability requirements

1. **Acceptance criteria as tests-to-make-pass.** Concrete, runnable, attached to
   the task. Prose criteria are not criteria.
2. **Exclusive file-scope.** The task declares every file it may touch; **no two
   open tasks own the same file.** The board enforces this at posting time.
3. **Declared dependency edges.** If task B needs task A merged first, the edge is
   on the board; the board won't offer B while A is open.
4. **Budget caps.** Token cap and wall-time cap, both set at posting
   (`config/fleet.toml` defaults; Producer may tighten, never remove).

## Changing behavior means changing tests — spec it, don't ban it

When a task changes existing functionality, some existing tests *should* break.
That is spec work, not Worker discretion (redline round 1, PR #1):

- The **acceptance tests are Producer-authored and immutable within the task.**
  For a behavior change, the task's acceptance tests define the *new* behavior.
  Changing acceptance tests mid-task is forbidden for everyone below the
  Producer; the Producer changes them only by reissue.
- **Existing tests the change will break are part of the spec:** their files go
  in the task's declared `files[]`, and the spec names the expected change
  ("test_wrap_at_edge now asserts clamp"). In-scope, spec-named test edits are
  legal work; anything else is a CLARIFY.
- The board refuses nothing here mechanically beyond file-scope — the law is
  that a Worker editing an undeclared test, or any acceptance test, is a
  doctrine violation regardless of green.

## The direction law

**Anything unspeccable goes *up*, never down.** A requirement that can't be
expressed as tests-to-make-pass becomes a CLARIFY toward Command — it is never
handed downward as a vague task for a Worker to interpret.

## Attribution

Decomposition quality is the Producer's ledger:

- Budget blowouts charge the Producer (the estimate was wrong, not the work).
- Rebase conflicts charge the Producer (file-scope exclusivity failed).
- Post-rebase suite failures charge the Producer (semantic coupling missed);
  the Worker is **not** charged.

## Task fields (board schema, minimum)

`task_id · phase · spec · acceptance_tests · files[] · deps[] · budget_tokens ·
budget_wall_s · status · attempt · remediation_of`

`remediation_of` is null unless this task exists to fix the *approach* of an
already-merged task (the spec-concern path, `ROLES/VERIFIER.md`); then it names
that task. Remediation work is first-class and visible (`profiler/views.sql`
m13) — fix-forward is allowed, buried fix-forward is not.
