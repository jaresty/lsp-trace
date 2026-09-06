import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { createHash } from 'node:crypto';
import { compareRelationMultisets, runCampaign } from './campaign.mjs';
import { validateQualificationReport } from './validate.mjs';
const sha = value => createHash('sha256').update(value).digest('hex');
function git(cwd, ...args) { const r = spawnSync('git', args, { cwd, encoding: 'utf8' }); assert.equal(r.status, 0, r.stderr); return r.stdout.trim(); }
function repository() {
  const root = mkdtempSync(join(tmpdir(), 'overlay-source-'));
  git(root, 'init', '-q'); git(root, 'config', 'user.name', 'test'); git(root, 'config', 'user.email', 'test@example.invalid');
  writeFileSync(join(root, 'value.txt'), 'baseline\n'); git(root, 'add', '.'); git(root, 'commit', '-qm', 'baseline');
  return { root, commit: git(root, 'rev-parse', 'HEAD') };
}
const observer = async ({ stage }) => ({ relations: stage === 'TYPE_OVERLAY' ? [{ identity: { kind: 'R', from: 'a', to: 'c' } }] : [{ identity: { kind: 'R', from: 'a', to: 'b' } }] });
function adapter(environment = {}) {
  return {
    proposeEnvironment: async ({ workspace }) => ({ complete: true, diagnostics: [{ code: 'ENV' }], edits: [{ path: 'value.txt', preimage_sha256: environment.preimage ?? sha(readFileSync(join(workspace, 'value.txt'))), content: 'environment\n', write_origin: 'TEST_ENVIRONMENT' }] }),
    proposeOverlay: async ({ workspace }) => ({ complete: true, diagnostics: [{ code: 'ORIGIN', write_origins_complete: true }], edits: [{ path: 'value.txt', preimage_sha256: sha(readFileSync(join(workspace, 'value.txt'))), content: 'overlay\n', write_origin: 'TEST_OVERLAY' }] }),
  };
}
test('runs fixed independently committed stages and atomically publishes validated qualification', async () => {
  const source = repository(), output = join(mkdtempSync(join(tmpdir(), 'overlay-output-')), 'result.json');
  const report = await runCampaign({ source: source.root, pinnedCommit: source.commit, adapter: adapter(), observe: observer, output, validate: validateQualificationReport });
  assert.deepEqual(report.stages.map(stage => stage.name), ['BASELINE', 'ENVIRONMENT_ONLY', 'TYPE_OVERLAY']);
  assert.equal(report.stages[1].predecessor, source.commit); assert.equal(report.stages[2].predecessor, report.stages[1].commit);
  assert.equal(report.authority, 'SOURCE_CONSTRAINED_ANALYSIS_INTERVENTION'); assert.equal(report.purpose, 'QUALIFICATION_ONLY');
  assert.equal(report.admission_mutated, false); assert.equal(report.inventory_mutated, false); assert.equal(report.automatic_continuation, false);
  assert.equal(JSON.parse(readFileSync(output)).schema_version, report.schema_version);
  assert.equal(git(source.root, 'status', '--porcelain'), ''); assert.equal(git(source.root, 'rev-parse', 'HEAD'), source.commit);
});
test('canonical relation comparison detects identity and multiplicity drift', () => {
  assert.equal(compareRelationMultisets([{ identity: { a: 1 } }], [{ identity: { a: 2 } }]).equal, false);
  assert.equal(compareRelationMultisets([{ identity: { a: 1 } }, { identity: { a: 1 } }], [{ identity: { a: 1 } }]).equal, false);
});
for (const [name, badAdapter, pattern] of [
  ['stale preimage', adapter({ preimage: '0'.repeat(64) }), /stale preimage/],
  ['source path escape', { proposeEnvironment: async () => ({ complete: true, diagnostics: [], edits: [{ path: '../escape', preimage_sha256: sha(''), content: 'x', write_origin: 'TEST' }] }) }, /source\/path escape/],
  ['incomplete writes', { proposeEnvironment: async () => ({ complete: false, diagnostics: [], edits: [] }) }, /incomplete write enumeration/],
]) test(`blocks ${name}`, async () => {
  const source = repository();
  await assert.rejects(runCampaign({ source: source.root, pinnedCommit: source.commit, adapter: badAdapter, observe: observer, output: join(tmpdir(), `unused-${process.pid}`), validate: validateQualificationReport }), pattern);
});
test('validation failure preserves caller-selected output', async () => {
  const source = repository(), output = join(mkdtempSync(join(tmpdir(), 'overlay-output-')), 'result.json'); writeFileSync(output, 'old');
  await assert.rejects(runCampaign({ source: source.root, pinnedCommit: source.commit, adapter: adapter(), observe: observer, output, validate: async () => { throw new Error('invalid'); } }), /invalid/);
  assert.equal(readFileSync(output, 'utf8'), 'old');
});
