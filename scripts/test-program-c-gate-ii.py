#!/usr/bin/env python3
"""Counterfactual tests for the private Program C Gate II checker."""

from __future__ import annotations

import hashlib
import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import Callable

ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-program-c-gate-ii.py"
REVISION = "69ca49f456a7a38cf370131834a2178d9aae17fe"
VERSION = "v0.17.1-0.20260426204603-69ca49f456a7"
LEDGER = "qualification/program-c/gate-ii-current-outcomes.v3.tsv"
EVIDENCE = {
    "A-01": ["qualification/program-c/gate-i-receipts.tsv"],
    "A-02": ["qualification/program-c/candidate-comparison.v1.json"],
    "A-03": ["qualification/program-c/i-04-license-inputs.v2.json"],
    "A-04": ["qualification/program-c/a-04-current-platform-scope.v1.json", "qualification/program-c/a-04-darwin-arm64-build-reproducibility.v1.json"],
    "A-05": ["qualification/program-c/candidates/gonum-leiden/candidate-execution.af7f038.receipt.json"],
    "A-06": ["qualification/program-c/candidates/gonum-leiden/candidate-execution.af7f038.receipt.json"],
    "A-07": ["qualification/program-c/a-07-boundary-accounting-contract.v1.json"],
    "A-08": ["qualification/program-c/a-08-instability-contract.v1.json"],
    "A-09": ["qualification/program-c/i-08-neutrality-examples.v1.json", "qualification/program-c/gate-i-i-08-neutrality.receipt.txt"],
    "A-10": ["qualification/program-c/a-10-implementation-test-plan.v1.json", "qualification/program-c/a-10-implementation-test-plan.v1.review.json", "qualification/program-c/a-10-implementation-test-plan.v1.review.txt"],
}
STATES = {"A-01": "PASS", "A-02": "PASS", "A-03": "BLOCKED", "A-04": "PASS", "A-05": "PASS", "A-06": "FAIL", "A-07": "PASS", "A-08": "PASS", "A-09": "PASS", "A-10": "PASS"}


def digest(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def read_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, value: dict) -> None:
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


