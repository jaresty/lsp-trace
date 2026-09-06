import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, realpath, rename, rm, symlink, unlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { analyzeSourceConstrainedTypeScript } from '../analyzers/source-constrained-typescript.mjs';

const providerRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const projectRoot = join(providerRoot, 'test-projects/caller-js');
const seedPath = join(projectRoot, 'src/seed.js');
const uncheckedProjectRoot = join(providerRoot, 'test-projects/caller-js-unchecked');
const uncheckedSeedPath = join(uncheckedProjectRoot, 'src/seed.js');
const commit = '04b683e784ba4ed62b182fe7c797bfff5f49d66d';
const omittedResolutionFixture = {
  'jsconfig.json': `${JSON.stringify({ compilerOptions: { target: 'ES2022', experimentalDecorators: true, checkJs: true, allowJs: true, noEmit: true }, include: ['src/**/*.js'] }, null, 2)}\n`,
  'src/positive.js': "import { task } from 'ember-concurrency';\n\nconst one = task(async () => 'one');\nconst two = task(async () => 'two');\nconst three = task(async () => 'three');\nconst four = task(async () => 'four');\nconst five = task(async () => 'five');\nconst six = task(async () => 'six');\none.perform();\ntwo.perform();\nthree.perform();\nfour.perform();\nfive.perform();\nsix.perform();\n",
  'src/same-spelling.js': 'export {};\nconst callerOwnedTask = { perform() {} };\ncallerOwnedTask.perform();\n',
  'src/wrong-parent.js': "import { confusableTask } from 'ember-concurrency';\nconfusableTask().perform();\n",
  'src/unknown.js': 'export {};\n/** @type {unknown} */\nconst receiver = {};\nreceiver.perform();\n',
  'src/unresolved.js': "import { missingTask } from 'missing-concurrency';\nmissingTask().perform();\n",
  'src/unsafe.js': "import Model from '@warp-drive/legacy/model';\n/** @type {any} */ const firstTask = {};\n/** @type {any} */ const secondTask = {};\n/** @type {any} */ const reloadTarget = new Model();\nfirstTask.perform();\nsecondTask.perform();\nreloadTarget.reload();\n",
  'src/reload.js': "import Model from '@warp-drive/legacy/model';\nconst model = new Model();\nmodel.reload();\n",
  'src/reload-origin.js': "import Model from '@warp-drive/legacy/model';\nclass Uploads {\n  uploads = [];\n  add(values) { this.uploads = [...this.uploads, ...values]; }\n  poll() { for (const upload of this.uploads) upload.reload(); }\n}\nconst uploads = new Uploads();\nconst model = new Model();\nconst alias = model;\nuploads.add([alias]);\n",
  'node_modules/ember-concurrency/package.json': `${JSON.stringify({ name: 'ember-concurrency', version: '5.2.0', type: 'module', types: './declarations/index.d.ts', exports: { '.': { types: './declarations/index.d.ts', default: './dist/index.js' } } })}\n`,
  'node_modules/ember-concurrency/declarations/index.d.ts': 'export interface TaskInstance<T> extends Promise<T> {}\nexport interface AbstractTask<Args extends unknown[], T> {\n  perform(...args: Args): T;\n}\nexport type TaskForAsyncTaskFunction<Fn extends (...args: any[]) => Promise<any>> = AbstractTask<Parameters<Fn>, TaskInstance<Awaited<ReturnType<Fn>>>> & { readonly callerProjectBrand: unique symbol };\ninterface WrongParent { perform(): void; }\nexport declare function task<Fn extends (...args: any[]) => Promise<any>>(callback: Fn): TaskForAsyncTaskFunction<Fn>;\nexport declare function confusableTask(): WrongParent;\n',
  'node_modules/@warp-drive/legacy/package.json': `${JSON.stringify({ name: '@warp-drive/legacy', version: '5.8.1', type: 'module', exports: { './model': { types: './declarations/model.d.ts', default: './dist/model.js' } } })}\n`,
  'node_modules/@warp-drive/legacy/declarations/model.d.ts': 'export default class Model { reload(): Promise<this>; }\n',
};

