"""Telemetry emitter (6d) — the ONLY writer of ledger events.

Emission law (CONTRACTS/ENVELOPE.md): machinery-only emission, no agent
self-reporting, ever. Seats hand structured results to the harness; the
harness calls this emitter.

Envelope: ts, sortie_id, doctrine_hash, seat, task_id, type, synthetic, payload.
Storage: JSONL, one file per sortie (layer 5b).
"""

import datetime
import json


class Emitter:
    def __init__(self, sortie_id: str, doctrine_hash: str, ledger_path: str):
        self.sortie_id = sortie_id
        self.doctrine_hash = doctrine_hash
        self.ledger_path = ledger_path
        self._last_ts = None

    def _now(self) -> str:
        """ISO-8601 UTC, strictly monotonic per event so normalization's
        distinct-ts mapping (<T0>, <T1>, ...) is stable."""
        ts = datetime.datetime.now(datetime.timezone.utc)
        stamp = ts.isoformat(timespec="microseconds")
        if stamp == self._last_ts:
            ts += datetime.timedelta(microseconds=1)
            stamp = ts.isoformat(timespec="microseconds")
        self._last_ts = stamp
        return stamp

    def emit(self, seat: str, task_id, type_: str, payload: dict, synthetic: int = 0):
        event = {
            "ts": self._now(),
            "sortie_id": self.sortie_id,
            "doctrine_hash": self.doctrine_hash,
            "seat": seat,
            "task_id": task_id,
            "type": type_,
            "synthetic": synthetic,
            "payload": payload,
        }
        with open(self.ledger_path, "a", encoding="utf-8") as f:
            f.write(json.dumps(event, separators=(",", ":"), sort_keys=True) + "\n")
        return event
