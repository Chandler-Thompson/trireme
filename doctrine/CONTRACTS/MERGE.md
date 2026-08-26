# Contract — Merge (Q7)

**Merge is mechanical — nobody's judgment.** No seat "decides" to merge; the board's
mechanical layer (3b) executes the gates in order.

## Gates (all mandatory, in order)

1. `VERDICT: PASS` on record for the task's current attempt.
2. **File-scope gate (mechanical).** Diff the branch **from the merge-base**
   (never two-dot against a base that may have moved — that indicts the
   Worker for the base's own commits); every changed path must be in the
   task's declared `files[]`. Any undeclared path → refuse, emit
   `task.scope_violation {files[], charged: "worker"}`, and the task
   re-enters the rejection ladder as a worker-charged rejection — the fix
   (reverting the undeclared edit) is the Worker's to make. Scope has been
   a wall since Q4; this gate makes the wall mechanical instead of
   verifier attention. Machinery MAY run the same check earlier (at task
   completion) to fail cheaper; the merge-time gate is the guarantee.
3. **Serial merge queue** — one merge at a time, completion order, so history stays
   bisectable.
4. Rebase onto current main.
5. **Full-suite re-run post-rebase.** Green → merge. Anything else → below.

## Failure attribution

- **Scope violation** = the Worker edited an undeclared path. Rejection-ladder
  rung, Charged: Worker (evidence = the diff itself, machinery-emitted).
- **Rebase conflict** = decomposition failure (exclusive file-scope was violated in
  spirit). Task → Producer. Charged: Producer.
- **Post-rebase suite failure** = semantic coupling the decomposition missed.
  Task → Producer. Charged: Producer. The Worker is not charged.

## Invariants

- **Main is always green.** A red main is a `FLASH`: the fleet stops, Command is
  interrupted, nothing merges until green is restored.
- **Trunk-based, not git-flow.** A `develop` branch is hidden shared state and
  integration debt — the "building on sand" failure mode for stateless workers.
  Release branches are cut from green main only when there are actual players
  (Phase 3+).
