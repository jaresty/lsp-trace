# Cross-Document Custody and Mapping claim

Status: CLAIMED
Baseline: 0702d12
Frame: Cross-Document Custody and Mapping

## Goal

Provide provider-side production JavaScript utilities for canonical original and virtual document identities, immutable digest/revision custody, deterministic exact mappings, mixed or unknown revision rejection, and generated-coordinate subordination without emitting semantic relation conclusions.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-ember-provider-0702d12-v1/FRAMEWORK.md`
- `provider-admission-semantic-binding.claim.md`

## Exact ownership

New files only:

- `provider/cross-document-custody-mapping.js`
  - canonical original and virtual document identities
  - digest/revision custody
  - deterministic exact range mapping
  - mixed/unknown revision rejection
  - generated-coordinate subordination
- `provider/cross-document-custody-mapping.test.js`
  - assertion-specific custody and mapping guards
- `cross-document-custody-mapping.claim.md`
  - this ownership and evidence record

No existing file or symbol is claimed. Analyzers, framing commands, release packaging, qualification-only paths, testdata, semantic relation inference, and NAIS files remain unmodified and unclaimed.

## Required assertion identities

- `ASSERT_DOCUMENT_IDENTITY_CANONICAL_KIND_DISTINCT`
- `ASSERT_DOCUMENT_CUSTODY_DIGEST_REVISION_IMMUTABLE`
- `ASSERT_DOCUMENT_CUSTODY_UNKNOWN_REVISION_REJECTED`
- `ASSERT_MAPPING_DETERMINISTIC`
- `ASSERT_MAPPING_MIXED_REVISION_REJECTED`
- `ASSERT_RANGE_TRANSLATION_EXACT`
- `ASSERT_GENERATED_COORDINATE_SUBORDINATE`
- `ASSERT_MAPPING_EMITS_NO_SEMANTIC_RELATIONS`

## Enforcement sequence

1. Commit this claim as the first repository mutation.
2. Add compile-valid assertion-specific tests against present-but-wrong behavior.
3. Capture focused red output naming each behavioral assertion.
4. Implement the smallest production JavaScript module satisfying those assertions.
5. Rerun the identical focused test green, then repository checks.
6. Record exact evidence and commit a clean worktree.

## Red evidence

The committed present-but-wrong production surface was exercised with:

`node --test provider/cross-document-custody-mapping.test.js`

Result: 0 passed, 8 failed. The output independently named all eight required assertion identities and each assertion reported its own mismatch or missing rejection.

## Implementation

- Canonical original identities normalize absolute URLs and remove fragments under an `original:` kind prefix.
- Canonical virtual identities are deterministic, generator-qualified SHA-256 identities derived from their original parent, generator, and generated-document name under a disjoint `virtual:` prefix.
- `DocumentCustody` retains frozen digest/revision records, rejects conflicting custody, requires virtual-parent custody at the same revision, and rejects unknown revisions.
- `createDocumentMapping` validates authoritative same-revision custody, canonicalizes exact equal-length segments, rejects mixed revisions and overlaps, and translates only fully covered contiguous ranges.
- Generated coordinates carry only subordinate virtual/original identities, revision, range, and coordinate kind; no semantic relation conclusion is emitted.

## Evidence

- Focused red: 0 passed, 8 failed; every required assertion identity was named.
- Focused green: 8 passed, 0 failed.
- Strengthened exact-range green, including interior-hole and unknown-revision rejection: 8 passed, 0 failed.
- `node --check provider/cross-document-custody-mapping.js`: pass.
- `git diff --check`: pass.
- Pre-commit full suite: 3229 passed, 2 failed, 2 skipped; both failures were the same dirty-worktree `ASSERT_PACKAGE_OWNERSHIP_ONLY` check observing the newly owned production module before commit.
- Claim-first commit: `91d5786`.
- Assertion-specific red commit: `cf2b6a9`.
- Implementation commit: `38baba8`.
- Clean-state focused guard: 8 passed, 0 failed.
- Clean-state full suite: 3231 passed in 40 packages.
- Clean-state `git diff --check`: pass; `git status --short`: empty.
