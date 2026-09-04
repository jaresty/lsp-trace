import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const required = [
  ['workspace', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE'],
  ['typed-template-resolution', 'ASSERT_GLINT_TYPED_TEMPLATE_RESOLUTION'],
  ['original-source-ranges', 'ASSERT_GLINT_EXACT_ORIGINAL_SOURCE_RANGES'],
  ['virtual-document-mappings', 'ASSERT_GLINT_VIRTUAL_DOCUMENT_MAPPINGS'],
  ['immutable-pinned-files', 'ASSERT_GLINT_IMMUTABLE_PINNED_FILES'],
  ['partial-failure-reporting', 'ASSERT_GLINT_EXPLICIT_PARTIAL_FAILURE'],
];

const reportURL = new URL('../retained/provider-qualification/report.json', import.meta.url);

test('retained report contains evidence-backed frozen Glint operations', async () => {
  const report = JSON.parse(await readFile(reportURL, 'utf8'));
  assert.equal(report.schema_version, 'lsp-trace.provider-qualification.v2', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE: report schema');
  assert.deepEqual(report.glint_operations?.map(({ id }) => id), required.map(([id]) => id));

  for (const [id, assertion] of required) {
    const operation = report.glint_operations.find((entry) => entry.id === id);
    assert.ok(operation, `${assertion}: missing operation`);
    assert.ok(['PASS', 'SCOPED_ROLE', 'BLOCKED'].includes(operation.outcome), `${assertion}: non-closed outcome`);
    assert.equal(operation.assertion, assertion, `${assertion}: identity`);
    assert.ok(operation.api && operation.observation && operation.evidence, `${assertion}: exact executed evidence`);
  }

  const byID = Object.fromEntries(report.glint_operations.map((entry) => [entry.id, entry]));
  assert.deepEqual(byID.workspace.evidence.diagnostics, [], 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE: zero diagnostics');
  assert.equal(byID.workspace.evidence.versions['@glint/core'], '1.5.2', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE: core pin');
  assert.equal(byID.workspace.evidence.versions['@glint/environment-ember-template-imports'], '1.5.2', 'ASSERT_GLINT_VALID_FROZEN_WORKSPACE: environment pin');
  assert.equal(byID['typed-template-resolution'].evidence.definitions.length, 1, 'ASSERT_GLINT_TYPED_TEMPLATE_RESOLUTION: one definition');
  assert.equal(byID['original-source-ranges'].evidence.original.text, 'this.itemCount', 'ASSERT_GLINT_EXACT_ORIGINAL_SOURCE_RANGES: token text');
  assert.deepEqual(
    byID['virtual-document-mappings'].evidence.round_trip_original_offsets,
    byID['virtual-document-mappings'].evidence.original_offsets,
    'ASSERT_GLINT_VIRTUAL_DOCUMENT_MAPPINGS: exact round trip',
  );
  assert.deepEqual(
    byID['immutable-pinned-files'].evidence.after,
    byID['immutable-pinned-files'].evidence.before,
    'ASSERT_GLINT_IMMUTABLE_PINNED_FILES: unchanged digests',
  );
  assert.equal(byID['partial-failure-reporting'].outcome, 'BLOCKED', 'ASSERT_GLINT_EXPLICIT_PARTIAL_FAILURE: unsupported remains blocked');
  assert.deepEqual(
    byID['partial-failure-reporting'].evidence.required_distinctions,
    ['unsupported', 'unavailable', 'partial', 'bounded', 'empty', 'transport-failed'],
    'ASSERT_GLINT_EXPLICIT_PARTIAL_FAILURE: exact distinctions',
  );
});
