import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

export const candidates = ['ember-template-compiler', 'glint', 'tree-sitter', 'typescript-service'];
export const prohibited = [
  'callback_invocation_from_passage',
  'feature_identity',
  'repaint',
  'runtime_execution',
  'whole_source_completeness',
];

export function validate(report) {
  assert.equal(report.schema_version, 'lsp-trace.provider-qualification.v1', 'P3 deterministic schema');
  assert.deepEqual(report.candidates.map((entry) => entry.id), candidates, 'P1 exact candidate set');
  for (const entry of report.candidates) {
    assert.ok(['PASS', 'BLOCKED', 'SCOPED_ROLE'].includes(entry.outcome), `P1 ${entry.id} closed outcome`);
    assert.ok(entry.operation && entry.observation, `P1 ${entry.id} executed observation`);
    assert.ok(entry.version, `P1 ${entry.id} provider version`);
    assert.deepEqual(entry.prohibited_claims, prohibited, `P2 ${entry.id} prohibited claims`);
    assert.ok(!entry.supported_claims.some((claim) => prohibited.includes(claim)), `P2 ${entry.id} support boundary`);
  }
  assert.equal(JSON.stringify(report).includes(process.cwd()), false, 'P3 no volatile repository path');
  assert.equal(Object.hasOwn(report, 'timestamp'), false, 'P3 no volatile timestamp');
}

const valid = {
  schema_version: 'lsp-trace.provider-qualification.v1',
  candidates: candidates.map((id) => ({
    id,
    outcome: 'SCOPED_ROLE',
    operation: 'synthetic observation procedure',
    observation: 'candidate operation completed',
    version: '0.0.0',
    supported_claims: ['bounded_source_structure'],
    prohibited_claims: prohibited,
  })),
};

if (process.argv.includes('--witness')) {
  const checks = [
    ['P1 exact candidate set', (r) => r.candidates.pop()],
    ['P1 glint closed outcome', (r) => { r.candidates[1].outcome = 'UNKNOWN'; }],
    ['P1 tree-sitter executed observation', (r) => { r.candidates[2].observation = ''; }],
    ['P2 typescript-service prohibited claims', (r) => { r.candidates[3].prohibited_claims = []; }],
    ['P2 ember-template-compiler support boundary', (r) => { r.candidates[0].supported_claims = ['runtime_execution']; }],
    ['P3 no volatile timestamp', (r) => { r.timestamp = 'volatile'; }],
  ];
  for (const [assertion, perturb] of checks) {
    const wrong = structuredClone(valid);
    perturb(wrong);
    let failure = '';
    try { validate(wrong); } catch (error) { failure = error.message.split('\n')[0]; }
    assert.ok(failure.includes(assertion), `${assertion} perturbation did not fire`);
    validate(valid);
    console.log(`WITNESS ${assertion}: FAIL=${failure}; PASS=accepted valid contrast`);
  }
} else {
  test('retained provider qualification report satisfies the closed contract', async () => {
    const report = JSON.parse(await readFile(new URL('../retained/provider-qualification/report.json', import.meta.url), 'utf8'));
    validate(report);
  });
}
