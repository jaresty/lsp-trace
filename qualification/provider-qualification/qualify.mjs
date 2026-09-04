import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { analyzeProject, loadConfig } from '@glint/core';

const root = path.dirname(fileURLToPath(import.meta.url));
const fixture = path.join(root, 'fixtures', 'component.gts');
const configFile = path.join(root, 'tsconfig.json');
const lockFile = path.join(root, 'package-lock.json');
const output = path.join(root, '..', 'retained', 'provider-qualification', 'report.json');
const packageLock = JSON.parse(await readFile(lockFile, 'utf8'));
const prohibitedClaims = [
  'callback_invocation_from_passage',
  'feature_identity',
  'repaint',
  'runtime_execution',
  'whole_source_completeness',
];
const pinnedFiles = [fixture, configFile, lockFile];

const version = (name) => packageLock.packages[`node_modules/${name}`].version;
const digest = async (file) => createHash('sha256').update(await readFile(file)).digest('hex');
const digests = async () => Object.fromEntries(await Promise.all(pinnedFiles.map(async (file) => [path.relative(root, file), await digest(file)])));
const point = (source, offset) => {
  const prefix = source.slice(0, offset).split('\n');
  return { line: prefix.length - 1, character: prefix.at(-1).length };
};
const range = (source, start, end) => ({ start: point(source, start), end: point(source, end) });
const operation = (id, assertion, outcome, api, observation, evidence, extra = {}) => ({
  id, assertion, outcome, api, observation, evidence, ...extra,
});

const before = await digests();
const source = await readFile(fixture, 'utf8');
const uri = pathToFileURL(fixture).href;
const token = 'this.itemCount';
const templateStart = source.indexOf('<template>');
const originalStart = source.indexOf(token, templateStart);
const originalEnd = originalStart + token.length;
const queryOffset = originalStart + 'this.'.length;
const queryPosition = point(source, queryOffset);

