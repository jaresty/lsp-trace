import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
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

test('ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT', async () => {
  const result = await analyzeSourceConstrainedTypeScript(await request());
  assert.equal(result.outcome, 'COMPLETE', 'ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT precondition: analysis must execute');
  assert.equal(result.observations.length, 1, 'ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT precondition: relation must be admitted');
  assert.match(result.observations[0].to.node_id, /path=node_modules\/ember-concurrency\/index\.d\.ts(?:;|$)/, 'ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT expected caller-local declaration');
});
