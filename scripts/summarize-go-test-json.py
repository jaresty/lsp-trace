#!/usr/bin/env python3
"""Summarize one captured `go test -json` stream without rerunning tests."""
import argparse
import datetime as dt
import json
import hashlib
import re
import os
import stat
from contextlib import contextmanager
from pathlib import Path


def parse_time(value):
    if not value:
        return None
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def evidence_path(base, name):
    """Admit a manifest-relative regular-file path, never an outside operand.

    The caller's artifact root is trusted/canonicalized; symlinks anywhere BELOW
    it are unsupported, including in pending invocations. No file is opened here.
    """
    if not isinstance(name, str) or not name or '\x00' in name:
        raise ValueError('unsafe invocation stream path')
    path = Path(name)
    if path.is_absolute() or '..' in path.parts or not path.parts:
        raise ValueError('unsafe invocation stream path')
    root = Path(base).resolve()
    cursor = root
    for part in path.parts:
        cursor = cursor / part
        if cursor.is_symlink():
            raise ValueError('symlink invocation stream path')
    return root, path.parts


@contextmanager
def open_evidence(base, name, binary=False):
    """No-follow descriptor walk also closes the preflight/open symlink race.

    Supported on the existing Linux/Darwin artifact readers. No fallback to an
    unsafe path open on a platform without dir_fd/O_NOFOLLOW support.
    """
    root, parts = evidence_path(base, name)
    directory = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    fd = None
    try:
        for part in parts[:-1]:
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
            os.close(directory)
            directory = child
        fd = os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise ValueError('invocation stream is not a regular file')
        stream = os.fdopen(fd, 'rb' if binary else 'r', **({} if binary else {'encoding': 'utf-8'}))
        fd = None
        with stream:
            yield stream
    finally:
        if fd is not None:
            os.close(fd)
        os.close(directory)


def reconcile_invocation(invocation, base):
    """Reconcile actual Go 1.26 root/child events, without parsing child output as JSON."""
    errors = []
    packages = invocation['packages']
    expected = set(invocation['roots']) if invocation['selector'] is not None else None
    started, terminal, runs, ends, outputs = {}, {}, {}, {}, {}
    failures = set()
    try:
        if invocation.get('capture_stability') == 'quarantined-mutable':
            raise ValueError('capture quarantined; external cleanup unobserved')
        with open_evidence(base, invocation['stdout']) as stream:
            for line in stream:
                event = json.loads(line)
                if not isinstance(event, dict):
                    raise ValueError('non-object event')
                package, action, test = event.get('Package'), event.get('Action'), event.get('Test')
                # Go 1.26 may emit build diagnostics as build-output/build-fail.
                if action in {'build-output', 'build-fail'}:
                    if action == 'build-fail':
                        failures.update(packages)
                    continue
                if package not in packages:
                    errors.append('unexpected package: ' + str(package)); continue
                if package in terminal:
                    errors.append('event after package terminal: ' + package)
                if action == 'start' and not test:
                    started[package] = started.get(package, 0) + 1
                    continue
                if started.get(package) != 1:
                    errors.append('event without unique package start: ' + package)
                if test:
                    if not isinstance(test, str):
                        raise ValueError('invalid test name')
                    root = test.split('/')[0]
                    if expected is not None and root not in expected:
                        errors.append('unexpected root: ' + root)
                    key = (package, test)
                    if action == 'run':
                        runs[key] = runs.get(key, 0) + 1
                        if runs[key] != 1 or key in ends:
                            errors.append('duplicate test run: ' + test)
                    elif action in {'pass', 'fail', 'skip'}:
                        if key in ends or runs.get(key) != 1:
                            errors.append('duplicate or unstarted test terminal: ' + test)
                        ends[key] = action
                        if action == 'fail':
                            failures.add(package)
                    elif action == 'output':
                        # Bounded skip explanation, not a second event parser.
                        outputs[key] = (outputs.get(key, '') + event.get('Output', ''))[-2048:]
                    elif action not in {'pause', 'cont', 'attr', 'artifacts'}:
                        errors.append('unknown test event: ' + str(action))
                elif action in {'pass', 'fail', 'skip'}:
                    if package in terminal:
                        errors.append('duplicate package terminal: ' + package)
                    terminal[package] = action
                    if action == 'fail':
                        failures.add(package)
                elif action != 'output':
                    errors.append('unknown package event: ' + str(action))
        if invocation.get('state') != 'finished' or invocation.get('exit_code') != 0:
            errors.append('invocation failed or unfinished')
            failures.update(packages)
        if set(terminal) != set(packages) or any(started.get(p) != 1 for p in packages):
            errors.append('missing package start/terminal')
        for key in runs.keys() | ends.keys():
            if runs.get(key) != 1 or key not in ends:
                errors.append('unfinished test: ' + key[1])
        if expected is not None:
            actual = {test for (_, test) in runs if '/' not in test}
            if actual != expected:
                errors.append('missing or extra roots: ' + repr(sorted(actual ^ expected)))
    except (OSError, ValueError, TypeError) as exc:
        errors.append('capture unreadable: ' + str(exc))
    if errors:
        failures.update(packages)
    return {'id': invocation['id'], 'errors': errors,
            'packages': [{'package': p, 'status': 'fail' if p in failures else terminal.get(p, 'unfinished')} for p in packages],
            'root_count': sum('/' not in test for _, test in runs),
            'skips': [{'package': p, 'test': t, 'explanation': outputs.get((p, t), '') or 'Go emitted an explicit skip terminal after run'}
                      for (p, t), action in sorted(ends.items()) if action == 'skip']}


