# ADR 0011 Call Hierarchy reuse assessment for the private D/R transaction recorder

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Assessed HEAD:** `8210109f3f71fd969fe3201935014d4226c90c13`
- **Status:** authoritative bounded reuse assessment; implementation not authorized
- **Scope:** private, default-off, test-only transaction custody for `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, and `callHierarchy/outgoingCalls`
- **Authority ceiling:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`
- **Implementation gate proposed for later review:** `ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_GO`

## Decision

The smallest shared core is a narrow extension of the existing private `sessionruntime` owner/lease/transaction machinery. A new recorder package is not justified.

The reusable unit is transaction custody only: immutable original request/response frames when actually observed, raw params/result subspans derived from those frames without re-encoding, exact workspace/session/generation/key/method linkage derived only from the exact managed session/generation custody that owns the transaction, completed write/read observations, bounded owner/lease lifetime, terminal release, independent recorder accounting, and production unreachability. Session identity alone never establishes workspace identity. Incoming and outgoing remain separate directional adapters outside that core. They share no CALLS semantics, profile authority, occurrence construction, qualification decision, or GO token.

This assessment is bounded by the files and symbols named below. Where the inspected structural output was truncated, the affected conclusion is explicitly `UNKNOWN`; absence is not inferred.

## Original transport frames versus decoded or remarshaled records

These representations are not interchangeable:

1. **Original transport frame:** the exact `Content-Length` header and JSON-RPC body bytes written to or accepted from the managed server. Only manager-owned frame capture and leases can preserve this identity. Relevant owners are `B4RequestFrameLease.WithBytes` and `B4ResponseFrameLease.WithBytes`.
2. **Raw token or subspan:** params, result, selected item, response member, or range bytes sliced or mechanically copied from a retained original frame, with byte span and ordinal. This remains original-byte evidence only while linked to that frame and never re-encoded.
3. **Decoded value:** `json.RawMessage`, `lspwire.Message`, `CallHierarchyItem`, ranges, or another Go value obtained by parsing. It may establish bounded structural agreement, not original serialization.
4. **Remarshaled or normalized record:** bytes produced from decoded values, including `json.Marshal` request reconstruction and acquisition `boundedJSON`. These are derived records. They cannot prove original whitespace, member order, duplicate-key history, framing, or exact transport bytes.

`sessionruntime.VerifyMethodFrameCorrespondence` reconstructs the request through `json.Marshal` and verifies response-frame agreement. It is reusable as a correspondence check, but its reconstructed request is not an original outbound request frame. Likewise, `OwnedMethodPair.Params` and `.Result` are useful decoded/raw payload copies but are not, by themselves, proof of complete original framing. Any future adapter that accepts remarshaled bytes as original custody violates this assessment.

## Requirement-by-requirement disposition

`REUSE_EXACT` means the present contract can be consumed without changing its meaning. `REUSE_WITH_EXTENSION` means the same private owner must be narrowly generalized or supplied an additional private method-neutral field while preserving current callers. `MISSING` means no inspected current symbol establishes the requirement. `INAPPLICABLE` means the requirement belongs exclusively to a directional adapter or later authority stage and must not enter the shared core.

