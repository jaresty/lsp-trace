import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const analyzer = createDefaultAnalyzer();

test('ASSERT_PACKAGE_PINNED_TEMPLATE_AND_SCRIPT_ANALYZERS', () => {
  const document = { document_id: 'template', uri: 'file:///workspace/example.hbs', revision: '1', blob: 'sha256:fixture' };
  const template = analyzer.analyze({ kind: 'template', source: '<Thing @value={{this.value}} />', document });
  assert.equal(template.status, 'SUPPORTED', 'ASSERT_PACKAGE_PINNED_TEMPLATE_AND_SCRIPT_ANALYZERS');
  assert.deepEqual(template.observations.map(({ kind }) => kind), ['BINDS_ARGUMENT'], 'ASSERT_PACKAGE_PINNED_TEMPLATE_AND_SCRIPT_ANALYZERS');

  const script = analyzer.analyze({ kind: 'script', documents: [{
    uri: 'file:///workspace/plain.ts',
    language: 'typescript',
    source: 'export function leaf() {}\nleaf();\n',
  }] });
  assert.equal(script.coverage.status, 'BOUNDED', 'ASSERT_PACKAGE_PINNED_TEMPLATE_AND_SCRIPT_ANALYZERS');
  assert.deepEqual(script.observations.map(({ kind }) => kind), ['SCRIPT_SYMBOL_DEFINITION'], 'ASSERT_PACKAGE_PINNED_TEMPLATE_AND_SCRIPT_ANALYZERS');
});

test('ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING', () => {
  const first = analyzer.analyze({ kind: 'glint', glintConfigAvailable: true, projectDirectory: root });
  const second = analyzer.analyze({ kind: 'glint', glintConfigAvailable: true, projectDirectory: root });
  assert.deepEqual(first, second, 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
  assert.equal(first.status, 'SCOPED_ROLE', 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
  assert.deepEqual(first.observations.map(({ kind }) => kind), ['TYPED_TEMPLATE_DEFINITION'], 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
  const observation = first.observations[0];
  assert.equal(observation.original_anchor.text, 'this.itemCount', 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
  assert.deepEqual(observation.virtual_mapping.original, observation.original_anchor.bytes, 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
  assert.equal(observation.support.outcome, 'SCOPED_ROLE', 'ASSERT_PACKAGE_FROZEN_GLINT_SCOPED_RELATION_EXACT_MAPPING');
});

test('ASSERT_PACKAGE_MISSING_GLINT_CONFIG_EXPLICIT_BLOCKED', () => {
  const result = analyzer.analyze({ kind: 'glint', glintConfigAvailable: false, projectDirectory: '/missing' });
  assert.equal(result.status, 'BLOCKED', 'ASSERT_PACKAGE_MISSING_GLINT_CONFIG_EXPLICIT_BLOCKED');
  assert.equal(result.reason, 'GLINT_CONFIG_UNAVAILABLE', 'ASSERT_PACKAGE_MISSING_GLINT_CONFIG_EXPLICIT_BLOCKED');
  assert.equal(result.coverage.status, 'UNKNOWN', 'ASSERT_PACKAGE_MISSING_GLINT_CONFIG_EXPLICIT_BLOCKED');
});
