export const PINNED_ANALYZERS = Object.freeze({
  typescript: '5.9.2',
  treeSitter: '0.21.1',
  treeSitterTypeScript: '0.23.2',
});

const SUPPORTED_LANGUAGES = new Set(['javascript', 'javascriptreact', 'typescript', 'typescriptreact']);

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

function validateDependencies({ ts, versions }) {
  const actual = versions ?? {};
  if (actual.typescript !== PINNED_ANALYZERS.typescript || typeof ts?.createLanguageService !== 'function') {
    throw new Error('ASSERT_SCRIPT_SYMBOL_PINNED_ANALYZERS: exact qualified TypeScript compiler version and APIs are required');
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

function anchorForSpan(document, sourceFile, span) {
  const start = span.start;
  const end = span.start + span.length;
  const startPosition = sourceFile.getLineAndCharacterOfPosition(start);
  const endPosition = sourceFile.getLineAndCharacterOfPosition(end);
  return {
    uri: document.uri,
    bytes: { start: byteOffset(document.source, start), end: byteOffset(document.source, end) },
    range: {
      start: { row: startPosition.line, column: startPosition.character },
      end: { row: endPosition.line, column: endPosition.character },
    },
    text: document.source.slice(start, end),
  };
}

function stableObservationKey(observation) {
  if (observation.kind === 'UPDATES_STATE') {
    return [observation.kind, observation.from.node_id, observation.to.node_id, JSON.stringify(observation.original_anchor.range)].join('\u0000');
  }
  if (['INVOKES_TASK', 'TRIGGERS_RELOAD'].includes(observation.kind)) {
    const write = observation.original_anchor;
    return [write.uri, write.bytes.start, write.bytes.end, observation.kind, observation.from.symbol, observation.to.symbol].join('\u0000');
  }
  const reference = observation.reference.anchor;
  const definition = observation.definition.anchor;
  return [reference.uri, reference.bytes.start, reference.bytes.end, definition.uri, definition.bytes.start, definition.bytes.end, observation.symbol].join('\u0000');
}

function declarationSpan(sourceFile, node) {
  const start = node.getStart(sourceFile);
  const end = node.getEnd();
  return { start: byteOffset(sourceFile.text, start), end: byteOffset(sourceFile.text, end), text: sourceFile.text.slice(start, end) };
}

function sourceAnchor(document, sourceFile, node) {
  const start = node.getStart(sourceFile);
  const end = node.getEnd();
  const startPosition = sourceFile.getLineAndCharacterOfPosition(start);
  const endPosition = sourceFile.getLineAndCharacterOfPosition(end);
  return {
    uri: document.uri,
    bytes: { start: byteOffset(document.source, start), end: byteOffset(document.source, end) },
    range: {
      start: { row: startPosition.line, column: startPosition.character },
      end: { row: endPosition.line, column: endPosition.character },
    },
    text: document.source.slice(start, end),
  };
}

function providerAnchor(document, sourceFile, node) {
  const start = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
  const end = sourceFile.getLineAndCharacterOfPosition(node.getEnd());
  return {
    document_id: document.document_id,
    uri: document.uri,
    revision: document.revision,
    blob: document.blob,
    range: {
      start: { line: start.line, character: start.character },
      end: { line: end.line, character: end.character },
    },
  };
}

function declarationNodeID(role, document, sourceFile, declaration) {
  const span = declarationSpan(sourceFile, declaration);
  return `UPDATES_STATE:${role}:uri=${document.uri};bytes=${span.start}-${span.end}`;
}

function enclosingClass(ts, node) {
  for (let current = node.parent; current; current = current.parent) {
    if (ts.isClassDeclaration(current) || ts.isClassExpression(current)) return current;
  }
  return undefined;
}

function literalTypeIdentity(checker, type) {
  const property = type.getProperty('__lspTraceType');
  const declaration = property?.valueDeclaration ?? property?.declarations?.[0];
  if (!declaration) return undefined;
  const identity = checker.getTypeOfSymbolAtLocation(property, declaration);
  return identity.isStringLiteral?.() ? identity.value : undefined;
}

function typedCallObservation(ts, checker, record, sourceFile, node) {
  if (!ts.isCallExpression(node) || !ts.isPropertyAccessExpression(node.expression)) return undefined;
  const access = node.expression;
  const member = checker.getSymbolAtLocation(access.name);
  const declaration = member?.valueDeclaration ?? member?.declarations?.[0];
  if (!declaration) return undefined;
  const identity = literalTypeIdentity(checker, checker.getTypeAtLocation(access.expression));
  const specifications = {
    'ember-concurrency:Task': { member: 'perform', kind: 'INVOKES_TASK', fromRole: 'TASK_INVOKER', toRole: 'TASK' },
    '@ember-data/model:Model': { member: 'reload', kind: 'TRIGGERS_RELOAD', fromRole: 'RELOAD_TRIGGER', toRole: 'RELOADABLE_MODEL' },
  };
  const specification = specifications[identity];
  if (!specification || access.name.text !== specification.member) return undefined;
  const owner = enclosingClass(ts, node);
  return {
    kind: specification.kind,
    from: { role: specification.fromRole, symbol: owner?.name?.text ?? '<module>', declaration: owner?.name ? declarationSpan(sourceFile, owner.name) : declarationSpan(sourceFile, node) },
    to: { role: specification.toRole, symbol: access.expression.getText(sourceFile), declaration: declarationSpan(declaration.getSourceFile(), declaration) },
    original_anchor: sourceAnchor(record.document, sourceFile, node),
    resolution: { provider: `typescript@${PINNED_ANALYZERS.typescript}`, operation: 'getTypeAtLocation+getSymbolAtLocation', type_identity: identity },
    supports: ['source_dependency_relation'],
    does_not_support: ['runtime_execution', 'whole_source_completeness'],
  };
}

function writtenTarget(ts, node) {
  if (ts.isBinaryExpression(node) && node.operatorToken.kind >= ts.SyntaxKind.FirstAssignment && node.operatorToken.kind <= ts.SyntaxKind.LastAssignment) {
    return node.left;
  }
  if ((ts.isPrefixUnaryExpression(node) || ts.isPostfixUnaryExpression(node)) &&
      (node.operator === ts.SyntaxKind.PlusPlusToken || node.operator === ts.SyntaxKind.MinusMinusToken)) {
    return node.operand;
  }
  return undefined;
}

export function createScriptSymbolExtractor(dependencies) {
  validateDependencies(dependencies);
  const { ts } = dependencies;

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

        const program = service.getProgram();
        const checker = program?.getTypeChecker();

        for (const record of records) {
          const sourceFile = program?.getSourceFile(record.fileName);
          if (sourceFile && checker) {
            const visitWrites = (node) => {
              const callObservation = typedCallObservation(ts, checker, record, sourceFile, node);
              if (callObservation) {
                const key = stableObservationKey(callObservation);
                if (!seen.has(key)) {
                  seen.add(key);
                  observations.push(callObservation);
                }
              }
              const target = writtenTarget(ts, node);
              const currentClass = target && enclosingClass(ts, node);
              if (target && currentClass && currentClass.name && ts.isPropertyAccessExpression(target)) {
                const symbol = checker.getSymbolAtLocation(target.name);
                const fieldDeclaration = symbol?.declarations?.find((declaration) => ts.isPropertyDeclaration(declaration));
                if (fieldDeclaration?.parent === currentClass && byFileName.has(fieldDeclaration.getSourceFile().fileName)) {
                  const classDeclaration = currentClass.name;
                  const fieldNameDeclaration = fieldDeclaration.name;
                  const fieldSourceFile = fieldDeclaration.getSourceFile();
                  const observation = {
                    kind: 'UPDATES_STATE',
                    from: {
                      node_id: declarationNodeID('state-producer', record.document, sourceFile, classDeclaration),
                      role: 'STATE_PRODUCER',
                    },
                    to: {
                      node_id: declarationNodeID('state-value', record.document, fieldSourceFile, fieldNameDeclaration),
                      role: 'STATE_VALUE',
                    },
                    original_anchor: providerAnchor(record.document, sourceFile, node),
                    supports: ['source_dependency_relation'],
                    does_not_support: ['runtime_execution', 'runtime_mutation', 'whole_source_completeness'],
                  };
                  const key = stableObservationKey(observation);
                  if (!seen.has(key)) {
                    seen.add(key);
                    observations.push(observation);
                  }
                }
              }
              ts.forEachChild(node, visitWrites);
            };
            visitWrites(sourceFile);
          }

          const identifiers = [];
          if (sourceFile) {
            const collectIdentifiers = (node) => {
              if (ts.isIdentifier(node)) identifiers.push(node);
              ts.forEachChild(node, collectIdentifiers);
            };
            collectIdentifiers(sourceFile);
          }
          for (const identifier of identifiers) {
            const identifierStart = identifier.getStart(sourceFile);
            const definitions = service.getDefinitionAtPosition(record.fileName, identifierStart) ?? [];
            for (const definition of definitions) {
              const target = byFileName.get(definition.fileName);
              if (!target) {
                omittedDefinitions++;
                continue;
              }
              const targetSourceFile = program?.getSourceFile(definition.fileName);
              const definitionAnchor = targetSourceFile && anchorForSpan(target.document, targetSourceFile, definition.textSpan);
              if (!definitionAnchor) {
                omittedDefinitions++;
                continue;
              }
              const referenceAnchor = sourceAnchor(record.document, sourceFile, identifier);
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
