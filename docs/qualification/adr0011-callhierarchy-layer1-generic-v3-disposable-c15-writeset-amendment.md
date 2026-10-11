# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set Amendment

Status: **REVIEW_GATE_PENDING / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `1419784160f199f9de2f1ee1e784d34e8e0ddd0f`
- Required starting state: clean worktree at the pinned HEAD
- Authority: minimal amendment of the accepted v2 disposable three-file boundary for qualified v3 RED only
- Disposable prototype writer authority: **not issued by this document**
- Production writer authority: **not issued**
- Commit authority: **not issued**

## Decision and preserved boundary

This amendment remains review-pending and issues no GO. Subject to a separate review and writer-naming gate, the accepted v2 disposable boundary remains exactly these three existing files and no others:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The accepted v2 amendment and review remain controlling except for the single additive v3 field authorization stated below. V1, v2, and v3 tests, corpora, fixtures, testdata, registries, manifests, digests, sidecars, generated artifacts, retained RED evidence, historical C15 artifacts, reviews, and all other documentation are immutable during the disposable experiment. A failing immutable oracle is evidence for review, never authority to alter it.

## Exact amended disposable prototype boundary

### `sessionruntime/owned_method_pair.go`

Preserve exactly the accepted eight existing inert-symbol authorizations, unchanged:

1. `(*Manager).privateCallHierarchyRequestIdentity`
2. `newCallHierarchyObservationLimits`
3. `callHierarchyObservationAccounting.validAgainst`
4. `callHierarchyObservationTerminal.valid`
5. `newCallHierarchyOriginalRequestFrameObservation`
6. `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
7. `newCallHierarchyOriginalResponseFrameObservation`
8. `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

Together with the ninth authorization—the field-only amendment below—authorize exactly this finite tenth authorization and `owned_method_pair.go` symbol, matching the qualified v3 RED exact signature:

10. `(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithPredecessors([]callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment`

This narrow setter value-returns the attachment with only its ordered opaque predecessor commitment metadata replaced by a defensive copy of the supplied slice. It preserves order and cardinality exactly, does not alias caller-owned mutable slice backing, carries only the already-defined opaque metadata shape, and carries no raw commitment bytes, preimages, or semantic meaning. It must not activate, alter, or repurpose `newCallHierarchyObservationRequest` or the older `callHierarchyObservation` attachment.

No other constructor, type, method, accessor, helper, declaration, or existing inert symbol is authorized in `owned_method_pair.go`: the finite amended boundary contains exactly ten authorizations overall—the eight preserved symbols, the ninth field-only amendment, and the exact tenth setter above. If compilation or qualified v3 RED execution requires any other exact symbol in that file, hard-stop before canonical C15 enumeration and report the finite exact symbol:

```text
BLOCKED — owned_method_pair.go symbol outside finite amended boundary required: <exact symbol>; write-set amendment and independent review required before C15
```

No open-ended symbol allowance or workaround is permitted.

### `sessionruntime/b4_owned_method_pair_lease.go`

In addition to the accepted v2 finite owner/view/lease boundary, authorize exactly one existing type declaration for one field amendment paired only with the narrow setter authorized above:

- `type callHierarchyLayer1RequestAttachment struct`: add only `predecessors []callHierarchyOpaqueCommitmentMetadata`.

The field is an ordered defensive copy of the request attachment's opaque predecessor commitment metadata and may be set only by `(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithPredecessors`. Order and cardinality are preserved exactly. Each element carries only the already-defined opaque metadata shape: availability, opaque identity, SHA-256 digest, and length. It carries no raw commitment bytes or preimages and establishes no semantic meaning.

The accepted v2 owner/view/lease symbols remain the only implementation and lifecycle seams:

1. `type callHierarchyLayer1GenericOwner struct`
2. `type callHierarchyLayer1GenericView struct`
3. `type callHierarchyLayer1GenericLease struct`
4. `newCallHierarchyLayer1GenericOwner`
5. `(RoundTripResult).privateCallHierarchyLayer1GenericLease`
6. `(callHierarchyLayer1GenericLease).View`
7. `(callHierarchyLayer1GenericLease).Callback`
8. `(callHierarchyLayer1GenericLease).Release`
9. `(*callHierarchyLayer1GenericOwner).requestTerminal`
10. `(*callHierarchyLayer1GenericOwner).waitReleased`

