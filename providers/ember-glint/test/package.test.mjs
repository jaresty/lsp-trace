import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const allowed = [
  'README.md', 'analyzer.mjs', 'analyzers/custody.cjs', 'analyzers/glint.mjs',
  'analyzers/passes-callback.mjs', 'analyzers/script.mjs',
  'analyzers/source-constrained-typescript.mjs', 'analyzers/template-relations.mjs', 'analyzers/template.mjs',
  'bin/ember-glint.mjs', 'default-analyzer.mjs',
  'fixtures/source-constrained-synthetic/provenance.json',
  'fixtures/source-constrained-synthetic/tsconfig.json',
  'fixtures/source-constrained-synthetic/vendor/ember-concurrency/index.d.ts',
  'fixtures/source-constrained-synthetic/vendor/glimmer-component/index.d.ts',
  'fixtures/source-constrained-synthetic/vendor/warp-drive/model.d.ts',
  'fixtures/source-constrained-synthetic/vendor/warp-drive/private-model.d.ts',
  'package.json', 'renders-from-analyzer.mjs', 'src/protocol.mjs',
];

function runtimeFiles(directory, prefix = '') {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const relative = path.posix.join(prefix, entry.name);
    return entry.isDirectory() ? runtimeFiles(path.join(directory, entry.name), relative) : [relative];
  });
}

