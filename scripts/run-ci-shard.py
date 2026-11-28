#!/usr/bin/env python3
"""Hosted native root batching; local tests inject a fake launcher (no Go).

Artifact migration: the existing nine artifact names now contain inventory.json,
plan.json, checker.json, manifest.json, go-test-summary.json and nested raw stdout/
stderr plus command receipts. Do NOT concatenate invocation stdout: repeated package
terminals have distinct invocation identities. Legacy single-stream summarization is
still available for old artifacts; new consumers must use manifest + plan.

Listing executes init. Every batch repeats init and TestMain. Metadata identifies
compiler-selected inputs, not initialization compatibility. Cap 4 is provisional;
complete hosted accounting and runtime within unchanged job limits remain required.
No global test selector is exported; self-executing helpers keep explicit selectors.
"""
import argparse
import copy
import importlib.util
import json
import os
import re
import signal
import subprocess
from pathlib import Path

HERE = Path(__file__).resolve().parent


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), HERE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


CHECK = load('check-ci-shards')
GENERATE = load('generate-ci-shards')
SUMMARY = load('summarize-go-test-json')


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + '.tmp')
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + '\n')
    temporary.replace(path)


# Existing hosted containment boundary, not a local descendant-join claim:
# https://docs.github.com/en/actions/reference/workflow-cancellation-reference
# documents entry-process SIGINT/TERM and eventual process-tree/job cancellation.
# https://docs.github.com/en/actions/concepts/runners/github-hosted-runners
# documents fresh hosted VMs. Neither is a per-invocation cleanup receipt. On
# timeout/cancel/unresolved capture, stop work and await external job teardown;
# raw uploads are diagnostic/mutable, not qualified or immutable evidence.
CONTAINMENT = 'hosted-job-teardown-external-unobserved; no descendant join claimed'


class CaptureUnresolved(Exception):
    def __init__(self, receipt):
        self.receipt = receipt
        super().__init__(receipt.get('error', 'capture cleanup unresolved'))


def admit_capture(receipt):
    if receipt.get('capture_stability') == 'quarantined-mutable':
        raise CaptureUnresolved(receipt)


def quarantine(receipt, reason):
    receipt['capture_stability'] = 'quarantined-mutable'
    receipt['descendant_cleanup'] = 'externally-unresolved'
    receipt['error'] = reason
    if receipt['state'] == 'finished':
        receipt['state'] = 'quarantined'
    receipt.pop('stdout_sha256', None)
    receipt.pop('stderr_sha256', None)


def abnormal_termination_output(path):
    """Bounded observation of Go's timeout/killed diagnostics, not a stream join.

    Inspect at most the file size observed on open, never chase a descendant's
    growing file to EOF. Go test2json output records are line framed. The actual
    Go 1.26.5 timeout and cmd/go killed prefixes also apply to retained stderr.
    """
    marker = re.compile(r'^(?:panic: test timed out after\s|\*\*\* Test killed|signal: (?:killed|terminated))')
    with path.open('rb') as stream:
        remaining = os.fstat(stream.fileno()).st_size
        while remaining:
            raw = stream.readline(min(remaining, 1024 * 1024))
            if not raw:
                break
            remaining -= len(raw)
            text = raw.decode('utf-8', errors='replace')
            try:
                event = json.loads(text)
                text = event.get('Output', '') if isinstance(event, dict) else ''
            except ValueError:
                pass
            if isinstance(text, str) and any(marker.match(line) for line in text.splitlines()):
                return True
    return False


def launch(argv, stdout, stderr):
    """Unchanged direct-child launcher; return/kill is NOT descendant containment.

    No process-group signaling, session manipulation, shell or environment override.
    subprocess.run's exception cleanup is limited to its direct child. capture()
    quarantines interrupted streams instead of certifying them after that cleanup.
    """
    with stdout.open('wb') as out, stderr.open('wb') as err:
        return subprocess.run(argv, stdout=out, stderr=err, check=False).returncode


