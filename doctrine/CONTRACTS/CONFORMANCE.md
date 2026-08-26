# Contract — Telemetry Conformance Suite (Q11e)

**A harness type is certified for sorties only after producing the exact expected
ledger from the scripted known-answer flights in `conformance/`.** No certification,
no sorties — degraded mode included.

## Why

Instrument integrity. The profiler is only law if every harness bookkeeps
identically. The portability receipt is: **same doctrine, same scripted flight,
same ledger — on two different harness types.**

## How a flight runs

1. The candidate harness runs with **stubbed model and tool layers** — every
   model/tool call returns the scripted known answer from the flight's `FLIGHT.md`.
   (This certifies the emitter and plumbing, not the model.)
2. The harness executes the flight's action script end to end.
3. The emitted sortie JSONL is **normalized** (below) and compared **byte-exact**
   against `expected-ledger.jsonl`.
4. Any diff = not certified. There is no "close enough."

## The control model

The stub layer **is** the control model (raised in redline round 1: "the model
could act off-script" → it can't, by construction):

- During certification there is **no live model anywhere in the loop.** Every
  seat's "model" is a deterministic scripted stand-in that returns the flight's
  known answers verbatim, in order. It cannot improvise, so it cannot go
  off-script — off-script behavior has no source.
- Therefore **every divergence in the emitted ledger indicts the harness**, the
  thing under test. There is no "the model did something unexpected" excuse and
  no false-negative path through model nondeterminism.
- A stub that runs out of scripted answers, or is asked something the script
  doesn't cover, must **fail the flight loudly** — that means the harness drove
  the flight differently than the action script, which is itself a conformance
  failure, not a skip.
- What conformance certifies is the **emitter and plumbing only.** Live-model
  behavior is never certified here — that's what synthetic exercises in real
  sorties (Q8, `synthetic: 1`) and the profiler are for.

## Normalization rules

Applied to the emitted ledger before comparison:

1. Each distinct `ts`, in order of first appearance → `<T0>`, `<T1>`, …
2. `sortie_id` → `<SORTIE>`
3. `doctrine_hash` → `<HASH>`
4. Each distinct commit hash in payloads, in order of first appearance → `<COMMIT-1>`, …
5. JSON canonicalized: keys sorted alphabetically at every level, compact
   separators (`,` and `:`), UTF-8, one event per line, `\n` line endings.

Expected ledgers in `conformance/` are stored already-normalized and canonical.

## The flights

| Flight | Exercises |
|---|---|
| `flight-01-happy-path` | Full clean lifecycle: create → claim → work → DONE → PASS → merge |
| `flight-02-rejection-ladder` | Both rungs: FAIL #1 (worker charged, retry) → FAIL #2 (producer charged, hard stop, reissue) |
| `flight-03-clarify` | Worker CLARIFY up, Producer answers, then clean finish |

Every failure path exercised in real sorties (Q8's ≥3× rule) uses `synthetic: 1`
tagging; conformance flights are pre-sortie certification and never enter sortie
telemetry at all.

Adding a flight is an amendment; changing an expected ledger is a **`[yardstick]`**
amendment.
