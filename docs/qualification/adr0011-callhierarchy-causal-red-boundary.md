# ADR-0011 Call-Hierarchy Causal RED Boundary

Status: **BOUNDARY FREEZE / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD for this correction: `a4cdf704085c2878a29848c568caa8a32e01c7d3`
- Preserved pre-existing worktree delta: exactly 110 added lines in `sessionruntime/b4_owned_method_pair_lease_test.go`
- Authority: boundary and future-RED specification only
- Gate under review: `ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO`
- Gate disposition: **not issued**

## Decision

ADR-0011 call-hierarchy work is frozen into two ownership layers. The shared `sessionruntime` layer may retain and expose only generic transport-custody facts produced by `Manager`. Later directional adapters exclusively own target selection and all call-hierarchy semantic interpretation.

This document authorizes no source or test mutation, no GREEN implementation, no regeneration, no evidence production, no publication, and no commit. The partial RED boundary adds no helper constructor or accessor: same-package RED may populate private attachment fields directly, while `Manager.RoundTrip` remains unchanged and ignores those fields until a later, separately authorized behavior increment.

## Layer 1 — shared `sessionruntime` transport custody

Layer 1 owns only Manager-produced, direction-neutral facts that can be observed without interpreting call-hierarchy payload semantics:

1. the exact request and response frames as transported, including exact bytes, byte spans, lengths, and digests;
2. workspace, session, generation, request-key, transaction, and predecessor joins established by Manager custody;
3. opaque correlation commitments supplied by Layer 2 for selected-item/handoff correlation, represented exhaustively as a typed opaque identity, SHA-256 digest, exact byte length, and availability where needed; Layer 1 receives no selected-item/handoff payload or preimage bytes;
4. transport terminal classification and closed resource accounting;
5. owner, lease, reservation, callback, and transport-diagnostic effects;
6. enabled-versus-disabled equality of the externally returned result bytes, exact externally observable error representation, terminal and transport effects, and all other externally visible bytes required to establish observational equivalence;
7. concurrent lifecycle behavior, including callback drain, exactly-once release, terminal admission closure, and race-safe ownership; and
8. zero transport-side refusal effects: malformed, stale, foreign, reordered, duplicate, mixed, or otherwise semantically unsuitable directional inputs do not permit `sessionruntime` to reject, rewrite, normalize, or alter the underlying transport outcome.

Layer 1 records transport custody and opaque commitment metadata, not selected-item/handoff payload custody or meaning. An unavailable measurement is distinct from a known-zero measurement. Exact deltas are preferred where an effect is naturally additive; otherwise the ledger retains typed before and after values. Every permitted transport-custody byte slice, collection, nested record, and callback view is defensively copied so callers cannot mutate owner state, and repeated views remain byte-identical after attempted mutation, callback error, or contained callback panic. This defensive-copy requirement does not authorize Layer 1 to receive, retain, expose, or reconstruct selected-item/handoff payload or preimage bytes.

### Minimal private generic types

The future scaffold is limited to two private, direction-neutral types. The field spellings below are normative for the RED contract but do not authorize their implementation.

```go
type callHierarchyTransportEffectLedger struct {
	transportMessages callHierarchyMeasuredDelta
	transportBytes    callHierarchyMeasuredDelta
	externalBytes     callHierarchyMeasuredDelta
	reservations      callHierarchyMeasuredDelta
	leases            callHierarchyMeasuredDelta
	callbacks         callHierarchyMeasuredDelta
	diagnostics       callHierarchyMeasuredDelta
	terminalBefore    callHierarchyTransportTerminalSnapshot
	terminalAfter     callHierarchyTransportTerminalSnapshot
}

type callHierarchyExternalResultSnapshot struct {
	resultBytes       []byte
	resultAvailable   bool
	error             callHierarchyExternalErrorSnapshot
	errorAvailable    bool
	terminal          callHierarchyTransportTerminalSnapshot
	transportMessages callHierarchyMeasure
	transportBytes    callHierarchyMeasure
	externalBytes     []byte
	externalAvailable bool
}
```

Supporting private measurement types must encode availability independently from value, so unavailable and known zero cannot alias. `callHierarchyMeasuredDelta` must carry typed before/after measurements or one exact typed delta with availability; it must not infer a delta from unavailable endpoints. `callHierarchyExternalErrorSnapshot` must preserve error presence and concrete class plus the exact externally observable message, data, code, and serialized bytes wherever each is applicable; each component has an independent availability bit so unavailable cannot alias present-empty or present-zero. It is a lossless custody representation, not a normalized semantic error. `externalBytes` means the finite exact externally visible byte surface needed by the equivalence test, not a decoded or canonicalized semantic representation. Any later authorized custody behavior must defensively clone all permitted mutable transport storage, including error data and serialized bytes, on ingress and egress; retained state and repeated views must remain byte/value-identical after caller mutation. No helper constructor or accessor is part of this scaffold: same-package RED accesses the private attachment fields directly. These external request/response/result/error bytes are transport custody and do not authorize selected-item/handoff correlation payload or preimage bytes.

