import assert from 'node:assert/strict';
import test from 'node:test';

import { SUPPORTED_RELATIONS, createAnalyzer } from '../analyzer.mjs';

const originalAnchor = Object.freeze({
  uri: 'file:///workspace/component.gts',
  bytes: { start: 41, end: 55 },
  range: { start: { line: 2, character: 4 }, end: { line: 2, character: 18 } },
  text: 'this.itemCount',
});
const virtualMapping = Object.freeze({
  uri: 'file:///workspace/component.ts',
  original: { start: 41, end: 55 },
  virtual: { start: 210, end: 224 },
});

function analyzer(overrides = {}) {
  return createAnalyzer({
    templateExtractor: { extract: () => ({ status: 'SUPPORTED', observations: [] }) },
    scriptExtractor: { extract: () => ({ coverage: { status: 'BOUNDED' }, observations: [] }) },
    glintAnalyzer: { analyze: () => ({ status: 'SCOPED_ROLE', observations: [] }) },
    ...overrides,
  });
}

test('ASSERT_ANALYZER_ADVERTISES_ONLY_INDEPENDENT_RELATIONS', () => {
  assert.deepEqual(SUPPORTED_RELATIONS, ['BINDS_ARGUMENT', 'SCRIPT_SYMBOL_DEFINITION', 'TYPED_TEMPLATE_DEFINITION'], 'ASSERT_ANALYZER_ADVERTISES_ONLY_INDEPENDENT_RELATIONS');
});

test('ASSERT_ANALYZER_PRESERVES_EXACT_ORIGINAL_ANCHOR_AND_OPTIONAL_MAPPING', () => {
  const observation = { kind: 'TYPED_TEMPLATE_DEFINITION', original_anchor: originalAnchor, virtual_mapping: virtualMapping };
  const result = analyzer({ glintAnalyzer: { analyze: () => ({ status: 'SCOPED_ROLE', observations: [observation] }) } }).analyze({
    kind: 'glint',
    glintConfigAvailable: true,
  });
  assert.deepEqual(result.observations, [observation], 'ASSERT_ANALYZER_PRESERVES_EXACT_ORIGINAL_ANCHOR_AND_OPTIONAL_MAPPING');
  assert.equal(result.observations[0].original_anchor, originalAnchor, 'ASSERT_ANALYZER_PRESERVES_EXACT_ORIGINAL_ANCHOR_AND_OPTIONAL_MAPPING');
  assert.equal(result.observations[0].virtual_mapping, virtualMapping, 'ASSERT_ANALYZER_PRESERVES_EXACT_ORIGINAL_ANCHOR_AND_OPTIONAL_MAPPING');
});

test('ASSERT_ANALYZER_MISSING_GLINT_CONFIG_BLOCKED_NOT_EMPTY', () => {
  const result = analyzer().analyze({ kind: 'glint', glintConfigAvailable: false });
  assert.deepEqual(result, {
    status: 'BLOCKED',
    reason: 'GLINT_CONFIG_UNAVAILABLE',
    observations: [],
    coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
  }, 'ASSERT_ANALYZER_MISSING_GLINT_CONFIG_BLOCKED_NOT_EMPTY');
});

test('ASSERT_ANALYZER_DETERMINISTIC_AND_RELATION_FILTERED', () => {
  const observations = [
    { kind: 'UNSUPPORTED_RUNTIME_CALL', original_anchor: { ...originalAnchor, bytes: { start: 80, end: 81 } } },
    { kind: 'BINDS_ARGUMENT', original_anchor: { ...originalAnchor, bytes: { start: 20, end: 21 } } },
    { kind: 'BINDS_ARGUMENT', original_anchor: { ...originalAnchor, bytes: { start: 10, end: 11 } } },
  ];
  const subject = analyzer({ templateExtractor: { extract: () => ({ status: 'SUPPORTED', observations }) } });
  const first = subject.analyze({ kind: 'template' });
  const second = subject.analyze({ kind: 'template' });
  assert.deepEqual(first, second, 'ASSERT_ANALYZER_DETERMINISTIC_AND_RELATION_FILTERED');
  assert.deepEqual(first.observations.map(({ kind }) => kind), ['BINDS_ARGUMENT', 'BINDS_ARGUMENT'], 'ASSERT_ANALYZER_DETERMINISTIC_AND_RELATION_FILTERED');
  assert.deepEqual(first.observations.map(({ original_anchor }) => original_anchor.bytes.start), [10, 20], 'ASSERT_ANALYZER_DETERMINISTIC_AND_RELATION_FILTERED');
});
