#!/usr/bin/env python3
import json
import hashlib
import runpy
from unittest import mock
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "summarize-go-test-json.py"

class SummaryTest(unittest.TestCase):
    def summarize(self, lines, exit_code=0, raw_text=None):
        with tempfile.TemporaryDirectory() as td:
            raw = Path(td) / "raw.json"
            out = Path(td) / "summary.json"
            raw.write_text(raw_text if raw_text is not None else "\n".join(json.dumps(x) for x in lines) + "\n")
            subprocess.run(["python3", str(SCRIPT), "--input", str(raw), "--output", str(out), "--os", "linux", "--commit", "abc", "--job", "test-linux", "--shard", "linux-1", "--exit-code", str(exit_code)], check=True)
            return json.loads(out.read_text())

    def test_success_elapsed_and_cached(self):
        result = self.summarize([
            {"Time":"2026-01-01T00:00:00Z","Action":"start","Package":"p/a"},
            {"Time":"2026-01-01T00:00:01Z","Action":"pass","Package":"p/a","Elapsed":1.25,"Output":"ok  p/a (cached)\n"},
        ])
        self.assertEqual(result["status"], "success")
        self.assertEqual(result["packages"], [{"package":"p/a","status":"pass","elapsed":1.25,"cached":True}])
        self.assertEqual(result["os"], "linux")

    def test_failure_and_timeout(self):
        failure = self.summarize([{"Action":"fail","Package":"p/a","Elapsed":2.0}], 1)
        self.assertEqual(failure["status"], "failure")
        timeout = self.summarize([{"Action":"output","Package":"p/a","Output":"panic: test timed out after 10m0s\n"},{"Action":"fail","Package":"p/a","Elapsed":600.0}], 1)
        self.assertEqual(timeout["status"], "timeout")
        self.assertEqual(timeout["packages"][0]["status"], "timeout")

    def test_legacy_concatenation_and_wrong_exit_code_cannot_mask_failure(self):
        records = [{'Action': 'fail', 'Package': 'p/a'}, {'Action': 'pass', 'Package': 'p/a'}]
        result = self.summarize(records, exit_code=0)
        self.assertEqual(result['status'], 'failure')
        self.assertEqual(result['packages'][0]['status'], 'fail')
        self.assertEqual(result['duplicate_package_terminals'], ['p/a'])
        self.assertEqual(self.summarize(records[:1], exit_code=0)['status'], 'failure')

    def test_passing_timeout_named_subtest_is_not_a_timeout(self):
        result = self.summarize([
            {"Action":"run","Package":"p/a","Test":"TestExactTuplePrerequisitesAndNoPanic/timeout-unknown"},
            {"Action":"output","Package":"p/a","Test":"TestExactTuplePrerequisitesAndNoPanic/timeout-unknown","Output":"=== RUN   TestExactTuplePrerequisitesAndNoPanic/timeout-unknown\n"},
            {"Action":"pass","Package":"p/a","Test":"TestExactTuplePrerequisitesAndNoPanic/timeout-unknown","Elapsed":0.01},
            {"Action":"pass","Package":"p/a","Elapsed":0.02},
        ])
        self.assertEqual(result["status"], "success")
        self.assertEqual(result["packages"], [{"package":"p/a","status":"pass","elapsed":0.02,"cached":False}])

    def test_malformed_or_truncated_stream_emits_bounded_partial_summary(self):
        raw = '{"Action":"pass","Package":"p/a"}\n{"Action":'
        result = self.summarize([], exit_code=1, raw_text=raw)
        self.assertEqual(result["status"], "failure")
        self.assertEqual(result["malformed_lines"], 1)
        self.assertTrue(result["partial"])
        self.assertEqual(result["packages"], [{"package":"p/a","status":"pass","elapsed":None,"cached":False}])

