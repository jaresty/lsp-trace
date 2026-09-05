#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const repo = resolve(root, '../..');
const specificationPath = join(root, 'target-b05-seeds.v1.json');
const evidencePath = join(repo, 'qualification/retained/target-b05-synthetic/qualification-evidence.v1.json');
const originalSpecification = readFileSync(specificationPath);
const originalEvidence = readFileSync(evidencePath);
const perturbations = [
  ['P 17', (specification) => { specification.source.files.pop(); }],
  ['P 18', (specification) => { specification.premise_ledger.rejected[0] = 'accepted-by-name'; }],
  ['P 19', (specification) => { specification.seeds[0].source_evidence.initializer = 'uploadComplete = task(async () => { ... })'; }],
  ['P 20', (specification) => { specification.seeds[1].generated_call_range.start.line = 8; }],
  ['P 21', (_specification, evidence) => { evidence.qualifier.operation_count = 7; }],
  ['P 22', (_specification, evidence) => { evidence.admission.PROGRAM_B_ADMITTED = true; }],
];
const run = () => spawnSync(process.execPath, ['--test', 'qualification/source-constrained-synthetic/retained-properties-17-22.guard.test.mjs'], { cwd: repo, encoding: 'utf8' });

try {
  for (const [property, mutate] of perturbations) {
    const specification = JSON.parse(originalSpecification);
    const evidence = JSON.parse(originalEvidence);
    mutate(specification, evidence);
    writeFileSync(specificationPath, `${JSON.stringify(specification, null, 2)}\n`);
    writeFileSync(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`);
    const bad = run();
    const output = `${bad.stdout}${bad.stderr}`;
    const passes = (output.match(/Assertion \[P \d+\] [^\n]+ PASS:/g) || []).length;
    if (bad.status === 0 || !output.includes(`Assertion [${property}]`) || passes !== 5) throw new Error(`${property} isolation failed status=${bad.status} other-pass=${passes}\n${output}`);
    writeFileSync(specificationPath, originalSpecification);
    writeFileSync(evidencePath, originalEvidence);
    const good = run();
    const restored = `${good.stdout}${good.stderr}`;
    if (good.status !== 0 || !restored.includes(`Assertion [${property}]`)) throw new Error(`${property} restoration failed\n${restored}`);
    console.log(`A-FAIL Assertion [${property}] RED other-pass=5`);
    console.log(`A-PASS Assertion [${property}] PASS suite-pass=6`);
  }
} finally {
  writeFileSync(specificationPath, originalSpecification);
  writeFileSync(evidencePath, originalEvidence);
}
console.log('P17_P22_PERTURBATIONS_COMPLETE assertions=6 isolated=6 restored=true');
