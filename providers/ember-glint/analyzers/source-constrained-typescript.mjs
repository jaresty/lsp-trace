import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { existsSync, readFileSync, realpathSync } from 'node:fs';
import { dirname, isAbsolute, join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

export const IDENTITY = 'ember-glint';
export const VERSION = '1';
const packageRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const fixtureRoot = join(packageRoot, 'fixtures/source-constrained-synthetic');
const sha256 = value => createHash('sha256').update(value).digest('hex');
const relations = ['INVOKES_TASK', 'TRIGGERS_RELOAD'];
const roles = { INVOKES_TASK: ['TASK_INVOCATION', 'TASK'], TRIGGERS_RELOAD: ['RELOAD_REQUEST', 'RELOAD_TARGET'] };

let verified;
async function verifyPackage() {
  if (verified) return verified;
  const provenanceBytes = await readFile(join(fixtureRoot, 'provenance.json'));
  const provenance = JSON.parse(provenanceBytes);
  if (provenance.compiler?.version !== ts.version) throw new Error(`compiler provenance mismatch: expected ${provenance.compiler?.version}, got ${ts.version}`);
  const inputs = [provenance.config, ...Object.values(provenance.packages).flatMap(pkg => pkg.declarations)];
  for (const input of inputs) {
    const bytes = await readFile(join(fixtureRoot, input.path));
    if (sha256(bytes) !== input.sha256) throw new Error(`packaged input digest mismatch: ${input.path}`);
  }
  verified = { provenance };
  return verified;
}

function sourceRange(sourceFile, node) {
  const start = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
  const end = sourceFile.getLineAndCharacterOfPosition(node.getEnd());
  return { start: { line: start.line, character: start.character }, end: { line: end.line, character: end.character } };
}
function declarationIdentity(symbol, checker) {
  const declaration = symbol?.valueDeclaration ?? symbol?.declarations?.[0];
  if (!declaration) return null;
  const sourceFile = declaration.getSourceFile();
  return { declaration, sourceFile, parent: declaration.parent?.name?.text, symbol: checker.symbolToString(symbol) };
}
function typeSymbolName(type) { return type.aliasSymbol?.getName() ?? type.getSymbol()?.getName() ?? ''; }
function isUnsafe(type) { return (type.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)) !== 0; }
function hasBaseNamed(type, checker, target, seen = new Set()) {
  if (!type || seen.has(type)) return false;
  seen.add(type);
  const names = [type.aliasSymbol?.getName(), type.getSymbol()?.getName()];
  if (names.includes(target) || checker.typeToString(type).startsWith(`${target}<`)) return true;
  return (type.getBaseTypes?.() ?? []).some(base => hasBaseNamed(base, checker, target, seen));
}

