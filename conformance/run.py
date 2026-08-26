#!/usr/bin/env python3
"""Conformance runner — certifies THIS harness against the doctrine's
scripted known-answer flights (CONTRACTS/CONFORMANCE.md).

Real board service, real emitter, real lifecycle engine; stubbed model and
tool layers only. Emitted ledger is normalized and compared BYTE-EXACT to
the doctrine's expected ledgers. Any diff = not certified.

Usage: python3 conformance/run.py
Exit 0 = certified (all flights byte-identical); 1 = any diff.
"""

import difflib
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from harness import config
from harness.board import Board
from harness.emitter import Emitter
from harness.lifecycle import Engine, StubMergeGate, TaskSpec
from harness.normalize import normalize
from harness.seats import StubSeatDriver
from conformance.flights import FLIGHTS, TASK_SHAPE

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "out")


def run_flight(name: str, flight: dict, creds: dict, dhash: str, fleet: dict) -> bool:
    os.makedirs(OUT_DIR, exist_ok=True)
    ledger_path = os.path.join(OUT_DIR, f"{name}.jsonl")
    if os.path.exists(ledger_path):
        os.remove(ledger_path)

    sortie_id = f"conf-{name}-{int(time.time())}"
    emitter = Emitter(sortie_id, dhash, ledger_path)
    board = Board(creds["BOARD_API_URL"], creds["BOARD_API_KEY"], emitter,
                  project_name=f"conformance-{sortie_id}")
    engine = Engine(board, emitter, fleet, StubMergeGate())

    spec = TaskSpec(flight["task_id"], phase=TASK_SHAPE["phase"],
                    title=TASK_SHAPE["title"], description=TASK_SHAPE["description"],
                    files=TASK_SHAPE["files"], deps=TASK_SHAPE["deps"])

    engine.open_sortie("phase-1")
    engine.post(spec)
    engine.run_task(
        spec,
        worker=StubSeatDriver(flight["worker"]),
        verifier=StubSeatDriver(flight["verifier"]),
        producer=StubSeatDriver(flight["producer"]),
    )
    engine.close_sortie()

    # cleanup board artifacts (conformance never enters sortie telemetry)
    try:
        wft_id = board._tasks.get(flight["task_id"])
        if wft_id:
            board._req("DELETE", f"/tasks/{wft_id}")
        board._req("DELETE", f"/projects/{board.project_id}")
    except Exception as e:  # cleanup failure is not a certification failure
        print(f"  (cleanup warning: {e})")

    with open(ledger_path) as f:
        emitted = normalize(f.read())
    expected_path = os.path.join(config.DOCTRINE_REPO, "CONTRACTS", "conformance",
                                 name, "expected-ledger.jsonl")
    with open(expected_path) as f:
        expected = f.read()

    if emitted == expected:
        print(f"PASS  {name} (byte-identical, {len(emitted.splitlines())} events)")
        return True
    print(f"FAIL  {name}")
    for line in difflib.unified_diff(expected.splitlines(), emitted.splitlines(),
                                     "expected", "emitted", lineterm="", n=1):
        print("  " + line)
    return False


def main():
    creds = config.board_credentials()
    dhash = config.doctrine_hash()
    fleet = config.fleet_config()
    print(f"doctrine pin: {dhash}")
    print(f"board: {creds['BOARD_API_URL']}")
    ok = all(run_flight(n, f, creds, dhash, fleet) for n, f in FLIGHTS.items())
    print("\nCERTIFIED" if ok else "\nNOT CERTIFIED")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
