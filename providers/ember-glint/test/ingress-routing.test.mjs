import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const source = '<Widget @value={{this.x}} />';
const document = Object.freeze({
  uri: 'file:///app.gts',
  language: 'glimmer-js',
  revision: 'r1',
  digest: `sha256:${'a'.repeat(64)}`,
  source,
});

function request(documents) {
  return {
    schema: 'lsp-trace.provider-request.v1',
    request_id: 'ingress-routing',
    operation: 'analyze',
    relation_kinds: ['BINDS_ARGUMENT'],
    documents,
    limits: { max_observations: 10, timeout_ms: 1000 },
  };
}

test('ASSERT_INGRESS_FILE_PROJECT_ROUTES_AUTHORITATIVE_ANALYSIS', async () => {
  const uri = new URL('../fixtures/component.gts', import.meta.url).href;
  const projectDocument = {
    ...document,
    uri,
    source: readFileSync(fileURLToPath(uri), 'utf8'),
    position: { line: 14, character: 14 },
    range: { start: { line: 14, character: 9 }, end: { line: 14, character: 23 } },
  };
  const result = await createDefaultAnalyzer().analyze(request([projectDocument]));
  assert.equal(result.outcome, 'EMPTY', 'ASSERT_INGRESS_FILE_PROJECT_ROUTES_AUTHORITATIVE_ANALYSIS');
  assert.equal(result.coverage.reason, 'REQUESTED_GLINT_QUERY', 'ASSERT_INGRESS_FILE_PROJECT_ROUTES_AUTHORITATIVE_ANALYSIS');
  assert.deepEqual(result.observations, [], 'ASSERT_INGRESS_FILE_PROJECT_ROUTES_AUTHORITATIVE_ANALYSIS');
});

test('ASSERT_INGRESS_AMBIGUOUS_DOCUMENT_CONTEXT_FAILS_CLOSED', async () => {
  const result = await createDefaultAnalyzer().analyze(request([
    document,
    { ...document, uri: 'file:///other.gts', revision: 'r2' },
  ]));
  assert.equal(result.outcome, 'UNAVAILABLE', 'ASSERT_INGRESS_AMBIGUOUS_DOCUMENT_CONTEXT_FAILS_CLOSED');
  assert.equal(result.coverage.reason, 'AMBIGUOUS_PROJECT_CONTEXT', 'ASSERT_INGRESS_AMBIGUOUS_DOCUMENT_CONTEXT_FAILS_CLOSED');
  assert.deepEqual(result.observations, [], 'ASSERT_INGRESS_AMBIGUOUS_DOCUMENT_CONTEXT_FAILS_CLOSED');
});

test('ASSERT_INGRESS_UNSUPPORTED_URI_CONTEXT_FAILS_CLOSED', async () => {
  const result = await createDefaultAnalyzer().analyze(request([{ ...document, uri: 'https://example.test/app.gts' }]));
  assert.equal(result.outcome, 'UNAVAILABLE', 'ASSERT_INGRESS_UNSUPPORTED_URI_CONTEXT_FAILS_CLOSED');
  assert.equal(result.coverage.reason, 'PROJECT_CONTEXT_UNAVAILABLE', 'ASSERT_INGRESS_UNSUPPORTED_URI_CONTEXT_FAILS_CLOSED');
  assert.deepEqual(result.observations, [], 'ASSERT_INGRESS_UNSUPPORTED_URI_CONTEXT_FAILS_CLOSED');
});
