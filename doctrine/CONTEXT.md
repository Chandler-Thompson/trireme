# Glossary

The fleet's vocabulary. Terms are defined here once and used the same way everywhere.
Only terms that have been ruled on appear.

**Project**:
One body of work with its own repo, PRD, settings and fleet.

**Fleet**:
The running staff of one project. One project, one fleet. A fleet can be running,
paused, stopped or archived.
_Avoid_: sortie (retired in v1; milestones and releases do that job now).

**Seat**:
A role a project can staff, defined by a card in `roles/`. Filled by one long-lived
session under some harness and model.
_Avoid_: sailor, worker, agent (when you mean the role).

**Engineer seat**:
Any seat with "Engineer" in its title. These approve the architecture doc.

**XO**:
The user's second in command across all projects, living in the application's chat. A
pluggable seat.

**Harness**:
The program that runs a seat's session and talks to its model. Reached only through an
adapter.

**Consult**:
A question from one seat (or the XO) to the seat that owns the answer, asked and answered
as an item on the board.
_Avoid_: message, DM, ping.

**Directive**:
A command, directive or information from the user, carried by the XO to a project's
Technical Lead and Technical Producer.

**Blocker**:
A decision only the user can make. Lives in the blockers queue.

**Task log**:
A seat's append-only record of task starts, completions and compactions, each written in
an agent version and a plain version.

**Handoff**:
The task-log entry a seat writes before a planned stop, and the first thing it reloads on
restart.

**Hot path**:
Code the Technical Lead declares in `docs/performance.md`, where allocation after warm-up
must be zero or justified.

**Deviation**:
A listed, reasoned change a project makes to an inherited module. Also used for a feature
that departs from the PRD (a **PRD deviation**).

**Profile**:
A complete set of coding standards: `full`, `prototype`, or one of the user's own.

**Claim**:
An exclusive take of a ticket that the board enforces, so a second taker is refused.
_Avoid_: dibs, reserve, assign. Never use "claim" for a hold that nothing refuses; that is
**dibs**.

**Dibs**:
Not used here. A declaration of intent that gates nothing and refuses nobody. Listed so
the distinction stays visible: if a mechanism here stops refusing a second taker, it is
dibs and must be renamed.
