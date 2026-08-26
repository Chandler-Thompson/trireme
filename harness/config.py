"""Configuration for the degraded harness.

Reads:
- the pinned doctrine checkout (config/fleet.toml — law says config, never constants)
- the task-board credentials (referenced by location; never committed anywhere)

Both locations are overridable by environment variable so a fleet can pin a
doctrine checkout other than the one shipped in this repository:

    TRIREME_DOCTRINE=/path/to/doctrine   # default: <repo>/doctrine
    TRIREME_BOARD_ENV=/path/to/board.env # default: ~/.config/trireme/board.env

`board.env` holds two lines and never enters a repository or a ledger:

    BOARD_API_URL=http://your-board.example:3000
    BOARD_API_KEY=<personal access token>

`BOARD_API_URL` is the board's ORIGIN, with no path. `harness/board.py`
appends `/api/v1` itself; including it here produces requests to
`/api/v1/api/v1/...` and a 404 that names the doubled path.
"""

import os
import re
import subprocess

_REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

DOCTRINE_REPO = os.path.expanduser(
    os.environ.get("TRIREME_DOCTRINE", os.path.join(_REPO_ROOT, "doctrine")))
BOARD_ENV_PATH = os.path.expanduser(
    os.environ.get("TRIREME_BOARD_ENV", "~/.config/trireme/board.env"))


def doctrine_hash(repo: str = DOCTRINE_REPO) -> str:
    """The pinned doctrine commit — stamped on every event."""
    out = subprocess.run(
        ["git", "-C", repo, "rev-parse", "HEAD"],
        capture_output=True, text=True, check=True,
    )
    return out.stdout.strip()


def _parse_toml_scalars(text: str) -> dict:
    """Minimal TOML scalar parser (py3.10 has no tomllib). Handles the flat
    `key = value` lines fleet.toml uses; underscore digit separators allowed."""
    values = {}
    for line in text.splitlines():
        line = line.split("#", 1)[0].strip()
        m = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$", line)
        if not m:
            continue
        key, raw = m.group(1), m.group(2).strip()
        if re.fullmatch(r"-?[\d_]+", raw):
            values[key] = int(raw.replace("_", ""))
        elif re.fullmatch(r"-?[\d_]*\.[\d_]+", raw):
            values[key] = float(raw.replace("_", ""))
        elif raw.startswith('"') and raw.endswith('"'):
            values[key] = raw[1:-1]
        else:
            values[key] = raw
    return values


def fleet_config(repo: str = DOCTRINE_REPO) -> dict:
    with open(os.path.join(repo, "config", "fleet.toml")) as f:
        return _parse_toml_scalars(f.read())


def board_credentials(path: str = BOARD_ENV_PATH) -> dict:
    """Board URL + PAT. The PAT never enters any repo file or ledger."""
    creds = {}
    with open(path) as f:
        for line in f:
            line = line.strip()
            if "=" in line and not line.startswith("#"):
                k, v = line.split("=", 1)
                creds[k.strip()] = v.strip()
    return creds
