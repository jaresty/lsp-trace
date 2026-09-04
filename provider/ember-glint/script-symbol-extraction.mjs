export const PINNED_ANALYZERS = Object.freeze({
  typescript: '5.9.2',
  treeSitter: '0.21.1',
  treeSitterTypeScript: '0.23.2',
});

const SUPPORTED_LANGUAGES = new Set(['javascript', 'javascriptreact', 'typescript', 'typescriptreact']);
const IDENTIFIER_TYPES = ['identifier', 'property_identifier', 'shorthand_property_identifier_pattern'];

function analyzerIdentity() {
  return {
    typescript: PINNED_ANALYZERS.typescript,
    tree_sitter: PINNED_ANALYZERS.treeSitter,
    tree_sitter_typescript: PINNED_ANALYZERS.treeSitterTypeScript,
  };
}

function result(observations, coverage) {
  return {
    schema_version: 'lsp-trace.script-symbol-observations.v1',
    analyzer: analyzerIdentity(),
    observations,
    coverage,
  };
}

function validateDependencies({ ts, Parser, TypeScriptLanguages, versions }) {
  const actual = versions ?? {};
  if (
    actual.typescript !== PINNED_ANALYZERS.typescript ||
    actual.treeSitter !== PINNED_ANALYZERS.treeSitter ||
    actual.treeSitterTypeScript !== PINNED_ANALYZERS.treeSitterTypeScript ||
    typeof ts?.createLanguageService !== 'function' ||
    typeof Parser !== 'function' ||
    !TypeScriptLanguages?.typescript
  ) {
    throw new Error('ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS: exact qualified analyzer versions and APIs are required');
  }
}

function fileNameFor(document, index) {
  try {
    const url = new URL(document.uri);
    if (url.protocol === 'file:') return decodeURIComponent(url.pathname);
  } catch {
    // The URI remains an opaque original identity; TypeScript receives a stable synthetic path.
  }
  const suffix = document.language.startsWith('javascript') ? '.js' : '.ts';
  return `/__lsp_trace_original__/${String(index).padStart(6, '0')}${suffix}`;
}

function byteOffset(source, utf16Offset) {
  return Buffer.byteLength(source.slice(0, utf16Offset), 'utf8');
}

function anchorFromNode(document, node) {
  return {
    uri: document.uri,
    bytes: { start: byteOffset(document.source, node.startIndex), end: byteOffset(document.source, node.endIndex) },
    range: {
      start: { row: node.startPosition.row, column: node.startPosition.column },
      end: { row: node.endPosition.row, column: node.endPosition.column },
    },
    text: document.source.slice(node.startIndex, node.endIndex),
  };
}

function anchorForSpan(document, identifiers, span) {
  const start = byteOffset(document.source, span.start);
  const end = byteOffset(document.source, span.start + span.length);
  const node = identifiers.find((candidate) => (
    byteOffset(document.source, candidate.startIndex) === start &&
    byteOffset(document.source, candidate.endIndex) === end
  ));
  if (!node) return undefined;
  return anchorFromNode(document, node);
}

function stableObservationKey(observation) {
  const reference = observation.reference.anchor;
  const definition = observation.definition.anchor;
  return [reference.uri, reference.bytes.start, reference.bytes.end, definition.uri, definition.bytes.start, definition.bytes.end, observation.symbol].join('\u0000');
}

