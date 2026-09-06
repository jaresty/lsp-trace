import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

function point(source, offset) {
  const lines = source.slice(0, offset).split('\n');
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
  return {
    status: 'BLOCKED',
    reason,
    ...(detail ? { detail } : {}),
    observations: [],
    coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' },
  };
}

export function createGlintAnalyzer({ loadConfig, analyzeProject }) {
  if (typeof loadConfig !== 'function' || typeof analyzeProject !== 'function') {
    throw new TypeError('supported Glint loadConfig and analyzeProject APIs are required');
  }

  return Object.freeze({
    analyze({ projectDirectory, file, position, range } = {}) {
      const config = projectDirectory && path.join(projectDirectory, 'tsconfig.json');
      if (!config || !existsSync(config)) return blocked('GLINT_CONFIG_UNAVAILABLE');
      if (typeof file !== 'string' || file.length === 0 || !position || !range?.start || !range?.end) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');

      let analysis;
      try {
        loadConfig(projectDirectory);
        const filename = path.resolve(projectDirectory, file);
        const source = readFileSync(filename, 'utf8');
        const start = offset(source, range.start);
        const end = offset(source, range.end);
        if (start === undefined || end === undefined || end <= start || offset(source, position) === undefined) return blocked('GLINT_REQUEST_COORDINATES_UNAVAILABLE');

        analysis = analyzeProject(projectDirectory);
        let transformed;
        let roundTrip;
        try {
          transformed = analysis.transformManager.getTransformedRange(filename, start, end);
          if (!transformed) return blocked('GLINT_MAPPING_UNAVAILABLE');
          roundTrip = analysis.transformManager.getOriginalRange(transformed.transformedFileName, transformed.transformedStart, transformed.transformedEnd);
        } catch (error) {
          return blocked('GLINT_MAPPING_UNAVAILABLE', String(error?.message ?? error));
        }
        if (!roundTrip || roundTrip.originalFileName !== filename || roundTrip.originalStart !== start || roundTrip.originalEnd !== end) return blocked('GLINT_MAPPING_UNAVAILABLE');

        const uri = pathToFileURL(filename).href;
        const definitions = analysis.languageServer.getDefinition(uri, position);
        const type = analysis.languageServer.getHover(uri, position);
        if (!Array.isArray(definitions)) return blocked('GLINT_QUALIFIED_OPERATION_UNAVAILABLE');
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
            definitions,
            type,
            support: { outcome: 'SCOPED_ROLE', operation: 'analyzeProject.getDefinition+getHover' },
          }],
          coverage: { status: 'BOUNDED', reason: 'REQUESTED_GLINT_QUERY' },
        };
      } catch (error) {
        return blocked('GLINT_ANALYSIS_UNAVAILABLE', String(error?.message ?? error));
      } finally {
        analysis?.shutdown();
      }
    },
  });
}
