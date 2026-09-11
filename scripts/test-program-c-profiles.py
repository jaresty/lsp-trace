#!/usr/bin/env python3
import hashlib
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-program-c-profiles.py"
BASE = ROOT / "qualification/program-c/projection-profiles.v1.json"


class ProgramCProfilesTest(unittest.TestCase):
    def run_checker(self, root, profiles, matrix, *extra):
        return subprocess.run(
            ["python3", str(CHECKER), "--root", str(root), "--profiles", str(profiles), "--matrix", str(matrix), *extra],
            text=True,
            capture_output=True,
        )

    def write_receipt(self, root, name, status):
        path = root / f"{name}.receipt"
        path.write_text(f"status: {status}\n")
        digest = "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()
        return path.name, digest

    def write_matrix(self, root, ember_status="BLOCKED", duplicate=False):
        rows = []
        definitions = [
            ("calls-v1", "csharp", "none", "csharp-ls", "0.27.0.0", "PASS"),
            ("calls-v1", "go", "none", "gopls", "0.23.0", "PASS"),
            ("callback-flow-v1", "typescript", "ember", "ember-glint", "1.0.3", ember_status),
            ("state-flow-v1", "typescript", "ember", "ember-glint", "1.0.3", ember_status),
            ("ui-lifecycle-v1", "typescript", "ember", "ember-glint", "1.0.3", ember_status),
        ]
        for i, (profile, language, framework, provider, version, status) in enumerate(definitions):
            receipt, digest = self.write_receipt(root, f"row-{i}", status)
            rows.append("\t".join((profile, language, framework, provider, version, "yes", status, receipt, digest)))
        if duplicate:
            rows.append(rows[0])
        path = root / "matrix.tsv"
        path.write_text("# profile\tlanguage\tframework\tprovider\tprovider_version\trequired\tstatus\treceipt\treceipt_sha256\n" + "\n".join(rows) + "\n")
        return path

    def test_blocked_ember_does_not_block_calls(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profiles = root / "profiles.json"
            profiles.write_bytes(BASE.read_bytes())
            matrix = self.write_matrix(root)
            result = self.run_checker(root, profiles, matrix, "--require-profile", "calls-v1")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("profile=calls-v1", result.stdout)

    def test_blocked_member_blocks_all_qualified(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profiles = root / "profiles.json"
            profiles.write_bytes(BASE.read_bytes())
            matrix = self.write_matrix(root)
            result = self.run_checker(root, profiles, matrix, "--require-profile", "all-qualified-v1")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("not-qualified:all-qualified-v1", result.stdout)

    def test_semantic_change_requires_new_digest(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = json.loads(BASE.read_text())
            artifact["profiles"][0]["relations"][0]["weight"] = 2
            profiles = root / "profiles.json"
            profiles.write_text(json.dumps(artifact))
            matrix = self.write_matrix(root)
            result = self.run_checker(root, profiles, matrix)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("ASSERT_PROGRAM_C_PROFILE_SEMANTICS result=FAIL", result.stdout)

    def test_duplicate_qualification_tuple_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profiles = root / "profiles.json"
            profiles.write_bytes(BASE.read_bytes())
            matrix = self.write_matrix(root, duplicate=True)
            result = self.run_checker(root, profiles, matrix)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("duplicate-tuple", result.stdout)


if __name__ == "__main__":
    unittest.main()