`newCallHierarchyLayer1GenericOwner` may defensively copy the ordered predecessor metadata from the request attachment into the immutable generic observation/view. Repeated views and callbacks must not alias caller-owned or request-owned mutable slice backing. No additional helper, constructor, accessor, owner, view, lease, lifecycle method, alternate attachment, raw-byte carrier, or semantic type is authorized.

### `sessionruntime/sessionruntime.go`

Preserve the accepted exact `RoundTripResult` generic attachment/plumbing boundary:

1. add only the private field `callHierarchyLayer1Generic` to `type RoundTripResult struct`, typed as `*callHierarchyLayer1GenericOwner`;
2. in `(*Manager).RoundTrip`, receive or construct that completed owner and assign it to the returned `RoundTripResult`;
3. only if direct assignment in `(*Manager).RoundTrip` cannot compile, thread that same owner through an already-existing private round-trip terminal return value and `(*Manager).finishRoundTrip`; and
4. on every existing early or terminal branch reached after owner creation, either transfer the owner into the returned result or terminally release it exactly once.

The existing private `RoundTripRequest.callHierarchyLayer1` attachment may carry the added ordered predecessor metadata field into this plumbing. No public/exported request or result field, new transaction path, parser, serialization, retry, diagnostic, transport behavior, result/error behavior, accounting behavior, feature flag, CLI/MCP/schema behavior, or `internal/lspwire` change is authorized. The existing `callHierarchyLayer1` and `callHierarchyObservation` result attachments must not be repurposed.

## V3 semantic ceiling

The disposable prototype may retain only ordered opaque commitment metadata derived from the explicit v3 request attachment. It must:

1. preserve predecessor order and cardinality exactly;
2. defensively copy predecessor metadata at attachment and owner/view boundaries;
3. retain only availability, opaque identity, SHA-256 digest, and length;
4. retain no raw `bytes_hex`, decoded material, commitment preimage, selected-item preimage, handoff preimage, request/result payload substitute, or other raw bytes; and
5. infer no direction, target, occurrence, grouping, relation, selected-item meaning, handoff meaning, predecessor meaning, method-family meaning, token-form meaning, semantic content, source completeness, authenticity, producer identity, qualification, publication, authority, completeness, or production use.

Default-off transparency, refusal before effect, exact external result/error/transport behavior, defensive ownership, lifecycle closure, callback drain, panic containment, callback-error transparency, exactly one release winner, and fail-closed subsequent operations remain mandatory. No parallel transaction, accounting, lifecycle, or retention machinery is permitted.

## Disposable prototype, C15, review, disposal, and production gate

The mandatory order is unchanged and strict:

1. a separate review may authorize and name exactly one disposable prototype writer with exclusive custody of the exact three files and finite symbols above;
2. that writer may implement only enough to obtain a compiling qualified v3 RED and run the canonical C15 enumerator;
3. preserve the exact three-file patch, exact implemented symbol set, canonical baseline and generated candidate/classification sets, and complete finite `ADDED`, `REMOVED`, and `CHANGED` delta with source locations, AST hashes, and pre/post classifications;
4. independently review the finite symbols, candidates, and complete delta;
5. dispose of all prototype changes and the disposable worktree without commit; and
6. only after accepted review and verified disposal may a separate production gate freeze the final finite production file/symbol/candidate write-set and name exactly one production writer.

The disposable prototype is evidence tooling, not production implementation. It must not be committed or reused presumptively. The disposable writer is not the production writer. Any fourth source file, any symbol beyond the exact ten-authorizations count—the eight preserved `owned_method_pair.go` symbols, the ninth paired attachment field-only amendment, and the exact finite tenth `owned_method_pair.go` setter—or beyond the accepted v2 finite owner/view/lease and `sessionruntime.go` boundaries, any mutable test/corpus input, raw-preimage retention, semantic requirement, inaccessible required input, ambiguous disposition, or C15 delta outside the independently reviewed finite set is an immediate hard stop; report the exact missing file or symbol and do not improvise.

Until separate review issues disposable authority and a later independent production gate issues production authority:

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITER — NOT NAMED
```

Disposition: **REVIEW_GATE_PENDING / NO GO**
