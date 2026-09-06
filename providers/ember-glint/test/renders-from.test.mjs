import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { createDefaultAnalyzer } from '../default-analyzer.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const base = `import Component from '@glimmer/component';
interface PanelSignature { Args: { records: string[]; } }
export default class Panel extends Component<PanelSignature> {
  get visibleTotal(): number { return this.args.records.length; }
  <template><p>{{this.visibleTotal}}</p></template>
}`;

async function analyze(source, needle = 'this.visibleTotal') {
  const directory = mkdtempSync(path.join(root, 'fixtures', 'renders-from-test-'));
  const filename = path.join(directory, 'panel.gts');
  writeFileSync(filename, source);
  try {
    const selected = source.lastIndexOf(needle) + needle.lastIndexOf('.') + 1;
    const before = source.slice(0, selected).split('\n');
    const position = { line: before.length - 1, character: before.at(-1).length };
    const uri = pathToFileURL(filename).href;
    return await createDefaultAnalyzer().analyze({
      schema: 'lsp-trace.provider-request.v1', request_id: createHash('sha256').update(source).digest('hex'), operation: 'analyze', relation_kinds: ['RENDERS_FROM'],
      documents: [{ uri, language: 'glimmer-js', revision: 'test', digest: `sha256:${createHash('sha256').update(source).digest('hex')}`, source, position }],
      limits: { max_observations: 10, timeout_ms: 30000 },
    });
  } finally { rmSync(directory, { recursive: true, force: true }); }
}

test('ASSERT_RENDERS_FROM_REAL_GLINT_EXACT_TYPED_DIRECT_READ', async () => {
  const result = await analyze(base);
  assert.equal(result.outcome, 'COMPLETE');
  assert.equal(result.observations.length, 1);
  const [observation] = result.observations;
  assert.equal(observation.kind, 'RENDERS_FROM');
  assert.deepEqual(observation.from.role, 'RENDER_EXPRESSION');
  assert.deepEqual(observation.to.role, 'READ_VALUE');
  assert.equal(observation.original_anchor.document_id, 'original');
  assert.equal(observation.original_anchor.revision, 'test');
  assert.match(observation.to.node_id, /property:visibleTotal;definition=[0-9]+-[0-9]+;direct-read=[0-9]+-[0-9]+;args=records$/);
  assert.match(observation.virtual_anchor.mapping_id, /^glint:[0-9a-f]{64}:[0-9]+:[0-9]+$/);
  assert.notEqual(observation.virtual_anchor.uri, observation.original_anchor.uri);
  assert.deepEqual(observation.supports, ['bounded_source_structure', 'exact_source_anchor', 'typed_static_relation']);
  assert.deepEqual(observation.does_not_support, ['callback_execution', 'feature_identity', 'render_occurrence', 'repaint', 'runtime_execution', 'source_completeness']);
});

test('ASSERT_RENDERS_FROM_RENAMES_CHANGE_IDENTITIES_AND_ANCHORS', async () => {
  const original = (await analyze(base)).observations[0];
  const renamed = (await analyze(base.replaceAll('visibleTotal', 'shownAmount').replaceAll('records', 'entries'), 'this.shownAmount')).observations[0];
  assert.notEqual(renamed.from.node_id, original.from.node_id);
  assert.notEqual(renamed.to.node_id, original.to.node_id);
  assert.match(renamed.to.node_id, /property:shownAmount;/);
  assert.match(renamed.to.node_id, /args=entries$/);
  assert.notDeepEqual(renamed.original_anchor.range, original.original_anchor.range);
});

test('ASSERT_RENDERS_FROM_TEMPLATE_RESOLUTION_CHANGES_RELATION', async () => {
  const source = base.replace('get visibleTotal(): number { return this.args.records.length; }', 'get visibleTotal(): number { return this.args.records.length; }\n  get alternateTotal(): number { return this.args.records.length; }').replace('this.visibleTotal}}</p>', 'this.alternateTotal}}</p>');
  const result = await analyze(source, 'this.alternateTotal');
  assert.equal(result.observations.length, 1);
  assert.match(result.observations[0].to.node_id, /property:alternateTotal;/);
  assert.notDeepEqual(result.observations[0].original_anchor.range, (await analyze(base)).observations[0].original_anchor.range);
});

test('ASSERT_RENDERS_FROM_REJECTS_UNRELATED_UNRESOLVED_UNSAFE_AND_INDIRECT', async () => {
  const cases = [
    base.replace('get visibleTotal(): number { return this.args.records.length; }', 'visibleTotal: number = 7;'),
    base.replace('this.visibleTotal}}</p>', 'this.missingTotal}}</p>'),
    base.replace('records: string[]', 'records: any'),
    base.replace('get visibleTotal(): number { return this.args.records.length; }', 'get visibleTotal(): number { const alias = this.args.records; return alias.length; }'),
  ];
  const needles = ['this.visibleTotal', 'this.missingTotal', 'this.visibleTotal', 'this.visibleTotal'];
  const reasons = ['DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE', 'DEFINITION_UNRESOLVED', 'UNSAFE_OR_UNKNOWN_TYPE', 'DIRECT_QUALIFIED_SOURCE_READ_UNAVAILABLE'];
  for (let index = 0; index < cases.length; index += 1) {
    const result = await analyze(cases[index], needles[index]);
    assert.equal(result.outcome, 'UNAVAILABLE');
    assert.equal(result.coverage.reason, reasons[index]);
    assert.deepEqual(result.observations, []);
  }
});

test('ASSERT_RENDERS_FROM_DETERMINISTIC', async () => {
  const first = await analyze(base);
  const second = await analyze(base);
  const scrub = (value) => JSON.parse(JSON.stringify(value).replaceAll(/renders-from-test-[^/]+/g, 'renders-from-test-STABLE'));
  assert.deepEqual(scrub(first), scrub(second));
});
