"""Seat drivers — the model/tool layer behind each seat.

Two implementations:
- StubSeatDriver: deterministic scripted stand-in for conformance flights
  (CONTRACTS/CONFORMANCE.md — "no live model anywhere in the loop"; a stub
  asked for an answer its script doesn't cover FAILS LOUDLY).
- Live seats: see live_seats.py / claude_cli.py (headless Claude Code).

A driver never writes the ledger. It returns structured actions; the
lifecycle engine hands the telemetry to the emitter (machinery-only emission).

Action vocabulary a work() call yields, in order:
  ("model", payload)   — a model call happened; payload per ENVELOPE model.call
  ("tool",  payload)   — a tool call happened;  payload per ENVELOPE tool.call
  ("clarify", question) — worker escalates; engine blocks the task and returns
                          control after the Producer answers
  ("done", evidence)   — work finished, evidence string for the SITREP
Verifier judge() yields ("model", payload) actions then returns
  ("pass", spec_concern) or ("fail", evidence).
Producer answer_clarify() yields ("model", payload) actions then returns answer text.
"""

class StubExhausted(Exception):
    """The harness asked the stub something the flight script doesn't cover —
    a conformance failure by definition, never a skip."""


class StubSeatDriver:
    def __init__(self, script: list):
        """script: list of action tuples, consumed strictly in order."""
        self._script = list(script)

    def next_actions(self, until_terminal: bool = True):
        """Yield actions until (and including) the first terminal action
        (clarify / done / pass / fail / answer)."""
        while True:
            if not self._script:
                raise StubExhausted("stub script exhausted — harness went off-script")
            action = self._script.pop(0)
            yield action
            if action[0] in ("clarify", "done", "pass", "fail", "answer"):
                return
