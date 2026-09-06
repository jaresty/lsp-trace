export const SUPPORTED_RELATIONS = Object.freeze([
  'BINDS_ARGUMENT',
  'RENDERS_FROM',
  'SCRIPT_SYMBOL_DEFINITION',
  'TYPED_TEMPLATE_DEFINITION',
]);

function compareObservations(left, right) {
  const a = left.original_anchor ?? left.reference?.anchor ?? {};
  const b = right.original_anchor ?? right.reference?.anchor ?? {};
  return String(a.uri ?? '').localeCompare(String(b.uri ?? ''))
    || (a.bytes?.start ?? 0) - (b.bytes?.start ?? 0)
    || (a.bytes?.end ?? 0) - (b.bytes?.end ?? 0)
    || String(left.kind).localeCompare(String(right.kind));
}

function supportedObservations(result) {
  return (result.observations ?? [])
    .filter(({ kind }) => SUPPORTED_RELATIONS.includes(kind))
    .sort(compareObservations);
}

export function createAnalyzer({ templateExtractor, scriptExtractor, glintAnalyzer } = {}) {
  if (typeof templateExtractor?.extract !== 'function') throw new TypeError('templateExtractor.extract is required');
  if (typeof scriptExtractor?.extract !== 'function') throw new TypeError('scriptExtractor.extract is required');
  if (typeof glintAnalyzer?.analyze !== 'function') throw new TypeError('glintAnalyzer.analyze is required');

  return Object.freeze({
    supportedRelations: SUPPORTED_RELATIONS,
    analyze(request = {}) {
      if (request.kind === 'glint' && request.glintConfigAvailable !== true) {
        return {
          status: 'BLOCKED',
          reason: 'GLINT_CONFIG_UNAVAILABLE',
          observations: [],
          coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
        };
      }

      const result = request.kind === 'template'
        ? templateExtractor.extract(request)
        : request.kind === 'script'
          ? scriptExtractor.extract(request)
          : request.kind === 'glint'
            ? glintAnalyzer.analyze(request)
            : { status: 'UNSUPPORTED', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYZER_KIND_UNSUPPORTED' } };

      return { ...result, observations: supportedObservations(result) };
    },
  });
}
