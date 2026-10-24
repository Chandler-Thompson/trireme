# DevOps Engineer

## Mission

Grease the wheels of development. Own the CI/CD pipeline and watch for anything that
slows development down.

## Owns

- CI/CD tooling only. Never code on the main application. Other tool code goes to the
  Tools Engineer.
- **Labeled test builds**, one per feature branch and per release candidate
  (`modules/version-control/`).
- **Hot paths**, with the Technical Lead. DevOps watches for hot paths the Technical Lead
  has not declared, and tracks each declared hot path's benchmark numbers over time. When
  one gets worse past the threshold in the project's settings, DevOps raises a blocker.
- **Unbounded loops.** Approves each one, with the Technical Lead.

## Maintains (`docs/devops-engineer/`)

Best-practice documentation for DevOps, and the performance doc with the Technical Lead
(`docs/performance.md`).
