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
        "lsp-trace/sessionruntime",
    ],
    "darwin": [
        "lsp-trace/cmd/lsp-trace-mcp",
        "lsp-trace/internal/censuschunking",
        "lsp-trace/internal/censuscontinuation",
        "lsp-trace/internal/example",
        "lsp-trace/sessionruntime",
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

    def recompute_shard_weights(self, manifest):
        weights = manifest["weights"]["packages"]
        fallback = manifest["weights"]["fallback"]
        for platform in manifest["platforms"].values():
            for shard in platform["shards"]:
                shard["weight"] = sum(weights.get(package, fallback) for package in shard["packages"])
        return manifest

    def test_generated_manifest_is_deterministic_and_valid(self):
        first = self.generate()
        second = self.generate()
        self.assertEqual(first, second)
        self.assertEqual(self.check(first), "PASS CI shard manifest accounting\n")
        self.assertEqual(len(first["platforms"]["linux"]["shards"]), 5)
        self.assertEqual(len(first["platforms"]["darwin"]["shards"]), 4)
        reserved = next(shard for shard in first["platforms"]["linux"]["shards"] if shard["id"] == "linux-5")
        self.assertEqual(reserved["packages"], ["lsp-trace/sessionruntime"])

    def test_exclusive_package_mutations(self):
        base = self.generate()
        package = "lsp-trace/sessionruntime"
        reserved_id = "linux-5"

        co_located = copy.deepcopy(base)
        reserved = next(shard for shard in co_located["platforms"]["linux"]["shards"] if shard["id"] == reserved_id)
        reserved["packages"].append("lsp-trace/internal/example")
        source = next(shard for shard in co_located["platforms"]["linux"]["shards"] if shard["id"] != reserved_id and "lsp-trace/internal/example" in shard["packages"])
        source["packages"].remove("lsp-trace/internal/example")
        self.recompute_shard_weights(co_located)
        self.assertIn("FAIL linux: exclusive shard linux-5 must contain only ['lsp-trace/sessionruntime']", self.check(co_located, expected=False))

        missing = copy.deepcopy(base)
        next(shard for shard in missing["platforms"]["linux"]["shards"] if shard["id"] == reserved_id)["packages"].remove(package)
        self.recompute_shard_weights(missing)
        self.assertIn("FAIL linux: exclusive package lsp-trace/sessionruntime missing from linux-5", self.check(missing, expected=False))

        duplicate = copy.deepcopy(base)
        next(shard for shard in duplicate["platforms"]["linux"]["shards"] if shard["id"] != reserved_id)["packages"].append(package)
        self.recompute_shard_weights(duplicate)
        self.assertIn("FAIL linux: exclusive package lsp-trace/sessionruntime must appear only in linux-5", self.check(duplicate, expected=False))

        wrong_platform = copy.deepcopy(base)
        linux_reserved = next(shard for shard in wrong_platform["platforms"]["linux"]["shards"] if shard["id"] == reserved_id)
        wrong_platform["platforms"]["linux"]["shards"].remove(linux_reserved)
        wrong_platform["platforms"]["darwin"]["shards"].append(linux_reserved)
        self.assertNotEqual(self.check(wrong_platform, expected=False), "")

        reopened = copy.deepcopy(base)
        next(shard for shard in reopened["platforms"]["linux"]["shards"] if shard["id"] == reserved_id)["packages"].append("lsp-trace/internal/example")
        self.recompute_shard_weights(reopened)
        self.assertIn("FAIL linux: exclusive shard linux-5 must contain only ['lsp-trace/sessionruntime']", self.check(reopened, expected=False))

        comment_only = copy.deepcopy(base)
        comment_only["comment"] = "exclusive Linux shard policy"
        self.assertEqual(self.check(comment_only), "PASS CI shard manifest accounting\n")

    def test_mutations_are_rejected_with_exact_diagnostics(self):
        base = self.generate()
        cases = []
        omitted = copy.deepcopy(base)
        removed = next(s for s in omitted["platforms"]["linux"]["shards"] if s["packages"])["packages"].pop()
        self.recompute_shard_weights(omitted)
        cases.append(("omitted", omitted, f"FAIL linux: omitted packages: ['{removed}']\n"))
        duplicated = copy.deepcopy(base)
        package = duplicated["platforms"]["linux"]["shards"][0]["packages"][0]
        duplicated["platforms"]["linux"]["shards"][1]["packages"].append(package)
        self.recompute_shard_weights(duplicated)
        cases.append(("duplicate", duplicated, f"FAIL linux: duplicate packages: ['{package}']\n"))
        unknown = copy.deepcopy(base)
        unknown["platforms"]["darwin"]["shards"][0]["packages"].append("lsp-trace/internal/unknown")
        self.recompute_shard_weights(unknown)
        cases.append(("unknown", unknown, "FAIL darwin: unknown packages: ['lsp-trace/internal/unknown']\n"))
        misplaced = copy.deepcopy(base)
        source = next(s for s in misplaced["platforms"]["darwin"]["shards"] if s["needs_gopls"])
        target = next(s for s in misplaced["platforms"]["darwin"]["shards"] if not s["needs_gopls"])
        source["packages"].remove("lsp-trace/cmd/lsp-trace-mcp")
        target["packages"].append("lsp-trace/cmd/lsp-trace-mcp")
        self.recompute_shard_weights(misplaced)
        cases.append(("gopls misplaced", misplaced, "FAIL darwin: gopls package misplaced\n"))
        extra_platform = copy.deepcopy(base)
        extra_platform["platforms"]["windows"] = {"goos": "windows", "shards": []}
        cases.append(("platform", extra_platform, "FAIL platform keys must be exactly ['darwin', 'linux']\n"))
        weight_tamper = copy.deepcopy(base)
        weight_tamper["weights"]["packages"]["lsp-trace/internal/example"] = 999
        cases.append(("weight tamper", weight_tamper, "FAIL manifest weights differ from canonical policy\n"))
        noncanonical = copy.deepcopy(base)
        candidates = [s for s in noncanonical["platforms"]["linux"]["shards"] if s["id"] not in {"linux-1", "linux-2"} and s["packages"]]
        left, right = candidates[:2]
        left["packages"][0], right["packages"][0] = right["packages"][0], left["packages"][0]
        self.recompute_shard_weights(noncanonical)
        cases.append(("noncanonical assignment", noncanonical, "FAIL linux: assignment is not canonical deterministic LPT\n"))
        for name, manifest, diagnostic in cases:
            with self.subTest(name=name):
                self.assertEqual(self.check(manifest, expected=False), diagnostic)

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
