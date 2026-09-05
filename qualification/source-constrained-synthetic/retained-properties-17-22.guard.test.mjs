import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { analyze, IDENTITY, VERSION } from '../../providers/source-constrained-synthetic/src/provider.mjs';

const root = new URL('./', import.meta.url);
const fixture = JSON.parse(readFileSync(new URL('./target-b05-seeds.v1.json', root), 'utf8'));
const commit = '326718ae733cb26097bd30246276cecd371a4e79';
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const requestFor = seed => ({ schema_version: 'lsp-trace.provider-collector-request.v1', provider_id: `${IDENTITY}@${VERSION}`, adapter_id: 'lsp-trace-observation-adapter@1', session: { session_id: 'target-b05-stage-a', generation: 1 }, seed: { uri: new URL(seed.path, root).href }, relations: [seed.relation], languages: ['typescript'], frameworks: ['source-constrained-synthetic'], document_custody: { workspace_revision: { kind: 'git', commit } }, limits: { max_nodes: 100, request_timeout_ms: 30000 } });

function guard(property, id, statement, observe) {
  test(`${property} ${id}`, async () => {
    try { await observe(); console.log(`Assertion [${property}] ${id} PASS: ${statement}`); }
    catch (error) { console.error(`Assertion [${property}] ${id} RED: ${statement} :: ${error.message}`); throw error; }
  });
}

guard('P 17', 'ASSERT_EXACT_TWO_SOURCE_DERIVED_TYPESCRIPT_SEEDS', 'exactly two deterministic TypeScript seeds have exact five-file pinned blob/SHA256/range and call mapping provenance', () => {
  assert.equal(fixture.source.commit, commit);
  assert.equal(fixture.source.files.length, 5);
  assert.equal(new Set(fixture.source.files.map(file => file.path)).size, 5);
  for (const file of fixture.source.files) { assert.match(file.blob, /^[0-9a-f]{40}$/); assert.match(file.sha256, /^[0-9a-f]{64}$/); assert.ok(file.ranges.length > 0); }
  assert.equal(fixture.seeds.length, 2);
  for (const seed of fixture.seeds) { assert.equal(seed.language, 'typescript'); assert.match(seed.path, /^seeds\/.+\.ts$/); assert.ok(seed.original_call_range); assert.ok(seed.generated_call_range); assert.equal(seed.mapping.text.length > 0, true); const source = readFileSync(new URL(seed.path, root), 'utf8'); assert.ok(source.includes(seed.mapping.text)); }
  for (const file of fixture.source.files.filter(file => ['app/services/uploads.js', 'app/models/user-import.js'].includes(file.path))) assert.equal(digest(readFileSync(new URL(`vendor/market-view-ui/${file.path}`, root))), file.sha256);
});

guard('P 18', 'ASSERT_COMPLETE_PREMISE_LEDGER_REJECTS_UNSUPPORTED', 'premise ledger records exact accepted bases and rejects every unsupported receiver, collection, marker, and direct-JS premise', () => {
  assert.deepEqual(fixture.premise_ledger.accepted.map(item => item.basis), ['exact-target-source', 'lock-matched-declaration', 'exact-five-file-source-chain', 'target-source-plus-lock-matched-declaration']);
  assert.deepEqual(fixture.premise_ledger.rejected, ['receiver-name-only', 'any', 'unknown', 'non-UserImport collection element', 'NotTask.perform', 'NotModel.reload', 'marker-only relation', 'direct JavaScript analysis']);
  assert.equal(fixture.premise_ledger.disposition, 'unsupported-premises-rejected');
});

guard('P 19', 'ASSERT_PINNED_KEEPLATESTTASK_SOURCE_AND_IN_PROCESS_INVOCATION', 'task seed preserves exact pinned keepLatestTask import/initializer/call and is directly analyzed in-process as INVOKES_TASK', async () => {
  const seed = fixture.seeds[0];
  assert.deepEqual(seed.source_evidence, { import: "import { keepLatestTask } from 'ember-concurrency';", initializer: 'uploadComplete = keepLatestTask(async () => { ... })', call: 'this.uploadComplete.perform()' });
  const result = await analyze(requestFor(seed));
  assert.equal(result.observations.length, 1);
  const observation = result.observations[0];
  assert.equal(observation.kind, 'INVOKES_TASK');
  assert.deepEqual(observation.original_anchor.range, seed.generated_call_range);
  assert.match(observation.from.node_id, /receiver=TaskForAsyncTaskFunction/);
  assert.match(observation.to.node_id, /symbol=AbstractTask\.perform/);
});

guard('P 20', 'ASSERT_EXACT_RELOAD_PROVENANCE_AND_IN_PROCESS_RELATION', 'reload seed maps the exact source call and is directly analyzed in-process with UserImportModel collection and Model.reload provenance', async () => {
  const seed = fixture.seeds[1];
  const result = await analyze(requestFor(seed));
  assert.equal(result.observations.length, 1);
  const observation = result.observations[0];
  assert.equal(observation.kind, 'TRIGGERS_RELOAD');
  assert.deepEqual(observation.original_anchor.range, seed.generated_call_range);
  assert.match(observation.from.node_id, /receiver=UserImportModel/);
  assert.match(observation.from.node_id, /collection=uploads:UserImportModel\[\]/);
  assert.match(observation.to.node_id, /symbol=Model\.reload/);
});

guard('P 21', 'ASSERT_STAGE_A_HAS_NO_EXECUTION_PLACEHOLDERS', 'Stage A contains no qualifier observations, provider replay placeholders, operation counts, or execution claims', () => {
  assert.equal(fixture.stage_a.provider_analysis, 'direct-in-process-both-seeds');
  assert.deepEqual(fixture.stage_a.execution_claims, []);
  assert.equal('operations' in fixture, false);
  assert.equal('qualifier' in fixture, false);
  assert.equal(JSON.stringify(fixture).includes('provider_replays'), false);
});

guard('P 22', 'ASSERT_STAGE_A_DOES_NOT_ADMIT_TARGET_OR_PROGRAM', 'Stage A keeps provisional target B05 and PROGRAM_B admission false and direct JavaScript rejected', () => {
  assert.equal(fixture.stage_a.provisional_target_b05_admitted, false);
  assert.equal(fixture.stage_a.PROGRAM_B_ADMITTED, false);
  assert.ok(fixture.premise_ledger.rejected.includes('direct JavaScript analysis'));
});
