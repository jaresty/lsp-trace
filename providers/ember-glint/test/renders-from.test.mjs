import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { createRendersFromAnalyzer } from '../renders-from-analyzer.mjs';

const template = '<p>{{this.itemCount}}</p>';
const script = `import Component from '@glimmer/component';
export default class Panel extends Component<{ Args: { data: string[] } }> {
  get itemCount(): number {
    return this.args.data.length;
  }
}`;
const templateStart = template.indexOf('this.itemCount');
const definitionStart = script.indexOf('itemCount');
const readStart = script.indexOf('this.args.data');
const templateUri = 'file:///workspace/app/components/panel.gts';
const scriptUri = 'file:///workspace/app/components/panel.ts';

function anchor(uri, source, start, text) {
  const before = source.slice(0, start).split('\n');
  const point = { line: before.length - 1, character: before.at(-1).length };
  const endBefore = source.slice(0, start + text.length).split('\n');
  return {
    uri,
    bytes: { start: Buffer.byteLength(source.slice(0, start)), end: Buffer.byteLength(source.slice(0, start + text.length)) },
    range: { start: point, end: { line: endBefore.length - 1, character: endBefore.at(-1).length } },
    text,
  };
}

const qualified = {
  projectDirectory: '/workspace',
  template: { uri: templateUri, source: template, expression: { start: templateStart, end: templateStart + 'this.itemCount'.length } },
  mapping: { uri: 'file:///workspace/.glint/panel.ts', original: { start: templateStart, end: templateStart + 14 }, virtual: { start: 500, end: 514 } },
  definitions: [{ uri: scriptUri, source: script, start: definitionStart, end: definitionStart + 'itemCount'.length, generated: false }],
};

const expected = {
  kind: 'RENDERS_FROM',
  from: { node_id: `${scriptUri}#property:itemCount`, role: 'SOURCE_PROPERTY' },
  to: { node_id: `${templateUri}#expression:${templateStart}:${templateStart + 14}`, role: 'RENDER_EXPRESSION' },
  original_anchor: anchor(templateUri, template, templateStart, 'this.itemCount'),
  definition_anchor: anchor(scriptUri, script, definitionStart, 'itemCount'),
  source_read_anchor: anchor(scriptUri, script, readStart, 'this.args.data'),
  virtual_mapping: qualified.mapping,
  support: { outcome: 'SCOPED_ROLE', operation: 'glint.definition+typescript.direct-qualified-read' },
  supports: ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation'],
  does_not_support: ['repaint', 'render_occurrence', 'runtime_execution', 'callback_execution', 'feature_identity', 'source_completeness'],
};

function subject() {
  return createRendersFromAnalyzer();
}

const fixtureDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');
function frozen(name) {
  const bytes = readFileSync(path.join(fixtureDirectory, name));
  return { value: JSON.parse(bytes), digest: createHash('sha256').update(bytes).digest('hex') };
}

test('ASSERT_RENDERS_FROM_EXACT_TYPED_GETTER_DIRECT_QUALIFIED_READ_EVIDENCE', () => {
  const result = subject().analyze(qualified);
  assert.equal(result.status, 'SCOPED_ROLE', 'ASSERT_RENDERS_FROM_EXACT_TYPED_GETTER_DIRECT_QUALIFIED_READ_EVIDENCE');
  assert.deepEqual(result.observations, [expected], 'ASSERT_RENDERS_FROM_EXACT_TYPED_GETTER_DIRECT_QUALIFIED_READ_EVIDENCE');
  assert.deepEqual(result.coverage, { status: 'BOUNDED', boundary: 'one exact Glint template expression mapped to one source-declared typed getter/property with one direct this.args source read' }, 'ASSERT_RENDERS_FROM_EXACT_TYPED_GETTER_DIRECT_QUALIFIED_READ_EVIDENCE');
});

test('ASSERT_RENDERS_FROM_EXACT_RESOLVED_PROPERTY_NOT_IDENTIFIER_MATCH', () => {
  const propertySource = script.replace('get itemCount(): number {\n    return this.args.data.length;\n  }', 'total: number = this.args.data.length;');
  const propertyTemplate = template.replace('this.itemCount', 'this.total');
  const start = propertyTemplate.indexOf('this.total');
  const definition = propertySource.indexOf('total');
  const result = subject().analyze({
    ...qualified,
    template: { uri: templateUri, source: propertyTemplate, expression: { start, end: start + 'this.total'.length } },
    mapping: { ...qualified.mapping, original: { start, end: start + 'this.total'.length }, virtual: { start: 600, end: 610 } },
    definitions: [{ ...qualified.definitions[0], source: propertySource, start: definition, end: definition + 'total'.length }],
  });
  assert.equal(result.status, 'SCOPED_ROLE', 'ASSERT_RENDERS_FROM_EXACT_RESOLVED_PROPERTY_NOT_IDENTIFIER_MATCH');
  assert.equal(result.observations[0].from.node_id, `${scriptUri}#property:total`, 'ASSERT_RENDERS_FROM_EXACT_RESOLVED_PROPERTY_NOT_IDENTIFIER_MATCH');
  assert.equal(result.observations[0].original_anchor.text, 'this.total', 'ASSERT_RENDERS_FROM_EXACT_RESOLVED_PROPERTY_NOT_IDENTIFIER_MATCH');
  assert.equal(result.observations[0].source_read_anchor.text, 'this.args.data', 'ASSERT_RENDERS_FROM_EXACT_RESOLVED_PROPERTY_NOT_IDENTIFIER_MATCH');
});

