import { createRequire } from 'node:module';
import { analyzeProject, loadConfig } from '@glint/core';
import Parser from 'tree-sitter';
import TypeScriptLanguages from 'tree-sitter-typescript';
import * as tsModule from 'typescript';

import { createAnalyzer } from './analyzer.mjs';
import { createGlintAnalyzer } from './analyzers/glint.mjs';
import { createPassesCallbackAnalyzer } from './analyzers/passes-callback.mjs';
import { createScriptSymbolExtractor } from './analyzers/script.mjs';
import { createTemplateObservationExtractor } from './analyzers/template.mjs';
import { createRendersFromAnalyzer } from './renders-from-analyzer.mjs';

const require = createRequire(import.meta.url);
const compiler = require('ember-source/ember-template-compiler/index.js');
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const versions = Object.freeze({
  typescript: require('typescript/package.json').version,
  treeSitter: require('tree-sitter/package.json').version,
  treeSitterTypeScript: require('tree-sitter-typescript/package.json').version,
});

export function createDefaultAnalyzer() {
  const templateExtractor = createTemplateObservationExtractor({ compiler, compilerVersion: require('ember-source/package.json').version });
  const scriptExtractor = createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions });
  const analyzer = createAnalyzer({
    templateExtractor,
    scriptExtractor,
    glintAnalyzer: createGlintAnalyzer({ loadConfig, analyzeProject }),
  });
  const callbackAnalyzer = createPassesCallbackAnalyzer({ ts, Parser, TypeScriptLanguages, versions });
  const rendersFromAnalyzer = createRendersFromAnalyzer({ ts });
  const admitted = Object.freeze(['BINDS_ARGUMENT', 'PASSES_CALLBACK', 'UPDATES_STATE', 'RENDERS_FROM']);

  function analyzeProviderRequest(request) {
    const input = request.documents?.[0];
    const document = input && {
      document_id: 'original', uri: input.uri, revision: input.revision,
      blob: input.digest.replace(/^sha256:/, ''), source: input.source,
    };
    const observations = [];
    let blocked;
    for (const relation of request.relation_kinds) {
      let result;
      if (relation === 'BINDS_ARGUMENT') {
        result = analyzer.analyze({ kind: 'template', source: input?.source ?? '', document, relationKinds: [relation] });
      } else if (relation === 'PASSES_CALLBACK') {
        const source = input?.source ?? '';
        result = callbackAnalyzer.extract({ documents: [{ ...document, generated: {
          source, fileName: '/__lsp_trace__/seed.ts',
          mapToOriginal: ({ start, end }) => ({ uri: input.uri, start, end, roundTrip: { start, end } }),
        } }] });
      } else if (relation === 'UPDATES_STATE') {
        result = scriptExtractor.extract({ documents: [{ ...document, language: 'typescript' }] });
      } else if (relation === 'RENDERS_FROM') {
        try { result = rendersFromAnalyzer.analyze(JSON.parse(input?.source ?? '')); }
        catch { result = { status: 'BLOCKED', reason: 'QUALIFIED_RENDERS_FROM_INPUT_REQUIRED', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } }; }
      }
      if (!result || ['BLOCKED', 'FAILED', 'UNSUPPORTED'].includes(result.status)) blocked = result?.reason ?? 'RELATION_NOT_SUPPORTED';
      observations.push(...(result?.observations ?? []).filter(({ kind }) => kind === relation));
    }
    return {
      outcome: blocked ? 'UNAVAILABLE' : observations.length === 0 ? 'EMPTY' : 'COMPLETE',
      observations,
      coverage: blocked
        ? { status: 'UNKNOWN', reason: blocked }
        : { status: 'BOUNDED', denominator: [document?.uri].filter(Boolean), covered: [document?.uri].filter(Boolean) },
    };
  }

  return Object.freeze({
    id: 'ember-glint-default@1',
    relationKinds: admitted,
    languages: ['glimmer-js'],
    frameworks: ['ember'],
    supportedRelations: admitted,
    analyze(request, options) {
      if (request?.schema === 'lsp-trace.provider-request.v1') return analyzeProviderRequest(request);
      if (request?.kind === 'renders-from') return rendersFromAnalyzer.analyze(request);
      return analyzer.analyze(request, options);
    },
  });
}
