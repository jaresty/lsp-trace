import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';

import * as tsModule from 'typescript';

import { createPassesCallbackAnalyzer } from '../analyzers/passes-callback.mjs';

const require = createRequire(import.meta.url);
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const versions = Object.freeze({
  typescript: require('typescript/package.json').version,
});
const repository = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
const positivePath = path.join(repository, 'qualification/external-provider/passes-callback-positive.gts');
const negativePath = path.join(repository, 'qualification/external-provider/passes-callback-negative.gts');
const retainedPath = path.join(repository, 'qualification/retained/b05/passes-callback.json');

async function analyzeFixture(file, { map = true, transform = (source) => source } = {}) {
  const source = transform(await readFile(file, 'utf8'));
  const uri = pathToFileURL(file).href;
  const analyzer = createPassesCallbackAnalyzer({ ts, versions });
  return analyzer.extract({
    documents: [{
      document_id: 'original',
      uri,
      revision: '1825a56070d7b76174ebb086d7c464731ce12bb0',
      blob: 'sha256:fixture',
      language: 'glimmer-js',
      source,
      generated: {
        fileName: `${file}.ts`,
        source,
        mapToOriginal: map ? ({ start, end }) => ({ uri, start, end, roundTrip: { start, end } }) : undefined,
      },
    }],
  });
}

async function positive() {
  const result = await analyzeFixture(positivePath);
  assert.equal(result.observations.length, 1, 'ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER');
  return result;
}

test('ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER', async () => {
  const result = await positive();
  assert.equal(result.status, 'SUPPORTED', 'ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER');
  assert.equal(result.observations[0].kind, 'PASSES_CALLBACK', 'ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER');
  assert.deepEqual(result.observations[0].from, {
    node_id: result.observations[0].from.node_id,
    role: 'CALLABLE_REFERENCE',
  }, 'ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER');
});

