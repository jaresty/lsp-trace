import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const cases = JSON.parse(readFileSync(new URL('./cases.v1.json', import.meta.url)));
const evidence = JSON.parse(readFileSync(new URL('../retained/caller-project-javascript/evidence.v1.json', import.meta.url)));
const ids = new Set(cases.cases.map(x => x.id));

test('ASSERT_CALLER_JS_EVIDENCE_FAMILY_IDENTITY_AND_COUNTS', () => {
  assert.equal(evidence.schema_version, 'lsp-trace.caller-project-javascript.evidence.v1');
  assert.equal(evidence.provider.identity, 'ember-glint@1');
  assert.equal(evidence.provider.installation, 'independent-npm-pack-offline-install');
  assert.deepEqual(evidence.summary, { case_count: 12, operation_count: 24, replay_attempt_count: 48, pass_count: 48 });
  assert.equal(evidence.attempts.length, 48);
});

test('ASSERT_CALLER_JS_REQUIRED_VARIANTS_AND_EXACT_OUTCOMES', () => {
  for (const id of ['invokes-task-positive','triggers-reload-positive','invokes-task-same-spelling','triggers-reload-same-spelling','invokes-task-any','triggers-reload-unknown','missing-config','malformed-config','missing-task-declaration','wrong-task-declaration','missing-reload-declaration','wrong-reload-declaration']) assert.ok(ids.has(id), id);
  for (const item of cases.cases) for (const operation of cases.operations) {
    const records=evidence.attempts.filter(x=>x.case===item.id&&x.operation===operation);
    assert.equal(records.length,2,`${item.id}/${operation}`);
    for(const record of records){assert.equal(record.status,'PASS',record.id);assert.deepEqual(record.expected,{outcome:item.expect,count:item.count??0,code:item.code??null},record.id);assert.equal(record.observed.outcome,item.expect,record.id);assert.equal(record.observed.count,item.count??0,record.id)}
  }
});

test('ASSERT_CALLER_JS_SCHEMAS_PARITY_REPLAY_PROVIDER_AND_CUSTODY', () => {
  for(const record of evidence.attempts){assert.equal(record.parity,true,record.id);assert.equal(record.provider_identity,'ember-glint@1',record.id);assert.match(record.digests.request,/^sha256:[0-9a-f]{64}$/);assert.match(record.digests.response,/^sha256:[0-9a-f]{64}$/);assert.match(record.digests.transcript,/^sha256:[0-9a-f]{64}$/);if(record.observed.outcome==='GRAPH'){assert.equal(record.schemas.graph,'lsp-trace.graph.v4',record.id);assert.equal(record.original_custody,true,record.id);assert.equal(record.declaration_custody,true,record.id)}else assert.match(record.schemas.domain,/envelope-domain-error\.v1\.schema\.json$/,record.id)}
  for(const item of cases.cases)for(const operation of cases.operations){const [a,b]=evidence.attempts.filter(x=>x.case===item.id&&x.operation===operation);assert.equal(a.digests.response,b.digests.response,`${item.id}/${operation}`)}
});

test('ASSERT_CALLER_JS_RETAINED_RECORD_HAS_TIME_AND_BINARY_DIGESTS',()=>{assert.match(evidence.generated_at,/^\d{4}-\d\d-\d\dT/);for(const value of [evidence.provider.package_digest,evidence.provider.executable_digest,evidence.transport.mcp_digest])assert.match(value,/^sha256:[0-9a-f]{64}$/)});
