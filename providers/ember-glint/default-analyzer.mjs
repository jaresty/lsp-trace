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
    const results = [];
    for (const relation of request.relation_kinds) {
      let result;
      try {
        if (relation === 'BINDS_ARGUMENT' || relation === 'RENDERS_FROM') {
          result = analyzer.analyze(classification.kind === 'glint'
            ? { ...classification, document: { ...document, source: input.source }, relationKinds: [relation] }
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
        }
      } catch (error) {
        result = { outcome: 'FAILED', observations: [], coverage: { status: 'UNAVAILABLE' }, blocker: error instanceof Error ? error.message : String(error) };
      }
      // The compiler analyzer uses outcome/blocker; extractors use status/reason.
      const status = result?.outcome ?? result?.status ?? (result?.coverage?.status === 'BOUNDED'
        ? result.observations?.length ? 'COMPLETE' : 'EMPTY'
        : result?.coverage?.status) ?? 'UNSUPPORTED';
      const outcome = status === 'UNSUPPORTED' || (!result?.outcome && status === 'BLOCKED') ? 'UNAVAILABLE' : status;
      const reason = result?.blocker ?? result?.reason ?? result?.coverage?.reason;
      results.push({ relation, outcome, coverage: result?.coverage ?? { status: 'UNKNOWN' }, ...(reason ? { reason } : {}) });
      observations.push(...(result?.observations ?? []).filter(({ kind }) => kind === relation));
    }
    results.sort((a, b) => a.relation.localeCompare(b.relation));
    const unresolved = results.filter(({ outcome, coverage }) =>
      ['BLOCKED', 'UNAVAILABLE', 'FAILED', 'PARTIAL', 'BOUNDED'].includes(outcome) || ['UNKNOWN', 'UNAVAILABLE', 'PARTIAL'].includes(coverage.status));
    const outcome = unresolved.length
      ? observations.length ? 'PARTIAL' : ['FAILED', 'BLOCKED', 'UNAVAILABLE', 'PARTIAL', 'BOUNDED'].find(status => unresolved.some(result => result.outcome === status)) ?? 'UNAVAILABLE'
      : observations.length ? 'COMPLETE' : 'EMPTY';
    return {
      outcome, observations,
      coverage: unresolved.length
        ? {
          status: observations.length ? 'PARTIAL' : unresolved[0].coverage.status,
          denominator: [document.uri], covered: [],
          reason: unresolved.length === 1
            ? unresolved[0].reason ?? unresolved[0].outcome
            : unresolved.map(result => `${result.relation}: ${result.reason ?? result.outcome}`).join('; '),
          relations: results,
        }
        : { ...results.at(-1)?.coverage, status: 'BOUNDED', denominator: [document.uri], covered: [document.uri] },
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
      return analyzer.analyze(request, options);
    },
  });
}
