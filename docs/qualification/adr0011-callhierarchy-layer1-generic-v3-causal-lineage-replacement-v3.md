# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Causal-Lineage Replacement V3

Status: **REVIEW_REQUIRED / NO MUTATION / PRODUCTION BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `78b652537e7a4727977fb89ced1f45ccf1f246a4`
- Supersedes for review: `adr0011-callhierarchy-layer1-generic-v3-exhaustive-causal-lineage-inventory.md`, `adr0011-callhierarchy-layer1-generic-v3-causal-lineage-replacement-v2.md`, and every dependent `11 false / 74 true` mapping statement or review.
- Inputs: committed replacement V2 and the documentation-only attachment-enable occurrence-mapping correction.
- Mutation authority: this document only. No corpus, RED, write-set, source, test, prototype, generated artifact, execution, writer, activation, semantic, carrier-acceptance, or production authority is issued.

This document is the self-contained superseding lineage. It preserves the carrier-complete V2 producers, branches, exact A seams, refusal finalizer, and wrapper transfer while replacing the incorrect attachment partition everywhere. Historical `74 true / 11 false` documents remain immutable historical evidence but are superseded as the current mapping contract.

## 1. State labels and five-column rule

| Label | Meaning |
|---|---|
| `CURRENT_RAW` | Exact value or branch already produced by current `sessionruntime/sessionruntime.go`; it is raw runtime behavior, not the requested immutable generic owner. |
| `PROPOSED_NORMALIZATION` | Finite private producer/builder/finalizer/carrier proposed here solely to make lineage explicit. It is absent from accepted production source and is not accepted. |
| `RED_ONLY` | Derived/asserted by the qualification RED from actual execution; production seam is `NONE`. |
| `FIXTURE_ONLY` | Structural fixture/control assertion; production seam is `NONE`. |

Every inventory row has exactly five lineage columns: source; exact producer/branch; resulting observation; unavailable/nil-empty/known-zero semantics; anti-feed proof.

## 2. Occurrence-sensitive enablement contract

`driver.request.attachment_enabled` is an independently frozen **proposed** driver bit for all 85 occurrences:

```text
false = 11 even W25 standalone executions
      + 11 W25 disabled pair members
      = 22
true  = 11 odd W25 standalone executions
      + 11 W25 enabled pair members
      + 25 roots
      + 16 W24 workers
      = 63
total = 22 + 63 = 85
```

The false standalone set is exactly `w25-execution/02`, `/04`, `/06`, `/08`, `/10`, `/12`, `/14`, `/16`, `/18`, `/20`, and `/22`; the odd standalone IDs `/01` through `/21` are true. The false pair-member set is exactly `w25-pair/01/disabled` through `w25-pair/11/disabled`; each corresponding `/enabled` member is true.

It is not derived from `expected`, `expected.control`, commitment availability/content, attachment zero value, world/name/pair text, occurrence ID at runtime, transport, result, or hidden state.

C agreement is occurrence-sensitive:

- for the 69 occurrences where `expected.control` physically exists (25 roots + 22 W25 executions + 22 W25 pair members), validate `control.enabled`, `control.attachment_available`, and `control.observation_available` only after independently validating the driver bit;
- for all 16 `w24-worker/01..16` occurrences, `expected.control` is physically absent: perform structural validation of that absence only;
- do not inherit or borrow `root/24` controls, manufacture worker controls, treat absence as false, or claim C agreement for absent paths.

The physical-control agreement domain is exactly `22 false / 47 true = 69`: 25 true roots, 11 false even standalone executions, 11 true odd standalone executions, 11 false disabled pair members, and 11 true enabled pair members. The remaining 16 W24 workers are independently true while their controls remain physically absent. Thus the complete domain arithmetic is controls `22 false + 47 true = 69`, W24 omissions `0 false + 16 true = 16`, and all occurrences `22 false + 63 true = 85`. This closes the prior universal-all-85 contradiction without changing the expected-path schema: all 85 have an independently materialized driver bit; only the 69 physically carrying controls have C agreement checks.

### Exact proposed carrier seam — not accepted

All items below are `PROPOSED_NORMALIZATION`:

1. `callHierarchyLayer1RequestAttachment.enabled bool` in `sessionruntime/b4_owned_method_pair_lease.go`.
2. `(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithEnabled(bool) callHierarchyLayer1RequestAttachment` in `sessionruntime/owned_method_pair.go`; it value-returns a copy changing only `enabled`.
3. At the first executable line of `(*Manager).RoundTrip`, before context, session, generation, lifecycle, deadline, document, process, capacity, private-admission, or request-key/allocation checks, read only `req.callHierarchyLayer1.enabled`. If false, keep `attemptLineage == nil`; if true, call `newCallHierarchyLayer1GenericAttemptLineage(req)` and retain that attempt-scoped private builder through every return.
4. Immediately after that construction, the exact Manager entry branch `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting` is evaluated. When true, `attemptLineage` is already non-nil by the accepted caller contract; because private accounting is not implemented, the branch selects the exact proposed enum `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented` and exact proposed typed reason `session.Failure("private-accounting-not-implemented")`, then executes `attemptLineage.recordRefusal(...)`, `attemptLineage.finishRefusal(result)`, owner publication, and return in that order. It preserves the current external `ToolNotImplemented` behavior: the public result remains `ToolNotImplemented`, while the private refusal reason is the exact proposed reason above. No effect may precede refusal finalization.
5. An enabled attempt refused before admission finalizes that builder directly into an immutable refusal owner in `RoundTripResult`; an accepted attempt transfers the same builder pointer (not a replacement builder) through every wrapper to `roundTripWithPrivateTransaction`, where `admit(key, snapshot)` records the allocated request key and before snapshot at the exact position specified in §3.
6. Final publication is only `RoundTripResult.callHierarchyLayer1Generic *callHierarchyLayer1GenericOwner`; disabled returns nil, enabled refusal returns one refusal-only owner, and enabled+admitted returns one ordinary owner. No public field, side table, alternate owner, or content policy is proposed.

