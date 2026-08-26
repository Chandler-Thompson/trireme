# Flight 03 — CLARIFY Escalation

The unspeccable goes up: Worker hits a spec/test contradiction, files CLARIFY,
task blocks, Producer answers via TASKING, clean finish. Exercises the
direction law and the CLARIFY block/unblock plumbing.

## Stub values

Same stubs as flight-01, plus:

| Stub | Value |
|---|---|
| producer `model.call` | `{"cost_usd":0.03,"latency_ms":1800,"model":"stub-producer","provider":"stub","tokens_in":3000,"tokens_out":800}` |
| CLARIFY question | `"Spec says wrap-around at grid edge; acceptance test asserts clamp. Which is law?"` |

## Action script

1. Runner opens sortie (`phase-1`).
2. Producer posts `T-003` (same shape as flight-01's task) → TASKING.
3. Worker-1 claims → ACK; one model call; hits the contradiction.
4. Worker-1 files `task.clarify` with the question → CLARIFY message. Task blocks.
5. Producer: one model call → `task.clarify_resolved` → answering TASKING.
6. Worker-1 resumes: model call + tool call; DONE attempt 1 → SITREP ROUTINE.
7. Verifier-1 model call → `verdict.pass` → VERDICT.
8. Board merges (`<COMMIT-1>`).
9. Runner closes sortie.

## Pass condition

Normalized ledger byte-identical to `expected-ledger.jsonl`.