export function createScriptSymbolExtractor(dependencies) {
  validateDependencies(dependencies);
  const { ts, Parser, TypeScriptLanguages } = dependencies;

  return {
    extract({ documents = [] } = {}) {
      const ordered = [...documents].sort((a, b) => a.uri.localeCompare(b.uri));
      const unsupported = ordered.filter((document) => !SUPPORTED_LANGUAGES.has(document.language));
      const supported = ordered.filter((document) => SUPPORTED_LANGUAGES.has(document.language));

      if (supported.length === 0) {
        return result([], {
          status: 'UNSUPPORTED',
          reason: 'unsupported_language',
          documents: unsupported.map((document) => document.uri),
        });
      }

      const records = supported.map((document, index) => ({
        document,
        fileName: fileNameFor(document, index),
        version: '1',
      }));
      const byFileName = new Map(records.map((record) => [record.fileName, record]));

      try {
        for (const record of records) {
          const parser = new Parser();
          const language = record.document.language === 'typescriptreact' ? TypeScriptLanguages.tsx : TypeScriptLanguages.typescript;
          parser.setLanguage(language);
          record.tree = parser.parse(record.document.source);
          if (record.tree.rootNode.hasError) {
            return result([], { status: 'FAILED', reason: 'syntax_parse_failure', documents: [record.document.uri] });
          }
          record.identifiers = record.tree.rootNode.descendantsOfType(IDENTIFIER_TYPES);
        }
      } catch {
        return result([], {
          status: 'FAILED',
          reason: 'syntax_analyzer_failure',
          documents: supported.map((document) => document.uri),
        });
      }

      const host = {
        getScriptFileNames: () => records.map((record) => record.fileName),
        getScriptVersion: (fileName) => byFileName.get(fileName)?.version ?? '0',
        getScriptSnapshot: (fileName) => {
          const source = byFileName.get(fileName)?.document.source;
          if (source !== undefined) return ts.ScriptSnapshot.fromString(source);
          const library = ts.sys.readFile(fileName);
          return library === undefined ? undefined : ts.ScriptSnapshot.fromString(library);
        },
        getCurrentDirectory: () => '/',
        getCompilationSettings: () => ({
          allowJs: true,
          checkJs: true,
          strict: true,
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
          moduleResolution: ts.ModuleResolutionKind.Bundler,
        }),
        getDefaultLibFileName: (options) => ts.getDefaultLibFilePath(options),
        fileExists: ts.sys.fileExists,
        readFile: ts.sys.readFile,
        readDirectory: ts.sys.readDirectory,
      };

      let service;
      try {
        service = ts.createLanguageService(host);
        const observations = [];
        const seen = new Set();
        let omittedDefinitions = 0;

        for (const record of records) {
          for (const identifier of record.identifiers) {
            const definitions = service.getDefinitionAtPosition(record.fileName, identifier.startIndex) ?? [];
            for (const definition of definitions) {
              const target = byFileName.get(definition.fileName);
              if (!target) {
                omittedDefinitions++;
                continue;
              }
              const definitionAnchor = anchorForSpan(target.document, target.identifiers, definition.textSpan);
              if (!definitionAnchor) {
                omittedDefinitions++;
                continue;
              }
              const referenceAnchor = anchorFromNode(record.document, identifier);
              if (
                referenceAnchor.uri === definitionAnchor.uri &&
                referenceAnchor.bytes.start === definitionAnchor.bytes.start &&
                referenceAnchor.bytes.end === definitionAnchor.bytes.end
              ) continue;
              const observation = {
                kind: 'SCRIPT_SYMBOL_DEFINITION',
                symbol: identifier.text,
                reference: { role: 'SYMBOL_REFERENCE', anchor: referenceAnchor },
                definition: { role: 'SYMBOL_DEFINITION', anchor: definitionAnchor },
                resolution: {
                  provider: `typescript@${PINNED_ANALYZERS.typescript}`,
                  operation: 'getDefinitionAtPosition',
                },
              };
              const key = stableObservationKey(observation);
              if (!seen.has(key)) {
                seen.add(key);
                observations.push(observation);
              }
            }
          }
        }

        observations.sort((a, b) => stableObservationKey(a).localeCompare(stableObservationKey(b)));
        const coverage = unsupported.length > 0 || omittedDefinitions > 0
          ? {
              status: 'PARTIAL',
              reason: unsupported.length > 0 ? 'unsupported_documents' : 'definition_anchor_unavailable',
              documents: supported.map((document) => document.uri),
              unsupported_documents: unsupported.map((document) => document.uri),
              omitted_definitions: omittedDefinitions,
            }
          : {
              status: 'BOUNDED',
              reason: 'static_script_syntax',
              documents: supported.map((document) => document.uri),
            };
        return result(observations, coverage);
      } catch {
        return result([], {
          status: 'FAILED',
          reason: 'symbol_analyzer_failure',
          documents: supported.map((document) => document.uri),
        });
      } finally {
        service?.dispose();
      }
    },
  };
}
