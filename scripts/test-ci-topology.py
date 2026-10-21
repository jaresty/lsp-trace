#!/usr/bin/env python3
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECK = ROOT / "scripts/check-ci-topology.py"
WORKFLOW = ROOT / ".github/workflows/ci.yml"


class TopologyTest(unittest.TestCase):
    def check(self, text, diagnostic=None):
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "ci.yml"
            path.write_text(text)
            result = subprocess.run(["python3", str(CHECK), "--workflow", str(path)], cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if diagnostic is None:
            self.assertEqual(result.returncode, 0, result.stdout)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout)
            self.assertIn("FAIL " + diagnostic, result.stdout)

    def test_repository_workflow(self):
        self.check(WORKFLOW.read_text())

    def test_comment_cannot_satisfy_test_invocation(self):
        text = WORKFLOW.read_text().replace("go test -json -timeout=10m", "go test -timeout=10m")
        text += "\n# go test -json -timeout=10m\n"
        self.check(text, "test-linux: test invocation must include -json and -timeout=10m")

    def test_unrelated_job_cannot_satisfy_upload_condition(self):
        text = WORKFLOW.read_text().replace("        if: always()\n        uses: actions/upload-artifact@v4", "        uses: actions/upload-artifact@v4")
        text += "\n  unrelated-proof:\n    if: always()\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo harmless\n"
        self.check(text, "test-linux: upload step must use if: always()")


if __name__ == "__main__":
    unittest.main()
