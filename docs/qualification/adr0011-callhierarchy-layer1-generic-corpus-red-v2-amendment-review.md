# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 amendment review

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_BLOCKED`

Reviewed independently at exact repository HEAD `e16e1bc3aa95693d84e18b2f967fa35fa04c22e0`, with the proposed v2 amendment as the sole pre-existing untracked file. This review creates no corpus, RED, implementation, behavior, activation, generated-output, or commit authority.

## Verified

### Exact repository and v1 reachability

- HEAD is exactly `e16e1bc3aa95693d84e18b2f967fa35fa04c22e0`.
- Before this review, the only worktree change was the untracked amendment `docs/qualification/adr0011-callhierarchy-layer1-generic-corpus-red-v2-amendment.md`.
- The accepted v1 corpus and `sessionruntime/b4_owned_method_pair_lease_test.go` remain present in the HEAD tree.
- Their accepted history remains reachable, including corpus/RED commits `fee5cbe0`, `ea049c2d`, `12d9af17`, and `751b0276`.
- The committed RED still names `testdata/callhierarchy-layer1-red-v1`; no v2 corpus or v2 test path exists.
- The v1 manifest independently reproduces exactly 25 worlds, aggregate byte length `431974`, and aggregate SHA-256 `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`.

### Inventory accounting

Mechanical extraction of the amendment table verifies:

- exactly 162 numbered rows, consecutively numbered `001` through `162`;
- exactly 162 unique expectation paths;
- `PRESERVE GENERIC = 133`;
- `PRESERVE OPAQUE = 12`;
- `REMOVE/DEFER = 17`;
- `133 + 12 + 17 = 162`;
- successor denominator `133 + 12 = 145`.

The first 146 rows exactly equal the common leaf schema of all 25 committed v1 root records. There are no missing or extra root-schema paths.

The 17 individual removals are exactly:

1. `frames.request.body.available`
2. `frames.request.body.bytes_hex`
3. `frames.request.body.length`
4. `frames.request.body.sha256`
5. `frames.request.payload.available`
6. `frames.request.payload.bytes_hex`
7. `frames.request.payload.length`
8. `frames.request.payload.sha256`
9. `frames.response.body.available`
10. `frames.response.body.bytes_hex`
11. `frames.response.body.length`
12. `frames.response.body.sha256`
13. `frames.response.payload.available`
14. `frames.response.payload.bytes_hex`
15. `frames.response.payload.length`
16. `frames.response.payload.sha256`
17. `result.form`

The table source rules also reconcile exactly as 117 unchanged generic v1 literals, 12 unchanged opaque v1 literals, 16 unchanged named v1 RED derivations, and 17 removals with no successor field or derivation.

### Self-contained cardinalities and projection boundary

- World 24 contains exactly 16 complete worker records.
- World 25 contains exactly 22 complete execution records and exactly 11 ordered enabled/disabled pairs.
- The projection rule is additive and corpus-derived: strict decode, delete only the 17 named paths, preserve every retained decoded value and occurrence, canonical lexical-key compact JSON with one terminal LF, secure exact manifest paths, per-file lengths/hashes, and aggregate hash over ordered exact bytes.
- The proposed v2 paths are additive and leave the accepted v1 paths reachable.
- The amendment predicts no v2 canonical hashes and contains no implementation-produced oracle output.

### Finite schema boundary

The amendment correctly limits the proposed private contract to generic identity, complete request/response frames, opaque selected/handoff/ordered-predecessor metadata, generic result/error/external observations, accounting, ledger, terminal, lifecycle, and refusal/control state. It explicitly excludes body/result/payload subspans, offsets, parsed or token forms, `result.form`, raw commitment preimages, rich predecessor lineage, and symbol/item/range/call-site/caller/callee/direction/provider semantics.

The accepted owner/view/lease production seam in `sessionruntime/sessionruntime.go` and `sessionruntime/b4_owned_method_pair_lease.go` supports the required lifecycle shape: defensive byte ownership, admission only while open, callback outside the lock, terminal admission closure, callback drain, state clearing, and exactly-one release winner. The amendment does not authorize edits to either production file and does not authorize behavior.

## Blocking contradiction: deleted v1 inputs still drive the v1 RED

The amendment requires every retained expected value and derivation to remain unchanged, requires a self-contained successor RED, and states that each removed field has no successor field or derivation. The committed v1 RED nevertheless depends causally on two removed categories:

1. `callHierarchyLayer1ExecuteCorpusRecord` obtains `RoundTripRequest.Params` from `record.Frames.Request.Payload.BytesHex` before invoking `Manager.RoundTrip` (`sessionruntime/b4_owned_method_pair_lease_test.go`, lines 466–480).
2. `callHierarchyLayer1ModeForCorpusRecord` selects the mock transport behavior from `record.Result.Form`, and the external assertion separately compares the observed form to `expected.Result.Form` (lines 423–439 and 749–751).

After projecting away `frames.request.payload.*` and `result.form`, the proposed v2 corpus no longer contains the inputs used to construct the request or select the v1 transport scenario. The amendment supplies no permitted replacement derivation. A v2 RED therefore cannot both:

- preserve the v1 user flow and every retained expected value/derivation;
- consume only committed v2 corpus bytes;
- avoid body/payload/result subspan parsing and token-form derivation;
- avoid hidden world-name tables, ambient resolvers, runtime capture, or implementation-produced expectations.

Deriving params by parsing the complete request frame would reintroduce forbidden request subspan semantics. Deriving transport mode from result bytes would reintroduce the forbidden token/form classification. Using world names or a test-local side table would be a hidden replacement semantic and would violate the claimed mechanical projection and unchanged-derivation boundary.

This is not a corpus-count defect. It is an unresolved causal-input defect in the proposed successor RED contract.

## Required correction before rereview

The amendment must explicitly reconcile the v2 RED's request construction and deterministic transport-scenario selection with the 17 removals. Any correction must identify corpus-resident generic inputs and exact derivations, account for them in the 162/145 inventory, preserve v1 retained values without runtime output, and remain outside the forbidden subspan/token/rich-semantic boundary. It must not silently use names, side tables, complete-frame parsing, or implementation behavior as replacement custody.

Until that contradiction is corrected and independently rereviewed, neither v2 corpus authoring nor v2 RED authoring is authorized. Implementation and behavior remain unauthorized regardless.

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_BLOCKED`
