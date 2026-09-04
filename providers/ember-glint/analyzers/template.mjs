export const EMBER_TEMPLATE_COMPILER_VERSION = '7.2.0';
export const SUPPORTED_RELATION_KINDS = Object.freeze(['BINDS_ARGUMENT']);

const COVERAGE_BOUNDARY = 'named component argument attributes whose value is a mustache PathExpression';
const DOES_NOT_SUPPORT = Object.freeze([
  'callback_execution',
  'task_execution',
  'runtime',
  'repaint',
  'feature_identity',
  'rendering_execution',
  'source_completeness',
]);
const SUPPORTS = Object.freeze([
  'bounded_source_structure',
  'exact_source_anchor',
  'typed_static_relation',
]);

function cloneList(values) {
  return [...values];
}

function point(location) {
  return { line: location.line - 1, character: location.column };
}

function range(location) {
  return { start: point(location.start), end: point(location.end) };
}

function originalAnchor(document, location) {
  return {
    document_id: document.document_id,
    uri: document.uri,
    revision: document.revision,
    blob: document.blob,
    range: range(location),
  };
}

function validateDocument(document) {
  if (!document || ['document_id', 'uri', 'revision', 'blob'].some((key) => typeof document[key] !== 'string' || document[key] === '')) {
    throw new TypeError('document requires non-empty document_id, uri, revision, and blob');
  }
}

function visit(value, callback, seen = new Set()) {
  if (!value || typeof value !== 'object' || seen.has(value)) return;
  seen.add(value);
  callback(value);
  for (const [key, child] of Object.entries(value)) {
    if (key === 'loc') continue;
    if (Array.isArray(child)) {
      for (const item of child) visit(item, callback, seen);
    } else {
      visit(child, callback, seen);
    }
  }
}

function compareByAnchor(left, right) {
  const a = left.original_anchor.range;
  const b = right.original_anchor.range;
  return a.start.line - b.start.line
    || a.start.character - b.start.character
    || a.end.line - b.end.line
    || a.end.character - b.end.character
    || left.to.node_id.localeCompare(right.to.node_id)
    || left.from.node_id.localeCompare(right.from.node_id);
}

export function createTemplateObservationExtractor({ compiler, compilerVersion } = {}) {
  if (compilerVersion !== EMBER_TEMPLATE_COMPILER_VERSION) {
    throw new Error(`template observation extraction requires ember-source ${EMBER_TEMPLATE_COMPILER_VERSION}; received ${compilerVersion ?? 'unknown'}`);
  }
  if (!compiler || typeof compiler._preprocess !== 'function') {
    throw new TypeError('pinned Ember Template Compiler must expose _preprocess');
  }

  return Object.freeze({
    supportedRelationKinds: SUPPORTED_RELATION_KINDS,
    extract({ source, document, relationKinds = SUPPORTED_RELATION_KINDS } = {}) {
      validateDocument(document);
      if (typeof source !== 'string') throw new TypeError('source must be a string');
      if (!Array.isArray(relationKinds) || relationKinds.some((kind) => !SUPPORTED_RELATION_KINDS.includes(kind))) {
        return {
          status: 'UNSUPPORTED',
          requested_relation_kinds: Array.isArray(relationKinds) ? cloneList(relationKinds) : [],
          supported_relation_kinds: cloneList(SUPPORTED_RELATION_KINDS),
          observations: [],
          coverage: { status: 'UNKNOWN', reason: 'RELATION_NOT_SUPPORTED' },
        };
      }

      let ast;
      try {
        ast = compiler._preprocess(source, { moduleName: document.uri });
      } catch (error) {
        return {
          status: 'FAILURE',
          failure: { kind: 'TEMPLATE_PARSE_FAILED', message: error instanceof Error ? error.message : String(error) },
          observations: [],
          coverage: { status: 'UNKNOWN', reason: 'TEMPLATE_PARSE_FAILED' },
        };
      }

      const observations = [];
      const unsupported = [];
      let examined = 0;
      visit(ast, (node) => {
        if (node.type !== 'ElementNode' || typeof node.tag !== 'string') return;
        for (const attribute of node.attributes ?? []) {
          if (!attribute.name?.startsWith('@') || !attribute.loc) continue;
          examined += 1;
          const path = attribute.value?.type === 'MustacheStatement' ? attribute.value.path : null;
          if (path?.type !== 'PathExpression' || typeof path.original !== 'string') {
            unsupported.push({
              reason: 'UNQUALIFIED_ATTRIBUTE_VALUE',
              original_anchor: originalAnchor(document, attribute.loc),
            });
            continue;
          }
          observations.push({
            kind: 'BINDS_ARGUMENT',
            from: { node_id: `path:${path.original}`, role: 'SOURCE_EXPRESSION' },
            to: { node_id: `argument:${node.tag}:${attribute.name}`, role: 'BOUND_ARGUMENT' },
            original_anchor: originalAnchor(document, attribute.loc),
            supports: cloneList(SUPPORTS),
            does_not_support: cloneList(DOES_NOT_SUPPORT),
          });
        }
      });
      observations.sort(compareByAnchor);
      unsupported.sort(compareByAnchor);

      return {
        status: 'SUPPORTED',
        coverage: {
          status: 'BOUNDED',
          boundary: COVERAGE_BOUNDARY,
          examined,
          emitted: observations.length,
          unsupported: unsupported.length,
        },
        observations,
        unsupported,
      };
    },
  });
}
