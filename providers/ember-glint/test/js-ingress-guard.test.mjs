import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';

import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const commit = '04b683e784ba4ed62b182fe7c797bfff5f49d66d';

test('ASSERT_PRESENT_JAVASCRIPT_SEED_REACHES_QUALIFIED_ANALYZER', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'ember-glint-js-ingress-'));
  const seed = join(directory, 'present-but-unsupported.js');
  await writeFile(seed, '/** @type {*} */ let task; task.perform();\n');
  try {
    const result = await analyzeSourceConstrainedTypeScript({
      documents: [{ uri: pathToFileURL(seed).href, revision: commit }],
      relation_kinds: ['INVOKES_TASK'],
    });
    assert.equal(result.outcome, 'EMPTY', 'ASSERT_PRESENT_JAVASCRIPT_SEED_REACHES_QUALIFIED_ANALYZER');
    assert.deepEqual(result.observations, [], 'ASSERT_PRESENT_JAVASCRIPT_SEED_REACHES_QUALIFIED_ANALYZER');
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
