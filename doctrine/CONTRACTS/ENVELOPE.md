# Contract — Event Envelope & Type Registry (Q11a)

The profiler's single event stream. **One envelope, typed payloads.** Definitions
here are law; the machinery that ingests, reports, and glances lives outside
doctrine and must conform (see `CONFORMANCE.md`).

## Envelope (every event, no exceptions)

| Field | Type | Notes |
|---|---|---|
| `ts` | ISO-8601 UTC string | Emission time |
| `sortie_id` | string | Assigned at sortie open |
| `doctrine_hash` | string | The pinned doctrine commit — stamped on **every** event |
| `seat` | string | Seat vocabulary below |
| `task_id` | string \| null | null for sortie-scoped events |
| `type` | string | Registry below |
| `synthetic` | 0 \| 1 | 1 = injected failure exercise (Q8); excluded from metrics by default |
| `payload` | JSON object | Typed per event type |

## Storage

- Live: **JSONL, one file per sortie** (layer 5b).
- At sortie close: ingested into a **SQLite read-model** (separate file from the
  board DB). `profiler/views.sql` runs against that read-model.
- **Degraded mode** (no custom harness): same schema; `model.*` / `tool.*` events
  simply absent, their views empty. Two altitudes are a query-time distinction,
  never storage-time.

## Emission law

**Machinery-only emission. No agent self-reporting, ever.** The telemetry emitter
(6d) and board automation (3b) are the only writers. An agent claiming its own
token count is not data.

## Seat vocabulary

`principal` · `xo` · `producer` · `worker-<n>` · `verifier-<n>` ·
`board` (3b mechanical) · `runner` (3c process)

## Event type registry

### `task.*` (emitted by 3b, board automation)

| Type | Payload | Emitted when |
|---|---|---|
| `task.created` | `{phase, files[], deps[], budget_tokens, budget_wall_s, remediation_of}` | Producer posts a claimable task. `remediation_of`: task_id of the merged task whose approach this fixes (spec-concern path), else null — **always present**, never omitted |
| `task.claimed` | `{}` | Atomic claim succeeds |
| `task.clarify` | `{question}` | CLARIFY filed against the task |
| `task.clarify_resolved` | `{}` | Answering TASKING posted |
| `task.completed` | `{attempt}` | DONE SITREP with evidence accepted |
| `task.rejected` | `{attempt, charged}` | FAIL verdict lands; `charged` ∈ `worker` \| `producer` per the ladder |
| `task.reissued` | `{}` | Producer re-decomposes after hard stop |
| `task.merged` | `{commit}` | Merge gates all passed |
| `task.scope_violation` | `{files[], charged: "worker"}` | Merge gate (or earlier machinery check) finds changed paths outside the task's declared file-scope; diff taken from the merge-base. Re-enters the rejection ladder |
| `task.merge_conflict` | `{charged: "producer"}` | Rebase conflict in the queue |
| `task.suite_failed_post_rebase` | `{charged: "producer"}` | Full-suite re-run fails after rebase |

### `msg.*` (emitted by 3b on every board/log message)

| Type | Payload |
|---|---|
| `msg.sent` | `{msg_type: TASKING\|SITREP\|CLARIFY\|VERDICT\|ACK, prec: ROUTINE\|PRIORITY\|FLASH\|"-"}` |

### `verdict.*` (emitted by 3b when a VERDICT lands)

| Type | Payload |
|---|---|
| `verdict.pass` | `{attempt, spec_concern}` — `spec_concern` ∈ 0 \| 1, always present; 1 when the VERDICT body carries a `SPEC-CONCERN:` line (3b lifts it — wrong approach merged against contract-as-written, `ROLES/VERIFIER.md`) |
| `verdict.fail` | `{attempt, evidence}` — `evidence` = failing-test/repro reference, required |

### `model.*` / `tool.*` (emitted by 6d, harness telemetry — the innermost tap point; all spend bottoms out here)

| Type | Payload |
|---|---|
| `model.call` | `{model, provider, tokens_in, tokens_in_fresh, tokens_out, cost_usd, latency_ms}` — `tokens_in_fresh` = uncached input + cache-creation (cache READS excluded; a session re-reads its cached context every internal turn, so raw `tokens_in` measures turn count, not context). The budget-cap counter reads `tokens_in_fresh + tokens_out`, so every cap decision is reconstructible from ledger arithmetic alone |
| `tool.call` | `{tool, ok, latency_ms}` |

### Sortie / command / governance

| Type | Payload | Emitter |
|---|---|---|
| `sortie.open` | `{phase}` | runner |
| `sortie.close` | `{}` | runner |
| `budget.cap_hit` | `{kind: tokens\|session_tokens\|wall, charged: "producer"}` — `tokens` = task-cumulative fresh+out over budget; `session_tokens` = a single session over the per-session dumb-zone cap (checked first when one call trips both); `wall` = wall-time cap | 3b or 6d |
| `seat.error` | `{seat, reason}` | 3c/6d, when a seat session dies on INFRASTRUCTURE (provider api_error, unparsable envelope, nonzero CLI exit) and the task is reissued **without charge** — infra failure is nobody's work quality, and leaving it unlabeled blurs m03 attribution |
| `human.touch` | `{reason}` | 3b, whenever a human acts mid-sortie |
| `ruling.filed` | `{ruling_id, amendment: true\|false}` | 3b at sortie close |

Adding an event type is a doctrine amendment (`CHANGELOG.md`); renaming or changing
a payload field is a **`[yardstick]`** amendment.
