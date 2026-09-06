import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import { createTemplateRelationAdapter } from '../analyzers/template-relations.mjs';
import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const anchor = Object.freeze({ uri: 'file:///app.gts', revision: 'r1', blob: 'a'.repeat(64), range: { start: { line: 0, character: 1 }, end: { line: 0, character: 2 } } });
const records = Object.freeze([
  { kind: 'RENDERS_FROM', identity: { from: 'property:itemCount', to: 'expression:1' }, mapping: { original: anchor, virtual: { uri: 'file:///app.gts.ts', range: anchor.range } }, anchors: [anchor], operation: 'glint:getDefinitionAtPosition' },
  { kind: 'BINDS_ARGUMENT', identity: { from: 'path:this.value', to: 'argument:Widget:@value' }, mapping: { original: anchor }, anchors: [anchor], operation: 'ember:PathExpression+glint:mapping' },
  { kind: 'PASSES_CALLBACK', identity: { from: 'callable:onSave', to: 'parameter:handler' }, mapping: { original: anchor }, anchors: [anchor], operation: 'glint:getDefinitionAtPosition+typescript:resolvedSignature' },
]);

const expectedKinds = ['BINDS_ARGUMENT', 'PASSES_CALLBACK', 'RENDERS_FROM'];

function subject() {
  return createTemplateRelationAdapter();
}

for (const kind of expectedKinds) {
  test(`ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`, () => {
    const result = subject().normalize(records.filter((record) => record.kind === kind));
    assert.equal(result.status, 'SUPPORTED', `ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`);
    assert.equal(result.observations.length, 1, `ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`);
    assert.equal(result.observations[0].kind, kind, `ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`);
    assert.equal(result.observations[0].from.node_id, records.find((record) => record.kind === kind).identity.from, `ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`);
    assert.deepEqual(result.observations[0].anchors, [anchor], `ASSERT_TYPED_TEMPLATE_ADAPTER_${kind}_USES_UPSTREAM_IDENTITY_MAPPING`);
  });
}

test('ASSERT_TYPED_TEMPLATE_ADAPTER_DETERMINISTIC_ORDER', () => {
  const first = subject().normalize(records);
  const second = subject().normalize([...records].reverse());
  assert.deepEqual(first, second, 'ASSERT_TYPED_TEMPLATE_ADAPTER_DETERMINISTIC_ORDER');
  assert.deepEqual(first.observations.map(({ kind }) => kind), expectedKinds, 'ASSERT_TYPED_TEMPLATE_ADAPTER_DETERMINISTIC_ORDER');
});

test('ASSERT_TYPED_TEMPLATE_ADAPTER_REJECTS_UNAUTHORITATIVE_RECORDS', () => {
  for (const patch of [{ identity: undefined }, { mapping: undefined }, { anchors: [] }, { operation: 'identifier-spelling-match' }]) {
    const result = subject().normalize([{ ...records[0], ...patch }]);
    assert.deepEqual(result, { status: 'BLOCKED', reason: 'AUTHORITATIVE_TEMPLATE_RELATION_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } }, 'ASSERT_TYPED_TEMPLATE_ADAPTER_REJECTS_UNAUTHORITATIVE_RECORDS');
  }
});

test('ASSERT_TYPED_TEMPLATE_PROVIDER_NORMALIZES_AUTHORITATIVE_UPSTREAM_RESULTS', async () => {
  const result = await createDefaultAnalyzer().analyze({
    schema: 'lsp-trace.provider-request.v1',
    relation_kinds: expectedKinds,
    documents: [{ uri: anchor.uri, revision: anchor.revision, digest: `sha256:${anchor.blob}`, source: '', upstream_template_relations: records }],
  });
  assert.equal(result.outcome, 'COMPLETE', 'ASSERT_TYPED_TEMPLATE_PROVIDER_NORMALIZES_AUTHORITATIVE_UPSTREAM_RESULTS');
  assert.deepEqual(result.observations.map(({ kind }) => kind), expectedKinds, 'ASSERT_TYPED_TEMPLATE_PROVIDER_NORMALIZES_AUTHORITATIVE_UPSTREAM_RESULTS');
});

test('ASSERT_TYPED_TEMPLATE_ADAPTER_HAS_NO_RECURSIVE_SEMANTIC_WALKER_OR_SPELLING_INFERENCE', async () => {
  const source = await readFile(new URL('../analyzers/template-relations.mjs', import.meta.url), 'utf8');
  for (const forbidden of ['forEachChild', 'createSourceFile', 'descendantsOfType', 'identifier-spelling', 'memberName']) {
    assert.equal(source.includes(forbidden), false, `ASSERT_TYPED_TEMPLATE_ADAPTER_HAS_NO_RECURSIVE_SEMANTIC_WALKER_OR_SPELLING_INFERENCE:${forbidden}`);
  }
});
