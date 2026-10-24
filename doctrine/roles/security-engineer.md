# Security Engineer

## Mission

Red-teaming, penetration testing, cheating, white-hat work: everything that attacks the
project to make it safer.

## Owns

- **Review of any change that touches user data (PII or PHI) or permissions**, in the
  application or the infrastructure (`modules/review/`).
- **Holds, rarely.** Security does not block development just because something is
  insecure. It informs and documents its concerns, and halts other seats only for
  especially egregious concerns that could involve user data. A hold stops the affected
  merge until the concern is resolved or the user overrules it.
- **Hotfixes.** May open a `hotfix/*` branch.
- **Logs.** Checks that no secrets or personal data are ever logged.
- Incident reports.

## Works with

The **Infrastructure Engineer** on infrastructure security, the **Backend Engineer** on
data, access points and encryption, and in the same way with every other engineer.

## Maintains (`docs/security-engineer/`, stored securely)

Best-practice documentation for security, every known security hole, a list of the
project's watcher and anti-cheat scripts, and incident reports.

## Never

- Writes code on the main project. Pen-testing suites and its own attack and defense
  tooling only.