| Requirement | Disposition | Exact current files and symbols | Bounded assessment |
| --- | --- | --- | --- |
| Exact-generation exclusive transport ownership and conflict behavior | `REUSE_EXACT` | `sessionruntime/sessionruntime.go`: `Manager.RoundTrip`, `Manager.roundTripWithPrivateTransaction`, `Manager.finishRoundTrip`; `sessionruntime/owned_transport.go`: `ownedTransport.run`, `ownedTransport.retire` | Reuse the existing manager transaction and lifecycle; do not add a parallel transport owner. |
| Session, generation, request key, method, write/read linkage | `REUSE_EXACT` | `sessionruntime/owned_method_pair.go`: `OwnedMethodPair`, `validOwnedMethodPair`, `ownedMethodPairValue`; `sessionruntime/sessionruntime.go`: `RoundTripResult` | Existing identity joins are part of the shared provenance base, but do not establish workspace identity. |
| Workspace identity and exact workspace/session/generation custody | `REUSE_WITH_EXTENSION` | Exact managed `sessionruntime.Manager` session/generation custody and its workspace routing/binding; no workspace field exists in `OwnedMethodPair`, `OwnedMethodPairView`, `RequestWriteObservation`, `ResponseReadObservation`, `RoundTripRequest`, or `RoundTripResult` | Add workspace identity to the retained shared provenance only through an exact manager-owned join to the managed session and generation that owns the transaction. Session identity alone, caller assertion, decoded payload, profile label, or adapter inference cannot supply workspace identity. Missing, stale, foreign, or ambiguous workspace custody refuses before semantic effects. |
| Defensive private ownership of params/result bytes | `REUSE_EXACT` | `sessionruntime/b4_owned_method_pair_lease.go`: `newPrivateB4OwnedMethodPairOwner`, `OwnedMethodPairLease.WithPair`; `sessionruntime/owned_method_pair.go`: `CompletedOwnedMethodPair` | Existing copies and lease-bounded views are reusable; they remain payload custody, not complete frame custody. |
| Exact original request frame | `REUSE_WITH_EXTENSION` | `sessionruntime/b4_request_frame_lease.go`: `privateB4RequestFrameOwner`, `B4RequestFrameLease.WithBytes`, `Release`; current manager construction in `sessionruntime/sessionruntime.go` | Reuse this owner/lease pattern and existing captured frame. Extend only the private eligibility/method plumbing needed for the three Call Hierarchy methods. No reconstructed request qualifies. |
| Exact original response frame | `REUSE_WITH_EXTENSION` | `sessionruntime/b4_response_frame_lease.go`: `privateB4ResponseFrameOwner`, `B4ResponseFrameLease.WithBytes`, `managerBytes`, `acquireCaller`, `releaseManager`, `Release` | Reuse the dual-held owner and terminal release. Extend private eligibility to Call Hierarchy without changing existing D/R behavior. |
| Raw params/result subspans linked to frame spans | `REUSE_WITH_EXTENSION` | `sessionruntime/owned_method_pair.go`: `OwnedMethodPair.Params`, `.Result`; `sessionruntime/method_frame_correspondence.go`: `VerifyMethodFrameCorrespondence` | Add bounded span metadata derived from retained frames. Do not treat copied `RawMessage` or remarshaled bytes as source spans. |
| Frame lengths and SHA-256 integrity links | `REUSE_EXACT` for observations; `REUSE_WITH_EXTENSION` for retained frame bytes | `sessionruntime/method_frame_correspondence.go`: `methodCorrespondenceHash`, `VerifyMethodFrameCorrespondence`; `RequestWriteObservation`/`ResponseReadObservation` as carried by `OwnedMethodPair` | Digests link retained bytes; they never replace bytes. Request correspondence remains derived unless backed by the exact request-frame lease. |
| Owner/lease terminal closing, active-borrow exclusion, byte zeroing, charge release | `REUSE_EXACT` | `sessionruntime/b4_owned_method_pair_lease.go`: `requestTerminal`, `waitReleased`, `Release`; `sessionruntime/b4_request_frame_lease.go`: `finalizeLocked`; `sessionruntime/b4_response_frame_lease.go`: `finalizeIfUnheldLocked` | Preserve the current lifecycle algebra and release ordering. |
| Cancellation, timeout, pipe interruption, worker join, poisoned owner retirement | `REUSE_EXACT` | `sessionruntime/owned_transport.go`: `ownedTransport.run`, `retire`, `contextFailure`; `sessionruntime/sessionruntime.go`: `finishRoundTrip` | Recorder state may observe these outcomes but must not alter them. |
| Independent recorder bytes/objects/work/diagnostic accounting | `REUSE_WITH_EXTENSION` | `sessionruntime/b4_owned_method_pair_lease.go`: `privateB4ByteAccountV2` leases used by `newPrivateB4OwnedMethodPairOwner`; request/response frame owner charges | Reuse existing reservation/release machinery, adding only closed Call Hierarchy recorder dimensions and checked arithmetic. Never borrow production or directional occurrence budgets. |
| Recorder enabled/disabled semantic transparency and panic containment | `REUSE_WITH_EXTENSION` | `sessionruntime/owned_method_pair.go`: `emitOwnedMethodPairTestHook`; existing lease callbacks; `Manager.RoundTrip` default path | Existing hook demonstrates panic isolation, but complete Call Hierarchy frame-capture transparency remains prospective and requires RED tests. |
| Default-off private/test-only production unreachability | `REUSE_WITH_EXTENSION` | `RoundTripRequest` private `json:"-"` fields and private owner fields in `sessionruntime/sessionruntime.go`; package-private owners in the files above | Preserve no CLI/MCP/config/environment/registry route. Add a static reachability test for any new method-neutral private switch. |
| Prepare-result complete order, selected ordinal, exact selected-item span, unchanged handoff | `INAPPLICABLE` to shared core; `MISSING` until directional adapters exist | No shared recorder symbol; specified by the incoming/outgoing capture-seam designs | The shared core supplies frames/spans only. Each adapter independently scans prepare bytes and proves its own selected handoff. |
| Incoming direction, caller identity, `fromRanges`, occurrence multiplicity | `INAPPLICABLE` | Future incoming-only adapter; historical requirements in `adr0011-calls-go-gopls-incoming-capture-seam-design.md` | Must never be represented by shared recorder fields or inferred by reversing outgoing records. |
| Outgoing direction, callee identity, caller-side `fromRanges`, occurrence multiplicity | `INAPPLICABLE` | Future outgoing-only adapter; historical requirements in `adr0011-calls-go-gopls-outgoing-capture-seam-design.md` | Must never be represented by shared recorder fields or inferred from incoming records. |
| CALLS provider truth or exact gopls profile authority | `MISSING` and nontransferable | No current recorder symbol; both independent reviews retain `provider_authority=UNAVAILABLE` | Byte custody cannot authenticate provider meaning or establish CALLS truth. |
| Occurrence admission, graph edge emission, grouping, Leiden, feature identity | `INAPPLICABLE` and nontransferable | Outside `sessionruntime`; no authorized write-set here | The recorder and adapters produce no admitted relation or semantic-vector input. |
| Stage 3–7 evidence roles, independent review, profile GO, production admission | `MISSING` and nontransferable | No current recorder symbol; controlled by later ADR0011 artifacts and gates | Equal bytes, reused code, or one direction's review cannot satisfy another direction or later stage. |
| Complete method coverage and every lifecycle branch | `UNKNOWN` | Structural search output for `sessionruntime` and broader repository matches was truncated | A later implementation review must run exact symbol/reference and test census before claiming completeness. |