class Baseline:
    def __init__(self, root: Path):
        self.root = root
        for paths in EVIDENCE.values():
            for relative in paths:
                target = root / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                source = ROOT / relative
                if source.is_file():
                    shutil.copyfile(source, target)
        qualifier = root / "scripts/qualify-program-c-a-04.py"
        qualifier.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "scripts/qualify-program-c-a-04.py", qualifier)
        self._a04_receipt()
        self._a10_review()
        ledger = root / LEDGER
        ledger.parent.mkdir(parents=True, exist_ok=True)
        lines = ["# gate_id\tstate\tevidence\tbasis"]
        for gate in sorted(EVIDENCE):
            lines.append(f"{gate}\t{STATES[gate]}\t{';'.join(EVIDENCE[gate])}\ttest basis for {gate}")
        ledger.write_text("\n".join(lines) + "\n", encoding="utf-8")

    def _a04_receipt(self) -> None:
        scope_path = self.root / EVIDENCE["A-04"][0]
        scope = read_json(scope_path)
        command = ["/exact/go", "-C", "/exact/module", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", "/exact/candidate", "./cmd/candidate"]
        executable_digest = "sha256:" + "a" * 64
        receipt = {
            "schema_version": "lsp-trace.private.program-c.a-04-build-reproducibility.v1",
            "gate_id": "A-04",
            "result": "PASS",
            "repository_revision": "a" * 40,
            "qualifier": {
                "path": "scripts/qualify-program-c-a-04.py",
                "sha256": digest((self.root / "scripts/qualify-program-c-a-04.py").read_bytes()),
            },
            "scope_policy": {"path": EVIDENCE["A-04"][0], "sha256": digest(scope_path.read_bytes()), "policy_id": scope["policy_id"]},
            "candidate": {"module": "gonum.org/v1/gonum", "revision": REVISION, "pseudo_version": VERSION},
            "coordinate": {"goos": "darwin", "goarch": "arm64", "cgo_enabled": "0"},
            "identities": {"tool": {}, "source": {}, "module": {}, "command": {}, "environment": {}, "package": {}},
            "builds": [
                {"index": 0, "command": command, "outcome": "PASS", "failure": None, "executable_sha256": executable_digest},
                {"index": 1, "command": command, "outcome": "PASS", "failure": None, "executable_sha256": executable_digest},
            ],
            "typed_failure_policy": scope["reproducibility_requirement"]["typed_outcomes"],
            "network": {"goproxy": "https://proxy.golang.org", "general_fallback_allowed": False, "fallback_attempted": False},
        }
        write_json(self.root / EVIDENCE["A-04"][1], receipt)

    def _a10_review(self) -> None:
        plan_path = self.root / EVIDENCE["A-10"][0]
        raw_path = self.root / EVIDENCE["A-10"][2]
        raw_path.write_text("Independent read-only review: APPROVED; all eight classes present; no execution claim.\n", encoding="utf-8")
        plan = read_json(plan_path)
        review = {
            "schema_version": "lsp-trace.private.program-c.a-10-test-plan-review.v1",
            "result": "APPROVED",
            "read_only": True,
            "independent_process": True,
            "plan_sha256": digest(plan_path.read_bytes()),
            "raw_review_sha256": digest(raw_path.read_bytes()),
            "reviewed_commit": "b" * 40,
            "reviewer": {"tool": "independent-test-reviewer", "model": "test-double"},
            "coverage_confirmed": sorted(plan["planned_guard_classes"]),
            "execution_status_confirmed": "NOT_EXECUTED",
            "tests_pass_claimed_confirmed": False,
        }
        write_json(self.root / EVIDENCE["A-10"][1], review)

    def json(self, relative: str) -> dict:
        return read_json(self.root / relative)

    def write_json(self, relative: str, value: dict) -> None:
        write_json(self.root / relative, value)

    def run(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(["python3", str(CHECKER), "--root", str(self.root), "--outcomes", LEDGER], text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=False)


class GateIITests(unittest.TestCase):
    def baseline(self) -> tuple[tempfile.TemporaryDirectory[str], Baseline]:
        directory = tempfile.TemporaryDirectory(prefix="program-c-gate-ii-")
        return directory, Baseline(Path(directory.name))

    def test_valid_baseline(self) -> None:
        directory, baseline = self.baseline()
        with directory:
            result = baseline.run()
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertEqual(result.stdout, "PROGRAM_C_GATE_II PASS=8 FAIL=1 BLOCKED=1 IMPLEMENTATION_DECISION_ALLOWED=false\n")
            print("ASSERT_GATE_II_VALID_BASELINE result=PASS")

    def mutation(self, name: str, mutate: Callable[[Baseline], None], expected: str) -> None:
        directory, baseline = self.baseline()
        with directory:
            pristine = baseline.run()
            self.assertEqual(pristine.returncode, 0, pristine.stdout)
            mutate(baseline)
            result = baseline.run()
            self.assertNotEqual(result.returncode, 0, f"{name} unexpectedly passed")
            self.assertIn(expected, result.stdout)
            print(f"{name} result=FAIL expected=true witness={expected}")

    def test_one_mutation_semantic_rejections(self) -> None:
        def mutate_json(relative: str, change: Callable[[dict], None]) -> Callable[[Baseline], None]:
            def apply(baseline: Baseline) -> None:
                doc = baseline.json(relative)
                change(doc)
                baseline.write_json(relative, doc)
            return apply

        cases: list[tuple[str, Callable[[Baseline], None], str]] = [
            ("ASSERT_GATE_II_A_01_MUTATION", lambda b: (b.root / EVIDENCE["A-01"][0]).write_text((b.root / EVIDENCE["A-01"][0]).read_text().replace("I-01\tPASS", "I-01\tFAIL", 1)), "ASSERT_GATE_II_A_01"),
            ("ASSERT_GATE_II_A_02_MUTATION", mutate_json(EVIDENCE["A-02"][0], lambda d: d["selection"].update(revision="0" * 40)), "ASSERT_GATE_II_A_02"),
            ("ASSERT_GATE_II_A_03_MUTATION", lambda b: (b.root / LEDGER).write_text((b.root / LEDGER).read_text().replace("A-03\tBLOCKED", "A-03\tPASS", 1)), "ASSERT_GATE_II_A_03"),
            ("ASSERT_GATE_II_A_04_MUTATION", mutate_json(EVIDENCE["A-04"][1], lambda d: d["coordinate"].update(goarch="amd64")), "ASSERT_GATE_II_A_04"),
            ("ASSERT_GATE_II_A_05_MUTATION", mutate_json(EVIDENCE["A-05"][0], lambda d: d["canonical_output"].update(digest="sha256:" + "0" * 64)), "ASSERT_GATE_II_A_05"),
            ("ASSERT_GATE_II_A_06_MUTATION", lambda b: (b.root / LEDGER).write_text((b.root / LEDGER).read_text().replace("A-06\tFAIL", "A-06\tPASS", 1)), "ASSERT_GATE_II_A_06"),
            ("ASSERT_GATE_II_A_07_MUTATION", mutate_json(EVIDENCE["A-07"][0], lambda d: d["required_measures"].pop("conductance")), "ASSERT_GATE_II_A_07"),
            ("ASSERT_GATE_II_A_08_MUTATION", mutate_json(EVIDENCE["A-08"][0], lambda d: d["label_independent_matching"].update(numeric_or_text_labels_are_identities=True)), "ASSERT_GATE_II_A_08"),
            ("ASSERT_GATE_II_A_09_MUTATION", mutate_json(EVIDENCE["A-09"][0], lambda d: d["prohibited"].pop(1)), "ASSERT_GATE_II_A_09"),
            ("ASSERT_GATE_II_A_10_MUTATION", mutate_json(EVIDENCE["A-10"][0], lambda d: d.update(execution_status="EXECUTED")), "ASSERT_GATE_II_A_10"),
        ]
        for name, mutation, expected in cases:
            with self.subTest(name=name):
                self.mutation(name, mutation, expected)

    def test_unsafe_evidence_paths_rejected(self) -> None:
        def replace_evidence(old: str, new: str) -> Callable[[Baseline], None]:
            return lambda b: (b.root / LEDGER).write_text((b.root / LEDGER).read_text().replace(old, new, 1))

        self.mutation("ASSERT_GATE_II_ABSOLUTE_PATH", replace_evidence(EVIDENCE["A-01"][0], "/tmp/gate-i-receipts.tsv"), "invalid Gate II evidence path")
        self.mutation("ASSERT_GATE_II_ESCAPE_PATH", replace_evidence(EVIDENCE["A-01"][0], "../gate-i-receipts.tsv"), "invalid Gate II evidence path")

        def symlink(baseline: Baseline) -> None:
            evidence = baseline.root / EVIDENCE["A-07"][0]
            outside = baseline.root / "outside.json"
            shutil.copyfile(evidence, outside)
            evidence.unlink()
            evidence.symlink_to(outside)

        self.mutation("ASSERT_GATE_II_SYMLINK_PATH", symlink, "symlink")


if __name__ == "__main__":
    unittest.main(verbosity=2)
