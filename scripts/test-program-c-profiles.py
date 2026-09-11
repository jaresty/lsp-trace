#!/usr/bin/env python3
import copy
import hashlib
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-program-c-profiles.py"
BASE = ROOT / "qualification/program-c/projection-profiles.v1.json"
REAL_MATRIX = ROOT / "qualification/program-c/profile-qualification.tsv"
DIGEST = "sha256:" + "1" * 64


class ProgramCProfilesTest(unittest.TestCase):
    def run_checker(self, root, profiles, matrix, *extra):
        return subprocess.run(
            ["python3", str(CHECKER), "--root", str(root), "--profiles", str(profiles), "--matrix", str(matrix), *extra],
            text=True, capture_output=True,
        )

    def valid_receipt(self, profile="calls-v1", language="go", framework="none", provider="gopls", version="0.23.0"):
        profiles = {p["name"]: p for p in json.loads(BASE.read_text())["profiles"]}
        relation_names = [r["kind"] for r in profiles[profile]["relations"]]
        revision = "a" * 40
        command = ["$LSP_TRACE_BIN", "slice", "--workspace", "$WORKSPACE", "--server", "$PROVIDER"]
        return {
            "schema_version": "lsp-trace.program-c-profile-qualification-receipt.v1",
            "result": "PASS", "current": True, "repository_revision": revision,
            "selection": {"status": "CURRENT", "supersedes": None},
            "profile": {"name": profile, "logical_digest": profiles[profile]["logical_digest"], "relations": relation_names},
            "coordinate": {"language": language, "framework": framework, "provider": provider, "provider_version": version,
                           "observed_provider_identity": provider, "observed_provider_version": version},
            "policy_matrix_identity": {
                "profile_manifest": "profiles.json",
                "profile_manifest_sha256": "sha256:" + hashlib.sha256(BASE.read_bytes()).hexdigest(),
                "qualification_matrix": "matrix.tsv",
                "qualification_matrix_prepublication_sha256": DIGEST,
                "matrix_row": [profile, language, framework, provider, version, "yes", "PASS"],
            },
            "tool_identity": {"name": "lsp-trace", "revision": revision, "version_output": f"revision={revision} modified=false"},
            "exact_command": command,
            "command_bindings": {"LSP_TRACE_BIN": "/tmp/lsp-trace", "WORKSPACE": "/tmp/workspace", "PROVIDER": f"/tmp/{provider}"},
            "run_identity": {"run_id": "run-1", "generation": "generation-1"},
            "input_sha256": {"manifest": DIGEST, "qualification_matrix_prepublication": DIGEST}, "result_sha256": DIGEST,
            "relation_evidence": [
                {"relation": relation, "status": "PASS", "count": 1, "custody": "SERVER_REPORTED", "replay": "EXACT_BYTES"}
                for relation in relation_names
            ],
            "claim_ceiling": {"scope": "EXACT_COORDINATE_AND_INPUTS_ONLY", "whole_source_complete": False,
                              "source_authenticated": False, "cross_coordinate_transfer": False},
        }

    def write_case(self, root, receipt, *, status="PASS", profile=None, coordinate=None):
        profiles = root / "profiles.json"
        profiles.write_bytes(BASE.read_bytes())
        profile = profile or "calls-v1"
        coordinate = coordinate or {"language": "go", "framework": "none", "provider": "gopls", "provider_version": "0.23.0"}
        row = [profile, coordinate.get("language", "go"), coordinate.get("framework", "none"),
               coordinate.get("provider", "gopls"), coordinate.get("provider_version", "0.23.0"), "yes", status]
        if receipt.get("policy_matrix_identity", {}).get("matrix_row") is not None:
            receipt["policy_matrix_identity"]["matrix_row"] = row.copy()
        receipt_path = root / "receipt.json"
        receipt_path.write_text(json.dumps(receipt, sort_keys=True))
        row += [receipt_path.name, "sha256:" + hashlib.sha256(receipt_path.read_bytes()).hexdigest()]
        missing = [
            [name, "typescript", "ember", "ember-glint", "1.0.3", "yes", "MISSING", "-", "-"]
            for name in ("callback-flow-v1", "state-flow-v1", "ui-lifecycle-v1") if name != profile
        ]
        matrix = root / "matrix.tsv"
        matrix.write_text("# profile\tlanguage\tframework\tprovider\tprovider_version\trequired\tstatus\treceipt\treceipt_sha256\n" +
                          "\n".join(["\t".join(row)] + ["\t".join(item) for item in missing]) + "\n")
        return profiles, matrix

    def assert_rejected(self, mutate, expected="ASSERT_PROGRAM_C_PROFILE_RECEIPT"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            receipt = self.valid_receipt()
            mutate(receipt)
            profiles, matrix = self.write_case(root, receipt)
            result = self.run_checker(root, profiles, matrix)
            self.assertNotEqual(result.returncode, 0, result.stdout)
            self.assertIn(expected, result.stdout)

    def test_private_qualification_schema_contract(self):
        path = ROOT / "qualification/program-c/profile-qualification-receipt.v1.schema.json"
        schema = json.loads(path.read_text())
        self.assertEqual(schema["properties"]["schema_version"]["const"],
                         "lsp-trace.program-c-profile-qualification-receipt.v1")
        self.assertEqual(set(schema["required"]), {
            "schema_version", "result", "current", "repository_revision", "selection", "profile", "coordinate",
            "policy_matrix_identity", "tool_identity", "exact_command", "command_bindings", "run_identity",
            "input_sha256", "result_sha256", "relation_evidence", "claim_ceiling",
        })
        self.assertFalse(schema["additionalProperties"])

    def test_valid_strict_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            receipt = self.valid_receipt()
            profiles, matrix = self.write_case(root, receipt)
            result = self.run_checker(root, profiles, matrix, "--require-profile", "calls-v1")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_existing_calls_receipts_remain_admitted(self):
        result = self.run_checker(ROOT, BASE, REAL_MATRIX, "--require-profile", "calls-v1")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("legacy_compatibility=2", result.stdout)

    def test_foreign_artifacts_rejected(self):
        for schema in ("lsp-trace.b05-qualification-evidence.v3", "lsp-trace.b05-release-selection.v1",
                       "lsp-trace.provider-conformance-report.v1"):
            with self.subTest(schema=schema):
                self.assert_rejected(lambda r, s=schema: r.update(schema_version=s))

    def test_b05_generation_2_rejected(self):
        self.assert_rejected(lambda r: r.update(schema_version="lsp-trace.b05-qualification-evidence.v3", generation=2))

    def test_arbitrary_digest_matching_json_rejected(self):
        self.assert_rejected(lambda r: (r.clear(), r.update(arbitrary=True)))

    def test_identity_and_projection_mutations_rejected(self):
        mutations = [
            lambda r: r.update(schema_version="wrong"), lambda r: r.update(result="BLOCKED"),
            lambda r: r.update(current=False), lambda r: r["profile"].update(name="state-flow-v1"),
            lambda r: r["profile"].update(logical_digest=DIGEST), lambda r: r["profile"].update(relations=["UPDATES_STATE"]),
            lambda r: r["coordinate"].update(language="csharp"), lambda r: r["coordinate"].update(provider_version="9.9.9"),
            lambda r: r["coordinate"].update(observed_provider_identity="foreign-provider"),
            lambda r: r["coordinate"].update(observed_provider_version="9.9.9"),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)):
                self.assert_rejected(mutation)

    def test_absent_command_bindings_and_digests_rejected(self):
        for key in ("exact_command", "command_bindings", "input_sha256", "result_sha256"):
            with self.subTest(key=key):
                self.assert_rejected(lambda r, k=key: r.pop(k))

    def test_policy_digest_and_required_relation_fail_closed(self):
        self.assert_rejected(lambda r: r["policy_matrix_identity"].update(qualification_matrix_prepublication_sha256="bad"))
        self.assert_rejected(lambda r: r["input_sha256"].update(qualification_matrix_prepublication="sha256:" + "2" * 64))
        self.assert_rejected(lambda r: r["relation_evidence"][0].update(status="BLOCKED"))
        self.assert_rejected(lambda r: r["relation_evidence"][0].update(custody="PROVIDER_PROVED"))

    def test_stale_selection_rejected_and_current_successor_admitted(self):
        self.assert_rejected(lambda r: r["selection"].update(status="STALE"))
        self.assert_rejected(lambda r: r["selection"].update(status="SUPERSEDED", supersedes=DIGEST))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            receipt = self.valid_receipt()
            receipt["selection"]["supersedes"] = DIGEST
            profiles, matrix = self.write_case(root, receipt)
            result = self.run_checker(root, profiles, matrix)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_unresolved_or_extra_command_binding_rejected(self):
        self.assert_rejected(lambda r: r["command_bindings"].pop("PROVIDER"))
        self.assert_rejected(lambda r: r["command_bindings"].update(EXTRA="x"))

    def test_symlink_receipt_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            receipt = self.valid_receipt()
            profiles, matrix = self.write_case(root, receipt)
            target = root / "receipt.json"
            actual = root / "actual.json"
            target.rename(actual)
            target.symlink_to(actual.name)
            result = self.run_checker(root, profiles, matrix)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unverified-receipt", result.stdout)


if __name__ == "__main__":
    unittest.main()
