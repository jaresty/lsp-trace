import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const allowed = [
  'README.md', 'analyzer.mjs', 'analyzers/custody.cjs', 'analyzers/glint.mjs',
  'analyzers/passes-callback.mjs', 'analyzers/script.mjs',
  'analyzers/source-constrained-typescript.mjs', 'analyzers/template-relations.mjs', 'analyzers/template.mjs',
  'bin/ember-glint.mjs', 'default-analyzer.mjs',
  'fixtures/source-constrained-synthetic/provenance.json',
  'fixtures/source-constrained-synthetic/tsconfig.json',
  'fixtures/source-constrained-synthetic/vendor/ember-concurrency/index.d.ts',
  'fixtures/source-constrained-synthetic/vendor/warp-drive/model.d.ts',
  'fixtures/source-constrained-synthetic/vendor/warp-drive/private-model.d.ts',
  'package.json', 'renders-from-analyzer.mjs', 'src/protocol.mjs',
];

function runtimeFiles(directory, prefix = '') {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const relative = path.posix.join(prefix, entry.name);
    return entry.isDirectory() ? runtimeFiles(path.join(directory, entry.name), relative) : [relative];
  });
}

test('ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS', () => {
  const pack = JSON.parse(execFileSync('npm', ['pack', '--dry-run', '--json'], { cwd: root, encoding: 'utf8' }))[0];
  assert.deepEqual(pack.files.map(({ path: name }) => name).sort(), allowed, 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
  assert.equal(pack.files.find(({ path: name }) => name === 'bin/ember-glint.mjs')?.mode, 0o755, 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');

  const temporary = mkdtempSync(path.join(tmpdir(), 'ember-glint-install-'));
  try {
    const [{ filename }] = JSON.parse(execFileSync('npm', ['pack', '--json', '--pack-destination', temporary], { cwd: root, encoding: 'utf8' }));
    execFileSync('npm', ['install', '--ignore-scripts', '--offline', path.join(temporary, filename)], { cwd: temporary, stdio: 'pipe' });
    const installed = path.join(temporary, 'node_modules', '@lsp-trace', 'ember-glint-provider');
    assert.deepEqual(runtimeFiles(installed).filter((name) => name !== 'package.json').sort(), allowed.filter((name) => name !== 'package.json'), 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
    const manifest = JSON.parse(readFileSync(path.join(installed, 'package.json'), 'utf8'));
    assert.equal(manifest.bin['ember-glint'], './bin/ember-glint.mjs', 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
    assert.equal(manifest.exports['.'], './default-analyzer.mjs', 'ASSERT_NPM_PACK_AND_OFFLINE_INSTALL_EXACT_RUNTIME_CONTENTS');
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
});

test('ASSERT_PROVIDER_RUNTIME_HAS_NO_DOWNLOAD_DISCOVERY_OR_CORE_IMPORTS', () => {
  const forbidden = [
    /(?:from\s+|import\s*)['"]node:(?:http|https|net|tls|child_process)['"]/,
    /\b(?:fetch|curl|wget|which|whereis)\s*\(/,
    /(?:lsp-trace\/internal|\.\.\/\.\.\/\.\.\/(?:internal|cmd)\/)/,
    /process\.env\.PATH/,
  ];
  for (const relative of allowed.filter((name) => /\.(?:mjs|cjs)$/.test(name))) {
    const source = readFileSync(path.join(root, relative), 'utf8');
    for (const pattern of forbidden) assert.doesNotMatch(source, pattern, `ASSERT_PROVIDER_RUNTIME_HAS_NO_DOWNLOAD_DISCOVERY_OR_CORE_IMPORTS: ${relative}`);
  }
});
