# ADR0011 call-hierarchy Layer-1 RED oracle corpus v1 — independent byte review v2

## Scope and boundary

Reviewed the committed corpus at `/Users/schwa/dev/lsp-trace-adr0011-c17` HEAD `fee5cbe0eb76c3eb15acf8bb1b06b095f69b2c96`, with the pre-existing modified RED file left unchanged. This adjudication used committed corpus and documentation bytes only. It did not execute the corpus, RED, tests, generators, or runtime code and does not authorize implementation or runtime behavior.

## Defect

The committed corpus is not self-contained at its two aggregate roots.

- `worlds/world-24.json` contains `record.concurrency.workers` as exactly 16 string IDs, `WORKER_201` through `WORKER_216`. It contains no complete worker records.
- `worlds/world-25.json` contains `record.comparison.executions` as exactly 22 string IDs, `EXEC_301` through `EXEC_322`, plus an 11-pair matrix whose `enabled` and `disabled` members are also string IDs. It contains no complete execution records.
- No other file under `sessionruntime/testdata/callhierarchy-layer1-red-v1/` supplies those 38 referenced records. The corpus path contains only `manifest.json` and the 25 world files; the manifest contains world metadata and source pins, not worker or execution records.

The qualified design registry separately defines complete `WORKER_201`–`WORKER_216` and `EXEC_301`–`EXEC_322` records, but the registry is outside the corpus path. Registry availability does not make the committed corpus roots recursively complete or permit a future RED constrained to corpus-only consumption to resolve the IDs.

## Requirement comparison

The user requirement is for root files that are recursively complete literal trees and for future RED consumption from the committed corpus alone. Under that boundary, an aggregate world must carry the complete records needed to validate its aggregate/control claims without consulting the qualified design registry, a generator, runtime output, or any ambient lookup.

World 24 and World 25 fail that requirement. Their direct JSON syntax and published hashes are internally consistent, but those facts establish only exact serialization of incomplete roots. Counting 16 worker IDs or 22 execution IDs does not establish 16 or 22 complete records in the corpus. Likewise, World 25's pair booleans do not establish the claimed enabled/disabled external and transport equality without the referenced execution records.

The unchanged modified RED does not presently consume this corpus, so it does not repair or mask the omission. A future corpus-only RED loader would necessarily either stop at unresolved identifiers or obtain semantics from an uncommitted external authority, violating the required closure boundary.

## Prior disposition

The prior `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_BYTES_GO` is invalid and superseded. Its statements that World 24 contains “16 unique complete workers,” World 25 contains “22 unique complete executions,” and closure defects are zero conflate complete records in the qualified design registry with records emitted into the committed corpus. The reviewed bytes themselves are not self-contained.

This does not reject the qualified design registry as the source specification. It rejects the current emitted-world definition and frozen corpus bytes as a self-contained RED oracle corpus.

## Exact correction boundary

Before a new byte-level GO is possible:

1. Replace World 24's 16 worker-ID strings with the fully expanded canonical literal records for `WORKER_201` through `WORKER_216` inside `world-24.json`.
2. Replace World 25's 22 execution-ID strings with the fully expanded canonical literal records for `EXEC_301` through `EXEC_322` inside `world-25.json`.
3. Make the World 24 aggregate/control material self-contained against those embedded workers, including the aggregate accounting/ledger values and the control structure needed to validate them without registry lookup.
4. Make the World 25 aggregate/control material self-contained against those embedded executions, including the complete 11-pair enabled/disabled matrix and the external/transport equality claims needed to validate each pair without registry lookup. Pair references may not remain unresolved IDs; they must identify embedded records unambiguously within the same literal tree.
5. Re-canonicalize the corrected world files and update their exact byte lengths and SHA-256 values.
6. Recompute the root-ID-sorted aggregate SHA-256 and update `manifest.json`, including the corrected World 24 and World 25 length/hash entries and canonical manifest bytes.
7. Re-review corpus-path closure by parsing only the committed corpus directory, with no qualified-registry, generator, runtime, or ambient lookup.
8. Preserve the qualified design registry as the source specification, but amend and independently review its `C_RECORD`/`C_WORLD` emitted-world and hash-definition documentation if those mappings currently define World 24 and World 25 as ID-only roots. Registry record definitions alone are not sufficient; the documented emitted roots, their published lengths/hashes, and the manifest derivation must match the recursively expanded corpus bytes.

No change to runtime code or the modified RED is within this correction boundary.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_BYTES_BLOCKED`
