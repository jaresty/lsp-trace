#!/usr/bin/env python3
"""Validate the private, non-authorizing Program C Gate II outcome ledger."""

from __future__ import annotations

import argparse
from pathlib import Path

ALLOWED = {"PASS", "FAIL", "BLOCKED"}
EXPECTED = [f"A-{i:02d}" for i in range(1, 11)]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument(
        "--outcomes",
        default="qualification/program-c/gate-ii-current-outcomes.tsv",
    )
    args = parser.parse_args()
    root = Path(args.root).resolve()
    rows: list[tuple[str, str]] = []
    for number, line in enumerate((root / args.outcomes).read_text().splitlines(), 1):
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) != 4:
            raise SystemExit(f"invalid Gate II row at line {number}")
        gate, state, evidence, basis = fields
        if state not in ALLOWED or not evidence or not basis:
            raise SystemExit(f"invalid Gate II fields at line {number}")
        for relative in evidence.split(";"):
            path = root / relative
            if not path.is_file() or not path.resolve().is_relative_to(root):
                raise SystemExit(f"invalid Gate II evidence path: {relative}")
        rows.append((gate, state))
    if [gate for gate, _ in rows] != EXPECTED:
        raise SystemExit("Gate II must contain exactly ordered A-01 through A-10")
    counts = {state: sum(value == state for _, value in rows) for state in sorted(ALLOWED)}
    allowed = counts["FAIL"] == 0 and counts["BLOCKED"] == 0 and counts["PASS"] == 10
    print(
        "PROGRAM_C_GATE_II "
        f"PASS={counts['PASS']} FAIL={counts['FAIL']} BLOCKED={counts['BLOCKED']} "
        f"IMPLEMENTATION_DECISION_ALLOWED={str(allowed).lower()}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
