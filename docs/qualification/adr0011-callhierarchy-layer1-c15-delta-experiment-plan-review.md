# ADR-0011 Call-Hierarchy Layer-1 C15 Delta Experiment Plan Review

## Scope

Independent review of exact repository HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343` and the untracked corrected planning packet:

- `docs/qualification/adr0011-callhierarchy-layer1-owned-method-pair-writeset-amendment.md`;
- `docs/qualification/adr0011-callhierarchy-layer1-owned-method-pair-writeset-amendment-review.md`; and
- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan.md`.

This review creates only this document. It makes no source, test, corpus, generated-artifact, amendment, activation, or commit change. It does not authorize implementation, the experiment, C15 classification or binding changes, artifact succession, or any C15 successor.

## Verified boundaries

### 1. Exact baseline and disposable isolation are stated correctly

The plan pins the experiment to clean committed HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`, requires a newly created disposable isolated Git worktree, forbids basing the experiment on uncommitted state, requires baseline identity and cleanliness evidence, and requires the main worktree to remain unchanged throughout.

The optional advanced-HEAD clause does not silently change the baseline: it requires an intentionally advanced clean committed HEAD to be recorded before prototype mutation. Any execution packet must therefore name the actual baseline exactly; no uncommitted amendment, review, plan, or other worktree state may enter the prototype baseline.

### 2. The prototype write-set and symbol ceiling are exact

The disposable prototype is limited to exactly three existing production files:

1. `sessionruntime/sessionruntime.go`;
2. `sessionruntime/b4_owned_method_pair_lease.go`; and
3. `sessionruntime/owned_method_pair.go`.

Within `sessionruntime/owned_method_pair.go`, authority is limited to exactly these eight existing inert generic symbols:

1. `(*Manager).privateCallHierarchyRequestIdentity`;
2. `newCallHierarchyObservationLimits`;
3. `callHierarchyObservationAccounting.validAgainst`;
4. `callHierarchyObservationTerminal.valid`;
5. `newCallHierarchyOriginalRequestFrameObservation`;
6. `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`;
7. `newCallHierarchyOriginalResponseFrameObservation`; and
8. `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`.

The corrected amendment accurately treats `sessionruntime/owned_method_pair.go` as an existing tracked file containing those inert declarations. The plan correctly hard-stops on a fourth source file or ninth symbol.

### 3. Tests, documentation, corpus, and directional behavior remain excluded

The plan forbids edits or additions to tests, documentation, fixtures, corpus inputs, manifests, digest sidecars, configuration, schemas, and any fourth source file, except for disposable regenerated C15 output. It also forbids directional symbols or semantics, method triples, token/result/params interpretation, subspan accessors, selected/handoff bytes or commitments, predecessor chains, occurrence/relation behavior, public surface, activation, CLI, and MCP behavior.

The retained generic and corpus-backed RED cases may be run unchanged. Directional tests must remain pinned and untouched. If the prototype cannot satisfy the retained generic cases within the three-file/eight-symbol ceiling, the required outcome is a stop, not scope expansion.

### 4. The prototype is intended to be faithful enough to enumerate final implementation sites

The plan requires the minimum implementation sufficient to turn the retained generic and corpus-backed RED cases green, not a synthetic copy microbenchmark or alternate helper shape. It requires the exact three-file prototype diff, exact copy/allocation path for every affected row, exact source location, enclosing symbol, AST kind, and AST hash. Those constraints are appropriate for enumerating the candidate-producing sites of the contemplated final generic shape.

That conclusion remains conditional on executing the final intended code shape without renaming, inlining, relocating, or otherwise reshaping it after review. Any later implementation spelling that changes candidate identity, location, AST hash, ownership, or accounting would exceed the reviewed finite set and require renewed C15 review.

### 5. Row accounting and experiment dispositions are appropriately strict

The plan requires exhaustive added, removed, and changed candidate tables; before/after classification including explicit absence; ownership/accounting changes; predecessor/binding changes; reconciliation to baseline and regenerated totals; and independent treatment of ordering-only or serialization-only changes.

For each affected row it requires evidence for exactly one of `NECESSARY`, `REUSE`, or `FALSE_POSITIVE`, while unresolved ambiguity blocks implementation. `REUSE` may not hide a distinct copy/allocation owner, bypass accounting, or evade discovery. `FALSE_POSITIVE` requires the exact C15 rule and counter-evidence. The experiment may not alter classifications, bindings, rules, names, code shape, or corpus inputs to force a preferred result.

### 6. Patch preservation, discard, and main-worktree non-mutation are correctly required

Before discard, the plan requires an external patch containing only the exact three production files, with external location, byte length, and SHA-256, plus external preservation of the report and delta tables. It requires application verification against the recorded baseline and confirmation that the patch contains no test, documentation, corpus, generated-artifact, or fourth-source-file change.

It then requires removal of disposable outputs, discard of prototype changes, removal of the disposable worktree, proof that the main worktree remains unchanged, and proof that no commit was created.

## Blocking finding

### The plan does not provide an executable canonical C15 v2 enumeration contract

The plan says to run “the repository's exact C15 regeneration procedure” and to record the command and tool revision, but it does not name the command or the exact baseline/generated artifact paths.

At the reviewed HEAD, the canonical v2 generator contract is encoded by `sessionruntime/b4_c15_census_v2_successor_private_test.go`:

```sh
C15_V2_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15V2SuccessorCatalog$' \
  -count=1
