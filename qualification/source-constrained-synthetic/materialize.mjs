#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const sourceRepo = process.argv[2];
const packageDir = process.argv[3];
if (!sourceRepo || !packageDir) {
  console.error('usage: node materialize.mjs <market-view-ui-git-dir> <npm-pack-dir>');
  process.exit(2);
}

const commit = '326718ae733cb26097bd30246276cecd371a4e79';
const files = [
  ['app/services/uploads.js', '8714414be3d70935e55e0fd6dccf1cd867363547', '82f1e0fbd92dda79801b4d521cdb3edc976ae13ab20cb018879a6390fc308205'],
  ['app/models/user-import.js', 'eae121fffc6a82d1111466f71a68fe5f5b8cc400', '37ea85621d5a84356417ee3128df63a0e90a35e8856e46cfc8fc602c5aed7fd9'],
];
const packages = [
  ['ember-concurrency-5.2.0.tgz', 'sha512-NUptPzaxaF2XWqn3VQ5KqiLSRqPFIZhWXH3UkOMhiedmiolxGYjUV96maoHWdd5msxNgQBC0UkZ28m7pV7A0sQ==', 'package/declarations/index.d.ts', 'vendor/packages/ember-concurrency/index.d.ts'],
  ['warp-drive-legacy-5.8.1.tgz', 'sha512-i2+LhOQ+RO4AKeI4mSMWGNsiIY/sVB9CoslPDqCVc9BofcQE7uvAZinIjjPgPU5bbzqEThqdlHiEeoOdd19Tfw==', 'package/declarations/model.d.ts', 'vendor/packages/warp-drive/model.d.ts'],
  ['warp-drive-legacy-5.8.1.tgz', 'sha512-i2+LhOQ+RO4AKeI4mSMWGNsiIY/sVB9CoslPDqCVc9BofcQE7uvAZinIjjPgPU5bbzqEThqdlHiEeoOdd19Tfw==', 'package/declarations/model/-private/model.d.ts', 'vendor/packages/warp-drive/private-model.d.ts'],
];
function digest(algorithm, bytes, encoding = 'hex') {
  return createHash(algorithm).update(bytes).digest(encoding);
}
function git(...args) {
  return execFileSync('git', ['-C', sourceRepo, ...args]);
}
if (git('cat-file', '-t', commit).toString().trim() !== 'commit') throw new Error('source commit unavailable');
for (const [path, blob, sha256] of files) {
  const actualBlob = git('rev-parse', `${commit}:${path}`).toString().trim();
  const bytes = git('show', `${commit}:${path}`);
  if (actualBlob !== blob || digest('sha256', bytes) !== sha256) throw new Error(`source custody mismatch: ${path}`);
  const destination = join(root, 'vendor/market-view-ui', path);
  mkdirSync(dirname(destination), { recursive: true });
  writeFileSync(destination, bytes);
}
for (const [archive, integrity, member, relative] of packages) {
  const bytes = readFileSync(join(packageDir, archive));
  if (`sha512-${digest('sha512', bytes, 'base64')}` !== integrity) throw new Error(`package integrity mismatch: ${archive}`);
  const body = execFileSync('tar', ['-xOf', join(packageDir, archive), member]);
  const destination = join(root, relative);
  mkdirSync(dirname(destination), { recursive: true });
  writeFileSync(destination, body);
}
console.log(`MATERIALIZED ${commit} source=2 package-declarations=3`);
