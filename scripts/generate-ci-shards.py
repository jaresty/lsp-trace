#!/usr/bin/env python3
"""Generate the canonical deterministic CI shard manifest."""
import argparse
import json
import os
import subprocess
from pathlib import Path

SHARD_COUNTS = {"linux": 5, "darwin": 4}
POLICY = json.loads((Path(__file__).with_name("ci-shard-policy.json")).read_text())
FALLBACK_WEIGHT = POLICY["fallback_weight"]
DEFAULT_WEIGHT = POLICY["default_weight"]
OVERRIDES = POLICY["overrides"]
EXCLUSIVE_SHARDS = POLICY["exclusive_shards"]
LINUX_PINS = {
    "lsp-trace/internal/censuschunking": "linux-1",
    "lsp-trace/internal/censuscontinuation": "linux-2",
}
GOPLS_PACKAGES = {"lsp-trace/cmd/lsp-trace-mcp"}


def list_packages(goos):
    env = os.environ.copy()
    env["GOOS"] = goos
    result = subprocess.run(["go", "list", "./..."], check=True, text=True, stdout=subprocess.PIPE, env=env)
    return sorted(line for line in result.stdout.splitlines() if line)


def load_packages(path):
    if path:
        return json.loads(Path(path).read_text())
    return {goos: list_packages(goos) for goos in SHARD_COUNTS}


def assign(goos, packages, weights):
    shards = [{"id": f"{goos}-{i}", "needs_gopls": False, "packages": [], "weight": 0} for i in range(1, SHARD_COUNTS[goos] + 1)]
    by_id = {shard["id"]: shard for shard in shards}
    pins = dict(LINUX_PINS) if goos == "linux" else {p: "darwin-1" for p in GOPLS_PACKAGES if p in packages}
    exclusive = {package: shard_id for shard_id, package in EXCLUSIVE_SHARDS.items() if shard_id.startswith(f"{goos}-")}
    if goos == "darwin":
        by_id["darwin-1"]["needs_gopls"] = True
    for package, shard_id in sorted({**pins, **exclusive}.items()):
        if package in packages:
            shard = by_id[shard_id]
            shard["packages"].append(package)
            shard["weight"] += weights.get(package, FALLBACK_WEIGHT)
    fixed = set(pins) | set(exclusive)
    for package in sorted((p for p in packages if p not in fixed), key=lambda p: (-weights.get(p, FALLBACK_WEIGHT), p)):
        eligible = [s for s in shards if s["id"] not in EXCLUSIVE_SHARDS]
        if goos == "darwin":
            eligible = [s for s in eligible if not s["needs_gopls"]]
        shard = min(eligible, key=lambda s: (s["weight"], s["id"]))
        shard["packages"].append(package)
        shard["weight"] += weights.get(package, FALLBACK_WEIGHT)
    for shard in shards:
        shard["packages"].sort()
    return shards


def build(packages_by_os):
    all_packages = sorted(set().union(*map(set, packages_by_os.values())))
    weights = {package: OVERRIDES.get(package, DEFAULT_WEIGHT) for package in all_packages}
    return {
        "version": 1,
        "algorithm": "deterministic-lpt-v1",
        "weights": {"fallback": FALLBACK_WEIGHT, "packages": weights},
        "gopls_packages": sorted(GOPLS_PACKAGES),
        "platforms": {
            goos: {"goos": goos, "shards": assign(goos, packages_by_os[goos], weights)}
            for goos in ("linux", "darwin")
        },
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--packages-file")
    parser.add_argument("--output", default="scripts/ci-shards.json")
    args = parser.parse_args()
    manifest = build(load_packages(args.packages_file))
    Path(args.output).write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