function within(file, root) {
  const rel = relative(realpathSync(root), realpathSync(file));
  return rel === '' || (!rel.startsWith(`..${sep}`) && rel !== '..' && !isAbsolute(rel));
}
function nearestConfig(seedPath) {
  let directory = dirname(seedPath);
  for (;;) {
    for (const name of ['tsconfig.json', 'jsconfig.json']) {
      const candidate = join(directory, name);
      if (existsSync(candidate)) return candidate;
    }
    const parent = dirname(directory);
    if (parent === directory) return null;
    directory = parent;
  }
}
function projectConfiguration(seedPath) {
  const configPath = nearestConfig(seedPath);
  if (!configPath) return null;
  const loaded = ts.readConfigFile(configPath, ts.sys.readFile);
  if (loaded.error) throw new Error(`project config failure: ${ts.flattenDiagnosticMessageText(loaded.error.messageText, ' ')}`);
  const parsed = ts.parseJsonConfigFileContent(loaded.config, ts.sys, dirname(configPath), { noEmit: true }, configPath);
  if (parsed.errors.length) throw new Error(`project config failure: ${parsed.errors.map(error => ts.flattenDiagnosticMessageText(error.messageText, ' ')).join('; ')}`);
  return { configPath, root: dirname(configPath), options: parsed.options, files: parsed.fileNames };
}
function makeProgram(seedPath) {
  const javascript = seedPath.endsWith('.js');
  const fixtureOwned = within(seedPath, fixtureRoot);
  const project = fixtureOwned ? null : projectConfiguration(seedPath);
  if (javascript && !fixtureOwned && !project) return { blocked: 'JavaScript seed has no containing jsconfig/tsconfig' };
  if (project) {
    const options = { ...project.options, noEmit: true, ...(javascript ? { allowJs: true, checkJs: true } : {}) };
    const host = ts.createCompilerHost(options);
    const files = project.files.includes(seedPath) ? project.files : [...project.files, seedPath];
    return { program: ts.createProgram(files, options, host), context: { kind: 'caller-project', root: project.root } };
  }
  const options = { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.NodeNext, moduleResolution: ts.ModuleResolutionKind.NodeNext, strict: true, noEmit: true, skipLibCheck: true, allowNonTsExtensions: true, allowJs: javascript, checkJs: javascript, baseUrl: fixtureRoot, paths: { '@glimmer/component': ['vendor/glimmer-component/index.d.ts'], 'ember-concurrency': ['vendor/ember-concurrency/index.d.ts'], '@ember-data/model': ['vendor/warp-drive/model.d.ts'], '@warp-drive/legacy/model': ['vendor/warp-drive/model.d.ts'] } };
  const host = ts.createCompilerHost(options);
  host.resolveModuleNames = (names, containingFile) => names.map(name => {
    if (name === './model/-private/model.js' && containingFile.endsWith('/vendor/warp-drive/model.d.ts')) return { resolvedFileName: join(fixtureRoot, 'vendor/warp-drive/private-model.d.ts'), extension: ts.Extension.Dts, isExternalLibraryImport: true };
    return ts.resolveModuleName(name, containingFile, options, host).resolvedModule;
  });
  const declarations = ['vendor/glimmer-component/index.d.ts', 'vendor/ember-concurrency/index.d.ts', 'vendor/warp-drive/model.d.ts', 'vendor/warp-drive/private-model.d.ts'].map(path => join(fixtureRoot, path));
  return { program: ts.createProgram([seedPath, ...declarations], options, host), context: { kind: 'provider-fixture', root: fixtureRoot } };
}
function exactEndpoint(kind, side, fields) { return `${kind}:${side}:${Object.entries(fields).map(([key, value]) => `${key}=${value}`).join(';')}`; }

