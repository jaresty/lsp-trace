# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Lineage Replacement V2 — Final Superseding Independent Review V3

Status: **DOCUMENTATION-LINEAGE GO / PRODUCTION BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `1a551a0f3c183a8f8e81b30c27256c15138fc756`
- Reviewed replacement: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`
- Supersedes:
  - `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2-review.md`
  - `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2-review-v2.md`
- Review mode: final independent documentation-only semantic and mechanical review
- Mutation by this review: this document only
- Tests run: none
- Corpus, RED, source, test, prototype, generated artifact, write set, writer, implementation, activation, carrier acceptance, public authority, semantic authority, and production authority: **not issued**

## Review rule and evidence boundary

Assumption: a documentation-lineage GO is permitted only when every retained production-owned outcome and conditional has an exact current or finite proposed producer, transfer, finalizer, and publication carrier. Basis: the two predecessor reviews withheld GO for concrete breaks in that chain rather than for missing implementation authority. Conclusion: this review treats mechanical accounting as necessary but insufficient and does not treat proposed symbols as implemented, accepted, or production-authorized.

The review rechecked the replacement against the committed inventory and review, attachment-enable amendment lineage, both superseded replacement reviews, and current `sessionruntime/sessionruntime.go` at the pinned HEAD. Current-source facts establish branch and wrapper topology only. Proposed symbols remain `PROPOSED_NORMALIZATION`; RED-only and fixture-only rows retain production seam `NONE`.

## Mechanical and occurrence-sensitive result

Assumption: retained-path completeness is occurrence-keyed, with only the World-24 root ledger migration changing ownership. Basis: the replacement preserves the finite census and explicit category arithmetic. Conclusion: the mechanical inventory passes exactly:

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

Only the 35 members of the seven World-24 root ledger dimensions migrate from A to B. The exact 17 removed/deferred paths remain absent. B, C, World-24 aggregation, and World-25 equality retain production seam `NONE`.

Assumption: enablement must be independently causal and C controls may only check agreement where physically present. Basis: the replacement freezes 11 disabled World-25 pair occurrences and 74 enabled occurrences, limits C agreement to the 69 occurrences carrying controls, and preserves structural absence for all 16 World-24 workers. Conclusion: the prior all-85 C contradiction is closed without borrowing, manufacturing, or false-defaulting worker controls.

## Exact correction 1 — Manager accounting refusal publication

Assumption: because the proposed enabled attempt is created at the first executable line of `(*Manager).RoundTrip`, the current first branch must finalize and publish that attempt before returning. Basis: current source has the exact production-owned conditional

```go
if req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting {
    return RoundTripResult{Failure: session.ToolNotImplemented}
}
```

before either helper dispatch. The corrected replacement now places this branch immediately after proposed attempt construction and names the complete chain:

```text
producer:
  callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented
  session.Failure("private-accounting-not-implemented")
transfer:
  attemptLineage.recordRefusal(kind, reason)
finalizer:
  attemptLineage.finishRefusal(result)
publication:
  newCallHierarchyLayer1GenericOwner
  publishCallHierarchyLayer1GenericOwner
  RoundTripResult.callHierarchyLayer1Generic
return:
  existing public ToolNotImplemented result