The zero value is disabled. Eligibility comes only from the explicit causal input `attachment_enabled`; `expected`, world, commitment, failure text, and branch outcome cannot create the builder. The private-accounting flags may select the named refusal only after the enabled builder exists; they cannot create a builder for a disabled request. This finite field/setter/read/publication branch is a proposal, not accepted carrier authority.

### Exact proposed builder transfer carrier — not accepted

The sole accepted transfer carrier proposed for review is the explicit parameter `attempt *callHierarchyLayer1GenericAttemptLineage`. Exactly one pointer identity is created at enabled `RoundTrip` entry and passed unchanged; disabled calls pass nil. No package global, Manager field, request-hidden state, context value, side table, reconstruction, or replacement builder may carry lineage.

Conceptually, every declaration and call site is changed together as follows:

```text
(*Manager).RoundTrip(ctx, req)
  -> m.roundTrip(ctx, req, attempt)
  -> m.roundTripWithPrivate(ctx, req, ..., attempt)
  -> m.roundTripWithPrivateTransaction(ctx, req, ..., attempt)

(*Manager).RoundTrip(ctx, req) private route
  -> m.roundTripWithPrivate(ctx, req, ..., attempt)
  -> m.roundTripWithPrivateTransaction(ctx, req, ..., attempt)
```

Thus the proposed signatures for `roundTrip`, `roundTripWithPrivate`, and `roundTripWithPrivateTransaction` each add the final explicit `attempt *callHierarchyLayer1GenericAttemptLineage` parameter, and every ordinary/private wrapper call site supplies the same value received from its caller. Branch matrix: disabled ordinary and disabled private routes pass nil and publish no owner; enabled ordinary and enabled private routes pass the one entry-created pointer; enabled pre-admission refusal finalizes/publishes that pointer before return; enabled admitted execution passes it to the transaction and finalizes/publishes it once; the private-accounting-not-implemented branch is enabled-only, refusal-before-effect, and never reaches admission.

## 3. Exact proposed observation construction and call positions

The following private names make each currently absent producer finite. They are `PROPOSED_NORMALIZATION`, not current symbols and not write-set authority.

| Proposed symbol | Exact call position and responsibility |
|---|---|
| `callHierarchyLayer1GenericAttemptLineage` | Exact proposed attempt-scoped private type. Its zero fields are mutable only inside the owning `RoundTrip` call; no pointer escapes until immutable owner construction. |
| `newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest) *callHierarchyLayer1GenericAttemptLineage` | Called at the first executable line of `(*Manager).RoundTrip` iff `req.callHierarchyLayer1.enabled`; this is before every context/session/generation/lifecycle/deadline/document/process/capacity/private-admission/allocation check. Copies only request identity and explicit attachment inputs, marks the attempt eligible, and never reads expected/world/commitment content/failure text. |
| `(*callHierarchyLayer1GenericAttemptLineage).recordRefusal(kind callHierarchyLayer1GenericRefusalType, reason session.Failure)` | Immediately before each exact pre-admission early return in §4.1. Records the finite branch type and the already-selected current failure value; it performs no transport, pending-key, worker, lease, reservation, callback, frame, diagnostic, or other allocation/effect. |
| `(*callHierarchyLayer1GenericAttemptLineage).finishRefusal(result RoundTripResult) RoundTripResult` | Immediately after `recordRefusal` and before that branch returns. Freezes one refusal-only observation, calls `newCallHierarchyLayer1GenericOwner`, and privately installs it in `result.callHierarchyLayer1Generic`. It preserves the branch's existing public failure/result and cannot invoke transport or allocation. Nil receiver returns `result` unchanged, so disabled attempts publish no owner. |
| `(*callHierarchyLayer1GenericAttemptLineage).admit(key lspwire.RequestKey, snapshot callHierarchyLayer1GenericBeforeSnapshot)` | In `roundTripWithPrivateTransaction`, called on the explicitly passed `attempt` at the exact point immediately after `key := r.pending.Begin(req.Generation)` allocates the key and before request registration, identity capture, protocol ownership, worker allocation, or `m.mu.Unlock()`. Mutates the same entry-created pointer to capture that allocated key and the occurrence-local admitted before snapshot; no second builder or hidden carrier is constructed. |
| `(*callHierarchyLayer1GenericAttemptLineage).captureRequestFrame(frame []byte, write RequestWriteObservation)` | After successful `owner.run` and construction of `result.requestWrite`; captures the exact emitted request frame from `writeCapture.retainedFrame()`, never params reserialization. |
| `(*callHierarchyLayer1GenericAttemptLineage).captureResponseFrame(raw []byte, frame lspwire.ReadFrameObservation)` | In `roundTripWithPrivateTransaction` for each successfully emitted/read response-frame before parsing-derived normalization; malformed emitted bytes remain available raw. Control/read-error branches do not fabricate a frame. |
| `(*callHierarchyLayer1GenericAttemptLineage).captureResult(result json.RawMessage, serverError *lspwire.RPCError)` | At the matched-response branch after current assignments to `result.ServerError` and `result.Result`, before any private result clearing and before terminal finalization. Preserves unavailable versus available-empty. |
| `(*callHierarchyLayer1GenericAttemptLineage).finishAccepted(result RoundTripResult, failure session.Failure, terminal manageddiagnostic.Terminal, after callHierarchyLayer1GenericAfterSnapshot) callHierarchyLayer1GenericObservation` | Called inside `finishRoundTrip` after its exact terminal mapping and diagnostic construction, with after snapshot taken while `m.mu` owns session/worker accounting. Produces one immutable admitted value; no expected operand. |
| `newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner` | Called by `finishRefusal` for enabled refusal-only observations and by `finishRoundTrip` after `finishAccepted`; deep-copies all bytes/slices and installs immutable lifecycle state. |
| `publishCallHierarchyLayer1GenericOwner(result RoundTripResult, owner *callHierarchyLayer1GenericOwner) RoundTripResult` | Used by both finalizers at their final private publication point. Enabled refusal and enabled+admitted each get exactly one owner; disabled gets nil. No return after lineage creation may bypass the appropriate finalizer. |

