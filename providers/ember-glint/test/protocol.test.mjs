import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  PROVIDER_IDENTITY,
  LIFECYCLE_OUTCOMES,
  canonicalBytes,
  logicalDigest,
  encodeFrame,
  decodeFrame,
  createProvider,
} from '../src/protocol.mjs';

const request = Object.freeze({
  schema: 'lsp-trace.provider-request.v1',
  request_id: 'request-1',
  operation: 'analyze',
  relation_kinds: ['BINDS_ARGUMENT'],
  documents: [{ uri: 'file:///app.gts', language: 'glimmer-js', revision: 'r1', digest: `sha256:${'a'.repeat(64)}`, source: '<Widget @value={{this.x}} />' }],
  limits: { max_observations: 2, timeout_ms: 1000 },
});

function named(name, fn) {
  test(name, async () => {
    try { await fn(); } catch (error) { error.message = `${name}: ${error.message}`; throw error; }
  });
}

named('ASSERT_STRICT_CONTENT_LENGTH_FRAMING', () => {
  const frame = encodeFrame(request);
  assert.deepEqual(decodeFrame(frame), request);
  for (const invalid of [
    Buffer.from(`content-length: 2\r\n\r\n{}`),
    Buffer.from(`Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}`),
    Buffer.from(`Content-Length: 3\r\n\r\n{}`),
    Buffer.concat([frame, Buffer.from('x')]),
    Buffer.from(`Content-Length: 1\n\n{`),
    Buffer.concat([Buffer.from('Content-Length: 1\r\n\r\n', 'ascii'), Buffer.from([0xff])]),
  ]) assert.throws(() => decodeFrame(invalid));
});

named('ASSERT_GENERIC_SCHEMA_AND_STABLE_IDENTITY', async () => {
  assert.equal(PROVIDER_IDENTITY, 'ember-glint@1');
  const provider = createProvider({ analyzers: [] });
  const response = await provider.handle(request);
  assert.equal(response.schema, 'lsp-trace.provider-observations.v1');
  assert.equal(response.provider.identity, 'ember-glint@1');
  assert.equal(response.request_id, request.request_id);
});

named('ASSERT_PROVIDER_CUSTODY_AND_MAPPING_DELEGATION', async () => {
  const calls = [];
  const analyzer = Object.freeze({
    id: 'qualified-double@1',
    languages: ['glimmer-js'],
    frameworks: ['ember'],
    relationKinds: ['BINDS_ARGUMENT'],
    async analyze(input) {
      calls.push(input);
      return { outcome: 'COMPLETE', observations: [{ kind: 'BINDS_ARGUMENT', original_document: input.documents[0].uri }], coverage: { status: 'BOUNDED' } };
    },
  });
  const response = await createProvider({ analyzers: [analyzer] }).handle(request);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0], request);
  assert.equal(response.observations[0].kind, 'BINDS_ARGUMENT');
  assert.equal(response.observations[0].original_document, 'file:///app.gts');
});

named('ASSERT_BOUNDED_LIFECYCLE_OUTCOMES', async () => {
  assert.deepEqual(LIFECYCLE_OUTCOMES, ['BLOCKED', 'UNAVAILABLE', 'FAILED', 'PARTIAL', 'BOUNDED', 'EMPTY', 'COMPLETE']);
  const unavailable = await createProvider({ analyzers: [] }).handle(request);
  assert.equal(unavailable.outcome, 'UNAVAILABLE');
  const empty = await createProvider({ analyzers: [{ id: 'a', languages: ['glimmer-js'], frameworks: ['ember'], relationKinds: ['BINDS_ARGUMENT'], async analyze() { return { outcome: 'EMPTY', observations: [], coverage: { status: 'BOUNDED' } }; } }] }).handle(request);
  assert.equal(empty.outcome, 'EMPTY');

  const timedRequest = { ...request, limits: { ...request.limits, timeout_ms: 5 } };
  const timed = await createProvider({ analyzers: [{ id: 'slow', languages: ['glimmer-js'], frameworks: ['ember'], relationKinds: ['BINDS_ARGUMENT'], async analyze(_request, { signal }) { await new Promise((resolve) => signal.addEventListener('abort', resolve, { once: true })); return { outcome: 'EMPTY', observations: [], coverage: { status: 'BOUNDED' } }; } }] }).handle(timedRequest);
  assert.equal(timed.outcome, 'BLOCKED');
  assert.equal(timed.coverage.reason, 'ANALYZER_TIMEOUT');
});

named('ASSERT_DETERMINISTIC_CANONICAL_BYTES_LOGICAL_DIGEST', () => {
  const left = { z: 1, a: { y: 2, x: 3 } };
  const right = { a: { x: 3, y: 2 }, z: 1 };
  assert.deepEqual(canonicalBytes(left), canonicalBytes(right));
  assert.equal(logicalDigest(left), logicalDigest(right));
  assert.match(logicalDigest(left), /^sha256:[0-9a-f]{64}$/);
  assert.ok(!logicalDigest(left).includes('Content-Length'));
});

named('ASSERT_HONEST_CAPABILITY_METADATA', () => {
  const none = createProvider({ analyzers: [] }).metadata;
  assert.deepEqual(none.capabilities, { relation_kinds: [], languages: [], frameworks: [] });
  const one = createProvider({ analyzers: [{ id: 'a', languages: ['glimmer-js'], frameworks: ['ember'], relationKinds: ['BINDS_ARGUMENT'], async analyze() {} }] }).metadata;
  assert.deepEqual(one.capabilities, { relation_kinds: ['BINDS_ARGUMENT'], languages: ['glimmer-js'], frameworks: ['ember'] });
  assert.deepEqual(one.limits, { max_content_bytes: 1048576, max_observations: 10000, max_timeout_ms: 30000 });
});

named('ASSERT_INSTALL_ENTRY_POINT_OFFLINE', async () => {
  const { readFile } = await import('node:fs/promises');
  const { spawn } = await import('node:child_process');
  const { fileURLToPath } = await import('node:url');
  const pkg = JSON.parse(await readFile(new URL('../package.json', import.meta.url), 'utf8'));
  assert.equal(pkg.bin['ember-glint'], './bin/ember-glint.mjs');
  assert.deepEqual(pkg.dependencies, {
    '@glint/core': '1.5.2',
    '@glint/environment-ember-loose': '1.5.2',
    '@glint/environment-ember-template-imports': '1.5.2',
    'ember-source': '7.2.0',
    'tree-sitter': '0.21.1',
    'tree-sitter-typescript': '0.23.2',
    typescript: '5.9.2',
  });

  const child = spawn(process.execPath, [fileURLToPath(new URL('../bin/ember-glint.mjs', import.meta.url))], { stdio: ['pipe', 'pipe', 'pipe'] });
  const output = [];
  child.stdout.on('data', (chunk) => output.push(chunk));
  child.stdin.end(encodeFrame(request));
  const [code] = await new Promise((resolve) => child.once('close', (...args) => resolve(args)));
  assert.equal(code, 0);
  const response = decodeFrame(Buffer.concat(output));
  assert.equal(response.provider.identity, 'ember-glint@1');
  assert.equal(response.outcome, 'COMPLETE');
  assert.deepEqual(response.provider.capabilities.relation_kinds, ['BINDS_ARGUMENT']);
  assert.deepEqual(response.observations.map(({ kind }) => kind), ['BINDS_ARGUMENT']);
});
