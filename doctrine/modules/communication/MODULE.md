# Communication

- **Module:** communication
- **Version:** 1
- **Governs:** how seats, the XO and the user exchange anything.
- **Depends on:** journal.
- **Settings:** none yet.

## Shared state only

There are three stores, and no others:

1. **The task board** (wood-fired-tasks or an equivalent): tickets and every item below.
2. **The journal** (`modules/journal/`): the event record.
3. **The project repo**: code and `docs/`.

**Seats never message each other directly.** Direct messages are where important
conversations get lost, at a real workplace and in a fleet.

## Item types on the board

| Item | From → To | What it is |
|---|---|---|
| **Ticket** | Producer → a seat | A piece of work: spec, acceptance tests, file scope, dependencies, estimate. |
| **Consult** | Any seat or the XO → a seat | A question for the seat that owns the answer. Answered on the item. The asker may keep working on anything the question does not block. |
| **Directive** | User, through the XO → Technical Lead and Producer | A command, directive or information from the user. The XO previews it to the user before it posts. |
| **Blocker** | Any seat → the user | A decision only the user can make. Lands in the blockers queue. |
| **Review** | Reviewing seat → author | A pass or fail on a task, feature or release, with evidence for a fail (`modules/review/`). |
| **Report** | Any seat | Status on a ticket. A done report points at its evidence. |

A communication need that fits none of these is raised to the Technical Lead as a role
ambiguity, never improvised.

## The blockers queue

Every blocker goes to one queue in the application. The user answers each one with a text
box, buttons, or both. A blocker shows its plain-language summary first, with the detail
one click away. Things that arrive as blockers include: a staffing or retirement request
(when the at-will box is off), a PRD deviation asking to enter a release, a release or
hotfix asking for `main`, a doctrine change proposed by a fleet, a Security hold, a
benchmark regression past threshold, a red `main` or `develop`, and a fleet paused for
limits.

## Notifications

The doctrine requires only that blockers are visible in the queue. An instance may plug
in a notifier (for example, a phone push per new blocker) through Trireme's notification
hook.

## Style

- One item, one purpose.
- State facts and point at evidence. No pleasantries.
- Everything the user reads gets the plain version (`modules/seats/`).