```

Its tracked outputs are exactly:

- `sessionruntime/testdata/c15-census-v2/candidates.tsv`;
- `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256`; and
- `sessionruntime/testdata/c15-census-v2/manifest.json`.

That command cannot serve as the proposed delta enumerator for a changed prototype. Before writing outputs, `TestPrivateB4C15V2SuccessorCatalog` enforces the existing frozen final count `3847`, existing `+42` delta, predecessor byte identity, existing classifications, and unchanged owned mapping. A prototype that adds, removes, changes, or reclassifies a candidate can fail those assertions before `writePrivateB4C15V2` runs. Therefore the plan does not establish that the required regenerated artifact, exhaustive affected rows, or final C15 sites can be produced.

The retained v1 command is not a substitute:

```sh
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' \
  -count=1
```

It mutates `sessionruntime/testdata/c15-census-v1` in the disposable checkout and the retained experiment README explicitly distinguishes it from the successor-specific v2 contract. Using it without an exact reviewed comparison procedure would make the plan's repeated “C15 v2” claims ambiguous.

This is material rather than editorial. The plan forbids editing tests, generated contracts, or a fourth source file, yet supplies no reviewed non-writing command or external procedure that invokes current discovery logic and emits candidate/classification bytes despite an expected finite delta. Execution would consequently require either improvising an unreviewed enumerator, weakening/editing the pinned v2 test, using the wrong v1 output contract, or stopping after the prototype without answering the experiment question.

## Required correction before experiment GO

A corrected plan must name, verbatim:

1. the canonical non-writing baseline verification command;
2. the exact disposable enumeration/regeneration command that can emit candidate and classification outputs for an expected nonzero prototype delta without first enforcing the old v2 counts or classifications;
3. the exact code/tool revision supplying that enumeration behavior;
4. the immutable baseline inputs and their exact repository-relative paths;
5. every disposable generated output and sidecar used for comparison;
6. whether `manifest.json` is an immutable historical baseline, a disposable output, or excluded from semantic comparison;
7. the exact row identity and changed-row comparison rule used to distinguish `ADDED`, `REMOVED`, and `CHANGED`;
8. how the mechanism operates without editing tests, corpus, manifests, sidecars, schemas, or a fourth source file; and
9. the stop condition if no such already-reviewed enumeration mechanism exists.

If a new temporary enumerator, test change, helper, script, or successor generator contract is necessary, it requires its own explicit disposable write-set and independent review before this experiment can run. It may not be smuggled into “tool revision” or treated as implied by the experiment GO.

## Authority ceiling

The corrected amendment remains blocked and supplies no implementation authority. This review supplies neither experiment GO nor implementation GO. Even a later successful experiment would establish only bounded evidence about the exact preserved prototype patch and exact enumerated rows. It could inform a later reviewed C15 amendment; it could not classify or bind rows, mutate retained artifacts, authorize production implementation, authorize a commit, or authorize a C15 successor.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_BLOCKED`