For refusal, entry construction precedes all checked effects and `recordRefusal`/`finishRefusal` precede the exact early return. For acceptance, the same builder crosses admission, its before snapshot is taken at the existing key-allocation boundary, frame/result capture remains in `roundTripWithPrivateTransaction`, and terminal mapping/finalization remains in `finishRoundTrip`. Publication occurs once and immutably.

## 4. Current branch binding

### 4.1 Pre-admission refusal branches

All are `CURRENT_RAW` branches reached after the proposed entry initialization in `(*Manager).RoundTrip`; the exact proposed sequence on each row is `attemptLineage.recordRefusal(type, failure)` then `return attemptLineage.finishRefusal(result)`. Nil-receiver behavior preserves disabled returns. Neither method may change the current returned failure:

| Exact current branch, in evaluation order | Current returned failure | Proposed finite refusal type | Refusal observation mapping |
|---|---|---|---|
| `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting` | `ToolNotImplemented` | `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented` | private refusal reason exactly `session.Failure("private-accounting-not-implemented")`; `recordRefusal`, `finishRefusal`, and owner publication precede return; terminal and admitted state unavailable; no-effect ledger/accounting dimensions available-known-zero as specified below |
| `c18Receipt.ReachRuntime(PointDeadline)==false` | `ToolNotImplemented` | `callHierarchyLayer1GenericRefusalRuntimeGate` | `refused=true`, exact reason; all effect fields unavailable except attempt-scoped no-effect ledger dimensions below |
| `contextFailure(parent)!=""` | exact context failure | `callHierarchyLayer1GenericRefusalContext` | same mapping; reason is the typed current context failure, never parsed text |
| `m.sessions[req.SessionID]==nil` | `SessionNotFound` | `callHierarchyLayer1GenericRefusalSession` | same mapping; session-derived identity/snapshots unavailable |
| generation mismatch | `StaleGeneration` | `callHierarchyLayer1GenericRefusalGeneration` | same mapping; requested generation remains available, accepted generation unavailable |
| session not Ready or `protocolOwned` | `LifecycleConflict` | `callHierarchyLayer1GenericRefusalLifecycle` | same mapping; no protocol ownership acquired |
| private P2 cannot start before effective deadline | `RequestTimeout` | `callHierarchyLayer1GenericRefusalDeadline` | same mapping; this is refusal, not a transport terminal deadline |
| `!matchesOwnedDocument` | `DocumentSupplyUnavailable` | `callHierarchyLayer1GenericRefusalDocument` | same mapping; no document/transport effect inferred |
| process is not `wireChild` | `SessionPoisoned` | `callHierarchyLayer1GenericRefusalProcess` | same mapping; no transport closure/terminal inferred |
| request/worker limits exceeded | `ResourceExhausted` | `callHierarchyLayer1GenericRefusalCapacity` | same mapping; failed capacity check creates no reservation/worker allocation |
| readiness-history freeze/new reader/borrower failure | `ResourceExhausted` | `callHierarchyLayer1GenericRefusalPrivateAdmission` | same mapping; no private admission effect |
| `admitPrivateB4SourcesC17Locked` failure | exact returned failure | `callHierarchyLayer1GenericRefusalPrivateAdmission` | same mapping; exact typed returned failure retained without string inference |

For every refusal-only observation: `refusal.refused=true` and `refusal.reason` is available with the exact typed `session.Failure`; request-key, accepted identity, frames, result, RPC error, external response/notification payloads, admitted before/after snapshots, protocol terminal, and terminal-ledger values are unavailable; `terminal.available=false` (a refusal is not a completed protocol transaction). The attempt-scoped ledger dimensions `callbacks`, `external_bytes`, `leases`, `reservations`, `transport_bytes`, and `transport_messages` are available with `before=0`, `after=0`, `delta=0`, `known_zero=true`, because entry construction and refusal finalization have no such effects. `diagnostics` is unavailable unless the current branch actually produced a diagnostic; no zero is fabricated. Transport/request counters are available-known-zero only where the current `RoundTripResult` itself establishes an attempted count of zero; otherwise they remain unavailable. Specifically for `private-accounting-not-implemented`, both requested private accounting families are ledger/accounting unavailable as implementations, while every enumerated effect dimension proven untouched by the refusal-before-effect branch is available-known-zero (`0/0/0`, `known_zero=true`); no unavailable implementation value is converted to numeric zero.

