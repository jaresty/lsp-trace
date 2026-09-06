import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { pathToFileURL } from 'node:url';
import test from 'node:test';
import { spawnSync } from 'node:child_process';
import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';
import { createDefaultAnalyzer } from '../default-analyzer.mjs';
import { createProvider, encodeFrame, decodeFrame } from '../src/protocol.mjs';

const task = "import { task } from 'ember-concurrency'; const work = task(async () => 1); work.perform();\n";
const unsafe = '/** @type {any} */ const receiver = {}; receiver.reload();\n';
const cases = [
  ['any', unsafe, ['TRIGGERS_RELOAD'], 'BLOCKED', /unsafe compiler identity/],
  ['missing', "import Model from 'missing-model'; new Model().reload();\n", ['TRIGGERS_RELOAD'], 'BLOCKED', /TS2307/],
  ['empty', 'const receiver = { reload() {} }; receiver.reload();\n', ['TRIGGERS_RELOAD'], 'EMPTY'],
  ['task', task, ['INVOKES_TASK'], 'COMPLETE'],
  ['reload', "import Model from '@warp-drive/legacy/model'; new Model().reload();\n", ['TRIGGERS_RELOAD'], 'COMPLETE'],
  ['mixed-forward', task + unsafe, ['INVOKES_TASK', 'TRIGGERS_RELOAD'], 'PARTIAL', /unsafe compiler identity/],
  ['mixed-reverse', task + unsafe, ['TRIGGERS_RELOAD', 'INVOKES_TASK'], 'PARTIAL', /unsafe compiler identity/],
];