function declarationCustody(kind, declarationFile, context, provenance) {
  const normalized = declarationFile.replaceAll('\\', '/');
  if (context.kind === 'provider-fixture') {
    const path = relative(fixtureRoot, declarationFile).replaceAll('\\', '/');
    const record = Object.values(provenance.packages).flatMap(pkg => pkg.declarations).find(item => item.path === path);
    return record ? { path, sha256: record.sha256 } : null;
  }
  const dependencyRoot = join(context.root, 'node_modules', kind === 'INVOKES_TASK' ? 'ember-concurrency' : '@warp-drive/legacy');
  try { if (!within(declarationFile, dependencyRoot)) return null; } catch { return null; }
  const packageJSON = JSON.parse(readFileSync(join(dependencyRoot, 'package.json'), 'utf8'));
  const expectedName = kind === 'INVOKES_TASK' ? 'ember-concurrency' : '@warp-drive/legacy';
  if (packageJSON.name !== expectedName) return null;
  return { path: relative(context.root, declarationFile).replaceAll('\\', '/'), sha256: sha256(readFileSync(declarationFile)), package: `${packageJSON.name}@${packageJSON.version}`, normalized };
}
function analyzeCall(kind, call, checker, sourceFile, uri, provenance, context) {
  if (!ts.isPropertyAccessExpression(call.expression)) return null;
  const access = call.expression;
  const member = access.name.text;
  const receiverType = checker.getTypeAtLocation(access.expression);
  if (isUnsafe(receiverType)) return null;
  const symbol = checker.getSymbolAtLocation(access.name);
  const declaration = declarationIdentity(symbol, checker);
  if (!declaration) return null;
  const declarationPath = relative(fixtureRoot, declaration.sourceFile.fileName).replaceAll('\\', '/');
  const source = exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: checker.typeToString(receiverType) });
  if (kind === 'INVOKES_TASK') {
    const expected = provenance.relations.INVOKES_TASK;
    const receiverName = typeSymbolName(receiverType);
    const validReceiver = context.kind === 'provider-fixture' ? receiverName === expected.receiver : hasBaseNamed(receiverType, checker, 'Task');
    const custody = declarationCustody(kind, declaration.sourceFile.fileName, context, provenance);
    if (member !== 'perform' || !validReceiver || !hasBaseNamed(receiverType, checker, 'Task') || declaration.parent !== 'AbstractTask' || !custody) return null;
    return { from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: receiverName }), to: exactEndpoint(kind, 'declaration', { package: custody.package ?? 'ember-concurrency@5.2.0', path: custody.path, symbol: 'AbstractTask.perform', sha256: custody.sha256, chain: expected.chain.join('→'), evidence: 'typescript-checker', authority: 'non-authoritative' }) };
  }
  if (kind === 'TRIGGERS_RELOAD') {
    const expectedDeclaration = provenance.packages['@warp-drive/legacy'].declarations.find(({ path }) => path === 'vendor/warp-drive/private-model.d.ts');
    const custody = declarationCustody(kind, declaration.sourceFile.fileName, context, provenance);
    const validDeclaration = context.kind === 'provider-fixture' ? declarationPath === expectedDeclaration?.path : Boolean(custody);
    if (member !== 'reload' || declaration.symbol !== 'reload' || declaration.parent !== 'Model' || !validDeclaration || !custody) return null;
    return { from: source, to: exactEndpoint(kind, 'declaration', { package: custody.package ?? '@warp-drive/legacy@5.8.1', path: custody.path, symbol: 'Model.reload', sha256: custody.sha256, evidence: 'typescript-checker', authority: 'non-authoritative' }) };
  }
  return null;
}

function analyzeOther(kind, node, checker, sourceFile, uri) {
  if (kind === 'PASSES_CALLBACK' && ts.isCallExpression(node) && node.arguments.some(arg => { const type = checker.getTypeAtLocation(arg); return !isUnsafe(type) && type.getCallSignatures().length > 0; })) return { node, from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, node)) }), to: exactEndpoint(kind, 'callback', { identity: checker.typeToString(checker.getTypeAtLocation(node.arguments[0])), evidence: 'typescript-checker' }) };
  if (kind === 'UPDATES_STATE' && ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isIdentifier(node.left)) return { node, from: exactEndpoint(kind, 'assignment', { uri, range: JSON.stringify(sourceRange(sourceFile, node)) }), to: exactEndpoint(kind, 'symbol', { identity: checker.symbolToString(checker.getSymbolAtLocation(node.left)), evidence: 'typescript-checker' }) };
  if (kind === 'RENDERS_FROM' && ts.isVariableDeclaration(node) && node.initializer && ts.isPropertyAccessExpression(node.initializer)) return { node, from: exactEndpoint(kind, 'read', { uri, range: JSON.stringify(sourceRange(sourceFile, node.initializer)) }), to: exactEndpoint(kind, 'symbol', { identity: checker.symbolToString(checker.getSymbolAtLocation(node.initializer.name)), evidence: 'typescript-checker' }) };
  return null;
}

