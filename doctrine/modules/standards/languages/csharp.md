# C#

How the full profile maps onto C#, including C# in Godot or Unity.

| Rule | In C# | Checked by |
|---|---|---|
| No `goto` | Banned, including `goto case`. | Gate (source check) |
| Recursion with a depth limit | A named depth constant, checked on entry, throwing when exceeded. | Review |
| Unbounded loops | `// loop: unbounded - <reason>` above any `while (true)` or `for (;;)` without a bound. | Gate (source check) |
| No forced GC | `GC.Collect` banned outside pause points. | Gate (banned-API analyzer) |
| Explicit freeing | `using` for every `IDisposable`; engine objects freed by the engine's own rules. | Gate (CA2000), review |
| Hot-path allocation | Benchmarks (for example BenchmarkDotNet's memory diagnoser); zero after warm-up. | Gate (benchmarks) |
| 60-line functions | An analyzer rule for method length. | Gate |
| No mutable globals | No mutable `static` fields outside startup wiring. | Gate (analyzer), review |
| Never discard an error | No empty `catch`; async calls are awaited. | Gate (analyzers) |
| Nulls | `<Nullable>enable</Nullable>`. | Gate |
| Zero warnings | `<TreatWarningsAsErrors>true</TreatWarningsAsErrors>` with the .NET analyzers on. | Gate |
| Coverage | `coverlet`, **branch** coverage. | Gate |
| Test names | `Thing_Conditions_ExpectedResult`, for example `Draw_EmptyDeck_ReturnsError`. | Review |
| Logging | Structured logging with the standard fields. | Review |
