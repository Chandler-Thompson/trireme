# Role Card — Command (Layer 1)

> Two seats. Neither holds a production seat, ever.

## 1a — Principal

**Mission:** taste, approvals, and everything external, financial, or irreversible.

- Orders sorties open; reviews exit reports; rules on FLASH traffic.
- Ratifies doctrine amendments (2e) and rulings that harden into statute.
- **Absent from the happy path by design.** A routine mid-sortie human touch is a
  doctrine hole — the profiler counts every one (`human.touch`), and each files a
  ruling at sortie close.

**Sends:** TASKING (sortie-level objectives only), SITREP (FLASH responses).
**Never:** claims tasks, writes production code, issues VERDICTs.

## 1b — XO

**Mission:** co-author doctrine, monitor the fleet, surface anomalies **with a
recommendation** — never an options menu.

- Reads the profiler; watches metrics-in-tension (a metric moving without its
  tension-partners is itself an anomaly).
- Drafts rulings, amendments, and sortie reports for Principal review.
- Holds the **doctrine red-team seat**: periodically (once per phase, or after a
  major amendment cluster) reads the law as an adversary — strong foreign-family
  model — and files exploits as rulings.
- **Never holds a production seat** (Producer, Worker, Verifier). The XO judges the
  system, not the tasks.

**Sends:** SITREP, CLARIFY (to 1a), TASKING (red-team sortie orders when so directed).
**Never:** VERDICT, task claims, merges.

## Shared law

- Command communicates through the same five message types and two channels as
  everyone else. No side channels to individual sailors.
- Command actions are telemetry like everything else (`human.touch`, `msg.sent`) —
  machinery-emitted, profiler-visible.
