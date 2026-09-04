import { createRequire } from 'node:module';
import { analyzeProject, loadConfig } from '@glint/core';
import Parser from 'tree-sitter';
import TypeScriptLanguages from 'tree-sitter-typescript';
import * as tsModule from 'typescript';

import { createAnalyzer } from './analyzer.mjs';
import { createGlintAnalyzer } from './analyzers/glint.mjs';
import { createScriptSymbolExtractor } from './analyzers/script.mjs';
import { createTemplateObservationExtractor } from './analyzers/template.mjs';

const require = createRequire(import.meta.url);
const compiler = require('ember-source/ember-template-compiler/index.js');
const ts = tsModule['module.exports'] ?? tsModule.default ?? tsModule;
const versions = Object.freeze({
  typescript: require('typescript/package.json').version,
  treeSitter: require('tree-sitter/package.json').version,
  treeSitterTypeScript: require('tree-sitter-typescript/package.json').version,
});

export function createDefaultAnalyzer() {
  const analyzer = createAnalyzer({
    templateExtractor: createTemplateObservationExtractor({ compiler, compilerVersion: require('ember-source/package.json').version }),
    scriptExtractor: createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions }),
    glintAnalyzer: createGlintAnalyzer({ loadConfig, analyzeProject }),
  });
  return Object.freeze({
    id: 'ember-glint-default@1',
    relationKinds: ['BINDS_ARGUMENT'],
    languages: ['glimmer-js'],
    frameworks: ['ember'],
    supportedRelations: analyzer.supportedRelations,
    analyze(request, options) {
      if (request?.schema === 'lsp-trace.provider-request.v1') {
        const input = request.documents?.[0];
        const document = input && {
          document_id: 'original',
          uri: input.uri,
          revision: input.revision,
          blob: input.digest.replace(/^sha256:/, ''),
        };
        const result = analyzer.analyze({ kind: 'template', source: input?.source ?? '', document, relationKinds: request.relation_kinds });
        return {
          outcome: result.observations.length === 0 ? 'EMPTY' : 'COMPLETE',
          observations: result.observations,
          coverage: { status: 'BOUNDED', denominator: [document?.uri].filter(Boolean), covered: [document?.uri].filter(Boolean) },
        };
      }
      return analyzer.analyze(request, options);
    },
  });
}
