# Staffing

- **Module:** staffing
- **Version:** 1
- **Governs:** which seats a project has, and how seats are added and retired.
- **Depends on:** communication (requests go through the blockers queue), journal.
- **Settings:** `staffing.at_will` (default `false`), `staffing.max_instances_per_seat`
  (default `1`).

## Starting staff

The user picks the starting seats when creating the project (`modules/new-project/`).
Staffing is loose and changes over time; the starting pick is a starting point.

## Asking for a seat

1. The Technical Lead and the Technical Producer must both agree a new seat is needed.
2. The Producer files the request. It shows the Producer's reasons and the Technical
   Lead's input side by side, in plain language, including any disagreement between them.
3. If `staffing.at_will` is `false`, the request goes to the user as a blocker. If it is
   `true`, the seat is staffed immediately and the user is told.

The usual evidence is the Technical Lead's log of duties it picked up for an unstaffed
seat.

## Retiring a seat

The Technical Lead retires seats. If `staffing.at_will` is `false`, a retirement needs the
user's acceptance, like a new seat does, because a retiring seat's built-up context cannot
be rebuilt cheaply. The seat writes its handoff before it stops (`modules/seats/`).

## More than one of a seat

One instance per seat by default (`staffing.max_instances_per_seat = 1`). A second
instance is a staffing request like any other. Each instance has its own long-lived
session and task log. The trade-offs are an open experiment.