def capture(argv, base, name, launcher):
    directory = base / name
    directory.mkdir(parents=True, exist_ok=True)
    out, err = directory / 'stdout.jsonl', directory / 'stderr.txt'
    receipt = {'argv': argv, 'state': 'running', 'exit_code': None,
               'stdout': str(out.relative_to(base)), 'stderr': str(err.relative_to(base)),
               'capture_stability': 'pending', 'descendant_cleanup': 'not_observed',
               'containment_boundary': CONTAINMENT, 'hash_semantics': 'observed-bytes-not-descendant-join'}
    save(directory / 'command.json', receipt)
    out.touch(); err.touch()
    try:
        receipt['exit_code'] = launcher(argv, out, err)
        if type(receipt['exit_code']) is not int:
            raise ValueError('launcher did not return an integer status')
        receipt['state'] = 'finished'
        receipt['capture_stability'] = 'parent-exit-observation'
        if receipt['exit_code'] < 0 or receipt['exit_code'] >= 128:
            quarantine(receipt, 'signal/abnormal parent termination; descendant cleanup unresolved')
        elif abnormal_termination_output(out) or abnormal_termination_output(err):
            quarantine(receipt, 'timeout/killed diagnostic; descendant cleanup unresolved')
    except KeyboardInterrupt:
        receipt['state'] = 'cancelled'
        quarantine(receipt, 'cancelled during capture; descendant cleanup unresolved')
    except subprocess.TimeoutExpired:
        receipt['state'] = 'quarantined'
        quarantine(receipt, 'launcher timeout; descendant cleanup unresolved')
    except Exception as exc:
        # Unknown launcher/capture errors do not establish descendant cleanup.
        receipt['state'] = 'capture_error'
        quarantine(receipt, 'capture error; cleanup unresolved: ' + str(exc))
    finally:
        if receipt['capture_stability'] != 'quarantined-mutable':
            for stream in ('stdout', 'stderr'):
                try:
                    receipt[stream + '_sha256'] = CHECK.digest(base / receipt[stream])
                except OSError as exc:
                    receipt['state'] = 'capture_error'
                    quarantine(receipt, 'capture hash error; cleanup unresolved: ' + str(exc))
                    break
        save(directory / 'command.json', receipt)
    return receipt


def inventory_package(package, base, launcher):
    prefix = 'inventory/' + package.replace('/', '_')
    record = {}
    try:
        metadata_argv = ['go', 'list', '-deps', '-test', '-json', '-compiled', package]
        record['metadata'] = capture(metadata_argv, base, prefix + '/metadata', launcher)
        admit_capture(record['metadata'])
        metadata = CHECK.verify_receipt(record['metadata'], base, metadata_argv)
        record['sources'] = CHECK.selected_sources(metadata, package)
        argv = ['go', 'test', '-json', '-list', '.', '-count=1', '-timeout=10m', package]
        record['listing'] = capture(argv, base, prefix + '/listing', launcher)
        admit_capture(record['listing'])
        raw = CHECK.verify_receipt(record['listing'], base, argv)
        if (base / record['listing']['stderr']).read_bytes():
            raise ValueError('listing stderr diagnostics require review')
        record['roots'] = CHECK.parse_listing(raw, package)
        # Recheck after init: source mutation during listing cannot certify itself.
        if record['sources'] != CHECK.selected_sources(metadata, package):
            raise ValueError('source changed during listing')
    except (OSError, ValueError, KeyError, TypeError) as exc:
        record['error'] = str(exc)
    return record


def quarantine_manifest(manifest, reason, receipt=None):
    manifest.update(state='quarantined', cleanup_status='externally-unresolved',
                    evidence_qualification='quarantined-mutable', containment_boundary=CONTAINMENT)
    manifest['errors'].append(reason)
    if receipt is not None:
        manifest['unresolved_capture'] = receipt
    for invocation in manifest['invocations']:
        if invocation['state'] == 'pending':
            invocation.update(state='not_run', reason='prior capture cleanup unresolved')


