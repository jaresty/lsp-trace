import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';

import { createDefaultAnalyzer } from '../default-analyzer.mjs';
import packageManifest from '../package.json' with { type: 'json' };

const root = new URL('../', import.meta.url);
const repository = new URL('../../../', import.meta.url);
const perturb = process.env.PROVIDER_CONVERGENCE_PERTURB;
const relations = [
  'BINDS_ARGUMENT',
  'INVOKES_TASK',
  'PASSES_CALLBACK',
  'RENDERS_FROM',
  'TRIGGERS_RELOAD',
  'UPDATES_STATE',
];
const assertions = Object.freeze({
  package: 'ASSERT_PROVIDER_CONVERGENCE_SINGLE_INSTALLABLE_EXECUTABLE',
  relations: 'ASSERT_PROVIDER_CONVERGENCE_COMPLETE_QUALIFIED_RELATIONS',
  fixtures: 'ASSERT_PROVIDER_CONVERGENCE_SYNTHETIC_PROJECTS_ARE_FIXTURES',
  authority: 'ASSERT_PROVIDER_CONVERGENCE_SYNTHETIC_PROVIDER_NOT_SELECTABLE',
  archives: 'ASSERT_PROVIDER_CONVERGENCE_HISTORICAL_ARCHIVES_RETAINED',
});

function report(assertion) {
  console.log(JSON.stringify({ guard: 'providers/ember-glint/test/provider-convergence.test.mjs', assertion, result: 'PASS' }));
}

test(assertions.package, () => {
  assert.equal(packageManifest.name, '@lsp-trace/ember-glint-provider');
  const bin = perturb === 'second-executable'
    ? { ...packageManifest.bin, synthetic: './bin/synthetic.mjs' }
    : packageManifest.bin;
  assert.deepEqual(bin, { 'ember-glint': './bin/ember-glint.mjs' });
  assert.equal(packageManifest.exports['./source-constrained-typescript'], './analyzers/source-constrained-typescript.mjs');
  report(assertions.package);
});

test(assertions.relations, () => {
  const analyzer = createDefaultAnalyzer();
  const observed = perturb === 'missing-relation'
    ? analyzer.relationKinds.filter((relation) => relation !== 'INVOKES_TASK')
    : analyzer.relationKinds;
  assert.deepEqual([...observed].sort(), relations);
  assert.deepEqual([...analyzer.supportedRelations].sort(), relations);
  report(assertions.relations);
});

test(assertions.fixtures, () => {
  for (const path of [
    'fixtures/source-constrained-synthetic/provenance.json',
    'fixtures/source-constrained-synthetic/tsconfig.json',
    'fixtures/source-constrained-synthetic/vendor/glimmer-component/index.d.ts',
    'fixtures/source-constrained-synthetic/vendor/ember-concurrency/index.d.ts',
    'fixtures/source-constrained-synthetic/vendor/warp-drive/model.d.ts',
    'fixtures/source-constrained-synthetic/vendor/warp-drive/private-model.d.ts',
  ]) assert.equal(perturb === 'missing-fixture' ? false : existsSync(new URL(path, root)), true, path);
  report(assertions.fixtures);
});

test(assertions.authority, () => {
  const retired = new URL('providers/source-constrained-synthetic/', repository);
  assert.equal(perturb === 'selectable-retired-provider' ? true : existsSync(new URL('package.json', retired)), false, 'retired provider must not remain installable');
  assert.equal(existsSync(new URL('bin/provider.mjs', retired)), false, 'retired provider must expose no executable');
  assert.equal(existsSync(new URL('src/provider.mjs', retired)), false, 'retired provider must expose no semantic authority');
  report(assertions.authority);
});

test(assertions.archives, () => {
  const retained = readFileSync(new URL('qualification/retained/source-constrained-synthetic/provisional-matrix.v1.json', repository), 'utf8');
  const matrix = perturb === 'changed-archive' ? '' : retained;
  assert.match(matrix, /source-constrained-synthetic-provider@1\.0\.0/);
  assert.match(matrix, /"immutable": true/);
  report(assertions.archives);
});
