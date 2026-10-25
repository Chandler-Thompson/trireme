# Roles

One role file per seat. A seat is a role a project can staff; the agent filling it is
whatever harness and model the project assigns (`modules/seats/`). Seats are long-lived:
one session per seat keeps the context it has built up.

Role files are `AGENTS.md` files, the plain-Markdown agent briefing many harnesses read,
so a seat's briefing does not depend on one vendor. Trireme gives each project seat two
files, in this order:

1. **`roles/AGENTS.md`**, what every seat shares: the rules every seat follows, the shape
   of a turn, the task log, the board, version control, the gate, spend and writing.
2. **`roles/<seat>/AGENTS.md`**, what is that seat's alone: mission, what it owns, what it
   never does, its procedures, its docs, where it escalates, and an example turn.

The XO holds no project seat, so it gets only `roles/xo/AGENTS.md`. The user is not a
seat; `user.md` describes the user for the seats.

The roster is open. Projects staff the subset they need, and new seats can be added to
this default when a project shows the need (`modules/staffing/`). A new seat needs its own
`roles/<seat>/AGENTS.md`, in the same shape as the others.

| Seat | Role file | Writes main application code |
|---|---|---|
| User | `user.md` | no (may, as a human) |
| XO | `xo/AGENTS.md` | no |
| Technical Producer | `technical-producer/AGENTS.md` | no |
| Technical Lead | `technical-lead/AGENTS.md` | no |
| Technical Artist | `technical-artist/AGENTS.md` | no |
| Backend Engineer | `backend-engineer/AGENTS.md` | yes, server side |
| Frontend Engineer | `frontend-engineer/AGENTS.md` | yes, client side |
| Engine Engineer | `engine-engineer/AGENTS.md` | yes, the core (rules, simulation) |
| Tools Engineer | `tools-engineer/AGENTS.md` | no, side tools only |
| DevOps Engineer | `devops-engineer/AGENTS.md` | no, CI/CD tooling only |
| QA Engineer | `qa-engineer/AGENTS.md` | no, test tooling only |
| Security Engineer | `security-engineer/AGENTS.md` | no, security tooling only |
| Infrastructure Engineer | `infrastructure-engineer/AGENTS.md` | no, infrastructure code only |
| Data Engineer/Analyst | `data-engineer/AGENTS.md` | no |

**Engineer seats** are every seat with "Engineer" in its title. They are the seats that
approve the architecture doc (`modules/documentation/`).
