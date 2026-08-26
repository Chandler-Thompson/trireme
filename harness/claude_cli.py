"""Headless Claude Code invocation — the degraded harness's model layer.

Uses `claude -p --output-format json`, which returns a structured result
envelope with machinery-measured usage (tokens, cost, API latency). Emitting
model.call from that envelope satisfies the emission law: the harness reads
the provider's own accounting; the agent self-reports nothing.

Every session's raw envelope is written to a transcript file — a failed seat
must always be diagnosable after the fact.
"""

import json
import re
import subprocess

_JSON_BLOCK = re.compile(r"\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}", re.DOTALL)


class SeatSessionError(RuntimeError):
    def __init__(self, msg, transcript_path=None):
        super().__init__(msg + (f" (transcript: {transcript_path})" if transcript_path else ""))
        self.transcript_path = transcript_path


class SeatWallTimeout(SeatSessionError):
    """The session hit the wall-time budget and was killed by the harness.
    The engine converts this into budget.cap_hit {kind: wall}."""


def run_headless(prompt: str, system_prompt: str, cwd: str, transcript_path: str,
                 model: str | None = None, timeout_s: int = 3600,
                 allowed_tools: list | None = None) -> tuple[dict, dict]:
    """Run one headless turn. Returns (seat_json, model_call_payload).

    seat_json: the single JSON object the seat was instructed to end with.
    model_call_payload: ENVELOPE.md model.call payload from measured usage.

    Permissions are least-privilege per seat: acceptEdits (edits confined to
    the task worktree cwd) plus an explicit allowlist of the few Bash commands
    the seat's job requires — never a blanket permission bypass.
    """
    cmd = ["claude", "-p", prompt,
           "--append-system-prompt", system_prompt,
           "--output-format", "json",
           "--permission-mode", "acceptEdits"]
    if allowed_tools:
        cmd += ["--allowedTools", ",".join(allowed_tools)]
    if model:
        cmd += ["--model", model]

    try:
        out = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True,
                             timeout=timeout_s)
    except subprocess.TimeoutExpired as e:
        # A killed seat must still be diagnosable: write whatever it produced.
        with open(transcript_path, "w", encoding="utf-8") as f:
            f.write((e.stdout or b"").decode("utf-8", "replace")
                    if isinstance(e.stdout, bytes) else (e.stdout or ""))
            f.write(f"\n--- harness: killed at wall-time cap ({timeout_s}s) ---\n")
        raise SeatWallTimeout(f"session killed at wall cap {timeout_s}s",
                              transcript_path)
    with open(transcript_path, "w", encoding="utf-8") as f:
        f.write(out.stdout)
        if out.stderr:
            f.write("\n--- stderr ---\n" + out.stderr)

    if out.returncode != 0:
        raise SeatSessionError(f"session exited {out.returncode}", transcript_path)
    try:
        envelope = json.loads(out.stdout)
    except json.JSONDecodeError as e:
        raise SeatSessionError(f"unparsable result envelope: {e}", transcript_path)
    if envelope.get("is_error"):
        raise SeatSessionError(f"session error: {envelope.get('result', '')[:300]}",
                               transcript_path)

    result_text = envelope.get("result") or ""
    seat_json = None
    for match in _JSON_BLOCK.findall(result_text):
        try:
            candidate = json.loads(match)
            if isinstance(candidate, dict):
                seat_json = candidate  # keep the LAST valid object
        except json.JSONDecodeError:
            continue
    if seat_json is None:
        raise SeatSessionError("no parsable seat JSON in result", transcript_path)

    usage = envelope.get("usage", {})
    model_usage = envelope.get("modelUsage") or {}
    # Primary model = the one that did the work (max output tokens). Insertion
    # order lies: the CLI fires tiny auxiliary calls on a small model first, and
    # Sortie 1's ledger misattributed 7/8 model.calls to it before this fix.
    model_name = (max(model_usage, key=lambda k: model_usage[k].get("outputTokens", 0))
                  if model_usage else (model or "claude-code-default"))
    model_call = {
        "model": model_name,
        "provider": "anthropic",
        "tokens_in": (usage.get("input_tokens", 0)
                      + usage.get("cache_creation_input_tokens", 0)
                      + usage.get("cache_read_input_tokens", 0)),
        # Fresh input = uncached + cache-CREATION; excludes cache reads, which
        # count every internal turn's re-read of the same context. This is the
        # budget-cap counter (harness/budget.py) — in the ledger so cap
        # decisions are reconstructible from it (ENVELOPE amendment, pending).
        "tokens_in_fresh": (usage.get("input_tokens", 0)
                            + usage.get("cache_creation_input_tokens", 0)),
        "tokens_out": usage.get("output_tokens", 0),
        "cost_usd": envelope.get("total_cost_usd", 0),
        "latency_ms": envelope.get("duration_api_ms", 0),
    }
    return seat_json, model_call
