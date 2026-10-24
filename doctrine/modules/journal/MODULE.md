# Journal

- **Module:** journal
- **Version:** 1
- **Governs:** the event record every other module writes to.
- **Depends on:** nothing.
- **Settings:** none yet.

The journal is the instance's single event record. The node graph, spend tracking, the
blockers queue and the XO's answers all read from it. Doctrine v1 keeps the journal and
drops v0's required metric views and conformance flights; a metric view comes back when
the user wants a specific one.

## Who writes it

Machinery: the Trireme server, the board integration and the harness adapters. A seat's
task log is its own narrative and lives in the repo (`modules/seats/`); the journal is
never written by a seat about itself.

## Envelope

Every event has:

| Field | Notes |
|---|---|
| `ts` | ISO-8601 UTC |
| `project_id` | The project, which is also the fleet |
| `doctrine_version` | The doctrine and module versions in force |
| `seat` | The seat id, `xo`, `user`, or `system` |
| `ticket_id` | The current ticket, or null |
| `type` | From the registry below |
| `payload` | Typed per event |

## Registry

| Group | Events |
|---|---|
| fleet | `fleet.started`, `fleet.paused {reason: user \| usage_window \| project_cap}`, `fleet.resumed`, `fleet.stopped`, `fleet.archived` |
| seat | `seat.started`, `seat.compacted`, `seat.handoff`, `seat.stopped`, `seat.error {reason}` |
| staffing | `staffing.requested`, `staffing.accepted`, `staffing.declined`, `staffing.retired` |
| ticket | `ticket.created {estimate}`, `ticket.started`, `ticket.done`, `ticket.reopened` |
| board items | `consult.opened`, `consult.answered`, `directive.posted`, `blocker.opened`, `blocker.answered` |
| review | `review.pass`, `review.fail {evidence}`, `security.hold`, `security.released` |
| version control | `merge.done {from, into, commit}`, `merge.refused {reason}`, `prd_deviation.flagged`, `release.cut`, `release.shipped`, `hotfix.opened` |
| documents | `prd.changed`, `prd.accepted`, `architecture.approved {seat}`, `adr.written`, `ruling.logged`, `role_ambiguity.logged` |
| spend | `model.call {harness, model, tokens_in, tokens_out, cost_usd}` |
| settings | `doctrine.changed`, `project_settings.changed` |

Adding an event type or changing a payload field is a change to this module.
