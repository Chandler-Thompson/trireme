# Go

How the full profile maps onto Go. "Gate" means the mechanical gate; "review" means the
reviewer's judgment.

| Rule | In Go | Checked by |
|---|---|---|
| No `goto` | Banned. | Gate (source check) |
| Recursion with a depth limit | A named `maxDepth` constant, checked on entry, returning an error when exceeded. | Review |
| Unbounded loops | `// loop: unbounded - <reason>` above any `for {}` or channel loop without a bound. | Gate (source check) |
| No forced GC | `runtime.GC()` and `debug.FreeOSMemory()` banned outside pause points. | Gate (`forbidigo`) |
| Hot-path allocation | Benchmarks with `b.ReportAllocs()` or `testing.AllocsPerRun`; zero after warm-up. | Gate (benchmarks) |
| Explicit freeing | Close files, bodies, rows; stop timers and tickers; every goroutine has an owner that stops it. | Gate (`bodyclose`, `sqlclosecheck`), tests (`goleak`), review |
| 60-line functions | `funlen` with `lines: 60`. | Gate |
| Preconditions | Early `return err` at the top of the function. | Review |
| Invariants | A small `assert` package active under a build tag for tests and debug builds. | Review |
| No mutable globals | `gochecknoglobals`; constants and read-only tables allowed. | Gate |
| Never discard an error | `errcheck`; `_ =` needs a why-comment. | Gate |
| No hidden behavior | `reflect` only in serialization packages; generated files carry the standard `// Code generated ... DO NOT EDIT.` header. | Gate (`depguard`), review |
| Shared data between goroutines | Pass copies or use channels; run tests with `-race`. | Gate (`go test -race`) |
| Nil checks | `nilaway` or an equivalent nil-safety analyzer. | Gate |
| Zero warnings | `go vet` plus `golangci-lint` (including `staticcheck`) with no findings. | Gate |
| Coverage | `go test -coverprofile`. Go measures **statements only**, so Go projects get statement coverage. | Gate |
| Test names | Go only runs functions starting with `Test`: `TestThing_Conditions_ExpectedResult`, for example `TestDraw_EmptyDeck_ReturnsError`. | Gate (source check) |
| Logging | `log/slog` with the JSON handler. | Review |
