#!/usr/bin/env python3
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECK = ROOT / "scripts/check-ci-topology.py"
WORKFLOW = ROOT / ".github/workflows/ci.yml"


class TopologyTest(unittest.TestCase):
    def check(self, text, diagnostics=None):
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "ci.yml"
            path.write_text(text)
            result = subprocess.run(["python3", str(CHECK), "--workflow", str(path)], cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if diagnostics is None:
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertEqual(result.stdout, "PASS CI workflow topology\n")
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout)
            expected = "".join(f"FAIL {diagnostic}\n" for diagnostic in diagnostics)
            self.assertEqual(result.stdout, expected)

    def mutate_shard_run(self, old, new):
        text = WORKFLOW.read_text()
        self.assertEqual(text.count(old), 1)
        return text.replace(old, new)

    def test_repository_workflow(self):
        self.check(WORKFLOW.read_text())

    def test_in_block_comment_cannot_satisfy_test_invocation_flags(self):
        text = self.mutate_shard_run(
            "          go test -json -timeout=10m ${{ steps.packages.outputs.packages }} 2>&1 | tee go-test.json\n",
            "          # go test -json -timeout=10m\n"
            "          go test ${{ steps.packages.outputs.packages }} 2>&1 | tee go-test.json\n",
        )
        self.check(text, [
            "test-linux: test invocation must include -json and -timeout=10m",
            "test-darwin: test invocation must include -json and -timeout=10m",
        ])

    def test_in_block_comments_cannot_satisfy_status_fragments(self):
        replacements = {
            "          test_status=${PIPESTATUS[0]}\n": "          # test_status=${PIPESTATUS[0]}\n",
            "          tee_status=${PIPESTATUS[1]}\n": "          # tee_status=${PIPESTATUS[1]}\n",
            "          summary_status=$?\n": "          # summary_status=$?\n",
            '          if [ "$test_status" -ne 0 ]; then exit "$test_status"; fi\n': '          # if [ "$test_status" -ne 0 ]; then exit "$test_status"; fi\n',
        }
        for old, new in replacements.items():
            with self.subTest(fragment=old.strip()):
                text = self.mutate_shard_run(old, new)
                self.check(text, [
                    "test-linux: test and telemetry statuses must remain separate with test status primary",
                    "test-darwin: test and telemetry statuses must remain separate with test status primary",
                ])

    def test_reordered_status_exits_are_rejected(self):
        text = self.mutate_shard_run(
            '          if [ "$test_status" -ne 0 ]; then exit "$test_status"; fi\n'
            '          if [ "$tee_status" -ne 0 ]; then exit "$tee_status"; fi\n'
            '          exit "$summary_status"\n',
            '          if [ "$tee_status" -ne 0 ]; then exit "$tee_status"; fi\n'
            '          if [ "$test_status" -ne 0 ]; then exit "$test_status"; fi\n'
            '          exit "$summary_status"\n',
        )
        self.check(text, [
            "test-linux: test and telemetry statuses must remain separate with test status primary",
            "test-darwin: test and telemetry statuses must remain separate with test status primary",
        ])

    def test_duplicate_and_dead_go_test_commands_are_rejected(self):
        valid = "          go test -json -timeout=10m ${{ steps.packages.outputs.packages }} 2>&1 | tee go-test.json\n"
        for extra in (
            valid,
            "          false && go test -json -timeout=10m ./...\n",
        ):
            with self.subTest(extra=extra.strip()):
                text = self.mutate_shard_run(valid, extra + valid)
                self.check(text, [
                    "test-linux: test invocation must be exactly one effective pipeline",
                    "test-darwin: test invocation must be exactly one effective pipeline",
                ])

    def test_unrelated_job_cannot_satisfy_upload_condition(self):
        text = WORKFLOW.read_text().replace("        if: always()\n        uses: actions/upload-artifact@v4", "        uses: actions/upload-artifact@v4")
        text += "\n  unrelated-proof:\n    if: always()\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo harmless\n"
        self.check(text, [
            "test-linux: upload step must use if: always()",
            "test-darwin: upload step must use if: always()",
        ])


if __name__ == "__main__":
    unittest.main()
