# ADR0011 P1 — integration-first private composition plan

## Status and basis

This is the current sequencing successor for private P1 composition, not a rewrite of the [original C plan](adr0011-remaining-c-consolidated-plan.proposed.md), its [V2 successor](adr0011-remaining-c-consolidated-plan.v2.proposed.md), or their immutable evidence. The [historical status companion](adr0011-remaining-c-plan-status.md) remains an earlier observation point.

**Basis: parent-reported decisions in the authorizing conversation, not fresh execution or an independent re-review by this document's author.** Unit 1 is bounded-accepted under the revised evidence contract at `885eb4d3`; missing original pre-enforcement chronology remains disclosed. Unit 2 is bounded-accepted at `9f30ccf7f363c3608761849ca4c126befc25d353`, merged by `ec5bef099d385c09b2a29552fb06084da028db47`. The cancellation/join fixture correction was reported independently accepted and committed at `665523df`. None of these establishes full P1 composition.

The subsequent composition investigation reported that the manager still instantiated the ordinary `lspwire.Reader`, while Unit 1 exposed original frames through `SuccessorCapturePolicy.ObserveOriginalFrame`. No real manager-owned handoff to Unit 2 was established at that observation point. Therefore isolated package passes cannot close this plan. The user explicitly approved a new private handoff, phase-sensitive refusal, the accepted Unit 1 resource profile, and integration-first sequencing. Current implementation progress must be recorded separately; this document does not infer that the missing edge is now present.

## Integration spine and ownership

**Required path:** manager-owned attempt → private `SuccessorIngressReader` ingress → exact original frame → prepared B4/source transaction → atomic restricted commit and lease consumption.

The manager supplies held session, generation, request key, method/exact params, transaction, completed-owner identity, and source custody. The reader supplies exact original bytes, bounded consumption/acquisition observations, and terminal/refusal outcomes. Neither component manufactures the other's evidence. A test-only adapter that invents those bindings is not composition.

Reuse the accepted typed A4→B4a→B4b contract verbatim. Keep manager-acquired immutable source captures and derived query occurrence identity; caller-consistent bytes/digests/acquisition labels are not source custody. Preserve uncertainty about whether the language server analyzed those captured source bytes. Exact original-wire bytes are not decoded/re-marshaled bytes.

Preserve Manager → restricted root → restricted manager lock order. Complete fallible preparation and reservation before transfer/commit; revalidate held identities before mutation. Once the no-fail installation/publication segment begins, it must not introduce allocation, refusal, I/O, or manager reentry. Lease consumption and committed storage become visible atomically, with one concurrent winner.

## Sequence: integrate before widening

1. **Confirm the real seam.** The executing context establishes exact-workspace READY managed access and bounded source-bearing relationship evidence. Preserve any transport/preflight/traversal failure; no replacement language server or textual CALLS substitution. Use the actual private manager path rather than a second simulated transaction owner.
2. **Run one minimal successful composed transaction early.** Demonstrate that the frame captured by Unit 1 is the exact frame bound into the manager's Unit 2 transaction, including source and query identities. Record identities and counters at the handoff. This success is integration evidence, not acceptance before failure cases pass.
3. **Exercise the same path adversarially.** Add the refusal, lifecycle, concurrency, stale/forged identity, source-substitution, capacity, and publication cases below around that real path. Preserve already accepted unit tests as supporting regressions, not substitute composition evidence.
4. **Consolidate once.** Review exact implementation/source identities, lineage, terminal/capacity accounting, same-path failures, ordinary-reader compatibility, and remaining exclusions together. Routine fixes proceed under standing authorization; do not create an approval step for each test. Stop for changed evidence semantics, limits, unapproved disclosure, or materially broader architecture/public integration.

## Phase-sensitive refusal and release

**Basis:** a response arrives after its request may already have been written. No refusal rule can retroactively remove that WRITE or consumed ingress bytes.

- Setup refusal before reservation: no downstream reservation, source transfer, WRITE, or publication.
- Preparation refusal after reservation but before WRITE: no WRITE/publication; account for earlier allocation and unwind safely. Failed preparation preserves lease usability where the accepted transaction contract requires it.
- Post-WRITE ingress refusal: seal the attempt, return no usable result lease, and allow no subsequent Unit 2 reservation/transfer/write/publication initiated from the refused frame. Preserve prior request effects and truthful reader accounting.
- Release existing custody only after registration is closed, relevant obligations have joined, final references/borrowers are relinquished, and applicable retention conditions permit it. Cancellation or reader sealing alone does not establish these facts. Pending custody remains charged; no timeout-to-release or unconditional synchronous refund.

