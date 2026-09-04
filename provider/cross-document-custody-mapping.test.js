'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const {
  canonicalOriginalDocumentIdentity,
  canonicalVirtualDocumentIdentity,
  createDocumentMapping,
  DocumentCustody,
} = require('./cross-document-custody-mapping.js');

const ORIGINAL_URI = 'file:///workspace/components/../components/card.gts#template';
const DIGEST = 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const VIRTUAL_DIGEST = 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';

function fixture() {
  const custody = new DocumentCustody();
  const originalIdentity = canonicalOriginalDocumentIdentity(ORIGINAL_URI);
  const virtualIdentity = canonicalVirtualDocumentIdentity({
    originalIdentity,
    generator: 'glint',
    name: 'card.gts.ts',
  });
  const original = custody.register({ identity: originalIdentity, digest: DIGEST, revision: '17' });
  const virtual = custody.register({
    identity: virtualIdentity,
    digest: VIRTUAL_DIGEST,
    revision: '17',
    parentIdentity: originalIdentity,
  });
  return { custody, original, originalIdentity, virtual, virtualIdentity };
}

function mappingFixture(overrides = {}) {
  const state = fixture();
  const mapping = createDocumentMapping({
    custody: state.custody,
    original: state.original,
    virtual: state.virtual,
    segments: [
      { original: { start: 14, end: 18 }, generated: { start: 104, end: 108 }, revision: '17' },
      { original: { start: 10, end: 14 }, generated: { start: 100, end: 104 }, revision: '17' },
    ],
    ...overrides,
  });
  return { ...state, mapping };
}

test('ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT', () => {
  const originalA = canonicalOriginalDocumentIdentity(ORIGINAL_URI);
  const originalB = canonicalOriginalDocumentIdentity('file:///workspace/components/card.gts');
  const virtual = canonicalVirtualDocumentIdentity({ originalIdentity: originalA, generator: 'glint', name: 'card.gts.ts' });
  assert.equal(originalA, originalB, 'ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT canonical original');
  assert.match(originalA, /^original:/, 'ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT original kind');
  assert.match(virtual, /^virtual:/, 'ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT virtual kind');
  assert.notEqual(originalA, virtual, 'ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT disjoint kinds');
});

test('ASSERT_DOCUMENT_CUSTODY_DIGEST_REVISION_IMMUTABLE', () => {
  const { custody, original, originalIdentity } = fixture();
  assert.ok(Object.isFrozen(original), 'ASSERT_DOCUMENT_CUSTODY_DIGEST_REVISION_IMMUTABLE frozen record');
  assert.throws(
    () => custody.register({ identity: originalIdentity, digest: VIRTUAL_DIGEST, revision: '17' }),
    /custody conflict/,
    'ASSERT_DOCUMENT_CUSTODY_DIGEST_REVISION_IMMUTABLE conflict rejection',
  );
  assert.equal(custody.resolve(originalIdentity, '17').digest, DIGEST, 'ASSERT_DOCUMENT_CUSTODY_DIGEST_REVISION_IMMUTABLE retained digest');
});

test('ASSERT_DOCUMENT_CUSTODY_UNKNOWN_REVISION_REJECTED', () => {
  const { custody, originalIdentity } = fixture();
  assert.throws(
    () => custody.resolve(originalIdentity, '18'),
    /unknown document revision/,
    'ASSERT_DOCUMENT_CUSTODY_UNKNOWN_REVISION_REJECTED',
  );
});

test('ASSERT_MAPPING_DETERMINISTIC', () => {
  const first = mappingFixture().mapping;
  const second = mappingFixture({
    segments: [
      { original: { start: 10, end: 14 }, generated: { start: 100, end: 104 }, revision: '17' },
      { original: { start: 14, end: 18 }, generated: { start: 104, end: 108 }, revision: '17' },
    ],
  }).mapping;
  assert.deepEqual(first.segments, [
    { original: { start: 10, end: 14 }, generated: { start: 100, end: 104 }, revision: '17' },
    { original: { start: 14, end: 18 }, generated: { start: 104, end: 108 }, revision: '17' },
  ], 'ASSERT_MAPPING_DETERMINISTIC canonical segment order');
  assert.deepEqual(first.translateOriginalRange({ start: 11, end: 17 }, '17'), second.translateOriginalRange({ start: 11, end: 17 }, '17'), 'ASSERT_MAPPING_DETERMINISTIC stable result');
});

test('ASSERT_MAPPING_MIXED_REVISION_REJECTED', () => {
  const state = fixture();
  assert.throws(() => createDocumentMapping({
    custody: state.custody,
    original: state.original,
    virtual: state.virtual,
    segments: [
      { original: { start: 0, end: 1 }, generated: { start: 0, end: 1 }, revision: '17' },
      { original: { start: 1, end: 2 }, generated: { start: 1, end: 2 }, revision: '18' },
    ],
  }), /mixed mapping revisions/, 'ASSERT_MAPPING_MIXED_REVISION_REJECTED');
});

test('ASSERT_RANGE_TRANSLATION_EXACT', () => {
  const { mapping } = mappingFixture();
  assert.deepEqual(mapping.translateOriginalRange({ start: 11, end: 17 }, '17').range, { start: 101, end: 107 }, 'ASSERT_RANGE_TRANSLATION_EXACT contiguous exact range');
  assert.throws(() => mapping.translateOriginalRange({ start: 9, end: 11 }, '17'), /range is not exactly mapped/, 'ASSERT_RANGE_TRANSLATION_EXACT rejects partial range');
});

test('ASSERT_GENERATED_COORDINATE_SUBORDINATE', () => {
  const { mapping, originalIdentity, virtualIdentity } = mappingFixture();
  const coordinate = mapping.translateOriginalRange({ start: 12, end: 13 }, '17');
  assert.deepEqual(coordinate, {
    kind: 'generated-coordinate',
    documentIdentity: virtualIdentity,
    parentOriginalIdentity: originalIdentity,
    revision: '17',
    range: { start: 102, end: 103 },
  }, 'ASSERT_GENERATED_COORDINATE_SUBORDINATE');
});

test('ASSERT_MAPPING_EMITS_NO_SEMANTIC_RELATIONS', () => {
  const { mapping } = mappingFixture();
  const coordinate = mapping.translateOriginalRange({ start: 12, end: 13 }, '17');
  assert.deepEqual(Object.keys(coordinate).sort(), [
    'documentIdentity',
    'kind',
    'parentOriginalIdentity',
    'range',
    'revision',
  ], 'ASSERT_MAPPING_EMITS_NO_SEMANTIC_RELATIONS exact coordinate vocabulary');
  assert.equal(JSON.stringify(coordinate).includes('relation'), false, 'ASSERT_MAPPING_EMITS_NO_SEMANTIC_RELATIONS relation-neutral');
});
