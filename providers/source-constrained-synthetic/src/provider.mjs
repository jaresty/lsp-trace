import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

export const IDENTITY = 'source-constrained-synthetic-provider';
export const VERSION = '1.0.0';
export const EVIDENCE_CLASS = 'SOURCE_CONSTRAINED_SYNTHETIC';
export const DISCOVERY_STATUS = 'PROVISIONAL_DISCOVERY';
const packageRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const sha256 = value => createHash('sha256').update(value).digest('hex');
const relations = ['PASSES_CALLBACK', 'INVOKES_TASK', 'TRIGGERS_RELOAD', 'UPDATES_STATE', 'RENDERS_FROM'];
const roles = { PASSES_CALLBACK: ['CALLABLE_REFERENCE', 'CALLBACK_PARAMETER'], INVOKES_TASK: ['TASK_INVOCATION', 'TASK'], TRIGGERS_RELOAD: ['RELOAD_REQUEST', 'RELOAD_TARGET'], UPDATES_STATE: ['STATE_PRODUCER', 'STATE_VALUE'], RENDERS_FROM: ['RENDER_EXPRESSION', 'READ_VALUE'] };
export const metadata = Object.freeze({ identity: IDENTITY, version: VERSION, selectable: true, selection: 'EXPLICIT_ONLY', evidence_class: EVIDENCE_CLASS, discovery_status: DISCOVERY_STATUS, capabilities: { relations, languages: ['typescript'], frameworks: ['source-constrained-synthetic'] } });

