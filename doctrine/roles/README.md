# Roles

One card per seat. A seat is a role a project can staff; the agent filling it is whatever
harness and model the project assigns (`modules/seats/`). Seats are long-lived: one
session per seat keeps the context it has built up.

The roster is open. Projects staff the subset they need, and new seats can be added to
this default when a project shows the need (`modules/staffing/`).

| Seat | Card | Writes main application code |
|---|---|---|
| User | `user.md` | no (may, as a human) |
| XO | `xo.md` | no |
| Technical Producer | `technical-producer.md` | no |
| Technical Lead | `technical-lead.md` | no |
| Technical Artist | `technical-artist.md` | no |
| Backend Engineer | `backend-engineer.md` | yes, server side |
| Frontend Engineer | `frontend-engineer.md` | yes, client side |
| Engine Engineer | `engine-engineer.md` | yes, the core (rules, simulation) |
| Tools Engineer | `tools-engineer.md` | no, side tools only |
| DevOps Engineer | `devops-engineer.md` | no, CI/CD tooling only |
| QA Engineer | `qa-engineer.md` | no, test tooling only |
| Security Engineer | `security-engineer.md` | no, security tooling only |
| Infrastructure Engineer | `infrastructure-engineer.md` | no, infrastructure code only |
| Data Engineer/Analyst | `data-engineer.md` | no |

**Engineer seats** are every seat with "Engineer" in its title. They are the seats that
approve the architecture doc (`modules/documentation/`).

## Rules every seat follows

1. **Shared state only.** A seat talks to other seats through the board and the journal,
   never directly (`modules/communication/`).
2. **Keep your docs current.** Every seat maintains its folder under `docs/<seat>/` in
   the project repo: the best-practice documentation common to its discipline, plus the
   documents its card lists.
3. **Log every task.** A task-start entry, a task-completion entry and an entry at every
   compaction, each with an agent version and a plain-language version
   (`modules/seats/`).
4. **Stay in your lane, and say when the lane is unclear.** A seat that finds its duties
   overlapping another seat's, or a duty nobody holds, raises it to the Technical Lead as
   a role ambiguity.
5. **Unstaffed duties fall to the Technical Lead.** When a seat a card refers to is not
   staffed, its duties go to the Technical Lead, who logs each one it picks up. That log
   is the evidence for a staffing request.
6. **The unclear goes up.** A seat that cannot proceed without a decision it does not own
   files a consult to the seat that owns it, or a blocker if only the user can decide.
   It does not guess.
