import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import Parser from '../../qualification/provider-qualification/node_modules/tree-sitter/index.js';
import TypeScriptLanguages from '../../qualification/provider-qualification/node_modules/tree-sitter-typescript/bindings/node/index.js';
import * as tsModule from '../../qualification/provider-qualification/node_modules/typescript/lib/typescript.js';
import { createScriptSymbolExtractor, PINNED_ANALYZERS } from './script-symbol-extraction.mjs';

const require = createRequire(new URL('../../qualification/provider-qualification/package.json', import.meta.url));
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const versions = {
  typescript: require('typescript/package.json').version,
  treeSitter: require('tree-sitter/package.json').version,
  treeSitterTypeScript: require('tree-sitter-typescript/package.json').version,
};
const documents = [
  {
    uri: 'file:///workspace/plain.ts',
    language: 'typescript',
    source: 'export function leaf(value: number) { return value; }\nconst input = 1;\nleaf(input);\n',
  },
];

function extractor(overrides = {}) {
  return createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions, ...overrides });
}

function leafObservation(result) {
  return result.observations.find((item) => item.symbol === 'leaf');
}

function assertResolution(result) {
  assert.equal(result.coverage.status, 'BOUNDED', 'ASSERT_SCRIPT_SYMBOL_TS_DEFINITION');
  const leaf = leafObservation(result);
  assert.ok(leaf, 'ASSERT_SCRIPT_SYMBOL_TS_DEFINITION');
  assert.equal(leaf.resolution.provider, `typescript@${PINNED_ANALYZERS.typescript}`, 'ASSERT_SCRIPT_SYMBOL_TS_DEFINITION');
}

