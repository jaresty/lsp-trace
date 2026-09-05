#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const repo = resolve(root, '../..');
const fixturePath = join(root, 'testdata/present-but-wrong.json');
const canonical = readFileSync(fixturePath);
const perturbations = [
  ['P 1.1', (x) => { x.source.commit = 'wrong'; }],
  ['P 2.1', (x) => { x.toolchain['ember-concurrency'] = '5.1.0'; }],
  ['P 2.2', (x) => { x.fixture_kind = 'synthetic'; }],
  ['P 3.1', (x) => { x.introduced_declarations[0].target_source = 'guessed'; }],
  ['P 3.2', (x) => { x.introduced_declarations[0].annotations = ['unsupported']; }],
  ['P 4.1', (x) => { x.resolutions['uploadComplete.perform'] = ['Task', 'AbstractTask.perform']; }],
  ['P 4.2', (x) => { x.resolutions['unrelated.perform'] = ['Task', 'AbstractTask.perform']; }],
  ['P 4.3', (x) => { x.resolutions['contradictory.perform'] = ['Task', 'AbstractTask.perform']; }],
  ['P 5.1', (x) => { x.resolutions['upload.reload'] = ['Model.reload']; }],
  ['P 5.2', (x) => { x.resolutions['unrelated.reload'] = ['@warp-drive/legacy Model.reload']; }],
  ['P 5.3', (x) => { x.resolutions['contradictory.reload'] = ['@warp-drive/legacy Model.reload']; }],
  ['P 5.4', (x) => { x.resolutions['unsupported_collection_element.reload'] = ['@warp-drive/legacy Model.reload']; }],
  ['P 6.1', (x) => { x.evidence.generated_at = 'now'; }],
  ['P 7.1', (x) => { x.policy.advertised_relations = ['INVOKES_TASK']; }],
  ['P 7.2', (x) => { x.policy.PROGRAM_B_ADMITTED = true; }],
  ['P 8.1', (x) => { x.execution.reproducible = false; }],
];
const run = () => spawnSync(process.execPath, ['--test', 'qualification/source-constrained-synthetic/source-constrained-synthetic.guard.test.mjs'], { cwd: repo, encoding: 'utf8' });
try {
  for (const [id, mutate] of perturbations) {
    const fixture = JSON.parse(canonical);
    mutate(fixture);
    writeFileSync(fixturePath, `${JSON.stringify(fixture, null, 2)}\n`);
    const failed = run();
    const failureOutput = `${failed.stdout}${failed.stderr}`;
    const red = `Assertion [${id}] RED:`;
    const passCount = (failureOutput.match(/Assertion \[P \d+\.\d+\] PASS:/g) ?? []).length;
    if (failed.status === 0 || !failureOutput.includes(red) || passCount !== 15) throw new Error(`${id} did not isolate: status=${failed.status} pass=${passCount}`);
    writeFileSync(fixturePath, canonical);
    const passed = run();
    const passOutput = `${passed.stdout}${passed.stderr}`;
    const green = `Assertion [${id}] PASS:`;
    if (passed.status !== 0 || !passOutput.includes(green)) throw new Error(`${id} did not restore`);
    console.log(`A-FAIL ${red} other-pass=15`);
    console.log(`A-PASS ${green} suite-pass=16`);
  }
} finally {
  writeFileSync(fixturePath, canonical);
}
console.log('PERTURBATIONS_COMPLETE assertions=16 isolated=16 restored=true');
