# ADR0011 C15 main-convergence successor v1 — corrected proposal

This is a candidate-specific successor package. Its row classifications are **writer-authored proposals, not self-accepted**.

## Enumeration

The historical `enumerator_test.go` and `clean-regenerate.sh` produced the retained canonical outputs and `CLEAN_REGENERATION.txt`; they were removed from the current tree because the obsolete specimen was not an ordinary package. See [`../HISTORICAL_GENERATOR_PROVENANCE.md`](../HISTORICAL_GENERATOR_PROVENANCE.md) for restoration. Retained logs and conclusions remain historical evidence.

## Mechanical delta

- Baseline: 3,847
- Current: 4,155
- ADDED: 310
- REMOVED: 2
- CHANGED: 3
- UNCHANGED: 3,842

Identity reconciliation uses stable `(path, enclosing, AST kind, display)` buckets and ordered normalized-AST-digest sequences. It does not pair by ordinal.

## Proposed semantic adjudication

- NECESSARY: 29
- REUSE: 2
- FALSE_POSITIVE: 284
- UNRESOLVED: 0

`ROW_ADJUDICATION.tsv` and `.json` reason every affected row using owner, custody, release, concurrency, copy, and reuse criteria. Prior qualified classification is reused only for 314 exact candidate-ID and AST-digest identities. The single new test-hook call is independently classified. Deep owner-observation copies, frame copies, result/error-data copies, and original request/response frame copies remain NECESSARY; the two exact-capacity backing-reuse rows remain REUSE.

The independently blocked blanket proposal is preserved under `superseded-blocked-proposal-v1/`; it is historical and superseded, not erased. Historical C15 packages remain immutable.
