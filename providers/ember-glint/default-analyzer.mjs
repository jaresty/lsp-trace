import { existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { analyzeProject, loadConfig } from '@glint/core';
import * as tsModule from 'typescript';

import { createAnalyzer } from './analyzer.mjs';
import { createGlintAnalyzer } from './analyzers/glint.mjs';
import { createPassesCallbackAnalyzer } from './analyzers/passes-callback.mjs';
import { createScriptSymbolExtractor } from './analyzers/script.mjs';
import { createTemplateObservationExtractor } from './analyzers/template.mjs';
import { analyzeSourceConstrainedTypeScript } from './analyzers/source-constrained-typescript.mjs';
import { createTemplateRelationAdapter } from './analyzers/template-relations.mjs';
import { createRendersFromAnalyzer } from './renders-from-analyzer.mjs';

const require = createRequire(import.meta.url);
const compiler = require('ember-source/ember-template-compiler/index.js');
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const versions = Object.freeze({
  typescript: require('typescript/package.json').version,
});

function findProjectDirectory(filename) {
  let directory = path.dirname(filename);
  while (true) {
    if (existsSync(path.join(directory, 'tsconfig.json'))) return directory;
    const parent = path.dirname(directory);
    if (parent === directory) return undefined;
    directory = parent;
  }
}

function classifyProviderRequest(request) {
  if (request.documents?.length !== 1) return { reason: 'AMBIGUOUS_PROJECT_CONTEXT' };
  const [input] = request.documents;
  let filename;
  try {
    filename = fileURLToPath(input.uri);
  } catch {
    return { reason: 'PROJECT_CONTEXT_UNAVAILABLE' };
  }
  const projectDirectory = findProjectDirectory(filename);
  if (projectDirectory && ['.gjs', '.gts'].includes(path.extname(filename))) {
    return {
      kind: 'glint', glintConfigAvailable: true, projectDirectory,
      file: path.relative(projectDirectory, filename), position: input.position, range: input.range,
    };
  }
  return { kind: 'template' };
}

export function createDefaultAnalyzer() {
  const templateExtractor = createTemplateObservationExtractor({ compiler, compilerVersion: require('ember-source/package.json').version });
  const scriptExtractor = createScriptSymbolExtractor({ ts, versions });
  const analyzer = createAnalyzer({
    templateExtractor,
    scriptExtractor,
    glintAnalyzer: createGlintAnalyzer({ loadConfig, analyzeProject }),
  });
  const callbackAnalyzer = createPassesCallbackAnalyzer({ ts, versions });
  const rendersFromAnalyzer = createRendersFromAnalyzer({ ts });
  const templateRelationAdapter = createTemplateRelationAdapter();
  const admitted = Object.freeze(['BINDS_ARGUMENT', 'INVOKES_TASK', 'PASSES_CALLBACK', 'RENDERS_FROM', 'TRIGGERS_RELOAD', 'UPDATES_STATE']);

  async function analyzeProviderRequest(request) {
    const classification = classifyProviderRequest(request);
    if (classification.reason) {
      return { outcome: 'UNAVAILABLE', observations: [], coverage: { status: 'UNKNOWN', reason: classification.reason } };
    }
    const input = request.documents[0];
    if (Array.isArray(input.upstream_template_relations)) {
      const selected = input.upstream_template_relations.filter(({ kind }) => request.relation_kinds.includes(kind));
      const result = templateRelationAdapter.normalize(selected);
      return {
        outcome: result.status === 'BLOCKED' ? 'UNAVAILABLE' : result.observations.length === 0 ? 'EMPTY' : 'COMPLETE',
        observations: result.observations,
        coverage: result.coverage,
      };
    }
    const document = {
      document_id: 'original', uri: input.uri, revision: input.revision,
      blob: input.digest.replace(/^sha256:/, ''), source: input.source,
    };
    const observations = [];
    let blocked;
    let resultCoverage;
    for (const relation of request.relation_kinds) {
      let result;
      if (relation === 'BINDS_ARGUMENT') {
        result = analyzer.analyze(classification.kind === 'glint'
          ? { ...classification, relationKinds: [relation] }
          : { kind: 'template', source: input.source, document, relationKinds: [relation] });
      } else if (relation === 'PASSES_CALLBACK') {
        const source = input?.source ?? '';
        result = callbackAnalyzer.extract({ documents: [{ ...document, generated: {
          source, fileName: '/__lsp_trace__/seed.ts',
          mapToOriginal: ({ start, end }) => ({ uri: input.uri, start, end, roundTrip: { start, end } }),
        } }] });
      } else if (relation === 'INVOKES_TASK' || relation === 'TRIGGERS_RELOAD') {
        result = await analyzeSourceConstrainedTypeScript({ ...request, relation_kinds: [relation] });
      } else if (relation === 'UPDATES_STATE') {
        result = scriptExtractor.extract({ documents: [{ ...document, language: 'typescript' }] });
      } else if (relation === 'RENDERS_FROM') {
        try { result = rendersFromAnalyzer.analyze(JSON.parse(input?.source ?? '')); }
        catch { result = { status: 'BLOCKED', reason: 'QUALIFIED_RENDERS_FROM_INPUT_REQUIRED', observations: [], coverage: { status: 'UNKNOWN', reason: 'ANALYSIS_UNAVAILABLE' } }; }
      }
      if (!result || ['BLOCKED', 'FAILED', 'UNSUPPORTED'].includes(result.status)) blocked = result?.reason ?? 'RELATION_NOT_SUPPORTED';
      resultCoverage = result?.coverage;
      observations.push(...(result?.observations ?? []).filter(({ kind }) => kind === relation));
    }
    return {
      outcome: blocked ? 'UNAVAILABLE' : observations.length === 0 ? 'EMPTY' : 'COMPLETE',
      observations,
      coverage: blocked
        ? { status: 'UNKNOWN', reason: blocked }
        : { ...resultCoverage, status: 'BOUNDED', denominator: [document.uri], covered: [document.uri] },
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
