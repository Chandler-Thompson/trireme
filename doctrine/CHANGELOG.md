# Doctrine Changelog (layer 2e)

> Amendments **with the evidence that earned them**. An amendment without cited
> evidence (profiler metrics, ruling ids, sortie reports) is not valid law.
> Changes that alter what is measured or how (event types, payload fields,
> metric views, expected conformance ledgers) are flagged **`[yardstick]`**.

## Conventions

- One entry per amendment, newest first.
- Format: date · scope · what changed · **evidence** (ruling ids / metric deltas /
  vault link) · `[yardstick]` flag if applicable.
- Rulings (`RULINGS/`) are case law; recurring rulings harden into entries here.

---

## Doctrine v1 (draft) · 2026-10-23 · full rewrite `[yardstick]`

Replaces doctrine v0 wholesale. v0 was a stateless fleet (one fresh producer, worker and
verifier session per task, per-task and per-session token caps, a pinned sortie). v1 is a
fleet of long-lived specialized seats working under modular doctrine.

- **Seats:** the eleven roles of the user's ideal project layout plus an Engine Engineer
  and a pluggable XO replace producer, worker and verifier (`roles/`).
- **Modules:** the doctrine is split into ten modules, each inheritable, adjustable with
  listed deviations, or replaceable per project (`modules/`). Version control is its own
  module so it can be swapped wholesale.
- **Branching:** Git-Flow replaces trunk-based, with labeled test builds per feature and
  per release candidate.
- **Budgets:** no task or session caps. Sessions compact at 200k tokens and keep going. One
  project cap in API-equivalent dollars; usage-window exhaustion pauses the fleet until the
  user resumes it.
- **Measurement:** the journal stays; v0's metric views, conformance flights and rulings
  directory are dropped from required doctrine.
- **Standards:** default coding standards added, adapted from Holzmann's ten rules, with a
  full and a prototype profile and sections for Go, JavaScript, GDScript and C#.
- **Status:** this repo is now the default doctrine a Trireme application ships with.
  Instances hold their own doctrines as data.

**Evidence:** three design grills held 2026-10-23 (doctrine v1 Q1-Q59, coding standards
N1-N21, new project process P1-P7), recorded outside this repo.
