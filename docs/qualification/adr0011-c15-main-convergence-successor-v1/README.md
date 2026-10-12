# ADR0011 C15 main-convergence successor v1 — corrected proposal

This is a candidate-specific successor package. Its row classifications are **writer-authored proposals, not self-accepted**.

## Enumeration

`enumerator_test.go` defines the real Go test `TestADR0011C15MainConvergenceSuccessorEnumerate`. `clean-regenerate.sh` temporarily installs it into `sessionruntime`, performs two fresh canonical runs under the exact environment recorded in `COMMAND.txt`, proves 4,155 catalog/classification/location rows, zero duplicate IDs, 44 verified allocation-mapping rows, byte-identical outputs, and equality with `canonical/`, then removes the temporary test. Logs and `CLEAN_REGENERATION.txt` are retained here.

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
