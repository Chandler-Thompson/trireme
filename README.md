# Trireme

A doctrine for running a fleet of coding agents on one repository, and a
harness that flies it.

The premise is that the hard part of multi-agent software work is not the
agents. It is the law they operate under: what makes a task claimable, who is
charged when work is rejected, what a merge gate checks mechanically rather
than by judgment, and what the machinery is required to record so the whole
thing can be measured afterwards. Trireme is that law, written down, plus one
working runner.

Not affiliated with [aporeto-inc/trireme-lib](https://github.com/aporeto-inc/trireme-lib),
the archived Kubernetes application-segmentation library, or with any other
project of the same name. The word is common; this is a different thing
entirely.

## What is here

| Path | What it is |
|---|---|
| `doctrine/` | The law. Role cards, contracts, comms, config, the metric views, the changelog. Harness-independent by construction. |
| `harness/` | The supported runner: seats as headless agent-CLI sessions, board client, lifecycle engine, merge gate, telemetry emitter. |
| `conformance/` | Certification: three scripted flights replayed with stubbed model and tool layers and compared byte-exact against the doctrine's expected ledgers. |
| `tests/adversarial.py` | Proof that certification is earned — the checks that fail if the engine hardcodes what it is supposed to compute. |
| `example/` | One worked sortie against a small real fixture. Start here. |

## Start here

```bash
bash example/bootstrap_fixture.sh     # fixture -> /tmp/trireme-example-fixture
python3 example/sortie_example.py     # dry run: validates the manifest, spends nothing
```

Then read `example/README.md`, which walks the four claimability requirements
off the example's own task specs, and `doctrine/CONTRACTS/DECOMPOSITION.md`,
which is the law those specs answer to.

## The shape of it

Four seats, and none of them talk to each other directly.

- **Command** — Principal (approvals, anything external or irreversible) and
  XO (reads the profiler, drafts amendments, never holds a production seat).
  Command is absent from the happy path by design; every mid-sortie human
  touch is counted as a doctrine hole.
- **Technical Producer** — decomposes an objective into claimable tasks, and
  answers CLARIFYs. Charged when decomposition was wrong.
- **Worker** — does the work inside a declared file-scope, and stops rather
  than guessing.
- **Verifier** — judges, with evidence, and never fixes.

All interaction is mediated through shared state: the task board, the event
log, and git. There is no fourth channel. That constraint is what makes the
whole run reconstructible from the ledger afterwards.

Six standing invariants govern everything else, and they are in
`doctrine/README.md`. The one that does the most work: **all counts and caps
are config, never constants.** The rejection ladder is computed from
`doctrine/config/fleet.toml` at runtime, and `tests/adversarial.py` proves it
by changing the config and watching the attribution change.

## Requirements

- Python 3.10+ (standard library only — the harness has no dependencies)
- git
- An agent CLI that runs headless and returns a JSON result envelope. The
  shipped adapter drives Claude Code; `harness/claude_cli.py` is the only file
  that knows that, and the seats above it speak a plain action vocabulary.
- A [wood-fired-tasks](https://github.com/Wood-Fired-Games/wood-fired-tasks)
  board. See the next section before you commit to this.

Credentials live outside the repository, referenced by location only:

```
~/.config/trireme/board.env
    BOARD_API_URL=http://your-board.example:3000
    BOARD_API_KEY=<personal access token>
```

`BOARD_API_URL` is the origin, with no path — the client appends `/api/v1`.
Include it yourself and you get a 404 naming the doubled path.

Override with `TRIREME_BOARD_ENV`, and point at a different doctrine checkout
with `TRIREME_DOCTRINE`.

## The board is a hard requirement, and it has defects

Trireme talks to one task board: **wood-fired-tasks**. This is a deliberate
choice of one supported thing over a pluggable interface, and the cost of that
choice is that its defects become yours. These are the ones you will actually
hit, observed on v2.4.0 in production use. None are filed upstream yet.

**`done` and `closed` are too close to tell apart, and choosing wrong is
silent.** The board has two terminal states: `done` means the work happened,
`closed` means it terminated without happening. Only `done` fires the
auto-unblock that releases dependent tasks. The logic is right; the naming is
not. `closed` is the ordinary English word for finishing a ticket, and every
other tracker, tool and habit an agent brings uses it that way — so the correct
behavior requires knowing a convention that contradicts the word.

Choosing `closed` when you meant `done` raises no error, strands every
dependent task, and surfaces only if something happens to be waiting. In
practice this produced five misfiles on a single board before anyone noticed.
It matters more here than elsewhere, because Trireme's dependency edges are
load-bearing: a stranded task is a wave that never opens.

Repair is worse than it looks. The only transition out of `closed` is back to
`open`, which returns a finished task to the frontier where another session can
claim and redo it — and passing through `open` fires the claim sweep, which
clears the assignee. So repairing a misfile costs you the attribution
permanently.

**Read this as: never let anything in your tooling say "close the task."** Say
`done` or say `abandoned`, and grep your own prompts and scripts for the word
before you fly.

**`tasks login --server <url>` is ignored.** The CLI persists the server you
give it, `tasks whoami` echoes it back, and then every request goes to
`http://localhost:3000` regardless. It surfaces as `401 Authentication
required` while the same token works fine by curl. Export `API_BASE_URL` on
every command. The harness talks REST directly and is unaffected — this bites
you at the terminal.

**`deploy/upgrade.sh` migrates the wrong database.** It reads `PORT` from the
installed `.env` but runs the migration with no environment, so it migrates a
database nothing serves and reports success. Deploys work only because the app
also migrates itself on start. Its health probe hardcodes `localhost` in the
same script, so any non-loopback `HOST` fails a healthy deploy.

Two more worth knowing, neither a defect: task descriptions cap at 5000
characters server-side, and claims expire after 30 minutes with the assignee
cleared, so "I claimed it" stops being true across any long gap.

## Provenance

The doctrine was developed and flown privately before this extraction, across
seven sorties against two real repositories. The changelog entries cite commit
hashes and sortie reports from that private history — the reasoning is
preserved, the artifacts are not public, and those hashes will not resolve
here. Pin against this repository's own history.

The Principal and XO seats are roles, not people. They were held by a person
and an agent respectively, which is where the role cards' shape comes from,
but nothing in the law requires that arrangement.

## Licence

MIT. See `LICENSE`.
