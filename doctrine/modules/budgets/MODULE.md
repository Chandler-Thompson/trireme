# Budgets

- **Module:** budgets
- **Version:** 1
- **Governs:** what a project may spend, and what happens when it runs out.
- **Depends on:** seats (the adapter reports spend and usage limits), journal.
- **Settings:** `budgets.project_cap_usd` (set per project at creation).

## No task caps

Tickets carry an **estimate**, for the Producer's estimate-versus-actual tracking, but no
cap. Sessions compact and keep going (`modules/seats/`).

## The project cap

Each project has one cap, in **API-equivalent dollars**, for the life of the project.
Tokens are recorded underneath. The user can raise it at any time from the project's
settings. When spend reaches the cap, the fleet **pauses** and a blocker tells the user.

## Spend is tracked at every level

Every model call's spend is charged to the seat that made it and to the ticket that seat
has marked as current. From there it rolls up to the feature, the release, the project
and the instance. Spend comes from the harness adapter, never from the seat.

## Usage windows

When a subscription's usage window is spent, the adapter reports it and **the fleet
pauses**. It stays paused until **the user resumes it manually**. Other than that, the
fleet keeps going until it reaches the project cap.

## Pause, stop, archive

- **Pause:** every seat writes its handoff and its session closes, so a paused fleet costs
  nothing. Resuming restarts each seat from its handoff.
- **Stop:** the same, and the fleet leaves the main view. Its information is kept.
- **Archive:** the fleet is hidden from every view. Its data is kept, never deleted.