`key := r.pending.Begin(req.Generation)` remains the admission/key-allocation boundary. The proposed refusal owner now exists only as a finalized immutable observation before that boundary; no pending key, transport, worker, reservation, lease, callback, diagnostic, frame, or admitted snapshot is created to produce it. Early returns after key allocation already funnel through `finishRoundTrip` and must use `finishAccepted` rather than the refusal finalizer.

### 4.2 Terminal mapping branches

`finishRoundTrip` is the exact `CURRENT_RAW` terminal producer:

| Branch | Current mapping |
|---|---|
| `failure==""` | diagnostic `TerminalResponseReceived`, read/write `IOComplete`; successful terminal normalization |
| `failure==RequestTimeout` | event `diagnosticEventTerminalDeadline`; diagnostic `TerminalDeadlineExceeded`; read `IOFailed` |
| `failure==RequestCancelled` | event `diagnosticEventTerminalFailure`; diagnostic `TerminalCancelled`; read `IOFailed` |
| `failure==SessionCrashed` | event `diagnosticEventTerminalFailure`; diagnostic `TerminalTransportClosed`; read `IOFailed` |
| any other nonempty failure | event `diagnosticEventTerminalFailure`; diagnostic `TerminalProtocolError`; read `IOFailed` |
| `result.ServerError!=nil` | numeric RPC code observed and diagnostic terminal forced to `TerminalProtocolError` |
| `poison==true` | `owner.retire()`, session state `Poisoned`, retired transport retained |

The proposed finalizer maps these exact branches; it does not reinterpret error, failure, or terminal from expected/control values.

## 5. Exhaustive five-column lineage

### 5.1 Category A — ordinary occurrence, 116 paths

