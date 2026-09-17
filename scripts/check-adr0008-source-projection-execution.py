#!/usr/bin/env python3
import json
import pathlib
import sys
from collections import Counter

ROOT = pathlib.Path(__file__).resolve().parent.parent
REVISION = "5d392f9710225bd1761af67fd9d851c2ff4b2f82"
MATRIX = ROOT / "qualification/adr0008-source-projection-matrix.v1.json"
EXECUTION = ROOT / f"qualification/adr0008-source-projection-matrix.execution.{REVISION}.json"
LEGAL = ("PASS", "FAIL", "BLOCKED", "NOT_RUN")
CELLS = (
    [f"C{i:02d}" for i in range(1, 7)]
    + [f"R{i:02d}" for i in range(1, 5)]
    + [f"L{i:02d}" for i in range(1, 5)]
    + [f"I{i:02d}" for i in range(1, 8)]
    + [f"X{i:02d}" for i in range(1, 3)]
)
TRACKS = [
    "COMMON_PROJECTION",
    "RETAINED_OBJECT_RESOLVER",
    "LIVE_SESSION_RESOLVER",
    "ADR0007_INTEROPERABILITY",
    "COMPATIBILITY",
]
INVARIANTS = {
    "graph_facts_added": 0,
    "semantic_authority": 0,
    "semantic_accepted": False,
    "source_graph_complete": "UNKNOWN",
    "canonical_operation_count": 41,
    "compact_advertised_tool_count": 12,
    "operation_44": "FORBIDDEN",
    "schema_change_policy": "ADDITIVE_ONLY",
}


def fail(message: str) -> None:
    raise SystemExit(f"FAIL ADR0008-EXECUTION: {message}")


def main() -> None:
    matrix = json.loads(MATRIX.read_text())
    execution = json.loads(EXECUTION.read_text())
    if matrix.get("schema_version") != "lsp-trace.private.adr0007-adr0008-interoperability-qualification-matrix.v1":
        fail("governing matrix identity changed")
    if matrix.get("global_invariants") is None:
        fail("governing matrix global invariants missing")
    for key, expected in INVARIANTS.items():
        if matrix["global_invariants"].get(key) != expected:
            fail(f"matrix invariant {key} changed")
        if execution.get("global_invariants", {}).get(key) != expected:
            fail(f"execution invariant {key} changed")
    if execution.get("reviewed_revision") != REVISION:
        fail("reviewed revision mismatch")
    if execution.get("matrix") != "qualification/adr0008-source-projection-matrix.v1.json":
        fail("matrix reference mismatch")
    if execution.get("implementation_qualified") is not False:
        fail("implementation must remain unqualified")
    if execution.get("tests_pass_claimed") is not False:
        fail("partial tests must not become an aggregate pass claim")
    cells = execution.get("cells", [])
    ids = [cell.get("id") for cell in cells]
    if ids != CELLS:
        fail(f"cell IDs/order mismatch: {ids}")
    verdicts = [cell.get("verdict") for cell in cells]
    if any(verdict not in LEGAL for verdict in verdicts):
        fail("illegal verdict")
    observed = Counter(verdicts)
    counts = {verdict: observed[verdict] for verdict in LEGAL}
    if execution.get("verdict_counts") != counts:
        fail(f"verdict counts mismatch: {counts}")
    if counts != {"PASS": 0, "FAIL": 1, "BLOCKED": 22, "NOT_RUN": 0}:
        fail(f"unexpected qualification promotion: {counts}")
    if [cell["id"] for cell in cells if cell["verdict"] == "FAIL"] != ["L04"]:
        fail("L04 must remain the sole qualifying FAIL")
    if [track.get("id") for track in execution.get("tracks", [])] != TRACKS:
        fail("track set/order mismatch")
    if any(track.get("status") == "PASS" for track in execution["tracks"]):
        fail("no track is qualified")
    print(f"PASS ADR0008-EXECUTION revision={REVISION} cells=23 PASS=0 FAIL=1 BLOCKED=22 NOT_RUN=0 implementation_qualified=false")


if __name__ == "__main__":
    main()