let verified;
async function verifyPackage() {
  if (verified) return verified;
  const provenanceBytes = await readFile(join(packageRoot, 'provenance.json'));
  const provenance = JSON.parse(provenanceBytes);
  if (provenance.compiler?.version !== ts.version) throw new Error(`compiler provenance mismatch: expected ${provenance.compiler?.version}, got ${ts.version}`);
  const inputs = [provenance.config, ...Object.values(provenance.packages).flatMap(pkg => pkg.declarations)];
  for (const input of inputs) {
    const bytes = await readFile(join(packageRoot, input.path));
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
function ancestor(node, predicate) { for (let current = node.parent; current; current = current.parent) if (predicate(current)) return current; return null; }

function makeProgram(seedPath) {
  const options = { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.NodeNext, moduleResolution: ts.ModuleResolutionKind.NodeNext, strict: true, noEmit: true, skipLibCheck: true, baseUrl: packageRoot, paths: { 'ember-concurrency': ['vendor/ember-concurrency/index.d.ts'], '@warp-drive/legacy/model': ['vendor/warp-drive/model.d.ts'] } };
  const host = ts.createCompilerHost(options);
  host.resolveModuleNames = (names, containingFile) => names.map(name => {
    if (name === './model/-private/model.js' && containingFile.endsWith('/vendor/warp-drive/model.d.ts')) return { resolvedFileName: join(packageRoot, 'vendor/warp-drive/private-model.d.ts'), extension: ts.Extension.Dts, isExternalLibraryImport: true };
    return ts.resolveModuleName(name, containingFile, options, host).resolvedModule;
  });
  return ts.createProgram([seedPath, join(packageRoot, 'vendor/ember-concurrency/index.d.ts'), join(packageRoot, 'vendor/warp-drive/model.d.ts'), join(packageRoot, 'vendor/warp-drive/private-model.d.ts')], options, host);
}
function declaredTypeAliasName(expression, checker) {
  const symbol = checker.getSymbolAtLocation(expression);
  const declaration = symbol?.valueDeclaration ?? symbol?.declarations?.[0];
  const typeNode = declaration?.type;
  if (!typeNode || !ts.isTypeReferenceNode(typeNode)) return '';
  const alias = checker.getSymbolAtLocation(typeNode.typeName);
  return alias?.getName() ?? '';
}
function exactEndpoint(kind, side, fields) { return `${kind}:${side}:${Object.entries(fields).map(([key, value]) => `${key}=${value}`).join(';')}`; }

function analyzeCall(kind, call, checker, sourceFile, uri, provenance) {
  if (!ts.isPropertyAccessExpression(call.expression)) return null;
  const access = call.expression;
  const member = access.name.text;
  const receiverType = checker.getTypeAtLocation(access.expression);
  if (isUnsafe(receiverType)) return null;
  const symbol = checker.getSymbolAtLocation(access.name);
  const declaration = declarationIdentity(symbol, checker);
  if (!declaration) return null;
  const declarationPath = relative(packageRoot, declaration.sourceFile.fileName).replaceAll('\\', '/');
  const source = exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: checker.typeToString(receiverType) });
  if (kind === 'INVOKES_TASK') {
    const expected = provenance.relations.INVOKES_TASK;
    const receiverName = declaredTypeAliasName(access.expression, checker);
    const validReceiver = receiverName === expected.receiver;
    if (member !== 'perform' || !validReceiver || !hasBaseNamed(receiverType, checker, 'Task') || declaration.parent !== 'AbstractTask' || !declaration.sourceFile.fileName.replaceAll('\\', '/').endsWith('/vendor/ember-concurrency/index.d.ts')) return null;
    return { from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: receiverName }), to: exactEndpoint(kind, 'declaration', { package: 'ember-concurrency@5.2.0', path: 'vendor/ember-concurrency/index.d.ts', symbol: 'AbstractTask.perform', sha256: provenance.packages['ember-concurrency'].declarations[0].sha256, chain: expected.chain.join('→'), evidence: 'typescript-checker', authority: 'non-authoritative' }) };
  }
  if (kind === 'TRIGGERS_RELOAD') {
    const receiverName = typeSymbolName(receiverType);
    const loop = ancestor(access, ts.isForOfStatement);
    if (!loop) return null;
    const collectionType = checker.getTypeAtLocation(loop.expression);
    const elementType = checker.getIndexTypeOfType(collectionType, ts.IndexKind.Number);
    const collectionOK = elementType && !isUnsafe(elementType) && typeSymbolName(elementType) === 'UserImportModel';
    if (member !== 'reload' || receiverName !== 'UserImportModel' || !collectionOK || declaration.parent !== 'Model' || declarationPath !== 'vendor/warp-drive/private-model.d.ts') return null;
    return { from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: receiverName, collection: `${checker.symbolToString(checker.getSymbolAtLocation(loop.expression) ?? { getName: () => 'collection' })}:UserImportModel[]` }), to: exactEndpoint(kind, 'declaration', { package: '@warp-drive/legacy@5.8.1', path: declarationPath, symbol: 'Model.reload', sha256: provenance.packages['@warp-drive/legacy'].declarations[1].sha256, evidence: 'typescript-checker', authority: 'non-authoritative' }) };
  }
  return null;
}

function analyzeOther(kind, node, checker, sourceFile, uri) {
  if (kind === 'PASSES_CALLBACK' && ts.isCallExpression(node) && node.arguments.some(arg => { const type = checker.getTypeAtLocation(arg); return !isUnsafe(type) && type.getCallSignatures().length > 0; })) return { node, from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, node)) }), to: exactEndpoint(kind, 'callback', { identity: checker.typeToString(checker.getTypeAtLocation(node.arguments[0])), evidence: 'typescript-checker' }) };
  if (kind === 'UPDATES_STATE' && ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isIdentifier(node.left)) return { node, from: exactEndpoint(kind, 'assignment', { uri, range: JSON.stringify(sourceRange(sourceFile, node)) }), to: exactEndpoint(kind, 'symbol', { identity: checker.symbolToString(checker.getSymbolAtLocation(node.left)), evidence: 'typescript-checker' }) };
  if (kind === 'RENDERS_FROM' && ts.isVariableDeclaration(node) && node.initializer && ts.isPropertyAccessExpression(node.initializer)) return { node, from: exactEndpoint(kind, 'read', { uri, range: JSON.stringify(sourceRange(sourceFile, node.initializer)) }), to: exactEndpoint(kind, 'symbol', { identity: checker.symbolToString(checker.getSymbolAtLocation(node.initializer.name)), evidence: 'typescript-checker' }) };
  return null;
}

