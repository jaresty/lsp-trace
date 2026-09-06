import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';

import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const commit = '5567ec8e28dcea18067e23b32e4ff238fdffd218';

function requestFor(path) {
  return { documents: [{ uri: pathToFileURL(path).href, revision: commit }], relation_kinds: ['TRIGGERS_RELOAD'] };
}

async function analyze(source) {
  const directory = await mkdtemp(join(tmpdir(), 'triggers-reload-'));
  const path = join(directory, 'seed.ts');
  await writeFile(path, source);
  try {
    return await analyzeSourceConstrainedTypeScript(requestFor(path));
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

const directPersonReload = `
import Model from '@ember-data/model';
class Person extends Model {}
export function refresh(person: Person) { return person.reload(); }
`;

const compilerBackedNegatives = {
  'local same-spelling': `class LocalCache { reload() { return this; } }\nnew LocalCache().reload();`,
  'name-only': `const reload = () => undefined; reload();`,
  'wrong declaration parent': `class WrongParent { reload() { return this; } }\ndeclare const value: WrongParent; value.reload();`,
  'wrong declaration path': `class Model { reload() { return this; } }\ndeclare const value: Model; value.reload();`,
};

test('ASSERT_TRIGGERS_RELOAD_COMPILER_ACCEPTS_EXACT_PINNED_MODEL_RELOAD', async () => {
  const result = await analyze(directPersonReload);
  assert.equal(result.outcome, 'COMPLETE');
  assert.equal(result.observations.length, 1);
  const observation = result.observations[0];
  assert.equal(observation.kind, 'TRIGGERS_RELOAD');
  assert.match(observation.to.node_id, /path=vendor\/warp-drive\/private-model\.d\.ts;symbol=Model\.reload;sha256=2f02d0909d25249d0b404015762c6c593e51dfa3ff7993d0f0a6f07be15dd37b;/);
  assert.equal(observation.original_anchor.document_id, 'original');
  assert.equal(observation.original_anchor.uri.startsWith('file:'), true);
  assert.equal(observation.original_anchor.revision, commit);
});

test('ASSERT_TRIGGERS_RELOAD_COMPILER_REJECTS_NON_IDENTITIES', async () => {
  for (const [name, source] of Object.entries(compilerBackedNegatives)) {
    const result = await analyze(source);
    assert.equal(result.outcome, 'EMPTY', name);
    assert.deepEqual(result.observations, [], name);
  }
});

test('ASSERT_TRIGGERS_RELOAD_REJECTS_ANY_UNKNOWN_AND_UNRESOLVED_CHECKER_INPUT', async () => {
  const anyResult = await analyze(`declare const value: any; value.reload();`);
  assert.equal(anyResult.outcome, 'BLOCKED');
  assert.match(anyResult.blocker, /unsafe compiler identity:/);
  for (const [source, diagnostic] of [[`declare const value: unknown; value.reload();`, 'TS18046'], [`missing.reload();`, 'TS2304']]) {
    const result = await analyze(source);
    assert.equal(result.outcome, 'BLOCKED');
    assert.equal(result.coverage.status, 'UNAVAILABLE');
    assert.match(result.blocker, new RegExp(`bounded checker failure: ${diagnostic}`));
  }
});

test('ASSERT_TRIGGERS_RELOAD_IDS_ANCHORS_AND_FRAMING_ARE_DETERMINISTIC', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'triggers-reload-deterministic-'));
  const path = join(directory, 'seed.ts');
  await writeFile(path, directPersonReload);
  try {
    const first = await analyzeSourceConstrainedTypeScript(requestFor(path));
    const second = await analyzeSourceConstrainedTypeScript(requestFor(path));
    assert.deepEqual(first, second);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