test('ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS', () => {
  const pack = JSON.parse(execFileSync('npm', ['pack', '--dry-run', '--json'], { cwd: root, encoding: 'utf8' }))[0];
  assert.deepEqual(pack.files.map(({ path: name }) => name).sort(), allowed, 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
  assert.equal(pack.files.find(({ path: name }) => name === 'bin/ember-glint.mjs')?.mode, 0o755, 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');

  const temporary = mkdtempSync(path.join(tmpdir(), 'ember-glint-install-'));
  try {
    const [{ filename }] = JSON.parse(execFileSync('npm', ['pack', '--json', '--pack-destination', temporary], { cwd: root, encoding: 'utf8' }));
    execFileSync('npm', ['install', '--ignore-scripts', '--offline', path.join(temporary, filename)], { cwd: temporary, stdio: 'pipe' });
    const installed = path.join(temporary, 'node_modules', '@lsp-trace', 'ember-glint-provider');
    assert.deepEqual(runtimeFiles(installed).filter((name) => name !== 'package.json').sort(), allowed.filter((name) => name !== 'package.json'), 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
    const manifest = JSON.parse(readFileSync(path.join(installed, 'package.json'), 'utf8'));
    assert.equal(manifest.bin['ember-glint'], './bin/ember-glint.mjs', 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
    assert.equal(manifest.exports['.'], './default-analyzer.mjs', 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');

    const workspace = path.join(temporary, 'workspace');
    mkdirSync(workspace);
    const seed = path.join(workspace, 'invokes-task-positive.gts');
    writeFileSync(seed, readFileSync(path.join(root, 'fixtures/invokes-task-positive.gts')));
    const request = {
      schema_version: 'lsp-trace.provider-collector-request.v1',
      provider_id: 'ember-glint@1',
      adapter_id: 'lsp-trace-observation-adapter@1',
      session: { session_id: 'packed-invokes-task', generation: 1 },
      seed: { uri: pathToFileURL(seed).href },
      relations: ['INVOKES_TASK'],
      languages: ['typescript'],
      frameworks: ['source-constrained-synthetic'],
      document_custody: { workspace_revision: { kind: 'git', value: '25319f86536014f5afb01a7de0e404cf8b492e51' } },
      limits: { max_nodes: 10, request_timeout_ms: 1000 },
    };
    const body = Buffer.from(JSON.stringify(request));
    const run = spawnSync(path.join(temporary, 'node_modules', '.bin', 'ember-glint'), [], {
      cwd: workspace,
      input: Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`, 'ascii'), body]),
    });
    assert.equal(run.status, 0, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK: stderr=${run.stderr}`);
    assert.equal(run.stderr.length, 0, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    const separator = run.stdout.indexOf('\r\n\r\n');
    assert.ok(separator > 0, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    const header = run.stdout.subarray(0, separator).toString('ascii');
    assert.match(header, /^Content-Length: [0-9]+$/, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    const length = Number(header.slice('Content-Length: '.length));
    const responseBody = run.stdout.subarray(separator + 4);
    assert.equal(responseBody.length, length, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    const response = JSON.parse(responseBody);
    assert.equal(response.coverage.status, 'COMPLETE_WITHIN_BOUNDS', 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    assert.equal(response.failure, undefined, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    assert.equal(response.observations.length, 1, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    assert.equal(response.observations[0].kind, 'INVOKES_TASK', 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');
    assert.match(response.observations[0].to.node_id, /package=ember-concurrency@5\.2\.0;path=vendor\/ember-concurrency\/index\.d\.ts;symbol=AbstractTask\.perform;/, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_INVOKES_TASK');

    for (const [fixture, expectedCount] of [['triggers-reload-positive.ts', 1], ['triggers-reload-negative.ts', 0]]) {
      const reloadSeed = path.join(workspace, fixture);
      writeFileSync(reloadSeed, readFileSync(path.join(root, 'fixtures', fixture)));
      const reloadRequest = { ...request, seed: { uri: pathToFileURL(reloadSeed).href }, relations: ['TRIGGERS_RELOAD'] };
      const reloadBody = Buffer.from(JSON.stringify(reloadRequest));
      const reloadRun = spawnSync(path.join(temporary, 'node_modules', '.bin', 'ember-glint'), [], {
        cwd: workspace,
        input: Buffer.concat([Buffer.from(`Content-Length: ${reloadBody.length}\r\n\r\n`, 'ascii'), reloadBody]),
      });
      assert.equal(reloadRun.status, 0, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}: stderr=${reloadRun.stderr}`);
      assert.equal(reloadRun.stderr.length, 0, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      const reloadSeparator = reloadRun.stdout.indexOf('\r\n\r\n');
      assert.ok(reloadSeparator > 0, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      const reloadHeader = reloadRun.stdout.subarray(0, reloadSeparator).toString('ascii');
      assert.match(reloadHeader, /^Content-Length: [0-9]+$/, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      const reloadResponseBody = reloadRun.stdout.subarray(reloadSeparator + 4);
      assert.equal(reloadResponseBody.length, Number(reloadHeader.slice('Content-Length: '.length)), `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      const reloadResponse = JSON.parse(reloadResponseBody);
      assert.equal(reloadResponse.coverage.status, 'COMPLETE_WITHIN_BOUNDS', `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      assert.equal(reloadResponse.observations.length, expectedCount, `ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD: ${fixture}`);
      if (expectedCount) assert.match(reloadResponse.observations[0].to.node_id, /path=vendor\/warp-drive\/private-model\.d\.ts;symbol=Model\.reload;/, 'ASSERT_PACKED_OFFLINE_STRICT_COLLECTOR_TRIGGERS_RELOAD');
    }
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
});

test('ASSERT_PROVIDER_RUNTIME_HAS_NO_DOWNLOAD_DISCOVERY_OR_CORE_IMPORTS', () => {
  const forbidden = [
    /(?:from\s+|import\s*)['"]node:(?:http|https|net|tls|child_process)['"]/,
    /\b(?:fetch|curl|wget|which|whereis)\s*\(/,
    /(?:lsp-trace\/internal|\.\.\/\.\.\/\.\.\/(?:internal|cmd)\/)/,
    /process\.env\.PATH/,
  ];
  for (const relative of allowed.filter((name) => /\.(?:mjs|cjs)$/.test(name))) {
    const source = readFileSync(path.join(root, relative), 'utf8');
    for (const pattern of forbidden) assert.doesNotMatch(source, pattern, `ASSERT_PROVIDER_RUNTIME_HAS_NO_DOWNLOAD_DISCOVERY_OR_CORE_IMPORTS: ${relative}`);
  }
});