export async function analyze(request) {
  if (request.provider_id !== `${IDENTITY}@${VERSION}`) throw new Error('explicit provider identity required');
  const uri = request.seed?.uri;
  if (!uri?.startsWith('file://') || !uri.endsWith('.ts')) throw new Error('source-constrained TypeScript file seed required');
  const seedPath = fileURLToPath(uri);
  const sourceBytes = await readFile(seedPath);
  const commit = request.document_custody?.workspace_revision?.commit || request.document_custody?.workspace_revision?.value;
  if (!/^[0-9a-f]{40}$/.test(commit || '')) throw new Error('strict git commit required');
  const { provenance } = await verifyPackage();
  const program = makeProgram(seedPath);
  const sourceFile = program.getSourceFile(seedPath);
  if (!sourceFile) throw new Error('checker did not load seed');
  const diagnostics = ts.getPreEmitDiagnostics(program).filter(d => d.file?.fileName === seedPath);
  if (diagnostics.length) throw new Error(`bounded checker failure: ${diagnostics.map(d => ts.flattenDiagnosticMessageText(d.messageText, ' ')).join('; ')}`);
  const checker = program.getTypeChecker();
  const documentID = 'qualification-seed';
  const observations = [];
  const requested = new Set(request.relations || []);
  function visit(node) {
    for (const kind of requested) {
      let resolved = (kind === 'INVOKES_TASK' || kind === 'TRIGGERS_RELOAD') && ts.isCallExpression(node) ? analyzeCall(kind, node, checker, sourceFile, uri, provenance) : analyzeOther(kind, node, checker, sourceFile, uri);
      if (!resolved) continue;
      const anchorNode = resolved.node ?? node;
      observations.push({ kind, from: { node_id: resolved.from, role: roles[kind][0] }, to: { node_id: resolved.to, role: roles[kind][1] }, original_anchor: { document_id: documentID, uri, revision: commit, blob: sha256(sourceBytes), range: sourceRange(sourceFile, anchorNode) }, supports: ['source_dependency_relation'], does_not_support: ['runtime_execution', 'callback_invocation', 'repaint', 'feature_identity', 'whole_source_completeness'] });
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return { provider: { name: IDENTITY, version: VERSION }, protocol: { name: 'lsp-trace.provider-observations', version: '1' }, adapter: { name: 'lsp-trace-observation-adapter', version: '1' }, request_id: `${request.session.session_id}:${request.session.generation}:${uri}`, authority: 'PROVIDER_REPORTED', coverage: { Status: 'COMPLETE_WITHIN_BOUNDS', Denominator: [documentID], Covered: [documentID], CoveredCount: 1 }, documents: [{ document_id: documentID, original_uri: uri, content_sha256: sha256(sourceBytes), revision: { kind: 'git', value: commit, blob: sha256(sourceBytes), custody: 'PROVIDER_PROVED' }, coordinates: 'ORIGINAL' }], observations };
}

export function decodeFrame(buffer) { const i = buffer.indexOf('\r\n\r\n'); if (i < 0) throw new Error('framing'); const h = buffer.subarray(0, i).toString(); if (!/^Content-Length: (0|[1-9]\d*)$/.test(h)) throw new Error('framing'); const body = buffer.subarray(i + 4); if (body.length !== Number(h.slice(16))) throw new Error('length'); return JSON.parse(body); }
export function encodeFrame(value) { const body = Buffer.from(JSON.stringify(value)); return Buffer.concat([Buffer.from(`Content-Length: ${body.length}\r\n\r\n`), body]); }
