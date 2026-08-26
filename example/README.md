# The worked example

One sortie, three tasks, against a real repository small enough to read in a
sitting. This is the reference for what a well-formed task spec looks like
under `doctrine/CONTRACTS/DECOMPOSITION.md`.

```bash
bash example/bootstrap_fixture.sh     # fixture -> /tmp/trireme-example-fixture
python3 example/sortie_example.py     # dry run: validates the manifest, spends nothing
```

The dry run is the default and it is the mode you will use most. It checks the
dependency graph, checks every declared path against the fixture as it actually
exists on disk, and checks that no two tasks open at the same time declare the
same file. Flying for real needs `--launch-approved`, because a sortie is an
expensive thing to start by accident.

## The fixture

`example/fixture/` is a closed-integer-interval library: `Interval` with
`length`, `overlaps`, `merge`, and a nine-case unittest suite that passes.

It also contains one real defect. `overlaps` uses strict comparisons on
intervals the class documents as closed, so `Interval(1, 3)` and
`Interval(3, 5)` — which share the integer 3 — are reported as disjoint. The
existing suite covers the clearly-disjoint and clearly-nested cases and never
touches the boundary, which is exactly how a defect like this survives a green
suite in a real codebase. The sortie's second task repairs it and its third
task makes the gap uncloseable in future.

The defect is planted and documented as such. It is not a mistake in the
example.

## The four requirements, read off the specs

A task is claimable only when all four of these hold. Each is visible in
`sortie_example.py`.

**1 — Acceptance criteria as tests to make pass.** Every `acceptance_criteria`
in the example is a set of statements a machine can check: the command that
must exit 0, the specific calls that must return specific values, the file that
must be unchanged in the diff. None of them says "correctly", "properly" or
"as appropriate". If a criterion needs a person to adjudicate it, it is not a
criterion — and per the direction law, the unspeccable part goes *up* to
Command rather than down to a Worker as prose to interpret.

T-003's criterion is the strongest form of this: *reverting the T-002 fix
locally makes this suite fail*. That asks the Worker to prove the test has
teeth rather than merely to prove it is green. A test that cannot fail is not
evidence.

**2 — Exclusive file-scope.** Each task declares every file it may touch, and
no two *open* tasks declare the same file. The board enforces this at posting
time, not at review time.

This is the requirement that most often forces a redesign, and the example is
built to show it: T-001 and T-002 are logically independent — adding a
membership test has nothing to do with fixing an overlap comparison — but both
must edit `src/intervals.py`. They cannot both be open. So T-002 declares
`deps=["T-001"]` and the sortie flies them in separate waves.

**That dependency edge exists for file-scope reasons, not logical ones, and
the comment in the source says so.** The alternative is worse: splitting one
small module into two so two tasks can run in parallel is letting the
orchestration deform the codebase. Order the tasks instead. Scope is released
when a task merges, so ordering costs wall-clock time and nothing else.

**3 — Declared dependency edges.** Every edge names a task id in the same
list. The board will not offer a task while its dependencies are open.
T-003 declares both of its predecessors even though T-002 already depends on
T-001, because an edge documents what this task actually needs, and inferring
it through the graph makes the spec depend on a second document to read.

**4 — Budget caps.** Both the token cap and the wall-time cap are set at
posting. The example omits them, which accepts the fleet defaults in
`doctrine/config/fleet.toml`. A Producer may tighten a budget to make a task
fail faster; it may never loosen one.

## What the specs do beyond the four requirements

**They name the file and the symbol.** "src/intervals.py's Interval has no
membership test" beats "add a contains method". A Worker that has to search
for the thing it is meant to change will sometimes find the wrong thing.

**They fence off adjacent work explicitly.** T-001 ends with *do not modify
overlaps() — the defect in it is T-002's scope*, and T-002 ends with *the
existing TestOverlaps cases must keep passing unchanged — that file is NOT in
your file-scope*. A capable Worker that notices a nearby defect will want to
fix it. Telling it in advance which neighbouring problem belongs to someone
else converts a scope violation into a non-event.

**They say what is out of scope inside the task, too.** T-001 says
non-integers are out of scope and asks for no type coercion. Left unsaid, that
is a coin flip, and a Worker that guesses generously produces a diff the
acceptance criteria do not cover.

**The brief carries the standing CLARIFY answers.** `SORTIE_BRIEF` states, once
and in advance, that acceptance criteria outrank the description, that
acceptance tests are immutable within a task, and that a task whose lawful
reading makes the work unnecessary should be closed minimal with evidence.
These are the three questions Workers actually ask. Answering them in the brief
turns three CLARIFY round-trips into zero.

## Writing your own

Copy `sortie_example.py` and change three things: the `ProjectProfile` at the
top, the `SORTIE_BRIEF`, and the `TASKS` list. Everything else — the manifest
validator, the wave loop, the worktree handling, the launch gate — carries
over unchanged.

The `ProjectProfile` is the whole of what the harness knows about your
repository. Note that `test_cmd` and the tool-permission entry for it are named
separately, on purpose: a shell command and a permission pattern are different
grammars, and a harness that guesses one from the other produces either an
over-permitted seat or a silently blocked one. Spell out both.