| Explicit family/members (cardinality) | 1. Source | 2. Exact producer symbol/branch | 3. Observation/status | 4. Unavailable / nil-empty / known-zero | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `accounting.{bytes,diagnostics,objects,work}.{available,known_zero,value}` (12) | Refusal: exact entry attempt and current result; admitted: occurrence-local before/after snapshots plus current request/response/lifecycle counters. | `PROPOSED_NORMALIZATION` entry-created `callHierarchyLayer1GenericAttemptLineage`; refusal `finishRefusal` leaves non-established values unavailable; `admit` snapshots before after key allocation and `finishAccepted` snapshots after inside `finishRoundTrip`. | A immutable triples for enabled refusal or admitted attempt. | `available=false` means value unavailable, never zero; `known_zero=available && value==0`; refusal effects are zero only where §4.1 establishes no effect. | No expected accounting, world/name, C control, aggregate, or failure-string inference may populate snapshots. |
| `commitments.selected.{available,identity,length,sha256}`, `handoff.{available,identity,length,sha256}`, `predecessors[*].{available,identity,length,sha256}` (12 schema paths) | `RoundTripRequest.callHierarchyLayer1` built by exact value-returning attachment setters. | `PROPOSED_NORMALIZATION` builder copies `req.callHierarchyLayer1` after key allocation; predecessor setter remains exact proposed `privateCallHierarchyLayer1WithPredecessors`; owner constructor deep-copies values. | A opaque metadata only. | Unavailable = false/empty identity/0/zero digest; zero predecessors = empty ordered list, not one unavailable member. | Expected commitment metadata and commitment content cannot enable, refuse, or select outcomes. |
| `error.{available,class,code,data_base64,data_form,message,present,serialized_hex}` (8) | Actual matched `lspwire.Message.Error`, current runtime failure, and exact serialized RPC error when present. | `CURRENT_RAW` matched-response branches assigning `result.ServerError` (private lease branch or direct branch); `PROPOSED_NORMALIZATION` `captureResult`; `finish` adds failure class without fabricating payload. | A exact error observation. | No RPC error: payload fields unavailable/null, `present=false`; failure class may still be known. Available-empty data is distinct from unavailable. | Control event/failure cannot fabricate RPC payload; expected error never feeds actual. |
| `external.{available,failure,notifications_hex,responses_hex}` (4) | Current `RoundTripResult.Failure`, `Notifications`, `Responses`, retained private lists, and actual emitted/read messages. | `CURRENT_RAW` notification/request branch, unmatched/late response branches, matched response branch, and `finishRoundTrip` assignment `result.Failure=failure`; `PROPOSED_NORMALIZATION` `finish` freezes exact ordered bytes. | A external observation. | Attempted execution may be available with empty response sequence `""`; unavailable remains null/distinct. | `external.execution_expected` is fixture-only and cannot populate actual external fields. |
| `frames.auxiliary`; `frames.request.{available,frame_hex,frame_length,frame_sha256}`; `frames.response.{available,frame_hex,frame_length,frame_sha256}` (9) | Exact `writeCapture` frame and each `readResult.raw/frame`; auxiliary actual collection only. | `CURRENT_RAW` `newFramedWriteCaptureBounded`, successful `owner.run`, `result.requestWrite`; read goroutine `ReadFrame`/`ReadWithFrameIfWithin`; `PROPOSED_NORMALIZATION` `captureRequestFrame` and `captureResponseFrame`. | A original frame custody. | No response frame = false/null/0/zero digest; malformed emitted frame remains available; auxiliary empty is not a fabricated frame. | Params reserialization, parsed result, RPC error, terminal and expected frames are forbidden sources. |
| `identity.binding.{available,value}`, `generation.{available,value}`, `method.{available,value}`, `profile.{available,value}`, `request_identity.{generation,request_key,session,workspace_nonce_hex}`, `transaction.{available,local_hex}` (14) | Manager-owned session record, request, allocated `lspwire.RequestKey`, diagnostic generation/identity, and private transaction identity. | `CURRENT_RAW` session/generation checks; `key := r.pending.Begin`; `diagnosticGeneration, identity := ...`; `result := RoundTripResult{Key:key...}`. `PROPOSED_NORMALIZATION` builder snapshots these immediately after allocation. | A exact identity. | Each availability bit governs only its sibling; unavailable private zero is not available-empty. | Fixture occurrence/world/pair/name and expected identity cannot create actual identity. |
| `refusal.{reason,refused}` (2) | Exact typed current failures and finite branch type in §4.1; admitted terminal failures retain the exact typed failure in `finishAccepted`. | `CURRENT_RAW` named branches; `PROPOSED_NORMALIZATION` `recordRefusal` + `finishRefusal`, or `finishAccepted`. | A refusal normalization, including immutable refusal-only owner for every enabled pre-admission return. | No refusal = `refused=false`, reason unavailable/null; enabled pre-admission refusal = true + exact available reason; disabled publication alone is not refusal. | Commitment absence/content, `control.reason`, expected/world/name, and failure strings are not policy or inference sources. |
| `result.{available,bytes_hex}` (2) | Actual matched response `read.message.Result` / private captured result. | `CURRENT_RAW` matched-response assignment to `result.Result`; `PROPOSED_NORMALIZATION` `captureResult` before current private clearing. | A raw result. | Unavailable is null; available zero-length bytes remain available-empty. `result.form` stays absent. | Expected/parsed fields cannot feed result bytes. |
| ordinary `round_trip_ledger.dimensions.{callbacks,diagnostics,external_bytes,leases,reservations,transport_bytes,transport_messages}.{before,after,delta,available,known_zero}` (35) | Refusal: §4.1 no-effect facts and any exact current diagnostic fact; admitted: exact occurrence-local snapshots, never root/24 worker aggregate. | `PROPOSED_NORMALIZATION` `finishRefusal` emits §4.1 zero/unavailable mapping; same builder's `admit` records the post-key before snapshot and `finishAccepted` records after inside `finishRoundTrip`. | A for 84 ordinary occurrences, including refusal-only enabled occurrences and all 16 W24 workers. | Refusal callbacks/external-bytes/leases/reservations/transport dimensions are available `0/0/0`, known-zero; diagnostics is unavailable absent an actual diagnostic. Admitted `delta=after-before` only when available. | No expected arithmetic, sibling/worker/root aggregate, failure text, or hidden state feeds snapshots. |
| `round_trip_ledger.terminal.{before,after}.{available,value}` (4) | Refusal has no protocol terminal snapshot; admitted uses before terminal snapshot and exact `finishRoundTrip` terminal branch. | `PROPOSED_NORMALIZATION` `finishRefusal` marks both unavailable; same builder's `admit` captures before and `finishAccepted` captures after §4.2 mapping; admitted owner `requestTerminal` closes admission before publication/drain. | A terminal ledger records. | Refusal before/after are unavailable, not empty or zero; admitted unavailable value remains null/private zero. | Refusal reason, error serialization, expected terminal, and names cannot feed terminal state. |
| `terminal.{available,value}` (2) | Refusal has no protocol terminal; admitted uses exact §4.2 terminal mapping. | `PROPOSED_NORMALIZATION` `finishRefusal` emits unavailable; `CURRENT_RAW` `finishRoundTrip` and proposed `finishAccepted` emit admitted terminal, then generic owner `requestTerminal`/`waitReleased`. | A terminal outcome. | Pre-admission refusal is unavailable; admitted unavailable remains distinct from empty; terminal closes new callback admission before drain. | Refusal failure, expected terminal/error bytes do not choose or synthesize a terminal branch. |
| `transport.{bytes,messages,request_bytes,request_messages}.{available,known_zero,value}` (12) | Current `RoundTripResult.Bytes`, `Messages`, `RequestBytes`, `RequestMessages`. | `CURRENT_RAW` request-body assignment, per-read increments, max-bound branches, and `finishRoundTrip` diagnostic outcome; `PROPOSED_NORMALIZATION` `finish` freezes four triples. | A exact transport accounting. | Attempted no-frame response is available-known-zero; unavailable is distinct. Request counts remain actual. | Expected frame sizes, errors, controls and names cannot supply counts. |

Cardinality: `12+12+8+4+9+14+2+2+35+4+2+12 = 116`.

### 5.2 Category B — ordinary occurrence, 20 paths (`RED_ONLY`, seam `NONE`)

