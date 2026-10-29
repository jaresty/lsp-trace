# Private retained-source interpretation preparation

`Prepare(context.Context, []Input, Limits) (Result, error)` is in-memory mechanical preparation only. `Input` requires an existing constructor-sealed `retainedpartialprogramc.Projection` and its sealed `InstabilityPrivate`. The builder uses `ValidatedInstabilityBytesPrivate`, which bounds and compares the mutable assessment view to its retained seal and validates actual retained partitions/metrics with `ValidateInstabilityPrivate`; it never reruns Leiden. Callers must not mutate inputs concurrently with validation/preparation. `Projection.PreparationEvidencePrivate` checks the projection seal, parses the derived envelope with `ParseDerivedRetainedPartialNativePrivate`, and parses its embedded record 5 with `ParseSourceWithholdingPrivateV1`. It returns detached observations, not new authority.

No source paths, root handles, source checkout lookup, arbitrary callbacks, Manager, provider, worker, persistence, replay or LLM invocation are accepted. This package adds no public operation. Ordinary Describe, workflow and handoff gates are unchanged. The existing caller-provided-inference amendment remains applicable: source disclosure needs authorization, interpretation is provisional and feature identity requires separate user acceptance.

## Adapter versus existing preparation: actual comparison

The existing qualified preparation chain is already implemented, not missing:

* `targetpacket/packet.go: Build` selects nominations, admits snapshots, checks exact constituent graph custody, calls `retainedprojection.Select`, `Resolve`, and `AssembleV2Bounded`, then reconciles consumer alternatives.
* `retainedprojection/select.go: Admit` validates supported retained snapshots, not the private source-withholding carrier. `Select` checks display provenance kind/method/policy (including `FULL_DEFINITION`) against retained bindings. It does not discover Go declarations with an AST.
* `retainedprojection/resolve.go: Resolve` is pure with respect to its lookup, but its input is an already selected retained-display `Plan`. Its whole-object lookup adds no missing display or caller-occurrence provenance. Building such a plan from this private carrier would bypass rather than adapt those checks.
* `retainedprojection/assemble.go: retainedInputs/validateResolvedSelection` recheck retained display provenance and UTF-16 constraints. `AssembleV2Bounded` has a generic custody type parameter, but that does **not** make private AST provenance an admissible retained selection. Neither function is used to mint misleading `FULL_DEFINITION`, `ProviderMethodResult` or V3 caller receipts.
* The pure common dependencies **are reused**: `sourceposition.Offset/PositionAtOffset` replace the initially duplicated position converter; `sourceprojection.Project` assembles the exact AST declaration byte range under a private policy and explicit `RETAINED_SOURCE_AST_DECLARATION` provenance. Its neutral result is not passed to ordinary Describe admission.
* `targetpacket.reconcileConsumerAlternatives` requires caller/relation units, exact occurrence/range, citation, emitted span and graph custody. These are stricter than merely observing a private direct CALLS edge. This preparation therefore preserves input-owned incoming/outgoing observations and gaps, but does not assign qualified consumer alternatives.
* `describerequest.Build/BuildV2` validates `targetpacket.Packet`, reconciles endpoint/emitted-span identity and fixes C2-calls-C1 consumer identity. Its request/message/correlation/lineage identities include ordinary admission/corpus/custody and consumer-alternative state. Reusing those hashes or envelopes for these private inputs would assert a different contract. No duplicate generic request-envelope, deadline, worker transport, consumer selector or inference machinery has been added here.

The irreducible delta is: adapt the sealed private producer/assessment pair; bind retained Go bytes to exact original endpoint selection/name/kind/receiver; account finite nominated subsets and all instability runs/community members; split typed private evidence rows without inventing native display custody. The preparation is explicitly `PRIVATE_TARGET_INTERPRETATION_PREPARATION_V1`, not completed qualified Describe/workflow/handoff.

