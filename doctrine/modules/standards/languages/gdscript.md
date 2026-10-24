# GDScript

How the full profile maps onto GDScript in Godot 4.

| Rule | In GDScript | Checked by |
|---|---|---|
| Recursion with a depth limit | A named depth constant, checked on entry, pushing an error and returning when exceeded. | Review |
| Unbounded loops | `# loop: unbounded - <reason>` above any `while true` without a bound. Godot's own `_process` and `_physics_process` callbacks are the engine's loop and need no label. | Gate (source check) |
| Explicit freeing | Nodes and plain `Object`s must be freed with `queue_free()` or `free()`. `RefCounted` and `Resource` free themselves. | Review, tests (orphan-node checks) |
| Hot-path allocation | Pool nodes and objects on declared hot paths; measure with Godot's performance monitors in a benchmark scene. | Gate (benchmarks) |
| 60-line functions | A linter where it supports the check, otherwise the gate's own line counter. | Gate |
| No mutable globals | Autoload singletons count as startup wiring: they may hold state, and every change to it is logged. | Review |
| Never discard an error | Check every returned `Error`; `push_error` on failure. | Review |
| Zero warnings | Every GDScript warning set to **Error** in Project Settings (`debug/gdscript/warnings/`). | Gate (headless import fails on error) |
| Typing | Static typing everywhere; the untyped-declaration warnings are errors. | Gate |
| Coverage | Godot has no built-in coverage. Use a coverage tool where one works for the project; where none does, record it as a deviation. | Gate or deviation |
| Test names | Test frameworks run functions starting with `test_`: `test_thing_conditions_expected_result`, for example `test_draw_empty_deck_returns_error`. | Gate (source check) |
| Logging | One JSON object per line with the standard fields, through one logging autoload. | Review |
