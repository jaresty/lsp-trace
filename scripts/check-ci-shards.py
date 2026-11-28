#!/usr/bin/env python3
"""Independently validate CI shard accounting and deterministic assignment."""
import argparse
import collections
import json
import hashlib
import re
import unicodedata
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


def validate_batch_policy(policy=None):
    """Shared schema admission only; partition checking stays independent."""
    policy = POLICY if policy is None else policy
    value = policy.get('root_batching') if isinstance(policy, dict) else None
    required = {'packages', 'max_roots', 'toolchain', 'compatibility'}
    if not isinstance(value, dict) or set(value) != required:
        return ['unsupported batching policy schema']
    packages = value['packages']
    if (not isinstance(packages, list) or not all(isinstance(p, str) for p in packages)
            or len(packages) != 2 or set(packages) != BATCH_PACKAGES):
        return ['unsupported batching policy package allowlist']
    if type(value['max_roots']) is not int or value['max_roots'] != 4:
        return ['unsupported batching policy root cap']
    if value['toolchain'] != 'go1.26.5':
        return ['unsupported batching policy toolchain']
    if not isinstance(value['compatibility'], str) or not value['compatibility'].strip():
        return ['batching policy must disclose initialization compatibility']
    return []


def validate(manifest, packages_by_os):
    errors = validate_batch_policy()
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


BATCH_PACKAGES = {'lsp-trace/cmd/lsp-trace', 'lsp-trace/internal/censuslineagepilot'}


