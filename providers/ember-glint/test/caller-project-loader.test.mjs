import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, rename, symlink, unlink, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const providerRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const projectRoot = join(providerRoot, 'test-projects/caller-js');
const seedPath = join(projectRoot, 'src/seed.js');
const commit = '04b683e784ba4ed62b182fe7c797bfff5f49d66d';

async function request() {
  const source = await readFile(seedPath);
  return {
    documents: [{ uri: pathToFileURL(seedPath).href, revision: commit, digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source: source.toString() }],
    relation_kinds: ['INVOKES_TASK'],
  };
}

async function assertCallerQualified(assertion) {
  const result = await analyzeSourceConstrainedTypeScript(await request());
  assert.equal(result.outcome, 'COMPLETE', `${assertion} precondition: analysis must execute`);
  assert.equal(result.observations.length, 1, `${assertion} precondition: relation must be admitted`);
  assert.match(result.observations[0].to.node_id, /package=ember-concurrency@99\.0\.0-caller;path=node_modules\/ember-concurrency\/index\.d\.ts(?:;|$)/, `${assertion} expected caller-local declaration and package custody`);
}

async function mutateFile(path, replacement, observe) {
  const original = await readFile(path);
  try {
    await writeFile(path, replacement);
    await observe();
  } finally {
    await writeFile(path, original);
  }
}

async function hideFile(path, observe) {
  const hidden = `${path}.mutation-hidden`;
  await rename(path, hidden);
  try {
    await observe();
  } finally {
    await rename(hidden, path);
  }
}

test('ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT', async () => {
  await assertCallerQualified('ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT');
});

test('ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT', async () => {
  const config = join(projectRoot, 'jsconfig.json');
  await hideFile(config, async () => {
    await assert.rejects(analyzeSourceConstrainedTypeScript(await request()), /project config failure:/, 'ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT A-fail: ignored containing config must fail explicitly rather than qualify through an ancestor config');
  });
  await assertCallerQualified('ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT A-pass');
});

test('ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY', async () => {
  const config = join(projectRoot, 'jsconfig.json');
  await mutateFile(config, '{ malformed', async () => {
    await assert.rejects(analyzeSourceConstrainedTypeScript(await request()), /project config failure:/, 'ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY A-fail');
  });
  await assertCallerQualified('ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY A-pass');
});

test('ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY', async () => {
  const declaration = join(projectRoot, 'node_modules/ember-concurrency/index.d.ts');
  await hideFile(declaration, async () => {
    await assert.rejects(analyzeSourceConstrainedTypeScript(await request()), /bounded checker failure:/, 'ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-fail');
  });
  await assertCallerQualified('ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-pass');
});

test('ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY', async () => {
  const manifest = join(projectRoot, 'node_modules/ember-concurrency/package.json');
  await mutateFile(manifest, '{"name":"confusable-concurrency","version":"99.0.0-caller","type":"module","types":"index.d.ts"}\n', async () => {
    const result = await analyzeSourceConstrainedTypeScript(await request());
    assert.equal(result.outcome, 'EMPTY', 'ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-fail');
    assert.deepEqual(result.observations, [], 'ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-fail');
  });
  await assertCallerQualified('ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-pass');
});

test('ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY', async () => {
  const declaration = join(projectRoot, 'node_modules/ember-concurrency/index.d.ts');
  const hidden = `${declaration}.mutation-hidden`;
  await rename(declaration, hidden);
  try {
    await symlink(join(providerRoot, 'fixtures/source-constrained-synthetic/vendor/ember-concurrency/index.d.ts'), declaration);
    const result = await analyzeSourceConstrainedTypeScript(await request());
    assert.equal(result.outcome, 'EMPTY', 'ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-fail');
    assert.deepEqual(result.observations, [], 'ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-fail');
  } finally {
    await unlink(declaration);
    await rename(hidden, declaration);
  }
  await assertCallerQualified('ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-pass');
});
