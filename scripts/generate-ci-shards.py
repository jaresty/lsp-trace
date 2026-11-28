#!/usr/bin/env python3
"""Generate the canonical deterministic CI shard manifest."""
import argparse
import json
import os
import runpy
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


def root_selector(roots):
    """Go regexp.QuoteMeta, NOT Python re.escape; slash is Go subtest syntax."""
    if not roots or any(not r or '/' in r or '\n' in r for r in roots):
        raise ValueError('empty or slash-containing root')
    meta = set('\\\\.+*?()|[]{}^$')
    return '^(' + '|'.join(''.join('\\' + c if c in meta else c for c in r) for r in roots) + ')$'


def invocation_plan(inventory, shard):
    """Pure runtime mode. Package ownership is not regenerated or reassigned."""
    admission = runpy.run_path(str(Path(__file__).with_name('check-ci-shards.py')))['validate_batch_policy']
    errors = admission(POLICY)
    if errors:
        raise ValueError('; '.join(errors))
    policy = POLICY['root_batching']
    allowed = set(policy['packages'])
    records = inventory['packages']
    invocations = []
    ordinary = [p for p in shard['packages'] if p not in allowed]
    if ordinary:
        invocations.append({'id': 'ordinary', 'packages': ordinary, 'roots': [], 'selector': None})
    blocked = []
    for package in sorted(set(shard['packages']) & allowed):
        record = records.get(package)
        if record is None or record.get('error'):
            blocked.append(package)
            continue
        roots = sorted(n for n, kind in record['roots'].items() if kind != 'Benchmark')
        if not roots:
            raise ValueError('empty runnable inventory: ' + package)
        for offset in range(0, len(roots), policy['max_roots']):
            chunk = roots[offset:offset + policy['max_roots']]
            invocations.append({'id': package.replace('/', '_') + '-%04d' % (offset // policy['max_roots'] + 1),
                                'packages': [package], 'roots': chunk, 'selector': root_selector(chunk)})
    return {'version': 1, 'context': inventory['context'], 'shard': shard['id'],
            'inventory_id': inventory['inventory_id'], 'blocked_packages': blocked,
            'invocations': invocations}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--packages-file")
    parser.add_argument("--inventory", help="pure invocation planning; does not execute Go")
    parser.add_argument("--shard")
    parser.add_argument("--manifest", default="scripts/ci-shards.json")
    parser.add_argument("--output", default="scripts/ci-shards.json")
    args = parser.parse_args()
    if args.inventory:
        inventory = json.loads(Path(args.inventory).read_text())
        ownership = json.loads(Path(args.manifest).read_text())
        shards = ownership['platforms'][inventory['context']['platform']]['shards']
        shard = next(s for s in shards if s['id'] == args.shard)
        manifest = invocation_plan(inventory, shard)
    else:
        manifest = build(load_packages(args.packages_file))
    Path(args.output).write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
