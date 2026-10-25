# Trireme Default Doctrine

The default doctrine that a Trireme application ships with. A doctrine is the written
rule set a fleet works under: who the seats are, how work moves, how it is reviewed,
what it may spend, and what the code must look like.

**This repo is the default, not the only doctrine.** A Trireme instance holds any number
of doctrines as its own data: this default unmodified, a version with listed deviations,
or a full replacement. Changing the doctrine inside an instance never changes this
default. A change to this default is a change to Trireme itself and arrives by reviewed
PR, with the reason in `CHANGELOG.md`.

## Standing invariants

1. **Harness-agnostic seats.** Every seat runs under any harness that implements the
   adapter interface in `modules/seats/`. No rule here may depend on one vendor's tool,
   model or syntax.
2. **One fleet per project.** A fleet is the running staff of one project. There are no
   sorties. Work is organized into milestones and releases.
3. **Shared state only.** Seats never message each other directly. Everything goes
   through the task board, the journal and the project repo, so nothing important is
   said where it cannot be found later. Questions between seats are consults
   (`modules/communication/`).
4. **`main` and `develop` are always green.**
5. **The PRD states intent; the code states the current state.** Every other document
   defers to one of those two.
6. **Counts and limits are settings, never constants.** If a number matters, it lives in
   `config/` and the project can change it.
7. **Spend is measured by machinery.** Token and cost figures come from the harness
   adapter, never from a seat's own account of itself.
8. **Human-readable and agent-readable are different things.** Wherever the user reads
   something, the seat writes a plain-language version alongside the version written
   for agents.

## Layout

| Path | Contents |
|---|---|
| `roles/` | One `AGENTS.md` role file per seat, plus `roles/AGENTS.md`, which every seat shares. |
| `modules/` | The doctrine itself, split into modules. Each one can be inherited, adjusted with listed deviations, or replaced, per project. |
| `config/` | Default settings: seats, models, limits. |
| `CONTEXT.md` | The glossary. Terms are defined once, there. |
| `CHANGELOG.md` | Every change to this default, with the reason that earned it. |

## Modules

| Module | Governs |
|---|---|
| `modules/version-control/` | Branching (Git-Flow), merge gates, test builds, hotfixes. Kept separate so it can be swapped wholesale. |
| `modules/staffing/` | Which seats a project has, how seats are requested and retired. |
| `modules/seats/` | Long-lived seat sessions, compaction, task logs, the harness adapter. |
| `modules/communication/` | The board, consults, directives, the blockers queue. |
| `modules/review/` | Who approves what before code moves. |
| `modules/budgets/` | The project cap, spend tracking, usage-window pauses. |
| `modules/documentation/` | Where project docs live, the PRD, the architecture doc, ADRs, per-seat docs. |
| `modules/standards/` | The default coding standards: full and prototype profiles, language sections, the new-language procedure. |
| `modules/new-project/` | What starting a project asks for and what it sets up. |
| `modules/journal/` | The event record every other module writes to. |

## How a project uses a doctrine

A project names one doctrine and records its version. For each module it either
inherits the doctrine's module as-is, inherits it with a list of deviations, or replaces
it with a different module. Deviations are listed in the project's settings, each with a
reason. The rule in a module that the project has not deviated from binds.

## Every project is different

This is the ideal. A project may need a unique approach, and the module system exists so
it can take one openly. The goal stays fixed: every actor, agent or human, on the same
page and moving in the same direction.

## Version

**Doctrine v1, draft.** Replaces doctrine v0 (the stateless producer/worker/verifier
fleet, law 2026-07-12 to this draft). The decisions behind v1 were made in three grills
on 2026-10-23: doctrine v1 (Q1-Q59), default coding standards (N1-N21) and the new
project process (P1-P7). (AIGenerated)
