# Flight 02 — Rejection Ladder (both rungs)

One task, two evidence-backed FAILs: rung 1 charges the Worker (retry, same
Worker), rung 2 charges the Producer (hard stop, reissue). Exercises
`CONTRACTS/REJECTION.md` end to end.

## Stub values

Same stubs as flight-01, plus:

| Stub | Value |
|---|---|
| verdict evidence (both FAILs) | `"tests/test_foo.py::test_bar FAILED"` |

## Action script

1. Runner opens sortie (`phase-1`).
2. Producer posts `T-002` (same shape as flight-01's task) → TASKING.
3. Worker-1 claims → ACK; model call + tool call; DONE attempt 1 → SITREP ROUTINE.
4. Verifier-1 model call → `verdict.fail` attempt 1 with evidence → VERDICT.
5. Board emits `task.rejected` attempt 1, **charged: worker**.
6. Worker-1 retries (`max_worker_retries` = 1): model call + tool call; DONE
   attempt 2 → SITREP ROUTINE.
7. Verifier-1 model call → `verdict.fail` attempt 2 with evidence → VERDICT.
8. Board emits `task.rejected` attempt 2, **charged: producer** — hard stop.
9. Producer emits `task.reissued`.
10. Runner closes sortie.

## Pass condition

Normalized ledger byte-identical to `expected-ledger.jsonl`. The two `charged`
values are the heart of this flight — an emitter that misattributes rungs fails
certification even if everything else matches.
