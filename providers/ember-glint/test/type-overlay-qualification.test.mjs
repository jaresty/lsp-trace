import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { createEmberJSTypeOverlayAdapter } from '../qualification/type-overlay.mjs';

function fixture(body) {
  const root = mkdtempSync(join(tmpdir(), 'ember-overlay-'));
  mkdirSync(join(root, 'src'));
  writeFileSync(join(root, 'jsconfig.json'), `${JSON.stringify({ compilerOptions: { allowJs: true, checkJs: true, module: 'nodenext', moduleResolution: 'nodenext' }, include: ['src/*.js'] }, null, 2)}\n`);
  writeFileSync(join(root, 'src/model.js'), 'export default class UserImport { reload() {} }\n');
  writeFileSync(join(root, 'src/other.js'), 'export default class Other { reload() {} }\n');
  writeFileSync(join(root, 'src/subject.js'), body);
  return root;
}
const adapter = () => createEmberJSTypeOverlayAdapter({ sourcePath: 'src/subject.js', collectionProperty: 'uploads', elementImport: './model.js' });
const positive = `export default class Subject {
  uploads = [];
  /** @param {import('./model.js').default} value */
  add(value) { this.uploads.push(value); }
  reloadAll() { for (const upload of this.uploads) upload.reload(); }
}\n`;
test('proposes environment and preconditioned imported-element JSDoc overlay', async () => {
  const workspace = fixture(positive), instance = adapter();
  const environment = await instance.proposeEnvironment({ workspace });
  assert.equal(environment.complete, true); assert.match(environment.edits[0].content, /ember-source\/types/);
  const overlay = await instance.proposeOverlay({ workspace });
  assert.equal(overlay.complete, true); assert.equal(overlay.edits.length, 1); assert.match(overlay.edits[0].content, /@type \{import\("\.\/model\.js"\)\.default\[\]\}/);
  assert.equal(overlay.diagnostics.at(-1).write_origins_complete, true);
});
for (const [name, body, code] of [
  ['heterogeneous origins', `export default class Subject {
    uploads=[];
    /** @param {import('./model.js').default} a */
    one(a){this.uploads.push(a)}
    /** @param {import('./other.js').default} b */
    two(b){this.uploads.push(b)}
    run(){for(const x of this.uploads)x.reload()}
  }`, 'HETEROGENEOUS_WRITE_ORIGINS'],
  ['any origin', `export default class Subject { uploads=[]; /** @param {*} x */ add(x){this.uploads.push(x)} run(){for(const x of this.uploads)x.reload()} }`, 'UNSAFE_WRITE_ORIGIN'],
  ['unknown origin', `export default class Subject { uploads=[]; /** @param {unknown} x */ add(x){this.uploads.push(x)} run(){for(const x of this.uploads)x.reload()} }`, 'UNSAFE_WRITE_ORIGIN'],
  ['reassignment', `export default class Subject { uploads=[]; reset(){this.uploads=[]} run(){for(const x of this.uploads)x.reload()} }`, 'UNSUPPORTED_WRITE'],
  ['unsupported write enumeration', `export default class Subject { uploads=[]; run(){for(const x of this.uploads)x.reload()} }`, 'INCOMPLETE_WRITE_ENUMERATION'],
]) test(`blocks ${name}`, async () => {
  const result = await adapter().proposeOverlay({ workspace: fixture(body) });
  assert.equal(result.complete, true); assert.equal(result.edits.length, 0); assert.equal(result.blocker.code, code);
});