test('ASSERT_RENDERS_FROM_DETERMINISTIC', () => {
  const first = subject().analyze(qualified);
  const second = subject().analyze(qualified);
  assert.equal(first.status, 'SCOPED_ROLE', 'ASSERT_RENDERS_FROM_DETERMINISTIC');
  assert.deepEqual(first, second, 'ASSERT_RENDERS_FROM_DETERMINISTIC');
  assert.deepEqual(first.observations, [expected], 'ASSERT_RENDERS_FROM_DETERMINISTIC');
});

test('ASSERT_RENDERS_FROM_UNRESOLVED_MULTIPLE_OUTSIDE_GENERATED_ONLY_BLOCKED', () => {
  const cases = [
    ['DEFINITION_UNRESOLVED', { definitions: [] }],
    ['DEFINITION_AMBIGUOUS', { definitions: [qualified.definitions[0], { ...qualified.definitions[0], start: definitionStart + 1 }] }],
    ['DEFINITION_OUTSIDE_PROJECT', { definitions: [{ ...qualified.definitions[0], uri: 'file:///other/panel.ts' }] }],
    ['DEFINITION_GENERATED_ONLY', { definitions: [{ ...qualified.definitions[0], generated: true }] }],
  ];
  for (const [reason, patch] of cases) {
    assert.deepEqual(subject().analyze({ ...qualified, ...patch }), {
      status: 'BLOCKED', reason, observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
    }, `ASSERT_RENDERS_FROM_UNRESOLVED_MULTIPLE_OUTSIDE_GENERATED_ONLY_BLOCKED:${reason}`);
  }
});

test('ASSERT_RENDERS_FROM_REJECTS_IDENTIFIER_TEXT_AND_GENERAL_DATAFLOW', () => {
  const identifierOnly = qualified.template.source.replace('this.itemCount', 'this.data');
  const indirectScript = script.replace('return this.args.data.length;', 'const value = this.args.data;\n    return value.length;');
  assert.deepEqual(subject().analyze({ ...qualified, template: { ...qualified.template, source: identifierOnly } }), {
    status: 'BLOCKED', reason: 'ORIGINAL_EXPRESSION_MISMATCH', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
  }, 'ASSERT_RENDERS_FROM_REJECTS_IDENTIFIER_TEXT_AND_GENERAL_DATAFLOW:identifier');
  assert.deepEqual(subject().analyze({ ...qualified, definitions: [{ ...qualified.definitions[0], source: indirectScript }] }), {
    status: 'BLOCKED', reason: 'DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
  }, 'ASSERT_RENDERS_FROM_REJECTS_IDENTIFIER_TEXT_AND_GENERAL_DATAFLOW:dataflow');
});

test('ASSERT_RENDERS_FROM_NON_ENTAILMENTS_EXACT', () => {
  const [observation] = subject().analyze(qualified).observations;
  assert.deepEqual(observation?.does_not_support, ['repaint', 'render_occurrence', 'runtime_execution', 'callback_execution', 'feature_identity', 'source_completeness'], 'ASSERT_RENDERS_FROM_NON_ENTAILMENTS_EXACT');
  assert.equal('runtime' in (observation ?? {}), false, 'ASSERT_RENDERS_FROM_NON_ENTAILMENTS_EXACT');
  assert.equal('feature' in (observation ?? {}), false, 'ASSERT_RENDERS_FROM_NON_ENTAILMENTS_EXACT');
});

test('ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE', () => {
  const positive = frozen('renders-from-positive.json');
  const negative = frozen('renders-from-negative.json');
  const retained = frozen('renders-from-retained-evidence.json').value;
  assert.equal(positive.value.expected.status, 'SCOPED_ROLE', 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:positive');
  assert.equal(positive.value.expected.observations[0].kind, 'RENDERS_FROM', 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:positive');
  assert.deepEqual(negative.value.expected, { status: 'BLOCKED', reason: 'DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } }, 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:negative');
  assert.deepEqual(retained.seed_sha256, { positive: positive.digest, negative: negative.digest }, 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:digests');
  assert.equal(retained.replay_equal, true, 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:replay');
  assert.deepEqual(retained.does_not_establish, ['repaint', 'render_occurrence', 'runtime_execution', 'callback_execution', 'feature_identity', 'source_completeness'], 'ASSERT_RENDERS_FROM_FROZEN_SEEDS_AND_RETAINED_EVIDENCE:ceiling');
});
