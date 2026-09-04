import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';

import {
  EMBER_TEMPLATE_COMPILER_VERSION,
  SUPPORTED_RELATION_KINDS,
  createTemplateObservationExtractor,
} from './ember-template-observations.mjs';

const require = createRequire(import.meta.url);
const compiler = require('../qualification/provider-qualification/node_modules/ember-source/dist/dev/packages/ember-template-compiler/index.js');
const compilerVersion = require('../qualification/provider-qualification/node_modules/ember-source/package.json').version;
const document = {
  document_id: 'doc-template',
  uri: 'file:///workspace/app/components/example.hbs',
  revision: 'rev-1',
  blob: 'sha256:template',
};
const forbidden = [
  'callback_execution',
  'task_execution',
  'runtime',
  'repaint',
  'feature_identity',
  'rendering_execution',
  'source_completeness',
];

function offsetAt(source, point) {
  const lines = source.split('\n');
  let offset = 0;
  for (let line = 0; line < point.line; line += 1) offset += lines[line].length + 1;
  return offset + point.character;
}

function anchoredText(source, anchor) {
  return source.slice(offsetAt(source, anchor.range.start), offsetAt(source, anchor.range.end));
}

function extractor() {
  return createTemplateObservationExtractor({ compiler, compilerVersion });
}

test('advertises only AST-qualified BINDS_ARGUMENT', () => {
  assert.equal(EMBER_TEMPLATE_COMPILER_VERSION, '7.2.0', 'P2a pinned compiler version');
  assert.deepEqual(SUPPORTED_RELATION_KINDS, ['BINDS_ARGUMENT'], 'P3b exact advertised relation kinds');
});

test('extracts deterministic argument bindings with exact original anchors', () => {
  const source = [
    '<First @value={{this.alpha}} />',
    '<Second @later={{@beta}} @plain="not-an-expression" />',
  ].join('\n');
  const first = extractor().extract({ source, document });
  const second = extractor().extract({ source, document });

  assert.deepEqual(first, second, 'P3a deterministic observations');
  assert.equal(first.status, 'SUPPORTED', 'P3b supported outcome');
  assert.deepEqual(first.coverage, {
    status: 'BOUNDED',
    boundary: 'named component argument attributes whose value is a mustache PathExpression',
    examined: 3,
    emitted: 2,
    unsupported: 1,
  }, 'P3b explicit bounded coverage');
  assert.deepEqual(first.observations.map(({ kind }) => kind), ['BINDS_ARGUMENT', 'BINDS_ARGUMENT']);
  assert.deepEqual(first.observations.map(({ from }) => from.node_id), ['path:this.alpha', 'path:@beta']);
  assert.deepEqual(first.observations.map(({ to }) => to.node_id), ['argument:First:@value', 'argument:Second:@later']);

  assert.deepEqual(
    first.observations.map(({ original_anchor }) => anchoredText(source, original_anchor)),
    ['@value={{this.alpha}}', '@later={{@beta}}'],
    'P2b exact original source anchor',
  );
  for (const observation of first.observations) {
    assert.deepEqual(observation.supports, ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
    assert.deepEqual(observation.does_not_support, forbidden, 'P4a forbidden claims disclaimed');
  }
  assert.equal(first.unsupported[0].reason, 'UNQUALIFIED_ATTRIBUTE_VALUE', 'P3b explicit unsupported evidence');
  assert.equal(anchoredText(source, first.unsupported[0].original_anchor), '@plain="not-an-expression"');
});

test('reports unsupported requests without parsing or claiming absence', () => {
  const result = extractor().extract({ source: '<Thing @x={{this.x}} />', document, relationKinds: ['PASSES_CALLBACK'] });
  assert.deepEqual(result, {
    status: 'UNSUPPORTED',
    requested_relation_kinds: ['PASSES_CALLBACK'],
    supported_relation_kinds: ['BINDS_ARGUMENT'],
    observations: [],
    coverage: { status: 'UNKNOWN', reason: 'RELATION_NOT_SUPPORTED' },
  }, 'P3b unsupported relation outcome');
});

test('reports compiler failures distinctly and emits no observations', () => {
  const result = extractor().extract({ source: '{{', document });
  assert.equal(result.status, 'FAILURE', 'P3b compiler failure outcome');
  assert.equal(result.failure.kind, 'TEMPLATE_PARSE_FAILED');
  assert.equal(typeof result.failure.message, 'string');
  assert.deepEqual(result.observations, []);
  assert.deepEqual(result.coverage, { status: 'UNKNOWN', reason: 'TEMPLATE_PARSE_FAILED' });
});

test('rejects unpinned compiler versions before extraction', () => {
  assert.throws(
    () => createTemplateObservationExtractor({ compiler, compilerVersion: '7.2.1' }),
    /requires ember-source 7\.2\.0/,
    'P2a reject unpinned compiler version',
  );
});
