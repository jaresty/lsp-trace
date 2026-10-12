# ADR-0011 Call-Hierarchy Layer-1 C15 Delta Experiment Plan Review v4

## Scope

Independent final superseding review of exact repository HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`, the corrected untracked plan, and the three preserved prior reviews:

- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan.md`;
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan-review.md`;
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan-review-v2.md`; and
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan-review-v3.md`.

The corrected write-set amendment and its review remain preserved. This review creates only this document. It does not execute the experiment, modify the plan or prior reviews, modify source/tests/corpus/generated artifacts, authorize implementation, authorize classification or binding changes, authorize artifact succession, or authorize a commit.

## Independent verification

### 1. Exact baseline identity and tracked-byte hashes

`git rev-parse HEAD` returned exact commit `9132d0759f49f9748c23442f31a1cd15dfe12343`.

The corrected plan's exact five-file SHA-256 verification passed for:

- `sessionruntime/testdata/c15-census-v2/candidates.tsv` — `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2`;
- `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256` — `b8725253368bc5257ddf85e4ef56535a31cbeb910100470c42cd138a799fb838`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv` — `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256` — `7e2cc13f1ce84854c8ff54ea6afdeb4f0d1d14cfb0edefdecdba9284f9694f53`; and
- `sessionruntime/testdata/c15-census-v2/manifest.json` — `548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf`.

The two bare-hash TSV sidecars were also explicitly bound to their corresponding filenames and both `shasum -a 256 -c -` checks returned `OK`.

### 2. TSV semantic baseline counts and candidate-ID sets

An independent strict headerless TSV parse verified:

- `candidates.tsv`: exactly 3,847 rows, exactly two fields per row, no empty candidate ID, no duplicate candidate ID, and 3,847 unique candidate IDs;
- `classifications.tsv`: exactly 3,847 rows, exactly five fields per row, no empty candidate ID, no duplicate candidate ID, and 3,847 unique candidate IDs; and
- exact equality of the candidate and classification candidate-ID sets, with zero candidate-only and zero classification-only IDs.

The corrected plan requires the same exact parser constraints independently for the generated pair before delta computation. It hard-stops on malformed rows, empty or duplicate IDs, count mismatch, sidecar mismatch, or candidate/classification ID-set inequality.

### 3. Historical manifest preservation and exact expected sole failure

The canonical non-writing command was run exactly:

```sh
go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1 -v
```

It reached and reported the sole failure:

```text
b4_c15_census_v2_successor_private_test.go:85: ASSERT_C15_V2_CLEAN_REGENERATION_manifest.json
```

No candidate-count, classification-count, predecessor-identity, classification, mapping, TSV pin, or sidecar failure preceded or accompanied it. This is exactly the provenance-control result now required by the corrected plan.

The plan correctly preserves tracked `manifest.json` as immutable historical provenance bytes, records its path/hash/length, forbids regenerating or rewriting it, excludes it from current-source semantic output and delta comparison, and does not misrepresent its source-sensitive mismatch as a semantic-baseline failure.

### 4. Canonical enumerator and ordering contract

At the pinned HEAD, the named same-package helpers exist with the required roles:

- `discoverPrivateB4C15Candidates` performs canonical discovery;
- `generatePrivateB4C15Catalog` emits candidate and classification rows in lockstep and returns the resulting byte slices; and
- `readPrivateB4C15Mapping` verifies the pinned mapping digest and requires owner `15`, alias `7`, copy `22`, total `44`, and exactly 44 rows.

The corrected plan resolves the v2 ordering blocker by requiring generated candidate and classification TSVs and sidecars to preserve the exact helper-emitted bytes and order. It forbids sorting, reordering, normalization, or rewriting those artifacts. Candidate-ID lexical sorting is permitted only for separate human-readable delta report tables. Candidate ID remains the sole semantic row identity, with candidate-row digest bytes and classification-row bytes compared independently.

### 5. Generated ID-set equality and complete delta accounting

Before any delta is computed, the plan independently requires:

- equality of baseline candidate/classification candidate-ID sets;
- equality of generated candidate/classification candidate-ID sets;
- rejection of every duplicate ID rather than collapsing rows; and
- successful sidecar verification for both baseline and generated TSVs.

It then defines disjoint `ADDED`, `REMOVED`, and `CHANGED` sets by candidate ID, classifies every common ID exactly once as changed or unchanged, separates ordering-only or serialization-only differences from semantic differences, and reconciles exhaustive affected-row tables to baseline and generated totals. The historical manifest has no role in those sets.

### 6. External-output safety and enumerator limits

The plan authorizes exactly one temporary same-package test file with exactly one test entry point, `TestADR0011CallHierarchyC15DeltaEnumerate`, and the exact command:

```sh
ADR0011_C15_DELTA_OUTPUT=<external-empty-dir> go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v
```

The enumerator must use the canonical helpers, preserve exact helper-emitted bytes, stage only under `t.TempDir()`, and refuse an unset output variable, a nonempty destination, a resolved destination inside any repository worktree, or any tracked-path write. It must preserve its exact source bytes/hash externally and must not generate, copy, synthesize, or rewrite `manifest.json`.

These constraints are explicit stop conditions, not implementation suggestions. Failure to enforce any one of them blocks the experiment rather than permitting an improvised enumerator.

### 7. Prototype boundary, safety, cleanup, and authority ceiling

All prior substantive boundaries remain intact:

- a newly created disposable isolated worktree based on the recorded clean commit;
- exactly three prototype production files;
- exactly eight existing inert symbols in `owned_method_pair.go`;
- no directional semantics, method triples, token/result/params interpretation, subspan accessors, selected/handoff bytes or commitments, predecessor chains, occurrence/relation behavior, public surface, activation, CLI, or MCP behavior;
- no retained test, documentation, fixture, corpus, manifest, sidecar, schema, configuration, generated-artifact, or fourth-source-file mutation;
- unchanged retained generic, corpus-backed, and directional tests;
- enumerator deletion before patch creation;
- an externally preserved patch containing only the three authorized production files, with path, length, SHA-256, and baseline application proof;
- external preservation of the report, delta tables, and enumerator source evidence;
- removal of disposable outputs and prototype changes;
- disposable-worktree removal;
- proof that the main worktree remained unchanged; and
- no experiment commit.

A successful experiment supplies bounded evidence only. It cannot itself classify or bind rows, mutate retained artifacts, authorize a successor, authorize production implementation, or authorize a commit. Ambiguous row disposition remains an implementation blocker requiring later independent review.

## Final finding

The corrected plan addresses every blocker in the three preserved prior reviews:

1. it supplies an exact disposable canonical-helper enumeration contract;
2. it preserves helper-emitted order and confines candidate-ID sorting to reports;
3. it reclassifies the current-source manifest mismatch as an exact expected sole provenance-control failure while preserving immutable historical bytes; and
4. it explicitly requires candidate/classification ID-set equality for both baseline and generated artifacts.

The independently run baseline hashes, sidecar checks, strict TSV parsing, ID-set comparison, canonical-test result, and helper inspection agree with those contracts. No unresolved plan-level blocker remains.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_GO`

This is experiment GO only. It grants no implementation GO and no authority beyond the exact disposable experiment plan.
