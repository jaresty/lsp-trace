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
