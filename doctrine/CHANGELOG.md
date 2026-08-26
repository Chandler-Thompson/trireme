# Doctrine Changelog (layer 2e)

> Amendments **with the evidence that earned them**. An amendment without cited
> evidence (profiler metrics, ruling ids, sortie reports) is not valid law.
> Changes that alter what is measured or how (event types, payload fields,
> metric views, expected conformance ledgers) are flagged **`[yardstick]`**.

## Conventions

- One entry per amendment, newest first.
- Format: date · scope · what changed · **evidence** (ruling ids / metric deltas /
  sortie report) · `[yardstick]` flag if applicable.
- Rulings (`RULINGS/`) are case law; recurring rulings harden into entries here.

---

## Extraction to Trireme — 2026-08-26 — public release `[yardstick]`

The doctrine and its degraded-mode harness were lifted out of the private
repositories they were developed in and published as `Trireme`. No rule
changed. The entries below are the law as it was flown; this entry records
what the move itself touched.

- `CONTRACTS/conformance/*/expected-ledger.jsonl` + `flight-01/FLIGHT.md` +
  `flight-02/FLIGHT.md` — the flights' placeholder paths carried the file
  extension of the original private fixture. Renamed to `src/foo.py` /
  `tests/test_foo.py`. Semantically inert, but it is an expected-ledger edit,
  so it is flagged and re-certified rather than slipped in. `[yardstick]`
- `ROLES/COMMAND.md` — the two Command seats were named as
  "Principal (<person>)" and "XO (<agent>)". The parentheticals are gone. The
  seats are roles; who holds them is not doctrine.
- Attribution throughout now names the **Principal** and the **XO seat**
  rather than the individuals who held them. The evidence each amendment
  cites is unchanged.
- **Harness, not law:** seat tool allowlists moved out of `live_seats.py` and
  into a `ProjectProfile` (`harness/project.py`). They had accumulated
  entries naming the original fixture's build tooling, which made the harness
  describe one repository rather than any repository. The allowlists in the
  harness are now the generic core — read, edit, git — and each project adds
  its own build commands. This changes no doctrine and no measured quantity.
- **Evidence:** the byte-exact conformance flights and the full adversarial
  suite both re-certified against this repository's own doctrine pin after
  the move. Nothing else in the law was touched.

## Mechanical scope gate + seat.error — 2026-07-13 — Sortie 3 amendments `[yardstick]`

Both earned by Sortie 3 incidents (see the Sortie 3 report).

- `CONTRACTS/MERGE.md` — new **gate 2: mechanical file-scope gate**. Diff from
  the MERGE-BASE (never two-dot against a moved base — SYN-M2 attempt 1's
  false charge); undeclared path → `task.scope_violation {files[],
  charged: "worker"}`, re-enters the rejection ladder. Scope was law-a-wall
  since Q4 but enforcement was verifier attention: T-300 attempt 2's
  out-of-scope edit to an undeclared shared helper was caught only because the verifier
  happened to read the diff.
- `CONTRACTS/ENVELOPE.md` — `task.scope_violation` (task.*) and `seat.error
  {seat, reason}` (governance) registered. seat.error labels INFRA session
  deaths (flight 2: one provider api_error was sortie-fatal and its reissue
  would have been indistinguishable from a producer-charged one).
  `[yardstick]`
- `CONTRACTS/REJECTION.md` — scope violation named as a rejection rung
  (evidence = the diff itself, machinery-emitted).
- `profiler/views.sql` — m03 includes `task.scope_violation`; new
  **m14_seat_errors** keeps infra failures OUT of the blame views by design
  (rising m14 + flat m03 = infrastructure problem). 19 views verified on
  box SQLite 3.37.2. `[yardstick]`
- **Evidence:** Sortie 3 (3 flights, pins in report): T-300 attempt-2 scope
  breach; SYN-M2 two-dot false charge; flight-2 api_error sortie kill.

## Budget-cap enforcement — 2026-07-13 — counter semantics + payload field `[yardstick]`

First amendment after law (v0 merged 2026-07-12, pin `4afc0be`). Closes the
standing "enforcement unbuilt" gap (Sortie 1 + 2 reports).

- `CONTRACTS/ENVELOPE.md` — `model.call` payload gains **`tokens_in_fresh`**
  (uncached input + cache-creation; cache reads excluded). Additive; needed so
  cap decisions are reconstructible from the ledger, not trusted to machinery.
- `config/fleet.toml` — token-cap **counter semantics defined**: fresh+out,
  cumulative across all seat sessions charged to the task (retries included).
  `default_task_budget_tokens` 100k → **300k**: the 100k was a per-context
  "dumb zone" judgment; task-cumulative it would have capped 5 of 7 healthy
  merged real tasks. Per-context intuition unchanged.
