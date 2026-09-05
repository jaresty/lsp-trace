import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import test from 'node:test';

const fixture = JSON.parse(readFileSync(new URL('./testdata/retained-properties-9-16.present-but-wrong.json', import.meta.url), 'utf8'));
const matrix = JSON.parse(readFileSync(new URL('../retained/source-constrained-synthetic/provisional-matrix.v1.json', import.meta.url), 'utf8'));
const sha256 = value => createHash('sha256').update(typeof value === 'string' || Buffer.isBuffer(value) ? value : JSON.stringify(value)).digest('hex');
const digest = value => `sha256:${sha256(value)}`;
const canonical = value => {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonical(value[key])}`).join(',')}}`;
  return JSON.stringify(value);
};
const replayValue = response => {
  const copy = structuredClone(response);
  copy.id = 0;
  if (copy.result?.structuredContent?.request_id) copy.result.structuredContent.request_id = 'replay-normalized';
  if (copy.result?.content?.[0]?.text) {
    const text = JSON.parse(copy.result.content[0].text);
    if (text.request_id) text.request_id = 'replay-normalized';
    copy.result.content[0].text = JSON.stringify(text);
  }
  return copy;
};

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
    version: '1.0.0',
    selectable: true,
    evidence_class: 'SOURCE_CONSTRAINED_SYNTHETIC',
    discovery_status: 'PROVISIONAL_DISCOVERY',
  });
});

guard('P 10', 'ASSERT_SYNTHETIC_INVOKES_TASK_EXACT_QUALIFIED_CHAIN', 'INVOKES_TASK is observed only for the qualified synthetic Task chain with original anchors, dependency, declaration provenance, and negative boundaries', () => {
  assert.deepEqual(fixture.observations.INVOKES_TASK, {
    chain: ['TaskForAsyncTaskFunction', 'Task', 'AbstractTask.perform'],
    anchor: 'exact TypeScript call expression',
    receiver: 'TaskForAsyncTaskFunction',
    member_declaration: 'ember-concurrency@5.2.0:vendor/ember-concurrency/index.d.ts#AbstractTask.perform',
    declaration_sha256: '1ca4672f7e39b0e3c16f099b501d3b8000ece297764398634770a56c15ce115a',
    evidence_basis: 'typescript-checker',
    authority_ceiling: 'non-authoritative',
    negative_boundaries: ['marker-only', 'same-spelling unrelated method', 'any receiver', 'unknown receiver', 'contradictory receiver', 'unresolved declaration'],
  });
});