def digest(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def inventory_identity(inventory):
    content = {k: v for k, v in inventory.items() if k != 'inventory_id'}
    return hashlib.sha256(json.dumps(content, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def json_records(text):
    """Decode go list's sequence of JSON objects, not a synthetic JSON array."""
    decoder = json.JSONDecoder()
    records = []
    while text.strip():
        text = text.lstrip()
        record, end = decoder.raw_decode(text)
        if not isinstance(record, dict):
            raise ValueError('metadata record is not an object')
        records.append(record)
        text = text[end:]
    return records


def selected_sources(metadata, package):
    """Hash only compiler-selected files of the affected package, never a tree sweep.

    Dependency/compiler metadata is retained in full separately. Reading selected
    source bytes proves identity, NOT absence of init/TestMain side effects.
    """
    paths = set()
    for record in json_records(metadata):
        if record.get('Error') or record.get('DepsErrors') or record.get('Incomplete'):
            raise ValueError('incomplete compiler metadata')
        if record.get('ImportPath', '').split(' [')[0] not in {package, package + '_test'}:
            continue
        directory = Path(record['Dir'])
        for key in ('GoFiles', 'CgoFiles', 'TestGoFiles', 'XTestGoFiles', 'CompiledGoFiles'):
            for name in record.get(key, []):
                paths.add(str((directory / name).resolve()))
    if not paths:
        raise ValueError('empty compiler-selected source metadata')
    return [{'path': p, 'sha256': digest(p)} for p in sorted(paths)]


def parse_listing(text, package):
    """Pinned Go 1.26.5 grammar: testing.listTests, cmd/go/internal/test:1731.

    listTests returns before PASS printing. test2json emits start, name output,
    `ok  \\tpackage\\t0.012s\\n` output, pass; output may be fragmented at 1024 bytes.
    Unknown output (including initialization chatter) is deliberately fail-closed.
    """
    records = [json.loads(line) for line in text.splitlines() if line.strip()]
    if len(records) < 3 or not all(isinstance(e, dict) for e in records):
        raise ValueError('empty or malformed listing')
    if any(e.get('Package') != package or e.get('Test') for e in records):
        raise ValueError('listing package/test attribution mismatch')
    if records[0].get('Action') != 'start' or records[-1].get('Action') != 'pass':
        raise ValueError('listing lacks start/pass terminal')
    if any(e.get('Action') != 'output' or not isinstance(e.get('Output'), str) for e in records[1:-1]):
        raise ValueError('unknown or duplicate listing lifecycle')
    output = ''.join(e['Output'] for e in records[1:-1])
    if not output.endswith('\n'):
        raise ValueError('unfinished listing output')
    names = output.splitlines()
    footer = r'ok  \t' + re.escape(package) + r'\t[0-9]+\.[0-9]{3}s'
    if not names or not re.fullmatch(footer, names.pop()):
        raise ValueError('unknown listing footer')
    roots = {}
    for name in names:
        if not name or '/' in name or not all(c == '_' or unicodedata.category(c).startswith('L') or unicodedata.category(c) == 'Nd' for c in name):
            raise ValueError('invalid root name: ' + repr(name))
        kind = next((k for k in ('Test', 'Benchmark', 'Fuzz', 'Example') if name.startswith(k)), None)
        suffix = name[len(kind):] if kind else ''
        if kind is None or (suffix and unicodedata.category(suffix[0]) == 'Ll'):
            raise ValueError('unknown root kind: ' + name)
        if name in roots:
            raise ValueError('duplicate root: ' + name)
        roots[name] = kind
    if not roots or all(k == 'Benchmark' for k in roots.values()):
        raise ValueError('empty runnable inventory')
    return roots


def verify_receipt(receipt, base, argv):
    if (receipt.get('argv') != argv or receipt.get('state') != 'finished' or receipt.get('exit_code') != 0
            or receipt.get('capture_stability') == 'quarantined-mutable'):
        raise ValueError('unsuccessful or mismatched inventory command')
    for stream in ('stdout', 'stderr'):
        path = Path(receipt[stream])
        if path.is_absolute() or '..' in path.parts:
            raise ValueError('unsafe evidence path')
        if digest(base / path) != receipt[stream + '_sha256']:
            raise ValueError('inventory raw evidence changed')
    return (base / receipt['stdout']).read_text(encoding='utf-8')


def validate_invocation_plan(plan, inventory, shard, context, base):
    """Independent coverage proof. No generator imports or calls."""
    errors = []
    try:
        policy_errors = validate_batch_policy()
        if policy_errors:
            raise ValueError('; '.join(policy_errors))
        policy = POLICY['root_batching']
        if context['platform'] not in EXPECTED_COUNTS or not shard['id'].startswith(context['platform'] + '-'):
            raise ValueError('wrong platform/shard')
        if not re.fullmatch('[0-9a-f]{40}', context['revision']) or context['toolchain'] != policy['toolchain']:
            raise ValueError('wrong revision/toolchain')
        if not context.get('goarch') or context.get('goflags') != '' or context.get('cgo_enabled') not in {'0', '1'}:
            raise ValueError('unsupported build context')
        if plan.get('version') != 1 or inventory.get('version') != 1:
            raise ValueError('unknown invocation schema')
        if plan['context'] != context or inventory['context'] != context or plan['shard'] != shard['id']:
            raise ValueError('source/platform/revision context mismatch')
        if inventory['inventory_id'] != inventory_identity(inventory) or plan['inventory_id'] != inventory['inventory_id']:
            raise ValueError('inventory identity mismatch')
        packages = shard['packages']
        if len(packages) != len(set(packages)) or not packages:
            raise ValueError('invalid exact-once package assignment')
        affected = set(packages) & BATCH_PACKAGES
        if set(inventory['packages']) != affected:
            raise ValueError('inventory package allowlist mismatch')
        blocked = sorted(p for p in affected if inventory['packages'][p].get('error'))
        if plan['blocked_packages'] != blocked:
            raise ValueError('inventory failures must remain blocked')
        expected = {}
        for package in sorted(affected - set(blocked)):
            record = inventory['packages'][package]
            raw = verify_receipt(record['listing'], base, ['go', 'test', '-json', '-list', '.', '-count=1', '-timeout=10m', package])
            # Any compiler diagnostic is retained, but not silently admitted.
            if (base / record['listing']['stderr']).read_bytes():
                raise ValueError('listing has stderr diagnostics')
            roots = parse_listing(raw, package)
            if roots != record['roots']:
                raise ValueError('structured roots differ from raw listing')
            metadata = verify_receipt(record['metadata'], base, ['go', 'list', '-deps', '-test', '-json', '-compiled', package])
            if selected_sources(metadata, package) != record['sources']:
                raise ValueError('compiler-selected source identity mismatch')
            expected[package] = sorted(n for n, k in roots.items() if k != 'Benchmark')
        invocations = plan['invocations']
        ids = [i['id'] for i in invocations]
        if len(ids) != len(set(ids)) or any(not re.fullmatch('[A-Za-z0-9_.-]+', i) for i in ids):
            raise ValueError('invalid or duplicate invocation IDs')
        ordinary = [p for p in packages if p not in BATCH_PACKAGES]
        ordinary_invocations = [i for i in invocations if i['selector'] is None]
        if len(ordinary_invocations) != bool(ordinary):
            raise ValueError('ordinary packages must run in exactly one invocation')
        if ordinary and ordinary_invocations[0] != {'id': 'ordinary', 'packages': ordinary, 'roots': [], 'selector': None}:
            raise ValueError('ordinary package ownership changed')
        seen = {p: [] for p in expected}
        for invocation in invocations:
            if invocation['selector'] is None:
                continue
            ps, roots = invocation['packages'], invocation['roots']
            if len(ps) != 1 or ps[0] not in expected or not 1 <= len(roots) <= policy['max_roots']:
                raise ValueError('unknown package or invalid batch size')
            if roots != sorted(set(roots)) or any('/' in r for r in roots):
                raise ValueError('duplicate, unordered or slash root')
            escaped = [''.join(('\\' + c) if c in '\\\\.+*?()|[]{}^$' else c for c in r) for r in roots]
            if invocation['selector'] != '^(' + '|'.join(escaped) + ')$':
                raise ValueError('selector not exact anchored Go RE2')
            seen[ps[0]].extend(roots)
        if seen != expected:
            raise ValueError('root coverage must be exhaustive, disjoint and sorted')
        # Independently enforce deterministic chunk boundaries, IDs and invocation order.
        expected_ids = ['ordinary'] if ordinary else []
        for p in sorted(expected):
            chunks = [i for i in invocations if i['packages'] == [p]]
            offset = 0
            for index, item in enumerate(chunks, 1):
                if item['roots'] != expected[p][offset:offset + policy['max_roots']]:
                    raise ValueError('noncanonical root partition')
                expected_id = p.replace('/', '_') + '-%04d' % index
                if item['id'] != expected_id:
                    raise ValueError('noncanonical invocation ID')
                expected_ids.append(expected_id)
                offset += len(item['roots'])
        if ids != expected_ids:
            raise ValueError('noncanonical invocation order')
    except (KeyError, TypeError, ValueError, OSError) as exc:
        errors.append(str(exc))
    return errors


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default="scripts/ci-shards.json")
    parser.add_argument("--packages-file")
    parser.add_argument("--platform", choices=["linux", "darwin"])
    parser.add_argument("--shard")
    parser.add_argument("--invocation-plan")
    parser.add_argument("--inventory")
    parser.add_argument("--context", help="independently captured expected execution context JSON")
    args = parser.parse_args()
    policy_errors = validate_batch_policy()
    if policy_errors:
        for error in policy_errors:
            print('FAIL ' + error)
        return 1
    manifest = json.loads(Path(args.manifest).read_text())
    packages = json.loads(Path(args.packages_file).read_text()) if args.packages_file else {o: current_packages(o) for o in EXPECTED_COUNTS}
    errors = validate(manifest, packages)
    if errors:
        for error in errors:
            print(f"FAIL {error}")
        return 1
    if args.invocation_plan:
        if not (args.platform and args.shard and args.inventory and args.context):
            parser.error('invocation validation requires platform, shard, inventory and context')
        shard = next(s for s in manifest['platforms'][args.platform]['shards'] if s['id'] == args.shard)
        errors = validate_invocation_plan(json.loads(Path(args.invocation_plan).read_text()),
                                         json.loads(Path(args.inventory).read_text()), shard,
                                         json.loads(Path(args.context).read_text()), Path(args.inventory).parent)
        for error in errors:
            print('FAIL ' + error)
        if not errors:
            print('PASS exhaustive invocation plan')
        return bool(errors)
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