The ledger dimensions are exact and exhaustive: transport messages, transport bytes, externally visible bytes, reservations, leases, callbacks, transport diagnostics, and terminal transition. It may not gain correlation commitment fields, selected-item/handoff payload or preimage bytes, target, direction, method-family, occurrence, member, range, group, relation, admission, publication, or downstream counters.

The external snapshot is likewise exact and generic: exact returned result bytes, exact externally observable error presence/class/message/data/code/serialized bytes as applicable, transport terminal, transport message and byte effects, and the finite externally visible bytes sufficient to compare enabled and disabled execution. It must preserve immutable defensive transport custody and present-versus-unavailable distinctions for every optional result and error component. It contains no correlation commitment field and no selected-item/handoff payload or preimage bytes. It must not parse JSON token forms, classify empty call-hierarchy results, inspect returned items, or interpret any protocol field.

Any later private Layer-1 owner, lease, view, callback, or other API that carries selected-item/handoff correlation may accept or expose only the Layer-2-supplied typed opaque identity, SHA-256 digest, exact byte length, and availability where needed. That metadata tuple is exhaustive: no raw bytes, byte slice, span, range, token, decoded field, semantic field, serialization, payload handle, preimage handle, or recovery callback is permitted. Layer 1 may compare the typed metadata values only as opaque values; it may not recompute or independently validate the digest.

## Layer 2 — later directional adapters

Incoming and outgoing adapters, under later separately reviewed gates, exclusively own:

1. target discovery and selection effects;
2. directional method-family admission and allowlist policy;
3. semantic normalization of prepared and returned call-hierarchy items;
4. incoming `from` versus outgoing `to` orientation;
5. range interpretation and occurrence production;
6. member/range ordinals, multiplicity, zero-site handling, parallel edges, and self-loops;
7. grouping and relation-admission effects;
8. qualification and publication decisions; and
9. every downstream ADR-0007 composition, grouping, admission, source, replay, and production-visible effect.

These concerns are categorically forbidden from the `sessionruntime` view and scaffold. Shared runtime types must contain no direction, target identity, semantic binding identity, semantic method allowlist, result token form, selected ordinal meaning, member/range/occurrence count, normalized endpoint, relation, grouping, admission, qualification, publication, ADR-0007 identity, or semantic diagnostic.

The sole permitted selected-item/handoff boundary is an opaque, uninterpreted correlation commitment supplied by Layer 2. Generic `sessionruntime` may store and compare only its typed opaque identity, SHA-256 digest, exact byte length, and availability where needed. It receives no selected-item/handoff payload or preimage bytes and no spans, ranges, decoded fields, semantic fields, serialization, handles, or callbacks from which those bytes or meanings can be recovered. It may not construct or recompute a semantic selection or commitment, inspect or recover target or direction, validate the preimage, meaning, or suitability, derive an occurrence, treat equality as semantic equivalence, or confer evidence or authority. Layer 2 exclusively owns the raw selected-item/handoff bytes, proves preimage-to-commitment correspondence independently in adapter RED, and owns the artifact semantics and every effect arising from them.

## Enabled/disabled causal equivalence

The complete Layer-1 causal RED must execute the same transport operation with the private scaffold disabled and enabled, then compare immutable snapshots. It must establish exact equality of:

- returned result bytes and their availability;
- exact externally observable error presence and concrete class, plus exact message, data, code, and serialized bytes as applicable, with present-versus-unavailable distinctions;
- terminal state and transition;
- transport message effects;
- transport byte effects; and
- the bounded externally visible byte surface.

Every comparison is byte-for-byte and value-for-value over the applicable representation; classification-only, normalized, reconstructed, or semantically equivalent errors are insufficient. Result bytes, error components, terminal state/transition, transport effects, and external bytes must all remain immutable under mutation attempts against directly populated private attachment fields or returned views.

It must separately prove that enabling observation introduces only the explicitly measured private observation effects and never a transport refusal, retry, extra protocol message, changed external byte, changed result, changed error, changed terminal, or changed resource outcome. A refusal-shaped attachment is a Layer-2 concern: Layer 1 transports normally and produces no semantic refusal effect.

## Complete shared causal RED obligations

A successor RED tranche is incomplete unless it causally covers all of the following:

