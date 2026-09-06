import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import * as tsModule from 'typescript';

import { createScriptSymbolExtractor } from '../analyzers/script.mjs';

const require = createRequire(import.meta.url);
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const dependencies = { ts, versions: { typescript: require('typescript/package.json').version } };

function analyze(source) {
  return createScriptSymbolExtractor(dependencies).extract({ documents: [{ uri: 'file:///workspace/fixture.ts', language: 'typescript', source }] }).observations;
}

const taskPositive = `
class Task<T> { readonly __lspTraceType!: 'ember-concurrency:Task'; perform(): T { throw new Error(); } }
class Upload { upload!: Task<string>; start() { this.upload.perform(); } }
`;
const taskNegative = `
class Runner { perform(): string { return 'ordinary'; } }
class Report { runner = new Runner(); start() { this.runner.perform(); } }
`;
const reloadPositive = `
class Model { readonly __lspTraceType!: '@ember-data/model:Model'; reload(): this { return this; } }
class Person extends Model {}
function refresh(person: Person) { return person.reload(); }
`;
const reloadNegative = `
class LocalCache { reload(): this { return this; } }
function refresh(cache: LocalCache) { return cache.reload(); }
`;

test('ASSERT_TYPED_SCRIPT_EXTRACTOR_REQUIRES_ONLY_TYPESCRIPT_COMPILER', () => {
  let failure;
  try { createScriptSymbolExtractor(dependencies); } catch (error) { failure = error; }
  assert.equal(failure, undefined, `ASSERT_TYPED_SCRIPT_EXTRACTOR_REQUIRES_ONLY_TYPESCRIPT_COMPILER: ${failure?.message}`);
});

test('ASSERT_INVOKES_TASK_COMPILER_DECLARATION_QUALIFIED', () => {
  assert.equal(analyze(taskPositive).filter((o) => o.kind === 'INVOKES_TASK').length, 1);
  assert.deepEqual(analyze(taskNegative).filter((o) => o.kind === 'INVOKES_TASK'), []);
});

test('ASSERT_TRIGGERS_RELOAD_COMPILER_DECLARATION_QUALIFIED', () => {
  assert.equal(analyze(reloadPositive).filter((o) => o.kind === 'TRIGGERS_RELOAD').length, 1);
  assert.deepEqual(analyze(reloadNegative).filter((o) => o.kind === 'TRIGGERS_RELOAD'), []);
});

test('ASSERT_TYPED_SCRIPT_RELATIONS_DETERMINISTIC_AND_SOURCE_MAPPED', () => {
  const first = analyze(`${taskPositive}\n${reloadPositive}`);
  const second = analyze(`${taskPositive}\n${reloadPositive}`);
  assert.deepEqual(first, second);
  for (const observation of first.filter((o) => ['INVOKES_TASK', 'TRIGGERS_RELOAD'].includes(o.kind))) {
    assert.equal(observation.original_anchor.uri, 'file:///workspace/fixture.ts');
    assert.ok(observation.original_anchor.bytes.end > observation.original_anchor.bytes.start);
  }
});
