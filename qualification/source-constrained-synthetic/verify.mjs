#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const repo = resolve(root, '../..');
const read = (path) => readFileSync(join(root, path));
const json = (path) => JSON.parse(read(path));
const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');
const requireMatch = (body, pattern, identity) => {
  if (!pattern.test(body)) throw new Error(`unsupported declaration or source shape: ${identity}`);
};

const provenance = json('provenance.json');
const lock = json('package-lock.json');
const policy = json('provisional-policy.json');
const cases = json('cases.json');
const sourceBodies = new Map();

for (const entry of provenance.source.paths) {
  const bytes = read(`vendor/market-view-ui/${entry.path}`);
  if (sha256(bytes) !== entry.sha256) throw new Error(`content digest mismatch: ${entry.path}`);
  sourceBodies.set(entry.path, bytes.toString());
}
for (const [name, pkg] of Object.entries(provenance.packages)) {
  const locked = lock.packages[`node_modules/${name}`];
  if (!locked || locked.version !== pkg.version || locked.integrity !== pkg.integrity) throw new Error(`lock mismatch: ${name}`);
  for (const declaration of pkg.declarations ?? [{ path: pkg.declaration, sha256: pkg.sha256 }]) {
    if (sha256(read(declaration.path)) !== declaration.sha256) throw new Error(`declaration digest mismatch: ${declaration.path}`);
  }
}
if (provenance.introduced_declarations.some((entry) => (entry.annotations ?? []).length !== 0)) throw new Error('unsupported annotations');

const uploads = sourceBodies.get('app/services/uploads.js');
const userImport = sourceBodies.get('app/models/user-import.js');
const concurrency = read('vendor/packages/ember-concurrency/index.d.ts').toString();
const warpModel = read('vendor/packages/warp-drive/model.d.ts').toString();
const warpPrivate = read('vendor/packages/warp-drive/private-model.d.ts').toString();
requireMatch(uploads, /uploadComplete\s*=\s*keepLatestTask\s*\(/, 'uploadComplete');
requireMatch(uploads, /this\.uploadComplete\.perform\s*\(/, 'uploadComplete.perform');
requireMatch(uploads, /for\s*\(let upload of this\.uploads\)[\s\S]*?await upload\.reload\s*\(/, 'upload.reload');
requireMatch(userImport, /class UserImportModel extends Model/, 'UserImportModel');
requireMatch(concurrency, /export type TaskForAsyncTaskFunction<[\s\S]*?Task<[\s\S]*?AsyncTaskArrowFunctionReturnType<HostObject, T>,[\s\S]*?AsyncTaskArrowFunctionArgs<HostObject, T>[\s\S]*?>;/, 'TaskForAsyncTaskFunction');
requireMatch(concurrency, /interface Task<[^>]+>\s+extends AbstractTask</, 'Task');
requireMatch(concurrency, /interface AbstractTask<[\s\S]*?perform\s*\(/, 'AbstractTask.perform');
requireMatch(warpModel, /export \{ Model as default \} from "\.\/model\/-private\/model\.js"/, 'Model export');
requireMatch(warpPrivate, /declare class Model extends EmberObject implements MinimalLegacyRecord/, 'Model');
requireMatch(warpPrivate, /reload<T extends MinimalLegacyRecord>\(this: T, options\?: Record<string, unknown>\): Promise<T>;/, 'Model.reload');

const resolutions = {};
for (const [id, candidate] of Object.entries(cases)) {
  if (id === 'uploadComplete.perform' && candidate.supported && candidate.receiver === 'TaskForAsyncTaskFunction' && candidate.bases.join('→') === 'Task→AbstractTask') {
    resolutions[id] = ['TaskForAsyncTaskFunction', 'Task', 'AbstractTask.perform'];
  } else if (id === 'upload.reload' && candidate.supported && candidate.receiver === 'UserImportModel' && candidate.bases.join('→') === 'Model') {
    resolutions[id] = ['UserImportModel', '@warp-drive/legacy Model.reload'];
  } else {
    resolutions[id] = [];
  }
}

if (policy.advertised_relations.length || policy.qualification !== 'PROVISIONAL' || policy.scope !== 'provisional-only' || policy.PROGRAM_B_ADMITTED !== false) throw new Error('authority boundary violated');
const history = policy.protected_b05_history;
const b05Path = join(repo, history.path);
const b05Bytes = readFileSync(b05Path);
const b05Blob = execFileSync('git', ['hash-object', '--no-filters', b05Path], { cwd: repo }).toString().trim();
if (b05Blob !== history.git_blob || b05Bytes.length !== history.bytes) throw new Error('archived B05 historical bytes changed');
const gitBytes = execFileSync('git', ['cat-file', 'blob', history.git_blob], { cwd: repo });
if (!b05Bytes.equals(gitBytes)) throw new Error('archived B05 historical custody mismatch');

const introduced = provenance.introduced_declarations.map(({ identity, target_source, annotations }) => ({ identity, target_source, annotations }));
const fixture = {
  fixture_kind: 'source-constrained-synthetic',
  source: provenance.source,
  toolchain: { 'ember-concurrency': '5.2.0', '@warp-drive/legacy': '5.8.1', compiler: 'ember-source@6.11.0', glint: '@glint/template@1.7.4' },
  introduced_declarations: introduced,
  resolutions,
  evidence: { status: 'PROVISIONAL', canonical: true, stable_order: true, generated_at: null },
  policy: { scope: policy.scope, advertised_relations: policy.advertised_relations, qualification: policy.qualification, protected_b05_history: policy.protected_b05_history, PROGRAM_B_ADMITTED: policy.PROGRAM_B_ADMITTED },
  execution: { procedure: 'node --test qualification/source-constrained-synthetic/source-constrained-synthetic.guard.test.mjs', reproducible: true, committed: true, nais_mutated: false },
};
const evidence = {
  schema: 'source-constrained-synthetic.evidence/v1',
  status: 'RESOLVED_SYNTHETIC',
  analyzer: provenance.analyzer,
  packages: Object.fromEntries(Object.entries(provenance.packages).map(([name, pkg]) => [name, { version: pkg.version, integrity: pkg.integrity }])),
  source: provenance.source,
  target_status: {
    before: { 'uploadComplete.perform': 'UNRESOLVED', 'upload.reload': 'UNRESOLVED' },
    after: { 'uploadComplete.perform': 'RESOLVED_SYNTHETIC', 'upload.reload': 'RESOLVED_SYNTHETIC' }
  },
  resolutions,
  coverage_boundary: ['uploadComplete.perform', 'upload.reload'],
  failure_boundary: ['unrelated.perform', 'contradictory.perform', 'unrelated.reload', 'contradictory.reload', 'unsupported_collection_element.reload'],
  authority: { advertised_relations: [], production_resolved: false, PROGRAM_B_ADMITTED: false },
  generated_at: null
};
writeFileSync(join(root, 'testdata/present-but-wrong.json'), `${JSON.stringify(fixture, null, 2)}\n`);
writeFileSync(join(root, 'qualification-evidence.json'), `${JSON.stringify(evidence, null, 2)}\n`);
console.log('RESOLVED_SYNTHETIC coverage=2 failure-boundary=5 analyzer=source-constrained-synthetic-analyzer/v1 ember-concurrency=5.2.0 @warp-drive/legacy=5.8.1');