test('ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR', async () => {
  const { observations: [observation] } = await positive();
  assert.equal(observation.from.role, 'CALLABLE_REFERENCE', 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.equal(observation.to.role, 'CALLBACK_PARAMETER', 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.equal(observation.original_anchor.uri, pathToFileURL(positivePath).href, 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.equal(observation.original_anchor.document_id, 'original', 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.deepEqual(observation.original_anchor.range, {
    start: { line: 10, character: 8 }, end: { line: 10, character: 14 },
  }, 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.match(observation.from.node_id, /^PASSES_CALLBACK:callable-reference:/, 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
  assert.match(observation.to.node_id, /^PASSES_CALLBACK:callable-parameter:/, 'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR');
});

test('ASSERT_PASSES_CALLBACK_EXACT_GLINT_MAPPING_REQUIRED', async () => {
  const result = await analyzeFixture(positivePath, { map: false });
  assert.equal(result.status, 'BLOCKED', 'ASSERT_PASSES_CALLBACK_EXACT_GLINT_MAPPING_REQUIRED');
  assert.equal(result.reason, 'GLINT_EXACT_MAPPING_UNAVAILABLE', 'ASSERT_PASSES_CALLBACK_EXACT_GLINT_MAPPING_REQUIRED');
  assert.deepEqual(result.observations, [], 'ASSERT_PASSES_CALLBACK_EXACT_GLINT_MAPPING_REQUIRED');
});

test('ASSERT_PASSES_CALLBACK_DETERMINISTIC', async () => {
  const first = await analyzeFixture(positivePath);
  const second = await analyzeFixture(positivePath);
  assert.deepEqual(first, second, 'ASSERT_PASSES_CALLBACK_DETERMINISTIC');
});

test('ASSERT_PASSES_CALLBACK_STRICT_OBSERVATION_ENVELOPE', async () => {
  const { observations: [observation] } = await positive();
  assert.deepEqual(Object.keys(observation).sort(), [
    'does_not_support', 'from', 'kind', 'original_anchor', 'supports', 'to',
  ], 'ASSERT_PASSES_CALLBACK_STRICT_OBSERVATION_ENVELOPE');
  assert.deepEqual(Object.keys(observation.from).sort(), ['node_id', 'role'], 'ASSERT_PASSES_CALLBACK_STRICT_OBSERVATION_ENVELOPE');
  assert.deepEqual(Object.keys(observation.to).sort(), ['node_id', 'role'], 'ASSERT_PASSES_CALLBACK_STRICT_OBSERVATION_ENVELOPE');
  assert.deepEqual(Object.keys(observation.original_anchor).sort(), [
    'blob', 'document_id', 'range', 'revision', 'uri',
  ], 'ASSERT_PASSES_CALLBACK_STRICT_OBSERVATION_ENVELOPE');
});

test('ASSERT_PASSES_CALLBACK_SEMANTIC_PERTURBABILITY', async () => {
  const nonCallableArgument = await analyzeFixture(positivePath, {
    transform: (source) => source.replace('function format(value: string): void {', 'const format: string = "same spelling";\nfunction unused(value: string): void {'),
  });
  assert.deepEqual(nonCallableArgument.observations, [], 'ASSERT_PASSES_CALLBACK_SEMANTIC_PERTURBABILITY');

  for (const parameterType of ['any', 'unknown', 'string']) {
    const result = await analyzeFixture(positivePath, {
      transform: (source) => source.replace('callback: Callback', `callback: ${parameterType}`),
    });
    assert.deepEqual(result.observations, [], `ASSERT_PASSES_CALLBACK_SEMANTIC_PERTURBABILITY_${parameterType}`);
  }
});

test('ASSERT_PASSES_CALLBACK_REJECTS_NONCALLABLE', async () => {
  const result = await analyzeFixture(negativePath);
  assert.equal(result.status, 'SUPPORTED', 'ASSERT_PASSES_CALLBACK_REJECTS_NONCALLABLE');
  assert.deepEqual(result.observations, [], 'ASSERT_PASSES_CALLBACK_REJECTS_NONCALLABLE');
  assert.equal(result.coverage.reason, 'qualified_static_callback_passage', 'ASSERT_PASSES_CALLBACK_REJECTS_NONCALLABLE');
});

test('ASSERT_PASSES_CALLBACK_SEMANTIC_CEILING', async () => {
  const result = await positive();
  const observation = result.observations[0];
  assert.deepEqual(observation.supports, ['source_dependency_relation'], 'ASSERT_PASSES_CALLBACK_SEMANTIC_CEILING');
  assert.deepEqual(observation.does_not_support, [
    'callback_invocation',
    'runtime_execution',
    'repaint',
    'feature_identity',
    'whole_source_completeness',
  ], 'ASSERT_PASSES_CALLBACK_SEMANTIC_CEILING');
  assert.equal(JSON.stringify(observation).includes('INVOKES_TASK'), false, 'ASSERT_PASSES_CALLBACK_SEMANTIC_CEILING');
});

test('ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST', async () => {
  const evidence = JSON.parse(await readFile(retainedPath, 'utf8'));
  assert.equal(evidence.schema_version, 'lsp-trace.ember-glint.passes-callback-evidence.v1', 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.equal(evidence.outcome, 'PASS', 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.deepEqual(evidence.admission, {
    stage: 'provider-package-tests',
    assertion: 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS',
    result: 'PASS',
  }, 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.equal(evidence.baseline, '1825a56070d7b76174ebb086d7c464731ce12bb0', 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.deepEqual(evidence.assertions, [
    'ASSERT_PASSES_CALLBACK_TYPED_REFERENCE_TO_CALLABLE_PARAMETER',
    'ASSERT_PASSES_CALLBACK_EXACT_ENDPOINTS_ROLES_AND_ANCHOR',
    'ASSERT_PASSES_CALLBACK_EXACT_GLINT_MAPPING_REQUIRED',
    'ASSERT_PASSES_CALLBACK_DETERMINISTIC',
    'ASSERT_PASSES_CALLBACK_REJECTS_NONCALLABLE',
    'ASSERT_PASSES_CALLBACK_SEMANTIC_CEILING',
    'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST',
  ], 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.deepEqual(evidence.does_not_support, [
    'callback_invocation',
    'runtime_execution',
    'repaint',
    'feature_identity',
    'whole_source_completeness',
  ], 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.match(evidence.red.observed_result, /fail 6/, 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
  assert.match(evidence.green.observed_result, /pass 7[\s\S]*fail 0/, 'ASSERT_PASSES_CALLBACK_RETAINED_EVIDENCE_HONEST');
});
