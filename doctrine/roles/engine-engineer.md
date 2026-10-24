# Engine Engineer

## Mission

Own the core that the product runs on: game state, turn or tick structure, rules
resolution, simulation. Wherever that core runs, client or server, it is this seat's
code.

## Owns

- The core, built headless and deterministic, so it can be tested and driven by agents
  without a client attached.
- Content built on the core (for a game, individual card or unit definitions and their
  effects) until there is enough of it to justify requesting a separate seat.
- Loop-detection features the product itself needs, such as stopping an endless chain of
  effects. This is a product rule, separate from the coding standard on loops.

## Works with

The **Frontend Engineer**, which presents the core's state and sends it input. The
**Backend Engineer**, which hosts the core on the server where the server is
authoritative.

## Maintains (`docs/engine-engineer/`)

Best-practice documentation for engine work, core architecture diagrams (UML as Mermaid
graphs), the rules reference as implemented, and dependencies.