async function withOmittedResolutionProject(observe) {
  const temporary = await mkdtemp(join(tmpdir(), 'lsp-trace-caller-omitted-resolution-'));
  const root = await realpath(temporary);
  try {
    for (const [relative, content] of Object.entries(omittedResolutionFixture)) {
      const path = join(root, relative);
      await mkdir(dirname(path), { recursive: true });
      await writeFile(path, content);
    }
    await observe(root);
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}

async function request() {
  const source = await readFile(seedPath);
  return {
    documents: [{ uri: pathToFileURL(seedPath).href, revision: commit, digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source: source.toString() }],
    relation_kinds: ['INVOKES_TASK'],
  };
}

async function assertCallerQualified(assertion) {
  const result = await analyzeSourceConstrainedTypeScript(await request());
  assert.equal(result.outcome, 'COMPLETE', `${assertion} precondition: analysis must execute`);
  assert.equal(result.observations.length, 1, `${assertion} precondition: relation must be admitted`);
  assert.match(result.observations[0].to.node_id, /package=ember-concurrency@5\.2\.0;path=node_modules\/ember-concurrency\/index\.d\.ts(?:;|$)/, `${assertion} expected caller-local declaration and package custody`);
}

async function mutateFile(path, replacement, observe) {
  const original = await readFile(path);
  try {
    await writeFile(path, replacement);
    await observe();
  } finally {
    await writeFile(path, original);
  }
}

async function hideFile(path, observe) {
  const hidden = `${path}.mutation-hidden`;
  await rename(path, hidden);
  try {
    await observe();
  } finally {
    await rename(hidden, path);
  }
}

test('ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT', async () => {
  await assertCallerQualified('ASSERT_P1B_CALLER_PROJECT_DEPENDENCY_ROOT');
});

test('ASSERT_CALLER_OMITTED_MODULE_RESOLUTION_USES_NODE_COMPATIBLE_POLICY', async () => {
  assert.deepEqual(Object.keys(omittedResolutionFixture).sort(), [
    'jsconfig.json',
    'node_modules/@warp-drive/legacy/declarations/model.d.ts',
    'node_modules/@warp-drive/legacy/package.json',
    'node_modules/ember-concurrency/declarations/index.d.ts',
    'node_modules/ember-concurrency/package.json',
    'src/positive.js',
    'src/reload-origin.js',
    'src/reload.js',
    'src/same-spelling.js',
    'src/unknown.js',
    'src/unresolved.js',
    'src/unsafe.js',
    'src/wrong-parent.js',
  ], 'ASSERT_CALLER_OMITTED_RESOLUTION_CLEAN_CHECKOUT_INPUTS_ARE_CONSTRUCTED');
  await withOmittedResolutionProject(async (omittedResolutionRoot) => {
    for (const [relative, expected] of Object.entries(omittedResolutionFixture)) {
      assert.equal((await readFile(join(omittedResolutionRoot, relative), 'utf8')), expected, `required constructed fixture input absent or changed: ${relative}`);
    }
    const taskDeclaration = await readFile(join(omittedResolutionRoot, 'node_modules/ember-concurrency/declarations/index.d.ts'), 'utf8');
    assert.match(taskDeclaration, /type TaskForAsyncTaskFunction<[\s\S]*AbstractTask<Parameters<Fn>/, 'ASSERT_GENERIC_TASK_FIXTURE_MODELS_MODERN_PUBLIC_ALIAS');
    assert.match(taskDeclaration, /interface AbstractTask<[\s\S]*perform\(\.\.\.args: Args\): T;/, 'required checker-resolved declaration must be present');

    async function analyze(name, relationKinds = ['INVOKES_TASK']) {
      const path = join(omittedResolutionRoot, `src/${name}.js`);
      const source = await readFile(path);
      return analyzeSourceConstrainedTypeScript({
        documents: [{ uri: pathToFileURL(path).href, revision: commit, digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source: source.toString() }],
        relation_kinds: relationKinds,
      });
    }

    const positive = await analyze('positive');
    assert.equal(positive.outcome, 'COMPLETE', `ASSERT_CALLER_OMITTED_MODULE_RESOLUTION_USES_NODE_COMPATIBLE_POLICY ${positive.blocker ?? ''}`);
    assert.equal(positive.observations.length, 6, 'ASSERT_GENERIC_TASK_SIX_DECLARATION_BACKED_RECEIVERS_QUALIFY');
    for (const observation of positive.observations) {
      assert.match(observation.from.node_id, /receiver=TaskForAsyncTaskFunction/, 'ASSERT_GENERIC_TASK_PUBLIC_ALIAS_RECEIVER_MUST_QUALIFY');
      assert.match(observation.to.node_id, /package=ember-concurrency@5\.2\.0;path=node_modules\/ember-concurrency\/declarations\/index\.d\.ts;symbol=AbstractTask\.perform;/, 'checker must resolve the exact AbstractTask.perform declaration identity in the installed layout');
    }

    const reload = await analyze('reload', ['TRIGGERS_RELOAD']);
    assert.equal(reload.outcome, 'COMPLETE', `ASSERT_CALLER_OMITTED_MODULE_RESOLUTION_EXPORTS_SUBPATH ${reload.blocker ?? ''}`);
    assert.equal(reload.observations.length, 1, 'exported model declaration must qualify');
    assert.match(reload.observations[0].to.node_id, /package=@warp-drive\/legacy@5\.8\.1;path=node_modules\/@warp-drive\/legacy\/declarations\/model\.d\.ts;symbol=Model\.reload;/);

    const reloadOrigin = await analyze('reload-origin', ['TRIGGERS_RELOAD']);
    assert.equal(reloadOrigin.outcome, 'COMPLETE', `ASSERT_TRIGGERS_RELOAD_FOR_OF_PROPERTY_PARAMETER_ALIAS_ORIGIN ${reloadOrigin.blocker ?? ''}`);
    assert.equal(reloadOrigin.observations.length, 1, 'ASSERT_TRIGGERS_RELOAD_FOR_OF_PROPERTY_PARAMETER_ALIAS_ORIGIN');
    assert.match(reloadOrigin.observations[0].from.node_id, /receiver=any;origin=compiler-value-flow/);
    assert.match(reloadOrigin.observations[0].to.node_id, /package=@warp-drive\/legacy@5\.8\.1;path=node_modules\/@warp-drive\/legacy\/declarations\/model\.d\.ts;symbol=Model\.reload;/);

    for (const [name, tail] of [
      ['heterogeneous', "uploads.add([model, { reload() {} }]);"],
      ['unknown-origin', "/** @type {any} */ const unknown = {}; uploads.add([model, unknown]);"],
      ['multiple-origins', "class Other { reload() {} } uploads.add([model]); uploads.add([new Other()]);"],
    ]) {
      const path = join(omittedResolutionRoot, `src/reload-origin-${name}.js`);
      await writeFile(path, `import Model from '@warp-drive/legacy/model';\nclass Uploads { uploads = []; add(values) { this.uploads = [...this.uploads, ...values]; } poll() { for (const upload of this.uploads) upload.reload(); } }\nconst uploads = new Uploads(); const model = new Model(); ${tail}\n`);
      const negative = await analyze(`reload-origin-${name}`, ['TRIGGERS_RELOAD']);
      assert.equal(negative.outcome, 'BLOCKED', `ASSERT_TRIGGERS_RELOAD_ORIGIN_REJECTS_${name.toUpperCase()}`);
      assert.equal(negative.coverage.status, 'UNAVAILABLE');
      assert.deepEqual(negative.observations, []);
    }

    const sameSpelling = await analyze('same-spelling');
    assert.equal(sameSpelling.outcome, 'EMPTY', 'same-spelling method must not qualify');
    assert.deepEqual(sameSpelling.observations, []);

    const wrongParent = await analyze('wrong-parent');
    assert.equal(wrongParent.outcome, 'EMPTY', 'wrong declaration parent must not qualify');
    assert.deepEqual(wrongParent.observations, []);

    const unknown = await analyze('unknown');
    assert.equal(unknown.outcome, 'BLOCKED', 'unknown receiver must remain explicit BLOCKED');
    assert.match(unknown.blocker, /bounded checker failure: TS2339 .* Property 'perform' does not exist on type 'unknown'/);

    const unresolved = await analyze('unresolved');
    assert.equal(unresolved.outcome, 'BLOCKED', 'unresolved receiver must fail closed');
    assert.match(unresolved.blocker, /bounded checker failure: TS2307/);

    const unsafe = await analyze('unsafe', ['INVOKES_TASK', 'TRIGGERS_RELOAD']);
    assert.equal(unsafe.outcome, 'BLOCKED', 'two any task calls and any reload must remain explicit BLOCKED');
    assert.match(unsafe.blocker, /unsafe compiler identity:/);
    assert.equal((unsafe.blocker.match(/receiver type any/g) ?? []).length, 3, 'ASSERT_TWO_ANY_TASKS_AND_ANY_RELOAD_FAIL_CLOSED');

    const manifestPath = join(omittedResolutionRoot, 'node_modules/ember-concurrency/package.json');
    await mutateFile(manifestPath, omittedResolutionFixture['node_modules/ember-concurrency/package.json'].replace('5.2.0', '5.2.1'), async () => {
      const wrongVersion = await analyze('positive');
      assert.equal(wrongVersion.outcome, 'EMPTY', 'wrong package version must not qualify');
      assert.deepEqual(wrongVersion.observations, []);
    });
  });
});

test('ASSERT_CALLER_MODULE_RESOLUTION_OMISSION_ONLY_POLICY_MATRIX', async () => {
  await withOmittedResolutionProject(async (omittedResolutionRoot) => {
    const configPath = join(omittedResolutionRoot, 'jsconfig.json');
    const positivePath = join(omittedResolutionRoot, 'src/positive.js');
    const source = await readFile(positivePath);
    const analyze = () => analyzeSourceConstrainedTypeScript({
      documents: [{ uri: pathToFileURL(positivePath).href, revision: commit, digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source: source.toString() }],
      relation_kinds: ['INVOKES_TASK'],
    });
    const config = compilerOptions => JSON.stringify({ compilerOptions: { target: 'ES2022', experimentalDecorators: true, checkJs: true, allowJs: true, noEmit: true, ...compilerOptions }, include: ['src/**/*.js'] });
    for (const [name, options, expected, blocker] of [
      ['omitted-default-module', {}, 'COMPLETE'],
      ['omitted-nodenext-module', { module: 'NodeNext' }, 'COMPLETE'],
      ['explicit-classic', { module: 'ESNext', moduleResolution: 'Classic' }, 'BLOCKED', /TS2792/],
      ['explicit-nodenext', { module: 'NodeNext', moduleResolution: 'NodeNext' }, 'COMPLETE'],
      ['explicit-bundler', { module: 'ESNext', moduleResolution: 'Bundler' }, 'COMPLETE'],
    ]) {
      await writeFile(configPath, config(options));
      const result = await analyze();
      assert.equal(result.outcome, expected, `ASSERT_CALLER_MODULE_RESOLUTION_OMISSION_ONLY_POLICY_MATRIX ${name}: ${result.blocker ?? ''}`);
      if (blocker) assert.match(result.blocker, blocker, name);
    }
  });
});

test('ASSERT_P2B_P3A_UNCHECKED_JS_SEMANTIC_UNCERTAINTY_NEVER_BECOMES_ABSENCE', async () => {
  const source = await readFile(uncheckedSeedPath);
  const result = await analyzeSourceConstrainedTypeScript({
    documents: [{ uri: pathToFileURL(uncheckedSeedPath).href, revision: commit, digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source: source.toString() }],
    relation_kinds: ['INVOKES_TASK', 'TRIGGERS_RELOAD'],
  });
  assert.equal(result.outcome, 'COMPLETE', 'ASSERT_P2B_P3A_UNCHECKED_JS_SEMANTIC_UNCERTAINTY_NEVER_BECOMES_ABSENCE outcome');
  assert.deepEqual(result.observations.map(({ kind }) => kind), ['INVOKES_TASK', 'INVOKES_TASK', 'TRIGGERS_RELOAD'], 'ASSERT_P2B_P3A_UNCHECKED_JS_SEMANTIC_UNCERTAINTY_NEVER_BECOMES_ABSENCE compiler-owned identities only');
});

test('ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT', async () => {
  const config = join(projectRoot, 'jsconfig.json');
  await hideFile(config, async () => {
    await assert.rejects(analyzeSourceConstrainedTypeScript(await request()), /project config failure:/, 'ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT A-fail: ignored containing config must fail explicitly rather than qualify through an ancestor config');
  });
  await assertCallerQualified('ASSERT_P1A_IGNORED_CONFIG_BLOCKS_CALLER_PROJECT A-pass');
});

test('ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY', async () => {
  const config = join(projectRoot, 'jsconfig.json');
  await mutateFile(config, '{ malformed', async () => {
    await assert.rejects(analyzeSourceConstrainedTypeScript(await request()), /project config failure:/, 'ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY A-fail');
  });
  await assertCallerQualified('ASSERT_P1A_MALFORMED_CONFIG_FAILS_EXPLICITLY A-pass');
});

test('ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY', async () => {
  const declaration = join(projectRoot, 'node_modules/ember-concurrency/index.d.ts');
  await hideFile(declaration, async () => {
    const result = await analyzeSourceConstrainedTypeScript(await request());
    assert.equal(result.outcome, 'BLOCKED', 'ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-fail');
    assert.equal(result.coverage.status, 'UNAVAILABLE', 'ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-fail');
    assert.match(result.blocker, /bounded checker failure: TS2307 .* Cannot find module 'ember-concurrency'/, 'ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-fail');
  });
  await assertCallerQualified('ASSERT_P1B_REMOVED_CALLER_DECLARATION_DOES_NOT_QUALIFY A-pass');
});

test('ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY', async () => {
  const manifest = join(projectRoot, 'node_modules/ember-concurrency/package.json');
  await mutateFile(manifest, '{"name":"confusable-concurrency","version":"5.2.0","type":"module","types":"index.d.ts"}\n', async () => {
    const result = await analyzeSourceConstrainedTypeScript(await request());
    assert.equal(result.outcome, 'EMPTY', 'ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-fail');
    assert.deepEqual(result.observations, [], 'ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-fail');
  });
  await assertCallerQualified('ASSERT_P2A_WRONG_PACKAGE_CUSTODY_DOES_NOT_QUALIFY A-pass');
});

test('ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY', async () => {
  const declaration = join(projectRoot, 'node_modules/ember-concurrency/index.d.ts');
  const hidden = `${declaration}.mutation-hidden`;
  await rename(declaration, hidden);
  try {
    await symlink(join(providerRoot, 'fixtures/source-constrained-synthetic/vendor/ember-concurrency/index.d.ts'), declaration);
    const result = await analyzeSourceConstrainedTypeScript(await request());
    assert.equal(result.outcome, 'EMPTY', 'ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-fail');
    assert.deepEqual(result.observations, [], 'ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-fail');
  } finally {
    await unlink(declaration);
    await rename(hidden, declaration);
  }
  await assertCallerQualified('ASSERT_P2A_WRONG_DECLARATION_CUSTODY_DOES_NOT_QUALIFY A-pass');
});
