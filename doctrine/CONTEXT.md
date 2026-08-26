# Trireme Fleet

The orchestration contracts for a Trireme fleet: how a sortie decomposes work, how seats
take and verify it, and how failures are attributed. This file is the fleet's glossary and
nothing else — no specs, no implementation notes. The contracts themselves live in `CONTRACTS/`.

This is the one copy of the vocabulary. Any harness, tool or report that speaks about the
fleet uses these words with these meanings.

Created lazily and still partial. Only terms that have actually been ruled on appear below.

## Language

**Claim**:
An exclusive take of a task that the board enforces, so a second taker is refused. Gated by the
four claimability requirements in `CONTRACTS/DECOMPOSITION.md` and emitted as `task.claimed`.
_Avoid_: dibs, reserve, assign. Never use "claim" for an advisory hold that nothing refuses —
that is **dibs**, and it belongs to a different system.

**Dibs**:
Not used here. A declaration of intent that gates nothing and refuses nobody, honoured by
convention rather than machinery. Listed so the distinction stays visible: if a mechanism in
this fleet ever stops refusing a second taker, it is dibs and must be renamed.
_Avoid_: calling it a claim, a soft mutex, or an advisory lock.

**Rejection rung**:
One of the two steps of the rejection ladder. Rung 1 charges the Worker and retries; rung 2
charges the Producer and hard-stops.
_Avoid_: bare "rung". Unqualified, that word means a level on the autonomy ladder, which belongs
to a different system. Always write "rejection rung" here, even where the surrounding document
is obviously about rejection.

**Rejection ladder**:
The failure-attribution escalation in `CONTRACTS/REJECTION.md`: rejection #1 charges the Worker
and retries, rejection #2 charges the Producer and hard-stops.
_Avoid_: bare "the ladder". A second, unrelated ladder exists elsewhere in the stack (the
autonomy ladder, WATCH → DRAFT → ACT → JUDGE), so both are always written qualified.
