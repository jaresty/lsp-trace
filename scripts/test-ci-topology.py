#!/usr/bin/env python3
"""Original topology guards, migrated from pipeline statuses to checked-plan exit."""
import subprocess
import copy
import runpy
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECK = ROOT / 'scripts/check-ci-topology.py'
WORKFLOW = ROOT / '.github/workflows/ci.yml'
COMMAND = '          exec python3 scripts/run-ci-shard.py --platform "$PLATFORM" --shard "${{ matrix.shard.id }}" --revision "$GITHUB_SHA" --output "$telemetry_dir"\n'
TELEMETRY = '          telemetry_dir=$(mktemp -d "$RUNNER_TEMP/go-test-${{ matrix.shard.id }}-XXXXXX")\n'


class TopologyTest(unittest.TestCase):
    def check(self, text, diagnostics=None):
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / 'ci.yml'
            path.write_text(text)
            result = subprocess.run(['python3', str(CHECK), '--workflow', str(path)], cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if diagnostics is None:
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertEqual(result.stdout, 'PASS CI workflow topology\n')
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout)
            self.assertEqual(result.stdout, ''.join('FAIL ' + d + '\n' for d in diagnostics))

    def mutate_shard_run(self, old, new, text=None):
        text = WORKFLOW.read_text() if text is None else text
        self.assertEqual(text.count(old), 1)
        return text.replace(old, new)

    def both(self, diagnostic):
        return [p + ': ' + diagnostic for p in ('test-linux', 'test-darwin')]

    def test_repository_workflow(self):
        self.check(WORKFLOW.read_text())

    def test_elixir_runner_must_support_pinned_otp(self):
        for runner in ('ubuntu-latest', 'ubuntu-26.04'):
            with self.subTest(runner=runner):
                self.check(WORKFLOW.read_text().replace('runs-on: ubuntu-22.04', 'runs-on: ' + runner),
                           ['elixir-companion: pinned OTP 25.3.2.7 requires ubuntu-22.04'])

    def test_sequential_pipestatus_capture_is_rejected(self):
        # No pipeline remains: a shell wrapper that loses the runner status is refused.
        text = self.mutate_shard_run(COMMAND, COMMAND.replace('exec ', '') + '          test_status=$?\n          true\n')
        self.check(text, self.both('checked runner arguments and unmasked exit status required'))

    def test_repository_relative_telemetry_is_rejected(self):
        text = self.mutate_shard_run(COMMAND, COMMAND.replace('"$telemetry_dir"', '"."'))
        self.check(text, self.both('test telemetry must use unique absolute paths under RUNNER_TEMP'))

    def test_comment_only_telemetry_fragments_are_rejected(self):
        text = self.mutate_shard_run(TELEMETRY, '          # ' + TELEMETRY.strip() + '\n          telemetry_dir=.\n')
        self.check(text, self.both('test telemetry must use unique absolute paths under RUNNER_TEMP'))

    def test_repository_relative_upload_paths_are_rejected(self):
        text = self.mutate_shard_run('            ${{ runner.temp }}/go-test-*/**\n', '            go-test-*/**\n')
        self.check(text, self.both('uploaded telemetry must use absolute runner.temp paths'))

    def test_in_block_comment_cannot_satisfy_test_invocation_flags(self):
        text = self.mutate_shard_run(COMMAND, '          # ' + COMMAND.strip() + '\n' + COMMAND.replace('--revision "$GITHUB_SHA" ', ''))
        self.check(text, self.both('checked runner arguments and unmasked exit status required'))

    def test_in_block_comments_cannot_satisfy_status_fragments(self):
        # exec and the exact argv now own status propagation and plan admission.
        for fragment in ('exec ', '--platform "$PLATFORM" ', '--shard "${{ matrix.shard.id }}" ', '--revision "$GITHUB_SHA" '):
            with self.subTest(fragment=fragment):
                text = self.mutate_shard_run(COMMAND, '          # ' + fragment + '\n' + COMMAND.replace(fragment, ''))
                self.check(text, self.both('checked runner arguments and unmasked exit status required'))

    def test_reordered_status_exits_are_rejected(self):
        for suffix in (' || true', '; exit 0', ' | tee log'):
            with self.subTest(suffix=suffix):
                text = self.mutate_shard_run(COMMAND, COMMAND.rstrip() + suffix + '\n')
                self.check(text, self.both('checked runner arguments and unmasked exit status required'))

    def test_duplicate_and_dead_go_test_commands_are_rejected(self):
        for extra in (COMMAND, '          false && go test -json -timeout=10m ./...\n'):
            with self.subTest(extra=extra):
                self.check(self.mutate_shard_run(COMMAND, extra + COMMAND),
                           self.both('test invocation must be exactly one checked exhaustive plan'))

    def test_unrelated_job_cannot_satisfy_upload_condition(self):
        text = WORKFLOW.read_text().replace('        if: always()\n        uses: actions/upload-artifact@v4', '        uses: actions/upload-artifact@v4')
        text += '\n  unrelated-proof:\n    if: always()\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo harmless\n'
        self.check(text, self.both('upload step must use if: always()'))

    def test_artifact_names_preserved(self):
        text = self.mutate_shard_run('          name: go-test-${{ matrix.shard.id }}\n', '          name: different\n')
        self.check(text, self.both('existing shard artifact names must be preserved'))

    def test_job_timeout_not_inflated(self):
        text = WORKFLOW.read_text().replace('  test-linux:\n', '  test-linux:\n    timeout-minutes: 600\n')
        self.check(text, ['test-linux: job timeout policy must remain unchanged'])


class ExecutionEnvelopeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.checker = runpy.run_path(str(CHECK))
        cls.workflow = cls.checker['load_workflow'](WORKFLOW)

    def changed(self, mutate):
        data = copy.deepcopy(self.workflow)
        mutate(data['jobs']['test-linux'])
        return self.checker['validate'](data)

    def setup_action_case(self, platform, index):
        data = copy.deepcopy(self.workflow)
        job = data['jobs']['test-' + platform]
        # Isolate the selected platform despite the workflow's shared YAML steps.
        job['steps'] = copy.deepcopy(job['steps'])
        return data, job['steps'][index]

    def test_setup_actions_named_and_unnamed_are_accepted(self):
        for platform in ('linux', 'darwin'):
            for index, action in enumerate(('actions/checkout@v4', 'actions/setup-go@v5')):
                for display in (None, action, 'Friendly setup display name'):
                    with self.subTest(platform=platform, action=action, display=display):
                        data, step = self.setup_action_case(platform, index)
                        self.assertEqual(step['uses'], action)
                        if display is None:
                            step.pop('name', None)
                        else:
                            step['name'] = display
                        self.assertEqual(self.checker['validate'](data), [])

    def test_setup_action_display_names_cannot_hide_wrong_or_missing_uses(self):
        for platform in ('linux', 'darwin'):
            for index, action in enumerate(('actions/checkout@v4', 'actions/setup-go@v5')):
                for replacement in ('actions/checkout@v3', 'actions/setup-go@v4', 'unrelated/action@v1', None, ''):
                    with self.subTest(platform=platform, action=action, replacement=replacement):
                        data, step = self.setup_action_case(platform, index)
                        step['name'] = action
                        if replacement is None:
                            step.pop('uses')
                        else:
                            step['uses'] = replacement
                        self.assertTrue(self.checker['validate'](data))

    def test_setup_action_display_names_cannot_hide_run_steps(self):
        for platform in ('linux', 'darwin'):
            for index, action in enumerate(('actions/checkout@v4', 'actions/setup-go@v5')):
                for keep_uses in (False, True):
                    with self.subTest(platform=platform, action=action, keep_uses=keep_uses):
                        data, step = self.setup_action_case(platform, index)
                        step['name'] = action
                        if not keep_uses:
                            step.pop('uses')
                        step['run'] = 'echo harmless'
                        self.assertTrue(self.checker['validate'](data))

    def test_job_and_runner_disable_or_mask_rejected(self):
        for owner in ('job', 'step'):
            for key, value in [('if', False), ('if', '${{ false }}'), ('if', 'always()'),
                               ('if', '${{ success() && false }}'), ('continue-on-error', True),
                               ('continue-on-error', '${{ true }}'), ('continue-on-error', 'false'),
                               ('defaults', {'run': {'shell': 'bash {0}'}})]:
                with self.subTest(owner=owner, key=key, value=value):
                    def mutate(job):
                        target = job if owner == 'job' else self.checker['step_by_name'](job, 'Run shard once and summarize')
                        target[key] = value
                    self.assertTrue(self.changed(mutate))

    def test_supported_success_conditions_preserve_previous_failure_skip(self):
        for value in (True, 'true', '${{ true }}', 'success()', '${{ success() }}'):
            with self.subTest(value=value):
                def mutate(job):
                    job['if'] = value
                    self.checker['step_by_name'](job, 'Run shard once and summarize')['if'] = value
                    job['continue-on-error'] = False
                self.assertEqual(self.changed(mutate), [])

    def test_matrix_checkout_and_environment_bypasses_rejected(self):
        mutations = [lambda j: j['strategy']['matrix'].update(exclude=[{'shard': {'id': 'linux-1'}}]),
                     lambda j: j['strategy'].update({'fail-fast': True}),
                     lambda j: j['env'].update(PLATFORM='darwin'),
                     lambda j: j['env'].update(ACTIONS_RUNNER_PROCESS_CLEANUP='false'),
                     lambda j: j['steps'][0].update({'if': False}),
                     lambda j: j['steps'][0].update({'with': {'ref': 'unrelated'}}),
                     lambda j: j['steps'][1].update({'continue-on-error': True}),
                     lambda j: j['steps'].append(copy.deepcopy(j['steps'][4])),
                     lambda j: j.update(environment='manual-approval'),
                     lambda j: j['strategy']['matrix']['shard'][0].update(needs_gopls='false'),
                     lambda j: self.checker['step_by_name'](j, 'Upload shard timing').update(run='go test ./...'),
                     lambda j: self.checker['step_by_name'](j, 'Run shard once and summarize').update(shell='bash {0}'),
                     lambda j: self.checker['step_by_name'](j, 'Run shard once and summarize').update({'working-directory': '../outside'})]
        for index, mutation in enumerate(mutations):
            with self.subTest(case=index):
                self.assertTrue(self.changed(mutation))
        for key, value in [('env', {'GOFLAGS': '-run=NoTests'}), ('defaults', {'run': {'shell': 'sh'}})]:
            data = copy.deepcopy(self.workflow); data[key] = value
            self.assertTrue(self.checker['validate'](data))

    def test_each_postcheck_delete_selector_deadline_condition_env_rejected(self):
        names = ['Check controlled inventory source identity', 'Check Go-only normal-folder caller',
                 'Check durable inventory and legacy Describe across process exit']
        for name in names:
            for mode in ('delete', 'selector', 'deadline', 'condition', 'status', 'env', 'tag', 'count', 'always'):
                with self.subTest(name=name, mode=mode):
                    def mutate(job):
                        step = self.checker['step_by_name'](job, name)
                        if mode == 'delete': job['steps'].remove(step)
                        elif mode == 'selector': step['run'] = step['run'].replace('^Test', '^Different')
                        elif mode == 'deadline': step['run'] = step['run'].replace('-timeout=2m', '-timeout=20m')
                        elif mode == 'condition': step['if'] = '${{ false }}'
                        elif mode == 'always': step['if'] = 'always() && (' + step['if'] + ')'
                        elif mode == 'status': step['continue-on-error'] = True
                        elif mode == 'env': step['env'] = {'GOPROXY': 'on'}
                        elif mode == 'tag': step['run'] = step['run'].replace('-tags lsp_trace_controlled_describe_fixture ', '')
                        elif mode == 'count': step['run'] = step['run'].replace('-count=1', '-count=0')
                    self.assertTrue(self.changed(mutate))


if __name__ == '__main__':
    unittest.main()