| Explicit members (cardinality) | 1. Source | 2. Exact producer | 3. Derivation/status | 4. Semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `lifecycle.{acquired,completed,released}` and `lifecycle.sequence[*]` (4) | Actual proposed lease acquire/callback/release returns and event order. | `NONE`; RED calls proposed `(RoundTripResult).privateCallHierarchyLayer1GenericLease`, lease `View`/`Callback`/`Release`. | Direct actual B values/order. | Disabled/no owner false and empty sequence; panic completed only after containment returns frozen error. | Expected lifecycle/C cannot trigger calls. |
| `derived.control.{actual_attachment_available,actual_reason}` (2) | Actual private owner discovery/acquisition/view and exact failed prerequisite. | `NONE`; RED inspection only. | B actual publication and exact reason. | Missing owner false; success reason empty; empty differs from unavailable. | C control values never supply actual/reason. |
| `derived.external.{result_bytes,server_error_bytes,notifications_bytes,responses_bytes,failure,messages,bytes,request_messages,request_bytes}` (9) | Exact current `RoundTripResult` and frozen actual serialization. | `NONE`; RED copies actuals after execution. | B exact bytes/counters/failure. | Result/error null iff unavailable; available-empty sequences remain empty; failure preserves absent/present. | Expected A values are comparison targets only. |
| `derived.lifecycle.{observation_available,callback_completed,release_first,release_second}` (4) | Actual lease/View/callback/two Release returns. | `NONE`; RED calls only. | B actual formulas; second false proves one winner. | Disabled/no lease false; callback error transparent; panic contained. | Expected controls/lifecycle cannot drive returns. |
| `derived.identity.transaction_request_equals_request_identity` (1) | Two actual A identities including availability. | `NONE`; RED equality. | B actual-to-actual equality. | Availability participates; available-zero is not unavailable. | No expected operand. |

Cardinality: `4+2+9+4+1 = 20`.

### 5.3 Category C — nine paths (`FIXTURE_ONLY`, seam `NONE`)

| Explicit member (1 each) | 1. Source | 2. Exact producer | 3. Classification/status | 4. Semantics | 5. Anti-feed proof |
|---|---|---|---|---|---|
| `comparison.available` | Fixture structure. | `NONE` | C traversal/cardinality gate. | False means no comparison container, not no capability. | Cannot select request/execution. |
| `concurrency.available` | Fixture structure. | `NONE` | C worker-container gate. | False means no concurrent organization. | Cannot select owner/lifecycle. |
| `control.attachment_available` | Physical fixture value in 69 non-worker occurrences only. | `NONE` | C agreement with independently derived bit where present; W24 worker structural absence validation otherwise. | Absence is not false/unavailable. | Never execution/B operand; no root borrowing. |
| `control.enabled` | Physical fixture value in 69 non-worker occurrences only. | `NONE` | C agreement-only where present. | False is a present disabled value; W24 absence remains absence. | Cannot choose enablement. |
| `control.observation_available` | Physical fixture value in 69 non-worker occurrences only. | `NONE` | C agreement-only where present. | Absence is not unavailable. | Cannot fabricate observation. |
| `control.paired_world` | Fixture pair metadata. | `NONE` | C pair closure/order. | Missing/invalid is fixture RED. | Cannot dispatch sibling/share actual. |
| `control.reason` | Fixture explanation. | `NONE` | C text only. | Empty/present never becomes runtime refusal. | Cannot feed `refusal.reason` or B reason. |
| `external.execution_expected` | Fixture declaration. | `NONE` | C requires represented execution. | False/missing is fixture invalid, not runtime result. | Cannot populate external actuals. |
| `name` | Human/manifest label. | `NONE` | C organization only. | Renaming leaves request/execution unchanged. | No name/world/scenario dispatch. |

C cardinality is 9 for ownership accounting. Physical W24 omission of the three `control.*` members is validated structurally and does not create, inherit, or borrow C values.

### 5.4 World-24 root migration — 35 paths (`RED_ONLY`, seam `NONE`)

Members are seven dimensions × `{before,after,delta,available,known_zero}`:

```text
callbacks, diagnostics, external_bytes, leases, reservations,
transport_bytes, transport_messages
```

| 1. Source | 2. Exact producer | 3. Derivation/status | 4. Semantics | 5. Anti-feed proof |
|---|---|---|---|---|
| The 16 independently executed `w24-worker/01..16` actual A ledgers in fixed order. | `NONE`; RED aggregate only. | `available=AND(worker.available)`; `before=0`; when available `delta=SUM(worker.actual.delta)`, `after=delta`, `known_zero=(delta==0)`. | Any unavailable worker makes aggregate unavailable; never substitute zero. | Root expected aggregate, worker expected values, names, and production root aggregate storage are forbidden. |

All other `root/24` paths are `81 A + 20 B + 9 C`; each worker remains ordinary `116/20/9`.

## 6. Conditional decision matrix

