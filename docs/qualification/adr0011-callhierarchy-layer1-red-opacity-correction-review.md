# ADR0011 callHierarchy Layer 1 RED opacity correction review

## Scope

Independent strict read-only review completed against exact HEAD `b9619d1521abab548dd09b167398d93835f8df7f` in `/Users/schwa/dev/lsp-trace-adr0011-c17`.

At review time, the worktree’s intended dirty scope was exactly two files:

- modified: `sessionruntime/b4_owned_method_pair_lease_test.go`;
- created: `docs/qualification/adr0011-callhierarchy-layer1-red-opacity-correction.md`.

This review document is the only additional file created to record the completed disposition. No source, corpus, testdata, generated artifact, implementation, activation, or other qualification file was changed, and no commit was made.

## Generic Layer 1 opacity boundary

The corrected generic Layer 1 RED no longer requires, retains, reconstructs, or validates raw selected-item or handoff preimages.

The generic commitment contract is limited to:

- availability;
- opaque identity;
- SHA-256;
- exact byte length; and
- generic predecessor cardinality, opaque identity, and commitment SHA-256 metadata.

The correction removes the generic reflection requirements for `selected.exactBytes` and `handoff.successorBytes`. It also removes generic digest and length recomputation from those raw fields. The corpus and active request/result assertions continue to compare the declared opaque availability, identity, SHA-256, and length values directly. Request and result frame bytes remain generic transport evidence and are unaffected by this correction.

## Separate semantic raw-owner tests

The separate `callHierarchyObservation` semantic owner tests remain distinct from the generic Layer 1 gate. They explicitly construct raw selected and handoff inputs, test defensive copying and callback-visible raw bytes, contain callback errors and panics, drain callbacks at terminal closure, enforce exact-once release, and test concurrent release behavior.

Those tests own raw bytes because their fixtures explicitly supply them. They are not used to require raw selected-item or handoff preimages from the generic Layer 1 contract.

The focused semantic-owner verification passed all five selected tests:

```text
Go test: 5 passed in 1 packages
```

## Focused causal RED verification

The accepted corpus-backed causal RED was rerun in normal and race modes:

```text
go test ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
go test -race ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
```

Both runs exited nonzero in the expected causal RED state. Each emitted exactly `5,764` admissible implementation-gap markers:

- `5,734` `ASSERT_CH_API_GAP_*` markers;
- `30` `ASSERT_CH_CORPUS_PROPAGATION_MISSING_*` markers.

After source-line and pointer-address normalization, the normal and race assertion multisets were identical:

```text
normal_only=0
race_only=0
normal_total=5764
race_total=5764
```

The race run reported no `WARNING: DATA RACE`. Neither run reported a panic, corpus manifest/schema/source-head/aggregate/path/digest/hash/decode/root-identity mismatch, cardinality mismatch, or unrelated harness defect.

Coverage remained complete and reached the intended implementation gaps:

- `25` ordered top-level corpus worlds;
- World 24: `16` independently executed concurrent workers plus aggregate checks;
- World 25: `22` independently executed named records in `11` isolated enabled/disabled pairs.

The `25/16/22/11` cardinality guards did not fail. The corrected causal RED therefore retains the prior final causal completeness after removal of the invalid cross-layer raw-preimage assertions. Only admissible missing private API and propagation behavior remains RED.

## Implementation write-set and authority ceiling

The existing Layer 1 implementation write-set recorded at HEAD `b9619d1521abab548dd09b167398d93835f8df7f` remains sufficient:

1. `sessionruntime/sessionruntime.go`;
2. `sessionruntime/b4_owned_method_pair_lease.go`.

No behavior edit is required by this opacity correction. No additional production file, test, corpus, testdata, generated artifact, or `internal/lspwire` change is admitted.

This review grants no behavior, implementation, writer, commit, activation, semantic interpretation, directional adapter, publication, public API, or production authority beyond the authority already established by the prior implementation write-set gate. All prior stop/amend boundaries, single-writer constraints, semantic ceilings, C15 preservation requirements, and verification requirements remain in force.

## Verdict

```text
ADR0011_CALLHIERARCHY_OBSERVATION_API_CAUSAL_RED_GO
```

The corrected generic Layer 1 causal RED is accepted. The existing implementation write-set remains sufficient, and no behavior edit is required.
