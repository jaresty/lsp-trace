const PINNED_TYPESCRIPT = '5.9.2';

function validateDependencies({ ts, versions }) {
  if (
    versions?.typescript !== PINNED_TYPESCRIPT ||
    typeof ts?.createLanguageService !== 'function'
  ) throw new Error('ASSERT_PASSES_CALLBACK_PINNED_QUALIFIED_ANALYZERS');
}

function point(source, offset) {
  const lines = source.slice(0, offset).split('\n');
  return { line: lines.length - 1, character: lines.at(-1).length };
}

function anchor(document, mapped) {
  return {
    document_id: document.document_id,
    uri: mapped.uri,
    revision: document.revision,
    blob: document.blob,
    range: { start: point(document.source, mapped.start), end: point(document.source, mapped.end) },
  };
}

function exactMap(document, start, end) {
  const mapped = document.generated?.mapToOriginal?.({ start, end });
  if (
    !mapped || mapped.uri !== document.uri ||
    mapped.roundTrip?.start !== start || mapped.roundTrip?.end !== end ||
    !Number.isInteger(mapped.start) || !Number.isInteger(mapped.end) ||
    mapped.start < 0 || mapped.end < mapped.start || mapped.end > document.source.length
  ) return undefined;
  return mapped;
}

function nodeID(role, value, mapped) {
  return `PASSES_CALLBACK:${role}:uri=${value.uri};bytes=${Buffer.byteLength(value.source.slice(0, mapped.start))}-${Buffer.byteLength(value.source.slice(0, mapped.end))}`;
}

function stableKey(observation) {
  return `${observation.from.node_id}\u0000${observation.to.node_id}`;
}

function isUnsafeType(ts, type) {
  return (type.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)) !== 0;
}

function sourceFileHost(ts, fileName, source) {
  return {
    getScriptFileNames: () => [fileName],
    getScriptVersion: () => '1',
    getScriptSnapshot: (requested) => {
      if (requested === fileName) return ts.ScriptSnapshot.fromString(source);
      const library = ts.sys.readFile(requested);
      return library === undefined ? undefined : ts.ScriptSnapshot.fromString(library);
    },
    getCurrentDirectory: () => '/',
    getCompilationSettings: () => ({
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
}

function collectCalls(ts, node, calls = []) {
  if (ts.isCallExpression(node)) calls.push(node);
  ts.forEachChild(node, (child) => { collectCalls(ts, child, calls); });
  return calls;
}

export function createPassesCallbackAnalyzer(dependencies) {
  validateDependencies(dependencies);
  const { ts } = dependencies;

  return Object.freeze({
    extract({ documents = [] } = {}) {
      const observations = [];
      for (const document of [...documents].sort((a, b) => a.uri.localeCompare(b.uri))) {
        if (!document.generated?.source || !document.generated?.fileName || typeof document.generated.mapToOriginal !== 'function') {
          return { status: 'BLOCKED', reason: 'GLINT_EXACT_MAPPING_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'analysis_unavailable' } };
        }

        const service = ts.createLanguageService(sourceFileHost(ts, document.generated.fileName, document.generated.source));
        try {
          const program = service.getProgram();
          const sourceFile = program?.getSourceFile(document.generated.fileName);
          if (!program || !sourceFile) return { status: 'BLOCKED', reason: 'TYPESCRIPT_PROGRAM_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'analysis_unavailable' } };
          const checker = program.getTypeChecker();

          for (const call of collectCalls(ts, sourceFile)) {
            const signature = checker.getResolvedSignature(call);
            if (!signature) continue;
            const parameters = signature.getParameters();

            for (let index = 0; index < call.arguments.length; index++) {
              const argument = call.arguments[index];
              if (!ts.isIdentifier(argument)) continue;
              const lastParameter = parameters.at(-1);
              const lastDeclaration = lastParameter?.valueDeclaration ?? lastParameter?.declarations?.[0];
              const parameter = index < parameters.length
                ? parameters[index]
                : lastDeclaration?.dotDotDotToken ? lastParameter : undefined;
              const parameterDeclaration = parameter?.valueDeclaration ?? parameter?.declarations?.[0];
              if (!parameter || !parameterDeclaration || !ts.isParameter(parameterDeclaration) || !ts.isIdentifier(parameterDeclaration.name)) continue;

              const definitions = service.getDefinitionAtPosition(document.generated.fileName, argument.getStart(sourceFile)) ?? [];
              const localDefinition = definitions.find(({ fileName }) => fileName === document.generated.fileName);
              if (!localDefinition) continue;
              const argumentType = checker.getTypeAtLocation(argument);
              const parameterType = checker.getTypeOfSymbolAtLocation(parameter, parameterDeclaration);
              if (isUnsafeType(ts, argumentType) || isUnsafeType(ts, parameterType)) continue;
              if (checker.getSignaturesOfType(argumentType, ts.SignatureKind.Call).length === 0) continue;
              if (checker.getSignaturesOfType(parameterType, ts.SignatureKind.Call).length === 0) continue;
              if (!checker.isTypeAssignableTo(argumentType, parameterType)) continue;

              const fromMapped = exactMap(document, argument.getStart(sourceFile), argument.end);
              const toMapped = exactMap(document, parameterDeclaration.name.getStart(sourceFile), parameterDeclaration.name.end);
              if (!fromMapped || !toMapped) {
                return { status: 'BLOCKED', reason: 'GLINT_EXACT_MAPPING_UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: 'analysis_unavailable' } };
              }
              const fromAnchor = anchor(document, fromMapped);
              const toAnchor = anchor(document, toMapped);
              observations.push({
                kind: 'PASSES_CALLBACK',
                from: { node_id: nodeID('callable-reference', document, fromMapped), role: 'CALLABLE_REFERENCE' },
                to: { node_id: nodeID('callable-parameter', document, toMapped), role: 'CALLBACK_PARAMETER' },
                original_anchor: fromAnchor,
                supports: ['source_dependency_relation'],
                does_not_support: ['callback_invocation', 'runtime_execution', 'repaint', 'feature_identity', 'whole_source_completeness'],
              });
            }
          }
        } finally {
          service.dispose();
        }
      }
      observations.sort((a, b) => stableKey(a).localeCompare(stableKey(b)));
      return {
        status: 'SUPPORTED',
        observations,
        coverage: { status: 'BOUNDED', reason: 'qualified_static_callback_passage' },
      };
    },
  });
}