| Conditional | Named carrier/branch | Outcome | Status |
|---|---|---|---|
| Attachment disabled/enabled | Proposed driver bit, attachment field/setter, first executable `RoundTrip` read, `newCallHierarchyLayer1GenericAttemptLineage`, dual finalizers | false exactly on 11 even W25 standalone executions plus 11 disabled pair members: nil transfer and no builder/owner/publication while existing behavior continues; true on all other 63: attempt eligible independently of commitments and every refusal/admitted return publishes one owner | `PROPOSED_NORMALIZATION`, not accepted |
| C agreement | 69 physical-control occurrences partitioned `22 false / 47 true`; 16 true W24 workers with structural absence checks | Compare the three controls downstream only where fields exist; no inherited C, no absent-as-false default | `FIXTURE_ONLY` |
| Root publication | `root/01..25` | all 25 driver bits true; each is eligible for the enabled refusal/admitted publication path | proposed, blocked |
| W24 worker publication | `w24-worker/01..16` | all 16 driver bits true and controls absent; each is eligible for the enabled publication path without C comparison | proposed, blocked |
| Selected/handoff/predecessor availability/order | Exact attachment inputs/setters | Opaque A metadata; no validity policy | proposed owner normalization blocked |
| Params nil/empty/present | `req.Params`, current request encoding/write | Exact request frame and external transport preserve distinction | `CURRENT_RAW` → proposed owner |
| Valid/malformed/no response frame | read goroutine and `readResult.raw/frame/err` branches | Raw available frame; malformed stays raw; no-frame unavailable with attempted known-zero accounting | `CURRENT_RAW` → proposed owner |
| Result unavailable/available-empty/present | matched-response `result.Result` assignment | Exact A result and B copy | `CURRENT_RAW` → proposed owner |
| RPC error absent/present | matched-response `result.ServerError` branches | Exact A error; no fabrication from failures | `CURRENT_RAW` → proposed owner |
| Private C16/C17 accounting requested | `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting`; builder already created; `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented`; `session.Failure("private-accounting-not-implemented")` | Public `ToolNotImplemented` preserved; private `recordRefusal` → `finishRefusal` → owner publication → return; terminal/admitted and unimplemented accounting unavailable, proven untouched effect dimensions known-zero; disabled nil/no owner | current branch + `PROPOSED_NORMALIZATION`, not accepted |
| Explicit builder transfer | `RoundTrip -> roundTrip(..., attempt) -> roundTripWithPrivate(..., attempt) -> roundTripWithPrivateTransaction(..., attempt)` and direct private `RoundTrip -> roundTripWithPrivate(..., attempt)` | One pointer identity when enabled, nil when disabled; every signature/call site explicit; no global/request-hidden state; `admit` immediately after key allocation | `PROPOSED_NORMALIZATION`, not accepted |
| Pre-admission refusal | Entry-created attempt lineage; exact §4.1 branch; `recordRefusal(type, reason)`; `finishRefusal(result)` immediately before return | Enabled: immutable owner with exact typed reason/type, unavailable terminal/admitted state, and explicit zero/unavailable no-effect ledger; disabled: nil owner; no transport/allocation effect | current branches + `PROPOSED_NORMALIZATION`, not accepted |
| Post-admission refusal false/true | Same builder after `admit`; post-key `finishRoundTrip` failures | Exact reason/flag through `finishAccepted`; disabled is not refusal | current branches + proposed normalization |
| Terminal success/timeout/cancel/crash/other | §4.2 `finishRoundTrip` branches only | Exact admitted terminal and terminal-ledger result; pre-admission refusal terminal remains unavailable | current branches + proposed normalization |
| Transport/accounting/ordinary ledger availability/zero | refusal no-effect mapping or admitted current result counters plus same-builder snapshots | Refusal zeros only for effects proven absent; all others unavailable; admitted exact triples/deltas | current raw + proposed normalization |
| Owner publication | proposed `publishCallHierarchyLayer1GenericOwner` from `finishRefusal` or `finishAccepted` path | enabled gets exactly one immutable refusal/admitted owner; disabled gets nil | proposed, blocked |
| View/callback success/error/panic | proposed lease `View`/`Callback`; panic containment | actual B completion/reason/order | `RED_ONLY`, production lifecycle proposed |
| Release/drain/terminal closure | proposed owner `requestTerminal`/`waitReleased`, lease `Release` | close admission, drain admitted callbacks, one winner | `RED_ONLY`, production lifecycle proposed |
| Defensive copying | proposed builder/finalizer/owner constructor and view/callback copies | no caller/request/holder/view aliasing | proposed, blocked |
| World-24 aggregate | sixteen worker actual ledgers | 35 root B values only | `RED_ONLY` |
| W25 standalone execution mapping/publication | exact execution ordinal and independently materialized bit | even `/02..22`: false, nil transfer, no owner/observation publication; odd `/01..21`: true, enabled refusal/admitted publication path | driver/seam proposed, blocked |
| W25 disabled/enabled pair mapping/publication | independently materialized disabled=false and enabled=true bits with independent executions | disabled member has nil transfer and no private owner/observation publication; enabled member is eligible for exactly one refusal/admitted owner publication | driver/seam proposed, blocked |
| W25 execution actual equality | each represented standalone execution compared only with its own retained actual external/transport values | equality is actual-to-retained-actual for that independently executed occurrence; false even executions remain represented and execute existing behavior but cannot publish the private owner; true odd executions may publish it | `RED_ONLY`; publication difference is not an external/transport inequality |
| W25 pair transport/external equality | disabled and enabled members execute independently | actual-to-actual deep equality of transport/external outcomes preserving unavailable versus available-empty and nil versus empty; equality never borrows a sibling actual and does not erase the required private-publication contrast | `RED_ONLY` |

## 7. Exact occurrence accounting

| Occurrence class | Members | Count | Driver false | Driver true | Ownership | Assertions |
|---|---|---:|---:|---:|---:|---:|
| roots except W24 | `root/01..23`, `root/25` | 24 | 0 | 24 | `116/20/9` | 3,480 |
| W24 root | `root/24` | 1 | 0 | 1 | `81/55/9` | 145 |
| W24 workers | `w24-worker/01..16` | 16 | 0 | 16 | `116/20/9`; physical control omission preserved | 2,320 |
| W25 executions | 11 even disabled + 11 odd enabled | 22 | 11 | 11 | `116/20/9` | 3,190 |
| W25 pair members | 11 disabled + 11 enabled | 22 | 11 | 11 | `116/20/9` | 3,190 |
| **total** | exact structural occurrences | **85** | **22** | **63** | occurrence-keyed | **12,325 = 85×145** |

