import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

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

export function createGlintAnalyzer({ loadConfig, analyzeProject }) {
  if (typeof loadConfig !== 'function' || typeof analyzeProject !== 'function') {
    throw new TypeError('supported Glint loadConfig and analyzeProject APIs are required');
  }

  return Object.freeze({
    analyze({ projectDirectory, file = 'fixtures/component.gts', token = 'this.itemCount' } = {}) {
      const config = projectDirectory && path.join(projectDirectory, 'tsconfig.json');
      if (!config || !existsSync(config)) {
        return { status: 'BLOCKED', reason: 'GLINT_CONFIG_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
      }

      let analysis;
      try {
        loadConfig(projectDirectory);
        const filename = path.resolve(projectDirectory, file);
        const source = readFileSync(filename, 'utf8');
        const start = source.indexOf(token);
        if (start < 0) return { status: 'BLOCKED', reason: 'GLINT_QUERY_TOKEN_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
        const end = start + token.length;
        analysis = analyzeProject(projectDirectory);
        const transformed = analysis.transformManager.getTransformedRange(filename, start, end);
        const roundTrip = analysis.transformManager.getOriginalRange(transformed.transformedFileName, transformed.transformedStart, transformed.transformedEnd);
        const uri = pathToFileURL(filename).href;
        const query = point(source, start + token.indexOf('.') + 1);
        const definitions = analysis.languageServer.getDefinition(uri, query);
        if (roundTrip.originalFileName !== filename || roundTrip.originalStart !== start || roundTrip.originalEnd !== end || definitions.length !== 1) {
          return { status: 'BLOCKED', reason: 'GLINT_QUALIFIED_OPERATION_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
        }
        return {
          status: 'SCOPED_ROLE',
          observations: [{
            kind: 'TYPED_TEMPLATE_DEFINITION',
            original_anchor: anchor(uri, source, start, end),
            virtual_mapping: {
              uri: pathToFileURL(transformed.transformedFileName).href,
              original: { start, end },
              virtual: { start: transformed.transformedStart, end: transformed.transformedEnd },
            },
            support: { outcome: 'SCOPED_ROLE', operation: 'analyzeProject.getDefinition' },
          }],
          coverage: { status: 'BOUNDED', reason: 'FROZEN_GLINT_QUERY' },
        };
      } catch (error) {
        return { status: 'BLOCKED', reason: 'GLINT_ANALYSIS_UNAVAILABLE', detail: String(error?.message ?? error), observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } };
      } finally {
        analysis?.shutdown();
      }
    },
  });
}
