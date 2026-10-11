# ADR-0011 Call-Hierarchy Layer-1 C15 Delta Experiment Plan

## Status and authority

This is a finite, disposable experiment plan only. It does not authorize implementation in the main worktree, any main-worktree mutation, source/test/documentation/corpus changes outside the disposable experiment, classification or binding changes, artifact succession, or a commit.

The experiment is blocked until this exact gate is granted:

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_GO`

No other approval string, amendment review, test result, or inferred consent satisfies the gate. There is no implementation GO in this plan.

## Question

Determine the exact C15 candidate and classification delta caused by the smallest generic Layer-1 owned-method-pair implementation sufficient to satisfy the retained generic and corpus-backed RED cases, while preserving the eight-symbol ceiling and all directional exclusions.

The experiment must answer:

1. Which candidate rows are added, removed, or changed?
2. Which exact source locations and AST hashes produce those rows?
3. What classification does each affected row have before and after regeneration?
4. Which affected rows are necessary consequences of the generic implementation?
5. Which paths can reuse existing correctly classified generic behavior without evading C15 controls?
6. Which rows, if any, are false positives under the existing C15 rules?
7. Does the result require a finite classification, binding, ownership/accounting, or successor-artifact amendment before implementation can be considered?

## Frozen baseline and isolation

Run only in a newly created disposable isolated Git worktree.

The worktree must begin at exact committed HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`. If this repository has intentionally advanced before the gate is granted, the experiment may instead use the then-current committed HEAD only when the main worktree is unchanged and clean and the replacement commit is recorded explicitly before any prototype mutation. Never base the experiment on uncommitted state.

Before prototyping, record:

- repository identity;
- baseline commit;
- clean status;
- disposable worktree path;
- baseline C15 v2 semantic artifact identity, byte length, and SHA-256;
- immutable historical `manifest.json` identity, byte length, and SHA-256;
- baseline candidate and classification row counts and candidate-ID key-set equality; and
- the exact regeneration command and tool revision.

The main worktree must remain unchanged throughout. Do not run regeneration with outputs directed into the main worktree.

## Exact disposable prototype boundary

