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
  return createAnalyzer({
    templateExtractor: createTemplateObservationExtractor({ compiler, compilerVersion: require('ember-source/package.json').version }),
    scriptExtractor: createScriptSymbolExtractor({ ts, Parser, TypeScriptLanguages, versions }),
    glintAnalyzer: createGlintAnalyzer({ loadConfig, analyzeProject }),
  });
}
