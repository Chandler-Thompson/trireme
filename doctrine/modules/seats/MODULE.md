# Seats

- **Module:** seats
- **Version:** 2
- **Governs:** how a seat's session runs, how it remembers, and how any harness hosts it.
- **Depends on:** journal, budgets (usage-window pauses).
- **Settings:** `seats.compact_at_tokens` (default `200_000`), `seats.log_entries_on_restart`
  (default `5`).

## Long-lived sessions

Each staffed seat runs as **one long-lived session**, so it keeps the context it has
already loaded. A session **autocompacts at `seats.compact_at_tokens`** and keeps going.
Nothing stops a session at that point; the only limits are project-level
(`modules/budgets/`).

## Task log

Every seat keeps an append-only task log. Its location and entry format are in
`roles/AGENTS.md`. While seats share one working tree, it lives in the untracked `.board/`
folder, so it is the same on every branch; it moves to the board when the board
integration lands. A seat writes an entry:

- **At task start:** what it is about to do, its plan, and what it read to get there.
- **At task completion:** what changed, what it learned, open threads, and what the next
  holder of this work needs.
- **At every compaction:** what it is carrying forward, because compaction is when a seat
  forgets the most.

Each entry has two versions, written by the seat at the same moment, because it knows what
actually mattered:

- an **agent version**, shaped for a seat to reload, and
- a **plain version**, a few sentences for the user. This is what the application shows
  on node details, blockers, staffing requests, Technical Lead rulings and ADR summaries.

## Handoff

Before any planned stop (pause, stop, retirement, harness switch) the seat writes a
handoff entry. A seat that restarts, planned or not, reloads from its role file, its handoff,
its last `seats.log_entries_on_restart` log entries, the current task's entries, and the
recent journal. This is also what makes a seat portable between harnesses.

## The harness adapter

A seat never depends on one harness. Trireme talks to every harness through one adapter
interface. An adapter must:

1. Start, resume and stop a seat session with a given role file, model and working directory.
2. Deliver work to the session and return its output.
3. Report spend per model call: model, input tokens, output tokens, and cost or
   API-equivalent cost (`modules/budgets/`).
4. **Detect the harness's usage-limit message** and report it, so the fleet can pause.
   Each harness says it differently.
5. Trigger or observe compaction.

The first adapter is for Claude Code. Every other harness or sandbox gets an adapter of
its own. The long-term intent is that a sandbox holds every seat, with any harness
running inside it.

## Models

Each seat's model comes from `config/models.toml` unless the project overrides it. The
Producer may request a change through the blockers queue when spend data supports it.
