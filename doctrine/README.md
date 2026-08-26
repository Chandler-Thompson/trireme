# Trireme Doctrine

The law. **The rule binds; the reasoning is recorded separately.** Every amendment in
`CHANGELOG.md` carries the evidence that earned it, so if a rule here looks arbitrary,
the changelog entry that introduced it says what measurement forced it. The rule still
binds until amended.

## Standing invariants

1. **Harness-independent.** Any harness implementing the four contracts (agent loop,
   tools, board/doctrine clients, telemetry emitter) runs this doctrine — including
   degraded mode on plain headless CLI sessions. The harness is an optimization, not
   a dependency.
2. **Pinned per sortie.** Every sortie pins one doctrine commit hash at open; the hash
   is stamped into every event-log entry. Mid-sortie doctrine changes do not exist.
3. **Main is always green.** Red main = FLASH, fleet stops.
4. **No point-to-point agent communication.** All interaction is mediated through
   shared state: task board (5a), event log (5b), git (5c). No fourth channel.
5. **All counts and caps are config, never constants.** If a number matters, it lives
   in `config/`.
6. **Machinery-only telemetry emission.** No agent self-reporting, ever.

## Layout

| Path | Layer | Contents |
|---|---|---|
| `ROLES/` | 2a Roster | Role cards — seats, not sailors. Command, Producer, Worker, Verifier. |
| `CONTRACTS/` | 2b Contracts | Decomposition, rejection ladder, merge gates, event envelope, telemetry conformance suite. |
| `COMMS.md` | 2c Comms | Message types, header format, precedence, channels. |
| `config/` | 2d Assignment | Model assignment policy, retries, budgets, caps. |
| `CHANGELOG.md` | 2e Changelog | Amendments **with the evidence that earned them**. `[yardstick]` flag when measures change. |
| `profiler/views.sql` | 2b (executable) | The metric views (m01–m13) + audit views. Bookkeeping law. |
| `RULINGS/` | 2e (case law) | One-paragraph rulings filed at sortie close for every escalation; recurring rulings harden into amendments. |

## The learning loop

Profiler evidence → ruling → precedent → `CHANGELOG.md` amendment → next sortie pins
new doctrine → profiler measures the delta. Loud gaps (law silent) escalate by
construction: CLARIFY / FLASH / human touch are all profiler-counted. Silent gaps
(Goodhart) are defended by metrics-in-tension, cross-family verification, and the
periodic doctrine red-team sortie (Command-ordered; XO seat).

## Version

**Doctrine v0 — LAW as of 2026-07-12.** Codified 2026-07-09 from an architecture
review of 2026-07-04 (Q1–Q11 locked), drafted by the XO seat, redlined by the
Principal on PR #1 (nine items, all resolved), merged to main.

Every sortie pins one doctrine commit hash at open and stamps it into every event.
Amendments arrive by reviewed PR with the evidence that earned them (`CHANGELOG.md`).

The commit hashes named in `CHANGELOG.md` are from the private repository this
doctrine was developed in. They are kept because the changelog entries cite them as
evidence; they do not resolve in this repository. Pin against this repository's own
history instead.
