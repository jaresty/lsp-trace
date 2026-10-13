# ADR0011 main-convergence exact-candidate Tier 3 qualification v1

## Decision

`BLOCKED_TEST_ONLY_COMPOSITION_AND_MISSING_PRODUCTION_EVIDENCE`

Source commit `8c6ae21e3709b80f6abae0ca8b1df185d81d99d9`, tree `712a950f7f6c0b1c6a09f6d8178808dca88dbc0d`. Qualification ceiling: `authority=0`, `accepted=false`, `completeness=UNKNOWN`, `featureIdentity=UNRESOLVED`, `production_admission=REFUSED_MISSING_EVIDENCE`.

HEAD is a single-parent cleanup over candidate-package commit `320edd24`. The underlying convergence merge is `2e38a92084073a676803465eee6177ec5c72abae`, parents `6fd25ba4dddb43b191316e3d01dc8c96b5e3b066` and `57225a1e9e7282ed7f58a7a19f8ca538c3d6105e`, with merge base recorded in `logs/source.txt`.

## Six overlap dispositions

1. ADR0011 document: semantic union retained; authority ceiling and no-CALLS statements remain explicit.
2. `internal/integratedconformance/harness_test.go`: ownership union retained, but the file explicitly declares disabled hermetic reference-only status and no production registration/Stage 2 authority.
3. `internal/censuscontinuation/continuation_test.go`: continuation assertions retained.
4. `internal/censuscontinuation/stage_matrix_test.go`: stage matrix assertions retained.
5. `sessionruntime/b4_explicit_bytes_c15_private.go`: C15 explicit-byte implementation union retained.
6. `sessionruntime/diagnostic_backing_c15_private.go`: C15 diagnostic backing union retained.

Exact parent diffs and merge-result bytes are retained in `logs/overlap-*`.

## C15

Two fresh canonical enumerations passed and were byte-identical. Candidate and classification rows are 4,155, duplicate IDs are 0, and `UNRESOLVED` is 0. Location data has one header plus 4,155 records. Exact successor SHA-256 values are in `logs/c15-verification.txt`. The frozen historical census has 3,847 rows and its exact byte SHA-256 is retained there; no historical package was changed.

## Composition and production capability

The requested private sequence can be represented across candidate packages (Describe, Search, Group, Location, Source Text Search, immutable candidate generation), but current exact-candidate evidence does not establish one production-capable composed chain. The integrated harness states that it contains disabled hermetic reference tests only and owns no production registration, advertisement, containment, or Stage 2 authority. Reference-grouping qualification helpers and qualification evidence are test-only. Therefore the seam is test-only/stale for production admission and the required hard block applies.

There is no directional CALLS activation: ADR0011 says definition/reference evidence never becomes CALLS, omitted selection remains historical CALLS-only, and the pilot remains disabled. Structural evidence establishes none of feature identity, ownership, purpose, runtime use, completeness, or acceptance.

## Verification

Focused ownership registration, unowned rejection, adjacent-path/fixture exclusion, the complete integrated-conformance package, and census-continuation package passed. Complete repository selectors and all failures are retained under `logs/` and classified in `FAILURE_CLASSIFICATION.md`. Tier 2 packages were retained unchanged; Tier 3 commands were rerun.
