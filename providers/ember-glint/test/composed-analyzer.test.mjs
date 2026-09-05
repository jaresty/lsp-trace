import assert from 'node:assert/strict';
import test from 'node:test';

import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const admitted = ['BINDS_ARGUMENT', 'PASSES_CALLBACK', 'RENDERS_FROM', 'UPDATES_STATE'];
const blocked = ['INVOKES_TASK', 'TRIGGERS_RELOAD'];

test('ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS', () => {
  const analyzer = createDefaultAnalyzer();
  assert.deepEqual([...analyzer.relationKinds].sort(), admitted, 'ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS');
  assert.deepEqual([...analyzer.supportedRelations].sort(), admitted, 'ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS');
});

test('ASSERT_COMPOSED_EMBER_ANALYZER_BLOCKED_RELATIONS_NOT_ADVERTISED', () => {
  const analyzer = createDefaultAnalyzer();
  for (const relation of blocked) {
    assert.equal(analyzer.relationKinds.includes(relation), false, 'ASSERT_COMPOSED_EMBER_ANALYZER_BLOCKED_RELATIONS_NOT_ADVERTISED');
    assert.equal(analyzer.supportedRelations.includes(relation), false, 'ASSERT_COMPOSED_EMBER_ANALYZER_BLOCKED_RELATIONS_NOT_ADVERTISED');
  }
});
