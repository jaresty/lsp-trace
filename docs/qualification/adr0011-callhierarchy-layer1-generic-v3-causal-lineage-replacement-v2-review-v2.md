# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Lineage Replacement V2 — Superseding Independent Review V2

Status: **BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `1a551a0f3c183a8f8e81b30c27256c15138fc756`
- Reviewed replacement: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`
- Supersedes: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2-review.md`
- Review mode: independent documentation-only semantic and mechanical review
- Mutation by this review: this document only
- Tests run: none
- Corpus, RED, write-set, source, test, prototype, generated artifact, implementation, writer, commit, activation, semantic-validity, public-authority, carrier acceptance, and production authority: **not issued**

## Review boundary and GO rule

The replacement was rechecked against the pinned current `sessionruntime` source and the prior full criteria. Proposed symbols were assessed only as finite documentation-lineage proposals. Their mention does not establish current existence, lawful implementation, acceptance, or production authority.

A documentation-lineage GO requires every retained production-owned outcome and conditional to name an exact lawful current or finite proposed producer, transfer, finalizer, and publication carrier. It also requires every return after proposed attempt construction to publish exactly once when enabled, preserve publication absence when disabled, preserve refusal-before-effect ordering, retain exact terminal/ledger availability semantics, and prohibit expected/world/name/pair/commitment-content/hidden-state feed. Mechanical accounting alone is insufficient.

## Mechanical, domain, and anti-feed result

The replacement preserves the exact census and ownership arithmetic:

```text
25 roots + 16 World-24 workers + 22 World-25 executions
+ 22 World-25 pair members = 85 occurrences

ordinary occurrence: A=116, B=20, C=9, total=145
World-24 root:       A=81,  B=55, C=9, total=145

A = 84*116 + 81 = 9,825
B = 84*20  + 55 = 1,735
C = 85*9        =   765
union                 12,325 = 85*145
```

Only the 35 World-24 root ledger members migrate from A to B. The exact 17 removed/deferred paths remain absent. B, C, the World-24 root aggregate, and World-25 equality retain production seam `NONE`.

The occurrence-sensitive enablement correction is exact as documentation: 11 disabled World-25 pair occurrences and 74 enabled occurrences; C agreement is limited to the 69 occurrences that physically carry controls; the 16 World-24 workers validate structural absence without borrowing or manufacturing controls.

The anti-feed boundary is explicit and adequate: expected values, C controls, world/name/pair text, commitment content, failure strings, and hidden state cannot construct or select runtime requests, enablement, refusal, observations, frames, results, terminal state, or ledgers. Registry and occurrence identity remain offline authoring inputs only. World-24 aggregation and World-25 equality remain RED-only actual-to-actual derivations.

This portion is a **mechanical and documentation-domain PASS**.

## Repaired pre-admission refusal lineage

The replacement corrects the prior review's principal blocker. It now names:

- first-line conditional construction by `newCallHierarchyLayer1GenericAttemptLineage(req)`;
- the attempt-scoped `callHierarchyLayer1GenericAttemptLineage` receiver;
- finite typed refusal production by `recordRefusal(kind, reason)`;
- immutable refusal finalization by `finishRefusal(result)`;
- owner construction by `newCallHierarchyLayer1GenericOwner`;
- private result installation through `publishCallHierarchyLayer1GenericOwner`;
- nil-receiver behavior preserving disabled publication absence; and
- refusal-before-effect ordering with no pending key, worker, transport, reservation, lease, callback, frame, or diagnostic production by the proposed refusal methods.

For the eleven §4.1 helper branches, the replacement supplies branch-specific typed failure/refusal mapping, unavailable terminal/admitted state, exact known-zero versus unavailable ledger semantics, and an immutable refusal-only publication proposal before return. The current source confirms those branches occur before `key := r.pending.Begin(req.Generation)`.

This repairs the predecessor review's missing pre-key receiver and publication proposal. It does **not**, however, close every return or accepted-transfer path.

## Blocking finding 1 — an entry-created enabled attempt can still return without finalization

The replacement requires the attempt lineage constructor at the first executable line of `(*Manager).RoundTrip` when `req.callHierarchyLayer1.enabled` is true and states that no return after lineage creation may bypass the appropriate finalizer.

