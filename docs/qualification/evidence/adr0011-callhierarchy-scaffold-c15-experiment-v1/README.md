# ADR-0011 Call-Hierarchy Scaffold C15 Experiment v1

Status: **EXPERIMENTAL / NON-AUTHORITATIVE / NO WRITE GATE**

This directory preserves the accepted, lifecycle-only disposable experiment performed from repository HEAD `2f5fd760f8d39cf0bca63258291bffd67f44a06c`. It is evidence about the exact `source.patch` bytes only. It is not accepted implementation, current-source evidence, a production claim, a semantic-ownership decision, a successor catalog, permission to overwrite `c15-census-v1`, or permission to commit.

## Exact experimental result

Applying `source.patch` to the pinned HEAD in a disposable clean checkout and then regenerating the C15 catalog produced:

- candidates: 3,805 → 3,838 (`+33`);
- classifications: 3,805 → 3,838 (`+33`);
- retained predecessor rows byte-identical: 3,805;
- removals: 0;
- changed/reclassified rows: 0;
- added classifications: 33, all `excluded`;
- accepted mapping: 44 rows (15 owner, 7 alias, 22 copy);
- Manager propagation: absent in the retained check.

The `+33` result is exact only for this pinned patch. Final accepted lifecycle implementation spelling may change AST-derived IDs, hashes, counts, or classifications and therefore requires a fresh independent finite-delta review.

## Reproduction boundary

At unmodified HEAD, the canonical non-writing v1 verification is:

```sh
go test ./sessionruntime \
  -run '^(TestPrivateB4C15OfflineCatalogAndCompactProductionContract|TestPrivateB4C15RegenerateOfflineCatalog|TestPrivateB4C15RegenerationDoesNotReadGeneratedOutputs|TestPrivateB4C15ProductionSiteBindings|TestPrivateB4C15MappingOrderIsDeterministic)$' \
  -count=1
```

That command verifies the immutable 3,805-row v1 state at HEAD; it does not reproduce this experiment.

To reproduce the experiment, use a disposable clean checkout pinned to the recorded HEAD, apply `source.patch`, and only then run the retained regeneration command:

```sh
git apply --check source.patch
git apply source.patch
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' \
  -count=1
```

`C15_REGENERATE=1` mutates `sessionruntime/testdata/c15-census-v1` in that disposable checkout solely to capture comparison bytes. It must never be run against retained v1 evidence for an accepted repository change. Compare the generated files with the retained before/after files and `delta.json`, then discard/reset the disposable checkout.

After an accepted lifecycle implementation exists, final `c15-census-v2` bytes must be generated from that final source spelling under a successor-specific generator/test contract. They must receive a new exact independent delta review; this experiment cannot pre-authorize or predict those bytes.

## Packet map

- Patch and context: `source.patch`, `source.patch.sha256`, `context.txt`, `head-before-reset.txt`, `modified-source.sha256`.
- Generated snapshots and sidecars: `candidates.{before,after}.tsv`, `classifications.{before,after}.tsv`, and their retained sidecars/hashes.
- Finite delta: `delta.json`, `delta-summary.txt`, and candidate/classification `added`, `removed`, and `changed` TSVs.
- Classification/mapping/nonpropagation: `classification-verification.txt`, `mapping-count.txt`, `mapping.sha256`, `manager-nonpropagation.txt`.
- Verification/status: `lifecycle-verification.txt`, `package-compile.txt`, `status.before-regeneration.txt`, `status.before-reset.txt`, `changed-paths.before-reset.txt`, `numstat.before-reset.txt`.
- Repository-relative integrity manifest: `SHA256SUMS`.

The excluded `attempt-overbroad-149` packet and ambiguous helper/packet manifests were intentionally not copied.