Tests must distinguish **no new downstream effects after refusal** from safe cleanup of earlier effects. Never assert a universal zero-resource history after actual acquisition or WRITE.

## Private resource profile

Reuse accepted constants and their exact units; do not derive substitutes from request settings or silently widen ordinary-reader limits:

| Quantity | Bound / interpretation |
|---|---|
| Partial header | 65,536 bytes |
| Complete original frame | 2,097,152 bytes |
| Attempt consumption | 8,388,608 bytes; only the accepted single crossing-sentinel behavior |
| Attempt acquisition | 8,392,705 bytes; not extra admissible payload |
| Individual underlying read | 4,096 bytes |
| Outstanding prefetch capacity | 4,096 bytes |
| Generation-history acquisition / outstanding capacity | Separately scoped 8,388,608-byte bounds |
| Manager-wide custody | 16 slots / 64 MiB, including applicable active, retired, quarantined custody |
| Exact-generation custody | 4 slots / 16 MiB; both generation and manager ceilings apply |

One declared attempt owns supporting operations and target accounting; no per-request reset. Keep generation history separate, preserve carried-prefetch acquisition provenance, and avoid double charge. All existing reservations consume headroom. These are the accepted private policies, not a whole-process RSS guarantee or full C qualification.

## Composite acceptance evidence

| Case | Required observable result on the real composed path |
|---|---|
| Success / exact lineage | Reader-captured original frame reaches the exact manager-bound transaction without reconstruction or caller-derived identity. |
| Pre/post-WRITE refusal | Phase-appropriate zero downstream effects, truthful earlier accounting, no publication or returned lease after ingress refusal. |
| Failed preparation | No restricted mutation or premature lease consumption; safe reservation cleanup and valid retry under the accepted contract. |
| Cancellation / join | Exact close/teardown accounting, joined relevant I/O, no publication, and safe charge release; preserve the corrected fixture and unchanged join assertion. |
| Concurrent consume | Exactly one winner; no partial duplicate publication. |
| Stale/forged identity | Wrong session/generation/request/transaction/completed-owner/source bindings cannot alter the committed identity or release another owner's storage. |
| Source substitution | Independently held acquisition correspondence is required, not merely a matching caller-supplied digest. |
| Capacity / publication | Refusal before forbidden transfer/allocation; infallible fixed-storage publication after the accepted commit point; no leaked or unaccounted ownership. |
| Incomplete input / compatibility | Bare-array/null refusal remains pre-mutation; ordinary reader and public paths retain their behavior. |

Retain exact attempts, failures, corrections, and cumulative execution accounting. Missing-fixture exclusions remain exclusions, never pass evidence; do not invent absent historical test names. Report integrated test coverage separately from unit/package tests. No sleeps, polling, weakened assertions, or altered fixtures merely to hide lifecycle failure.

## Completion, independence, and deferred work

Completion is independent acceptance of this actual private composed path and its critical failures, with exact source/evidence identities and explicitly bounded scope. A happy path, sum of unit passes, or fixture correction alone cannot establish it.

Broader C (message/time/result/source/object/event/work closure), applicable retention obligations, references/definitions occurrence admission, the common normalized occurrence model and historical CALLS adapter, and indexing/grouping qualification are remaining stages of the overall integration—not unrelated workstreams or automatic approval stops. Continue toward them under standing authorization, exercising real producer-consumer handoffs early and verifying each boundary before claiming completion. Missing admission, custody, or resource guarantees must remain explicit; early integration does not permit bypassing them. P1 success alone does not establish end-to-end acceptance. Ask for a new decision only for material scope/architecture expansion, changed evidence semantics or safety limits, or new disclosure. Production enablement remains separately authorized.

Preserve `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and no producer authentication throughout. These are evidence-claim limits, not an indication that the integration must remain unfinished.

ADR 0007's CALLS-only synthetic community slice runs independently. Its graph/host/index work does not depend on this P1 increment and must not be pulled into its write set.

This document records the already approved private integration direction and standing execution boundaries. It changes no ADR decision, accepts no new implementation, and grants no commit, merge, push, public API, or production enablement.
