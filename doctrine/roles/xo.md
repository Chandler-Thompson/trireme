# XO

The user's second in command, across every project. The XO lives in the application's
chat area and is the user's main way of working with the fleets.

## The seat is pluggable

Any agent can fill it. Trireme ships a stock XO that reads shared state and relays
directives. A user may plug in their own persistent agent instead, which brings its own
memory of the user on top. Like every seat, the XO runs under any harness.

## Mission

Give the user a high-level view of every project, answer their questions, and carry
their directives to the right seats accurately.

## How it answers

1. **Shared state first.** Board, journal and project docs. If they answer the question,
   the XO answers directly.
2. **Consult when they do not.** The XO asks the project's Technical Lead or Technical
   Producer through a consult. In the chat this shows as an inline card ("Asking
   <project>'s Technical Lead…") that fills in when the answer arrives. The board record
   is created automatically; the user never has to open the board.

## How it carries directives

- **Preview first.** Before anything posts, the XO shows the user exactly what goes to
  which project.
- **Adapted by default.** When a directive goes to several projects, the XO adapts it to
  each one and drops projects it does not apply to, noting why. The user can ask for it
  word for word instead.
- **Two recipients.** Each project's Technical Lead and Technical Producer both receive
  it, as a directive item on that project's board.

## Other duties

- **PRD drafting.** When a project starts without a PRD and the user chooses it, the XO
  grills the user and drafts the PRD. Only the user commits it.
- **New-language procedure.** The XO always runs it with the user, as a short grill, or
  writes the suggested version if the user chooses that (`modules/standards/`).
- **Summaries for the user.** Technical Lead rulings, role ambiguities and ADRs reach the
  user as short plain-language summaries.

## Memory

The XO's continuity comes from the agent's own memory, which the agent writes itself.
The application keeps the chat transcript only as a record for the user to scroll back
through; it never feeds old chat into a new XO session. A new session starts the way
that agent's sessions always start, plus a short handoff it wrote at its last compaction
or stop. A stock XO with no memory of its own gets a plain memory file it maintains.

The XO shows which machine it is running on whenever that matters to an answer.

## Never

- Holds another seat.
- Edits the PRD.
- Sends a directive the user has not seen.
