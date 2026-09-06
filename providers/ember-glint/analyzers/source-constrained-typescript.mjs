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
function omittedResolutionOptions(options) {
  if (options.module === ts.ModuleKind.NodeNext) return { ...options, moduleResolution: ts.ModuleResolutionKind.NodeNext };
  if (options.module === ts.ModuleKind.Node16) return { ...options, moduleResolution: ts.ModuleResolutionKind.Node16 };
  if (options.module === undefined) return { ...options, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler };
  const bundlerCompatible = options.module === ts.ModuleKind.Preserve || options.module >= ts.ModuleKind.ES2015;
  return { ...options, moduleResolution: bundlerCompatible ? ts.ModuleResolutionKind.Bundler : ts.ModuleResolutionKind.Node10 };
}
function projectConfiguration(seedPath) {
  const configPath = nearestConfig(seedPath);
  if (!configPath) return null;
  const loaded = ts.readConfigFile(configPath, ts.sys.readFile);
  if (loaded.error) throw new Error(`project config failure: ${ts.flattenDiagnosticMessageText(loaded.error.messageText, ' ')}`);
  const parsed = ts.parseJsonConfigFileContent(loaded.config, ts.sys, dirname(configPath), { noEmit: true }, configPath);
  if (parsed.errors.length) throw new Error(`project config failure: ${parsed.errors.map(error => ts.flattenDiagnosticMessageText(error.messageText, ' ')).join('; ')}`);
  const compilerOptions = loaded.config?.compilerOptions;
  const options = compilerOptions && Object.hasOwn(compilerOptions, 'moduleResolution')
    ? parsed.options
    : omittedResolutionOptions(parsed.options);
  return { configPath, root: dirname(configPath), options, files: parsed.fileNames };
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

function expectedPackageIdentity(kind, provenance) {
  const identity = provenance.relations[kind]?.member_declaration?.split(':', 1)[0];
  const separator = identity?.lastIndexOf('@') ?? -1;
  if (separator <= 0) return null;
  return { name: identity.slice(0, separator), version: identity.slice(separator + 1) };
}
function owningPackageRoot(declarationFile, projectRoot) {
  const modulesRoot = join(projectRoot, 'node_modules');
  let directory = dirname(realpathSync(declarationFile));
  const boundary = realpathSync(modulesRoot);
  for (;;) {
    if (!within(directory, boundary)) return null;
    if (existsSync(join(directory, 'package.json'))) return directory;
    if (directory === boundary) return null;
    directory = dirname(directory);
  }
}
function declarationCustody(kind, declarationFile, context, provenance) {
  const expected = expectedPackageIdentity(kind, provenance);
  if (!expected) return null;
  if (context.kind === 'provider-fixture') {
    const path = relative(fixtureRoot, declarationFile).replaceAll('\\', '/');
    const entry = Object.entries(provenance.packages).find(([, pkg]) => pkg.declarations.some(item => item.path === path));
    const record = entry?.[1].declarations.find(item => item.path === path);
    return record && entry[0] === expected.name && entry[1].version === expected.version
      ? { path, sha256: record.sha256, package: `${entry[0]}@${entry[1].version}` }
      : null;
  }
  try {
    const dependencyRoot = owningPackageRoot(declarationFile, context.root);
    if (!dependencyRoot || !within(declarationFile, dependencyRoot)) return null;
    const packageJSON = JSON.parse(readFileSync(join(dependencyRoot, 'package.json'), 'utf8'));
    if (packageJSON.name !== expected.name || packageJSON.version !== expected.version) return null;
    return { path: relative(context.root, declarationFile).replaceAll('\\', '/'), sha256: sha256(readFileSync(declarationFile)), package: `${packageJSON.name}@${packageJSON.version}` };
  } catch { return null; }
}
function sameSymbol(checker, left, right) {
  if (!left || !right) return false;
  const resolve = symbol => (symbol.flags & ts.SymbolFlags.Alias) ? checker.getAliasedSymbol(symbol) : symbol;
  return resolve(left) === resolve(right);
}
function walkProgram(program, visit) {
  for (const file of program.getSourceFiles()) {
    if (file.isDeclarationFile) continue;
    const walk = node => { visit(node); ts.forEachChild(node, walk); };
    walk(file);
  }
}
function compilerValueOrigins(expression, checker, program, seen = new Set(), collection = false) {
  while (ts.isParenthesizedExpression(expression)) expression = expression.expression;
  if (ts.isAsExpression(expression) || ts.isTypeAssertionExpression(expression)) {
    if (isUnsafe(checker.getTypeAtLocation(expression))) return [null];
    expression = expression.expression;
  }
  const key = `${expression.getSourceFile().fileName}:${expression.pos}:${expression.end}:${collection}`;
  if (seen.has(key)) return [];
  seen.add(key);
  if (collection && ts.isArrayLiteralExpression(expression)) {
    return expression.elements.flatMap(element => ts.isSpreadElement(element)
      ? compilerValueOrigins(element.expression, checker, program, new Set(seen), true)
      : compilerValueOrigins(element, checker, program, new Set(seen), false));
  }
  if (collection && ts.isCallExpression(expression) && ts.isPropertyAccessExpression(expression.expression)) {
    if (expression.expression.name.text === 'filter') return compilerValueOrigins(expression.expression.expression, checker, program, new Set(seen), true);
    if (expression.expression.name.text === 'map') {
      const callback = expression.arguments[0];
      if (ts.isArrowFunction(callback) || ts.isFunctionExpression(callback)) {
        const returns = [];
        if (ts.isBlock(callback.body)) {
          const walk = node => { if (ts.isReturnStatement(node) && node.expression) returns.push(node.expression); ts.forEachChild(node, walk); };
          walk(callback.body);
        } else returns.push(callback.body);
        return returns.flatMap(value => compilerValueOrigins(value, checker, program, new Set(seen), true));
      }
    }
  }
  if (ts.isElementAccessExpression(expression)) return compilerValueOrigins(expression.expression, checker, program, new Set(seen), true);
  if (ts.isIdentifier(expression)) {
    const symbol = checker.getSymbolAtLocation(expression);
    const declaration = symbol?.valueDeclaration ?? symbol?.declarations?.[0];
    if (declaration && ts.isVariableDeclaration(declaration)) {
      if (ts.isForOfStatement(declaration.parent?.parent)) return compilerValueOrigins(declaration.parent.parent.expression, checker, program, new Set(seen), true);
      if (isUnsafe(checker.getTypeAtLocation(declaration.name))) return [];
      if (declaration.initializer) {
        let reassigned = false;
        walkProgram(program, node => {
          if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isIdentifier(node.left) && sameSymbol(checker, checker.getSymbolAtLocation(node.left), symbol)) reassigned = true;
        });
        if (!reassigned) return compilerValueOrigins(declaration.initializer, checker, program, new Set(seen), collection);
      }
    }
    if (declaration && ts.isParameter(declaration)) {
      const signatureOwner = declaration.parent;
      if ((ts.isArrowFunction(signatureOwner) || ts.isFunctionExpression(signatureOwner)) && ts.isCallExpression(signatureOwner.parent) && ts.isPropertyAccessExpression(signatureOwner.parent.expression)) {
        return compilerValueOrigins(signatureOwner.parent.expression.expression, checker, program, new Set(seen), true);
      }
      const index = signatureOwner.parameters.indexOf(declaration);
      const origins = [];
      let calls = 0;
      walkProgram(program, node => {
        if (!ts.isCallExpression(node)) return;
        const signature = checker.getResolvedSignature(node);
        if (signature?.declaration !== signatureOwner || !node.arguments[index]) return;
        calls++;
        origins.push(...compilerValueOrigins(node.arguments[index], checker, program, new Set(seen), collection));
      });
      return calls > 0 ? origins : [];
    }
  }
  if (ts.isPropertyAccessExpression(expression)) {
    const symbol = checker.getSymbolAtLocation(expression.name);
    if (symbol) {
      const origins = [];
      walkProgram(program, node => {
        if (!ts.isBinaryExpression(node) || node.operatorToken.kind !== ts.SyntaxKind.EqualsToken || !ts.isPropertyAccessExpression(node.left)) return;
        if (sameSymbol(checker, checker.getSymbolAtLocation(node.left.name), symbol)) origins.push(...compilerValueOrigins(node.right, checker, program, new Set(seen), collection));
      });
      const declaration = symbol.valueDeclaration ?? symbol.declarations?.[0];
      if (declaration?.initializer) origins.push(...compilerValueOrigins(declaration.initializer, checker, program, new Set(seen), collection));
      if (origins.length) return origins;
    }
  }
  if (!collection) {
    const type = checker.getTypeAtLocation(expression);
    return isUnsafe(type) ? [null] : [type];
  }
  return [];
}
function reloadOriginDeclaration(receiver, checker, program) {
  const origins = compilerValueOrigins(receiver, checker, program);
  if (origins.length === 0) return null;
  const declarations = origins.map(type => type && declarationIdentity(checker.getPropertyOfType(type, 'reload'), checker));
  if (declarations.some(value => !value)) return null;
  const first = declarations[0];
  return declarations.every(value => value.symbol === first.symbol && value.parent === first.parent && value.sourceFile.fileName === first.sourceFile.fileName) ? first : null;
}
function analyzeCall(kind, call, checker, program, sourceFile, uri, provenance, context) {
  if (!ts.isPropertyAccessExpression(call.expression)) return null;
  const access = call.expression;
  const member = access.name.text;
  const receiverType = checker.getTypeAtLocation(access.expression);
  const originDeclaration = kind === 'TRIGGERS_RELOAD' && isUnsafe(receiverType) ? reloadOriginDeclaration(access.expression, checker, program) : null;
  if (isUnsafe(receiverType) && !originDeclaration) return null;
  const symbol = checker.getSymbolAtLocation(access.name);
  const declaration = originDeclaration ?? declarationIdentity(symbol, checker);
  if (!declaration) return null;
  const declarationPath = relative(fixtureRoot, declaration.sourceFile.fileName).replaceAll('\\', '/');
  const source = exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: checker.typeToString(receiverType) });
  if (kind === 'INVOKES_TASK') {
    const expected = provenance.relations.INVOKES_TASK;
    const receiverName = typeSymbolName(receiverType);
    const custody = declarationCustody(kind, declaration.sourceFile.fileName, context, provenance);
    if (member !== 'perform' || declaration.symbol !== 'perform' || declaration.parent !== 'AbstractTask' || !custody) return null;
    return { from: exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: receiverName }), to: exactEndpoint(kind, 'declaration', { package: custody.package, path: custody.path, symbol: 'AbstractTask.perform', sha256: custody.sha256, chain: expected.chain.join('→'), evidence: 'typescript-checker', authority: 'non-authoritative' }) };
  }
  if (kind === 'TRIGGERS_RELOAD') {
    const expectedDeclaration = provenance.packages['@warp-drive/legacy'].declarations.find(({ path }) => path === 'vendor/warp-drive/private-model.d.ts');
    const custody = declarationCustody(kind, declaration.sourceFile.fileName, context, provenance);
    const validDeclaration = context.kind === 'provider-fixture' ? declarationPath === expectedDeclaration?.path : Boolean(custody);
    if (member !== 'reload' || declaration.symbol !== 'reload' || declaration.parent !== 'Model' || !validDeclaration || !custody) return null;
    const from = originDeclaration ? exactEndpoint(kind, 'call', { uri, range: JSON.stringify(sourceRange(sourceFile, call)), receiver: checker.typeToString(receiverType), origin: 'compiler-value-flow' }) : source;
    return { from, to: exactEndpoint(kind, 'declaration', { package: custody.package, path: custody.path, symbol: 'Model.reload', sha256: custody.sha256, evidence: 'typescript-checker', authority: 'non-authoritative' }) };
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
  if (!uri?.startsWith('file://') || !/\.(?:js|ts|gts)$/.test(uri)) {
    return {
      outcome: 'UNAVAILABLE',
      observations: [],
      coverage: { status: 'UNKNOWN', reason: 'SOURCE_CONSTRAINED_INPUT_NOT_SUPPORTED' },
    };
  }
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
      const member = node.expression.name.text;
      const requestedUnsafeCandidate = (member === 'perform' && requested.has('INVOKES_TASK')) || (member === 'reload' && requested.has('TRIGGERS_RELOAD'));
      const receiverType = checker.getTypeAtLocation(node.expression.expression);
      if (requestedUnsafeCandidate && isUnsafe(receiverType) && !analyzeCall(member === 'perform' ? 'INVOKES_TASK' : 'TRIGGERS_RELOAD', node, checker, program, sourceFile, uri, provenance, context)) unsafeCalls.push(`${sourceFile.fileName}:${sourceRange(sourceFile, node).start.line + 1}:${sourceRange(sourceFile, node).start.character + 1} receiver type ${checker.typeToString(receiverType)}`);
    }
    for (const kind of requested) {
      let resolved = (kind === 'INVOKES_TASK' || kind === 'TRIGGERS_RELOAD') && ts.isCallExpression(node) ? analyzeCall(kind, node, checker, program, sourceFile, uri, provenance, context) : analyzeOther(kind, node, checker, sourceFile, uri);
      if (!resolved) continue;
      const anchorNode = resolved.node ?? node;
      observations.push({ kind, from: { node_id: resolved.from, role: roles[kind][0] }, to: { node_id: resolved.to, role: roles[kind][1] }, original_anchor: { document_id: documentID, uri, revision: commit, blob: sha256(sourceBytes), range: sourceRange(sourceFile, anchorNode) }, supports: ['source_dependency_relation'], does_not_support: ['runtime_execution', 'callback_invocation', 'repaint', 'feature_identity', 'whole_source_completeness'] });
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  if (unsafeCalls.length) return { outcome: observations.length ? 'PARTIAL' : 'BLOCKED', observations, coverage: { status: observations.length ? 'PARTIAL' : 'UNAVAILABLE', denominator: [uri], covered: [] }, blocker: `unsafe compiler identity: ${unsafeCalls.join('; ')}` };
  return { outcome: observations.length === 0 ? 'EMPTY' : 'COMPLETE', observations, coverage: { status: 'BOUNDED', denominator: [uri], covered: [uri] } };
}