```

Conclusion: predecessor V2 blocking finding 1 is closed. Enabled execution publishes exactly one immutable refusal-only owner before return; disabled execution has a nil attempt and no owner. Terminal and admitted state remain unavailable. Unimplemented private-accounting values remain unavailable, while only enumerated effects proven untouched by the refusal-before-effect branch are available-known-zero. No transport, pending key, worker, reservation, lease, callback, frame, diagnostic, or other admitted effect precedes refusal finalization.

## Exact correction 2 — explicit builder transfer through every wrapper

Assumption: an accepted entry-created builder reaches post-key `admit` lawfully only if each intervening ordinary/private wrapper carries the same pointer explicitly. Basis: current source has two entry routes:

```text
RoundTrip -> roundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
RoundTrip -> roundTripWithPrivate -> roundTripWithPrivateTransaction
```

The corrected replacement adds the final parameter

```go
attempt *callHierarchyLayer1GenericAttemptLineage
```

to the proposed signatures of `roundTrip`, `roundTripWithPrivate`, and `roundTripWithPrivateTransaction`, and requires every declaration and call site to pass unchanged the value received from its caller. Enabled routes carry the one pointer created at `RoundTrip` entry; disabled routes carry nil. No Manager field, package global, request-hidden state, context value, side table, reconstruction, or replacement builder is permitted.

Current source allocates the request key at:

```go
key := r.pending.Begin(req.Generation)
```

The corrected proposal calls `attempt.admit(key, snapshot)` immediately after that allocation and before request registration, identity capture, protocol ownership, worker allocation, or unlock. Conclusion: predecessor V2 blocking finding 2 is closed for the ordinary route and the direct private route, through every applicable wrapper and call site, with one-pointer identity preserved to post-key admission.

## Exhaustive producer-transfer-finalizer-publication recheck

Assumption: a remaining blocker exists if any retained production-owned conditional can produce an enabled outcome without reaching its exact proposed finalizer/publication path, or if any retained value lacks a finite current/proposed producer and transfer. Basis: the pinned current source and corrected replacement produce the following complete partition.

| Conditional family | Producer | Transfer | Finalizer | Publication / status |
|---|---|---|---|---|
| attachment disabled/enabled | explicit proposed attachment bit, setter, first-line `RoundTrip` read, conditional attempt constructor | nil or one attempt pointer through all wrappers | `finishRefusal` or `finishAccepted` | nil when disabled; exactly one private owner when enabled |
| Manager private-accounting refusal | exact current first branch plus named refusal enum and typed private reason | `recordRefusal` on entry-created attempt | `finishRefusal` before return | owner helper into private result; public `ToolNotImplemented` preserved |
| every other pre-admission refusal | exact §4.1 current branch and typed current failure | branch-local `recordRefusal` | `finishRefusal` immediately before return | immutable refusal-only owner when enabled; nil when disabled |
| accepted admission | current post-check `pending.Begin` | explicit attempt parameter through ordinary/direct-private wrapper chains | `admit` immediately post-key, then `finishAccepted` in terminal funnel | exactly one admitted owner |
| request/response frame custody | current emitted write frame and emitted/read response frame | proposed capture methods on same attempt | `finishAccepted` | admitted owner; no fabricated or reserialized frame |
| result/RPC error | current matched-response assignments before private clearing | `captureResult` on same attempt | `finishAccepted` | admitted owner preserving unavailable/available-empty distinctions |
| terminal success/timeout/cancel/crash/other and numeric RPC error | exact current `finishRoundTrip` branches | exact terminal/failure and after snapshot | `finishAccepted` | admitted owner only; refusal terminal remains unavailable |
| accounting, transport, external, and ordinary ledger availability/zero | exact current counters plus occurrence-local snapshots, or refusal no-effect mapping | same attempt snapshots/captures | refusal or accepted finalizer as applicable | unavailable is never fabricated zero; `known_zero` requires availability |
| owner lifecycle, view/callback/release/drain | proposed owner/lease methods | private lease | terminal close, callback drain, one release winner | production lifecycle remains proposed and blocked; B is RED-only |
| World-24 aggregate and World-25 equality | independently executed actual observations | RED-only derivation | RED-only | production seam `NONE` |

All current returns before key allocation are covered by the Manager branch or §4.1 refusal mapping. All returns after key allocation already use the current `finishRoundTrip` funnel and are mapped to the proposed accepted finalizer/publication. No return after enabled attempt construction is left without a finalizer, and no admitted branch reconstructs lineage from expected data, world/name/pair text, commitment content, terminal strings, failure strings, or hidden state.

Conclusion: no remaining production-owned conditional lacking an exact current or finite proposed producer-transfer-finalizer-publication carrier was identified in the reviewed replacement at the pinned HEAD.

## Recheck of remaining prior requirements

Assumption: closing the two named blockers is insufficient if another prior semantic or authority requirement regressed. Basis: the corrected replacement retains the prior finite inventory and explicit state semantics. Conclusion: it also passes the following documentation checks:

- all 85 occurrences retain independently materialized enablement with exact `11 false / 74 true` accounting;
- C agreement is limited to 69 physically present controls, with 16 worker absences preserved;
- every A family has an exact current branch or finite proposed producer;
- every B and C family, World-24 aggregate, and World-25 equality retains production seam `NONE`;
- selected, handoff, and predecessor metadata remain opaque, ordered, defensively copied, and non-policy-bearing;
- request params and result/error/frame observations preserve unavailable, nil, available-empty, malformed-raw, present, and order distinctions;
- refusal-only observations retain unavailable admitted/terminal state and only branch-proven known-zero effects;
- admitted observations use the exact current terminal mappings and occurrence-local snapshots;
- owner construction deep-copies, pointer identity carries no metadata equality semantics, terminal closes callback admission before drain, panic is contained, and one release wins;
- no production World-24 aggregate or B/C production carrier is introduced;
- expected values, C controls, occurrence/world/name/pair text, commitment content, failure-string parsing, and hidden state remain forbidden execution or observation feeds; and
- no semantic validity, direction, relation, completeness, authenticity, feature identity, publication authority, or production use is inferred.

## Authority disposition

Assumption: a documentation-lineage GO accepts only the exhaustiveness of this finite proposed carrier description. Basis: every proposed production symbol remains explicitly unimplemented/unaccepted, no tests were run, and this review changed only this document. Conclusion: the corrected replacement is exhaustive and carrier-complete for documentation review, but it authorizes no corpus, RED, source, test, prototype, generated artifact, write set, writer, implementation, activation, carrier acceptance, semantic authority, public surface, release, deployment, or production mutation.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V2_GO
```

This GO is documentation-only. All mutation and production work remains blocked pending separate authority.

## Model interpretation

This review interprets “carrier-complete” as a complete finite documentation chain from current/proposed producer through transfer and finalizer to private publication for every retained production-owned conditional, not as proof that any proposed carrier exists or is accepted.

Key assumptions:

- The pinned source defines the current branch and wrapper topology.
- The corrected replacement defines proposed names and positions only.
- RED-only and fixture-only values require no production carrier.

Gaps and verification items:

1. No implementation or causal runtime evidence was requested or produced.
2. Any later source authorization must separately verify every proposed signature and call site.
3. Any later corpus/RED authorization must separately prove the occurrence and assertion accounting.

Improved framing: accept the replacement as a complete reviewable carrier specification while preserving a hard boundary against implementation or production authority.
