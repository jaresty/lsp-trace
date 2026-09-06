import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import * as tsModule from 'typescript';

const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const SUPPORTS = Object.freeze(['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
const DOES_NOT_SUPPORT = Object.freeze(['callback_execution', 'feature_identity', 'render_occurrence', 'repaint', 'runtime_execution', 'source_completeness']);
const BOUNDARY = 'one exact Glint template property access mapped to one source-declared typed getter/property with one direct this.args member read';

function point(source, value) {
  const lines = source.slice(0, value).split('\n');
  return { line: lines.length - 1, character: lines.at(-1).length };
}

function offset(source, position) {
  if (!Number.isInteger(position?.line) || position.line < 0 || !Number.isInteger(position?.character) || position.character < 0) return undefined;
  const lines = source.split('\n');
  if (position.line >= lines.length || position.character > lines[position.line].length) return undefined;
  let value = position.character;
  for (let line = 0; line < position.line; line += 1) value += lines[line].length + 1;
  return value;
}

function anchor(uri, source, start, end) {
  return {
    uri,
    bytes: { start: Buffer.byteLength(source.slice(0, start)), end: Buffer.byteLength(source.slice(0, end)) },
    range: { start: point(source, start), end: point(source, end) },
    text: source.slice(start, end),
  };
}

function blocked(reason, detail) {
  return { status: 'BLOCKED', reason, ...(detail ? { detail } : {}), observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
}

function withinProject(filename, projectDirectory) {
  const relative = path.relative(path.resolve(projectDirectory), path.resolve(filename));
  return relative !== '' && relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function locationOffsets(location, projectDirectory) {
  try {
    const filename = fileURLToPath(location.uri);
    if (!withinProject(filename, projectDirectory) || !existsSync(filename)) return undefined;
    const source = readFileSync(filename, 'utf8');
    const start = offset(source, location.range?.start);
    const end = offset(source, location.range?.end);
    if (start === undefined || end === undefined || end <= start) return undefined;
    return { filename, source, start, end };
  } catch {
    return undefined;
  }
}

function exactNodeAt(root, start, end, predicate) {
  let found;
  function visit(node) {
    if (node.getStart(root) > start || node.getEnd() < end) return;
    if (predicate(node) && node.getStart(root) === start && node.getEnd() === end) found = node;
    ts.forEachChild(node, visit);
  }
  visit(root);
  return found;
}

function unsafeType(checker, node) {
  const type = checker.getTypeAtLocation(node);
  return Boolean(type.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown));
}

function directArgsRead(member) {
  let expression;
  if (ts.isGetAccessorDeclaration(member)) {
    const statements = member.body?.statements ?? [];
    if (statements.length !== 1 || !ts.isReturnStatement(statements[0])) return undefined;
    expression = statements[0].expression;
  } else if (ts.isPropertyDeclaration(member)) {
    expression = member.initializer;
  }
  if (!expression) return undefined;
  let current = expression;
  while (ts.isPropertyAccessExpression(current)) {
    if (ts.isPropertyAccessExpression(current.expression)
      && current.expression.expression.kind === ts.SyntaxKind.ThisKeyword
      && current.expression.name.text === 'args') return current;
    current = current.expression;
  }
  return undefined;
}

function rendersFrom({ analysis, projectDirectory, filename, source, position, requestedStart, requestedEnd, document }) {
  const uri = pathToFileURL(filename).href;
  const selectedOffset = offset(source, position);
  if (selectedOffset === undefined) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');
  const selected = analysis.transformManager.getTransformedOffset(filename, selectedOffset);
  if (!selected) return blocked('GLINT_MAPPING_UNAVAILABLE');
  const program = analysis.languageServer.service.getProgram();
  if (!program) return blocked('TYPESCRIPT_PROGRAM_UNAVAILABLE');
  const generated = program.getSourceFile(selected.transformedFileName);
  if (!generated) return blocked('TYPESCRIPT_GENERATED_SOURCE_UNAVAILABLE');

  let access;
  function visit(node) {
    if (node.getStart(generated) <= selected.transformedOffset && node.getEnd() >= selected.transformedOffset) {
      if (ts.isPropertyAccessExpression(node)) access = node;
      ts.forEachChild(node, visit);
    }
  }
  visit(generated);
  if (!access || !ts.isPropertyAccessExpression(access)
    || !ts.isPropertyAccessExpression(access.expression)
    || access.expression.name.text !== 'this'
    || !ts.isIdentifier(access.expression.expression)) return blocked('DIRECT_TEMPLATE_PROPERTY_ACCESS_UNAVAILABLE');

  let original;
  try {
    original = analysis.transformManager.getOriginalRange(generated.fileName, access.getStart(generated), access.getEnd());
  } catch (error) {
    return blocked('GLINT_MAPPING_UNAVAILABLE', String(error?.message ?? error));
  }
  if (!original || original.originalFileName !== filename) return blocked('GLINT_MAPPING_UNAVAILABLE');
  if (requestedStart !== undefined && (original.originalStart !== requestedStart || original.originalEnd !== requestedEnd)) return blocked('GLINT_MAPPING_UNAVAILABLE');
  const roundTrip = analysis.transformManager.getTransformedRange(filename, original.originalStart, original.originalEnd);
  if (!roundTrip || roundTrip.transformedFileName !== generated.fileName || roundTrip.transformedStart !== access.getStart(generated) || roundTrip.transformedEnd !== access.getEnd()) return blocked('GLINT_MAPPING_UNAVAILABLE');

  const definitions = analysis.languageServer.getDefinition(uri, position);
  if (!Array.isArray(definitions) || definitions.length === 0) return blocked('DEFINITION_UNRESOLVED');
  if (definitions.length !== 1) return blocked('DEFINITION_AMBIGUOUS');
  const definition = locationOffsets(definitions[0], projectDirectory);
  if (!definition) return blocked('DEFINITION_GENERATED_ONLY');
  const generatedDefinition = analysis.transformManager.getTransformedRange(definition.filename, definition.start, definition.end);
  if (!generatedDefinition) return blocked('GLINT_MAPPING_UNAVAILABLE');
  const definitionSource = program.getSourceFile(generatedDefinition.transformedFileName);
  if (!definitionSource) return blocked('TYPESCRIPT_GENERATED_SOURCE_UNAVAILABLE');
  const definitionName = exactNodeAt(definitionSource, generatedDefinition.transformedStart, generatedDefinition.transformedEnd, ts.isIdentifier);
  if (!definitionName) return blocked('DEFINITION_IDENTITY_UNAVAILABLE');

  const checker = program.getTypeChecker();
  const referenceSymbol = checker.getSymbolAtLocation(access.name);
  const declarationSymbol = checker.getSymbolAtLocation(definitionName);
  if (!referenceSymbol || !declarationSymbol || referenceSymbol !== declarationSymbol) return blocked('DEFINITION_IDENTITY_UNAVAILABLE');
  const declarations = declarationSymbol.declarations ?? [];
  const members = declarations.filter((node) => ts.isGetAccessorDeclaration(node) || ts.isPropertyDeclaration(node));
  if (members.length !== 1 || !members[0].type) return blocked('TYPED_GETTER_OR_PROPERTY_UNAVAILABLE');
  const member = members[0];
  if (unsafeType(checker, definitionName) || unsafeType(checker, access.name)) return blocked('UNSAFE_OR_UNKNOWN_TYPE');

  const read = directArgsRead(member);
  if (!read || unsafeType(checker, read.name)) return blocked(read ? 'UNSAFE_OR_UNKNOWN_TYPE' : 'DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE');
  const argsSymbol = checker.getSymbolAtLocation(read.name);
  if (!argsSymbol || !(argsSymbol.declarations ?? []).some((node) => ts.isPropertySignature(node) || ts.isPropertyDeclaration(node))) return blocked('ARGS_MEMBER_IDENTITY_UNAVAILABLE');
  const originalRead = analysis.transformManager.getOriginalRange(definitionSource.fileName, read.expression.getStart(definitionSource), read.name.getEnd());
  if (!originalRead || originalRead.originalFileName !== definition.filename) return blocked('GLINT_MAPPING_UNAVAILABLE');

  if (!document?.document_id || !document.revision || !document.blob) return blocked('ORIGINAL_DOCUMENT_CUSTODY_UNAVAILABLE');
  const memberName = definition.source.slice(definition.start, definition.end);
  const argsName = definition.source.slice(originalRead.originalStart, originalRead.originalEnd).split('.').at(-1);
  const mappingID = `glint:${document.blob}:${original.originalStart}:${original.originalEnd}`;
  return {
    status: 'SCOPED_ROLE',
    observations: [{
      kind: 'RENDERS_FROM',
      from: { node_id: `${uri}#expression:${original.originalStart}:${original.originalEnd}`, role: 'RENDER_EXPRESSION' },
      to: { node_id: `${definitions[0].uri}#property:${memberName};definition=${definition.start}-${definition.end};direct-read=${originalRead.originalStart}-${originalRead.originalEnd};args=${argsName}`, role: 'READ_VALUE' },
      original_anchor: { document_id: document.document_id, uri, revision: document.revision, blob: document.blob, range: { start: point(source, original.originalStart), end: point(source, original.originalEnd) } },
      virtual_anchor: { document_id: document.document_id, uri: pathToFileURL(generated.fileName).href, revision: document.revision, blob: document.blob, mapping_id: mappingID, range: { start: point(generated.text, access.getStart(generated)), end: point(generated.text, access.getEnd()) } },
      supports: [...SUPPORTS],
      does_not_support: [...DOES_NOT_SUPPORT],
    }],
    coverage: { status: 'BOUNDED', boundary: BOUNDARY },
  };
}

export function createGlintAnalyzer({ loadConfig, analyzeProject }) {
  if (typeof loadConfig !== 'function' || typeof analyzeProject !== 'function') throw new TypeError('supported Glint loadConfig and analyzeProject APIs are required');
  return Object.freeze({
    analyze({ projectDirectory, file, position, range, relationKinds = [], document } = {}) {
      const config = projectDirectory && path.join(projectDirectory, 'tsconfig.json');
      if (!config || !existsSync(config)) return blocked('GLINT_CONFIG_UNAVAILABLE');
      if (typeof file !== 'string' || file.length === 0 || !position) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');
      let analysis;
      try {
        loadConfig(projectDirectory);
        const filename = path.resolve(projectDirectory, file);
        const source = readFileSync(filename, 'utf8');
        const start = range ? offset(source, range.start) : undefined;
        const end = range ? offset(source, range.end) : undefined;
        if (range && (start === undefined || end === undefined || end <= start)) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');
        analysis = analyzeProject(projectDirectory);
        if (relationKinds.includes('RENDERS_FROM')) return rendersFrom({ analysis, projectDirectory, filename, source, position, requestedStart: start, requestedEnd: end, document });
        if (start === undefined || end === undefined) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');
        let transformed;
        let roundTrip;
        try {
          transformed = analysis.transformManager.getTransformedRange(filename, start, end);
          if (!transformed) return blocked('GLINT_MAPPING_UNAVAILABLE');
          roundTrip = analysis.transformManager.getOriginalRange(transformed.transformedFileName, transformed.transformedStart, transformed.transformedEnd);
        } catch (error) { return blocked('GLINT_MAPPING_UNAVAILABLE', String(error?.message ?? error)); }
        if (!roundTrip || roundTrip.originalFileName !== filename || roundTrip.originalStart !== start || roundTrip.originalEnd !== end) return blocked('GLINT_MAPPING_UNAVAILABLE');
        const uri = pathToFileURL(filename).href;
        const definitions = analysis.languageServer.getDefinition(uri, position);
        const type = analysis.languageServer.getHover(uri, position);
        if (!Array.isArray(definitions)) return blocked('GLINT_QUALIFIED_OPERATION_UNAVAILABLE');
        return { status: 'SCOPED_ROLE', observations: [{ kind: 'TYPED_TEMPLATE_DEFINITION', original_anchor: anchor(uri, source, start, end), virtual_mapping: { uri: pathToFileURL(transformed.transformedFileName).href, original: { start, end }, virtual: { start: transformed.transformedStart, end: transformed.transformedEnd } }, definitions, type, support: { outcome: 'SCOPED_ROLE', operation: 'analyzeProject.getDefinition+getHover' } }], coverage: { status: 'BOUNDED', reason: 'REQUESTED_GLINT_QUERY' } };
      } catch (error) { return blocked('GLINT_ANALYSIS_UNAVAILABLE', String(error?.message ?? error)); }
      finally { analysis?.shutdown(); }
    },
  });
}
