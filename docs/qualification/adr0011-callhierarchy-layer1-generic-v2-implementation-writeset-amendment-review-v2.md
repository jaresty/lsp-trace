# ADR-0011 Call-Hierarchy Layer-1 Generic V2 Implementation Write-Set Amendment Review v2

Status: **DISPOSABLE C15 WRITE-SET GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `c321a96e5ce6ffea8f8550ae0d8d9695cece53c8`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-implementation-writeset-amendment.md`
- Preserved blocked review: `docs/qualification/adr0011-callhierarchy-layer1-generic-v2-implementation-writeset-amendment-review.md`
- Review mutation: this document only
- Authority: one finite disposable isolated prototype and canonical C15 experiment only
- Production writer authority: **not issued**
- Production mutation authority: **not issued**
- Commit authority: **not issued**

## Independent rereview

The corrected amendment resolves both blockers in the preserved review without erasing or superseding its historical finding. The earlier `ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_IMPLEMENTATION_WRITESET_BLOCKED` remains the reason this corrected ordering and finite symbol boundary are required.

## Exact disposable boundary

A separate gate may name exactly one disposable prototype writer with exclusive custody of exactly these three physical files in a newly created disposable worktree:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

No fourth source file is admitted.

Within `sessionruntime/owned_method_pair.go`, the disposable writer is restricted to exactly these eight existing inert symbols:

1. `(*Manager).privateCallHierarchyRequestIdentity`
2. `newCallHierarchyObservationLimits`
3. `callHierarchyObservationAccounting.validAgainst`
4. `callHierarchyObservationTerminal.valid`
5. `newCallHierarchyOriginalRequestFrameObservation`
6. `callHierarchyOriginalRequestFrameObservation.copyFrameBytes`
7. `newCallHierarchyOriginalResponseFrameObservation`
8. `callHierarchyOriginalResponseFrameObservation.copyFrameBytes`

A ninth symbol is not implicitly admitted. Its necessity is a hard stop before canonical C15 enumeration and requires a new write-set amendment and independent review.

## Finite attachment and lifecycle seams

The `sessionruntime/sessionruntime.go` allowance is finite: add only the private `RoundTripResult.callHierarchyLayer1Generic` field, construct or receive the same completed owner in `(*Manager).RoundTrip`, assign or transfer that owner through the already-existing private terminal return and `(*Manager).finishRoundTrip` only if direct assignment cannot compile, and transfer or terminally release the created owner exactly once on every reached early or terminal branch. It does not admit another transaction path, result behavior, accounting behavior, diagnostic, retry, parser, serialization, public API, or transport change.

The `sessionruntime/b4_owned_method_pair_lease.go` allowance is also finite and restricted to the ten named owner/view/lease symbols:

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

These seams may reuse the adjacent mutex/condition-variable lifecycle pattern only for immutable projection, defensive copying, acquisition refusal, callback admission and drain, panic containment, callback-error transparency, terminal closure, one release winner, and fail-closed later operations. Any additional owner, view, lease, helper, closure, drain/lifecycle method, alternate attachment, or other symbol is a hard stop before C15.

## Custody ordering

The corrected ordering is non-circular and mandatory:

1. independently authorize and name one disposable prototype writer for the exact boundary above;
2. obtain only enough disposable implementation to compile the immutable RED and run the canonical C15 enumerator;
3. preserve the exact three-file patch, exact symbol set, baseline/generated candidate and classification sets, and complete finite `ADDED`, `REMOVED`, and `CHANGED` delta with locations, AST hashes, and pre/post classifications;
4. independently review the finite symbols, candidates, and complete delta;
5. remove the prototype changes and disposable worktree without commit; and
6. only then conduct a separate C15-informed production review that freezes the final production file/symbol/candidate write-set and may name exactly one production writer.

The disposable writer is not the production writer. Disposable custody grants no presumptive production custody, patch-reuse authority, production mutation, implementation GO, or commit authority. A production writer must not be named and production custody must not be issued before the independent C15 review has accepted the finite experiment result and disposal has completed.

## Hard stops and semantic ceiling

The amendment correctly makes each of the following an immediate stop rather than permission to improvise: any ninth `owned_method_pair.go` symbol; any extra lease/sessionruntime symbol; any fourth source file; any delta outside the independently reviewed finite set; any inaccessible required input; any ambiguous disposition; any semantic requirement; or any failing immutable oracle that cannot be satisfied within the exact disposable boundary.

The disposable prototype remains transport-generic and default-off. It may retain only Manager-produced transport custody and opaque commitment metadata within the frozen contract. It may not parse or classify payload meaning; retain selected-item or handoff preimage bytes; infer direction, target, occurrence, grouping, relation, method-family, token form, qualification, publication, authority, completeness, or production use; repurpose existing attachments; add public/exported API, persistence, replay, catalog, logging, telemetry, configuration, activation, CLI/MCP/schema behavior, diagnostics, retries, or parallel transaction/accounting machinery; or mutate tests, corpus, fixtures, manifests, digests, sidecars, generated artifacts, historical C15 artifacts, reviews, or other documentation.

The required default-off transparency, refusal-before-effect ordering, external result/error/transport equivalence, defensive ownership, terminal callback drain, exactly-one release winner, and fail-closed post-terminal behavior remain mandatory. Passing the disposable experiment establishes bounded C15 evidence only and cannot promote semantic or production authority.

## Finding

The corrected amendment now provides an exact, finite, reviewable disposable write-set while preserving the three-file boundary, exact eight-symbol ceiling, bounded attachment/lifecycle seams, immutable evidence, explicit hard stops, semantic ceiling, and production-custody barrier required by the prior blocked review and accepted C15 experiment plan.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V2_DISPOSABLE_C15_WRITESET_GO`

This disposition authorizes only a separately named disposable prototype writer and canonical C15 experiment under the corrected amendment. It does not authorize a production writer, production-file mutation, reuse of a disposable patch, a production implementation, a commit, or any semantic promotion.
