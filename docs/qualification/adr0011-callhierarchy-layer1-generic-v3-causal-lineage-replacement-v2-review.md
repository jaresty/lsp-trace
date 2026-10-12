# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Lineage Replacement V2 — Independent Review

Status: **BLOCKED**

- Reviewed repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `1a551a0f3c183a8f8e81b30c27256c15138fc756`
- Reviewed replacement: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`
- Review mode: independent documentation-only semantic and mechanical review
- Mutation by this review: this document only
- Tests run: none
- Corpus, RED, write-set, source, prototype, generated artifact, implementation, writer, commit, activation, semantic-validity, public-authority, and production authority: **not issued**

## Review boundary

The replacement was checked against the committed blocked inventory/review and amendments and against the exact current `sessionruntime` source at the pinned HEAD. Proposed symbols were assessed only as finite lineage proposals. Their mention does not mean that they exist, are accepted, are production-authorized, or may be implemented.

A GO requires every retained outcome and every production-owned conditional to have an exact lawful current or finite proposed producer, transfer, and finalization/publication carrier. Mechanical accounting alone is insufficient.

## Mechanical result

The replacement preserves the exact occurrence census:

```text
25 roots
16 World-24 workers
22 World-25 executions
22 World-25 pair members
85 total occurrences
```

Its occurrence-sensitive ownership arithmetic is exact:

```text
ordinary occurrence: A=116, B=20, C=9, total=145
World-24 root:       A=81,  B=55, C=9, total=145