- `config/fleet.toml` — **`default_session_budget_tokens = 150_000`**
  (the Principal's PR #4 redline, 2026-07-13): every single session stays under
  the dumb-zone threshold, independent of the task-cumulative budget.
  `budget.cap_hit` kind enum gains `session_tokens` (checked before the
  cumulative kind when one call trips both). `[yardstick]`
- `CONTRACTS/conformance/*/expected-ledger.jsonl` + `flight-01/FLIGHT.md` —
  regenerated for the 300k default (same path as redline round 1's 200k→100k).
  `[yardstick]`
- **Evidence:** Sortie 1 + 2 session envelopes (16 tasks, 50 sessions): raw
  tokens_in 1.5M–3.8M per healthy task vs fresh+out 62k–259k; healthy merged
  66k–157k; worst retry case 259k; max single session 105k (all 50 under the
  150k session cap); wall 107–641s vs 3600s cap. Harness: budget meter +
  hard wall timeout, 7 new adversarial checks ALL HOLD, conformance flights
  01–03 re-certified byte-identical against pin `f1a2278`.

## v0 redline round 1 — 2026-07-11 — the Principal's PR #1 dispositions `[yardstick]`

Still pre-law (same unmerged PR); changes below are redline iteration, not
amendments-after-law. **Evidence:** PR #1 review threads, 2026-07-11.

- `config/fleet.toml` — `default_task_budget_tokens` 200k → **100k** (the observed
  "dumb zone" context threshold). **`max_open_tasks` removed** (file-scope
  exclusivity + branch isolation self-limit; reintroduction trigger documented
  in-file). `max_concurrent_workers` rationale documented in-file.
- `CONTRACTS/DECOMPOSITION.md` + `ROLES/WORKER.md` — the test-edit law: acceptance
  tests immutable within a task; existing tests a behavior change breaks are
  declared in file-scope with the change named in the spec; undeclared = CLARIFY.
  (The Principal's open question: "solution for when tests need to be edited".)
- `ROLES/VERIFIER.md` + `ROLES/PRODUCER.md` + `COMMS.md` — spec-concern path
  ratified (taste stays out of the verdict channel); concerns become structured
  `SPEC-CONCERN:` lines lifted into telemetry; remediation tasks carry
  `remediation_of`. (The Principal's condition: fixes "clearly tracked in reporting".)
- `CONTRACTS/ENVELOPE.md` — `task.created` payload gains `remediation_of`
  (always present, null default); `verdict.pass` payload gains `spec_concern`.
  `[yardstick]`
- `profiler/views.sql` — m01 renamed `m01_cycle_time_by_stage` (ambiguity he
  flagged), new `m01b_cycle_time_by_project_phase` ("let's have both"), new
  `m13_remediation` ledger. `[yardstick]`
- `CONTRACTS/CONFORMANCE.md` — control-model section: the stub layer IS the
  control model; no live model in certification, so divergence always indicts
  the harness (the Principal's false-negative concern).
- `CONTRACTS/conformance/*/expected-ledger.jsonl` — regenerated for the payload
  and budget changes above. `[yardstick]`
- `CONTRACTS/REJECTION.md` + `ROLES/WORKER.md` — retry default no longer
  restated in prose; `max_worker_retries` referenced by config key only (the
  Principal's data-duplication note).

RULINGS/-as-directory (🧩 #5) and audits-include-synthetic (✍️ #9) stand as
authored — dispositioned by the Principal's round-1 close-out review
("Everything looks good", 2026-07-12). All nine redline items now resolved.

## v0 — 2026-07-09 — Initial codification

Codified the law from the architecture review of 2026-07-04 (Q1–Q11 locked).

- `ROLES/` — Command (1a/1b), Technical Producer, Worker, Verifier.
- `COMMS.md` — five types, fixed header, SITREP-only precedence, two channels.
- `CONTRACTS/` — decomposition (four claimability requirements), rejection ladder,
  mechanical merge gates, event envelope + type registry, telemetry conformance
  suite with three scripted flights.
- `config/` — fleet caps + model assignment policy (all counts/caps as config).
- `profiler/views.sql` — twelve metric views (m01–m12) + three audit views
  (a01–a03). `[yardstick]` baseline.

**Evidence:** the grill record itself (architecture decisions Q1–Q11, each with
rationale). Drafted by the XO seat, redlined by the Principal on PR #1, merged to main
2026-07-12. Commits before the merge are history, not law: **the first valid
sortie pin is `4afc0be`** (the PR #1 merge commit).
