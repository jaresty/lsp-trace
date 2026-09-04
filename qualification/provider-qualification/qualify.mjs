import { createRequire } from 'node:module';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as tsModule from 'typescript';
import Parser from 'tree-sitter';
import TypeScriptLanguages from 'tree-sitter-typescript';
import { analyzeProject, loadConfig } from '@glint/core';

const require = createRequire(import.meta.url);
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const root = path.dirname(fileURLToPath(import.meta.url));
const fixtureRoot = path.join(root, 'fixtures');
const output = path.join(root, '..', 'retained', 'provider-qualification', 'report.json');
const packageLock = JSON.parse(await readFile(path.join(root, 'package-lock.json'), 'utf8'));
const prohibitedClaims = [
  'callback_invocation_from_passage',
  'feature_identity',
  'repaint',
  'runtime_execution',
  'whole_source_completeness',
];

function pkgVersion(name) {
  return packageLock.packages[`node_modules/${name}`].version;
}

function location(source, start, end) {
  const point = (offset) => {
    const prefix = source.slice(0, offset).split('\n');
    return { line: prefix.length - 1, character: prefix.at(-1).length };
  };
  return { start: point(start), end: point(end) };
}

function entry(id, version, outcome, role, operation, observation, anchors, supportedClaims) {
  return {
    id,
    version,
    outcome,
    role,
    operation,
    observation: observation.replaceAll(root, '${QUALIFICATION}'),
    anchors,
    supported_claims: supportedClaims,
    prohibited_claims: prohibitedClaims,
  };
}

async function checkEmberTemplateCompiler() {
  const source = await readFile(path.join(fixtureRoot, 'component.gts'), 'utf8');
  const template = source.match(/<template>([\s\S]*)<\/template>/)?.[1] ?? '';
  try {
    const compiler = require(path.join(root, 'node_modules', 'ember-source', 'dist', 'dev', 'packages', 'ember-template-compiler', 'index.js'));
    const ast = compiler._preprocess(template, { moduleName: 'fixtures/component.gts' });
    const paths = [];
    const walk = (value) => {
      if (!value || typeof value !== 'object') return;
      if (value.type === 'PathExpression' && value.loc) paths.push({ original: value.original, loc: value.loc });
      for (const child of Object.values(value)) if (child !== value.loc) Array.isArray(child) ? child.forEach(walk) : walk(child);
    };
    walk(ast);
    if (!paths.some((node) => node.original === 'this.uploadFile') || !paths.some((node) => node.original === '@data.length')) {
      throw new Error('expected template paths were not produced');
    }
    return entry('ember-template-compiler', pkgVersion('ember-source'), 'SCOPED_ROLE', 'template-syntax-ast', 'preprocess extracted <template> body', `parsed ${paths.length} PathExpression nodes including callback passage and argument read`, paths.filter((p) => ['this.uploadFile', '@data.length'].includes(p.original)), ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
  } catch (error) {
    return entry('ember-template-compiler', pkgVersion('ember-source'), 'BLOCKED', null, 'preprocess extracted <template> body', String(error.message), [], []);
  }
}

async function checkGlint() {
  try {
    const config = loadConfig(root);
    const result = await analyzeProject(config);
    const diagnostics = Array.isArray(result) ? result.length : 0;
    return entry('glint', pkgVersion('@glint/core'), 'SCOPED_ROLE', 'typed-template-project-analysis', 'loadConfig plus analyzeProject on qualification workspace', `analysis completed with ${diagnostics} diagnostics`, [], ['bounded_source_structure']);
  } catch (error) {
    return entry('glint', pkgVersion('@glint/core'), 'BLOCKED', null, 'loadConfig plus analyzeProject on qualification workspace', String(error.message), [], []);
  }
}

async function checkTreeSitter() {
  const source = await readFile(path.join(fixtureRoot, 'plain.ts'), 'utf8');
  const parser = new Parser();
  parser.setLanguage(TypeScriptLanguages.typescript);
  const tree = parser.parse(source);
  const calls = tree.rootNode.descendantsOfType('call_expression');
  if (tree.rootNode.hasError || calls.length !== 1) throw new Error('expected one error-free call expression');
  const call = calls[0];
  return entry('tree-sitter', pkgVersion('tree-sitter'), 'SCOPED_ROLE', 'typescript-concrete-syntax', 'parse plain.ts with tree-sitter-typescript grammar', `parsed call_expression ${call.text}`, [{ uri: 'fixtures/plain.ts', range: { start: call.startPosition, end: call.endPosition } }], ['bounded_source_structure', 'exact_source_anchor']);
}

async function checkTypeScriptService() {
  const file = path.join(fixtureRoot, 'plain.ts');
  const source = await readFile(file, 'utf8');
  if (typeof ts.createLanguageService !== 'function') {
    return entry('typescript-service', ts.version ?? pkgVersion('typescript'), 'BLOCKED', null, 'import TypeScript and inspect createLanguageService', 'installed provider does not export createLanguageService', [], []);
  }
  const files = new Map([[file, { version: '1', text: source }]]);
  const host = {
    getScriptFileNames: () => [...files.keys()],
    getScriptVersion: (name) => files.get(name)?.version ?? '0',
    getScriptSnapshot: (name) => files.has(name) ? ts.ScriptSnapshot.fromString(files.get(name).text) : undefined,
    getCurrentDirectory: () => root,
    getCompilationSettings: () => ({ target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, strict: true }),
    getDefaultLibFileName: (options) => ts.getDefaultLibFilePath(options),
    fileExists: ts.sys.fileExists,
    readFile: ts.sys.readFile,
    readDirectory: ts.sys.readDirectory,
  };
  const service = ts.createLanguageService(host);
  const callStart = source.lastIndexOf('leaf(value)');
  const definitions = service.getDefinitionAtPosition(file, callStart + 1) ?? [];
  if (!definitions.some((definition) => definition.name === 'leaf')) throw new Error('leaf definition not resolved');
  const definition = definitions.find((item) => item.name === 'leaf');
  return entry('typescript-service', ts.version, 'PASS', 'typescript-symbol-definition', 'createLanguageService then getDefinitionAtPosition at leaf call', `resolved ${definitions.length} definition(s) for leaf`, [{ uri: 'fixtures/plain.ts', range: location(source, definition.textSpan.start, definition.textSpan.start + definition.textSpan.length) }], ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
}

const candidates = [
  await checkEmberTemplateCompiler(),
  await checkGlint(),
  await checkTreeSitter(),
  await checkTypeScriptService(),
].sort((a, b) => a.id.localeCompare(b.id));
const report = { schema_version: 'lsp-trace.provider-qualification.v1', candidates };
await mkdir(path.dirname(output), { recursive: true });
await writeFile(output, JSON.stringify(report, null, 2) + '\n');
for (const candidate of candidates) console.log(`${candidate.id}: ${candidate.outcome}${candidate.role ? ` (${candidate.role})` : ''} — ${candidate.observation}`);
