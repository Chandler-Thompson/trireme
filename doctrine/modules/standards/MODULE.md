# Standards

- **Module:** standards
- **Version:** 1
- **Governs:** what the code itself must look like, and how that is checked.
- **Depends on:** review (the mechanical gate runs these checks), documentation.
- **Settings:** `standards.profile` (`full` or `prototype`, default `full`), `performance.regression_threshold_pct`,
  `standards.coverage_target_pct` (default `100`).

## Shape

- **Profiles:** `profiles/full.md` (the default) and `profiles/prototype.md`. A project
  can also run an entirely different set of standards, for A/B testing or because the
  ideal does not fit it.
- **Language sections:** `languages/<language>.md`, each mapping the profile's rules to
  that language's tools and syntax. Shipped: Go, JavaScript, GDScript, C#.
- **New languages:** `new-language-procedure.md`.

A project normally inherits a profile and lists its deviations, each with a reason. The
standards are per project because the ideal may not fit every project and will change
over time.

## Origin

The full profile adapts Gerard J. Holzmann's "The Power of Ten: Rules for Developing
Safety Critical Code" (NASA/JPL Laboratory for Reliable Software) "within reason". Those
rules were written for C on embedded, safety-critical systems. Each rule below keeps the
intent and translates or drops what does not carry over to garbage-collected languages.

## Enforcement

Everything a tool can check is enforced by the mechanical gate. The rest is the reviewer's
judgment. Breaking a rule on purpose takes an **inline suppression with a why-comment**
(for example `//nolint:funlen // dispatch table`). The Technical Lead sees every new
suppression in review, and each project's suppression count shows on its node.

A project moving out of prototype status switches to the full profile, and the switch is
recorded.
