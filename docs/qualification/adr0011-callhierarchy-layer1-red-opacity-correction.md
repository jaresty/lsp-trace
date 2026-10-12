# ADR0011 callHierarchy Layer 1 RED opacity correction

Status: `RED_CORRECTED_PENDING_REREVIEW`; no GO is issued by this correction.

Baseline: clean `b9619d1521abab548dd09b167398d93835f8df7f` in `/Users/schwa/dev/lsp-trace-adr0011-c17`.

## Audited defect

`LAYER1_RED_CROSS_LAYER_ASSERTION_DEFECT` made the generic Layer 1 RED scaffold cross the opaque commitment boundary. The generic contract is limited to commitment availability, identity, SHA-256, length, and generic predecessor opaque metadata. It does not own selected-item or successor-request semantic preimages.

## Exact assertion correction

In `sessionruntime/b4_owned_method_pair_lease_test.go`, the generic scaffold reflection requirements removed exactly:

- `selected.exactBytes` as a required `[]byte` field path;
- `handoff.successorBytes` as a required `[]byte` field path.

The active generic commitment assertion removed exactly these raw-preimage-derived clauses:

- `view.selected.exactSHA256 != sha256.Sum256(view.selected.exactBytes)`;
- `uint64(len(view.selected.exactBytes)) != selected.length`;
- `view.handoff.successorSHA256 != sha256.Sum256(view.handoff.successorBytes)`;
- `uint64(len(view.handoff.successorBytes)) != handoff.length`.

The corrected generic assertion retains:

- selected availability through `view.selected != nil`;
- selected SHA-256 through `view.selected.exactSHA256 == selected.sha256`;
- selected length through `view.selected.sourceLength == selected.length`;
- handoff availability through `view.handoff != nil`;
- handoff SHA-256 through `view.handoff.successorSHA256 == handoff.sha256`;
- handoff length through `view.handoff.successorLength == handoff.length`;
- predecessor cardinality, identity, and commitment SHA-256;
- request/result opaque commitment metadata comparison, including availability, identity, SHA-256, and length.

## Boundary rationale

Layer 1 receives opaque commitment metadata. Requiring Layer 1 to possess a raw preimage and recompute its digest or length converts an opaque metadata contract into a semantic-byte custody contract and incorrectly couples the generic scaffold to an owner outside Layer 1. Direct comparison of declared opaque metadata is the Layer 1 boundary; raw-byte provenance and semantic correctness remain the responsibility of the owner that explicitly supplies those bytes.

The corpus was not changed. Its commitment records were already opaque and remain authoritative for availability, identity, SHA-256, and length only.

## Preserved semantic-owner tests

The separate `callHierarchyObservation` owner lifecycle/raw-byte tests were not changed. In particular, the fixture and tests that explicitly construct `selected.exactBytes` and `handoff.successorBytes`, mutate callback-visible raw bytes, verify defensive copying, exercise callback error/panic transparency, and verify exact-once release remain intact. Those tests own raw bytes explicitly and are outside this generic Layer 1 correction.

## Verification

Commands were run from the exact baseline worktree after the scoped test edit.

### Compile

```text
go test ./sessionruntime -run '^$' -count=1
Go test: No tests found
SESSIONRUNTIME_COMPILE_OK
```

The package compiled successfully.

### Focused normal RED

```text
go test ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
FOCUSED_NORMAL_EXIT=1
```

The non-zero result is the expected aggregate Layer 1 RED state, not a panic. It retained 5,764 emitted RED marker occurrences across the same two marker families as baseline.

### Focused race RED

```text
go test -race ./sessionruntime -run '^TestCallHierarchyLayer1AcceptedCorpusRED$' -count=1
FOCUSED_RACE_EXIT=1
```

The race run retained the same 5,764 emitted RED marker occurrences, reported no `WARNING: DATA RACE`, and did not panic.

### RED-gap stability

After stripping source line numbers and normalizing nondeterministic pointer addresses, the baseline, corrected normal, and corrected race outputs had identical assertion-line multisets:

```text
before-normal: left_only=0 right_only=0
normal-race:   left_only=0 right_only=0
```

No baseline runtime failure line named `selected.exactBytes` or `handoff.successorBytes`, because those fields existed and the defective generic requirements therefore did not emit a gap. The intended raw-preimage gap removal is instead the exact source-level removal listed above. All emitted unrelated RED gaps remained stable; no unrelated assertion was weakened or removed.

### Scope and C15

The final intended write set is exactly:

- modified: `sessionruntime/b4_owned_method_pair_lease_test.go`;
- created: `docs/qualification/adr0011-callhierarchy-layer1-red-opacity-correction.md`.

No source, corpus, testdata, generated artifact, other qualification document, or commit is changed. An explicit comparison against `b9619d1521abab548dd09b167398d93835f8df7f` reported `C15_PATHS_UNCHANGED_FROM_b9619d1`; C15 remains unchanged.

## Disposition

Any prior Layer 1 RED GO or equivalent acceptance that relied on the removed cross-layer raw-preimage assertions is invalidated. The corrected RED is `PENDING_REREVIEW`. This document does not grant GO, implementation authority, activation authority, publication authority, or commit authority.
