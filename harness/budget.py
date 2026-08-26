"""Budget-cap enforcement (Q4 / CONTRACTS/DECOMPOSITION.md #4).

Doctrine: every task carries a token cap and a wall-time cap, stamped at
posting. Hitting either stops work, emits `budget.cap_hit
{kind: tokens|wall, charged: "producer"}` (machinery-emitted, 3b/6d), and the
task is reissued — the blowout charges the Producer's estimate, never the
Worker (ROLES/WORKER.md, ROLES/PRODUCER.md).

Token counter semantics: **fresh input + output tokens, cumulative across all
seat sessions charged to the task** (worker + verifier + producer-CLARIFY,
retries included). Cache-READ tokens are excluded — a headless session re-reads
its cached context on every internal turn, so raw tokens_in measures turn
count, not context (Sortie 1/2 ledgers: raw 1.5M–3.8M per healthy task vs
62k–259k fresh+out). Fresh+out is the counter that means what the 100k default
meant in the PR #1 redline: how much context the task actually consumed.
Live payloads carry it as `tokens_in_fresh` (ENVELOPE amendment, pending
redline); scripted stub payloads without the field fall back to
tokens_in + tokens_out.

Degraded-harness enforcement granularity (honest caveat): a `claude -p`
session cannot be interrupted mid-flight on token count, so the TOKEN cap is
checked at session boundaries. The WALL cap is hard — the remaining wall
budget is passed down as the subprocess timeout, so even a hung session dies
at the cap. A within-session token runaway is therefore bounded by the wall
cap.
"""

import time


class TokenCapExceeded(Exception):
    def __init__(self, spent: int, cap: int):
        super().__init__(f"token cap: spent {spent} of {cap}")
        self.spent, self.cap = spent, cap


class SessionCapExceeded(Exception):
    """A SINGLE session blew the per-session token budget (Principal, PR #4
    redline 2026-07-13: every session stays under the dumb-zone threshold —
    a session that needs more context than that is wasted tokens). Checked
    per model.call, BEFORE the cumulative task check: the per-call cause
    outranks the aggregate when one call trips both."""

    def __init__(self, spent: int, cap: int):
        super().__init__(f"session token cap: session spent {spent} of {cap}")
        self.spent, self.cap = spent, cap


class WallCapExceeded(Exception):
    def __init__(self, elapsed_s: float, cap_s: int):
        super().__init__(f"wall cap: {elapsed_s:.0f}s of {cap_s}s")
        self.elapsed_s, self.cap_s = elapsed_s, cap_s


class BudgetMeter:
    """Per-task budget stopwatch + token counter. Created by the engine at
    claim time from the task's stamped budgets; fed by the engine on every
    model.call it emits. clock is injectable for deterministic tests."""

    def __init__(self, budget_tokens: int, budget_wall_s: int, clock=time.monotonic,
                 session_budget_tokens: int | None = None):
        self.budget_tokens = budget_tokens
        self.budget_wall_s = budget_wall_s
        # Per-session cap (same fresh+out counter, single-session scope).
        # None = the key is not in config/fleet.toml yet — config is law, so
        # an undefined budget is lawfully unenforced, not silently disabled.
        self.session_budget_tokens = session_budget_tokens
        self._clock = clock
        self._t0 = clock()
        self.tokens_spent = 0
        self.last_session_tokens = 0

    @staticmethod
    def countable_tokens(model_call_payload: dict) -> int:
        """Fresh-in + out. Fallback for payloads without tokens_in_fresh
        (scripted stubs): tokens_in + tokens_out."""
        fresh = model_call_payload.get("tokens_in_fresh")
        if fresh is None:
            fresh = model_call_payload.get("tokens_in", 0)
        return fresh + model_call_payload.get("tokens_out", 0)

    def feed(self, model_call_payload: dict):
        self.last_session_tokens = self.countable_tokens(model_call_payload)
        self.tokens_spent += self.last_session_tokens

    def elapsed_s(self) -> float:
        return self._clock() - self._t0

    def wall_remaining_s(self) -> float:
        return self.budget_wall_s - self.elapsed_s()

    def check(self):
        """Raise if any cap is blown. Called by the engine after every seat
        session (token granularity) and before starting the next one.
        Session check first: when one call trips both the per-session and
        the cumulative cap, the per-call cause is the diagnosis."""
        if (self.session_budget_tokens is not None
                and self.last_session_tokens > self.session_budget_tokens):
            raise SessionCapExceeded(self.last_session_tokens,
                                     self.session_budget_tokens)
        if self.tokens_spent > self.budget_tokens:
            raise TokenCapExceeded(self.tokens_spent, self.budget_tokens)
        if self.elapsed_s() > self.budget_wall_s:
            raise WallCapExceeded(self.elapsed_s(), self.budget_wall_s)
