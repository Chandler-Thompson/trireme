# Flight 01 — Happy Path

Full clean lifecycle, one task, zero rejections, zero human touches.

## Stub values (known answers — the harness's stubbed layers MUST return exactly these)

| Stub | Value |
|---|---|
| worker `model.call` | `{"cost_usd":0.01,"latency_ms":1200,"model":"stub-worker","provider":"stub","tokens_in":1000,"tokens_out":500}` |
| verifier `model.call` | `{"cost_usd":0.02,"latency_ms":1500,"model":"stub-verifier","provider":"stub","tokens_in":2000,"tokens_out":300}` |
| `tool.call` (test runner) | `{"latency_ms":300,"ok":true,"tool":"test_runner"}` |

## Action script

1. Runner opens sortie (`phase-1`).
2. Producer posts task `T-001`: files `["src/foo.py","tests/test_foo.py"]`, no deps,
   budgets 300000 tokens / 3600 s, not a remediation (`remediation_of` null) → TASKING.
3. Worker-1 claims → ACK.
4. Worker-1: one stubbed model call, one stubbed tool call (tests green).
5. Worker-1 files DONE (attempt 1) → SITREP ROUTINE.
6. Verifier-1: one stubbed model call → `verdict.pass` → VERDICT.
7. Board merges (`<COMMIT-1>`).
8. Runner closes sortie.

## Pass condition

Emitted ledger, normalized per `CONTRACTS/CONFORMANCE.md`, is **byte-identical** to
`expected-ledger.jsonl`.
