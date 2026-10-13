# ADR0011 directional-raw custody C15 enumeration v1

Status: `ENUMERATION_COMPLETE_NOT_ADJUDICATED`  
Source HEAD: `a1cd77950ffa45ebed52310878f772f3c7344756`  
Authority: `0`; semantic acceptance: `false`; implementation authority: `none`.

This documentation-only package mechanically applies the accepted `C15_ENUMERATION_PLAN.md`. It freezes the exact six-path source manifest; enumerates current retained allocation/copy, zero-charge borrow/alias, and terminal release sites relevant to the six directional-raw charge-table components; and emits absent-successor owner, alias, transfer, and release candidates. Structural/source evidence only prepares rows. Every row is explicitly `UNRESOLVED` and assigned in `INDEPENDENT_ROW_ADJUDICATION_ASSIGNMENT.tsv`; this producer does not classify NECESSARY, REUSE, FALSE_POSITIVE, or acceptance.

`UNCHANGED` means only that an exact current-source baseline relationship is present at the frozen source pin. `ADDED` means an architecture-required successor relationship is absent from that baseline. No `CHANGED` identity pairing is guessed across different source paths/symbols. Empty `CHANGED.tsv` and `REMOVED.tsv` are retained as finite outputs.

## Outputs

- `SOURCE_MANIFEST.sha256`: exact six production paths plus accepted plan/baseline inputs.
- `BASELINE_PRIOR_UNCHARGED.tsv`: current exact baseline source relationships.
- `ADDED.tsv`, `CHANGED.tsv`, `REMOVED.tsv`, `UNCHANGED.tsv`: pairwise-disjoint mechanical delta partitions.
- `ALLOCATION_SITES.tsv`, `RELEASE_SITES.tsv`, `BORROW_ALIAS_SITES.tsv`, `TRANSFER_SITES.tsv`: complete operation projections of the row set.
- `SOURCE_LOCATIONS.tsv`: source-location accounting.
- `UNRESOLVED.tsv`: all rows pending independent adjudication.
- `INDEPENDENT_ROW_ADJUDICATION_ASSIGNMENT.tsv`: reviewer assignment surface.
- `COUNTS.json`: duplicate/lineage/genesis/branch accounting and row counts.
- `RUN_MANIFEST.sha256`: package file hashes.
- `generate.py`: deterministic producer with exact site assertions.

The required-new production file remains absent. Its proposal rows therefore use line `0`, `ABSENT_REQUIRED_NEW`, and `UNRESOLVED`; no source bytes or implementation are invented. No production/test file was changed or executed.

`ENUMERATION_COMPLETE_NOT_ADJUDICATED`
