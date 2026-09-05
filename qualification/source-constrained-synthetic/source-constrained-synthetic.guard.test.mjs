import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const fixtureURL = new URL('./testdata/present-but-wrong.json', import.meta.url);
const fixture = JSON.parse(readFileSync(fixtureURL, 'utf8'));

function guard(id, property, observe) {
  test(id, () => {
    try {
      observe();
      console.log(`Assertion [${id}] PASS: ${property}`);
    } catch (error) {
      console.error(`Assertion [${id}] RED: ${property} :: ${error.message}`);
      throw error;
    }
  });
}

const exactSource = {
  repository: 'market-view-ui.git-object',
  commit: '326718ae733cb26097bd30246276cecd371a4e79',
  paths: [
    {
      path: 'app/services/uploads.js',
      blob: '8714414be3d70935e55e0fd6dccf1cd867363547',
      sha256: '82f1e0fbd92dda79801b4d521cdb3edc976ae13ab20cb018879a6390fc308205',
    },
    {
      path: 'app/models/user-import.js',
      blob: 'eae121fffc6a82d1111466f71a68fe5f5b8cc400',
      sha256: '37ea85621d5a84356417ee3128df63a0e90a35e8856e46cfc8fc602c5aed7fd9',
    },
  ],
  nais_mutated: false,
};

guard('P 1.1', 'custody is the exact immutable Market View Git object with path/blob/content digests and no NAIS mutation', () => {
  assert.deepEqual(fixture.source, exactSource);
});

guard('P 2.1', 'synthetic dependencies and compiler/Glint identities are lock-faithful', () => {
  assert.deepEqual(fixture.toolchain, {
    'ember-concurrency': '5.2.0',
    '@warp-drive/legacy': '5.8.1',
    compiler: 'ember-source@6.11.0',
    glint: '@glint/template@1.7.4',
  });
});

guard('P 2.2', 'the fixture is explicitly labeled synthetic', () => {
  assert.equal(fixture.fixture_kind, 'source-constrained-synthetic');
});

guard('P 3.1', 'introduced declarations carry exact target-source provenance', () => {
  assert.deepEqual(fixture.introduced_declarations.map(({ identity, target_source }) => ({ identity, target_source })), [
    { identity: 'TaskForAsyncTaskFunction', target_source: 'ember-concurrency@5.2.0' },
    { identity: 'Task', target_source: 'ember-concurrency@5.2.0' },
    { identity: 'AbstractTask.perform', target_source: 'ember-concurrency@5.2.0' },
    { identity: 'UserImportModel', target_source: 'market-view-ui:app/models/user-import.js@326718ae733cb26097bd30246276cecd371a4e79' },
    { identity: 'Model.reload', target_source: '@warp-drive/legacy@5.8.1' },
  ]);
});

guard('P 3.2', 'introduced declarations reject unsupported annotations', () => {
  assert.deepEqual(fixture.introduced_declarations.flatMap((entry) => entry.annotations ?? []), []);
});

guard('P 4.1', 'uploadComplete.perform resolves exactly TaskForAsyncTaskFunction→Task→AbstractTask.perform', () => {
  assert.deepEqual(fixture.resolutions['uploadComplete.perform'], ['TaskForAsyncTaskFunction', 'Task', 'AbstractTask.perform']);
});

guard('P 4.2', 'an unrelated perform receiver does not resolve', () => {
  assert.deepEqual(fixture.resolutions['unrelated.perform'], []);
});

guard('P 4.3', 'a contradictory perform receiver does not resolve', () => {
  assert.deepEqual(fixture.resolutions['contradictory.perform'], []);
});

guard('P 5.1', 'upload.reload resolves exactly UserImportModel→@warp-drive/legacy Model.reload', () => {
  assert.deepEqual(fixture.resolutions['upload.reload'], ['UserImportModel', '@warp-drive/legacy Model.reload']);
});

guard('P 5.2', 'an unrelated reload receiver does not resolve', () => {
  assert.deepEqual(fixture.resolutions['unrelated.reload'], []);
});

guard('P 5.3', 'a contradictory reload receiver does not resolve', () => {
  assert.deepEqual(fixture.resolutions['contradictory.reload'], []);
});

guard('P 5.4', 'an unsupported collection element does not resolve reload', () => {
  assert.deepEqual(fixture.resolutions['unsupported_collection_element.reload'], []);
});

guard('P 6.1', 'evidence is deterministic, canonical, and uses a closed status', () => {
  assert.deepEqual(fixture.evidence, {
    status: 'PROVISIONAL',
    canonical: true,
    stable_order: true,
    generated_at: null,
  });
});

guard('P 7.1', 'provisional policy advertises no authoritative synthetic relations or production resolution', () => {
  assert.deepEqual(fixture.policy.advertised_relations, []);
  assert.equal(fixture.policy.scope, 'provisional-only');
  assert.equal(fixture.policy.qualification, 'PROVISIONAL');
});

guard('P 7.2', 'existing B05 v2 semantics remain byte-identical and PROGRAM_B_ADMITTED remains false', () => {
  assert.equal(fixture.policy.b05_v2_blob, '0bd7501a1fa9307002fd09c10e4bb3843141df0e');
  assert.equal(fixture.policy.PROGRAM_B_ADMITTED, false);
});

guard('P 8.1', 'execution is committed, reproducible, and does not mutate NAIS', () => {
  assert.deepEqual(fixture.execution, {
    procedure: 'node --test qualification/source-constrained-synthetic/source-constrained-synthetic.guard.test.mjs',
    reproducible: true,
    committed: true,
    nais_mutated: false,
  });
});