Prototype only the generic implementation needed to turn the retained generic and corpus-backed RED cases green. The prototype write-set is exactly these three existing production files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/b4_owned_method_pair_lease.go`
3. `sessionruntime/owned_method_pair.go`

Within `sessionruntime/owned_method_pair.go`, the prototype may implement only these eight existing inert symbols:

- `(*Manager).privateCallHierarchyRequestIdentity`
- `newCallHierarchyObservationLimits`
- `callHierarchyObservationAccounting.validAgainst`
- `callHierarchyObservationTerminal.valid`
- `newCallHierarchyOriginalRequestFrameObservation`
- `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
- `newCallHierarchyOriginalResponseFrameObservation`
- `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

Except for the single temporary enumerator authorized below, do not edit or add tests, documentation, fixtures, corpus inputs, manifests, generated artifacts other than the disposable regenerated C15 output, digest sidecars, configuration, schemas, or any fourth source file. Do not add directional symbols or semantics, method triples, token/result/params interpretation, subspan accessors, selected/handoff bytes or commitments, predecessor chains, occurrence or relation behavior, public surface, activation, CLI, or MCP behavior.

If the retained RED cases cannot be satisfied within that exact boundary, stop and report the missing requirement. Do not expand the prototype.

## Explicit disposable enumerator write-set and contract

The experiment may create exactly one additional temporary file, and only in the disposable worktree:

`sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go`

That file must declare `package sessionruntime` and exactly one test entry point named `TestADR0011CallHierarchyC15DeltaEnumerate`. It is experiment tooling only, not an implementation artifact or retained test artifact. It grants no authority to modify any existing test, helper, corpus, manifest, sidecar, schema, or generated contract.

The test must call the canonical `discoverPrivateB4C15Candidates` and `generatePrivateB4C15Catalog` helpers. It may call the canonical `readPrivateB4C15Mapping`, and if it does, that retained accepted 44-row mapping is pinned: the test must validate exactly 44 accepted mapping rows before generation and stop on any validation failure. It must stop if either canonical discovery/generation helper is unavailable; it must not copy, replace, weaken, or reimplement those helpers or mapping validation.

The test must first write generated `candidates.tsv`, `candidates.tsv.sha256`, `classifications.tsv`, `classifications.tsv.sha256`, and deterministic metadata only beneath `t.TempDir()`. The candidate and classification TSV bytes must preserve the exact order emitted by the canonical helpers: the enumerator must not sort, reorder, normalize, or rewrite rows before writing them, and each SHA-256 sidecar must hash those exact helper-emitted bytes. It may then copy those exact bytes to the exact external experiment evidence directory supplied by `ADR0011_C15_DELTA_OUTPUT`. It must refuse to run when the environment variable is unset, when the named output directory is nonempty, or when its resolved path is inside any repository worktree. It must not write any tracked path. The external output must preserve the enumerator source bytes and SHA-256 in addition to the generated outputs and deterministic metadata.

The exact disposable enumeration command is:

```sh
ADR0011_C15_DELTA_OUTPUT=<external-empty-dir> go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v
```

The code/tool revision is the recorded baseline commit plus the preserved three-production-file prototype patch and the externally preserved exact enumerator source bytes/hash. No other generator revision is permitted.

Before preserving the prototype patch, delete `sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go` from the disposable worktree and verify the patch contains only the three authorized production files. The enumerator source is evidence preserved externally; it must not appear in the prototype patch, retained repository state, or a commit.

## Procedure

### 1. Establish immutable baseline

The immutable semantic baseline at the recorded commit is exactly the tracked `candidates.tsv` and `classifications.tsv`, their digest sidecars, their exact parsed row counts, and equality of their candidate-ID key sets. Verify the two TSVs against the hashes declared by their bare-hash sidecars by binding each sidecar value to its corresponding filename and passing the resulting checks to `shasum -c`:

```sh
(
  cd sessionruntime/testdata/c15-census-v2
  printf '%s  %s\n' "$(tr -d '\r\n' < candidates.tsv.sha256)" candidates.tsv \
    | shasum -a 256 -c -
  printf '%s  %s\n' "$(tr -d '\r\n' < classifications.tsv.sha256)" classifications.tsv \
    | shasum -a 256 -c -
)
```

Use an exact TSV row parser, not `wc -l`, to require exactly 3,847 data rows in each TSV, reject malformed rows, empty or duplicate candidate IDs, and require exact equality of the candidate and classification candidate-ID sets. Any sidecar mismatch, parse failure, count other than 3,847, duplicate ID, or candidate/classification ID-set difference is a hard stop.

Then run the canonical v2 test exactly:

```sh
go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1 -v
```

This command is an expected provenance control, not a green-baseline requirement. Require it to fail with the exact sole failure `ASSERT_C15_V2_CLEAN_REGENERATION_manifest.json`. If it passes, fails earlier, reports a different failure, or reports more than that one failure, hard stop. The expected sole failure demonstrates that current-source clean regeneration no longer reproduces the immutable historical manifest after source advancement; it does not invalidate the accepted TSV semantic baseline.

Also verify the exact tracked bytes with:

```sh
printf '%s  %s\n' \
  536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2 sessionruntime/testdata/c15-census-v2/candidates.tsv \
  b8725253368bc5257ddf85e4ef56535a31cbeb910100470c42cd138a799fb838 sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256 \
  494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730 sessionruntime/testdata/c15-census-v2/classifications.tsv \
  7e2cc13f1ce84854c8ff54ea6afdeb4f0d1d14cfb0edefdecdba9284f9694f53 sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256 \
  548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf sessionruntime/testdata/c15-census-v2/manifest.json | shasum -a 256 -c -
