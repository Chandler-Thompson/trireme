# Contract — Rejection Ladder (Q5)

## The ladder

| Rung | Event | Consequence | Charged |
|---|---|---|---|
| **Rejection #1** | First evidence-backed FAIL on a task | Same Worker retries, up to `max_worker_retries` (`config/fleet.toml`) | Worker |
| **Rejection #2** | Second FAIL (or retry count exhausted) | **Hard stop.** Task returns to Producer: re-decompose, reissue, or CLARIFY upward | Producer |

## Laws

- **Rejections must be evidence-backed** — a failing test or a reproduction, never
  opinion (see `ROLES/VERIFIER.md`). A mechanical `task.scope_violation`
  (`CONTRACTS/MERGE.md` gate 2) is a rejection rung with the diff itself as
  evidence — machinery-emitted, worker-charged.
- **No appeals channel.** A Worker does not argue a VERDICT; a Producer does not
  overturn one. Disagreement with the law goes into a ruling at sortie close.
- **Verifiers never fix code.**
- Retry counts are configuration, not constants (`config/fleet.toml`).
- Every rung emits attributed telemetry (`task.rejected` with `charged`); the
  attribution split is a first-class profiler dimension.
