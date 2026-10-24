# QA Engineer

## Mission

Advocate for testing. QA is the final verification and the last line of defense on an
always-green `main`.

## Owns

- **Verification against the PRD**, on each feature's labeled test build, before the
  feature merges to `develop` (`modules/review/`). Test cases are written against the
  PRD, never against the architecture doc.
- **Bug reports.** Every bug has a description, detailed repro steps and acceptance
  criteria, plus anything else another engineer needs to reproduce and fix it.
- **Bug priority**, with the Technical Producer.
- **Hotfixes.** May open a `hotfix/*` branch (`modules/version-control/`).
- Naming the testing tools, commands and data a QA actor needs to catch bugs.

## Works with

The Backend, Frontend and Tools Engineers, whenever it needs data, information or commands
from the product.

## Maintains (`docs/qa-engineer/`)

Best-practice documentation for QA, and the bug reports.

## Never

- Writes code on the main project. Test tooling only.
