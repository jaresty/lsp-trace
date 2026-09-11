#!/usr/bin/env python3
"""Assertion-specific semantic mutations for the Program C Gate II v4 checker."""

from __future__ import annotations

import copy
import importlib.util
import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import Any, Callable

ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-program-c-gate-ii-v4.py"
_SPEC = importlib.util.spec_from_file_location("program_c_gate_ii_v4", CHECKER)
if _SPEC is None or _SPEC.loader is None:
    raise RuntimeError("cannot load Gate II v4 checker")
V4 = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(V4)

APPROVAL = "qualification/program-c/a-03-caller-approval.v1.json"
INVENTORY = "qualification/program-c/a-06-fixture-inventory.v1.json"
LOUVAIN = "qualification/program-c/candidates/gonum-louvain/candidate-qualification.8c99aef.receipt.json"
LEIDEN = "qualification/program-c/candidates/gonum-leiden/candidate-qualification.8c99aef.receipt.json"
LEDGER = "qualification/program-c/gate-ii-current-outcomes.v4.tsv"
RECEIPT = "qualification/program-c/gate-ii-current-outcomes.v4.4403719.receipt.json"


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, value: dict[str, Any]) -> None:
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def reseal(path: Path, field: str) -> None:
    value = read_json(path)
    value[field] = ""
    value[field] = V4.digest(json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode())
    write_json(path, value)


class Fixture:
    def __init__(self, root: Path):
        self.root = root.resolve()
        shutil.copytree(ROOT / "qualification/program-c", self.root / "qualification/program-c")
        retained = self.root / "internal/retainedcalls/testdata/frozen-v1-export.json"
        retained.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "internal/retainedcalls/testdata/frozen-v1-export.json", retained)

    def json(self, relative: str) -> dict[str, Any]:
        return read_json(self.root / relative)

    def write(self, relative: str, value: dict[str, Any], seal: str | None = None) -> Path:
        path = self.root / relative
        path.chmod(0o644)
        write_json(path, value)
        if seal:
            reseal(path, seal)
        return path


