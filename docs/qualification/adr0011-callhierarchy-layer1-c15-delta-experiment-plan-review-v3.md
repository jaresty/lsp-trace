# ADR-0011 Call-Hierarchy Layer-1 C15 Delta Experiment Plan Review v3

## Scope

Independent superseding review of exact repository HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`, the corrected untracked plan, and both prior plan reviews:

- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan.md`;
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan-review.md`; and
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan-review-v2.md`.

The corrected write-set amendment and its prior review were also checked for the three-production-file/eight-symbol authority boundary. This review creates only this document. It does not execute the experiment, modify the plan or prior reviews, modify source/tests/corpus/generated artifacts, authorize implementation, authorize classification or binding changes, authorize artifact succession, or authorize a commit.

## Verified corrections and contracts

### 1. Exact helper-emitted bytes and sidecar hashes are preserved

The corrected plan resolves the v2 review's ordering conflict. It now requires the candidate and classification TSVs to preserve the exact byte order returned by `generatePrivateB4C15Catalog`, forbids sorting, reordering, normalization, or rewriting before persistence, and permits lexical candidate-ID sorting only in separately produced human-readable delta tables.

At the pinned HEAD, `generatePrivateB4C15Catalog` writes candidate and classification rows directly while iterating `discoverPrivateB4C15Candidates`, then appends the accepted mapping rows, and returns the two buffer byte slices without a sorting or normalization pass. The plan requires each generated sidecar to hash those exact returned bytes and requires external copies to preserve the same bytes. This is internally consistent and executable.

The plan's exact five-file SHA-256 verification command passed at the pinned HEAD for:

- `sessionruntime/testdata/c15-census-v2/candidates.tsv`;
- `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256`; and
- `sessionruntime/testdata/c15-census-v2/manifest.json`.

The canonical baseline test reached manifest comparison only after candidate/classification byte equality and both TSV-sidecar checks, so the TSV and sidecar contracts themselves are green.

### 2. Candidate-ID-keyed maps and delta semantics are finite

The plan makes candidate ID the sole row identity; requires candidate and classification inputs to be parsed without modifying either artifact; hard-stops on duplicate IDs rather than collapsing rows; defines disjoint `ADDED`, `REMOVED`, and `CHANGED` sets; compares candidate digest bytes and classification row bytes independently; classifies every common candidate ID exactly once as changed or unchanged; and reconciles exhaustive row tables to baseline and regenerated totals.

Ordering-only differences cannot become semantic deltas because semantic comparison is map-based while report ordering is separate. Explicit baseline/regenerated classification absence is retained for affected rows. As a defensive tightening, an execution-quality revision should state `keys(candidateMap)=keys(classificationMap)` independently for baseline and generated inputs, but the canonical helper emits the two generated rows in lockstep and the pinned baseline contract already verifies paired counts and IDs. That omission does not create the present blocker.

### 3. The disposable test contract and command are exact

The authorized temporary path is exactly:

`sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go`

It must use `package sessionruntime` and contain exactly one test entry point named `TestADR0011CallHierarchyC15DeltaEnumerate`. The same-package test can call the actual private helper signatures at the pinned HEAD:

- `discoverPrivateB4C15Candidates(t *testing.T) []privateB4C15Candidate`;
- `generatePrivateB4C15Catalog(t *testing.T, mapping privateB4C15OperationMapping) ([]byte, []byte)`; and
- `readPrivateB4C15Mapping(t *testing.T) privateB4C15OperationMapping`.

`readPrivateB4C15Mapping` validates owner `15`, alias `7`, copy `22`, total `44`, and exactly `44` rows. The plan forbids copying, replacing, weakening, or reimplementing discovery, generation, or mapping validation.

The exact disposable command is unambiguous and names only the temporary test:

```sh
ADR0011_C15_DELTA_OUTPUT=<external-empty-dir> go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v
```

### 4. External-path safety is explicit

The test must refuse an unset `ADR0011_C15_DELTA_OUTPUT`, a nonempty destination, a resolved destination inside any repository worktree, and every tracked-path write. It must stage under `t.TempDir()` before copying exact bytes externally. External evidence must include the exact enumerator source bytes and SHA-256, generated outputs and sidecars, and deterministic metadata. No generated `manifest.json` is permitted.

These are objective refusal conditions. They close the first review's missing external non-writing enumeration contract without expanding the production write-set.

### 5. Cleanup and prototype-patch purity are exact

Before patch preservation, the temporary enumerator must be deleted. The preserved patch may then contain only:

1. `sessionruntime/sessionruntime.go`;
2. `sessionruntime/b4_owned_method_pair_lease.go`; and
3. `sessionruntime/owned_method_pair.go`.

The enumerator source and hash remain external evidence and may not enter the patch, retained repository state, or a commit. The plan requires patch location, byte length, SHA-256, baseline-application verification, and proof that no test, documentation, corpus, generated artifact, or fourth source file appears. It then requires removal of generated outputs, discard of prototype changes, disposable-worktree removal, unchanged-main-worktree proof, and no-commit proof.

## Blocking finding

### The canonical non-writing baseline command fails at the exact pinned HEAD

The plan identifies this command as the canonical non-writing baseline verification:

```sh
go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1 -v
```

Running that exact command in `/Users/schwa/dev/lsp-trace-adr0011-c17` at exact HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343` failed:

```text
--- FAIL: TestPrivateB4C15V2SuccessorCatalog
    b4_c15_census_v2_successor_private_test.go:85: ASSERT_C15_V2_CLEAN_REGENERATION_manifest.json
```

This is not a hash-pin failure: the plan's separate exact SHA-256 command reports all five tracked baseline files `OK`. It is a semantic reproducibility failure between the manifest regenerated from current pinned source/tool behavior and the tracked immutable `manifest.json` bytes.

The plan requires the immutable baseline to be established before prototype mutation and hard-stops unless the baseline is the exact recorded clean committed HEAD. Because its own canonical baseline verification is red at that exact HEAD, the experiment cannot establish the required immutable starting state as written. A later disposable prototype result cannot repair or waive a pre-prototype baseline failure.

## Required correction before experiment GO

A corrected plan or baseline repair must provide an independently reviewed baseline contract for the actual pinned commit and must make its canonical non-writing verification command pass before any prototype mutation. It must preserve the existing tracked-byte hashes or explicitly review and authorize any successor baseline rather than treating a failing generated-manifest comparison as acceptable.

The correction should also state, as an explicit hard stop, that baseline candidate IDs equal baseline classification IDs and generated candidate IDs equal generated classification IDs before delta computation.

All existing helper-byte preservation, exact sidecar hashing, disposable test, external-path refusal, three-file/eight-symbol ceiling, cleanup, prototype-patch purity, and authority-ceiling requirements must remain unchanged.

## Authority ceiling

This review supplies neither experiment GO nor implementation GO. It authorizes no source/test/corpus/generated-artifact mutation, classification or binding change, artifact succession, commit, or main-worktree experiment.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_BLOCKED`
