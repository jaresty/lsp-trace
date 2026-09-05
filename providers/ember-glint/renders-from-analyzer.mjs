import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as tsModule from 'typescript';

const defaultTs = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const BOUNDARY = 'one exact Glint template expression mapped to one source-declared typed getter/property with one direct this.args source read';
const SUPPORTS = Object.freeze(['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
const DOES_NOT_SUPPORT = Object.freeze(['repaint', 'render_occurrence', 'runtime_execution', 'callback_execution', 'feature_identity', 'source_completeness']);

function blocked(reason) {
  return { status: 'BLOCKED', reason, observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
}

function point(source, offset) {
  const lines = source.slice(0, offset).split('\n');
  return { line: lines.length - 1, character: lines.at(-1).length };
}

function anchor(uri, source, start, end) {
  return {
    uri,
    bytes: { start: Buffer.byteLength(source.slice(0, start)), end: Buffer.byteLength(source.slice(0, end)) },
    range: { start: point(source, start), end: point(source, end) },
    text: source.slice(start, end),
  };
}

function insideProject(uri, projectDirectory) {
  try {
    const relative = path.relative(path.resolve(projectDirectory), fileURLToPath(uri));
    return relative !== '' && !relative.startsWith(`..${path.sep}`) && relative !== '..' && !path.isAbsolute(relative);
  } catch {
    return false;
  }
}

function memberAtDefinition(ts, sourceFile, start, end) {
  let match;
  function visit(node) {
    if (match) return;
    if ((ts.isGetAccessorDeclaration(node) || ts.isPropertyDeclaration(node))
      && node.name && node.type
      && node.name.getStart(sourceFile) === start && node.name.getEnd() === end) {
      match = node;
      return;
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return match;
}

function directRead(ts, sourceFile, member) {
  let expression;
  if (ts.isGetAccessorDeclaration(member)) {
    const statements = member.body?.statements ?? [];
    if (statements.length !== 1 || !ts.isReturnStatement(statements[0])) return null;
    expression = statements[0].expression;
  } else {
    expression = member.initializer;
  }
  if (!expression) return null;

  let candidate = expression;
  while (ts.isPropertyAccessExpression(candidate) && ts.isPropertyAccessExpression(candidate.expression)) {
    const inner = candidate.expression;
    if (inner.expression.kind === ts.SyntaxKind.ThisKeyword && inner.name.text === 'args') break;
    candidate = inner;
  }
  if (!ts.isPropertyAccessExpression(candidate)
    || !ts.isPropertyAccessExpression(candidate.expression)
    || candidate.expression.expression.kind !== ts.SyntaxKind.ThisKeyword
    || candidate.expression.name.text !== 'args') return null;
  return { start: candidate.getStart(sourceFile), end: candidate.getEnd() };
}

export function createRendersFromAnalyzer({ ts = defaultTs } = {}) {
  if (!ts || typeof ts.createSourceFile !== 'function') throw new TypeError('TypeScript compiler API is required');
  return Object.freeze({
    analyze(request = {}) {
      const definitions = Array.isArray(request.definitions) ? request.definitions : [];
      if (definitions.length === 0) return blocked('DEFINITION_UNRESOLVED');
      if (definitions.length !== 1) return blocked('DEFINITION_AMBIGUOUS');
      const definition = definitions[0];
      if (definition.generated === true) return blocked('DEFINITION_GENERATED_ONLY');
      if (!insideProject(definition.uri, request.projectDirectory)) return blocked('DEFINITION_OUTSIDE_PROJECT');

      const expression = request.template?.expression;
      const templateSource = request.template?.source;
      if (typeof templateSource !== 'string' || !expression) return blocked('ORIGINAL_EXPRESSION_MISMATCH');
      if (!request.mapping
        || request.mapping.original?.start !== expression.start
        || request.mapping.original?.end !== expression.end) return blocked('GLINT_MAPPING_UNAVAILABLE');
      if (typeof definition.source !== 'string') return blocked('DEFINITION_SOURCE_UNAVAILABLE');

      const sourceFile = ts.createSourceFile(fileURLToPath(definition.uri), definition.source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
      const member = memberAtDefinition(ts, sourceFile, definition.start, definition.end);
      if (!member) return blocked('TYPED_GETTER_OR_PROPERTY_UNAVAILABLE');
      const memberName = definition.source.slice(definition.start, definition.end);
      if (templateSource.slice(expression.start, expression.end) !== `this.${memberName}`) return blocked('ORIGINAL_EXPRESSION_MISMATCH');
      const read = directRead(ts, sourceFile, member);
      if (!read) return blocked('DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE');

      const observation = {
        kind: 'RENDERS_FROM',
        from: { node_id: `${definition.uri}#property:${memberName}`, role: 'SOURCE_PROPERTY' },
        to: { node_id: `${request.template.uri}#expression:${expression.start}:${expression.end}`, role: 'RENDER_EXPRESSION' },
        original_anchor: anchor(request.template.uri, templateSource, expression.start, expression.end),
        definition_anchor: anchor(definition.uri, definition.source, definition.start, definition.end),
        source_read_anchor: anchor(definition.uri, definition.source, read.start, read.end),
        virtual_mapping: request.mapping,
        support: { outcome: 'SCOPED_ROLE', operation: 'glint.definition+typescript.direct-qualified-read' },
        supports: [...SUPPORTS],
        does_not_support: [...DOES_NOT_SUPPORT],
      };
      return { status: 'SCOPED_ROLE', observations: [observation], coverage: { status: 'BOUNDED', boundary: BOUNDARY } };
    },
  });
}
