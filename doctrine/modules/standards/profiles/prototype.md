# Prototype Profile

For quick proofs of concept, where strict limits would slow down the one question the
prototype exists to answer.

**Keeps** from the full profile:

- **Warnings and analysis:** all warnings on, zero warnings, analysis on every change.
- **Errors:** never discard an error; a deliberate ignore takes a why-comment.

**Drops** everything else, including the coverage target.

A project leaving prototype status switches to the full profile, and the switch is
recorded. For an existing codebase the usual baseline applies: new violations are blocked
and touched files drop their old ones (`modules/new-project/`).
