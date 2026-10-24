# Full Profile

The default for every project. Rule numbers in brackets refer to Holzmann's ten rules.

## Control flow [1]

- No `goto`.
- **Recursion only with a named depth limit** that fails loudly when reached. Otherwise
  use a loop with an explicit stack.
- Panics and exceptions are never normal control flow. They are for conditions that
  should be impossible.

## Loops [2]

- Every loop has a bound of some sort. A loop over a fixed collection is bounded already.
- A loop driven by outside input (network data, files, queues, a player's actions) has a
  **named maximum** and fails loudly when it reaches it. So does a loop over a channel,
  stream or socket, unless it is labeled unbounded.
- **A loop allowed to run forever is strictly labeled**, on the line above it, with the
  marker comment `loop: unbounded - <reason>` in the language's comment syntax. Only
  top-level loops may be unbounded (a server's main loop, a game loop).
- The gate fails any endless loop without the label. A newly added label pulls the DevOps
  Engineer into that feature's review, and **the Technical Lead and the DevOps Engineer
  must both approve it.** Where DevOps is unstaffed, the Technical Lead approves alone and
  logs it.

## Memory [3]

- A **hot path** is code the Technical Lead declares in `docs/performance.md` (for
  example the per-frame update, the per-tick server loop, or effect resolution). The
  DevOps Engineer also watches for hot paths nobody has declared.
- **On a declared hot path, allocation after warm-up is zero** (preallocate and pool) or
  is justified with a why-comment. A benchmark test measures it, so the gate enforces it.
  DevOps tracks each hot path's benchmarks over time and raises a blocker when one gets
  worse past the project's threshold.
- **Explicit freeing is required** wherever the language does not do it for you: engine
  objects that must be freed by hand, and files, timers, sockets and background workers,
  which must be closed or stopped. A linter or test checks this where one can.
- **Forcing a garbage collection is banned**, except at a known pause point (a level load,
  the gap between matches) with a why-comment.

## Functions [4]

- **At most 60 lines per function**, enforced by the linter. Aim much smaller: small,
  single-purpose functions as far as is feasible and sensible.
- Exceptions: table-driven data, a switch over an enumeration, and generated code.
- The 60 is revisited if it becomes an unnecessary blocker.

## Preconditions and invariants [5]

- **Early-outs first.** Every function checks its preconditions at the top and fails with
  an error when they do not hold.
- Invariants that should never break use an assert helper that is active in tests and
  debug builds.
- Nothing is counted. There is no assertion-density target, because a counted target
  invites padding.

## Scope and shared state [6]

- Declare everything at the smallest possible scope.
- **No mutable package-level or global state** outside the code that wires the program
  together at startup. State created at startup is passed to the functions that need it,
  so who uses it is visible in their signatures. Constants and read-only lookup tables at
  top level are fine.

## Errors and parameters [7]

- **Never discard an error.** Ignoring one on purpose takes a why-comment.
- Validate parameters **at boundaries**: exported functions, API handlers, and anything
  that reads outside data. Inside a boundary, rely on types.

## Hidden behavior [8]

- No `eval` and no dynamically built code.
- Reflection only in serialization layers.
- Generated code is committed, marked as generated, and kept readable.

## Indirection [9]

- No extra layer of interfaces or callbacks where a direct call would do. Callbacks,
  handlers and signals are fine where they are the idiom.
- No mutable data shared between threads. Pass copies, or use channels or the language's
  equivalent.
- Nil and null checks enforced by the linter where one exists.

## Warnings and analysis [10]

- **All warnings on from day one, and zero warnings.**
- Static analysis runs in the mechanical gate **on every change**.

## Logging

- Structured, one record per line, with: time, level, component, the acting seat and
  ticket when there is one, an event name, and a plain-English message. Agents can filter
  it and humans can read it.
- The shared state created at startup logs every change at debug level, with the
  component that made it, so a corrupted value can be traced to whoever touched it.
- **Never log secrets or personal data.** The Security Engineer checks.

## Tests and coverage

- **Coverage target 100%** (`standards.coverage_target_pct`) until there is a clear
  understanding of why it is not possible. **Branch coverage** wherever the tooling
  supports it, statement coverage where it does not. Generated code and startup wiring are
  excluded.
- Test names follow **(thing being tested)_(specific conditions)_(expected result)**, in
  each language's required form.

## Writing

- Comments explain **why**, never what, and only when the reasoning is not obvious.
- Consistent, clearly named functions, classes and variables. Descriptive names over short
  ones.
- Do not duplicate code.
- Docs are code; API references are generated (`modules/documentation/`).

## From the start

Keep profiling, testing, logging, analytics, documentation, build deployments (local, test
and live builds), infrastructure and server scalability, and security in mind from the
beginning. **Agents must be able to interact with the project from day one**: every
action a user can take through a UI also exists as an API call, so seats can do anything
a user can without driving the UI. (For example, a game engine needs a command-line or
MCP-style control surface; a web app needs an API under every screen.)
