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

function document(name, source) {
  return {
    document_id: 'original',
    uri: `file:///workspace/${name}`,
    language: 'typescript',
    revision: '3338e63278689bc368055bb450357be0cbb3906e',
    blob: createHash('sha256').update(source).digest('hex'),
    source,
  };
}

function fixture(name) {
  return document(name, readFileSync(path.join(root, 'fixtures', name), 'utf8'));
}

function updates(document) {
  return extractor.extract({ documents: [document] }).observations.filter(({ kind }) => kind === 'UPDATES_STATE');
}

const positive = fixture('updates-state-positive.ts');
const negative = fixture('updates-state-negative.ts');

test('ASSERT_UPDATES_STATE_REQUIRES_TYPESCRIPT_OWNED_CURRENT_CLASS_FIELD', () => {
  assert.equal(updates(positive).length, 2, 'ASSERT_UPDATES_STATE_REQUIRES_TYPESCRIPT_OWNED_CURRENT_CLASS_FIELD');
});

test('ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY', () => {
  const observations = updates(positive);
  for (const observation of observations) {
    assert.deepEqual(Object.keys(observation).sort(), ['does_not_support', 'from', 'kind', 'original_anchor', 'supports', 'to'], 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
    assert.deepEqual(Object.keys(observation.from).sort(), ['node_id', 'role'], 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
    assert.deepEqual(Object.keys(observation.to).sort(), ['node_id', 'role'], 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
    assert.equal(observation.from.role, 'STATE_PRODUCER', 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
    assert.equal(observation.to.role, 'STATE_VALUE', 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
    assert.match(observation.from.node_id, /^UPDATES_STATE:state-producer:uri=file:\/\/\/workspace\/updates-state-positive\.ts;bytes=6-18$/, 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
  }
  assert.match(observations[0].to.node_id, /:state-value:.*;bytes=23-28$/, 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
  assert.match(observations[1].to.node_id, /:state-value:.*;bytes=36-43$/, 'ASSERT_UPDATES_STATE_STRICT_ENDPOINTS_PRESERVE_ROLES_AND_DECLARATION_IDENTITY');
});

test('ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY', () => {
  const anchors = updates(positive).map(({ original_anchor }) => original_anchor);
  assert.deepEqual(anchors.map((value) => Object.keys(value).sort()), [
    ['blob', 'document_id', 'range', 'revision', 'uri'],
    ['blob', 'document_id', 'range', 'revision', 'uri'],
  ], 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
  assert.deepEqual(anchors.map(({ range }) => range), [
    { start: { line: 6, character: 4 }, end: { line: 6, character: 16 } },
    { start: { line: 5, character: 4 }, end: { line: 5, character: 23 } },
  ], 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
  for (const value of anchors) {
    assert.equal(value.document_id, positive.document_id, 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
    assert.equal(value.uri, positive.uri, 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
    assert.equal(value.revision, positive.revision, 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
    assert.equal(value.blob, positive.blob, 'ASSERT_UPDATES_STATE_EXACT_WRITE_ANCHOR_AND_CUSTODY');
  }
});

test('ASSERT_UPDATES_STATE_DETERMINISTIC', () => {
  const first = updates(positive);
  const second = updates(positive);
  assert.equal(first.length, 2, 'ASSERT_UPDATES_STATE_DETERMINISTIC');
  assert.deepEqual(first, second, 'ASSERT_UPDATES_STATE_DETERMINISTIC');
});

test('ASSERT_UPDATES_STATE_REJECTS_LOCAL_UNRESOLVED_ANY_UNKNOWN_AND_NAME_ONLY_TARGETS', () => {
  assert.equal(updates(positive).length, 2, 'ASSERT_UPDATES_STATE_REJECTS_LOCAL_UNRESOLVED_ANY_UNKNOWN_AND_NAME_ONLY_TARGETS');
  assert.deepEqual(updates(negative), [], 'ASSERT_UPDATES_STATE_REJECTS_LOCAL_UNRESOLVED_ANY_UNKNOWN_AND_NAME_ONLY_TARGETS');
  const unsafe = document('unsafe.ts', `class State { count = 0; }\nclass Panel { count = 0; apply(anyValue: any, unknownValue: unknown, other: State) { let count = 0; count++; anyValue.count++; (unknownValue as any).count++; other.count++; this.missing++; } }\n`);
  assert.deepEqual(updates(unsafe), [], 'ASSERT_UPDATES_STATE_REJECTS_LOCAL_UNRESOLVED_ANY_UNKNOWN_AND_NAME_ONLY_TARGETS');
});

test('ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY', () => {
  const renamed = document('updates-state-positive.ts', positive.source.replace('CounterPanel', 'RenamedCounterPanel').replace('count = 0', 'total = 0').replace('this.count++', 'this.total++'));
  const baseline = updates(positive);
  const changed = updates(renamed);
  assert.equal(changed.length, 2, 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
  assert.notEqual(changed[0].from.node_id, baseline[0].from.node_id, 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
  assert.notEqual(changed[0].to.node_id, baseline[0].to.node_id, 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
  assert.equal(changed[0].original_anchor.document_id, 'original', 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
  assert.equal(changed[0].original_anchor.revision, renamed.revision, 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
  assert.equal(changed[0].original_anchor.blob, renamed.blob, 'ASSERT_UPDATES_STATE_DECLARATION_PERTURBATION_CHANGES_NODE_IDS_NOT_ANCHOR_CUSTODY');
});

test('ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS', () => {
  const observations = updates(positive);
  assert.equal(observations.length, 2, 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
  for (const observation of observations) {
    assert.deepEqual(observation.supports, ['source_dependency_relation'], 'ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS');
    assert.deepEqual(observation.does_not_support, [
      'render_occurrence',
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
