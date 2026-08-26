"""Q9 model assignment resolver (closes Sortie 1 finding #4).

Doctrine law (config/models.toml): policy lines are law, pool is data.
`model` slots are BINDING — resolved here at sortie open from the pool tiers,
and every resolved id is stamped into sortie metadata. Nothing here hardcodes
a model id; ids come only from the pinned doctrine's pool table.

Degraded caveat: with an Anthropic-only pool the verifier's
different-provider-family policy is unsatisfiable. The resolver falls back to
the strongest model that differs from the worker's and records the unmet
policy in the resolution notes — the gap is stamped, never hidden.
"""

import os
import re

from . import config


def _parse_toml_sections(text: str) -> dict:
    """Sectioned TOML subset: strings, numbers, and flat string arrays.
    (py3.10 on this box has no tomllib.)"""
    sections, current = {}, ""
    for line in text.splitlines():
        stripped = line.split("#", 1)[0].strip()
        if not stripped:
            continue
        m = re.match(r"^\[([A-Za-z0-9_.]+)\]$", stripped)
        if m:
            current = m.group(1)
            sections.setdefault(current, {})
            continue
        m = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$", stripped)
        if not m:
            continue
        key, raw = m.group(1), m.group(2).strip()
        if raw.startswith("[") and raw.endswith("]"):
            sections.setdefault(current, {})[key] = re.findall(r'"([^"]*)"', raw)
        elif raw.startswith('"') and raw.endswith('"'):
            sections.setdefault(current, {})[key] = raw[1:-1]
        elif re.fullmatch(r"-?[\d_]+", raw):
            sections.setdefault(current, {})[key] = int(raw.replace("_", ""))
        else:
            sections.setdefault(current, {})[key] = raw
    return sections


def _family(pool_id: str) -> str:
    """Family = provider prefix ('anthropic/claude-x' -> 'anthropic')."""
    return pool_id.split("/", 1)[0]


def _cli_id(pool_id: str) -> str:
    """The id the degraded model layer (claude CLI --model) accepts."""
    return pool_id.split("/", 1)[1] if "/" in pool_id else pool_id


class PoolEmpty(RuntimeError):
    """The pinned doctrine's pool table has no entries — a sortie may not
    resolve seats against it. Merge the pool PR (or pin a commit that has
    one) before flying."""


def resolve_seats(repo: str = config.DOCTRINE_REPO) -> dict:
    """Resolve the binding slots from the pinned doctrine's pool tiers.

    Returns {"producer"|"verifier"|"worker": {"id", "cli", "policy"},
             "notes": [...]} — stamp the whole dict into sortie metadata.
    """
    with open(os.path.join(repo, "config", "models.toml")) as f:
        toml = _parse_toml_sections(f.read())
    pool = toml.get("pool", {})
    strong = pool.get("strong") or []
    cheap = pool.get("cheap") or []
    if not strong:
        raise PoolEmpty(f"models.toml pool has no 'strong' tier at pin "
                        f"{config.doctrine_hash(repo)[:8]}")

    notes = []
    producer = strong[0]                          # strongest-available, always
    worker = cheap[0] if cheap else strong[-1]    # cheap-capable-swappable
    if not cheap:
        notes.append("no cheap tier in pool; worker falls back to weakest strong")

    # Verifier: strong + different family than the judged worker (7c).
    foreign = [m for m in strong if _family(m) != _family(worker)]
    if foreign:
        verifier = foreign[0]
    else:
        # Degraded fallback — stamped, never hidden.
        different = [m for m in strong if m != worker]
        verifier = different[0] if different else strong[0]
        notes.append("decorrelation UNMET (degraded): pool has no foreign "
                     "family; verifier = strongest different model")

    def seat(pool_id, policy):
        return {"id": pool_id, "cli": _cli_id(pool_id),
                "policy": toml.get(policy, {}).get("policy", "")}

    return {"producer": seat(producer, "producer"),
            "verifier": seat(verifier, "verifier"),
            "worker": seat(worker, "worker"),
            "notes": notes}
