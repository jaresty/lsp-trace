import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const fixture = JSON.parse(readFileSync(new URL('./testdata/retained-properties-17-22.present-but-wrong.json', import.meta.url), 'utf8'));

function guard(property, id, statement, observe) {
  test(`${property} ${id}`, () => {
    try {
      observe();
      console.log(`Assertion [${property}] ${id} PASS: ${statement}`);
    } catch (error) {
      console.error(`Assertion [${property}] ${id} RED: ${statement} :: ${error.message}`);
      throw error;
    }
  });
}

guard('P 17', 'ASSERT_MARKET_VIEW_EXACT_TWO_DETERMINISTIC_TYPESCRIPT_SEEDS', 'exactly two TypeScript seeds are deterministically generated from Market View commit 326718ae733cb26097bd30246276cecd371a4e79 with JS path/blob/SHA256, original and generated call ranges, and mappings', () => {
  assert.equal(fixture.seeds.source.commit, '326718ae733cb26097bd30246276cecd371a4e79');
  assert.equal(fixture.seeds.items.length, 2);
  for (const seed of fixture.seeds.items) {
    assert.equal(seed.language, 'typescript');
    assert.match(seed.javascript.path, /\.js$/);
    assert.match(seed.javascript.blob, /^[0-9a-f]{40}$/);
    assert.match(seed.javascript.sha256, /^[0-9a-f]{64}$/);
    assert.deepEqual(seed.generation, { deterministic: true, original_call_ranges: seed.original_call_ranges, generated_call_ranges: seed.generated_call_ranges, mappings: seed.mappings });
    assert.ok(seed.original_call_ranges.length > 0);
    assert.equal(seed.original_call_ranges.length, seed.generated_call_ranges.length);
    assert.equal(seed.original_call_ranges.length, seed.mappings.length);
  }
});

guard('P 18', 'ASSERT_INTRODUCED_DECLARATIONS_EXACT_PROVENANCE_OR_REJECTED', 'every introduced import, receiver, collection, and model declaration has exact target-source or lock-matched declaration provenance and unsupported premises are rejected', () => {
  assert.deepEqual(fixture.declarations.kinds, ['import', 'receiver', 'collection', 'model']);
  assert.equal(fixture.declarations.entries.every(entry => entry.provenance === 'exact-target-source' || entry.provenance === 'lock-matched-declaration'), true);
  assert.equal(fixture.declarations.entries.every(entry => entry.source && entry.declaration && /^[0-9a-f]{64}$/.test(entry.sha256)), true);
  assert.deepEqual(fixture.declarations.unsupported_premises, { disposition: 'rejected', accepted: [] });
});

guard('P 19', 'ASSERT_TASK_SEED_EXACT_PERFORM_EVIDENCE_REPLAY_STABLE', 'Task seed preserves exact uploadComplete initializer, import, and .perform evidence and two provider replays have identical logical evidence', () => {
  assert.deepEqual(fixture.task_seed.evidence, {
    initializer: 'uploadComplete = task(async () => { ... })',
    import: "import { task } from 'ember-concurrency'",
    call: 'this.uploadComplete.perform()',
  });
  assert.equal(fixture.task_seed.provider_replays.length, 2);
  assert.deepEqual(fixture.task_seed.provider_replays[0].logical_evidence, fixture.task_seed.provider_replays[1].logical_evidence);
});

guard('P 20', 'ASSERT_RELOAD_SEED_ALL_WRITES_PROVENANCE_REPLAY_STABLE', 'reload seed preserves all uploads collection writes, model provenance, .reload evidence, and two provider replays have identical logical evidence', () => {
  assert.deepEqual(fixture.reload_seed.uploads_collection_writes, fixture.reload_seed.expected_uploads_collection_writes);
  assert.deepEqual(fixture.reload_seed.model_provenance, { model: 'UserImportModel', provenance: 'lock-matched-declaration' });
  assert.equal(fixture.reload_seed.call, 'upload.reload()');
  assert.equal(fixture.reload_seed.provider_replays.length, 2);
  assert.deepEqual(fixture.reload_seed.provider_replays[0].logical_evidence, fixture.reload_seed.provider_replays[1].logical_evidence);
});

guard('P 21', 'ASSERT_QUALIFIER_ISOLATED_PACKED_REAL_STDIO_STRICT_GIT_CUSTODY', 'committed qualifier uses an isolated managed synthetic workspace, independently packed and installed provider, real lsp-trace-mcp stdio, and strict Git custody for both seeds', () => {
  assert.deepEqual(fixture.qualifier, {
    committed: true,
    workspace: 'isolated-managed-synthetic',
    provider: 'independently-packed-and-installed',
    transport: 'real-lsp-trace-mcp-stdio',
    seed_custody: ['strict-git', 'strict-git'],
  });
});

guard('P 22', 'ASSERT_DIRECT_JS_UNSUPPORTED_ONLY_PROVISIONAL_B05', 'direct JavaScript remains explicitly unsupported, only provisional target B05 is true, and PROGRAM_B_ADMITTED is false', () => {
  assert.deepEqual(fixture.admission, {
    direct_javascript: 'UNSUPPORTED',
    provisional_targets_true: ['B05'],
    PROGRAM_B_ADMITTED: false,
  });
});