## Transferable and nontransferable boundaries

### Transferable shared-core facts

The shared core may transfer only:

- exact workspace identity, session ID, generation, request key/JSON-RPC ID, method, and manager-owned transaction sequence where present; workspace is transferable only when derived from the exact managed session/generation custody that owns the transaction, never from session identity alone;
- original request and response frame bytes, frame/body lengths, and frame digests when capture completed;
- frame-relative raw params/result spans and their digests, without re-encoding;
- completed write/read observations and transport terminal facts;
- declared private recorder limits, reservations, use, refusal, release, and overflow-safe accounting;
- owner/lease lifecycle: admission, active borrow, close request, deferred or synchronous finalization, byte clearing, charge release, cancellation/retirement observation;
- explicit incomplete/missing/failed custody states; and
- static production unreachability and enabled/disabled behavior equivalence when proven by later tests.

These facts establish only bounded local transaction provenance and lifecycle consistency. Workspace identity is a classified shared-provenance field, but it exists only under the exact managed workspace/session/generation custody join above; absent or mismatched custody remains missing and refuses before semantic effects. These facts do not authenticate the server, repository, source revision, analyzed document version, or semantic result.

### Nontransferable facts and authority

The following stay owned by each directional adapter and its later independent evidence chain:

- exact incoming or outgoing profile name and provider binding;
- target-envelope or prepared-caller selection policy;
- prepare candidate interpretation and selected-item semantic identity;
- unchanged selected-item handoff proof for that method chain;
- caller/callee orientation;
- meaning and document side of `fromRanges`;
- member/range/occurrence ordinals, multiplicity, zero-site members, `null` versus `[]` semantic terminal classification;
- CALLS occurrence construction, admission, graph normalization, or publication;
- directional limits, evidence roles, manifests, reviews, profile decisions, and GO tokens;
- provider authority, source completeness, runtime-execution proof, production suitability, grouping, Leiden, or feature identity; and
- all Stage 3–7 authority.

No incoming artifact can satisfy an outgoing role, and no outgoing artifact can satisfy an incoming role. Equal transport bytes establish, at most, equal retained bytes under the shared core.

## Directional adapters and zero-overlap responsibilities

The architecture has three owners with intentionally disjoint responsibilities:

### Shared `sessionruntime` transaction-custody extension

