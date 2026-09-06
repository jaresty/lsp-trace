const KINDS = Object.freeze(['BINDS_ARGUMENT', 'PASSES_CALLBACK', 'RENDERS_FROM']);
const OPERATIONS = Object.freeze({
  BINDS_ARGUMENT: ['ember:', 'glint:'],
  PASSES_CALLBACK: ['glint:', 'typescript:'],
  RENDERS_FROM: ['glint:'],
});
const DOES_NOT_SUPPORT = Object.freeze(['runtime_execution', 'callback_execution', 'repaint', 'feature_identity', 'whole_source_completeness']);

function blocked() {
  return { status: 'BLOCKED', reason: 'AUTHORITATIVE_TEMPLATE_RELATION_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
}

function authoritative(record) {
  if (!KINDS.includes(record?.kind) || !record.identity?.from || !record.identity?.to || !record.mapping || !Array.isArray(record.anchors) || record.anchors.length === 0) return false;
  const operation = String(record.operation ?? '');
  return OPERATIONS[record.kind].every((authority) => operation.includes(authority));
}

function stableKey(record) {
  return [record.kind, record.identity.from, record.identity.to, JSON.stringify(record.mapping), JSON.stringify(record.anchors)].join('\u0000');
}

function observation(record) {
  return {
    kind: record.kind,
    from: { node_id: record.identity.from, role: record.kind === 'BINDS_ARGUMENT' ? 'SOURCE_EXPRESSION' : record.kind === 'PASSES_CALLBACK' ? 'CALLABLE_REFERENCE' : 'SOURCE_PROPERTY' },
    to: { node_id: record.identity.to, role: record.kind === 'BINDS_ARGUMENT' ? 'BOUND_ARGUMENT' : record.kind === 'PASSES_CALLBACK' ? 'CALLABLE_PARAMETER' : 'RENDER_EXPRESSION' },
    anchors: record.anchors.map((value) => structuredClone(value)),
    virtual_mapping: structuredClone(record.mapping),
    evidence_class: 'SOURCE_DERIVED_ADAPTER',
    confidence: 'EXACT',
    supports: ['source_dependency_relation'],
    does_not_support: [...DOES_NOT_SUPPORT],
    support: { operation: record.operation },
  };
}

export function createTemplateRelationAdapter() {
  return Object.freeze({
    normalize(records = []) {
      if (!Array.isArray(records) || records.some((record) => !authoritative(record))) return blocked();
      const observations = [...records].sort((left, right) => stableKey(left).localeCompare(stableKey(right))).map(observation);
      return { status: 'SUPPORTED', observations, coverage: { status: 'BOUNDED', reason: 'authoritative_upstream_template_relations' } };
    },
  });
}
