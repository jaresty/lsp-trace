#!/usr/bin/env python3
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "summarize-go-test-json.py"

class SummaryTest(unittest.TestCase):
    def summarize(self, lines, exit_code=0, raw_text=None):
        with tempfile.TemporaryDirectory() as td:
            raw = Path(td) / "raw.json"
            out = Path(td) / "summary.json"
            raw.write_text(raw_text if raw_text is not None else "\n".join(json.dumps(x) for x in lines) + "\n")
            subprocess.run(["python3", str(SCRIPT), "--input", str(raw), "--output", str(out), "--os", "linux", "--commit", "abc", "--job", "test-linux", "--shard", "linux-1", "--exit-code", str(exit_code)], check=True)
            return json.loads(out.read_text())

    def test_success_elapsed_and_cached(self):
        result = self.summarize([
            {"Time":"2026-01-01T00:00:00Z","Action":"start","Package":"p/a"},
            {"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"p/a","Elapsed":1.25,"Output":"ok  p/a (cached)\n"},
        ])
        self.assertEqual(result["status"], "success")
        self.assertEqual(result["packages"], [{"package":"p/a","status":"pass","elapsed":1.25,"cached":True}])
        self.assertEqual(result["os"], "linux")

    def test_failure_and_timeout(self):
        failure = self.summarize([{"Action":"fail","Package":"p/a","Elapsed":2.0}], 1)
        self.assertEqual(failure["status"], "failure")
        timeout = self.summarize([{"Action":"output","Package":"p/a","Output":"panic: test timed out after 10m0s\n"},{"Action":"fail","Package":"p/a","Elapsed":600.0}], 1)
        self.assertEqual(timeout["status"], "timeout")
        self.assertEqual(timeout["packages"][0]["status"], "timeout")

    def test_malformed_or_truncated_stream_emits_bounded_partial_summary(self):
        raw = '{"Action":"pass","Package":"p/a"}\n{"Action":'
        result = self.summarize([], exit_code=1, raw_text=raw)
        self.assertEqual(result["status"], "failure")
        self.assertEqual(result["malformed_lines"], 1)
        self.assertTrue(result["partial"])
        self.assertEqual(result["packages"], [{"package":"p/a","status":"pass","elapsed":None,"cached":False}])

if __name__ == "__main__":
    unittest.main()