def run_shard(shard, context, base, launcher=launch):
    base = Path(base)
    base.mkdir(parents=True, exist_ok=True)
    inventory = {'version': 1, 'context': copy.deepcopy(context), 'packages': {}}
    manifest = {'version': 1, 'context': copy.deepcopy(context), 'state': 'preparing',
                'errors': [], 'blocked_packages': [], 'invocations': [],
                'compatibility': 'policy admission pending',
                'cleanup_status': 'not_observed', 'evidence_qualification': 'pending',
                'containment_boundary': CONTAINMENT}
    plan = {'version': 1, 'shard': shard['id'], 'context': copy.deepcopy(context),
            'invocations': [], 'blocked_packages': []}
    save(base / 'manifest.json', manifest)
    save(base / 'context.json', context)
    save(base / 'checker.json', {'state': 'pending', 'errors': ['inventory/plan not yet checked']})
    save(base / 'checker-final.json', {'state': 'not_run', 'errors': ['execution incomplete']})
    try:
        policy_errors = CHECK.validate_batch_policy()
        if policy_errors:
            save(base / 'checker.json', {'state': 'checked', 'errors': policy_errors})
            raise ValueError('; '.join(policy_errors))
        manifest['compatibility'] = CHECK.POLICY['root_batching']['compatibility']
        for package in sorted(set(shard['packages']) & CHECK.BATCH_PACKAGES):
            inventory['packages'][package] = inventory_package(package, base, launcher)
            save(base / 'inventory.json', inventory)
        inventory['inventory_id'] = CHECK.inventory_identity(inventory)
        save(base / 'inventory.json', inventory)
        plan = GENERATE.invocation_plan(inventory, shard)
        save(base / 'plan.json', plan)
        errors = CHECK.validate_invocation_plan(plan, inventory, shard, context, base)
        save(base / 'checker.json', {'state': 'checked', 'errors': errors,
                                    'inventory_id': inventory['inventory_id']})
        manifest['blocked_packages'] = plan['blocked_packages']
        manifest['invocations'] = [dict(i, state='pending', exit_code=None,
                                        stdout='invocations/' + i['id'] + '/stdout.jsonl',
                                        stderr='invocations/' + i['id'] + '/stderr.txt') for i in plan['invocations']]
        save(base / 'manifest.json', manifest)
        if errors:
            raise ValueError('independent plan validation: ' + '; '.join(errors))
        manifest['state'] = 'running'
        for invocation in manifest['invocations']:
            argv = ['go', 'test', '-json', '-timeout=10m']
            if invocation['selector'] is not None:
                argv += ['-count=1', '-run', invocation['selector']]
            argv += invocation['packages']
            invocation.update(state='running', argv=argv)
            save(base / 'manifest.json', manifest)
            receipt = capture(argv, base, 'invocations/' + invocation['id'], launcher)
            invocation.update(receipt)
            save(base / 'manifest.json', manifest)
            admit_capture(receipt)
            # Ordinary test FAIL continues. Timeout/cancel/uncertain capture does
            # not: no further Go invocation or source revalidation in this job.
        post_errors = CHECK.validate_invocation_plan(plan, inventory, shard, context, base)
        save(base / 'checker-final.json', {'state': 'checked', 'errors': post_errors})
        manifest['errors'].extend(post_errors)
        manifest['state'] = 'finished'
        manifest['evidence_qualification'] = 'parent-exit-observation'
    except CaptureUnresolved as exc:
        quarantine_manifest(manifest, str(exc), exc.receipt)
    except KeyboardInterrupt:
        quarantine_manifest(manifest, 'cancelled outside capture; pending/running work unfinished')
    except OSError as exc:
        quarantine_manifest(manifest, 'evidence persistence/read failure; cleanup unresolved: ' + str(exc))
    except (ValueError, KeyError, TypeError) as exc:
        manifest['state'] = 'incomplete'
        manifest['errors'].append(str(exc))
    finally:
        # These checkpoints also exist on preparation failure and cancellation.
        save(base / 'inventory.json', inventory)
        save(base / 'plan.json', plan)
        save(base / 'manifest.json', manifest)
        summary = SUMMARY.summarize_manifest(manifest, plan, base)
        save(base / 'go-test-summary.json', summary)
    print(json.dumps({'shard': shard['id'], 'status': summary['status'],
                      'invocation_count': summary['invocation_count'], 'evidence': str(base)}))
    return 0 if summary['status'] == 'success' else 1


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--platform', choices=['linux', 'darwin'], required=True)
    parser.add_argument('--shard', required=True)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--manifest', default=str(HERE / 'ci-shards.json'))
    args = parser.parse_args()
    # Unit tests call run_shard with an injected launcher. No accidental local Go.
    if os.environ.get('GITHUB_ACTIONS') != 'true':
        parser.error('native inventory/execution is hosted-CI-only')
    base = Path(args.output).resolve()
    base.mkdir(parents=True, exist_ok=True)
    # Even setup cancellation must leave discoverable incomplete evidence.
    save(base / 'inventory.json', {'version': 1, 'state': 'pending', 'packages': {}})
    save(base / 'plan.json', {'version': 1, 'state': 'pending', 'shard': args.shard, 'invocations': [], 'blocked_packages': []})
    save(base / 'checker.json', {'state': 'pending', 'errors': ['setup incomplete']})
    save(base / 'manifest.json', {'version': 1, 'state': 'preparing', 'invocations': [], 'errors': []})
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    try:
        policy_errors = CHECK.validate_batch_policy()
        if policy_errors:
            raise ValueError('; '.join(policy_errors))
        ownership = json.loads(Path(args.manifest).read_text())
        shard = next(s for s in ownership['platforms'][args.platform]['shards'] if s['id'] == args.shard)
        # Package exact-once accounting remains independently checked before planning.
        packages = {p: CHECK.current_packages(p) for p in CHECK.EXPECTED_COUNTS}
        errors = CHECK.validate(ownership, packages)
        save(base / 'package-checker.json', {'errors': errors, 'packages': packages})
        if errors:
            raise ValueError('; '.join(errors))
        revision = capture(['git', 'rev-parse', 'HEAD'], base, 'context/revision', launch)
        admit_capture(revision)
        head = CHECK.verify_receipt(revision, base, ['git', 'rev-parse', 'HEAD']).strip()
        if head != args.revision:
            raise ValueError('checkout revision differs from expected revision')
        clean_argv = ['git', 'diff', '--quiet', 'HEAD', '--']
        clean = capture(clean_argv, base, 'context/tracked-cleanliness', launch)
        admit_capture(clean)
        CHECK.verify_receipt(clean, base, clean_argv)
        argv = ['go', 'env', '-json', 'GOOS', 'GOARCH', 'GOVERSION', 'GOFLAGS', 'CGO_ENABLED', 'GOTOOLCHAIN',
                'GOEXPERIMENT', 'GOAMD64', 'GOARM64', 'CGO_CFLAGS', 'CGO_CPPFLAGS', 'CGO_CXXFLAGS',
                'CGO_FFLAGS', 'CGO_LDFLAGS', 'CC', 'CXX']
        receipt = capture(argv, base, 'context/toolchain', launch)
        admit_capture(receipt)
        env = json.loads(CHECK.verify_receipt(receipt, base, argv))
        if env['GOOS'] != args.platform:
            raise ValueError('inventory must run on the native job platform')
        if env['GOVERSION'] != CHECK.POLICY['root_batching']['toolchain'] or env['GOFLAGS']:
            raise ValueError('unsupported toolchain or nonempty GOFLAGS requires explicit review')
        context = {'platform': env['GOOS'], 'goarch': env['GOARCH'], 'revision': head,
                   'toolchain': env['GOVERSION'], 'goflags': env['GOFLAGS'], 'cgo_enabled': env['CGO_ENABLED'],
                   'go_env': env}
        return run_shard(shard, context, base)
    except (OSError, ValueError, KeyError, StopIteration, subprocess.SubprocessError, KeyboardInterrupt, CaptureUnresolved) as exc:
        incomplete = {'version': 1, 'state': 'incomplete', 'errors': [], 'invocations': [], 'blocked_packages': []}
        quarantine_manifest(incomplete, str(exc) or 'cancelled during setup', getattr(exc, 'receipt', None))
        save(base / 'manifest.json', incomplete)
        save(base / 'go-test-summary.json', {'version': 2, 'status': 'failure', 'partial': True,
                                            'evidence_qualification': 'quarantined-mutable', 'errors': incomplete['errors']})
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