class GateIIV4Tests(unittest.TestCase):
    def test_valid_baseline(self) -> None:
        result = subprocess.run(
            ["python3", str(CHECKER), "--root", str(ROOT), "--outcomes", LEDGER, "--receipt", RECEIPT],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(result.stdout, "PROGRAM_C_GATE_II PASS=10 FAIL=0 BLOCKED=0 IMPLEMENTATION_DECISION_ALLOWED=true\n")
        print("ASSERT_GATE_II_V4_VALID_BASELINE result=PASS")

    def approval_mutation(self, name: str, change: Callable[[dict[str, Any]], None]) -> None:
        with tempfile.TemporaryDirectory(prefix="program-c-a03-") as directory:
            fixture = Fixture(Path(directory))
            value = fixture.json(APPROVAL)
            change(value)
            path = fixture.write(APPROVAL, value, "approval_sha256")
            with self.assertRaisesRegex(ValueError, "ASSERT_GATE_II_A_03"):
                V4.check_approval("A-03", path, enforce_identity=False)
            print(f"{name} result=FAIL expected=true witness=ASSERT_GATE_II_A_03")

    def test_a03_semantic_mutations(self) -> None:
        cases: list[tuple[str, Callable[[dict[str, Any]], None]]] = [
            ("ASSERT_A_03_AUTHORITY_MUTATION", lambda value: value["authority"].update(kind="POLICY_INFERRED")),
            ("ASSERT_A_03_SCOPE_MUTATION", lambda value: value["grant"]["distribution_modes"].append("source")),
            ("ASSERT_A_03_BINDING_MUTATION", lambda value: value["bindings"]["successful_qualification_receipts"][0].update(file_sha256="sha256:" + "0" * 64)),
            ("ASSERT_A_03_EXCLUSION_MUTATION", lambda value: value["exclusions"].pop()),
        ]
        for name, change in cases:
            with self.subTest(name=name):
                self.approval_mutation(name, change)

    def inventory_mutation(self, name: str, change: Callable[[dict[str, Any]], None]) -> None:
        with tempfile.TemporaryDirectory(prefix="program-c-a06-inventory-") as directory:
            fixture = Fixture(Path(directory))
            value = fixture.json(INVENTORY)
            change(value)
            path = fixture.write(INVENTORY, value)
            with self.assertRaisesRegex(ValueError, "ASSERT_GATE_II_A_06"):
                V4.check_fixture_inventory("A-06", path, enforce_identity=False)
            print(f"{name} result=FAIL expected=true witness=ASSERT_GATE_II_A_06")

    def test_a06_inventory_mutations(self) -> None:
        cases: list[tuple[str, Callable[[dict[str, Any]], None]]] = [
            ("ASSERT_A_06_CLASSES_MUTATION", lambda value: value["required_classes"].pop()),
            ("ASSERT_A_06_GRAPH_DIGEST_MUTATION", lambda value: value["fixtures"][0]["graph"].update(sha256="sha256:" + "0" * 64)),
            ("ASSERT_A_06_LIMIT_MUTATION", lambda value: value["limits"].update(maximum_nodes_per_fixture=10001)),
            ("ASSERT_A_06_SEED_MUTATION", lambda value: value.update(seed_inventory=[1])),
        ]
        for name, change in cases:
            with self.subTest(name=name):
                self.inventory_mutation(name, change)

    def receipt_mutation(self, name: str, change: Callable[[dict[str, Any]], None], relative: str = LOUVAIN) -> None:
        with tempfile.TemporaryDirectory(prefix="program-c-a06-receipt-") as directory:
            fixture = Fixture(Path(directory))
            inventory_path = fixture.root / INVENTORY
            inventory = V4.check_fixture_inventory("A-06", inventory_path, enforce_identity=False)
            value = fixture.json(relative)
            change(value)
            path = fixture.write(relative, value, "receipt_sha256")
            expected = copy.deepcopy(V4.EXPECTED_RECEIPTS[value["candidate"]["name"]])
            expected["receipt_sha256"] = value["receipt_sha256"]
            expected["file_sha256"] = V4.digest(path.read_bytes())
            with self.assertRaisesRegex(ValueError, "ASSERT_GATE_II_A_06"):
                V4.check_candidate_receipt(
                    "A-06", path, expected, inventory,
                    enforce_identity=False, verify_inputs=False,
                )
            print(f"{name} result=FAIL expected=true witness=ASSERT_GATE_II_A_06")

    def test_a06_receipt_mutations(self) -> None:
        cases: list[tuple[str, Callable[[dict[str, Any]], None], str]] = [
            ("ASSERT_A_06_MATRIX_MUTATION", lambda value: value["runs"].pop(), LOUVAIN),
            ("ASSERT_A_06_COUNT_MUTATION", lambda value: value["counts"].update(completed_runs=31), LOUVAIN),
            ("ASSERT_A_06_RAW_PARTITION_MUTATION", lambda value: value["runs"][0]["result"]["raw_candidate_output"]["communities"][0].append(value["runs"][0]["result"]["raw_candidate_output"]["communities"][0][0]), LOUVAIN),
            ("ASSERT_A_06_DIGEST_MUTATION", lambda value: value["runs"][0]["result"]["externally_canonicalized_output"].update(digest="sha256:" + "0" * 64), LOUVAIN),
            ("ASSERT_A_06_CANONICALIZATION_MUTATION", lambda value: value["runs"][0]["result"]["canonicalization"].update(empty_communities_removed=99), LEIDEN),
            ("ASSERT_A_06_RESOURCE_MUTATION", lambda value: value["runs"][0]["result"]["observation"]["resource_samples"][0].update(tree_rss_bytes=536870913), LOUVAIN),
            ("ASSERT_A_06_FAILURE_MUTATION", lambda value: value["runs"][0].update(outcome="FAIL", failure={"kind": "TIMEOUT", "detail": "counterfactual"}), LOUVAIN),
            ("ASSERT_A_06_NO_FALLBACK_MUTATION", lambda value: value["no_fallback_policy"].update(fallback=True), LOUVAIN),
        ]
        for name, change, relative in cases:
            with self.subTest(name=name):
                self.receipt_mutation(name, change, relative)


if __name__ == "__main__":
    unittest.main(verbosity=2)
