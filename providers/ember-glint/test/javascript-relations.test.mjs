import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { dirname, join } from 'node:path';
import test from 'node:test';

import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '../fixtures/source-constrained-synthetic');
const commit = '04b683e784ba4ed62b182fe7c797bfff5f49d66d';
const request = (fixture, relation) => ({ documents: [{ uri: pathToFileURL(join(root, fixture)).href, revision: commit }], relation_kinds: [relation] });

for (const [fixture, relation] of [
  ['invokes-task-positive.js', 'INVOKES_TASK'],
  ['triggers-reload-positive.js', 'TRIGGERS_RELOAD'],
]) {
  test(`ASSERT_JAVASCRIPT_JSDOC_POSITIVE_${relation}`, async () => {
    const first = await analyzeSourceConstrainedTypeScript(request(fixture, relation));
    const second = await analyzeSourceConstrainedTypeScript(request(fixture, relation));
    assert.equal(first.outcome, 'COMPLETE');
    assert.equal(first.observations.length, 1);
    assert.equal(first.observations[0].kind, relation);
    assert.deepEqual(first, second);
    assert.equal(first.observations[0].original_anchor.uri, pathToFileURL(join(root, fixture)).href);
  });
}

for (const [fixture, relation] of [
  ['invokes-task-negative.js', 'INVOKES_TASK'],
  ['triggers-reload-negative.js', 'TRIGGERS_RELOAD'],
]) {
  test(`ASSERT_JAVASCRIPT_CONFUSABLE_OR_UNSAFE_EMPTY_${fixture}`, async () => {
    const result = await analyzeSourceConstrainedTypeScript(request(fixture, relation));
    assert.equal(result.outcome, 'EMPTY');
    assert.deepEqual(result.observations, []);
  });
}

test('ASSERT_GJS_SOURCE_CONSTRAINED_RELATIONS_FAIL_CLOSED', async () => {
  const result = await analyzeSourceConstrainedTypeScript({
    documents: [{ uri: pathToFileURL(join(root, 'unsupported.gjs')).href, revision: commit }],
    relation_kinds: ['INVOKES_TASK', 'TRIGGERS_RELOAD'],
  });
  assert.equal(result.outcome, 'UNAVAILABLE');
  assert.equal(result.coverage.status, 'UNKNOWN');
  assert.equal(result.coverage.reason, 'SOURCE_CONSTRAINED_INPUT_NOT_SUPPORTED');
  assert.deepEqual(result.observations, []);
});

for (const [fixture, relation, blocker] of [
  ['invokes-task-any.js', 'INVOKES_TASK', /unsafe compiler identity:/],
  ['invokes-task-unresolved.js', 'INVOKES_TASK', /bounded checker failure: TS2307/],
  ['triggers-reload-unknown.js', 'TRIGGERS_RELOAD', /bounded checker failure: TS18046/],
  ['triggers-reload-unresolved.js', 'TRIGGERS_RELOAD', /bounded checker failure: TS2307/],
]) {
  test(`ASSERT_JAVASCRIPT_UNSAFE_OR_UNRESOLVED_FAILS_CLOSED_${fixture}`, async () => {
    const result = await analyzeSourceConstrainedTypeScript(request(fixture, relation));
    assert.equal(result.outcome, 'BLOCKED');
    assert.equal(result.coverage.status, 'UNAVAILABLE');
    assert.match(result.blocker, blocker);
  });
}