1. Manager-produced owner identity and exact workspace/session/generation/request/predecessor joins;
2. exact request/response frame bytes, spans, lengths, and digests;
3. opaque Layer-2-supplied correlation commitment metadata custody, exhaustively limited to typed opaque identity, SHA-256 digest, exact byte length, and availability where needed, with no payload/preimage bytes, spans, ranges, decoded or semantic fields, serialization, handles, or recovery callbacks;
4. proof that Layer-1 owner/lease/view/callback APIs neither accept nor expose selected-item/handoff payload or preimage bytes and cannot recompute or independently validate the commitment, with no helper constructor or accessor introduced by this scaffold;
5. transport terminal and closed generic resource accounting, including unavailable versus known zero;
6. immutable defensive copies of permitted transport-custody bytes when later authorized behavior consumes the directly populated private attachment fields, on lease/view egress, and during repeated callback access;
7. callback return-error transparency and callback-panic containment;
8. terminal admission closure, in-flight callback drain, exact-once release, and concurrent release/terminal races;
9. enabled/disabled `callHierarchyExternalResultSnapshot` byte-for-byte and value-for-value equivalence across returned bytes, exact externally observable error representation, terminal/transport effects, and all availability distinctions, while proving that the snapshot contains no correlation commitment or selected-item/handoff payload/preimage field;
10. exact `callHierarchyTransportEffectLedger` before/after or delta accounting, while proving that the ledger contains no correlation commitment or selected-item/handoff payload/preimage field;
11. zero transport-side refusal, retry, rewrite, normalization, diagnostic, or external-byte effects for every semantically unsuitable attachment class;
12. an independent Layer-2 adapter RED that retains the raw selected-item/handoff preimage bytes and proves preimage-to-SHA-256-commitment correspondence without exposing those bytes to Layer 1; and
13. inert attachment fields: before later behavior authority, `Manager.RoundTrip` ignores them and emits no owner, ledger, snapshot, lease, callback, reservation, diagnostic, or other observation effect.

The RED must fail only because the narrowly named private scaffold or inert Manager behavior is absent. It must not require or accidentally specify directional semantics.

## Deferred obligations

The following remain deferred to later directional-adapter RED and review: custody of raw selected-item/handoff payload and preimage bytes; independent proof that each preimage produces the supplied SHA-256 commitment; construction and semantic validation of selected-item/handoff artifacts; target selection, allowlists, incoming/outgoing policy, prepare-item validity, JSON semantic normalization, successful-empty interpretation, member/range validation, occurrence construction, grouping, relation admission, qualification, publication, ADR-0007 composition, and all production-visible semantic effects.

No passing shared-runtime test can satisfy, imply, or authorize any deferred obligation.

## Exact future scaffold write-set

Subject to a later issued boundary GO and a separate explicit implementation authorization, the maximal shared-runtime scaffold write-set is the following subset of already accepted paths:

1. `sessionruntime/b4_owned_method_pair_lease.go` — private generic owner/lease/view and the two private types only; no helper constructors or accessors, global storage, parallel implementation, selected-item/handoff payload or preimage bytes, spans, semantic fields, serialization, handles, or recovery callbacks;
2. `sessionruntime/sessionruntime.go` — private attachment fields on the existing `RoundTripRequest` and `RoundTripResult` declarations, because those declarations cannot be extended from another file; same-package RED writes and reads the fields directly, while `Manager.RoundTrip` continues to ignore them; and
3. `sessionruntime/b4_owned_method_pair_lease_test.go` — shared causal RED and preservation controls proving the Layer-1 API and state exclusions only; raw-preimage-to-commitment proof remains adapter-owned.

`sessionruntime/manager.go` does not exist and is not in the write-set. `sessionruntime/manager_test.go` is also excluded: existing tests are the frozen preference, and expansion requires a new boundary review. This scaffold creates zero helper constructors or accessors, zero global storage, and zero parallel implementation. It also preserves the C15 v2 candidate surface because private struct field and type declarations add no candidate AST calls or literals.

No public package path, exported identifier, public request/result schema, command, MCP schema, retained graph schema, adapter, provider, qualification artifact, generated catalog, testdata corpus, ADR-0007 path, or unrelated source/test is in the write-set. A later reviewer may narrow this set; expansion requires a new boundary review.

## Preserved 110-line partial RED

The existing uncommitted 110-line addition in `sessionruntime/b4_owned_method_pair_lease_test.go` is preserved byte-for-byte by this documentation-only change. It is useful evidence of the intended Manager owner handoff, exact-frame custody, defensive copying, callback containment, release behavior, and candidate refusal vectors, but it is not accepted as the complete RED.

In particular, its current `TestCallHierarchyManagerRoundTripPublishesSemanticObservationLease` framing crosses the frozen boundary by asking `sessionruntime` to publish or validate semantic binding/method allowlists, selected-item meaning, response token-form meaning, successful-empty semantics, and attachment refusal. A later authorized RED revision must retain the generic causal intent while moving those directional meanings to adapter-owned tests. This document neither edits nor approves the partial RED and does not convert it into implementation authority.

## Validation and gate

This change is valid only if the worktree after authoring contains:

- this one modified documentation file;
- the pre-existing 110-line modification to `sessionruntime/b4_owned_method_pair_lease_test.go`; and
- no other delta.

The review token is:

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO
```

It is **not issued** by this document. Until an independent review explicitly issues it, the boundary remains **NO GO**. Even if issued later, it authorizes only the frozen causal-RED boundary; it does not authorize source implementation, GREEN behavior, directional adapters, evidence production, publication, ADR-0007 effects, commit, or production admission.
