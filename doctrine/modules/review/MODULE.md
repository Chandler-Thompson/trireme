# Review

- **Module:** review
- **Version:** 1
- **Governs:** what has to pass before code moves.
- **Depends on:** standards (what the mechanical gate checks), version-control (where the
  gates sit).
- **Settings:** `review.fails_before_lead` (default `2`).

## Three layers

1. **The mechanical gate, on every change.** Tests, coverage, lint and static analysis
   with zero warnings, the standards checks, and file scope. No model cost.
2. **The Technical Lead, once per feature branch.** One review covers every ticket in the
   feature. The Technical Lead reviews the code; QA reviews the behavior.
3. **The Security Engineer, only when a change touches user data or permissions.**

On top of these, **QA passes each feature against the PRD** on its labeled test build
before it merges to `develop`.

## Decorrelation

The reviewing seat runs on a **different model family** from the author's wherever two
families are available. Where they are not, the fleet records that the rule is unmet.

## Failing

- A fail must be evidence-backed: a failing test, a reproduction, or a cited standard.
  "I don't like the approach" is not a fail. If the approach is wrong but meets the spec,
  the reviewer passes it and raises a consult to the Technical Lead about the spec.
- A fail returns the work to its author with the evidence.
- After `review.fails_before_lead` fails on the same work, the Technical Lead is pulled
  in. If the Technical Lead cannot resolve it, it becomes a blocker.
- Reviewers never fix the code they review.