for (const [name, source, relations, outcome, reason] of cases) {
  test(`ASSERT_STATUS_PROPAGATION_${name}`, async (t) => {
    const root = await realpath(await mkdtemp(join(tmpdir(), 'provider-status-')));
    try {
      const provenance = JSON.parse(await readFile(new URL('../fixtures/source-constrained-synthetic/provenance.json', import.meta.url)));
      const files = {
        'jsconfig.json': JSON.stringify({ compilerOptions: { target: 'ES2022', checkJs: true, allowJs: true, noEmit: true }, include: ['seed.js'] }),
        'seed.js': source,
        'node_modules/ember-concurrency/package.json': JSON.stringify({ name: 'ember-concurrency', version: provenance.packages['ember-concurrency'].version, types: 'index.d.ts' }),
        'node_modules/ember-concurrency/index.d.ts': 'export interface AbstractTask { perform(): void; } export declare function task(fn: () => Promise<number>): AbstractTask;',
        'node_modules/@warp-drive/legacy/package.json': JSON.stringify({ name: '@warp-drive/legacy', version: provenance.packages['@warp-drive/legacy'].version, exports: { './model': { types: './model.d.ts' } } }),
        'node_modules/@warp-drive/legacy/model.d.ts': 'export default class Model { reload(): Promise<this>; }',
      };
      for (const [file, bytes] of Object.entries(files)) { await mkdir(dirname(join(root, file)), { recursive: true }); await writeFile(join(root, file), bytes); }
      const uri = pathToFileURL(join(root, 'seed.js')).href;
      const revision = 'ec56a17a19843c9d7a226fef06260758b89d2ccd';
      const request = { schema: 'lsp-trace.provider-request.v1', request_id: name, operation: 'analyze', documents: [{ uri, revision, language: 'glimmer-js', source, digest: `sha256:${createHash('sha256').update(source).digest('hex')}` }], relation_kinds: relations, limits: { max_observations: 100, timeout_ms: 30000 } };
      const lower = await analyzeSourceConstrainedTypeScript(request);
      const wrapper = createDefaultAnalyzer();
      const direct = await wrapper.analyze(request);
      if (relations.length > 1) {
        assert.deepEqual(await wrapper.analyze({ ...request, relation_kinds: [...relations].reverse() }), direct, 'relation order must not change observations or unresolved coverage');
      }
      if (process.env.STATUS_BASELINE) {
        const baseline = await import(pathToFileURL(process.env.STATUS_BASELINE).href);
        const before = await baseline.analyzeSourceConstrainedTypeScript(request);
        assert.deepEqual(lower.observations, before.observations, 'pre-fix positive endpoint and original-anchor identities');
      }
      const provider = createProvider({ analyzers: [wrapper] });
      const framed = decodeFrame(encodeFrame(await provider.handle(decodeFrame(encodeFrame(request)))));
      if (process.env.STATUS_MCP) {
        const bootstrap = join(root, 'bootstrap.json');
        await writeFile(bootstrap, JSON.stringify({ version: 1, processes: [{ alias: 'status', profile: { trust_domain: 'status-test', workspace: root, profile: 'fake-lsp', environment_reference: 'test' }, execution: { path: process.env.STATUS_LSP, directory: root } }], providers: [{ schema_version: 'lsp-trace.bootstrap-provider.v1', identity: 'ember-glint@1', version: '1', protocol: { name: 'lsp-trace.provider-observations', version: '1' }, execution: { path: process.env.STATUS_PROVIDER, directory: root }, executable_available: true, conformance_verified: true, capabilities: { relations: ['INVOKES_TASK', 'TRIGGERS_RELOAD'], languages: ['glimmer-js'], frameworks: ['ember'] }, limits: { request_bytes: 1048576, response_bytes: 1048576, protocol_messages: 1, stderr_bytes: 4096, wall_time_ms: 30000, termination_grace_ms: 1000 } }] }));
        for (const operation of ['incoming', 'slice']) await t.test(`actual installed MCP ${operation}`, () => {
          const call = (id, name, args) => ({ jsonrpc: '2.0', id, method: 'tools/call', params: { name, arguments: args } });
          const args = { session_id: 'status', generation: 1, uri, line: 0, character: 0, relations, providers: ['ember-glint@1'], languages: ['glimmer-js'], frameworks: ['ember'], workspace_revision: { kind: 'git', commit: revision, custody: 'CALLER_ASSERTED' }, max_nodes: 100, timeout_ms: 30000, request_timeout_ms: 30000, ...(operation === 'slice' ? { start_mode: 'at' } : {}) };
          const execution = spawnSync(process.env.STATUS_MCP, ['--bootstrap-config', bootstrap], { input: [call(1, 'lsp_session_v1_list', {}), call(2, `lsp_trace_v1_${operation}`, args)].map(JSON.stringify).join('\n') + '\n', encoding: 'utf8' });
          assert.equal(execution.status, 0, execution.stderr);
          const response = JSON.parse(execution.stdout.trim().split('\n').at(-1)).result;
          const env = response.structuredContent;
          assert.deepEqual(JSON.parse(response.content[0].text), env, 'text/structured parity');
          if (outcome === 'BLOCKED' || outcome === 'EMPTY') {
            assert.equal(env.operation_status, 'FAILED');
            assert.match(JSON.stringify(env), reason ?? /no accepted observations/);
          } else {
            assert.equal(env.operation_status, 'SUCCEEDED', JSON.stringify(env));
            const graph = JSON.parse(env.content);
            assert.equal(graph.relations.length, 1);
            assert.equal(graph.provenance.coverage.Status, outcome === 'PARTIAL' ? 'PARTIAL' : 'COMPLETE_WITHIN_BOUNDS');
            if (reason) assert.match(JSON.stringify(graph.provenance.diagnostics), reason);
            assert.equal(graph.relations[0].from, lower.observations[0].from.node_id);
            assert.equal(graph.relations[0].to, lower.observations[0].to.node_id);
            assert.equal(graph.relations[0].anchors[0].blob, lower.observations[0].original_anchor.blob);
          }
        });
      }
      for (const [layer, result] of [['lower', lower], ['wrapper', direct], ['framed', framed]]) await t.test(layer, () => {
        assert.equal(result.outcome, outcome, `${layer}: preserve outcome`);
        assert.equal(result.observations.length, outcome === 'EMPTY' || outcome === 'BLOCKED' ? 0 : 1, `${layer}: safe observations`);
        if (reason) assert.match(result.coverage.reason ?? result.blocker ?? '', reason, `${layer}: diagnostic`);
        if (outcome === 'PARTIAL' || outcome === 'BLOCKED') assert.deepEqual(result.coverage.covered, [], `${layer}: unresolved is not covered`);
        assert.deepEqual(result.observations, lower.observations, `${layer}: endpoints and anchors unchanged`);
      });
      const strict = await provider.handle({ schema_version: 'lsp-trace.provider-collector-request.v1', provider_id: 'ember-glint@1', adapter_id: 'ember-glint@1', session: { session_id: 'status', generation: 1 }, seed: { uri }, relations, document_custody: { workspace_revision: { kind: 'git', value: revision } }, limits: { max_nodes: 100, timeout_ms: 30000 } });
      assert.equal(strict.coverage.status, outcome === 'PARTIAL' ? 'PARTIAL' : outcome === 'BLOCKED' ? 'UNKNOWN' : 'COMPLETE_WITHIN_BOUNDS');
      if (reason) assert.match(JSON.stringify(strict.diagnostics), reason);
      assert.deepEqual(strict.observations.map(o => o.original_anchor), lower.observations.map(o => o.original_anchor));
    } finally { await rm(root, { recursive: true, force: true }); }
  });
}