```text
A = 84×116 + 81 = 9,825
B = 84×20  + 55 = 1,735
C = 85×9        =   765
union                  12,325
```

The assertion key is `(occurrence_id, retained_path)`. Duplicate keys, missing keys, skip lists, wildcard exemptions, fallback-to-expected, record/update mode, and golden rewrite are forbidden. The 17 removed/deferred paths remain absent: four request-body, four request-payload, four response-body, four response-result, and `result.form`. The coverage claim now includes every enabled pre-admission return in §4.1, including the exact Manager private-accounting branch: each must prove its named refusal enum, exact typed reason, immutable refusal owner, unavailable terminal/admitted fields, and branch-appropriate zero/unavailable ledger mapping; disabled occurrences must prove nil transfer and absence of both attempt builder publication and owner. It also covers every conceptual declaration/call-site edge in both wrapper routes, one-pointer identity, prohibition of hidden carriers, and the exact post-key-allocation `admit` call. This is proposed review coverage only, not evidence that source or RED currently implements it.

## 8. Anti-feed proof

1. Final drivers are self-contained; registry/occurrence identity is offline authoring input only.
2. Expected data never constructs requests, attachment bits, commitments, transport, owners, observations, snapshots, or B operands.
3. The enable bit is independently materialized `22 false / 63 true`: exactly 11 even W25 standalone executions plus 11 disabled pair members are false; C is downstream agreement only on the 69 physical-control occurrences, partitioned `22 false / 47 true`.
4. All 16 W24 workers are independently true while preserving control absence; no root borrowing, manufacturing, false-defaulting, or inherited agreement.
5. Commitment availability/content never enables, admits, refuses, or selects result/error/terminal behavior.
6. Request/response originals come only from exact emitted frames; malformed remains raw and control/read-error paths fabricate no frame.
7. Production retains occurrence-local observations only; RED alone computes B, W24 root aggregate, and W25 equality.
8. Entry construction receives only the current request and explicit attachment input; refusal recording receives a finite branch enum plus exact typed current failure; only accepted attempts later receive an allocated key, current raw observations, and exact snapshots. No failure-string inference is permitted.
9. Every enabled pre-admission return calls `recordRefusal` and `finishRefusal`; both are refusal-before-effect, and neither can invoke transport, allocation, pending-key, worker, reservation, lease, callback, frame, or diagnostic production.
10. Accepted attempts hand the same entry-created builder through `admit` to `finishAccepted`; no branch reconstructs lineage from `expected`, world, commitment, terminal, or returned strings.
11. Owner construction deep-copies; pointer identity carries no metadata equality meaning.
12. No semantic validity, direction, relation, completeness, authenticity, feature identity, publication authority, or production use is inferred.

## 9. Review disposition

This replacement closes the documentation blockers by:

- giving all 85 occurrences an independently frozen proposed bit with exact `22 false / 63 true` accounting: 11 even disabled standalone executions, 11 odd enabled standalone executions, 11 disabled pair members, 11 enabled pair members, 25 enabled roots, and 16 enabled W24 workers;
- restricting C agreement to the 69 occurrences where controls physically exist with exact `22 false / 47 true` accounting, while validating structural absence and independent true enablement for the 16 W24 workers;
- naming the exact proposed field, value-returning setter, first-line enablement read, attempt-scoped type/constructor, refusal enum/recorder/finalizer, admission handoff, capture methods, accepted finalizer, owner constructor, and publication helper;
- fixing every pre-admission refusal call position as `recordRefusal` then `finishRefusal` immediately before the existing early return, with refusal-before-effect and no transport/allocation side effect;
- specifying branch-by-branch refusal type/reason plus availability, terminal, and ledger semantics, without failure-string inference or expected/world/commitment feed;
- requiring accepted attempts to hand the same entry-created builder as an explicit parameter through `roundTrip`, `roundTripWithPrivate`, and `roundTripWithPrivateTransaction` (including the direct private route), with one pointer identity, nil disabled transfer, no hidden state, and `admit` immediately after key allocation;
- closing the exact Manager `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting` branch with the proposed `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented` / `private-accounting-not-implemented` mapping, refusal finalization and owner publication before return, preserved public `ToolNotImplemented`, and explicit unavailable/known-zero semantics; and
- binding every broad A family/member to exact current source branches or a finite proposed private producer.

These are lineage names and reviewable proposals, not accepted carriers. The corpus, RED, source, tests, prototype, write set, writer, and production remain blocked and unchanged. Prior `74 true / 11 false` documents and their dependent publication outcomes are superseded; they must not be cited as the current mapping contract.

```text
REVIEW_REQUIRED / NO MUTATION
PRODUCTION BLOCKED
CORRECTED MAPPING: FALSE 22 / TRUE 63
PHYSICAL CONTROLS: FALSE 22 / TRUE 47 = 69
W24: 16 TRUE / CONTROL ABSENT
PRIOR 74 TRUE / 11 FALSE DOCS: SUPERSEDED
AUTHORITY: NONE
```