Owns only manager-local frame capture, frame-relative spans, exact managed workspace/session/generation custody, transaction identity, neutral predecessor/successor link records, write/read observations, terminal transport/lifecycle state, lease lifetime, recorder accounting, and production-unreachability mechanics. It records exact identity links but does not interpret their Call Hierarchy semantics. It does not import profile identifiers, Call Hierarchy item types for semantic interpretation, `fromRanges` meaning, occurrence types, evidence-role names, or GO tokens.

For the first implementation, the generic private admission policy is closed per directional adapter:

- incoming admits exactly `textDocument/documentSymbol`, `textDocument/prepareCallHierarchy`, and `callHierarchy/incomingCalls`;
- outgoing admits exactly `textDocument/documentSymbol`, `textDocument/prepareCallHierarchy`, and `callHierarchy/outgoingCalls`.

The shared policy is generic in the directional binding, but it never admits the union of both directional methods to either adapter. Any arbitrary method, the opposite directional method, or any other method refuses at the package-private entry/eligibility seam before reservation, cumulative charge, protocol ownership, request-ID consumption, write attempt, callback, diagnostic, retention, or lease publication.

### Incoming-only adapter

Consumes shared custody for exactly:

```text
textDocument/documentSymbol
-> incoming target-envelope binding
-> textDocument/prepareCallHierarchy
-> selected prepared callee/target under the incoming design
-> callHierarchy/incomingCalls
-> incoming caller-to-callee occurrence ledger
```

It alone owns incoming target-envelope binding, deterministic prepared selection, byte-identical incoming request handoff, caller interpretation, incoming `fromRanges` semantics, incoming terminals and directional accounting, incoming evidence roles, review, and future profile GO. Its exact three-transaction chain is `textDocument/documentSymbol` → `textDocument/prepareCallHierarchy` → `callHierarchy/incomingCalls`. Before any incoming semantic effect it must join exact workspace, session, generation, incoming profile, incoming binding, documentSymbol predecessor identity/key/ordinal, prepare predecessor identity/key/ordinal, selected original subspan, and unchanged request-item handoff. Any mismatch—including a same-session foreign prepare, cross-workspace predecessor, outgoing predecessor/binding, stale generation, or swapped selected span—refuses before semantic effects. It emits no outgoing record and cannot call an outgoing assembler.

### Outgoing-only adapter

Consumes shared custody for exactly:

```text
textDocument/documentSymbol
-> outgoing target-envelope binding
-> textDocument/prepareCallHierarchy
-> selected prepared caller under the outgoing design
-> callHierarchy/outgoingCalls
-> outgoing caller-to-callee occurrence ledger
```

It alone owns outgoing target binding, deterministic prepared-caller selection, byte-identical outgoing request handoff, callee interpretation, caller-side `fromRanges`, outgoing terminals and directional accounting, outgoing evidence roles, review, and future profile GO. Its exact three-transaction chain is `textDocument/documentSymbol` → `textDocument/prepareCallHierarchy` → `callHierarchy/outgoingCalls`. Before any outgoing semantic effect it must join exact workspace, session, generation, outgoing profile, outgoing binding, documentSymbol predecessor identity/key/ordinal, prepare predecessor identity/key/ordinal, selected original subspan, and unchanged request-item handoff. Any mismatch—including a same-session foreign prepare, cross-workspace predecessor, incoming predecessor/binding, stale generation, or swapped selected span—refuses before semantic effects. It emits no incoming record and cannot call an incoming assembler.

**Zero overlap rule:** adapter source types, classifiers, occurrence assemblers, profile constants, evidence-role constants, review tokens, and tests are direction-specific. The only common input is the method-neutral shared custody view. If an adapter helper needs a direction switch, union profile enum, shared occurrence struct, or conditional `incoming/outgoing` semantic branch, the boundary has failed and must return to review.

## Candidate implementation write-set for later review only

No implementation is authorized by this assessment. The smallest candidate write-set is limited to existing private `sessionruntime` files and focused tests/fixtures:

