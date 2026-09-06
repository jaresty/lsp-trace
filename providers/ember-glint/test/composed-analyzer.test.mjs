import assert from 'node:assert/strict';
import test from 'node:test';

import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const admitted = ['BINDS_ARGUMENT', 'INVOKES_TASK', 'PASSES_CALLBACK', 'RENDERS_FROM', 'TRIGGERS_RELOAD', 'UPDATES_STATE'];

test('ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS', () => {
  const analyzer = createDefaultAnalyzer();
  assert.deepEqual([...analyzer.relationKinds].sort(), admitted, 'ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS');
  assert.deepEqual([...analyzer.supportedRelations].sort(), admitted, 'ASSERT_COMPOSED_EMBER_ANALYZER_EXACT_RELATIONS');
});
