#!/usr/bin/env python3
"""Pure Python controls. Fake launcher writes streams; never invokes Go."""
import copy
import importlib.util
import json
import tempfile
import unittest
from unittest import mock
from pathlib import Path

HERE = Path(__file__).resolve().parent
PACKAGE = "lsp-trace/cmd/lsp-trace"
OTHER = "lsp-trace/internal/example"


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), HERE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def listing(names, package=PACKAGE):
    # Go 1.26.5 testing.listTests + cmd/go test.go:1731. No standalone PASS.
    return [{"Action": "start", "Package": package}] + [
        {"Action": "output", "Package": package, "Output": n + "\n"} for n in names
    ] + [{"Action": "output", "Package": package, "Output": "ok  \t" + package + "\t0.012s\n"},
         {"Action": "pass", "Package": package, "Elapsed": .012}]


def events(roots, package=PACKAGE, fail=False):
    result = [{"Action": "start", "Package": package}]
    for root in roots:
        result += [{"Action": "run", "Package": package, "Test": root},
                   {"Action": "output", "Package": package, "Test": root, "Output": "reason\n"},
                   {"Action": "skip" if root.endswith("Helper") else "pass", "Package": package, "Test": root}]
    result.append({"Action": "fail" if fail else "pass", "Package": package})
    return result


def lines(items):
    return ''.join(json.dumps(x) + '\n' for x in items)


class RunnerTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.runner = load('run-ci-shard')
        cls.checker = load('check-ci-shards')
        cls.generator = load('generate-ci-shards')

    def test_listing_uses_pinned_go_framing(self):
        roots = ['TestA', 'TestHelper', 'BenchmarkB', 'FuzzC', 'Example']
        parsed = self.checker.parse_listing(lines(listing(roots)), PACKAGE)
        self.assertEqual(parsed, {n: next(k for k in ('Test', 'Benchmark', 'Fuzz', 'Example') if n.startswith(k)) for n in roots})
        raw = listing(roots)
        raw[1:2] = [dict(raw[1], Output='Tes'), dict(raw[1], Output='tA\n')]
        self.assertEqual(self.checker.parse_listing(lines(raw), PACKAGE), parsed)

    def test_bad_listing_fails_closed(self):
        for names in ([], ['TestA', 'TestA'], ['TestA/sub'], ['garbage'], ['PASS'], ['Testlower']):
            with self.subTest(names=names), self.assertRaises(ValueError):
                self.checker.parse_listing(lines(listing(names)), PACKAGE)
        for raw in (lines(listing(['TestA'])[:-1]), '{}\n', lines(listing(['TestA'], OTHER)), '{'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                self.checker.parse_listing(raw, PACKAGE)

    def test_re2_escape_exact_and_slash_refusal(self):
        self.assertEqual(self.generator.root_selector(['TestA+B', 'TestC-D']), '^(TestA\\+B|TestC-D)$')
        with self.assertRaises(ValueError):
            self.generator.root_selector(['TestA/child'])

    def fixture(self, td, platform='linux'):
        root = Path(td)
        source = root / 'selected.go'
        source.write_text('package fixture\n')
        context = {'platform': platform, 'revision': 'a' * 40, 'goarch': 'amd64',
                   'toolchain': 'go1.26.5', 'goflags': '', 'cgo_enabled': '1'}
        shard = {'id': platform + '-1', 'packages': [PACKAGE, OTHER]}
        names = ['Test%03d' % n for n in range(65)] + ['TestHelper', 'Example', 'FuzzSeeds', 'BenchmarkExcluded']
        calls = []
        def fake(argv, stdout, stderr):
            calls.append(argv)
            stderr.write_bytes(b'')
            if argv[:3] == ['go', 'list', '-deps']:
                stdout.write_text(json.dumps({'ImportPath': PACKAGE, 'Dir': str(root), 'GoFiles': ['selected.go']}))
                return 0
            if '-list' in argv:
                stdout.write_text(lines(listing(names)))
            else:
                packages = [a for a in argv if a.startswith('lsp-trace/')]
                selected = argv[argv.index('-run') + 1][2:-2].split('|') if '-run' in argv else []
                stdout.write_text(''.join(lines(events(selected if p == PACKAGE else [], p)) for p in packages))
            return 0
        return context, shard, names, calls, fake

    def execute(self, td, wrap=None):
        context, shard, names, calls, fake = self.fixture(td)
        launcher = wrap(fake) if wrap else fake
        out = Path(td) / 'evidence'
        status = self.runner.run_shard(shard, context, out, launcher=launcher)
        manifest = json.loads((out / 'manifest.json').read_text())
        summary = json.loads((out / 'go-test-summary.json').read_text())
        return status, manifest, summary, calls, out

    def test_all_roots_batches_and_unsplit_once(self):
        with tempfile.TemporaryDirectory() as td:
            status, manifest, summary, calls, out = self.execute(td)
            self.assertEqual(status, 0, summary)
            self.assertEqual(summary['evidence_qualification'], 'parent-exit-observation')
            self.assertEqual(summary['cleanup_status'], 'not_observed')
            tests = [a for a in calls if a[:2] == ['go', 'test'] and '-list' not in a]
            self.assertEqual(len(tests), 18)
            self.assertNotIn('-run', tests[0])
            self.assertEqual(tests[0][-1], OTHER)
            self.assertTrue(all('-timeout=10m' in a and '-count=1' in a for a in tests[1:]))
            self.assertEqual(summary['logical_package_count'], 2)
            self.assertEqual(summary['invocation_count'], 18)
            plan = json.loads((out / 'plan.json').read_text())
            roots = [r for i in plan['invocations'] for r in i['roots']]
            self.assertEqual(len(roots), 68)
            self.assertNotIn('BenchmarkExcluded', roots)
            self.assertEqual(len(roots), len(set(roots)))
            self.assertTrue(all(i['state'] == 'finished' for i in manifest['invocations']))

    def test_first_failure_does_not_mask_or_stop_tails(self):
        def wrap(fake):
            first = True
            def launch(argv, stdout, stderr):
                nonlocal first
                result = fake(argv, stdout, stderr)
                if '-run' in argv and first:
                    first = False
                    records = [json.loads(x) for x in stdout.read_text().splitlines()]
                    records[-1]['Action'] = 'fail'
                    stdout.write_text(lines(records))
                    return 1
                return result
            return launch
        with tempfile.TemporaryDirectory() as td:
            status, manifest, summary, calls, _ = self.execute(td, wrap)
            self.assertEqual(status, 1)
            self.assertEqual(len([a for a in calls if '-run' in a]), 17)
            self.assertEqual(summary['status'], 'failure')
            self.assertEqual(next(p for p in summary['packages'] if p['package'] == PACKAGE)['status'], 'fail')

    def test_inventory_failure_blocks_only_affected_package(self):
        def wrap(fake):
            def launch(argv, stdout, stderr):
                result = fake(argv, stdout, stderr)
                return 1 if '-list' in argv else result
            return launch
        with tempfile.TemporaryDirectory() as td:
            status, manifest, summary, calls, _ = self.execute(td, wrap)
            self.assertEqual(status, 1)
            self.assertFalse(any('-run' in a for a in calls))
            self.assertTrue(any(a[-1] == OTHER for a in calls))
            self.assertEqual(manifest['blocked_packages'], [PACKAGE])

    def test_capture_failure_and_cancel_never_green(self):
        for exception in (OSError('capture failed'), KeyboardInterrupt()):
            def wrap(fake):
                def launch(argv, stdout, stderr):
                    if '-run' in argv:
                        raise exception
                    return fake(argv, stdout, stderr)
                return launch
            with self.subTest(exception=type(exception).__name__), tempfile.TemporaryDirectory() as td:
                status, manifest, summary, _, _ = self.execute(td, wrap)
                self.assertEqual(status, 1)
                self.assertNotEqual(summary['status'], 'success')
                self.assertTrue(any(i['state'] != 'finished' for i in manifest['invocations']))

    def test_native_launcher_is_argv_only_and_preserves_environment(self):
        with tempfile.TemporaryDirectory() as td, mock.patch.object(self.runner.subprocess, 'run') as process:
            process.return_value.returncode = 7
            argv = ['go', 'test', '-json', '-count=1', '-timeout=10m', '-run', '^(TestA)$', PACKAGE]
            self.assertEqual(self.runner.launch(argv, Path(td) / 'out', Path(td) / 'err'), 7)
            args, kwargs = process.call_args
            self.assertEqual(args, (argv,))
            self.assertNotIn('shell', kwargs)
            self.assertNotIn('env', kwargs)

    def test_both_allowlisted_packages_on_both_platforms(self):
        for platform in ('linux', 'darwin'):
            for package in self.checker.BATCH_PACKAGES:
                with self.subTest(platform=platform, package=package), tempfile.TemporaryDirectory() as td:
                    context, shard, names, calls, original = self.fixture(td, platform)
                    shard['packages'] = [package]
                    def fake(argv, stdout, stderr):
                        stderr.write_text('')
                        calls.append(argv)
                        if argv[:3] == ['go', 'list', '-deps']:
                            stdout.write_text(json.dumps({'ImportPath': package, 'Dir': td, 'GoFiles': ['selected.go']}))
                        elif '-list' in argv:
                            stdout.write_text(lines(listing(names, package)))
                        else:
                            selected = argv[argv.index('-run') + 1][2:-2].split('|')
                            stdout.write_text(lines(events(selected, package)))
                        return 0
                    self.assertEqual(self.runner.run_shard(shard, context, Path(td) / 'evidence', launcher=fake), 0)
                    self.assertEqual(len([a for a in calls if '-run' in a]), 17)
                    self.assertTrue(all(a[-1] == package for a in calls))

    def test_shard_without_affected_packages_stays_one_ordinary_invocation(self):
        with tempfile.TemporaryDirectory() as td:
            context, shard, _, calls, fake = self.fixture(td)
            shard['packages'] = [OTHER]
            self.assertEqual(self.runner.run_shard(shard, context, Path(td) / 'evidence', launcher=fake), 0)
            self.assertEqual(calls, [['go', 'test', '-json', '-timeout=10m', OTHER]])

    def test_checker_is_mandatory_before_any_invocation(self):
        original = self.runner.GENERATE.invocation_plan
        def corrupted(*args):
            plan = original(*args)
            plan['invocations'][-1]['roots'].pop()
            return plan
        with tempfile.TemporaryDirectory() as td, mock.patch.object(self.runner.GENERATE, 'invocation_plan', corrupted):
            status, _, _, calls, out = self.execute(td)
            self.assertEqual(status, 1)
            self.assertTrue(json.loads((out / 'checker.json').read_text())['errors'])
            self.assertFalse(any(a[:2] == ['go', 'test'] and '-list' not in a for a in calls))

    def test_inventory_parse_or_metadata_failure_still_runs_ordinary(self):
        for mode in ('unknown-output', 'metadata-failure'):
            def wrap(fake):
                def launch(argv, stdout, stderr):
                    result = fake(argv, stdout, stderr)
                    if mode == 'metadata-failure' and argv[:3] == ['go', 'list', '-deps']:
                        return 1
                    if mode == 'unknown-output' and '-list' in argv:
                        stdout.write_text(lines(listing(['TestA', 'initialization chatter'])))
                    return result
                return launch
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as td:
                status, _, _, calls, _ = self.execute(td, wrap)
                self.assertEqual(status, 1)
                self.assertFalse(any('-run' in a for a in calls))
                self.assertTrue(any(a[-1] == OTHER for a in calls))

    def test_raw_inventory_and_selected_source_tampering_are_rejected(self):
        for target in ('raw', 'source', 'structured', 'metadata', 'exit-code'):
            with self.subTest(target=target), tempfile.TemporaryDirectory() as td:
                _, _, _, _, out = self.execute(td)
                inventory = json.loads((out / 'inventory.json').read_text())
                plan = json.loads((out / 'plan.json').read_text())
                record = inventory['packages'][PACKAGE]
                if target == 'raw':
                    (out / record['listing']['stdout']).write_text(lines(listing(['TestDifferent'])))
                elif target == 'source':
                    Path(record['sources'][0]['path']).write_text('package changed\n')
                elif target == 'structured':
                    record['roots'].pop('Test000')
                elif target == 'metadata':
                    (out / record['metadata']['stdout']).write_text('{}')
                else:
                    record['listing']['exit_code'] = 1
                # Re-sealing the JSON cannot substitute for raw/selected-file evidence.
                inventory['inventory_id'] = self.checker.inventory_identity(inventory)
                plan['inventory_id'] = inventory['inventory_id']
                self.assertTrue(self.checker.validate_invocation_plan(plan, inventory,
                    {'id': 'linux-1', 'packages': [PACKAGE, OTHER]}, inventory['context'], out))

    def test_timeout_signal_and_unresolved_capture_quarantine_tail(self):
        for mode in ('timeout', 'killed', 'signal', 'capture-error', 'unknown-error', 'cancel'):
            def wrap(fake):
                def launch(argv, stdout, stderr):
                    result = fake(argv, stdout, stderr)
                    if '-run' in argv:
                        if mode == 'cancel': raise KeyboardInterrupt()
                        if mode == 'capture-error': raise OSError('cleanup unknown')
                        if mode == 'unknown-error': raise RuntimeError('unknown launcher failure')
                        if mode == 'signal': return -9
                        marker = 'panic: test timed out after 10m0s\n' if mode == 'timeout' else '*** Test killed: ran too long (11m0s).\n'
                        records = [json.loads(s) for s in stdout.read_text().splitlines()]
                        records.insert(-1, {'Action': 'output', 'Package': PACKAGE, 'Output': marker})
                        records[-1]['Action'] = 'fail'
                        stdout.write_text(lines(records))
                        return 1
                    return result
                return launch
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as td:
                with mock.patch.object(self.runner.SUMMARY, 'open_evidence', side_effect=AssertionError('quarantined read')):
                    status, manifest, summary, calls, out = self.execute(td, wrap)
                self.assertEqual(status, 1)
                self.assertEqual(len([a for a in calls if '-run' in a]), 1)
                self.assertEqual(manifest['cleanup_status'], 'externally-unresolved')
                self.assertEqual(manifest['evidence_qualification'], 'quarantined-mutable')
                self.assertEqual([i['state'] for i in manifest['invocations'][2:]], ['not_run'] * 16)
                receipt = json.loads((out / 'invocations' / manifest['invocations'][1]['id'] / 'command.json').read_text())
                self.assertEqual(receipt['capture_stability'], 'quarantined-mutable')
                self.assertNotIn('stdout_sha256', receipt)
                self.assertEqual(summary['status'], 'failure')
                self.assertEqual(summary['evidence_qualification'], 'quarantined-mutable')

    def test_inventory_timeout_stops_before_ordinary_execution(self):
        def wrap(fake):
            def launch(argv, stdout, stderr):
                result = fake(argv, stdout, stderr)
                if '-list' in argv:
                    stdout.write_text(lines([{'Action': 'output', 'Package': PACKAGE,
                                             'Output': 'panic: test timed out after 10m0s\n'}]))
                    return 1
                return result
            return launch
        with tempfile.TemporaryDirectory() as td:
            status, manifest, _, calls, _ = self.execute(td, wrap)
            self.assertEqual(status, 1)
            self.assertFalse(any(a[:2] == ['go', 'test'] and '-list' not in a for a in calls))
            self.assertEqual(manifest['cleanup_status'], 'externally-unresolved')

    def test_real_launch_exception_contract_uses_only_fake_processes(self):
        # Real subprocess.run exception handling, fake Popen only. No native child,
        # killpg or signal is permitted; killing a fake direct child is no join.
        import subprocess
        for exception in (KeyboardInterrupt(), subprocess.TimeoutExpired(['go'], 1), OSError('mock wait/capture failure')):
            calls = []
            class FakeProcess:
                args = ['go']; returncode = None
                def __enter__(self): return self
                def __exit__(self, *args): calls.append('context-exit')
                def communicate(self, *args, **kwargs): raise exception
                def kill(self): calls.append('kill-direct-child')
                def wait(self, *args, **kwargs): calls.append('wait-direct-child'); return -9
            def popen(*args, **kwargs):
                calls.append(('Popen', sorted(kwargs)))
                return FakeProcess()
            with self.subTest(exception=type(exception).__name__), tempfile.TemporaryDirectory() as td:
                with mock.patch.object(subprocess, 'Popen', popen), \
                     mock.patch.object(self.runner.os, 'kill', side_effect=AssertionError('real signal forbidden')), \
                     mock.patch.object(self.runner.os, 'killpg', side_effect=AssertionError('group signal forbidden')):
                    try:
                        receipt = self.runner.capture(['go', 'test'], Path(td), 'fake', self.runner.launch)
                    except (KeyboardInterrupt, subprocess.TimeoutExpired):
                        self.fail('capture did not return an explicitly quarantined receipt')
                self.assertEqual(receipt['capture_stability'], 'quarantined-mutable')
                self.assertEqual(receipt['descendant_cleanup'], 'externally-unresolved')
                self.assertNotIn('stdout_sha256', receipt)
                self.assertEqual(calls[0], ('Popen', ['stderr', 'stdout']))

    def test_malformed_policy_stops_before_any_launcher_call(self):
        with tempfile.TemporaryDirectory() as td, mock.patch.dict(self.runner.CHECK.POLICY, {'root_batching': {}}):
            status, _, _, calls, _ = self.execute(td)
            self.assertEqual(status, 1)
            self.assertEqual(calls, [])

    def test_full_inventory_cap_change_preserves_exact_eligible_union(self):
        # Same complete synthetic listing at both caps, never a sampled subset.
        for platform in ('linux', 'darwin'):
            for package in sorted(self.checker.BATCH_PACKAGES):
                for count in (1100, 391, 2337):
                    with self.subTest(platform=platform, package=package, roots=count), tempfile.TemporaryDirectory() as td:
                        context, shard, _, _, _ = self.fixture(td, platform)
                        shard['packages'] = [package, OTHER]
                        names = ['Test%04d' % n for n in range(count - 2)] + ['Example', 'FuzzSeeds', 'BenchmarkExcluded']
                        eligible = sorted(n for n in names if n != 'BenchmarkExcluded')
                        def fake(argv, stdout, stderr):
                            stderr.write_text('')
                            if argv[:3] == ['go', 'list', '-deps']:
                                stdout.write_text(json.dumps({'ImportPath': package, 'Dir': td, 'GoFiles': ['selected.go']}))
                            elif '-list' in argv:
                                stdout.write_text(lines(listing(names, package)))
                            else:
                                selected = argv[argv.index('-run') + 1][2:-2].split('|') if '-run' in argv else []
                                stdout.write_text(lines(events(selected, package if selected else OTHER)))
                            return 0
                        out = Path(td) / 'evidence'
                        self.assertEqual(self.runner.run_shard(shard, context, out, launcher=fake), 0)
                        inventory = json.loads((out / 'inventory.json').read_text())
                        plan = json.loads((out / 'plan.json').read_text())
                        self.assertEqual(self.checker.validate_invocation_plan(plan, inventory, shard, context, out), [])
                        batches = plan['invocations'][1:]
                        roots = [r for item in batches for r in item['roots']]
                        self.assertEqual(roots, eligible)
                        self.assertEqual(len(roots), len(set(roots)))
                        self.assertEqual(len(batches), (count + 3) // 4)
                        self.assertTrue(all(1 <= len(item['roots']) <= 4 for item in batches))
                        self.assertEqual(plan['invocations'][0], {'id': 'ordinary', 'packages': [OTHER], 'roots': [], 'selector': None})
                        for oversized_cap in (32, 5):
                            # Independent counterfactual partition: keep inventory,
                            # context, ordinary ownership and eligible union fixed.
                            oversized = copy.deepcopy(plan)
                            oversized['invocations'] = [copy.deepcopy(plan['invocations'][0])]
                            for index, offset in enumerate(range(0, count, oversized_cap), 1):
                                chunk = eligible[offset:offset + oversized_cap]
                                oversized['invocations'].append({
                                    'id': package.replace('/', '_') + '-%04d' % index,
                                    'packages': [package], 'roots': chunk,
                                    'selector': '^(' + '|'.join(chunk) + ')$'})
                            old_roots = [r for item in oversized['invocations'][1:] for r in item['roots']]
                            self.assertEqual(old_roots, roots)
                            self.assertEqual(len(old_roots), len(set(old_roots)))
                            self.assertGreater(len(batches), len(oversized['invocations']) - 1)
                            self.assertEqual(self.checker.validate_invocation_plan(oversized, inventory, shard, context, out),
                                             ['unknown package or invalid batch size'])

    def test_independent_plan_red_controls(self):
        with tempfile.TemporaryDirectory() as td:
            _, _, _, _, out = self.execute(td)
            inventory = json.loads((out / 'inventory.json').read_text())
            plan = json.loads((out / 'plan.json').read_text())
            shard = {'id': 'linux-1', 'packages': [PACKAGE, OTHER]}
            context = inventory['context']
            self.assertEqual(self.checker.validate_invocation_plan(plan, inventory, shard, context, out), [])
            mutations = []
            omitted = copy.deepcopy(plan); omitted['invocations'][-1]['roots'].pop(); mutations.append(omitted)
            duplicate = copy.deepcopy(plan); duplicate['invocations'].append(copy.deepcopy(duplicate['invocations'][-1])); mutations.append(duplicate)
            wrong_os = copy.deepcopy(plan); wrong_os['context']['platform'] = 'darwin'; mutations.append(wrong_os)
            wrong_rev = copy.deepcopy(plan); wrong_rev['context']['revision'] = 'b' * 40; mutations.append(wrong_rev)
            unknown = copy.deepcopy(plan); unknown['invocations'][-1]['roots'][0] = 'TestUnknown'; mutations.append(unknown)
            selector = copy.deepcopy(plan); selector['invocations'][-1]['selector'] = '.'; mutations.append(selector)
            for mutation in mutations:
                self.assertTrue(self.checker.validate_invocation_plan(mutation, inventory, shard, context, out))


if __name__ == '__main__':
    unittest.main()
