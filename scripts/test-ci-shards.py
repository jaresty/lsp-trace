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
            return result.stdout

    def test_generated_manifest_is_deterministic_and_valid(self):
        first = self.generate()
        second = self.generate()
        self.assertEqual(first, second)
        self.check(first)
        self.assertEqual(len(first["platforms"]["linux"]["shards"]), 5)
        self.assertEqual(len(first["platforms"]["darwin"]["shards"]), 4)

    def test_mutations_are_rejected(self):
        base = self.generate()
        mutations = {}
        omitted = copy.deepcopy(base)
        next(s for s in omitted["platforms"]["linux"]["shards"] if s["packages"])["packages"].pop()
        mutations["omitted"] = omitted
        duplicated = copy.deepcopy(base)
        package = duplicated["platforms"]["linux"]["shards"][0]["packages"][0]
        duplicated["platforms"]["linux"]["shards"][1]["packages"].append(package)
        mutations["duplicated"] = duplicated
        unknown = copy.deepcopy(base)
        unknown["platforms"]["darwin"]["shards"][0]["packages"].append("lsp-trace/internal/unknown")
        mutations["unknown"] = unknown
        misplaced = copy.deepcopy(base)
        source = next(s for s in misplaced["platforms"]["darwin"]["shards"] if s["needs_gopls"])
        target = next(s for s in misplaced["platforms"]["darwin"]["shards"] if not s["needs_gopls"])
        source["packages"].remove("lsp-trace/cmd/lsp-trace-mcp")
        target["packages"].append("lsp-trace/cmd/lsp-trace-mcp")
        mutations["gopls misplaced"] = misplaced
        mismatch = copy.deepcopy(base)
        mismatch["platforms"]["linux"]["goos"] = "darwin"
        mutations["platform mismatch"] = mismatch
        wrong_count = copy.deepcopy(base)
        wrong_count["platforms"]["linux"]["shards"].pop()
        mutations["wrong shard count"] = wrong_count
        heavy_together = copy.deepcopy(base)
        chunk = next(s for s in heavy_together["platforms"]["linux"]["shards"] if "lsp-trace/internal/censuschunking" in s["packages"])
        continuation = next(s for s in heavy_together["platforms"]["linux"]["shards"] if "lsp-trace/internal/censuscontinuation" in s["packages"])
        continuation["packages"].remove("lsp-trace/internal/censuscontinuation")
        chunk["packages"].append("lsp-trace/internal/censuscontinuation")
        chunk["weight"] += 1200
        continuation["weight"] -= 1200
        mutations["heavy packages together"] = heavy_together
        for name, manifest in mutations.items():
            with self.subTest(name=name):
                self.check(manifest, expected=False)

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