export async function analyzeSourceConstrainedTypeScript(request) {
  const input = request.documents?.[0];
  const uri = input?.uri;
  if (!uri?.startsWith('file://') || !/\.(?:js|ts|gts)$/.test(uri)) throw new Error('source-constrained JavaScript or TypeScript file seed required');
  const requested = new Set(request.relation_kinds || []);
  if (![...requested].some(kind => ['INVOKES_TASK', 'TRIGGERS_RELOAD', 'PASSES_CALLBACK', 'UPDATES_STATE', 'RENDERS_FROM'].includes(kind))) {
    return { outcome: 'EMPTY', observations: [], coverage: { status: 'BOUNDED', denominator: [uri], covered: [uri] } };
  }
  const seedPath = fileURLToPath(uri);
  const sourceBytes = await readFile(seedPath);
  const commit = input.revision;
  if (!/^[0-9a-f]{40}$/.test(commit || '')) throw new Error('strict git commit required');
  const { provenance } = await verifyPackage();
  const loaded = makeProgram(seedPath);
  if (loaded.blocked) return { outcome: 'BLOCKED', observations: [], coverage: { status: 'UNAVAILABLE', denominator: [uri], covered: [] }, blocker: loaded.blocked };
  const { program, context } = loaded;
  const sourceFile = program.getSourceFile(seedPath);
  if (!sourceFile) throw new Error('checker did not load seed');
  const diagnostics = ts.getPreEmitDiagnostics(program).filter(d => d.file?.fileName === seedPath);
  if (diagnostics.length) {
    const blocker = `bounded checker failure: ${diagnostics.map(d => {
      const position = d.file && d.start !== undefined ? d.file.getLineAndCharacterOfPosition(d.start) : null;
      const location = position ? `${d.file.fileName}:${position.line + 1}:${position.character + 1}` : d.file?.fileName ?? seedPath;
      return `TS${d.code} ${location} ${ts.flattenDiagnosticMessageText(d.messageText, ' ')}`;
    }).join('; ')}`;
    return { outcome: 'BLOCKED', observations: [], coverage: { status: 'UNAVAILABLE', denominator: [uri], covered: [] }, blocker };
  }
  const checker = program.getTypeChecker();
  const documentID = 'original';
  const observations = [];
  const unsafeCalls = [];
  function visit(node) {
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression)) {
      const receiverType = checker.getTypeAtLocation(node.expression.expression);
      if (isUnsafe(receiverType)) unsafeCalls.push(`${sourceFile.fileName}:${sourceRange(sourceFile, node).start.line + 1}:${sourceRange(sourceFile, node).start.character + 1} receiver type ${checker.typeToString(receiverType)}`);
    }
    for (const kind of requested) {
      let resolved = (kind === 'INVOKES_TASK' || kind === 'TRIGGERS_RELOAD') && ts.isCallExpression(node) ? analyzeCall(kind, node, checker, sourceFile, uri, provenance, context) : analyzeOther(kind, node, checker, sourceFile, uri);
      if (!resolved) continue;
      const anchorNode = resolved.node ?? node;
      observations.push({ kind, from: { node_id: resolved.from, role: roles[kind][0] }, to: { node_id: resolved.to, role: roles[kind][1] }, original_anchor: { document_id: documentID, uri, revision: commit, blob: sha256(sourceBytes), range: sourceRange(sourceFile, anchorNode) }, supports: ['source_dependency_relation'], does_not_support: ['runtime_execution', 'callback_invocation', 'repaint', 'feature_identity', 'whole_source_completeness'] });
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  if (unsafeCalls.length) return { outcome: 'BLOCKED', observations: [], coverage: { status: 'UNAVAILABLE', denominator: [uri], covered: [] }, blocker: `unsafe compiler identity: ${unsafeCalls.join('; ')}` };
  return { outcome: observations.length === 0 ? 'EMPTY' : 'COMPLETE', observations, coverage: { status: 'BOUNDED', denominator: [uri], covered: [uri] } };
}