```

The immutable baseline inputs are exactly:

- `sessionruntime/testdata/c15-census-v2/candidates.tsv` — SHA-256 `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2`, exactly 3,847 parsed candidate rows;
- `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256` — SHA-256 `b8725253368bc5257ddf85e4ef56535a31cbeb910100470c42cd138a799fb838`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv` — SHA-256 `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730`, exactly 3,847 parsed classification rows with a candidate-ID key set exactly equal to the candidate TSV key set;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256` — SHA-256 `7e2cc13f1ce84854c8ff54ea6afdeb4f0d1d14cfb0edefdecdba9284f9694f53`; and
- `sessionruntime/testdata/c15-census-v2/manifest.json` — SHA-256 `548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf`.

Copy or otherwise preserve those baseline C15 v2 artifacts for byte-for-byte comparison. Treat them as immutable and record their SHA-256 values, byte lengths, schema/version identity, row counts, candidate-ID key-set equality, and locations. The two TSVs, their sidecars, their exact 3,847-row counts, and their equal ID key sets are the immutable semantic baseline at the recorded commit.

`manifest.json` remains immutable historical provenance bytes. Preserve and record its exact bytes, hash, byte length, and location, and never regenerate or rewrite it. Because source advanced after v2 acceptance, exclude the manifest from current-source regeneration equality, generated semantic output, and semantic delta comparison. Its expected canonical-test mismatch is reported only as the provenance control above; it is not a green baseline and is not a semantic delta.

### 2. Build the bounded prototype

Make the minimum changes in the exact three-file disposable write-set. Do not amend tests or corpus data to obtain green results. Run only the retained generic and corpus-backed checks needed to establish that the prototype is sufficient for the experiment; leave every directional test pinned and untouched.

Record the prototype diff and confirm no path outside the exact three files changed before regeneration.

### 3. Enumerate C15 through the disposable contract

Create only the authorized temporary enumerator file and run exactly:

```sh
ADR0011_C15_DELTA_OUTPUT=<external-empty-dir> go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v
```

The exact disposable generated semantic outputs are `<external-empty-dir>/candidates.tsv`, `<external-empty-dir>/candidates.tsv.sha256`, `<external-empty-dir>/classifications.tsv`, and `<external-empty-dir>/classifications.tsv.sha256`, plus deterministic metadata and the preserved enumerator source bytes/hash. The TSVs and their sidecars must remain the exact helper-emitted bytes and hashes produced under the enumerator contract; no comparison or reporting step may sort or rewrite them. Verify each generated TSV against its generated sidecar through the same explicit filename binding and `shasum -c` method used for the baseline. No generated `manifest.json` is permitted: never regenerate, copy, synthesize, or rewrite the historical manifest. Regenerated artifacts are experimental outputs only and must not be copied into or committed in any repository worktree.

Parse both generated TSVs with the same exact parser used for the baseline. Reject malformed rows, empty or duplicate candidate IDs, and require exact equality of the generated candidate and classification candidate-ID key sets before computing any delta; inequality is a hard stop. Retain the generated C15 v2 candidate and classification artifacts long enough to perform an exact comparison against the immutable TSV semantic baseline only.

### 4. Enumerate the complete delta

Parse the immutable TSV semantic baseline and generated candidate and classification rows into candidate-ID-keyed maps without modifying any artifact. Candidate ID is the sole row identity. A duplicate candidate ID in any baseline or generated candidate or classification input is a hard stop; do not continue comparison or collapse duplicate rows. Before comparison, require candidate/classification candidate-ID key-set equality independently for the baseline pair and the generated pair; either mismatch is a hard stop. The preserved historical manifest does not participate in parsing or delta computation. Compute the sets by candidate ID:

- `ADDED`: candidate IDs present only in generated output;
- `REMOVED`: candidate IDs present only in the immutable baseline; and
- `CHANGED`: the same candidate ID exists in both, but candidate-row digest bytes or classification-row bytes differ.

Compare candidate bytes and classification bytes independently so ordering-only differences cannot be mislabeled as semantic changes. Added and removed sets must be disjoint, and every common ID must be classified exactly once as changed or unchanged. Sort only the human-readable delta report tables lexically by candidate ID; never apply that report ordering to either generated TSV or its hash input.

For every affected candidate row, record at minimum:

- stable candidate identity or the complete identity fields used by C15 v2;
- delta kind: `ADDED`, `REMOVED`, or `CHANGED`;
- exact repository-relative source file;
- exact source span or location;
- enclosing symbol;
- AST kind and AST hash;
- baseline classification, including explicit absence;
- regenerated classification, including explicit absence;
- ownership/accounting fields that changed;
- predecessor or binding fields that changed, if any; and
- the exact prototype copy/allocation path responsible for the row.

Do not summarize away duplicate-looking rows. Reconcile the exhaustive row tables to baseline and regenerated totals.

### 5. Compare C15 v2 immutably

Compare the regenerated candidate and classification TSVs with the preserved immutable TSV semantic baseline without modifying any comparison input. Do not regenerate, rewrite, or compare generated state to `manifest.json`. Record:

- baseline and regenerated SHA-256 and byte lengths;
- baseline and regenerated candidate counts;
- baseline and regenerated classification counts;
- exact added/removed/changed counts;
- unchanged-row count when derivable under the C15 v2 identity rules;
- schema/version equality or any mismatch; and
- whether ordering-only or serialization-only differences exist independently of semantic row changes.

A nonzero delta is evidence to classify, not a failure to suppress. A zero delta must be demonstrated by exhaustive comparison and must not be presumed.

### 6. Classify the experiment findings

For every affected row, assign exactly one experiment disposition with evidence:

- `NECESSARY` — the row is an unavoidable consequence of the bounded generic implementation and must be admitted by a later reviewed C15 amendment;
- `REUSE` — an existing correctly classified generic path can be reused without hiding a distinct allocation/copy owner, bypassing accounting, or evading candidate discovery; or
- `FALSE_POSITIVE` — the row is produced by the current C15 rule despite not representing the controlled candidate class, with the exact rule and counter-evidence identified.

Do not change classifications, bindings, rules, names, code shape, or corpus inputs during the experiment to force a preferred disposition. If a disposition is ambiguous, record it as unresolved and block implementation.

### 7. Preserve evidence outside the repository

Before discarding the disposable worktree, create a patch containing only the exact three-file prototype. Preserve that patch outside every repository worktree and record:

- external patch location;
- patch byte length; and
- patch SHA-256.

Also preserve outside the repository the experiment report and exact delta tables needed for independent review. Do not add them to the repository under this plan.

Verify that the preserved patch applies to the recorded baseline only as expected and contains no test, documentation, corpus, generated-artifact, or fourth-source-file change.

### 8. Discard the experiment

After evidence preservation and review handoff:

1. remove all disposable generated outputs;
2. discard all disposable prototype changes;
3. remove the disposable worktree; and
4. verify the main worktree remains unchanged from its pre-experiment state.

Do not commit any experiment state or patch.

## Stop conditions

Stop without broadening the experiment if:

- the required gate is absent;
- the baseline is not the exact recorded clean committed HEAD;
- the main worktree changes;
- a fourth source file is required;
- any test edit other than creating the exact authorized temporary enumerator, or any documentation, fixture, corpus, manifest, sidecar, schema, or configuration edit is required;
- a ninth `owned_method_pair.go` symbol or any directional symbol is required;
- either canonical `discoverPrivateB4C15Candidates` or `generatePrivateB4C15Catalog` is unavailable;
- the retained accepted 44-row mapping cannot be read and validated exactly;
- the enumerator cannot enforce unset/nonempty/inside-repository output refusal or would write a tracked path;
- generated candidate or classification TSV bytes do not preserve exact canonical helper-emitted order, or a sidecar does not hash the exact emitted bytes;
- any baseline or generated candidate or classification input contains a duplicate candidate ID;
- either immutable baseline TSV does not parse to exactly 3,847 data rows, or the baseline candidate/classification candidate-ID sets differ;
- the generated candidate/classification candidate-ID sets differ;
- either baseline or generated TSV fails verification against its corresponding sidecar through `shasum -c`;
- the canonical v2 test does not fail with exact sole failure `ASSERT_C15_V2_CLEAN_REGENERATION_manifest.json`, including if it passes, fails earlier, fails differently, or reports multiple failures;
- any step would regenerate, rewrite, or use `manifest.json` in current-source equality or semantic delta comparison;
- C15 enumeration cannot be reproduced or reconciled;
- an affected row cannot be tied to an exact source location and AST hash;
- the C15 v2 baseline would need mutation;
- evidence cannot distinguish necessity, legitimate reuse, false positive, or unresolved ambiguity; or
- preserving evidence would require a repository commit.

## Required report

The independent-review packet must contain:

1. baseline commit and cleanliness proof;
2. disposable worktree identity;
3. exact prototype three-file diff summary;
4. retained generic and corpus-backed RED-to-green result, with directional skips unchanged;
5. exact baseline verification and disposable enumeration commands, recorded code/tool revision, and externally preserved enumerator source location, SHA-256, and byte length;
6. immutable baseline and generated C15 v2 paths, identities, SHA-256 values, lengths, exact parsed counts, sidecar `shasum -c` results, and candidate/classification candidate-ID key-set equality, with immutable `manifest.json` bytes/hash/length/location reported only as historical provenance and the exact sole canonical-test failure recorded as the expected provenance control;
7. exhaustive added, removed, and changed row tables with source locations, AST hashes, and before/after classifications;
8. reconciled delta totals;
9. per-row `NECESSARY`, `REUSE`, `FALSE_POSITIVE`, or unresolved determination with evidence;
10. required follow-up amendments or blockers;
11. external patch location, SHA-256, and byte length, plus proof that the enumerator was removed first and the patch contains only the three production files;
12. proof that the disposable worktree was discarded; and
13. proof that the main worktree was not mutated and no commit was created.

## Gate disposition

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_PENDING`

The experiment may run only after explicit `ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_GO`. This plan grants no implementation GO, no main-worktree mutation authority, and no commit authority.