1. `sessionruntime/owned_method_pair.go` — narrowly generalize private eligibility/value construction for the three Call Hierarchy methods without changing existing D/R semantics; do not add CALLS interpretation.
2. `sessionruntime/b4_owned_method_pair_lease.go` — only if the method-neutral custody view needs frame-span metadata or additional independently charged byte fields.
3. `sessionruntime/b4_request_frame_lease.go` — reuse, and only if necessary narrowly generalize, request-frame lease eligibility/lifetime; no new owner abstraction.
4. `sessionruntime/b4_response_frame_lease.go` — reuse, and only if necessary narrowly generalize, response-frame lease eligibility/lifetime; preserve manager/caller hold semantics.
5. `sessionruntime/method_frame_correspondence.go` — add method-neutral exact-frame correspondence that consumes retained original request bytes rather than claiming reconstructed bytes are original; preserve current D/R behavior.
6. `sessionruntime/sessionruntime.go` — the minimum private default-off package-private entry and generic per-direction exact-three-method allow policy needed to connect existing transaction ownership to existing leases. Incoming admits only documentSymbol/prepare/incoming; outgoing admits only documentSymbol/prepare/outgoing. No exported/public option and no production default work.
7. `sessionruntime/b4_definition_private.go` — reuse and minimally generalize the existing private transaction assembly seam. This file owns atomic reservation and preflight, transaction installation, exact-frame shrinking and caller-owner transfer, zero-effect refusal cleanup, caller response-frame hold acquisition, terminal release, retirement joining, and stopped-owner retirement. It must remain method-neutral: no definition or Call Hierarchy semantic interpretation may enter the shared core.
8. Existing focused tests, extended rather than duplicated where possible: `sessionruntime/owned_method_pair_test.go`, `sessionruntime/b4_owned_method_pair_lease_test.go`, `sessionruntime/b4_request_frame_lease_test.go`, `sessionruntime/b4_response_frame_lease_test.go`, `sessionruntime/method_frame_correspondence_test.go`, `sessionruntime/roundtrip_test.go`, and `sessionruntime/c_frame_observation_test.go`.
9. At most one small fixture per direction under existing `sessionruntime/testdata/`, containing scripted framed documentSymbol, prepare, and incoming or outgoing responses. Fixtures are test inputs only and confer no evidence or authority.

Directional adapter implementation files are deliberately not named in this candidate write-set: exact package placement remains `UNKNOWN` because the bounded structural output did not establish a zero-overlap test-only composition seam without truncation. That placement requires later review. It must not force a new recorder package or broaden production acquisition interfaces.

Explicitly excluded: CLI, MCP, production registration, capability catalogs, schemas, manifests, evidence artifacts or copies, design edits, public APIs, environment/config/build-tag switches, profile registries, selectors, publication/replay/admission, graph normalization, semantic-vector, production enablement, or commits. Needing any excluded file invalidates this candidate and requires a new assessment.

## RED-first cases for a later authorized implementation

Before implementation, each case must fail for the stated missing behavior, not for a harness or compilation accident:

| RED | Required failing assertion before implementation |
| --- | --- |
| `DRR-R01` | Prepare, incoming, and outgoing each retain exact original request and response frames; a decode/remarshal with equal meaning but different bytes does not satisfy custody. |
| `DRR-R02` | Frame-relative params/result spans reproduce exact source bytes and reject off-by-one, body-relative/frame-relative confusion, substitution, truncation, and digest-only claims. |
| `DRR-R03` | Workspace, session, generation, key, method, write/read observations, owner sequence, and request/response ordinals reject independent stale, foreign, reordered, duplicated, or mismatched substitutions. Workspace must derive only from exact managed session/generation custody; session identity alone and caller-supplied or adapter-inferred workspace refuse. |
| `DRR-R04` | Exact limit succeeds and plus one refuses independently for request frame, response frame, params, result, aggregate retained bytes, objects, spans, hashes, work, allocation, diagnostics, and callback duration; no borrowing or overflow. |
| `DRR-R05` | Release during active callback follows existing deferred-finalization rules, rejects new borrows, clears bytes, releases every charge exactly once, and leaves no usable alias. |
| `DRR-R06` | Cancellation/timeout/short write/read/protocol error preserve the manager's existing terminal and join/retirement behavior; recorder state cannot retry, suppress, replace, or reclassify it. |
| `DRR-R07` | Recorder disabled versus enabled yields identical production `RoundTripResult`, errors, lifecycle, ordering, cancellation, diagnostics, and production accounting. |
| `DRR-R08` | Recorder panic, callback error, hash/allocation refusal, and recorder exhaustion remain bounded and cannot change the underlying managed transaction; no qualification success is produced. |
| `DRR-R09` | Static reachability finds no CLI/MCP/config/environment/build-tag/registry/public-constructor/default route and no production import of a directional adapter. |
| `DRR-R10` | The package-private generic allow policy admits exactly documentSymbol/prepare/incoming for the incoming binding and exactly documentSymbol/prepare/outgoing for the outgoing binding. An arbitrary fourth method, the opposite directional method, and every other disallowed method refuse before reservation/cumulative charge, protocol ownership, request-ID consumption, write attempt, callback, diagnostic, retention, or lease publication; every listed effect remains exactly zero. |
| `DRR-R11` | Incoming and outgoing separately prove their exact documentSymbol→prepare→directional chain. Each joins exact workspace/session/generation/profile/binding plus documentSymbol and prepare predecessor identity/key/ordinal, selected original subspan, and unchanged request-item handoff before semantic effects. Swapped directional predecessors, same-session foreign prepare results, stale generations, cross-workspace substitutions, profile/binding mismatch, and selected-span substitution refuse before semantic effects. The shared core records neutral links but does not interpret them. |
| `DRR-R12` | Incoming adapter cannot consume outgoing profile/evidence/review types, outgoing cannot consume incoming types, and neither can construct the other's occurrence; equal shared custody does not bridge authority. |
| `DRR-R13` | Prepare selection and unchanged handoff are independently proven per direction from original spans; a shared direction switch or shared CALLS occurrence assembler fails the zero-overlap assertion. |
| `DRR-R14` | Original `null`, `[]`, missing payload, malformed complete payload, partial capture, transport error, protocol error, and cancellation remain distinct custody states; shared core emits no CALLS semantic classification. |
| `DRR-R15` | Existing definition/reference/documentSymbol tests and byte/accounting behavior remain unchanged, proving this is a narrow extension rather than replacement. |
| `DRR-R16` | Repository census confirms no changed source/test/fixture outside the approved candidate set and no CLI/MCP/registration/catalog/semantic-vector diff. |

