'use strict';

const { createHash } = require('node:crypto');

const ORIGINAL_PREFIX = 'original:';
const VIRTUAL_PREFIX = 'virtual:';
const DIGEST_PATTERN = /^sha256:[0-9a-f]{64}$/;

function requireNonEmptyString(value, name) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new TypeError(`${name} must be a non-empty string`);
  }
  return value;
}

function canonicalOriginalDocumentIdentity(uri) {
  const canonical = new URL(requireNonEmptyString(uri, 'uri'));
  canonical.hash = '';
  return `${ORIGINAL_PREFIX}${canonical.href}`;
}

function canonicalVirtualDocumentIdentity({ originalIdentity, generator, name }) {
  requireIdentityKind(originalIdentity, 'original');
  requireNonEmptyString(generator, 'generator');
  requireNonEmptyString(name, 'name');
  const digest = createHash('sha256')
    .update(originalIdentity)
    .update('\0')
    .update(generator)
    .update('\0')
    .update(name)
    .digest('hex');
  return `${VIRTUAL_PREFIX}${encodeURIComponent(generator)}:${digest}`;
}

function identityKind(identity) {
  requireNonEmptyString(identity, 'identity');
  if (identity.startsWith(ORIGINAL_PREFIX)) return 'original';
  if (identity.startsWith(VIRTUAL_PREFIX)) return 'virtual';
  throw new TypeError('identity must be a canonical original or virtual document identity');
}

function requireIdentityKind(identity, expected) {
  const actual = identityKind(identity);
  if (actual !== expected) throw new TypeError(`expected ${expected} document identity`);
  return identity;
}

function immutableRange(range, name) {
  if (!range || !Number.isSafeInteger(range.start) || !Number.isSafeInteger(range.end) || range.start < 0 || range.end < range.start) {
    throw new TypeError(`${name} must be a non-negative half-open integer range`);
  }
  return Object.freeze({ start: range.start, end: range.end });
}

class DocumentCustody {
  #records = new Map();

  register({ identity, digest, revision, parentIdentity }) {
    const kind = identityKind(identity);
    requireNonEmptyString(revision, 'revision');
    if (!DIGEST_PATTERN.test(digest)) throw new TypeError('digest must be a lowercase sha256 digest');

    if (kind === 'original' && parentIdentity !== undefined) {
      throw new TypeError('original document custody cannot have a parent');
    }
    if (kind === 'virtual') {
      requireIdentityKind(parentIdentity, 'original');
      this.resolve(parentIdentity, revision);
    }

    const byRevision = this.#records.get(identity) ?? new Map();
    const existing = byRevision.get(revision);
    if (existing) {
      if (existing.digest !== digest || existing.parentIdentity !== parentIdentity) {
        throw new Error(`custody conflict for ${identity} at revision ${revision}`);
      }
      return existing;
    }

    const record = Object.freeze({
      identity,
      kind,
      digest,
      revision,
      ...(parentIdentity === undefined ? {} : { parentIdentity }),
    });
    byRevision.set(revision, record);
    this.#records.set(identity, byRevision);
    return record;
  }

  resolve(identity, revision) {
    requireNonEmptyString(revision, 'revision');
    const record = this.#records.get(identity)?.get(revision);
    if (!record) throw new Error(`unknown document revision: ${identity} at ${revision}`);
    return record;
  }
}

function compareSegments(left, right) {
  return left.original.start - right.original.start
    || left.original.end - right.original.end
    || left.generated.start - right.generated.start
    || left.generated.end - right.generated.end;
}

function createDocumentMapping({ custody, original, virtual, segments }) {
  if (!(custody instanceof DocumentCustody)) throw new TypeError('custody must be a DocumentCustody');
  if (!original || !virtual) throw new TypeError('original and virtual custody records are required');
  requireIdentityKind(original.identity, 'original');
  requireIdentityKind(virtual.identity, 'virtual');

  if (original.revision !== virtual.revision) throw new Error('mixed mapping revisions');
  const revision = original.revision;
  const authoritativeOriginal = custody.resolve(original.identity, revision);
  const authoritativeVirtual = custody.resolve(virtual.identity, revision);
  if (authoritativeOriginal !== original || authoritativeVirtual !== virtual) {
    throw new Error('mapping custody records are not authoritative');
  }
  if (virtual.parentIdentity !== original.identity) {
    throw new Error('virtual document is not subordinate to the original document');
  }
  if (!Array.isArray(segments) || segments.length === 0) throw new TypeError('segments must be a non-empty array');

  const canonicalSegments = segments.map((segment, index) => {
    if (!segment || segment.revision !== revision) throw new Error('mixed mapping revisions');
    const originalRange = immutableRange(segment.original, `segments[${index}].original`);
    const generatedRange = immutableRange(segment.generated, `segments[${index}].generated`);
    if (originalRange.end - originalRange.start !== generatedRange.end - generatedRange.start) {
      throw new Error('mapping segment must preserve exact range length');
    }
    return Object.freeze({ original: originalRange, generated: generatedRange, revision });
  }).sort(compareSegments);

  for (let index = 1; index < canonicalSegments.length; index += 1) {
    const previous = canonicalSegments[index - 1];
    const current = canonicalSegments[index];
    if (current.original.start < previous.original.end || current.generated.start < previous.generated.end) {
      throw new Error('mapping segments overlap');
    }
  }
  Object.freeze(canonicalSegments);

  function translateOriginalRange(input, requestedRevision) {
    if (requestedRevision !== revision) {
      throw new Error(`unknown document revision: ${original.identity} at ${requestedRevision}`);
    }
    custody.resolve(original.identity, requestedRevision);
    custody.resolve(virtual.identity, requestedRevision);
    const range = immutableRange(input, 'range');

    let generatedStart;
    let generatedEnd;
    if (range.start === range.end) {
      const segment = canonicalSegments.find((candidate) => range.start >= candidate.original.start && range.start <= candidate.original.end);
      if (segment) generatedStart = generatedEnd = segment.generated.start + (range.start - segment.original.start);
    } else {
      let cursor = range.start;
      for (const segment of canonicalSegments) {
        if (segment.original.end <= cursor) continue;
        if (segment.original.start !== cursor && !(segment.original.start < cursor && cursor < segment.original.end)) break;
        const pieceEnd = Math.min(segment.original.end, range.end);
        const pieceStartGenerated = segment.generated.start + (cursor - segment.original.start);
        const pieceEndGenerated = segment.generated.start + (pieceEnd - segment.original.start);
        if (generatedStart === undefined) generatedStart = pieceStartGenerated;
        if (generatedEnd !== undefined && generatedEnd !== pieceStartGenerated) break;
        generatedEnd = pieceEndGenerated;
        cursor = pieceEnd;
        if (cursor === range.end) break;
      }
      if (cursor !== range.end) generatedStart = generatedEnd = undefined;
    }

    if (generatedStart === undefined || generatedEnd === undefined) throw new Error('range is not exactly mapped');
    return Object.freeze({
      kind: 'generated-coordinate',
      documentIdentity: virtual.identity,
      parentOriginalIdentity: original.identity,
      revision,
      range: Object.freeze({ start: generatedStart, end: generatedEnd }),
    });
  }

  return Object.freeze({
    original,
    virtual,
    revision,
    segments: canonicalSegments,
    translateOriginalRange,
  });
}

module.exports = Object.freeze({
  canonicalOriginalDocumentIdentity,
  canonicalVirtualDocumentIdentity,
  createDocumentMapping,
  DocumentCustody,
});