function assertAnchors(result) {
  const leaf = leafObservation(result);
  assert.ok(leaf, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
  assert.deepEqual(leaf.reference.anchor, {
    uri: 'file:///workspace/plain.ts',
    bytes: { start: 71, end: 75 },
    range: { start: { row: 2, column: 0 }, end: { row: 2, column: 4 } },
    text: 'leaf',
  }, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
  assert.deepEqual(leaf.definition.anchor, {
    uri: 'file:///workspace/plain.ts',
    bytes: { start: 16, end: 20 },
    range: { start: { row: 0, column: 16 }, end: { row: 0, column: 20 } },
    text: 'leaf',
  }, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
}

function assertHonestCoverage(result) {
  assert.deepEqual(result, {
    schema_version: 'lsp-trace.script-symbol-observations.v1',
    analyzer: { typescript: '5.9.2', tree_sitter: '0.21.1', tree_sitter_typescript: '0.23.2' },
    observations: [],
    coverage: { status: 'UNSUPPORTED', reason: 'unsupported_language', documents: ['file:///workspace/component.hbs'] },
  }, 'ASSERT_SCRIPT_SYMBOL_UNSUPPORTED_NOT_EMPTY');
}

function assertDeterministic(first, second) {
  assert.equal(JSON.stringify(first), JSON.stringify(second), 'ASSERT_SCRIPT_SYMBOL_DETERMINISTIC');
}

function assertBounded(result) {
  const serialized = JSON.stringify(result);
  for (const forbidden of ['runtime_execution', 'callback_invocation', 'task_execution', 'repaint', 'feature_identity', 'complete']) {
    assert.equal(serialized.includes(forbidden), false, 'ASSERT_SCRIPT_SYMBOL_NO_RUNTIME_OR_COMPLETENESS');
  }
}

function assertFailure(result) {
  assert.deepEqual(result.coverage, {
    status: 'FAILED',
    reason: 'syntax_analyzer_failure',
    documents: ['file:///workspace/plain.ts'],
  }, 'ASSERT_SCRIPT_SYMBOL_FAILURE_DISTINCT');
  assert.deepEqual(result.observations, [], 'ASSERT_SCRIPT_SYMBOL_FAILURE_DISTINCT');
}

test('ASSERT_SCRIPT_SYMBOL_TS_DEFINITION', () => {
  assertResolution(extractor().extract({ documents }));
});

test('ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR', () => {
  assertAnchors(extractor().extract({ documents }));
});

test('ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR preserves UTF-8 bytes while TypeScript uses UTF-16 offsets', () => {
  const unicodeDocuments = [{
    uri: 'file:///workspace/unicode.ts',
    language: 'typescript',
    source: "const marker = 'é';\nexport function leaf() {}\nleaf();\n",
  }];
  const leaf = leafObservation(extractor().extract({ documents: unicodeDocuments }));
  assert.ok(leaf, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
  assert.deepEqual(leaf.reference.anchor.bytes, { start: 47, end: 51 }, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
  assert.deepEqual(leaf.reference.anchor.range, { start: { row: 2, column: 0 }, end: { row: 2, column: 4 } }, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
  assert.deepEqual(leaf.definition.anchor.bytes, { start: 37, end: 41 }, 'ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR');
});

test('ASSERT_SCRIPT_SYMBOL_UNSUPPORTED_NOT_EMPTY', () => {
  assertHonestCoverage(extractor().extract({ documents: [{ uri: 'file:///workspace/component.hbs', language: 'handlebars', source: '{{x}}' }] }));
});

test('ASSERT_SCRIPT_SYMBOL_DETERMINISTIC', () => {
  const second = { uri: 'file:///workspace/other.js', language: 'javascript', source: 'export const other = 1;\nother;\n' };
  assertDeterministic(extractor().extract({ documents: [...documents, second] }), extractor().extract({ documents: [second, ...documents] }));
});

test('ASSERT_SCRIPT_SYMBOL_NO_RUNTIME_OR_COMPLETENESS', () => {
  assertBounded(extractor().extract({ documents }));
});

test('ASSERT_SCRIPT_SYMBOL_FAILURE_DISTINCT', () => {
  class FailingParser { setLanguage() {} parse() { throw new Error('/volatile/path parser exploded'); } }
  assertFailure(extractor({ Parser: FailingParser }).extract({ documents }));
});

test('ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS', () => {
  assert.throws(() => createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions: { ...versions, typescript: '5.9.1' } }), /ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS/, 'ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS');
});

if (process.argv.includes('--witness')) {
  const valid = extractor().extract({ documents });
  const cases = [
    ['ASSERT_SCRIPT_SYMBOL_TS_DEFINITION', () => assertResolution({ ...valid, coverage: { status: 'UNKNOWN' } })],
    ['ASSERT_SCRIPT_SYMBOL_EXACT_ORIGINAL_ANCHOR', () => { const wrong = structuredClone(valid); wrong.observations.find((item) => item.symbol === 'leaf').reference.anchor.bytes.start++; assertAnchors(wrong); }],
    ['ASSERT_SCRIPT_SYMBOL_UNSUPPORTED_NOT_EMPTY', () => assertHonestCoverage({ ...valid, observations: [] })],
    ['ASSERT_SCRIPT_SYMBOL_DETERMINISTIC', () => assertDeterministic({ ...valid, nonce: 1 }, { ...valid, nonce: 2 })],
    ['ASSERT_SCRIPT_SYMBOL_NO_RUNTIME_OR_COMPLETENESS', () => assertBounded({ ...valid, runtime_execution: true })],
    ['ASSERT_SCRIPT_SYMBOL_FAILURE_DISTINCT', () => assertFailure({ ...valid, coverage: { status: 'UNKNOWN' }, observations: [] })],
    ['ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS', () => {
      createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions });
      createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions: { ...versions, typescript: '5.9.1' } });
    }],
  ];
  for (const [name, run] of cases) {
    let failure = '';
    try { run(); } catch (error) { failure = error.message.split('\n')[0]; }
    assert.ok(failure.includes(name), `${name} perturbation did not fire: ${failure}`);
    console.log(`WITNESS ${name}: FAIL=${failure}; PASS=accepted valid contrast`);
  }
}