The private AST association accepts callable provider kinds 12 or 6 for a receiver-bearing Go `FuncDecl`, including a bare identifier name. The retained endpoints `StringPartition.MarshalJSON` and `IndexPartition.MarshalJSON` were reported as bare `MarshalJSON`, kind 12: that provider category does not establish whether the source has a receiver. Exact unique source identifier coordinates and the matching name establish this AST association; qualified names must additionally match the concrete AST receiver spelling. A receiver-free declaration still requires kind 12. The original provider name/kind remain unchanged in TARGET records; the receiver is independently visible in the `RETAINED_SOURCE_AST_DECLARATION` body, not asserted as provider metadata. No new field or format version is needed. NativeRange and SelectionRange remain exact even when identifier-only; the separate DeclarationRange is the whole AST-derived declaration, not provider FULL_DEFINITION. Ordinary native kind/display validation remains unchanged.

## Positive example (owned synthetic producer, not production evidence)

An owned fixture installs valid root source **before** freezing its policy and hashes. It then executes the existing fixture producer, qualifies eight retained records, closes temporary roots, derives the private native input, projects it and computes instability. The builder receives only sealed values.

```text
RequestType: PRIVATE_TARGET_INTERPRETATION_PREPARATION_V1
Target: root
Handle: <exact-derived-input-sha256>/seed/root/declaration
Kind: RETAINED_SOURCE_AST_DECLARATION
Body: func root() {}
Bytes: exact UTF-8 bytes of Body
StartByte/EndByte: exact retained byte offsets, end exclusive
SelectionRange: original server-reported endpoint selection
DeclarationRange: AST-derived range, separately labeled
Prompt: Interpret only the supplied retained-source technical evidence ...
Authority: 0
Accepted: false
FeatureIdentity: UNRESOLVED
SourceCompleteness: UNKNOWN
```

The root packet retains six containing-community rows, six membership rows and all fifteen pair metrics. The actual sparse 27-node fixture's assessment is `RESOURCE_INCOMPLETE`, not `UNSTABLE`; preparation never changes that disposition. Every containing community retains a member count and a row for **each** member including raw source availability. Missing callsite excerpts are explicit gaps. No membership or unreadable member implies a feature or hidden technical meaning.

The exact original discovery record remains available through the sealed input capsule; coverage references its digest and separately reports 219 original enumeration symbols and 25 nominated targets. Explicit requested subsets of 1/4/25 produce one prepared root and 0/3/24 source gaps. The other original mock endpoint labels do not correspond to valid Go declarations; these are honest source-binding gaps, not semantic successes. Independent input digests namespace identical labels and source bytes; graphs and CALLS are never unioned.

## Finite budgets and accounting

Limits are explicit per invocation, not defaults or public promises. The exercised proposal is 64 targets, 64 KiB per part, 8 MiB canonical result, 256 parts, 2 MiB retained source per target. Private hard ceilings enforce those bounds. Upstream source policy still fixes six files/25 nominations; this builder does not assume 25 requested targets or six instability runs (declared upstream runs are 2..6).

`requested = prepared + source_gap + resource_gap` for each independent input. Invalid/unsealed/mixed-input assessments reject the invocation before AST parsing. MaxTargets creates indexed untouched resource gaps. Oversized declarations/records are indivisible resource gaps, not truncated full snippets. Reference/member rows split deterministically into typed parts. Ordinal predecessor/next links avoid recursive hash identities; packet identity binds exact input, selected scope, assessment digest and issued limits. Part hashes bind final payload bytes. The full canonical result, including its index, must fit MaxTotalBytes; complete packets are withdrawn to resource gaps if required. A budget too small even for the complete requested index rejects the invocation with no packets issued.