guard('P 11', 'ASSERT_SYNTHETIC_TRIGGERS_RELOAD_EXACT_QUALIFIED_CHAIN', 'TRIGGERS_RELOAD is observed only for the qualified synthetic Warp Drive chain with collection and model provenance, anchors, dependencies, and negative boundaries', () => {
  assert.deepEqual(fixture.observations.TRIGGERS_RELOAD, {
    receiver: 'UserImportModel',
    member_declaration: '@warp-drive/legacy@5.8.1:vendor/warp-drive/private-model.d.ts#Model.reload',
    declaration_sha256: '2f02d0909d25249d0b404015762c6c593e51dfa3ff7993d0f0a6f07be15dd37b',
    collection_provenance: 'source-declared uploads:UserImportModel[] element',
    anchor: 'exact TypeScript call expression',
    evidence_basis: 'typescript-checker',
    authority_ceiling: 'non-authoritative',
    negative_boundaries: ['marker-only', 'same-spelling unrelated method', 'any receiver', 'unknown receiver', 'contradictory receiver', 'unsupported collection element', 'unresolved declaration'],
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
  const relations = ['PASSES_CALLBACK', 'INVOKES_TASK', 'TRIGGERS_RELOAD', 'UPDATES_STATE', 'RENDERS_FROM'];
  assert.equal(matrix.schema, 'lsp-trace.source-constrained-synthetic.provisional-matrix.v1');
  assert.equal(matrix.seed_count, 10);
  assert.equal(matrix.attempt_count, 20);
  assert.deepEqual(matrix.operations, ['incoming', 'slice']);
  assert.equal(matrix.attempts.length, 20);
  assert.equal(matrix.PROGRAM_B_ADMITTED, false);
  const expected = new Set(relations.flatMap(relation => ['positive', 'negative'].flatMap(polarity => ['incoming', 'slice'].map(operation => `${relation}:${polarity}:${operation}`))));
  for (const attempt of matrix.attempts) {
    const key = `${attempt.relation}:${attempt.polarity}:${attempt.operation}`;
    assert.ok(expected.delete(key), `duplicate or unexpected attempt ${key}`);
    assert.equal(attempt.status, 'PASS', key);
    assert.equal(attempt.transport.kind, 'real-child-process-stdio', key);
    assert.equal(attempt.transport.executable, matrix.invocation.executable, key);
    assert.deepEqual(attempt.transport.argv, matrix.invocation.argv, key);
    assert.equal(attempt.transport.pid_kind, 'os-child-pid', key);
    assert.match(String(attempt.transport.pid), /^[1-9]\d*$/, key);
    assert.equal(matrix.invocation.identity, 'production-lsp-trace-mcp');
    assert.ok(isAbsolute(matrix.invocation.executable));
    assert.match(matrix.invocation.executable_digest, /^sha256:[0-9a-f]{64}$/);
    assert.equal(matrix.provider.identity, 'source-constrained-synthetic-provider@1.0.0');
    assert.equal(matrix.provider.path_kind, 'absolute-external-npm-install');
    assert.ok(isAbsolute(matrix.provider.executable));
    assert.ok(!matrix.provider.executable.startsWith(matrix.repository_root + '/'));
    assert.ok(isAbsolute(matrix.provider.package_tarball));
    assert.deepEqual(matrix.provider.pack_command, ['npm', 'pack', '--pack-destination', '/tmp/lsp-trace-source-constrained-synthetic-qualification/dist']);
    assert.deepEqual(matrix.provider.install_command.slice(0, 4), ['npm', 'install', '--ignore-scripts', '--prefix']);
    assert.equal(matrix.provider.pack_exit_code, 0);
    assert.equal(matrix.provider.install_exit_code, 0);
    assert.match(matrix.provider.package_digest, /^sha256:[0-9a-f]{64}$/);
    assert.match(matrix.provider.executable_digest, /^sha256:[0-9a-f]{64}$/);
    assert.equal(attempt.provider.executable, matrix.provider.executable, key);
    assert.equal(attempt.provider.executable_digest, matrix.provider.executable_digest, key);
    assert.equal(attempt.provider.package_digest, matrix.provider.package_digest, key);
    assert.equal(attempt.custody.kind, 'git');
    assert.equal(attempt.custody.custody, 'PROVIDER_PROVED');
    assert.match(attempt.custody.commit, /^[0-9a-f]{40}$/);
    assert.equal(attempt.custody.clean, true);
    assert.equal(attempt.request_digest, digest(canonical(attempt.request)), key);
    assert.equal(attempt.response_digest, digest(canonical(replayValue(attempt.response))), key);
    assert.equal(attempt.transcript_digest, digest(attempt.transcript), key);
    assert.equal(attempt.content_structured_equivalent, true, key);
    assert.deepEqual(JSON.parse(attempt.response.result.content[0].text), attempt.response.result.structuredContent, key);
    assert.ok(attempt.validation === 'graph-v4' || attempt.validation === 'explicit-domain-envelope', key);
    if (attempt.validation === 'graph-v4') {
      const artifact = JSON.parse(attempt.response.result.structuredContent.content);
      assert.equal(artifact.schema_version, 'lsp-trace.graph.v4', key);
      assert.equal(artifact.provenance.provider_id, matrix.provider.identity, key);
      assert.equal(artifact.provenance.custody.documents.length, 1, key);
      assert.equal(artifact.provenance.custody.documents[0].revision.custody, 'PROVIDER_PROVED', key);
      assert.equal(artifact.provenance.custody.documents[0].revision.value, attempt.custody.commit, key);
    }
    assert.equal(attempt.replay_digest, attempt.response_digest, key);
    assert.equal(attempt.replay_response_digest, attempt.response_digest, key);
    assert.equal(attempt.observed_relation, attempt.polarity === 'positive', key);
    assert.equal(attempt.semantic_provenance_valid, true, key);
    assert.deepEqual(attempt.request.params.arguments.languages, ['typescript'], key);
    assert.deepEqual(attempt.request.params.arguments.providers, [matrix.provider.identity], key);
    if (attempt.polarity === 'positive' && ['INVOKES_TASK', 'TRIGGERS_RELOAD'].includes(attempt.relation)) {
      const artifact = JSON.parse(attempt.response.result.structuredContent.content);
      const relation = artifact.relations.find(item => item.kind === attempt.relation);
      assert.equal(relation.anchors.length, 1, key);
      assert.equal(relation.anchors[0].uri, attempt.request.params.arguments.uri, key);
      assert.match(relation.from, /evidence=typescript-checker|receiver=/, key);
      assert.match(relation.to, /evidence=typescript-checker/, key);
      assert.match(relation.to, /authority=non-authoritative/, key);
      assert.match(relation.to, /sha256=[0-9a-f]{64}/, key);
      if (attempt.relation === 'INVOKES_TASK') {
        assert.match(relation.from, /receiver=TaskForAsyncTaskFunction/, key);
        assert.match(relation.to, /package=ember-concurrency@5\.2\.0/, key);
        assert.match(relation.to, /symbol=AbstractTask\.perform/, key);
      } else {
        assert.match(relation.from, /receiver=UserImportModel/, key);
        assert.match(relation.from, /collection=uploads:UserImportModel\[\]/, key);
        assert.match(relation.to, /package=@warp-drive\/legacy@5\.8\.1/, key);
        assert.match(relation.to, /symbol=Model\.reload/, key);
      }
    }
  }
  assert.equal(expected.size, 0, `missing attempts: ${[...expected].join(',')}`);
  assert.deepEqual(matrix.relation_stages, Object.fromEntries(relations.map(relation => [relation, { positive: 2, negative: 2, status: 'PASS' }])));
  assert.equal(matrix.PROGRAM_B_PROVISIONAL_DISCOVERY_ADMITTED, true);
});

guard('P 14', 'ASSERT_PROGRAM_B_PROVISIONAL_ONLY_B05_V2_UNCHANGED', 'PROGRAM_B_PROVISIONAL_DISCOVERY_ADMITTED is true while PROGRAM_B_ADMITTED is false and B05 v2 bytes remain unchanged', () => {
  assert.deepEqual(fixture.admission, {
    PROGRAM_B_PROVISIONAL_DISCOVERY_ADMITTED: true,
    PROGRAM_B_ADMITTED: false,
    b05_v2_blob: '0bd7501a1fa9307002fd09c10e4bb3843141df0e',
  });
});

guard('P 15', 'ASSERT_SYNTHETIC_ANALYZER_TYPESCRIPT_CHECKER_CORE_GO_GENERIC', 'synthetic analysis uses the TypeScript checker while core Go has no framework parser', () => {
  assert.deepEqual(fixture.analyzer_scope, {
    languages: ['TypeScript'],
    framework_matchers: [],
    core_go_framework_parsing: false,
  });
});

guard('P 16', 'ASSERT_SYNTHETIC_OMISSION_GRAPH_V3_ZERO_START_PRODUCTION_PARITY', 'omitted relations preserve exact graph-v3 bytes, start no provider, and leave production capabilities and evidence unchanged', () => {
  assert.equal(fixture.omitted_relations.graph_v3_bytes, fixture.omitted_relations.baseline_graph_v3_bytes);
  assert.equal(fixture.omitted_relations.provider_startups, 0);
  assert.equal(fixture.omitted_relations.production_capabilities, fixture.omitted_relations.baseline_production_capabilities);
  assert.equal(fixture.omitted_relations.production_evidence, fixture.omitted_relations.baseline_production_evidence);
});
