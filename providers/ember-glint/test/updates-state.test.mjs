import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

import Parser from 'tree-sitter';
import TypeScriptLanguages from 'tree-sitter-typescript';
import * as tsModule from 'typescript';

import { createScriptSymbolExtractor } from '../analyzers/script.mjs';

const require = createRequire(import.meta.url);
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const versions = Object.freeze({
  typescript: require('typescript/package.json').version,
  treeSitter: require('tree-sitter/package.json').version,
  treeSitterTypeScript: require('tree-sitter-typescript/package.json').version,
});
const extractor = createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions });

function fixture(name) {
  const source = readFileSync(path.join(root, 'fixtures', name), 'utf8');
  return { uri: `file:///workspace/${name}`, language: 'typescript', source };
}

function updates(document) {
  return extractor.extract({ documents: [document] }).observations.filter(({ kind }) => kind === 'UPDATES_STATE');
}

function anchor(source, text) {
  const start = Buffer.byteLength(source.slice(0, source.indexOf(text)), 'utf8');
  return { start, end: start + Buffer.byteLength(text, 'utf8'), text };
}

const positive = fixture('updates-state-positive.ts');
const negative = fixture('updates-state-negative.ts');

test('ASSERT_UPDATES_STATE_REQUIRES_TYPESCRIPT_OWNED_CURRENT_CLASS_FIELD', () => {
  assert.equal(updates(positive).length, 2, 'ASSERT_UPDATES_STATE_REQUIRES_TYPESCRIPT_OWNED_CURRENT_CLASS_FIELD');
});

test('ASSERT_UPDATES_STATE_EXACT_STATE_PRODUCER_AND_STATE_VALUE_ENDPOINTS', () => {
  const observations = updates(positive);
  assert.deepEqual(observations.map(({ from, to }) => ({ from, to })), [
    {
      from: { role: 'STATE_PRODUCER', symbol: 'CounterPanel', declaration: anchor(positive.source, 'CounterPanel') },
      to: { role: 'STATE_VALUE', symbol: 'count', declaration: anchor(positive.source, 'count') },
    },
    {
      from: { role: 'STATE_PRODUCER', symbol: 'CounterPanel', declaration: anchor(positive.source, 'CounterPanel') },
      to: { role: 'STATE_VALUE', symbol: 'enabled', declaration: anchor(positive.source, 'enabled') },
    },
  ], 'ASSERT_UPDATES_STATE_EXACT_STATE_PRODUCER_AND_STATE_VALUE_ENDPOINTS');
});

test('ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR', () => {
  assert.deepEqual(updates(positive).map(({ original_anchor }) => ({
    bytes: original_anchor.bytes,
    text: original_anchor.text,
  })), [
    { bytes: { start: anchor(positive.source, 'this.count++').start, end: anchor(positive.source, 'this.count++').end }, text: 'this.count++' },
    { bytes: { start: anchor(positive.source, 'this.enabled = true').start, end: anchor(positive.source, 'this.enabled = true').end }, text: 'this.enabled = true' },
  ], 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR');
});

test('ASSERT_UPDATES_STATE_DETERMINISTIC', () => {
  const first = updates(positive);
  const second = updates(positive);
  assert.equal(first.length, 2, 'ASSERT_UPDATES_STATE_DETERMINISTIC');
  assert.deepEqual(first, second, 'ASSERT_UPDATES_STATE_DETERMINISTIC');
});

test('ASSERT_UPDATES_STATE_REJECTS_LOCAL_AND_UNRESOLVED_THIS_TARGETS', () => {
  assert.equal(updates(positive).length, 2, 'ASSERT_UPDATES_STATE_REJECTS_LOCAL_AND_UNRESOLVED_THIS_TARGETS');
  assert.deepEqual(updates(negative), [], 'ASSERT_UPDATES_STATE_REJECTS_LOCAL_AND_UNRESOLVED_THIS_TARGETS');
});

test('ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS', () => {
  const observations = updates(positive);
  assert.equal(observations.length, 2, 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
  for (const observation of observations) {
    assert.deepEqual(observation.supports, ['source_dependency_relation'], 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
    assert.deepEqual(observation.does_not_support, [
      'runtime_execution',
      'runtime_mutation',
      'whole_source_completeness',
    ], 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
    assert.equal('runtime_mutation_occurred' in observation, false, 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
  }
});

test('ASSERT_UPDATES_STATE_RETAINED_EVIDENCE_EXACT', () => {
  const evidence = JSON.parse(readFileSync(path.join(root, 'fixtures', 'updates-state-retained-evidence.json'), 'utf8'));
  const digest = (source) => createHash('sha256').update(source).digest('hex');
  assert.equal(evidence.seeds.positive.sha256, digest(positive.source), 'ASSERT_UPDATES_STATE_RETAINED_EVIDENCE_EXACT');
  assert.equal(evidence.seeds.negative.sha256, digest(negative.source), 'ASSERT_UPDATES_STATE_RETAINED_EVIDENCE_EXACT');
  assert.deepEqual(evidence.observations, updates(positive), 'ASSERT_UPDATES_STATE_RETAINED_EVIDENCE_EXACT');
});