class InvocationSummaryTest(unittest.TestCase):
    def setUp(self):
        self.module = runpy.run_path(str(SCRIPT))
        self.definition = {'id': 'batch-1', 'packages': ['p/a'],
                           'roots': ['TestA', 'TestHelper', 'Example', 'FuzzSeeds'],
                           'selector': '^(TestA|TestHelper|Example|FuzzSeeds)$'}

    def stream(self):
        result = [{'Action': 'start', 'Package': 'p/a'}]
        for root in self.definition['roots']:
            result.append({'Action': 'run', 'Package': 'p/a', 'Test': root})
            if root == 'FuzzSeeds':
                result.extend([{'Action': 'run', 'Package': 'p/a', 'Test': root + '/seed#0'},
                               {'Action': 'pass', 'Package': 'p/a', 'Test': root + '/seed#0'}])
            result.append({'Action': 'skip' if root == 'TestHelper' else 'pass', 'Package': 'p/a', 'Test': root})
        result.append({'Action': 'pass', 'Package': 'p/a'})
        return result

    def reconcile(self, records, **changes):
        with tempfile.TemporaryDirectory() as td:
            base = Path(td)
            (base / 'raw').write_text(''.join(json.dumps(e) + '\n' for e in records))
            invocation = dict(self.definition, stdout='raw', state='finished', exit_code=0)
            invocation.update(changes)
            return self.module['reconcile_invocation'](invocation, base)

    def test_go_examples_fuzz_seeds_and_explicit_skip(self):
        result = self.reconcile(self.stream())
        self.assertEqual(result['errors'], [])
        self.assertEqual(result['root_count'], 4)
        self.assertEqual(result['skips'][0]['test'], 'TestHelper')
        self.assertTrue(result['skips'][0]['explanation'])

    def test_pure_red_root_accounting_controls(self):
        records = self.stream()
        cases = {
            'omit-root': [e for e in records if e.get('Test') != 'TestA'],
            'duplicate-run': records[:2] + [records[1]] + records[2:],
            'duplicate-terminal': records[:3] + [records[2]] + records[3:],
            'extra-root': [dict(e, Test='TestExtra') if e.get('Test') == 'TestA' else e for e in records],
            'unfinished-root': [e for e in records if not (e.get('Test') == 'TestA' and e['Action'] == 'pass')],
            'unfinished-child': [e for e in records if not (e.get('Test') == 'FuzzSeeds/seed#0' and e['Action'] == 'pass')],
            'missing-package-terminal': records[:-1],
            'duplicate-package-terminal': records + [records[-1]],
            'terminal-before-run': records[:1] + [records[2], records[1]] + records[3:],
        }
        for name, mutated in cases.items():
            with self.subTest(control=name):
                self.assertTrue(self.reconcile(mutated)['errors'])
        self.assertTrue(self.reconcile(records, state='running')['errors'])
        self.assertTrue(self.reconcile(records, exit_code=1)['errors'])

    def test_unsafe_manifest_paths_rejected_before_any_stream_open(self):
        for state in ('finished', 'pending', 'cancelled'):
            for stream in ('stdout', 'stderr'):
                for unsafe in ('../outside', '/outside', 'nested/../../outside'):
                    item = dict(self.definition, stdout='raw', stderr='err', state=state, exit_code=0)
                    item[stream] = unsafe
                    with self.subTest(state=state, stream=stream, unsafe=unsafe), mock.patch.object(Path, 'open', side_effect=OSError('spy blocked read')) as opened, mock.patch.object(self.module['os'], 'open', side_effect=OSError('spy blocked descriptor')) as descriptor:
                        summary = self.module['summarize_manifest']({'state': 'finished', 'invocations': [item]},
                            {'invocations': [self.definition]}, Path('/safe'))
                        self.assertEqual(summary['status'], 'failure')
                        opened.assert_not_called()
                        descriptor.assert_not_called()

    def test_symlink_escape_rejected_before_read(self):
        with tempfile.TemporaryDirectory() as td:
            base = Path(td) / 'artifact'; base.mkdir()
            outside = Path(td) / 'outside'; outside.mkdir()
            (outside / 'raw').write_text('must never be read')
            (base / 'link').symlink_to(outside, target_is_directory=True)
            for target in ('link/raw', 'link'):
                item = dict(self.definition, stdout=target, stderr='err', state='pending', exit_code=None)
                with self.subTest(target=target), mock.patch.object(Path, 'open', side_effect=OSError('spy blocked read')) as opened, mock.patch.object(self.module['os'], 'open', side_effect=OSError('spy blocked descriptor')) as descriptor:
                    summary = self.module['summarize_manifest']({'state': 'finished', 'invocations': [item]},
                        {'invocations': [self.definition]}, base)
                    self.assertEqual(summary['status'], 'failure')
                    opened.assert_not_called()
                    descriptor.assert_not_called()

    def test_symlink_swap_between_admission_and_open_cannot_read_outside(self):
        with tempfile.TemporaryDirectory() as td:
            base = Path(td) / 'artifact'; base.mkdir()
            outside = Path(td) / 'outside'; outside.write_text('must never be read')
            (base / 'raw').write_text('initial regular file')
            original_open = self.module['os'].open
            def swapped(path, flags, **kwargs):
                if path == 'raw':
                    (base / 'raw').unlink()
                    (base / 'raw').symlink_to(outside)
                return original_open(path, flags, **kwargs)
            with mock.patch.object(self.module['os'], 'open', swapped), mock.patch.object(self.module['os'], 'fdopen', side_effect=AssertionError('outside read')) as reader:
                result = self.module['reconcile_invocation'](dict(self.definition, stdout='raw', state='finished', exit_code=0), base)
                self.assertTrue(result['errors'])
                reader.assert_not_called()

    def test_child_output_is_not_an_event(self):
        records = self.stream()
        records.insert(2, {'Action': 'output', 'Package': 'p/a', 'Test': 'TestA',
                           'Output': '{"Action":"pass","Package":"child","Test":"TestExtra"}\n'})
        self.assertFalse(self.reconcile(records)['errors'])

    def test_first_failure_and_unfinished_tail_cannot_be_masked(self):
        with tempfile.TemporaryDirectory() as td:
            base = Path(td)
            definitions = [dict(self.definition, id='batch-1'), dict(self.definition, id='batch-2')]
            invocations = []
            for index, definition in enumerate(definitions):
                records = self.stream()
                records[-1]['Action'] = 'fail' if index == 0 else 'pass'
                name = str(index) + '.jsonl'
                (base / name).write_text(''.join(json.dumps(e) + '\n' for e in records))
                (base / (name + '.stderr')).write_text('')
                invocations.append(dict(definition, stdout=name, stderr=name + '.stderr', state='finished',
                                        stdout_sha256=hashlib.sha256((base / name).read_bytes()).hexdigest(),
                                        stderr_sha256=hashlib.sha256(b'').hexdigest(), exit_code=1 if index == 0 else 0))
            manifest = {'state': 'finished', 'invocations': invocations, 'blocked_packages': []}
            plan = {'invocations': definitions, 'blocked_packages': []}
            summary = self.module['summarize_manifest'](manifest, plan, base)
            self.assertEqual(summary['status'], 'failure')
            self.assertEqual(summary['packages'], [{'package': 'p/a', 'status': 'fail'}])
            self.assertEqual(summary['logical_package_count'], 1)
            self.assertEqual(summary['invocation_count'], 2)
            # Positive control: same valid receipts and complete work, no early failure.
            (base / '0.jsonl').write_text(''.join(json.dumps(e) + '\n' for e in self.stream()))
            manifest['invocations'][0]['stdout_sha256'] = hashlib.sha256((base / '0.jsonl').read_bytes()).hexdigest()
            manifest['invocations'][0]['exit_code'] = 0
            self.assertEqual(self.module['summarize_manifest'](manifest, plan, base)['status'], 'success')
            del manifest['invocations'][-1]
            self.assertEqual(self.module['summarize_manifest'](manifest, plan, base)['status'], 'failure')


if __name__ == "__main__":
    unittest.main()