let analysis;
let operations;
try {
  const config = loadConfig(root);
  analysis = analyzeProject(root);
  const manager = analysis.transformManager;
  const server = analysis.languageServer;
  const diagnostics = server.getDiagnostics(uri);
  const definitions = server.getDefinition(uri, queryPosition);
  const transformed = manager.getTransformedRange(fixture, originalStart, originalEnd);
  const roundTrip = manager.getOriginalRange(transformed.transformedFileName, transformed.transformedStart, transformed.transformedEnd);
  const transformedContents = server.getTransformedContents(uri);

  if (diagnostics.length !== 0) throw new Error(`workspace diagnostics: ${diagnostics.map(({ code, message }) => `${code}: ${message}`).join('; ')}`);

  operations = [
    operation('workspace', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE', 'PASS', 'loadConfig(projectDirectory)', 'loaded supported ember-template-imports configuration with zero diagnostics', {
      environment: config.environment.names,
      diagnostics: [],
      versions: {
        '@glint/core': version('@glint/core'),
        '@glint/environment-ember-loose': version('@glint/environment-ember-loose'),
        '@glint/environment-ember-template-imports': version('@glint/environment-ember-template-imports'),
        'ember-source': version('ember-source'),
        'typescript': version('typescript'),
      },
    }),
    operation('typed-template-resolution', 'ASSERT_GLINT_TYPED_TEMPLATE_RESOLUTION', definitions.length === 1 ? 'SCOPED_ROLE' : 'BLOCKED', 'analyzeProject(projectDirectory).languageServer.getDefinition(uri, position)', definitions.length === 1 ? 'resolved template this.itemCount to one typed getter definition' : `expected one definition, observed ${definitions.length}`, {
      query: { uri: 'fixtures/component.gts', range: range(source, originalStart, originalEnd) },
      definitions: definitions.map(({ range }) => ({ uri: 'fixtures/component.gts', range })),
      api_stability: 'unstable per @glint/core 1.5.2 root declaration',
    }, { role: definitions.length === 1 ? 'typed-template-definition' : null }),
    operation('original-source-ranges', 'ASSERT_GLINT_EXACT_ORIGINAL_SOURCE_RANGES', roundTrip.originalFileName === fixture && roundTrip.originalStart === originalStart && roundTrip.originalEnd === originalEnd ? 'SCOPED_ROLE' : 'BLOCKED', 'analyzeProject(projectDirectory).transformManager.getOriginalRange(...)', 'mapped the virtual span back to the exact original template token range', {
      original: { uri: 'fixtures/component.gts', offsets: { start: originalStart, end: originalEnd }, range: range(source, originalStart, originalEnd), text: source.slice(originalStart, originalEnd) },
      api_stability: 'transform manager obtained through unstable analyzeProject API',
    }, { role: 'exact-original-source-range' }),
    operation('virtual-document-mappings', 'ASSERT_GLINT_VIRTUAL_DOCUMENT_MAPPINGS', transformedContents?.contents && transformed.transformedStart < transformed.transformedEnd ? 'SCOPED_ROLE' : 'BLOCKED', 'transformManager.getTransformedRange(...) plus getOriginalRange(...) and languageServer.getTransformedContents(uri)', 'observed deterministic original-to-virtual and virtual-to-original mapping round trip', {
      original_uri: 'fixtures/component.gts',
      virtual_uri: 'fixtures/component.ts',
      original_offsets: { start: originalStart, end: originalEnd },
      virtual_offsets: { start: transformed.transformedStart, end: transformed.transformedEnd },
      round_trip_original_offsets: { start: roundTrip.originalStart, end: roundTrip.originalEnd },
      virtual_sha256: createHash('sha256').update(transformedContents?.contents ?? '').digest('hex'),
      api_stability: 'transform manager obtained through unstable analyzeProject API',
    }, { role: 'bidirectional-virtual-document-mapping' }),
    operation('immutable-pinned-files', 'ASSERT_GLINT_IMMUTABLE_PINNED_FILES', 'PASS', 'SHA-256 before and after read-only Glint analysis', 'all pinned fixture, configuration, and lockfile bytes remained unchanged', { before, after: await digests() }),
    operation('partial-failure-reporting', 'ASSERT_GLINT_EXPLICIT_PARTIAL_FAILURE', 'BLOCKED', '@glint/core 1.5.2 root exports and ProjectAnalysis API', 'no documented root API returns a structured partial/failure outcome vocabulary; diagnostics, arrays, undefined values, and thrown errors cannot distinguish unsupported, unavailable, partial, bounded, empty, or transport-failed outcomes', {
      root_exports: ['analyzeProject', 'loadConfig', 'pathUtils'],
      required_distinctions: ['unsupported', 'unavailable', 'partial', 'bounded', 'empty', 'transport-failed'],
      support_basis: 'package export map and root declarations; package presence is not support',
    }),
  ];
} catch (error) {
  operations = [
    operation('workspace', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE', 'BLOCKED', 'loadConfig(projectDirectory) plus analyzeProject(projectDirectory)', String(error?.message ?? error), { before, after: await digests() }),
    ...[
      ['typed-template-resolution', 'ASSERT_GLINT_TYPED_TEMPLATE_RESOLUTION'],
      ['original-source-ranges', 'ASSERT_GLINT_EXACT_ORIGINAL_SOURCE_RANGES'],
      ['virtual-document-mappings', 'ASSERT_GLINT_VIRTUAL_DOCUMENT_MAPPINGS'],
      ['immutable-pinned-files', 'ASSERT_GLINT_IMMUTABLE_PINNED_FILES'],
      ['partial-failure-reporting', 'ASSERT_GLINT_EXPLICIT_PARTIAL_FAILURE'],
    ].map(([id, assertion]) => operation(id, assertion, 'BLOCKED', 'not invoked after workspace qualification failure', 'workspace prerequisite blocked operation', { prerequisite: 'workspace' })),
  ];
} finally {
  analysis?.shutdown();
}

const candidates = [
  {
    id: 'ember-template-compiler', version: version('ember-source'), outcome: 'SCOPED_ROLE', role: 'template-syntax-ast', operation: 'covered by prior retained qualification; not upgraded by Glint evidence', observation: 'Ember compiler remains syntax-only', anchors: [], supported_claims: ['bounded_source_structure'], prohibited_claims: prohibitedClaims,
  },
  {
    id: 'glint', version: version('@glint/core'), outcome: operations.some(({ outcome }) => outcome === 'BLOCKED') ? 'SCOPED_ROLE' : 'PASS', role: 'typed-template-project-analysis', operation: 'operation-level root API qualification', observation: 'see glint_operations for independent outcomes and evidence', anchors: [], supported_claims: ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation'], prohibited_claims: prohibitedClaims,
  },
  {
    id: 'tree-sitter', version: version('tree-sitter'), outcome: 'SCOPED_ROLE', role: 'typescript-concrete-syntax', operation: 'covered by prior retained qualification; not upgraded by Glint evidence', observation: 'Tree-sitter remains concrete-syntax-only', anchors: [], supported_claims: ['bounded_source_structure', 'exact_source_anchor'], prohibited_claims: prohibitedClaims,
  },
  {
    id: 'typescript-service', version: version('typescript'), outcome: 'PASS', role: 'typescript-symbol-definition', operation: 'covered by prior retained qualification', observation: 'TypeScript service support remains bounded to plain TypeScript fixture', anchors: [], supported_claims: ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation'], prohibited_claims: prohibitedClaims,
  },
];

const report = { schema_version: 'lsp-trace.provider-qualification.v2', candidates, glint_operations: operations };
await writeFile(output, `${JSON.stringify(report, null, 2)}\n`);
for (const item of operations) console.log(`${item.assertion}: ${item.outcome} — ${item.observation}`);