def summarize_manifest(manifest, plan, base):
    """v2 layout: never collapse repeated package terminals across invocations."""
    errors = list(manifest.get('errors', []))
    planned = plan.get('invocations', [])
    actual = manifest.get('invocations', [])
    if [i['id'] for i in actual] != [i['id'] for i in planned]:
        errors.append('manifest does not account for every planned invocation')
    expected_by_id = {i['id']: i for i in planned}
    if len(expected_by_id) != len(planned):
        errors.append('duplicate planned invocation ID')
    # Admit ALL stream paths before any reconciliation or hashing, including
    # pending/cancelled entries. One bad operand refuses the entire read batch.
    paths_ok = True
    for item in actual:
        for stream in ('stdout', 'stderr'):
            try:
                evidence_path(base, item[stream])
            except (KeyError, OSError, ValueError, TypeError) as exc:
                errors.append(str(exc))
                paths_ok = False
    quarantined = (manifest.get('cleanup_status') == 'externally-unresolved'
                   or manifest.get('evidence_qualification') == 'quarantined-mutable'
                   or any(i.get('capture_stability') == 'quarantined-mutable' for i in actual))
    if quarantined:
        # Never read/hash possibly live inherited stream descriptors. Uploads may
        # retain diagnostic bytes, but neither a stable capture nor a join is proven.
        errors.append('evidence quarantined; external job teardown/cleanup unobserved')
        packages = sorted({p for i in planned for p in i['packages']} | set(manifest.get('blocked_packages', [])))
        return {'version': 2, 'layout': 'per-invocation-json-v1', 'status': 'failure', 'partial': True,
                'evidence_qualification': 'quarantined-mutable', 'cleanup_status': 'externally-unresolved',
                'errors': errors, 'invocation_count': len(planned),
                'finished_invocation_count': sum(i.get('state') == 'finished' for i in actual),
                'logical_package_count': len(packages), 'packages': [{'package': p, 'status': 'fail'} for p in packages],
                'invocations': [{'id': i['id'], 'state': i.get('state'), 'reconciled': False} for i in actual]}
    results = []
    for item in actual:
        definition = {k: item.get(k) for k in ('id', 'packages', 'roots', 'selector')}
        if definition != expected_by_id.get(item['id']):
            errors.append('invocation definition differs from checked plan: ' + item['id'])
        if paths_ok:
            results.append(reconcile_invocation(item, base))
    blocked = manifest.get('blocked_packages', [])
    if blocked or blocked != plan.get('blocked_packages', []):
        errors.append('blocked inventory packages: ' + repr(blocked))
    logical = {p: 'unfinished' for i in planned for p in i['packages']}
    logical.update({p: 'fail' for p in blocked})
    actual_ids = {i['id'] for i in actual}
    missing_packages = {p for i in planned if i['id'] not in actual_ids for p in i['packages']}
    if not paths_ok:
        missing_packages.update(logical)
    for item in actual:
        if paths_ok and item.get('state') == 'finished':
            for stream in ('stdout', 'stderr'):
                try:
                    digest = hashlib.sha256()
                    with open_evidence(base, item[stream], binary=True) as raw:
                        for chunk in iter(lambda: raw.read(1024 * 1024), b''):
                            digest.update(chunk)
                    if item.get(stream + '_sha256') != digest.hexdigest():
                        raise ValueError('invocation stream identity mismatch')
                except (KeyError, OSError, ValueError) as exc:
                    errors.append(str(exc))
                    missing_packages.update(item['packages'])
    for result in results:
        errors.extend(result['errors'])
        for package in result['packages']:
            name, status = package['package'], package['status']
            previous = logical.get(name)
            logical[name] = 'fail' if previous == 'fail' or status not in {'pass', 'skip'} else ('pass' if 'pass' in {previous, status} else 'skip')
    for package in missing_packages:
        logical[package] = 'fail'
    if manifest.get('state') != 'finished':
        errors.append('shard execution unfinished')
    context = manifest.get('context', {})
    return {'version': 2, 'layout': 'per-invocation-json-v1', 'status': 'failure' if errors or 'fail' in logical.values() else 'success',
            'os': context.get('platform'), 'commit': context.get('revision'), 'shard': plan.get('shard'),
            'evidence_qualification': manifest.get('evidence_qualification', 'parent-exit-observation'),
            'cleanup_status': manifest.get('cleanup_status', 'not_observed'),
            'partial': bool(errors), 'errors': errors, 'invocation_count': len(planned),
            'finished_invocation_count': sum(i.get('state') == 'finished' for i in actual),
            'logical_package_count': len(logical), 'packages': [{'package': p, 'status': logical[p]} for p in sorted(logical)],
            'invocations': results}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input")
    parser.add_argument("--manifest", help="v2 per-invocation evidence manifest")
    parser.add_argument("--plan")
    parser.add_argument("--output", required=True)
    parser.add_argument("--os")
    parser.add_argument("--commit")
    parser.add_argument("--job")
    parser.add_argument("--shard")
    parser.add_argument("--exit-code", type=int)
    args = parser.parse_args()
    if args.manifest:
        base = Path(args.manifest).parent
        manifest = json.loads(Path(args.manifest).read_text())
        plan = json.loads(Path(args.plan or base / 'plan.json').read_text())
        summary = summarize_manifest(manifest, plan, base)
        Path(args.output).write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
        return 0 if summary['status'] == 'success' else 1
    if not args.input or args.exit_code is None:
        parser.error('legacy mode requires --input and --exit-code')
    events = []
    malformed_lines = 0
    for line in Path(args.input).read_text(errors="replace").splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            malformed_lines += 1
            continue
        if isinstance(event, dict):
            events.append(event)
        else:
            malformed_lines += 1
    cached = set()
    timeout_diagnostics = set()
    timeout_pattern = re.compile(r"^panic: test timed out after\s+\S+", re.IGNORECASE)
    for event in events:
        package = event.get("Package")
        output = event.get("Output", "")
        if package and "(cached)" in output:
            cached.add(package)
        if package and any(timeout_pattern.match(line.strip()) for line in output.splitlines()):
            timeout_diagnostics.add(package)
    terminal_events = {}
    duplicate_terminals = set()
    for event in events:
        package = event.get("Package")
        action = event.get("Action")
        if package and action in {"pass", "fail", "skip"} and "Test" not in event:
            if package in terminal_events:
                duplicate_terminals.add(package)
                # Legacy mode cannot identify invocation boundaries. Refuse a
                # concatenated stream, and never erase an earlier failure.
                if terminal_events[package]['Action'] == 'fail' or action != 'fail':
                    continue
            terminal_events[package] = event
    timed_out = {
        package
        for package in timeout_diagnostics
        if args.exit_code != 0 and terminal_events.get(package, {}).get("Action") == "fail"
    }
    terminal = {}
    for package, event in terminal_events.items():
        action = event["Action"]
        terminal[package] = {
            "package": package,
            "status": "timeout" if package in timed_out else action,
            "elapsed": event.get("Elapsed"),
            "cached": package in cached,
        }
    times = [parse_time(e.get("Time")) for e in events]
    times = [value for value in times if value is not None]
    total = (max(times) - min(times)).total_seconds() if len(times) >= 2 else None
    failed_terminal = any(e['Action'] == 'fail' for e in terminal_events.values())
    status = "timeout" if timed_out else ("failure" if args.exit_code or failed_terminal or duplicate_terminals or malformed_lines else "success")
    summary = {
        "os": args.os,
        "commit": args.commit,
        "job": args.job,
        "shard": args.shard,
        "status": status,
        "exit_code": args.exit_code,
        "packages": [terminal[p] for p in sorted(terminal)],
        "shard_total_seconds": total,
        "setup_seconds": None,
        "cached": bool(cached),
        "malformed_lines": malformed_lines,
        "partial": malformed_lines > 0 or bool(duplicate_terminals),
        "duplicate_package_terminals": sorted(duplicate_terminals),
    }
    Path(args.output).write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    raise SystemExit(main())
