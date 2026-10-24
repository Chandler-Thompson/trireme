# JavaScript

How the full profile maps onto JavaScript.

| Rule | In JavaScript | Checked by |
|---|---|---|
| Recursion with a depth limit | A named depth constant, checked on entry, throwing a typed error when exceeded. | Review |
| Unbounded loops | `// loop: unbounded - <reason>` above any `while (true)`, `for (;;)` or stream loop without a bound. | Gate (source check) |
| No forced GC | Not available in browsers; never call `global.gc()` in Node outside pause points. | Gate (source check) |
| Hot-path allocation | Benchmarks for declared hot paths, measured with the runtime's heap statistics. | Gate (benchmarks) |
| Explicit freeing | Remove event listeners, clear timers and intervals, close streams and sockets. | Review, tests |
| 60-line functions | ESLint `max-lines-per-function: 60`. | Gate |
| Preconditions | Early `return` or `throw` at the top. | Review |
| No mutable globals | ESLint `no-implicit-globals`; no top-level `let` that is reassigned. | Gate |
| Never discard an error | No empty `catch`; no unhandled promise (a lint rule that rejects floating promises). | Gate |
| No hidden behavior | ESLint `no-eval`, `no-implied-eval`, `no-new-func`. | Gate |
| Zero warnings | ESLint with `--max-warnings 0`. | Gate |
| Coverage | V8 coverage (for example `c8`), **branch** coverage. | Gate |
| Test names | `test("thing_conditions_expectedResult", ...)`, for example `draw_emptyDeck_returnsError`. | Review |
| Logging | One JSON object per line with the standard fields. | Review |
