# ADR-0011 Call-Hierarchy Layer-1 C15 Delta Experiment Plan Review v2

## Scope

Independent superseding review of exact repository HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343` and the corrected untracked plan:

- `docs/qualification/adr0011-callhierarchy-layer1-c15-delta-experiment-plan.md`.

The prior blocked review is preserved unchanged. This review creates only this document. It does not execute the experiment, modify source, tests, corpus, generated artifacts, the corrected plan, or the prior review, and it grants no experiment, implementation, classification, binding, artifact-succession, or commit authority.

## Verified corrections and boundaries

### 1. The disposable enumerator can compile in the canonical helper package

The authorized file `sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go` is constrained to `package sessionruntime` with exactly one test entry point, `TestADR0011CallHierarchyC15DeltaEnumerate`. At the pinned HEAD, that same-package test can directly call the existing private test helpers with their actual signatures:

- `discoverPrivateB4C15Candidates(t *testing.T) []privateB4C15Candidate`;
- `generatePrivateB4C15Catalog(t *testing.T, mapping privateB4C15OperationMapping) ([]byte, []byte)`; and
- `readPrivateB4C15Mapping(t *testing.T) privateB4C15OperationMapping`.

`generatePrivateB4C15Catalog` already calls `discoverPrivateB4C15Candidates`. `readPrivateB4C15Mapping` verifies the pinned mapping digest and requires owner `15`, alias `7`, copy `22`, total `44`, and exactly `44` rows. The plan correctly forbids copying, weakening, replacing, or reimplementing those helpers and stops if discovery or generation is unavailable.

### 2. External-output safety is explicit and finite

The enumerator must stage the two generated TSV byte slices and their SHA-256 sidecars under `t.TempDir()` before copying the exact bytes to `ADR0011_C15_DELTA_OUTPUT`. It must refuse an unset variable, a nonempty destination, or a resolved destination inside any repository worktree, and it must not write a tracked path. The external evidence also preserves exact enumerator source bytes and SHA-256 plus deterministic metadata.

Those requirements close the prior review's missing non-writing enumeration mechanism without authorizing edits to an existing test, generator, corpus, manifest, sidecar, schema, or fourth production file. The exact command is finite and names only the disposable test:

```sh
ADR0011_C15_DELTA_OUTPUT=<external-empty-dir> go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v
```

### 3. Baseline artifacts, hashes, and commands are exact

The plan pins clean committed HEAD `9132d0759f49f9748c23442f31a1cd15dfe12343`, a newly created disposable worktree, and this non-writing baseline check:

```sh
go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1 -v
```

The five tracked C15 v2 baseline files exist at the stated paths, and their bytes at the pinned HEAD match every stated SHA-256:

- `sessionruntime/testdata/c15-census-v2/candidates.tsv` — `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2`;
- `sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256` — `b8725253368bc5257ddf85e4ef56535a31cbeb910100470c42cd138a799fb838`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv` — `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730`;
- `sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256` — `7e2cc13f1ce84854c8ff54ea6afdeb4f0d1d14cfb0edefdecdba9284f9694f53`; and
- `sessionruntime/testdata/c15-census-v2/manifest.json` — `548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf`.

The plan correctly treats `manifest.json` as immutable historical provenance, excludes it from regenerated semantic output and semantic row comparison, and requires its preserved bytes and hash to be reported.

### 4. Delta comparison is otherwise exhaustive

The plan selects candidate ID as sole row identity, compares candidate-row bytes and classification-row bytes independently, defines disjoint `ADDED`, `REMOVED`, and `CHANGED` sets, requires every common ID to be exactly changed or unchanged, and reconciles exhaustive row tables to baseline and regenerated totals. It requires explicit absence, source location, enclosing symbol, AST kind/hash, ownership/accounting changes, predecessor/binding changes, and the responsible prototype copy/allocation path for every affected row.

It also separates ordering-only and serialization-only differences from semantic changes and requires exactly one evidenced experiment disposition—`NECESSARY`, `REUSE`, or `FALSE_POSITIVE`—or an unresolved blocker.

### 5. Tooling cleanup and prototype-patch purity are explicit

Before preserving the prototype patch, the temporary enumerator must be deleted. The patch must then contain only:

1. `sessionruntime/sessionruntime.go`;
2. `sessionruntime/b4_owned_method_pair_lease.go`; and
3. `sessionruntime/owned_method_pair.go`.

The exact enumerator source and hash remain external evidence and may not enter the patch, retained repository state, or a commit. The plan requires external patch path, byte length, SHA-256, application verification against the recorded baseline, and proof that no test, documentation, corpus, generated artifact, or fourth source file is present. It then requires disposal of generated outputs, prototype changes, and the worktree, plus proof that the main worktree remained unchanged and no commit was created.

### 6. No authority creep is introduced

The plan remains an experiment plan only. It retains the three-production-file/eight-symbol ceiling; excludes directional semantics, method triples, token/result/params interpretation, subspan accessors, selected/handoff bytes or commitments, predecessor chains, occurrence/relation behavior, public surface, activation, CLI, and MCP behavior; and hard-stops instead of broadening scope. Experiment results may inform a later reviewed amendment but cannot classify or bind rows, mutate retained artifacts, authorize implementation, authorize a successor, or authorize a commit.

## Blocking finding

### Exact canonical-helper bytes conflict with mandatory candidate-ID ordering

The corrected plan imposes both of these requirements:

1. the enumerator must call `generatePrivateB4C15Catalog`, write its generated candidate and classification outputs under `t.TempDir()`, and copy those exact bytes externally; and
2. candidate ID is the sole row identity and canonical candidate-ID ordering must be used for all generated TSVs and delta tables.

At the pinned HEAD, the canonical helper does not emit either TSV in lexicographic candidate-ID order. It emits discovered candidates in sorted production-file traversal and AST traversal order, then appends the 44 mapping rows. The tracked baseline produced by that contract has 3,847 candidate rows and 3,847 classification rows, and neither file is candidate-ID sorted. The first inversion in both files is:

- previous ID: `alloc.adr0011.c17.c15.observer.contract.private.newprivateb4c15custodycontext.composite-literal.composite.literal.1`;
- next ID: `alloc.adr0011.c17.c15.observer.contract.private.newprivateb4c15custodycontext.builtin-make.make.1`.

Therefore the disposable enumerator cannot satisfy both obligations as written. Preserving the canonical helper's exact returned bytes preserves non-ID ordering. Sorting those rows before writing would no longer copy the canonical helper's exact bytes and would introduce an unreviewed transformation not specified by the enumerator contract. Treating only delta tables as sorted would contradict the explicit phrase “for all generated TSVs and delta tables.”

This is material because byte identity, sidecar hashes, ordering-only diagnostics, and independent reproducibility depend on which representation is authoritative. The experiment must not improvise that decision.

## Required correction before experiment GO

The plan must choose and state one exact representation contract:

1. preserve the canonical helper's returned TSV bytes unchanged as the generated artifacts, use candidate ID only for map-based semantic identity, and require candidate-ID ordering only for separately produced delta tables; or
2. explicitly authorize a reviewed deterministic candidate-ID sorting transformation, define whether the unsorted canonical bytes are also preserved, and define which bytes receive the generated sidecars and participate in baseline/generated byte comparison.

The correction must retain the existing external-output refusals, baseline pins, cleanup, patch-purity checks, authority ceiling, and stop conditions. No experiment may run until the corrected representation contract receives independent review and the exact gate is granted.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_C15_DELTA_EXPERIMENT_PLAN_BLOCKED`
