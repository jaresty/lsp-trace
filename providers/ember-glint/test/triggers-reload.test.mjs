import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import test from 'node:test';

const root = new URL('../', import.meta.url);
const positive = readFileSync(new URL('fixtures/triggers-reload-positive.ts', root), 'utf8');
const negative = readFileSync(new URL('fixtures/triggers-reload-negative.ts', root), 'utf8');
const packageLock = JSON.parse(readFileSync(new URL('package-lock.json', root), 'utf8'));
const analyzer = readFileSync(new URL('analyzer.mjs', root), 'utf8');
const defaultAnalyzer = readFileSync(new URL('default-analyzer.mjs', root), 'utf8');
const perturb = process.env.TRIGGERS_RELOAD_PERTURB;

const assertions = Object.freeze({
  seedPair: 'ASSERT_TRIGGERS_RELOAD_SEEDS_SHARE_SPELLING_BUT_NOT_TARGET',
  qualifiedIdentity: 'ASSERT_TRIGGERS_RELOAD_REQUIRES_QUALIFIED_EMBER_DATA_TARGET',
  blockedAdvertisement: 'ASSERT_TRIGGERS_RELOAD_BLOCKED_IS_NOT_ADVERTISED_OR_WIRED',
});

function report(assertion, result) {
  console.log(JSON.stringify({ guard: 'providers/ember-glint/test/triggers-reload.test.mjs', assertion, result }));
}

function qualifiesReload({ method, receiverPackage, declarationPackage }) {
  if (perturb === 'name-only') return method === 'reload';
  return method === 'reload'
    && receiverPackage === '@ember-data/model'
    && declarationPackage === '@ember-data/model';
}

test(assertions.seedPair, () => {
  const observedNegative = perturb === 'seed-confusion'
    ? negative.replace('class LocalCache', "import Model from '@ember-data/model';\nclass LocalCache extends Model")
    : negative;
  assert.match(positive, /from '@ember-data\/model'/);
  assert.match(positive, /\.reload\(\)/);
  assert.match(observedNegative, /class LocalCache/);
  assert.doesNotMatch(observedNegative, /from '@ember-data\/model'/);
  assert.match(observedNegative, /\.reload\(\)/);
  report(assertions.seedPair, 'PASS');
});

test(assertions.qualifiedIdentity, () => {
  const nameOnly = { method: 'reload' };
  const emberData = {
    method: 'reload',
    receiverPackage: '@ember-data/model',
    declarationPackage: '@ember-data/model',
  };
  assert.equal(qualifiesReload(nameOnly), false, 'method spelling alone must not qualify');
  assert.equal(qualifiesReload(emberData), true, 'both receiver and declaration package identity are required');
  const rootDependencies = packageLock.packages?.['']?.dependencies ?? {};
  assert.equal(rootDependencies['@ember-data/model'], undefined, 'pinned tooling does not provide qualified Ember Data model declarations');
  report(assertions.qualifiedIdentity, 'PASS');
});

test(assertions.blockedAdvertisement, () => {
  const observedAnalyzer = perturb === 'advertise' ? `${analyzer}\n'TRIGGERS_RELOAD'` : analyzer;
  assert.doesNotMatch(observedAnalyzer, /['"]TRIGGERS_RELOAD['"]/);
  assert.doesNotMatch(defaultAnalyzer, /['"]TRIGGERS_RELOAD['"]/);
  assert.equal(existsSync(new URL('analyzers/triggers-reload.mjs', root)), false);
  report(assertions.blockedAdvertisement, 'PASS');
});
