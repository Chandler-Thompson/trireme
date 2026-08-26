# COMMS v0 — Fleet Communication Specification

> Layer 2c. Naval message-center discipline, token-efficient. Five types, one header,
> two stores, no exceptions.

## The five message types

There are exactly five message types. A communication need that fits none of them is
a doctrine gap: file it upward as CLARIFY, never invent a sixth type in the field.

| Type | Sender → Receiver | Purpose |
|---|---|---|
| `TASKING` | Producer → Worker/Verifier | Issue or amend a task. Body points at the board task (spec, acceptance tests, file-scope, budget). Also used to answer a CLARIFY. |
| `SITREP` | Any seat → board | Status report. The only type that carries precedence. `DONE` SITREPs must attach evidence (test-run output ref). |
| `CLARIFY` | Downward seat → upward seat | The unspeccable goes up, never down. Worker → Producer; Producer → Command. A CLARIFY blocks the asking task until answered. |
| `VERDICT` | Verifier → board | `PASS` or `FAIL`. FAIL must be evidence-backed (failing test or repro), never opinion. A PASS may carry one `SPEC-CONCERN:` line (wrong approach, green tests — see `ROLES/VERIFIER.md`); 3b lifts it into `verdict.pass` telemetry. |
| `ACK` | Any seat | Bare receipt/claim acknowledgment. No body beyond the header. |

## Header format

Every message opens with one fixed header line:

```
TYPE | FROM | RE: task-id | PREC | body
```

- `TYPE` — one of the five, uppercase.
- `FROM` — seat id (see `CONTRACTS/ENVELOPE.md` seat vocabulary), not a model name.
- `RE: task-id` — the board task id; `RE: fleet` only for sortie-scoped SITREPs.
- `PREC` — precedence. **SITREP only**; all other types carry `-`.
- `body` — the message. Token-frugal: state facts, point at evidence, no pleasantries.

## Precedence (SITREP only)

| PREC | Meaning | Handling |
|---|---|---|
| `ROUTINE` | Normal traffic | Read at next natural poll. |
| `PRIORITY` | Needs attention this sortie | Producer handles before issuing new TASKING. |
| `FLASH` | Interrupts a human | Red main, runaway spend, reconciliation daylight, safety. Fleet stops until Command responds. |

FLASH is expensive by design. A false FLASH is a ruling-worthy event; a suppressed
FLASH is worse.

## Channels — exactly two

1. **Task-scoped messages live as comments on the board task** (state layer 5a/5b).
2. **Fleet-wide messages append to `FLEET-LOG.jsonl`** (append-only, layer 5b).

No third channel. No broadcasts. **No worker-to-worker communication, ever** — if two
workers need to coordinate, the decomposition failed; that is Producer work.

## Style law

- Phonetic-alphabet-style redundancy is *not* wanted — the naval inspiration is
  message discipline (typed traffic, precedence, evidence), not verbosity.
- One message, one purpose. Do not stack a CLARIFY inside a SITREP.
- Every message is also a `msg.sent` telemetry event (machinery-emitted; see
  `CONTRACTS/ENVELOPE.md`).
