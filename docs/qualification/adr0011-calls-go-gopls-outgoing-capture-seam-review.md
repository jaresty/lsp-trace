# Independent review — ADR 0011 Go/gopls outgoing capture seam design

- **Reviewed profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
- **Reviewed commit:** `bc3ec1efdcf4b4b44173ca5810625be75886a092`
- **Accepted parent:** `9b98e2c7d71114be48e9a7485546fbcaf5f61e36`
- **Reviewed artifact:** `docs/qualification/adr0011-calls-go-gopls-outgoing-capture-seam-design.md`
- **Decision:** `ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO`
- **Implementation/profile/acquisition GO:** not issued
- **Authority:** `0`
- **Accepted:** `false`
- **Completeness:** `UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

## Independent boundary and ancestry check

The reviewed commit has the accepted parent as its sole parent and adds exactly the one design document named above. It does not modify source, tests, the outgoing plan, causal-test inventory, evidence manifest, vector inventory, prior reviews, or the accepted acquisition stop packet. This review evaluates design coherence only. It does not establish runtime behavior, provider authority, evidence authenticity or completeness, implementation correctness, profile qualification, acquisition permission, production admission, or profile GO.

The parent artifacts remain controlling context: the accepted outgoing plan keeps all evidence roles `MISSING` or `PLANNED`, authority at zero, completeness `UNKNOWN`, and production `REFUSED_MISSING_EVIDENCE`; the accepted stop packet records that no managed process, initialize request, prepare request, or outgoing request was issued because exact paired manager-owned transaction custody was unavailable at the acquisition boundary.

## Original-wire chain, framing, and identity

The design correctly locates wire truth at the manager-owned transaction rather than at decoded Go values or coordinator re-encoding. The pinned source supports that distinction: `sessionruntime.RoundTripResult` retains manager-local write/read observations and frame state; `internal/acquisition/sessionclient.New` currently translates the managed result to only `result.Result` or an error; `internal/acquisition.WireClient` then handles raw result JSON and typed decoding; and the coordinator's `boundedJSON` records are explicitly normalized encodings rather than raw transport identity.

The proposed generic recorder contract retains immutable exact request and response frames, exact raw params/result slices, content and frame lengths, completed write/read observations, session, generation, request key/JSON-RPC ID, method, manager owner sequence, request/response ordinals, caller-supplied probe/expansion ordinal, terminal metadata, independent accounting, and SHA-256 integrity links. Re-encoding cannot satisfy the contract, and digests cannot replace retained bytes. Those requirements are sufficient to preserve original-wire framing and transaction identity across the prepare and outgoing transactions.

## Ordered prepare and unchanged selected handoff

The outgoing wrapper is correctly bound to the exclusive chain:

```text
textDocument/prepareCallHierarchy
-> selected prepared caller
-> callHierarchy/outgoingCalls
-> caller-to-callee outgoing occurrence ledger
```

It retains the complete ordered prepare result, every source ordinal, the selected prepared-caller ordinal, and the selected caller's full canonical identity: name, kind, canonical URI, range, selection range, optional detail, optional tags, and opaque data, including field presence and exact raw item bytes. The selected item must be byte-equal to the outgoing request's `item` member and both hashes must match; hash equality alone is explicitly insufficient. This preserves an unchanged, mechanically checkable handoff rather than reconstructing or normalizing the selected item before custody is established.

## Complete outgoing result and occurrence linkage

The design retains the complete outgoing raw result token and response frame, each unchanged outgoing member with its source ordinal, each unchanged `fromRanges` value with its range ordinal, and source byte spans. Every admitted occurrence links the selected item, outgoing request item, outgoing member, and exact caller-side range through byte equality where applicable, SHA-256, spans, and ordinals.

Multiplicity is preserved: one occurrence is emitted per retained range; equal duplicate ranges, repeated equal members, parallel edges, and self-loops remain distinct. A member with an empty `fromRanges` array remains a zero-site member and emits no occurrence, while missing or null member-level `fromRanges` is malformed. No sorting, deduplication, prefix salvage, or byte replacement is permitted.

A terminally successful complete top-level raw result token of JSON `null` is exclusively `SUCCESSFUL_EMPTY`, retains its exact token/bytes and successful accounting, and produces zero members, ranges, occurrences, rejected rows, and production-visible effects. A non-null empty array is instead `COMPLETE` with zero occurrences. Thus canonical emptiness may coincide while source custody and terminal identity remain distinct.

## Exclusive terminals, accounting, and refusal

The terminal set is exhaustive for the designed seam: `COMPLETE`, `SUCCESSFUL_EMPTY`, `TRANSPORT_ERROR`, `PROTOCOL_ERROR`, `MISSING_PAYLOAD`, `MALFORMED`, `PARTIAL_CAPTURE`, `LIMIT_REFUSED`, and `CANCELLED`. The design supplies evidence-based precedence and a conservative fallback to `PARTIAL_CAPTURE` when the retained facts cannot support exactly one class. Every non-success terminal yields zero qualified occurrences and zero semantic or production effects; malformed and partial inputs receive no prefix success.

Recorder and outgoing-wrapper limits are independently declared and reported for frames, tokens, aggregate retained bytes, prepare items, outgoing members, ranges, occurrences, spans, ordinals, acquisition time, callback time, work, hashes, diagnostics, allocation, and storage. Every dimension requires exact-limit success and plus-one refusal, cannot borrow from another dimension, and uses checked arithmetic. Declaration, use, remainder, terminal disposition, and refusal reason are retained. Recorder costs are accounted separately from acquisition and qualification accounting.

The refusal point is correctly before normalization and effects. Byte/framing checks happen before parsing where possible; bounded scanning is recorder work and cannot emit normalized items or occurrences; provisional semantic values are cleared on refusal. Recorder exhaustion before required custody begins is `LIMIT_REFUSED`; incomplete custody after it begins is `PARTIAL_CAPTURE`. Neither changes the underlying production round-trip result.

## Test-only reachability and semantic transparency

The design requires test-only ownership or test-compilation reachability, no CLI/MCP/schema/environment/config/profile/manifest/public-constructor/default enablement, unchanged ordinary production signatures, no disabled-path recorder allocation or work, and no production publication, replay, admission, or qualification acceptance of test-only records. The narrowly allowed fallback of one unexported default-nil `sessionruntime` hook is conditional on `_test.go` composition being impossible, must have only test callers, and must pass an independent production-unreachability test; use of that fallback remains implementation-review work and is not authorized here.

Enabled/disabled transparency covers the production `RoundTripResult`, errors, lifecycle, ordering, cancellation, acquisition values, normalized graph, diagnostics, and production accounting. Recorder panic, timeout, allocation/hash failure, invalid configuration, or recorder-limit exhaustion is bounded and test-only; it cannot replace, retry, cancel, mutate, suppress, or unboundedly delay the production transaction, and cannot produce qualification success. The existing package-private defensive-copy and panic-containment patterns demonstrate a plausible local extension seam but do not themselves prove the future implementation.

## Minimality and authority separation

Among the evaluated placements, the split seam is minimal and authority-preserving:

1. extend manager-owned private recorder/pair mechanics only for generic exact transaction custody;
2. assemble prepare selection, unchanged handoff, outgoing members/ranges, and occurrence meaning in a separate outgoing-only test wrapper immediately outside managed round trips; and
3. reject capture inside production outgoing normalization because it is too late for original-wire identity and risks effects before refusal.

This avoids a general production response-frame accessor and avoids broadening `acquisition.Client`. The proposed write-set is narrow and explicitly invalidates itself if implementation requires CLI/MCP schemas, manifests, evidence copies, production acquisition interfaces, public APIs, profile registries, selector behavior, graph normalization, publication/replay/admission, semantic-vector, incoming qualification, or production enablement.

The generic recorder mechanism is reusable only as transaction-custody mechanics. It carries no direction, profile, caller/callee interpretation, occurrence semantics, evidence role, qualification authority, review result, or GO token. Outgoing independently owns and must independently review the exact profile binding, prepare/outgoing method chain, selected caller and unchanged handoff, caller-to-callee orientation, caller-side `fromRanges`, outgoing evidence/provenance/limits/accounting/refusal, and every outgoing review or future GO. Equal generic bytes and any incoming evidence, receipt, manifest, review, or GO cannot satisfy those outgoing obligations.

## Required implementation falsifiers remain prospective

The sixteen RED rows cover original-frame custody, ordered multi-item prepare selection, changed handoff refusal, multiplicity, `null` versus `[]`, missing payload, transport/protocol/cancellation separation, malformed versus partial capture, each independent exact-limit/plus-one boundary, identity/ordinal/overflow refusal, enabled/disabled transparency, recorder-failure transparency, production unreachability, incoming-authority non-transfer, refusal of post-normalization capture, and refusal of digest-only custody. They are prospective implementation obligations only. This review does not claim that any row has run or passed.

## Decision

`ADR0011_CALLS_OUTGOING_CAPTURE_SEAM_DESIGN_GO`

This token approves only the coherence and minimality of the reviewed design for a separately authorized implementation proposal. Provider authority remains unresolved. Production remains `REFUSED_MISSING_EVIDENCE`. No implementation GO, acquisition GO, evidence GO, profile GO, semantic-vector GO, production activation, evidence-role transition, or authority increase is issued. Stop after this design decision.