The post-review-fix observed normal fixture measurement (two preparations per subset) reported 34,327 payload bytes, one part and roughly 0.66–0.67 seconds for each 1/4/25 case. These are finite synthetic observations, not performance guarantees or global defaults. The 8 KiB part test exercises multi-part positive output and preserves all six memberships. Race checks need a larger wall-clock allowance because existing closed-envelope qualification is repeated; a 120-second combined command timed out, and the two focused producer tests subsequently passed with a 240-second Go timeout.

## Review corrections: seed ownership and pre-encoding bounds

A shared input/seed-membership relation ID is **not** observation ownership. `ownedCallRecords` now reconciles the selected target's incoming/outgoing expansion request IDs with the retained request's method, queried endpoint, actual attempted successful response and `EdgeObservation`. A foreign target's request is admitted only when this target explicitly references it through a matching cached expansion. Cache references are labeled, retaining the original observed-for-target identity; they do not create another response or independent support.

Only those observations supply owned ranges. Original native math occurrences are filtered by the owned range, relation, caller/callee node indexes and original provider-bearing occurrence record. Each range/occurrence carries exact request-reference/direction/cache lineage; caller URI/position encoding are preserved. Multiple owned observations for the same site retain all references without multiplying the native occurrence. Excluded input-global observations produce counts/gaps, never silently borrowed rows. Community membership remains a separate structural ledger.

The same-input regression fixture uses outgoing request A and incoming request B for the **same** native relation with different response ranges. A gets only A's site/occurrence/request; B gets only B's. An explicitly cached B expansion referencing A retains A's original evidence with `Cached=true`; removing the cache flag refuses that foreign request. Another case retains two owned request references to the same site without duplicating its native occurrence. This is a focused pure reconciliation fixture, in addition to the owned producer integration fixture—not an assertion that arbitrary synthetic DTOs have producer custody. A second test runs actual `acquisition.Acquire` with an owned typed fake client (no server/process/IO), validates its result, and projects its original native occurrences: root outgoing and B incoming observe one merged relation at different ranges, while C explicitly caches root's request. Each prepared CALLS section retains only its actual range/occurrence and request/cache lineage. This tests coordinator-produced overlap and cache records, not only manually assembled DTOs.

Before maps, copies or assessment encoding, Prepare bounds input/explicit selection counts to the issued MaxTargets and hard ceiling 64, labels to 1,024 bytes each, aggregate label bytes to the issued total budget, and validates bounded label UTF-8. Nil selection still means all original finite nominations, with untouched MaxTargets resource gaps.

`preflightInstabilityPrivate` runs before encoding in both `ValidateInstabilityPrivate` and the new bounded accessor. It requires sealed completed state; 2..6 seeds/runs; the declared pair count (at most 15); at most 10,000 projection nodes/communities; and exactly the projection node count in each run, checked before member traversal or membership-map allocation. Top-level strings are capped at 4,096 bytes; run/pair/member strings at 1,024, with UTF-8 checks only after length bounds. A subtraction-based conservative JSON allowance (six bytes per string byte plus bounded key/scalar/container overhead) caps estimated encoding at the existing 16 MiB artifact ceiling before Marshal. This can reject a large conservative upper bound even if its eventual encoding would be smaller. Default accepted canonical encoding is unchanged. Mutable field/seal disagreements remain rejected; using original retained bytes does not bypass comparison.

Preallocation regression tests construct malformed 1 MiB strings and oversized arrays outside the measurement window, then assert rejection before a 128 KiB allocation allowance using controlled GC/TotalAlloc deltas. They also directly assert that the bounded serializer emits no bytes on rejected shapes. This is a bounded regression guard, not a general process-heap guarantee.

## Pending boundaries

Independent parent review is required before adoption/commit or any real retained preparation operation. There has been no real retained operation, generated explanation, ordinary Describe admission, replay claim, commit or push. The new builder's positive integration fixture covers six declared runs with `RESOURCE_INCOMPLETE`; it does not establish a production 100-node/all-six-UNSTABLE preparation or semantic completeness. Existing Describe qualification is not invalidated by this private input-profile gap.
