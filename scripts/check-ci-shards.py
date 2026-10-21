#!/usr/bin/env python3
"""Independently validate CI shard accounting and deterministic assignment."""
import argparse
import collections
import json
import os
import subprocess
import sys
from pathlib import Path

EXPECTED_COUNTS = {"linux": 5, "darwin": 4}
TOP_LEVEL_KEYS = {"algorithm", "gopls_packages", "platforms", "version", "weights"}
WEIGHTS_KEYS = {"fallback", "packages"}
PLATFORM_KEYS = {"goos", "shards"}
SHARD_KEYS = {"id", "needs_gopls", "packages", "weight"}
POLICY = json.loads(Path(__file__).with_name("ci-shard-policy.json").read_text())
CANONICAL_FALLBACK = POLICY["fallback_weight"]
CANONICAL_DEFAULT = POLICY["default_weight"]
CANONICAL_OVERRIDES = POLICY["overrides"]
EXCLUSIVE_SHARDS = POLICY["exclusive_shards"]
HEAVY = ("lsp-trace/internal/censuschunking", "lsp-trace/internal/censuscontinuation")
EXPECTED_GOPLS = {"lsp-trace/cmd/lsp-trace-mcp"}


def current_packages(goos):
    env = os.environ.copy()
    env["GOOS"] = goos
    result = subprocess.run(["go", "list", "./..."], check=True, text=True, stdout=subprocess.PIPE, env=env)
    return sorted(filter(None, result.stdout.splitlines()))


def fail(errors, message):
    errors.append(message)


def reject_unsupported_keys(errors, value, allowed, path):
    if isinstance(value, dict):
        unsupported = sorted(set(value) - allowed)
        if unsupported:
            fail(errors, f"{path}: unsupported keys: {unsupported}")


def expected_assignment(goos, packages, weights, fallback):
    count = EXPECTED_COUNTS[goos]
    shards = {f"{goos}-{i}": {"packages": [], "weight": 0} for i in range(1, count + 1)}
    pins = dict(zip(HEAVY, ("linux-1", "linux-2"))) if goos == "linux" else {p: "darwin-1" for p in EXPECTED_GOPLS if p in packages}
    exclusive = {package: shard_id for shard_id, package in EXCLUSIVE_SHARDS.items() if shard_id.startswith(f"{goos}-")}
    for package, shard_id in sorted({**pins, **exclusive}.items()):
        if package in packages:
            shards[shard_id]["packages"].append(package)
            shards[shard_id]["weight"] += weights.get(package, fallback)
    fixed = set(pins) | set(exclusive)
    for package in sorted((p for p in packages if p not in fixed), key=lambda p: (-weights.get(p, fallback), p)):
        eligible = [shard_id for shard_id in shards if shard_id not in EXCLUSIVE_SHARDS]
        if goos == "darwin":
            eligible.remove("darwin-1")
        shard_id = min(eligible, key=lambda sid: (shards[sid]["weight"], sid))
        shards[shard_id]["packages"].append(package)
        shards[shard_id]["weight"] += weights.get(package, fallback)
    return {sid: sorted(value["packages"]) for sid, value in shards.items()}