Current `RoundTrip` immediately contains this branch before either helper dispatch:

```go
if req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting {
    return RoundTripResult{Failure: session.ToolNotImplemented}
}
```

That current production-owned conditional is absent from §4.1 and from the conditional decision matrix. Under the replacement's proposed first-line construction, an enabled request reaching this branch has an attempt builder but returns directly with:

- no named refusal enum for this branch;
- no exact `recordRefusal` call;
- no exact `finishRefusal` call;
- no immutable owner construction/publication; and
- no branch-specific terminal/ledger availability mapping.

The general statement that every post-construction return must finalize does not supply the missing exact branch carrier. This is a direct contradiction between the proposed construction position and the claimed exhaustive publication rule.

A lawful correction must either enumerate this branch with its exact typed refusal producer, finalizer, publication, and availability mapping, or move construction after it and reconcile the claim that construction precedes every accounting/admission check. This review does not choose or authorize either correction.

## Blocking finding 2 — accepted handoff lacks an exact transfer carrier

The replacement says the same entry-created builder is handed to `roundTripWithPrivateTransaction` and later receives `admit(key, snapshot)`. It does not name an exact proposed carrier for that handoff.

The current call chains are:

```text
RoundTrip -> roundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
RoundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
```

The replacement names no revised helper parameter, no exact `privateB4Transaction` field, no request field, and no other finite transfer object carrying `*callHierarchyLayer1GenericAttemptLineage` through those wrappers. `admit` is an exact proposed consumer, but a consumer method does not establish how its receiver reaches that call site.

Consequently the accepted conditional lacks an exact producer-to-admission transfer chain. Prose that the builder is “retained” or “handed” is not a finite proposed carrier when multiple current wrappers intervene and none has a named parameter or field for it.

A lawful correction must name the exact proposed signature/field and propagation at each applicable dispatch path, including the ordinary and private-P2 routes, while preserving one builder and disabled nil propagation. This review does not choose or authorize that carrier.

## Recheck of remaining required criteria

Subject to the two blockers above, the replacement adequately documents:

- exact §4.1 pre-admission branch refusal producers and refusal-before-effect ordering;
- disabled absence through nil builder/owner semantics;
- frame custody from emitted request and read response frames without reserialization or fabricated frames;
- exact matched result/RPC-error capture before private clearing;
- terminal success, timeout, cancellation, crash, other-failure, numeric-RPC-error, and poison mappings in `finishRoundTrip`;
- unavailable versus available-empty states;
- `known_zero` only when availability is established;
- refusal terminal unavailability rather than synthetic completion;
- admitted terminal and ordinary-ledger finalization inside the current terminal funnel;
- fixed World-24 worker order and unavailable propagation;
- independent World-25 execution/equality;
- defensive owner copying, callback admission closure, drain, panic containment, and one release winner; and
- absence of production World-24 aggregate storage or B/C production seams.

No additional production-owned conditional lacking an exact current-or-proposed carrier was identified in this bounded documentation review. That statement does not cure the two explicit gaps and does not establish implementation or source completeness.

## Exact production-owned conditionals lacking lawful carriers

1. **Manager entry accounting refusal:** enabled `RoundTrip` attempts taking `EnablePrivateC16ObjectAccounting || EnablePrivateC17EventAccounting` lack an exact refusal enum, recorder, finalizer, publication, and terminal/ledger availability mapping after the proposed first-line builder construction.
2. **Accepted attempt transfer:** the same entry-created builder lacks an exact proposed parameter/field/propagation carrier through `roundTrip` and/or `roundTripWithPrivate` into `roundTripWithPrivateTransaction` before `admit`.

## Authority disposition

The replacement is mechanically exact, closes the prior pre-key refusal receiver/publication defect for the enumerated §4.1 branches, preserves disabled absence and anti-feed boundaries, and documents terminal/ledger semantics. It is not carrier-complete because one current early return bypasses the proposed finalizers and the accepted builder handoff has no exact transfer carrier.

Therefore the required documentation-lineage GO is not issued.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V2_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V2_BLOCKED
```

This BLOCKED disposition is documentation-lineage-only. Production, corpus, RED, source, tests, prototype, write set, writer, activation, semantic validity, public authority, and carrier acceptance remain blocked.