Passing these cases would establish only the private mechanical seam. It would not make evidence roles present, qualify either profile, authenticate gopls, admit CALLS, authorize Stage 3–7, or permit production activation.

## Interpretation of the existing incoming and outgoing designs

The following accepted design references remain historical design inputs but their statements about a “generic recorder,” a new test-only recorder package, or their proposed write-sets must be interpreted through this reuse assessment:

- `docs/qualification/adr0011-calls-go-gopls-incoming-capture-seam-design.md`
- `docs/qualification/adr0011-calls-go-gopls-incoming-capture-seam-review.md`
- `docs/qualification/adr0011-calls-go-gopls-outgoing-capture-seam-design.md`
- `docs/qualification/adr0011-calls-go-gopls-outgoing-capture-seam-review.md`

This assessment does not edit, revoke, or broaden their design-only decisions. It narrows the implementation interpretation: “generic recorder” means the narrow extension of existing private `sessionruntime` owner/lease/transaction machinery described here, not a new recorder package; directional semantics and authority remain separate; and the candidate write-set in this file controls any later implementation proposal unless a new review explicitly supersedes it.

## Unknowns and stop conditions

- Structural search output over the repository and `sessionruntime` was truncated; complete reference coverage, every release path, and exact directional adapter placement are `UNKNOWN` until an untruncated later census.
- No live execution or test run was used to claim recorder behavior. Source-visible contracts are not runtime proof.
- Existing private B4 names are historical implementation names; whether they should be renamed is not established and renaming is not in the candidate write-set.
- Request-frame capture availability for every Call Hierarchy failure point is not established by the inspected symbols. Missing exact frame custody must remain missing/partial, never reconstructed.
- Provider/toolchain/source-revision authentication and exact gopls profile authority remain unavailable.
- Workspace identity is available only as an exact managed workspace/session/generation custody join. If that join is absent, stale, foreign, or ambiguous, workspace provenance is missing and the adapter must refuse before semantic effects; neither session identity nor payload/profile inference may fill the gap.

Stop and return to review if implementation requires a new recorder package, shared directional semantics, a production acquisition interface change, public exposure, any excluded file, or a claim that remarshaled bytes are original transport bytes.

## Gate

The sole proposed implementation-review gate is:

```text
ADR0011_CALLHIERARCHY_DR_RECORDER_REUSE_GO
```

It is **not issued by this document**. If later issued by an independent review over this exact file hash and repository ancestry, it authorizes only a separately bounded implementation proposal within the candidate write-set. It does not authorize evidence acquisition, manifest changes, profile qualification, Stage 3–7 progression, production admission, semantic-vector work, or a commit.