def validate(manifest, packages_by_os):
    errors = []
    reject_unsupported_keys(errors, manifest, TOP_LEVEL_KEYS, "manifest")
    reject_unsupported_keys(errors, manifest.get("weights"), WEIGHTS_KEYS, "manifest.weights")
    platform_keys = set(manifest.get("platforms", {}))
    if platform_keys != set(EXPECTED_COUNTS):
        fail(errors, f"platform keys must be exactly {sorted(EXPECTED_COUNTS)}")
    if manifest.get("algorithm") != "deterministic-lpt-v1":
        fail(errors, "unknown algorithm")
    manifest_weights = manifest.get("weights", {}).get("packages", {})
    fallback = manifest.get("weights", {}).get("fallback")
    all_packages = sorted(set().union(*(set(value) for value in packages_by_os.values())))
    weights = {package: CANONICAL_OVERRIDES.get(package, CANONICAL_DEFAULT) for package in all_packages}
    expected_weights = {"fallback": CANONICAL_FALLBACK, "packages": weights}
    supported_weights = {
        key: value
        for key, value in manifest.get("weights", {}).items()
        if key in WEIGHTS_KEYS
    }
    if supported_weights != expected_weights:
        fail(errors, "manifest weights differ from canonical policy")
    fallback = CANONICAL_FALLBACK
    if set(manifest.get("gopls_packages", [])) != EXPECTED_GOPLS:
        fail(errors, "gopls classification mismatch")
    for goos, count in EXPECTED_COUNTS.items():
        platform = manifest.get("platforms", {}).get(goos, {})
        reject_unsupported_keys(errors, platform, PLATFORM_KEYS, f"manifest.platforms.{goos}")
        if platform.get("goos") != goos:
            fail(errors, f"{goos}: platform mismatch")
        shards = platform.get("shards", [])
        for index, shard in enumerate(shards):
            reject_unsupported_keys(
                errors,
                shard,
                SHARD_KEYS,
                f"manifest.platforms.{goos}.shards[{index}]",
            )
        if len(shards) != count:
            fail(errors, f"{goos}: expected {count} shards")
        ids = [s.get("id") for s in shards]
        if len(ids) != len(set(ids)):
            fail(errors, f"{goos}: duplicate shard IDs")
        flattened = [p for s in shards for p in s.get("packages", [])]
        if any(not isinstance(p, str) or not p for p in flattened):
            fail(errors, f"{goos}: empty package path")
        counts = collections.Counter(flattened)
        expected = set(packages_by_os[goos])
        actual = set(flattened)
        missing = expected - actual
        unknown = actual - expected
        duplicates = sorted(p for p, n in counts.items() if n != 1)
        if missing:
            fail(errors, f"{goos}: omitted packages: {sorted(missing)}")
        if unknown:
            fail(errors, f"{goos}: unknown packages: {sorted(unknown)}")
        if duplicates:
            fail(errors, f"{goos}: duplicate packages: {duplicates}")
        accounting_valid = not missing and not unknown and not duplicates
        exclusive_for_platform = {
            shard_id: package
            for shard_id, package in EXCLUSIVE_SHARDS.items()
            if shard_id.startswith(f"{goos}-")
        }
        for shard_id, package in sorted(exclusive_for_platform.items()):
            reserved = next((shard for shard in shards if shard.get("id") == shard_id), None)
            locations = [shard.get("id") for shard in shards if package in shard.get("packages", [])]
            if reserved is None or package not in reserved.get("packages", []):
                fail(errors, f"{goos}: exclusive package {package} missing from {shard_id}")
            elif locations != [shard_id]:
                fail(errors, f"{goos}: exclusive package {package} must appear only in {shard_id}")
            if reserved is not None and reserved.get("packages", []) != [package]:
                fail(errors, f"{goos}: exclusive shard {shard_id} must contain only ['{package}']")
        if goos == "linux" and all(p in expected for p in HEAVY):
            owners = {p: next((s.get("id") for s in shards if p in s.get("packages", [])), None) for p in HEAVY}
            if owners[HEAVY[0]] == owners[HEAVY[1]]:
                fail(errors, "linux: heavy packages must be in distinct shards")
        gopls_valid = True
        if goos == "darwin":
            marked = [s for s in shards if s.get("needs_gopls") is True]
            if len(marked) != 1:
                fail(errors, "darwin: exactly one needs_gopls shard required")
                gopls_valid = False
            elif not EXPECTED_GOPLS.issubset(set(marked[0].get("packages", []))):
                fail(errors, "darwin: gopls package misplaced")
                gopls_valid = False
            elif any(not shard.get("needs_gopls") and EXPECTED_GOPLS.intersection(shard.get("packages", [])) for shard in shards):
                fail(errors, "darwin: gopls package in unmarked shard")
                gopls_valid = False
        for shard in shards:
            calculated = sum(weights.get(p, fallback) for p in shard.get("packages", []))
            if shard.get("weight") != calculated:
                fail(errors, f"{goos}: incorrect weight for {shard.get('id')}")
        expected_assignment_by_id = expected_assignment(goos, packages_by_os[goos], weights, fallback)
        actual_assignment_by_id = {s.get("id"): sorted(s.get("packages", [])) for s in shards}
        if accounting_valid and gopls_valid and actual_assignment_by_id != expected_assignment_by_id:
            fail(errors, f"{goos}: assignment is not canonical deterministic LPT")
    return errors


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default="scripts/ci-shards.json")
    parser.add_argument("--packages-file")
    parser.add_argument("--platform", choices=["linux", "darwin"])
    parser.add_argument("--shard")
    args = parser.parse_args()
    manifest = json.loads(Path(args.manifest).read_text())
    packages = json.loads(Path(args.packages_file).read_text()) if args.packages_file else {o: current_packages(o) for o in EXPECTED_COUNTS}
    errors = validate(manifest, packages)
    if errors:
        for error in errors:
            print(f"FAIL {error}")
        return 1
    if args.platform and args.shard:
        shard = next((s for s in manifest["platforms"][args.platform]["shards"] if s["id"] == args.shard), None)
        if shard is None:
            print(f"FAIL unknown shard {args.shard}")
            return 1
        print(" ".join(shard["packages"]))
    else:
        print("PASS CI shard manifest accounting")
    return 0


if __name__ == "__main__":
    sys.exit(main())
