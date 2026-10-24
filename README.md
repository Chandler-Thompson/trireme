# Trireme

An application for running a fleet of AI agents on a software project. Each seat in the
fleet (Technical Producer, Technical Lead, QA Engineer, and so on) is one long-lived agent
session working under a written doctrine, and the user directs the whole thing through a
chat with an XO and a queue of decisions that need them.

Not affiliated with [aporeto-inc/trireme-lib](https://github.com/aporeto-inc/trireme-lib),
the archived Kubernetes application-segmentation library, or with any other
project of the same name. The word is common; this is a different thing
entirely.

## Status

**The core is working; the MVP is not finished.** What exists today:

- the server (`cmd/trireme`): the journal, fleet states, the blockers queue, raisable
  project caps, seat turns with spend tracking, compaction at a threshold, and pausing on
  the usage window or the cap;
- a harness adapter for the Claude Code CLI (`internal/harness/claudecode`);
- the XO chat;
- a browser UI (`web/`), served by the server;
- `trireme-cli`, which is how seats and the XO reach the shared state;
- the default doctrine (`doctrine/`).

Still to come: the node-graph view, release management, staffing requests, the PRD
editor, the Git-Flow gates, the task-board integration, and a desktop build.

## Run it

```bash
go build ./cmd/trireme ./cmd/trireme-cli
cp examples/trireme.example.json trireme.json   # then edit the paths
PATH="$PWD:$PATH" ./trireme -config trireme.json
```

Open `http://127.0.0.1:8740`. `trireme-cli` must be on the `PATH` the server runs with, so
the seats it starts can find it.

Listening on anything other than loopback requires a `token_file`; the API then expects
that token as a bearer token, and the page asks for it once.

## Tests

```bash
go test -race ./...     # unit and integration tests
tests/e2e/run.sh        # builds everything, runs it with a stand-in harness, and clicks through the UI
```

## Layout

| Path | What it is |
|---|---|
| `doctrine/` | The default doctrine: role cards, modules, default settings. |
| `cmd/trireme` | The server. |
| `cmd/trireme-cli` | The command line seats and the XO use. |
| `internal/` | The server's packages, one per doctrine concern. |
| `web/` | The browser UI, embedded in the server binary. |
| `tests/e2e/` | The end-to-end check. |

## License

MIT. See `LICENSE`.
