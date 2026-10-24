# Version Control

- **Module:** version-control
- **Version:** 1
- **Governs:** branches, merges into them, test builds and hotfixes.
- **Depends on:** review (the gates name its approvals), journal.
- **Settings:** none yet.

This module is deliberately self-contained so that a project can replace it wholesale
with a different branching model.

## Branches (Git-Flow)

| Branch | Lives | Comes from | Merges into |
|---|---|---|---|
| `main` | forever | | |
| `develop` | forever | `main` | `release/*` is cut from it |
| `feature/<name>` | until merged | `develop` | `develop` |
| `task/<ticket>` | until merged | its feature branch | its feature branch |
| `release/<version>` | until merged | `develop` | `main` and back into `develop` |
| `hotfix/<name>` | until merged | `main` | `main` and `develop` |

`main` is what the PRD rule calls the main build. **`main` and `develop` are always
green.** A red `main` or `develop` stops all merges into it until it is green again, and
raises a blocker.

## Merge gates

### Task → feature

1. The mechanical gate passes (`modules/review/`): tests, coverage, lint, standards
   checks, file scope.
2. Every changed path is in the ticket's declared file scope, judged on the diff from the
   merge-base. An undeclared path refuses the merge and returns the task to the seat that
   did it.

### Feature → `develop`

1. Every task in the feature is merged into the feature branch.
2. The mechanical gate passes on the feature branch.
3. **The Technical Lead approves the code.**
4. **The QA Engineer passes the feature against the PRD on its labeled test build.**
5. Security approves, when the feature touches user data or permissions.
6. Rebase onto current `develop`, run the full suite again, merge only if green.

A feature that departs from the PRD may merge to `develop` with a **PRD deviation** flag,
so the implementers can prove their solution works before the user rules on it. A flagged
feature cannot enter a release without the user's approval.

### `develop` → `release/<version>`

The Technical Producer proposes the release's contents. The branch is cut from a green
`develop`. Only fixes to the release's own features land on it after that.

### `release/<version>` → `main`

1. The full suite passes on the release branch.
2. QA passes the release candidate's labeled build.
3. **The user approves**, including any PRD deviation the release contains.
4. Merge to `main`, tag the version, merge back into `develop`.

### `hotfix/<name>`

Opened from `main` by the QA Engineer or the Security Engineer. Goes through the
mechanical gate and the Technical Lead's review, then **the user approves**. Merges into
both `main` and `develop`.

## Test builds

DevOps produces a labeled test build for every feature branch and every release
candidate. The label says what the build contains, so any actor, agent or human, can see
at a glance what needs testing and test as few things as possible in each build.

## Records

Every merge, refusal, deviation flag and approval is a journal event
(`modules/journal/`).
