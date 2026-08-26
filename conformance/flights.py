"""The three certification flights as scenario data (CONTRACTS/conformance/).

Stub scripts hold ONLY the flights' known answers — every lifecycle decision
(message emission, attempt counting, charge attribution, merge recording) is
made by the real harness plumbing under test.
"""

WORKER_MODEL = {"cost_usd": 0.01, "latency_ms": 1200, "model": "stub-worker",
                "provider": "stub", "tokens_in": 1000, "tokens_out": 500}
VERIFIER_MODEL = {"cost_usd": 0.02, "latency_ms": 1500, "model": "stub-verifier",
                  "provider": "stub", "tokens_in": 2000, "tokens_out": 300}
PRODUCER_MODEL = {"cost_usd": 0.03, "latency_ms": 1800, "model": "stub-producer",
                  "provider": "stub", "tokens_in": 3000, "tokens_out": 800}
TOOL_TESTS_GREEN = {"latency_ms": 300, "ok": True, "tool": "test_runner"}

FAIL_EVIDENCE = "tests/test_foo.py::test_bar FAILED"
CLARIFY_Q = ("Spec says wrap-around at grid edge; acceptance test asserts clamp. "
             "Which is law?")

TASK_SHAPE = dict(
    title="conformance task",
    description="known-answer flight task",
    files=["src/foo.py", "tests/test_foo.py"],
    deps=[],
    phase="phase-1",
)

FLIGHTS = {
    "flight-01-happy-path": {
        "task_id": "T-001",
        "worker": [
            ("model", WORKER_MODEL),
            ("tool", TOOL_TESTS_GREEN),
            ("done", "tests green"),
        ],
        "verifier": [
            ("model", VERIFIER_MODEL),
            ("pass", 0),
        ],
        "producer": [],
    },
    "flight-02-rejection-ladder": {
        "task_id": "T-002",
        "worker": [
            ("model", WORKER_MODEL),
            ("tool", TOOL_TESTS_GREEN),
            ("done", "tests green"),
            ("model", WORKER_MODEL),
            ("tool", TOOL_TESTS_GREEN),
            ("done", "tests green"),
        ],
        "verifier": [
            ("model", VERIFIER_MODEL),
            ("fail", FAIL_EVIDENCE),
            ("model", VERIFIER_MODEL),
            ("fail", FAIL_EVIDENCE),
        ],
        "producer": [],
    },
    "flight-03-clarify": {
        "task_id": "T-003",
        "worker": [
            ("model", WORKER_MODEL),
            ("clarify", CLARIFY_Q),
            ("model", WORKER_MODEL),
            ("tool", TOOL_TESTS_GREEN),
            ("done", "tests green"),
        ],
        "verifier": [
            ("model", VERIFIER_MODEL),
            ("pass", 0),
        ],
        "producer": [
            ("model", PRODUCER_MODEL),
            ("answer", "Clamp is law; the spec line is stale."),
        ],
    },
}
