import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const fixture = JSON.parse(readFileSync(new URL('./testdata/retained-properties-9-16.present-but-wrong.json', import.meta.url), 'utf8'));

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

guard('P 9', 'ASSERT_SYNTHETIC_PROVIDER_DISTINCT_EXPLICIT_PROVISIONAL_IDENTITY', 'a distinct explicitly selectable synthetic provider identity and version declare SOURCE_CONSTRAINED_SYNTHETIC and PROVISIONAL_DISCOVERY', () => {
  assert.deepEqual(fixture.provider, {
    identity: 'source-constrained-synthetic-provider',
    version: '1.0.0-provisional',
    selectable: true,
    evidence_class: 'SOURCE_CONSTRAINED_SYNTHETIC',
    discovery_status: 'PROVISIONAL_DISCOVERY',
  });
});

guard('P 10', 'ASSERT_SYNTHETIC_INVOKES_TASK_EXACT_QUALIFIED_CHAIN', 'INVOKES_TASK is observed only for the qualified synthetic Task chain with original anchors, dependency, declaration provenance, and negative boundaries', () => {
  assert.deepEqual(fixture.observations.INVOKES_TASK, {
    chain: ['TaskForAsyncTaskFunction', 'Task', 'AbstractTask.perform'],
    anchors: ['app/services/uploads.js#uploadComplete.perform'],
    dependency: 'ember-concurrency@5.2.0',
    declaration_provenance: ['TaskForAsyncTaskFunction', 'Task', 'AbstractTask.perform'],
    negative_boundaries: ['unrelated.perform', 'contradictory.perform'],
  });
});

guard('P 11', 'ASSERT_SYNTHETIC_TRIGGERS_RELOAD_EXACT_QUALIFIED_CHAIN', 'TRIGGERS_RELOAD is observed only for the qualified synthetic Warp Drive chain with collection and model provenance, anchors, dependencies, and negative boundaries', () => {
  assert.deepEqual(fixture.observations.TRIGGERS_RELOAD, {
    chain: ['UserImportModel', '@warp-drive/legacy Model.reload'],
    collection_provenance: 'upload collection element',
    model_provenance: 'market-view-ui:app/models/user-import.js',
    anchors: ['app/models/user-import.js#upload.reload'],
    dependencies: ['@warp-drive/legacy@5.8.1'],
    negative_boundaries: ['unrelated.reload', 'contradictory.reload', 'unsupported_collection_element.reload'],
  });
});

guard('P 12', 'ASSERT_SYNTHETIC_PROVISIONAL_SELECTION_AUTHORITY_ISOLATION', 'synthetic evidence requires explicit provisional selection and production or auto-authoritative selection cannot consume or advertise it', () => {
  assert.deepEqual(fixture.selection, {
    requires_explicit_provisional: true,
    production_consumes_synthetic: false,
    auto_advertises_synthetic: false,
  });
});

guard('P 13', 'ASSERT_SYNTHETIC_REAL_MCP_TEN_SEED_TWENTY_ATTEMPT_MATRIX', 'an immutable ten-seed and twenty-attempt real incoming plus slice MCP matrix uses an independently installed synthetic provider and is deterministic, schema-valid, and provisionally satisfies all five relation stages', () => {
  assert.deepEqual(fixture.matrix, {
    immutable: true,
    seed_count: 10,
    attempt_count: 20,
    operations: ['incoming', 'slice'],
    provider_installation: 'independent',
    deterministic: true,
    schema_valid: true,
    provisionally_satisfied_relations: ['PASSES_CALLBACK', 'INVOKES_TASK', 'TRIGGERS_RELOAD', 'UPDATES_STATE', 'RENDERS_FROM'],
  });
});

guard('P 14', 'ASSERT_PROGRAM_B_PROVISIONAL_ONLY_B05_V2_UNCHANGED', 'PROGRAM_B_PROVISIONAL_DISCOVERY_ADMITTED is true while PROGRAM_B_ADMITTED is false and B05 v2 bytes remain unchanged', () => {
  assert.deepEqual(fixture.admission, {
    PROGRAM_B_PROVISIONAL_DISCOVERY_ADMITTED: true,
    PROGRAM_B_ADMITTED: false,
    b05_v2_blob: '0bd7501a1fa9307002fd09c10e4bb3843141df0e',
  });
});

guard('P 15', 'ASSERT_SYNTHETIC_ANALYZER_GENERIC_GO_NO_FRAMEWORK_MATCHER', 'synthetic analysis is generic Go only and has no framework matcher', () => {
  assert.deepEqual(fixture.analyzer_scope, {
    languages: ['Go'],
    framework_matchers: [],
  });
});

guard('P 16', 'ASSERT_SYNTHETIC_OMISSION_GRAPH_V3_ZERO_START_PRODUCTION_PARITY', 'omitted relations preserve exact graph-v3 bytes, start no provider, and leave production capabilities and evidence unchanged', () => {
  assert.equal(fixture.omitted_relations.graph_v3_bytes, fixture.omitted_relations.baseline_graph_v3_bytes);
  assert.equal(fixture.omitted_relations.provider_startups, 0);
  assert.equal(fixture.omitted_relations.production_capabilities, fixture.omitted_relations.baseline_production_capabilities);
  assert.equal(fixture.omitted_relations.production_evidence, fixture.omitted_relations.baseline_production_evidence);
});
