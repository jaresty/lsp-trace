import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import ts from 'typescript';

const sha256 = value => createHash('sha256').update(value).digest('hex');
const diagnostic = (code, path, detail, extra = {}) => ({ code, path, detail, ...extra });
function proposal(blocker, diagnostics, edits = []) { return { complete: true, ...(blocker ? { blocker } : {}), diagnostics, edits }; }

function environmentEdit(workspace, configPath) {
  const path = join(workspace, configPath), bytes = readFileSync(path);
  const config = JSON.parse(bytes);
  config.compilerOptions ??= {};
  const types = Array.isArray(config.compilerOptions.types) ? config.compilerOptions.types : [];
  if (!types.includes('ember-source/types')) types.push('ember-source/types');
  config.compilerOptions.types = [...new Set(types)].sort();
  return { path: configPath, preimage_sha256: sha256(bytes), content: `${JSON.stringify(config, null, 2)}\n`, write_origin: 'EMBER_TYPES_ENVIRONMENT' };
}
function loadProgram(workspace, configPath) {
  const absolute = join(workspace, configPath);
  const loaded = ts.readConfigFile(absolute, ts.sys.readFile);
  if (loaded.error) throw new Error(ts.flattenDiagnosticMessageText(loaded.error.messageText, ' '));
  const parsed = ts.parseJsonConfigFileContent(loaded.config, ts.sys, workspace, { allowJs: true, checkJs: true, noEmit: true }, absolute);
  return ts.createProgram(parsed.fileNames, parsed.options);
}
function typeName(checker, node) { return checker.typeToString(checker.getTypeAtLocation(node)); }
function unsafe(checker, node) { return (checker.getTypeAtLocation(node).flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)) !== 0; }
function reloadsLoopReceiver(loop, checker) {
  const declaration = loop.initializer.declarations[0];
  if (!declaration || !ts.isIdentifier(declaration.name)) return false;
  const receiver = checker.getSymbolAtLocation(declaration.name);
  let found = false;
  const visit = node => {
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === 'reload' && ts.isIdentifier(node.expression.expression) && checker.getSymbolAtLocation(node.expression.expression) === receiver) found = true;
    ts.forEachChild(node, visit);
  };
  visit(loop.statement);
  return found;
}

export function createEmberJSTypeOverlayAdapter({ configPath = 'jsconfig.json', sourcePath, collectionProperty, elementImport, elementExport = 'default' }) {
  if (!sourcePath || !collectionProperty || !elementImport) throw new Error('sourcePath, collectionProperty, and elementImport are required');
  return Object.freeze({
    async proposeEnvironment({ workspace }) {
      const edit = environmentEdit(workspace, configPath);
      return proposal(null, [diagnostic('ENVIRONMENT_TYPES_ADDED', configPath, 'adds ember-source/types for analysis only')], [edit]);
    },
    async proposeOverlay({ workspace }) {
      const program = loadProgram(workspace, configPath), checker = program.getTypeChecker();
      const path = join(workspace, sourcePath), source = program.getSourceFile(path);
      if (!source) return proposal({ code: 'SOURCE_NOT_LOADED', path: sourcePath }, []);
      const diagnostics = [], writes = [], loops = [], properties = [];
      const visit = node => {
        if (ts.isPropertyDeclaration(node) && node.name?.getText(source) === collectionProperty) properties.push(node);
        if (ts.isForOfStatement(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === collectionProperty) loops.push(node);
        if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === 'push' && ts.isPropertyAccessExpression(node.expression.expression) && node.expression.expression.name.text === collectionProperty) {
          for (const argument of node.arguments) writes.push({ kind: 'push', node: argument });
        }
        if (ts.isBinaryExpression(node) && ts.isPropertyAccessExpression(node.left) && node.left.name.text === collectionProperty) writes.push({ kind: 'assignment', node: node.right });
        ts.forEachChild(node, visit);
      };
      visit(source);
      if (properties.length !== 1 || !properties[0].initializer || !ts.isArrayLiteralExpression(properties[0].initializer) || properties[0].initializer.elements.length !== 0) return proposal({ code: 'UNSUPPORTED_INITIALIZER', path: sourcePath, property: collectionProperty }, diagnostics);
      const receiver = loops[0]?.initializer.declarations[0]?.name;
      if (loops.length !== 1 || !receiver || !reloadsLoopReceiver(loops[0], checker) || !unsafe(checker, receiver)) return proposal({ code: 'UNSUPPORTED_RECEIVER', path: sourcePath, property: collectionProperty }, diagnostics);
      if (writes.some(write => write.kind !== 'push')) return proposal({ code: 'UNSUPPORTED_WRITE', path: sourcePath, property: collectionProperty }, writes.map(write => diagnostic('WRITE_ORIGIN', sourcePath, write.kind)));
      if (writes.length === 0) return proposal({ code: 'INCOMPLETE_WRITE_ENUMERATION', path: sourcePath, property: collectionProperty }, diagnostics);
      const origins = [];
      for (const write of writes) {
        const name = typeName(checker, write.node);
        diagnostics.push(diagnostic('WRITE_ORIGIN', sourcePath, 'push argument', { type: name }));
        if (unsafe(checker, write.node)) return proposal({ code: 'UNSAFE_WRITE_ORIGIN', path: sourcePath, type: name }, diagnostics);
        origins.push(name);
      }
      if (new Set(origins).size !== 1) return proposal({ code: 'HETEROGENEOUS_WRITE_ORIGINS', path: sourcePath, types: [...new Set(origins)].sort() }, diagnostics);
      const bytes = readFileSync(path), property = properties[0], start = property.getFullStart();
      const quote = JSON.stringify(elementImport), imported = elementExport === 'default' ? `import(${quote}).default` : `import(${quote}).${elementExport}`;
      const content = `${bytes.subarray(0, start).toString()}  /** @type {${imported}[]} */\n${bytes.subarray(start).toString()}`;
      diagnostics.push(diagnostic('OVERLAY_PROPOSED', sourcePath, 'empty collection property receives imported element type', { write_origins_complete: true, origin_type: origins[0] }));
      return proposal(null, diagnostics, [{ path: relative(workspace, path), preimage_sha256: sha256(bytes), content, write_origin: 'PRECONDITIONED_EMPTY_COLLECTION_JSDOC_OVERLAY' }]);
    },
  });
}
