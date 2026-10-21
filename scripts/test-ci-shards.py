#!/usr/bin/env python3
import copy
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECK = ROOT / "scripts" / "check-ci-shards.py"
GENERATE = ROOT / "scripts" / "generate-ci-shards.py"

PACKAGES = {
    "linux": [
        "lsp-trace/cmd/lsp-trace-mcp",
        "lsp-trace/internal/censuschunking",
        "lsp-trace/internal/censuscontinuation",
        "lsp-trace/internal/example",
    ],
    "darwin": [
        "lsp-trace/cmd/lsp-trace-mcp",
        "lsp-trace/internal/censuschunking",
        "lsp-trace/internal/censuscontinuation",
        "lsp-trace/internal/example",
    ],
}


class CIShardsTest(unittest.TestCase):
    def generate(self):
        with tempfile.TemporaryDirectory() as td:
            package_file = Path(td) / "packages.json"
            output = Path(td) / "manifest.json"
            package_file.write_text(json.dumps(PACKAGES))
            subprocess.run(
                ["python3", str(GENERATE), "--packages-file", str(package_file), "--output", str(output)],
                cwd=ROOT,
                check=True,
            )
            return json.loads(output.read_text())

    def check(self, manifest, expected=True, packages=PACKAGES):
        with tempfile.TemporaryDirectory() as td:
            manifest_file = Path(td) / "manifest.json"
            package_file = Path(td) / "packages.json"
            manifest_file.write_text(json.dumps(manifest))
            package_file.write_text(json.dumps(packages))
            result = subprocess.run(
                ["python3", str(CHECK), "--manifest", str(manifest_file), "--packages-file", str(package_file)],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
            )
            self.assertEqual(result.returncode == 0, expected, result.stdout)
            return result.stdout.splitlines()

    def test_generated_manifest_is_deterministic_and_valid(self):
        first = self.generate()
        second = self.generate()
        self.assertEqual(first, second)
        self.check(first)
        self.assertEqual(len(first["platforms"]["linux"]["shards"]), 5)
        self.assertEqual(len(first["platforms"]["darwin"]["shards"]), 4)

    def test_mutations_are_rejected_with_exact_diagnostics(self):
        base = self.generate()
        cases = []
        omitted = copy.deepcopy(base)
        removed = next(s for s in omitted["platforms"]["linux"]["shards"] if s["packages"])["packages"].pop()
        omitted["platforms"]["darwin"]["shards"][-1]["weight"] += 7
        cases.append(("omitted", omitted, f"FAIL linux: omitted packages: ['{removed}']"))
        duplicated = copy.deepcopy(base)
        package = duplicated["platforms"]["linux"]["shards"][0]["packages"][0]
        duplicated["platforms"]["linux"]["shards"][1]["packages"].append(package)
        duplicated["platforms"]["darwin"]["shards"][-1]["weight"] += 9
        cases.append(("duplicate", duplicated, f"FAIL linux: duplicate packages: ['{package}']"))
        unknown = copy.deepcopy(base)
        unknown["platforms"]["darwin"]["shards"][0]["packages"].append("lsp-trace/internal/unknown")
        cases.append(("unknown", unknown, "FAIL darwin: unknown packages: ['lsp-trace/internal/unknown']"))
        misplaced = copy.deepcopy(base)
        source = next(s for s in misplaced["platforms"]["darwin"]["shards"] if s["needs_gopls"])
        target = next(s for s in misplaced["platforms"]["darwin"]["shards"] if not s["needs_gopls"])
        source["packages"].remove("lsp-trace/cmd/lsp-trace-mcp")
        target["packages"].append("lsp-trace/cmd/lsp-trace-mcp")
        cases.append(("gopls misplaced", misplaced, "FAIL darwin: gopls package misplaced"))
        extra_platform = copy.deepcopy(base)
        extra_platform["platforms"]["windows"] = {"goos": "windows", "shards": []}
        cases.append(("platform", extra_platform, "FAIL platform keys must be exactly ['darwin', 'linux']"))
        weight_tamper = copy.deepcopy(base)
        weight_tamper["weights"]["packages"]["lsp-trace/internal/example"] = 999
        cases.append(("weight tamper", weight_tamper, "FAIL manifest weights differ from canonical policy"))
        noncanonical = copy.deepcopy(base)
        candidates = [s for s in noncanonical["platforms"]["linux"]["shards"] if s["id"] not in {"linux-1", "linux-2"} and s["packages"]]
        left, right = candidates[:2]
        left["packages"][0], right["packages"][0] = right["packages"][0], left["packages"][0]
        cases.append(("noncanonical assignment", noncanonical, "FAIL linux: assignment is not canonical deterministic LPT"))
        for name, manifest, diagnostic in cases:
            with self.subTest(name=name):
                self.assertIn(diagnostic, self.check(manifest, expected=False))

    def test_unseen_packages_use_conservative_fallback(self):
        packages = copy.deepcopy(PACKAGES)
        packages["linux"].append("lsp-trace/internal/new-package")
        packages["darwin"].append("lsp-trace/internal/new-package")
        with tempfile.TemporaryDirectory() as td:
            package_file = Path(td) / "packages.json"
            output = Path(td) / "manifest.json"
            package_file.write_text(json.dumps(packages))
            subprocess.run(["python3", str(GENERATE), "--packages-file", str(package_file), "--output", str(output)], cwd=ROOT, check=True)
            manifest = json.loads(output.read_text())
            self.assertEqual(manifest["weights"]["fallback"], 300)
            self.check(manifest, packages=packages)


if __name__ == "__main__":
    unittest.main()
