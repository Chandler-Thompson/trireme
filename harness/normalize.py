"""Ledger normalization — exactly CONTRACTS/CONFORMANCE.md's five rules:

1. Each distinct ts, in order of first appearance -> <T0>, <T1>, ...
2. sortie_id -> <SORTIE>
3. doctrine_hash -> <HASH>
4. Each distinct commit hash in payloads, in order of first appearance -> <COMMIT-1>, ...
5. JSON canonicalized: keys sorted at every level, compact separators,
   UTF-8, one event per line, \n line endings.
"""

import json
import re

_COMMIT_RE = re.compile(r"\b[0-9a-f]{40}\b")


def normalize(jsonl_text: str) -> str:
    events = [json.loads(line) for line in jsonl_text.splitlines() if line.strip()]

    ts_map = {}
    commit_map = {}

    def map_ts(ts: str) -> str:
        if ts not in ts_map:
            ts_map[ts] = f"<T{len(ts_map)}>"
        return ts_map[ts]

    def map_commits_in(value):
        if isinstance(value, str):
            def sub(m):
                h = m.group(0)
                if h not in commit_map:
                    commit_map[h] = f"<COMMIT-{len(commit_map) + 1}>"
                return commit_map[h]
            return _COMMIT_RE.sub(sub, value)
        if isinstance(value, list):
            return [map_commits_in(v) for v in value]
        if isinstance(value, dict):
            return {k: map_commits_in(v) for k, v in value.items()}
        return value

    out_lines = []
    for ev in events:
        ev["ts"] = map_ts(ev["ts"])
        ev["sortie_id"] = "<SORTIE>"
        ev["doctrine_hash"] = "<HASH>"
        ev["payload"] = map_commits_in(ev["payload"])
        out_lines.append(json.dumps(ev, separators=(",", ":"), sort_keys=True,
                                    ensure_ascii=False))
    return "\n".join(out_lines) + "\n"