A = 84*116 + 81 = 9,825
B = 84*20  + 55 = 1,735
C = 85*9        =   765
union                 12,325 = 85*145
```

Only the 35 members of the seven World-24 root ledger dimensions migrate from A to B. The same path names remain A for all 16 World-24 workers and all other occurrences.

The exact 17 removed/deferred paths remain absent: four request-body paths, four request-payload paths, four response-body paths, four response-result paths, and `result.form`. The replacement does not restore them by alias or hidden carrier.

This is a **mechanical PASS**.

## Occurrence-sensitive enablement and C result

The replacement correctly separates the proposed causal driver bit from C metadata:

```text
false = 11 w25-pair/*/disabled occurrences
true  = 74 other occurrences
total = 85
```

The bit is expressly forbidden from being derived at runtime from `expected`, any `expected.control` value, commitment availability/content, attachment zero value, occurrence/world/name/pair text, transport, result, or hidden state.

The replacement also corrects the prior universal C contradiction:

- the three `control.*` agreement checks apply only to the 69 occurrences where `expected.control` is physically present;
- the 16 `w24-worker/01..16` occurrences validate structural absence only;
- workers do not inherit, borrow, manufacture, or false-default the World-24 root controls.

Current HEAD contains neither the proposed `enabled bool` field nor `privateCallHierarchyLayer1WithEnabled`; the committed RED currently carries only the existing attachment and predecessor setter. The replacement labels the enable field, setter, read, propagation, and publication branch as proposed and not accepted. That labeling is accurate.

This portion is a **documentation-lineage PASS**, not implementation or production acceptance.

## A/B/C lineage result

### B and C

Every B row, every C row, and the 35 World-24 root aggregate paths names production seam `NONE`. The formulas and classes remain RED-only or fixture-only:

- lifecycle and derived lifecycle values come from actual acquisition, callback, containment, and release returns;
- derived external values copy actual result/error/message/counter observations;
- identity equality is actual-to-actual and availability-sensitive;
- C is structural organization/agreement only;
- World-24 aggregation uses the 16 worker actual ledgers, `AND` availability, fixed order, and no unavailable-to-zero substitution;
- World-25 equality is actual-to-actual after two independent executions.

No B or C value is represented as a production source. This is exact.

### A and current source bindings

The replacement materially improves the prior broad wording by naming finite proposed builder, frame-capture, result-capture, finalizer, owner-construction, and publication symbols and by binding them to exact current source regions:

- `roundTripWithPrivateTransaction` performs the current pre-admission checks;
- `key := r.pending.Begin(req.Generation)` is followed by request registration, identity capture, protocol ownership, worker allocation, and unlock;
- successful write custody is available after `owner.run`, `result.requestWrite`, and `writeCapture.retainedFrame()`;
- read custody is available from `readResult.raw/frame` before parsing-derived normalization;
- matched response branches assign `result.ServerError` and `result.Result`, including the private clearing branches;
- `finishRoundTrip` sets failure, completes diagnostics, maps terminal/read/write state, records numeric RPC error, updates worker accounting, unlocks, and returns.

The proposed carrier set is finite, private, generic, and expressly non-semantic. It does not propose commitment-validity policy, direction/relation inference, feature identity, completeness/authenticity, public API authority, or production authorization.

However, one required production-owned conditional family still lacks a lawful proposed carrier.

## Blocking finding — pre-admission refusal has no constructible or publishable carrier

The replacement places:

```text
newCallHierarchyLayer1GenericObservationBuilder(...)
```

inside `roundTripWithPrivateTransaction` only after `key := r.pending.Begin(req.Generation)`, request registration, identity capture, protocol ownership, and worker allocation, immediately before unlock.

It separately proposes:

```text
(*callHierarchyLayer1GenericObservationBuilder).refuseBeforeAdmission(reason session.Failure)
```

for every pre-admission return in §4.1.

Those two statements are incompatible. At current HEAD, all of the listed pre-admission branches return before key allocation and therefore before the proposed builder construction point. This includes:

- C18 deadline receipt failure;
- parent context failure;
- missing session;
- generation mismatch;
- non-ready/protocol-owned lifecycle conflict;
- private-P2 effective-deadline refusal;
- owned-document mismatch;
- non-`wireChild` process;
- request/worker exhaustion;
- readiness-history freeze, reader, or borrower failure; and
- `admitPrivateB4SourcesC17Locked` failure.

Consequently, the replacement supplies no receiver on which `refuseBeforeAdmission` can be called. It also supplies no separately named pre-key refusal producer, no refusal value returned from the helper, no immutable refusal observation constructor, and no transfer/publication seam for such a value. The final publication helper is explicitly limited to `finishRoundTrip`, but these branches return directly and never enter `finishRoundTrip`.

The prose statement that `refuseBeforeAdmission` “records only a proposed local refusal normalization” is not an exact carrier. A local normalization with no named construction, return, transfer, or publication path cannot satisfy the A ownership row for `refusal.{reason,refused}` or the conditional matrix's claim that every pre-admission refusal is normalized.

Moving the ordinary builder before admission would also contradict its current proposed signature and duties because it requires an allocated `lspwire.RequestKey` and the post-allocation before snapshot. A lawful correction must therefore name a distinct finite pre-admission refusal carrier or explicitly exclude those direct-return branches from the retained A observation contract and reconcile ownership/cardinality accordingly. This review does not choose or authorize either correction.

## Explicit production-owned conditionals lacking a carrier

Exactly the following documented production-owned outcomes lack an exact current-or-proposed producer/transfer/finalization chain in the replacement:

1. `refusal.refused=true` for each §4.1 pre-admission direct-return branch;
2. `refusal.reason=<exact current failure>` for each such branch; and
3. any immutable/publicly discoverable private generic observation claimed to carry those two A values for those branches.

The current `RoundTripResult.Failure` return is an exact current failure carrier, but the replacement does not name a lawful transfer from that direct return into its proposed immutable generic owner/observation. Disabled owner publication is correctly distinct from refusal and cannot fill this gap.

No other missing conditional carrier was identified in this documentation-only review. That statement is bounded to the reviewed replacement and pinned HEAD and does not confer implementation or production completeness.

## Nil/empty, known-zero, order, cardinality, lifecycle, and concurrency review

Subject to the blocker above, the replacement explicitly preserves:

- unavailable versus available-empty result/error/external/frame states;
- no-frame response custody without fabricated bytes;
- malformed emitted frame custody as raw available bytes;
- `known_zero = available && value==0` and ledger `known_zero = available && delta==0`;
- ordered predecessor cardinality, including an empty list rather than one unavailable element;
- fixed World-24 worker order and unavailable propagation;
- independent World-25 executions and nil/empty-preserving equality;
- terminal closure before new callback admission, admitted callback drain, panic containment, and one release winner;
- defensive copying and no pointer-semantic commitment equality; and
- no production World-24 aggregate storage.

The anti-feed section also expressly forbids expected/world/name/pair/commitment-content/hidden-state execution operands and confines registry/occurrence identity to offline authoring.

## Authority disposition

The replacement is exhaustive mechanically and closes the earlier enablement/C-domain and broad-A-wording defects, but it does not give every outcome a lawful exact current-or-proposed carrier. The pre-admission refusal family is an explicit unresolved carrier gap. Therefore the required GO condition is not met.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V2_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_CAUSAL_LINEAGE_REPLACEMENT_V2_BLOCKED
```

This BLOCKED disposition is documentation-lineage-only. Corpus/RED mutation remains paused pending caller authorization. Source, test, prototype, write-set, writer, activation, semantic-validity, public-authority, and production authority remain blocked.
