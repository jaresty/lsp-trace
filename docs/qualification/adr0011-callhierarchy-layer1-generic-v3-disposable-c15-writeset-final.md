# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Review Input

Status: **REVIEW_REQUIRED / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `83e43a14f3d0ec4f3f02f14eb8ff9930ded98fde`
- Required starting state: exact clean worktree at the pinned HEAD
- Scope: documentation-only refresh of the finite disposable prototype write-set
- Source, test, corpus, RED, prototype, generated-artifact, execution, commit, activation, semantic, carrier-acceptance, production-writer, and production authority: **not issued**
- Mutation authorized by this task: this document only

## 1. Controlling inputs and supersession

This document refreshes the disposable C15 boundary from the following accepted current inputs:

1. the qualified current v3 RED and the reviewed predecessor disposable write-set;
2. the private attachment-enable carrier contract, limited to its still-current field/setter shape;
3. the accepted raw-response-custody correction;
4. the accepted semantic-residue correction; and
5. the accepted causal-lineage replacement-v3 and independent review.

The replacement-v3 occurrence mapping controls: exactly 85 occurrences partition as 22 disabled and 63 enabled. Physical C controls exist on exactly 69 occurrences and partition as 22 false and 47 true. All 16 W24 workers are independently enabled while their C controls remain structurally absent. Earlier `11 false / 74 true` or universal-all-85 C-agreement claims are superseded and must not drive the prototype.

This document is a finite write-set proposal for review. It does not accept any proposed carrier, classify any C15 candidate, authorize prototype execution, or authorize production mutation.

## 2. Exact physical file boundary

The causal lineage is physically satisfiable within exactly these three existing production files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

No fourth source file is admitted. If a compiling qualified v3 RED or canonical C15 enumeration requires another physical production file, stop before C15 and report:

```text
BLOCKED — causal lineage requires additional production file: <exact path>; write-set revision and independent review required
```

No source file is authorized by this document. The three paths above are the complete candidate custody boundary only.

## 3. Cross-file causal contract

The prototype, if separately authorized, must satisfy this exact flow:

```text
explicit request attachment
  -> first executable RoundTrip enabled-bit read
  -> one attempt pointer when enabled, nil when disabled
  -> private RoundTripRequest attempt carrier
  -> refusal finalizer before every pre-key early return
  -> unchanged ordinary/private helper signatures
  -> transient RoundTripResult attempt carrier
  -> admit immediately after pending-key allocation
  -> raw request/response/result capture
  -> finishRoundTrip terminal/error/result/accounting/ledger finalizer
  -> clear transient attempt carrier before every external return
  -> one immutable owner publication
  -> (any, bool) lease acquisition
  -> copied View / copied Callback / Release / drain
```

There is no package global, `Manager` field, context value, side table, reconstructed builder, replacement builder, alternate owner, or second publication path. The only attempt transfer is the qualified private `RoundTripRequest` field followed by the transient private `RoundTripResult` field specified in §6.2; neither field is serialized, exposed, retained after finalization, or usable as identity/equality evidence.

Disabled means `req.callHierarchyLayer1.enabled == false`: no attempt object, no generic owner, `(nil, false)` acquisition, no view/callback lifecycle, and no change to external result, error, failure, messages, bytes, transport, accounting, timing, or existing attachments.

## 4. `sessionruntime/owned_method_pair.go`

### 4.1 Exact existing symbols authorized for mutation

The following eight current inert symbols exist at the pinned HEAD and are the complete predecessor mutation allowance:

```go
func (*Manager) privateCallHierarchyRequestIdentity() (callHierarchyRequestIdentity, bool)
func newCallHierarchyObservationLimits(uint64, uint64, uint64, uint64) (callHierarchyObservationLimits, bool)
func (callHierarchyObservationAccounting) validAgainst(callHierarchyObservationLimits) bool
func (callHierarchyObservationTerminal) valid() bool
func newCallHierarchyOriginalRequestFrameObservation(callHierarchyRequestIdentity, []byte, uint64, uint64, uint64, uint64, callHierarchyObservationLimits) (callHierarchyOriginalRequestFrameObservation, bool)
func (callHierarchyOriginalRequestFrameObservation) copyFrameBytes() []byte
func newCallHierarchyOriginalResponseFrameObservation(callHierarchyRequestIdentity, []byte, uint64, uint64, uint64, uint64, callHierarchyObservationLimits) (callHierarchyOriginalResponseFrameObservation, bool)
func (callHierarchyOriginalResponseFrameObservation) copyFrameBytes() []byte
```

Permitted mutation is limited respectively to: minting the actual Manager-owned identity; validating the finite observation limits; validating actual accounting against those limits; validating the finite terminal enum; constructing exact request-frame custody; copying exact request-frame bytes; constructing exact raw response-frame custody; and copying exact response-frame bytes.

The response constructor receives the complete pre-parse emitted/read frame bytes. It must not receive a parsed body, parsed result, normalized frame, synthetic terminal response, serialized RPC error, or control-event rendering. Malformed emitted bytes remain available raw; no-frame control/read-error branches remain unavailable rather than fabricated.

### 4.2 Exact proposed symbols and fields

Only these additions are proposed in this file:

```go
type callHierarchyOpaqueCommitmentList struct {
	values []callHierarchyOpaqueCommitmentMetadata
}

func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithPredecessors(values []callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment

func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithEnabled(enabled bool) callHierarchyLayer1RequestAttachment
```

Permitted semantics:

- `callHierarchyOpaqueCommitmentList.values` is defensively copied exactly once at holder allocation, preserves order/cardinality, and is never mutated thereafter.
- `privateCallHierarchyLayer1WithPredecessors` value-returns a copy changing only `predecessors`; nil/empty canonicalizes to a nil holder; nonempty input allocates one holder and one defensive copy.
- `privateCallHierarchyLayer1WithEnabled` value-returns a copy changing only `enabled`; zero remains disabled; every other attachment field is preserved exactly.

Forbidden semantics:

- holder-pointer identity is not metadata equality;
- no raw commitment bytes, commitment preimages, selected/handoff preimages, semantic validity, refusal policy, direction, target, occurrence, grouping, relation, method-family, token form, qualification, publication, authority, completeness, or production-use inference;
- no mutation of `newCallHierarchyObservationRequest`, old `callHierarchyObservation`, predecessor-chain, selected-item, handoff, allowlist, params/result-subspan, or semantic symbols;
- no additional constructor, type, field, method, accessor, helper, or declaration in this file.

Any additional symbol is a hard stop:

```text
BLOCKED — owned_method_pair.go symbol outside final finite boundary required: <exact symbol>
```

## 5. `sessionruntime/b4_owned_method_pair_lease.go`

### 5.1 Exact existing fields extended

At the pinned HEAD the comparable request attachment is exactly:

```go
type callHierarchyLayer1RequestAttachment struct {
	selected callHierarchyOpaqueCommitmentMetadata
	handoff  callHierarchyOpaqueCommitmentMetadata
	ledger   callHierarchyTransportEffectLedger
}
```

The only proposed field additions are:

```go
predecessors *callHierarchyOpaqueCommitmentList
enabled      bool
```

The resulting proposed field set is exactly:

```go
type callHierarchyLayer1RequestAttachment struct {
	selected     callHierarchyOpaqueCommitmentMetadata
	handoff      callHierarchyOpaqueCommitmentMetadata
	ledger       callHierarchyTransportEffectLedger
	predecessors *callHierarchyOpaqueCommitmentList
	enabled      bool
}
```

`predecessors` may be assigned only by `privateCallHierarchyLayer1WithPredecessors`; `enabled` may be assigned only by `privateCallHierarchyLayer1WithEnabled`. Nil predecessor holder is the sole empty/unavailable representation. The pointer preserves attachment comparability but has no equality semantics for metadata. `enabled` affects only private owner-publication eligibility.

### 5.2 Exact proposed normalization types and every field

The complete proposed private type declarations in this file are exactly the following. Every field and exact Go type is closed here; there are no maps, anonymous structs, open sets, omitted fields, or implementation-selected additions.

```go
type callHierarchyLayer1GenericRefusalType uint8

const (
	callHierarchyLayer1GenericRefusalUnavailable callHierarchyLayer1GenericRefusalType = iota
	callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented
	callHierarchyLayer1GenericRefusalRuntimeGate
	callHierarchyLayer1GenericRefusalContext
	callHierarchyLayer1GenericRefusalSession
	callHierarchyLayer1GenericRefusalGeneration
	callHierarchyLayer1GenericRefusalLifecycle
	callHierarchyLayer1GenericRefusalDeadline
	callHierarchyLayer1GenericRefusalDocument
	callHierarchyLayer1GenericRefusalProcess
	callHierarchyLayer1GenericRefusalCapacity
	callHierarchyLayer1GenericRefusalPrivateAdmission
)

type callHierarchyLayer1GenericAvailableUint64 struct {
	value     uint64
	available bool
	knownZero bool
}

type callHierarchyLayer1GenericAvailableString struct {
	value     string
	available bool
}

type callHierarchyLayer1GenericLedgerDimension struct {
	before    uint64
	after     uint64
	delta     uint64
	available bool
	knownZero bool
}

type callHierarchyLayer1GenericLedgerDimensions struct {
	callbacks         callHierarchyLayer1GenericLedgerDimension
	diagnostics       callHierarchyLayer1GenericLedgerDimension
	externalBytes     callHierarchyLayer1GenericLedgerDimension
	leases            callHierarchyLayer1GenericLedgerDimension
	reservations      callHierarchyLayer1GenericLedgerDimension
	transportBytes    callHierarchyLayer1GenericLedgerDimension
	transportMessages callHierarchyLayer1GenericLedgerDimension
}

type callHierarchyLayer1GenericTerminalLedger struct {
	before callHierarchyLayer1GenericAvailableString
	after  callHierarchyLayer1GenericAvailableString
}

type callHierarchyLayer1GenericRoundTripLedger struct {
	dimensions callHierarchyLayer1GenericLedgerDimensions
	terminal   callHierarchyLayer1GenericTerminalLedger
}

type callHierarchyLayer1GenericCommitments struct {
	selected     callHierarchyOpaqueCommitmentMetadata
	handoff      callHierarchyOpaqueCommitmentMetadata
	predecessors []callHierarchyOpaqueCommitmentMetadata
}

type callHierarchyLayer1GenericFrame struct {
	frameBytes  []byte
	frameLength uint64
	frameSHA256 [32]byte
	available   bool
}

type callHierarchyLayer1GenericFrames struct {
	auxiliary []callHierarchyLayer1GenericFrame
	request   callHierarchyLayer1GenericFrame
	response  callHierarchyLayer1GenericFrame
}

type callHierarchyLayer1GenericError struct {
	class               string
	code                int64
	codeAvailable       bool
	data                []byte
	dataAvailable       bool
	dataForm            callHierarchyJSONTokenForm
	message             string
	messageAvailable    bool
	present             bool
	serializedBytes     []byte
	serializedAvailable bool
	available           bool
}

type callHierarchyLayer1GenericExternal struct {
	failure            session.Failure
	notificationsBytes []byte
	responsesBytes     []byte
	available          bool
}

type callHierarchyLayer1GenericResult struct {
	bytes     json.RawMessage
	available bool
}

type callHierarchyLayer1GenericTransport struct {
	bytes           callHierarchyLayer1GenericAvailableUint64
	messages        callHierarchyLayer1GenericAvailableUint64
	requestBytes    callHierarchyLayer1GenericAvailableUint64
	requestMessages callHierarchyLayer1GenericAvailableUint64
}

type callHierarchyLayer1GenericBeforeSnapshot struct {
	accounting callHierarchyObservationAccounting
	ledger     callHierarchyTransportEffectLedger
}

type callHierarchyLayer1GenericAfterSnapshot struct {
	accounting callHierarchyObservationAccounting
	ledger     callHierarchyTransportEffectLedger
	terminal   callHierarchyTransportTerminalSnapshot
}

type callHierarchyLayer1GenericObservation struct {
	accounting callHierarchyObservationAccounting
	commitments callHierarchyLayer1GenericCommitments
	error callHierarchyLayer1GenericError
	external callHierarchyLayer1GenericExternal
	frames callHierarchyLayer1GenericFrames
	identity callHierarchyObservationIdentity
	refusalType callHierarchyLayer1GenericRefusalType
	refusalReason session.Failure
	refused bool
	result callHierarchyLayer1GenericResult
	roundTripLedger callHierarchyLayer1GenericRoundTripLedger
	terminal callHierarchyLayer1GenericAvailableString
	transport callHierarchyLayer1GenericTransport
}

type callHierarchyLayer1GenericAttemptLineage struct {
	requestIdentity callHierarchyRequestIdentity
	profile callHierarchyProfileIdentity
	binding callHierarchyBindingIdentity
	method callHierarchyMethodIdentity
	commitments callHierarchyLayer1GenericCommitments
	refusalType callHierarchyLayer1GenericRefusalType
	refusalReason session.Failure
	refused bool
	key lspwire.RequestKey
	admitted bool
	before callHierarchyLayer1GenericBeforeSnapshot
	requestFrame callHierarchyLayer1GenericFrame
	responseFrame callHierarchyLayer1GenericFrame
	result callHierarchyLayer1GenericResult
	serverError *lspwire.RPCError
	finished bool
}

type callHierarchyLayer1GenericOwner struct {
	mu sync.Mutex
	cond sync.Cond
	observation callHierarchyLayer1GenericObservation
	active uint32
	admissionClosed bool
	releaseStarted bool
	released bool
}

type callHierarchyLayer1GenericView struct {
	accounting callHierarchyObservationAccounting
	commitments callHierarchyLayer1GenericCommitments
	error callHierarchyLayer1GenericError
	external callHierarchyLayer1GenericExternal
	frames callHierarchyLayer1GenericFrames
	identity callHierarchyObservationIdentity
	refusalType callHierarchyLayer1GenericRefusalType
	refusalReason session.Failure
	refused bool
	result callHierarchyLayer1GenericResult
	roundTripLedger callHierarchyLayer1GenericRoundTripLedger
	terminal callHierarchyLayer1GenericAvailableString
	transport callHierarchyLayer1GenericTransport
}

type callHierarchyLayer1GenericLease struct {
	owner *callHierarchyLayer1GenericOwner
}
```

The constant block above is complete: zero plus the eleven distinct enum values needed by the twelve §7 rows (the two private-admission rows deliberately share one value). No other enum value is permitted. `callHierarchyLayer1GenericObservation` and `callHierarchyLayer1GenericView` contain only A116 fields. B20, all C paths, the W24 root aggregate, and W25 equality are absent. No additional normalization, refusal, snapshot, observation, owner, view, lease, callback, drain, copy, registry-entry, or helper type is permitted.

### 5.3 Exact proposed producer and lifecycle symbols

The complete proposed symbol/signature set in this file is:

```go
func newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest) *callHierarchyLayer1GenericAttemptLineage
func (attempt *callHierarchyLayer1GenericAttemptLineage) recordRefusal(kind callHierarchyLayer1GenericRefusalType, reason session.Failure)
func (attempt *callHierarchyLayer1GenericAttemptLineage) finishRefusal(result RoundTripResult) RoundTripResult
func (attempt *callHierarchyLayer1GenericAttemptLineage) admit(key lspwire.RequestKey, snapshot callHierarchyLayer1GenericBeforeSnapshot)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureRequestFrame(frame []byte, write RequestWriteObservation)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureResponseFrame(raw []byte, frame lspwire.ReadFrameObservation)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureResult(result json.RawMessage, serverError *lspwire.RPCError)
func (attempt *callHierarchyLayer1GenericAttemptLineage) finishAccepted(result RoundTripResult, failure session.Failure, terminal manageddiagnostic.Terminal, after callHierarchyLayer1GenericAfterSnapshot) callHierarchyLayer1GenericObservation
func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner
func publishCallHierarchyLayer1GenericOwner(result RoundTripResult, owner *callHierarchyLayer1GenericOwner) RoundTripResult
func (result RoundTripResult) privateCallHierarchyLayer1GenericLease() (any, bool)
func (lease callHierarchyLayer1GenericLease) View() (callHierarchyLayer1GenericView, bool)
func (lease callHierarchyLayer1GenericLease) Callback(fn func(callHierarchyLayer1GenericView) error) error
func (lease callHierarchyLayer1GenericLease) Release() bool
func (owner *callHierarchyLayer1GenericOwner) releaseLocked() bool
func (owner *callHierarchyLayer1GenericOwner) requestTerminal()
func (owner *callHierarchyLayer1GenericOwner) waitReleased()
```

Permitted mutation and semantics are exact:

- constructor: copy only request identity and explicit attachment inputs; never inspect expected/world/occurrence/commitment content/failure text;
- refusal recorder: record only the finite enum and already-selected typed current failure; perform no allocation/effect named in §7;
- refusal finalizer: nil receiver is unchanged/no owner; enabled receiver freezes exactly one refusal observation and publishes exactly one owner while preserving the public result/failure;
- admission: mutate the same pointer immediately after `pending.Begin`, recording the allocated key and before snapshot only;
- request capture: copy the exact emitted request frame from retained write custody, never params reserialization;
- response capture: copy `readResult.raw` before parsing-derived normalization; retain malformed emitted bytes; fabricate nothing for control/read-error/no-frame branches;
- result capture: preserve raw result and RPC-error availability after current result/error assignment and before any private clearing;
- accepted finalizer: normalize current terminal/error/result/accounting/ledger values only after current terminal mapping and under the exact snapshot custody described above;
- owner constructor/publication: deep-copy all mutable bytes/slices, initialize `cond.L = &owner.mu`, publish once, and append that same owner pointer once to the exact session registry described in §6;
- ordinary `RoundTrip` finalization returns the published owner open and acquirable; it MUST NOT call `requestTerminal` or `waitReleased`;
- acquisition: return `(nil, false)` when owner is absent, admission is closed, or owner is released; otherwise return the dynamic `callHierarchyLayer1GenericLease` as `any` and `true`;
- `View`: lock the owner; fail when closed/released; otherwise deep-copy the observation, unlock, and return the copied view and `true`;
- `Callback`: reject nil lease/nil callback; under the owner lock reject closed/released, increment `active`, build a fresh deep-copied view, and unlock; invoke the callback under a defer that converts any panic to `errors.New("call hierarchy layer1 generic callback panicked")`, preserves a nonpanic callback error unchanged, then locks, decrements `active` exactly once, broadcasts when it reaches zero, and unlocks;
- `releaseLocked`: the sole owner-state transition that clears retained observation custody and sets the release bits; the caller MUST hold `owner.mu` and MUST have observed `active == 0`; if `releaseStarted || released`, return false without mutation; otherwise set `releaseStarted=true`, assign `callHierarchyLayer1GenericObservation{}` to `observation`, set `released=true`, broadcast, and return true; no other symbol may clear `observation` or set `releaseStarted`/`released`;
- explicit `Release`: nil-safe; lock, set `admissionClosed=true`, broadcast, wait while `active != 0`, call `releaseLocked`, unlock, and return exactly its result; among all explicit and teardown racers exactly one call returns true, and every repeated or losing call returns false;
- `requestTerminal`: nil-safe forced teardown release; lock, set `admissionClosed=true`, broadcast, wait while `active != 0`, call the same `releaseLocked`, and unlock; it may win when user code has not released and never waits for an absent user actor; after it wins, abandoned leases and views are invalid and all later acquisition, `View`, and `Callback` operations fail closed;
- callback completion, including the panic-safe defer path, decrements `active` exactly once and broadcasts at zero, so either release path drains admitted callbacks without deadlock;
- `waitReleased`: nil-safe and observational only; lock and wait while `released == false`, then unlock; it is called only after `requestTerminal`, whose forced transition guarantees completion, so its bound is the drain of callbacks active when admission closed, never a user `Release`.

Forbidden semantics include public/exported acquisition, B/C storage, W24 aggregation, W25 equality, callback-result reinterpretation, alternate release winners, pointer-equality metadata, semantic parsing, persistence, replay, telemetry, logging, diagnostics, or parallel lifecycle/accounting machinery.

## 6. `sessionruntime/sessionruntime.go`

### 6.1 Exact existing symbols and fields touched

The current relevant declarations at the pinned HEAD are:

```go
func (m *Manager) RoundTrip(parent context.Context, req RoundTripRequest) RoundTripResult
func (m *Manager) roundTrip(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool) RoundTripResult
func (m *Manager) roundTripWithPrivate(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, private *privateB4Reservation) RoundTripResult
func (m *Manager) roundTripWithPrivateTransaction(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, transaction *privateB4Transaction) RoundTripResult
func (m *Manager) finishRoundTrip(id string, owner *ownedTransport, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult
```

Current raw response custody exists in the transaction-local read result:

```go
type readResult struct {
	message    lspwire.Message
	frame      lspwire.ReadFrameObservation
	raw        []byte
	bodyBytes  int64
	successor  lspwire.SuccessorReadObservation
	frameOwner *privateB4ResponseFrameOwner
	retained   bool
	err        error
}
```

The current result already has `callHierarchyObservation` and `callHierarchyLayer1` attachments. They must not be repurposed.

### 6.2 Exact proposed fields and unchanged signatures

The only proposed attempt-carrier fields are:

```go
// in RoundTripRequest
callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage

// in RoundTripResult; transient and always cleared before an externally observable return
callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
```

The separate immutable-owner result field remains:

```go
callHierarchyLayer1Generic *callHierarchyLayer1GenericOwner `json:"-"`
```

All existing helper signatures remain byte-for-byte unchanged:

```go
func (m *Manager) RoundTrip(parent context.Context, req RoundTripRequest) RoundTripResult
func (m *Manager) roundTrip(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool) RoundTripResult
func (m *Manager) roundTripWithPrivate(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, private *privateB4Reservation) RoundTripResult
func (m *Manager) roundTripWithPrivateTransaction(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, transaction *privateB4Transaction) RoundTripResult
func (m *Manager) finishRoundTrip(id string, owner *ownedTransport, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult
```

At the first executable line, `RoundTrip` unconditionally overwrites all caller-supplied custody with `req.callHierarchyLayer1GenericAttempt = newCallHierarchyLayer1GenericAttemptLineage(req)`. The constructor remains default-off: enabled allocates exactly one attempt; disabled returns nil; it never reads the incoming carrier field. Existing by-value request flow carries that same pointer through `roundTrip`, `roundTripWithPrivate`, and `roundTripWithPrivateTransaction` without any signature change. No stage rebuilds, clears, replaces, compares, indexes, registers, or derives another attempt.

Immediately after `key := r.pending.Begin(req.Generation)` and before request registration, identity capture, protocol ownership, worker allocation, or `m.mu.Unlock()`, `roundTripWithPrivateTransaction` reads `attempt := req.callHierarchyLayer1GenericAttempt`, admits that pointer, and copies it exactly once into the transaction's one `RoundTripResult` initialization as `callHierarchyLayer1GenericAttempt: req.callHierarchyLayer1GenericAttempt`. All later admitted capture and terminal mutations consume only that transient result field, and every existing `finishRoundTrip` call carries the result unchanged.

Every pre-admission return finalizes the request-carried attempt directly through `finishRefusal`; it never places the pointer in the transient result field and never calls `finishRoundTrip` for carrier transfer. Every admitted return reaches `finishRoundTrip`; it reads only `result.callHierarchyLayer1GenericAttempt`, performs the existing terminal/error/result/accounting/ledger finalization and at most one owner publication, assigns the final owner field, then unconditionally clears the transient field before returning. Disabled and direct legacy nil-attempt paths preserve current public behavior and return no owner. The owner field is not the attempt carrier and remains open/acquirable after successful publication. Neither carrier field has a JSON tag, public setter, exported alias, interface indirection, wrapper, serialization, or persistence semantics.

No field is added to `Manager`, `privateB4Reservation`, `privateB4Transaction`, `ownedTransport`, context, or another carrier. No `AdmitSeedBinding`, `RoundTripPrivateB4`, equivalent bridge interface, alternate private wrapper, or signature-level pointer parameter is proposed or implied.

Permitted mutations are limited to:

1. first-line unconditional anti-forgery overwrite of the private `RoundTripRequest` carrier with the sole builder result;
2. preservation of that exact pointer through existing by-value request flow with unchanged helper signatures;
3. the exact pre-admission refusal calls in §7, directly against the request carrier and without using the transient result carrier;
4. `attempt.admit(...)` immediately after `key := r.pending.Begin(req.Generation)` and before request registration, identity capture, protocol ownership, worker allocation, or `m.mu.Unlock()`;
5. exactly one pointer-value copy into the transient private `RoundTripResult` field in the transaction's one result initialization;
6. request-frame capture after successful `owner.run` and `result.requestWrite` construction;
7. response-frame capture from `readResult.raw` before parsing-derived normalization;
8. result/error capture after current `result.ServerError`/`result.Result` assignments and before private clearing;
9. terminal/error/result/accounting/ledger finalization inside `finishRoundTrip` after exact terminal mapping and diagnostic construction, with the after snapshot taken while `m.mu` owns session/worker accounting;
10. exactly one private publication before return, leaving the owner open/acquirable, followed by unconditional clearing of `result.callHierarchyLayer1GenericAttempt`; and
11. no ordinary-return `requestTerminal` or `waitReleased`; lifecycle joining occurs only at the exact teardown branches below.

The sole proposed session registry field is:

```go
// in runtimeSession
callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner
```

There is no Manager-global owner field, map, side table, registry-entry type, or owner back-pointer. `publishCallHierarchyLayer1GenericOwner`, while `finishRoundTrip` still holds `m.mu`, appends the newly published pointer exactly once to `m.sessions[id].callHierarchyLayer1GenericOwners` when the generation still matches. Ownership then splits exactly: `RoundTripResult.callHierarchyLayer1Generic` permits lease acquisition, while the session slice permits only teardown joining.

Exact invocation ownership and integration call sites are:

1. `(*Manager).finishRoundTrip`, after terminal mapping/after-snapshot normalization and before `m.mu.Unlock`, constructs, publishes, and registers the owner. It does not request terminal state or wait.
2. `(*Manager).runLifecycle`, completed STOP branch, immediately before `delete(m.sessions, operation.SessionID)`, uses this exact call order: while holding `m.mu`, copy `r.callHierarchyLayer1GenericOwners` to a local `[]*callHierarchyLayer1GenericOwner`; set the session field to nil; unlock `m.mu`; for every copied owner call `owner.requestTerminal()` and then `owner.waitReleased()`; reacquire `m.mu`; call `delete(m.sessions, operation.SessionID)`; complete the operation. `requestTerminal` itself arbitrates an already released owner through `releaseLocked`; no pre-check may replace or reorder the pair.
3. `(*Manager).runLifecycle`, completed RESTART branch, immediately before successor startup, uses this exact call order for the old generation: while holding `m.mu`, copy the owner slice; set the old-generation session field to nil; unlock `m.mu`; for every copied owner call `owner.requestTerminal()` and then `owner.waitReleased()`; reacquire `m.mu`; call `m.starter.Start`. The successor begins with `r.callHierarchyLayer1GenericOwners = nil`.
4. `(*Manager).Shutdown`, after `m.workers` reaches zero, uses this exact call order: while holding `m.mu`, append every remaining session's finite owner slice into one local `[]*callHierarchyLayer1GenericOwner`; nil every session slice; unlock `m.mu`; for every copied owner call `owner.requestTerminal()` and then `owner.waitReleased()`; reacquire `m.mu`; continue the existing shutdown-owned release sequence; perform the existing final unlock. `requestTerminal`/`waitReleased` remain mandatory for already-released owners and complete immediately through the shared state.
5. Failure branches in `runLifecycle` before completed STOP/RESTART do not detach or terminalize the owners because the session remains present/poisoned and no completed manager/session teardown has occurred.

No other function may call `requestTerminal` or `waitReleased`. No alternate transaction path, parser, serializer, retry, diagnostic, transport behavior, public result behavior, public error behavior, public accounting behavior, timing change, feature flag, CLI/MCP/schema behavior, or `internal/lspwire` mutation is permitted.

## 7. Complete pre-admission refusal closure

For every enabled pre-key return, the exact sequence is:

```text
attempt := req.callHierarchyLayer1GenericAttempt
attempt.recordRefusal(<exact enum>, <exact typed private reason>)
result = attempt.finishRefusal(<current public result>)
return result
```

The direct return never uses `RoundTripResult.callHierarchyLayer1GenericAttempt` and never calls `finishRoundTrip` for carrier transfer. No helper parameter, alternate bridge, or reconstruction is permitted. For disabled requests the overwritten nil request carrier preserves the current return and publishes no owner.

The finite refusal rows, in current evaluation order, are:

| Current branch | Exact proposed refusal enum | Exact reason source | Public behavior |
|---|---|---|---|
| `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting` | `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented` | `session.Failure("private-accounting-not-implemented")` | preserve `ToolNotImplemented` |
| C18 runtime gate not reached | `callHierarchyLayer1GenericRefusalRuntimeGate` | existing `ToolNotImplemented` branch value | unchanged |
| `contextFailure(parent) != ""` | `callHierarchyLayer1GenericRefusalContext` | exact typed context failure | unchanged |
| session absent | `callHierarchyLayer1GenericRefusalSession` | `SessionNotFound` | unchanged |
| generation mismatch | `callHierarchyLayer1GenericRefusalGeneration` | `StaleGeneration` | unchanged |
| not Ready or protocol-owned | `callHierarchyLayer1GenericRefusalLifecycle` | `LifecycleConflict` | unchanged |
| private P2 cannot start before deadline | `callHierarchyLayer1GenericRefusalDeadline` | `RequestTimeout` | unchanged |
| owned document mismatch | `callHierarchyLayer1GenericRefusalDocument` | `DocumentSupplyUnavailable` | unchanged |
| process is not `wireChild` | `callHierarchyLayer1GenericRefusalProcess` | `SessionPoisoned` | unchanged |
| request/worker limit exceeded | `callHierarchyLayer1GenericRefusalCapacity` | `ResourceExhausted` | unchanged |
| readiness-history reader/borrower failure | `callHierarchyLayer1GenericRefusalPrivateAdmission` | `ResourceExhausted` | unchanged |
| `admitPrivateB4SourcesC17Locked` failure | `callHierarchyLayer1GenericRefusalPrivateAdmission` | exact typed returned failure | unchanged |

No failure-string parsing is permitted. The private-accounting branch is exact: the enabled builder already exists, no effect precedes refusal finalization, private C16/C17 accounting values remain unavailable because unimplemented, and the enumerated untouched effect dimensions alone are available-known-zero. Public `ToolNotImplemented` remains unchanged.

Every refusal observation has refusal true and exact typed reason; request key, accepted identity, frames, result, RPC payload, external response/notification payload, admitted snapshots, protocol terminal, and terminal ledger are unavailable. `callbacks`, `external_bytes`, `leases`, `reservations`, `transport_bytes`, and `transport_messages` are `before=0`, `after=0`, `delta=0`, `available=true`, `known_zero=true` only because refusal precedes those effects. Diagnostics and other counters are unavailable unless the current result independently establishes them. Unavailable is never converted to zero.

Any early return after key allocation is admitted and must funnel through `finishRoundTrip`; it must not use `finishRefusal`.

## 8. A116 normalization and current sources

The immutable production observation/view owns exactly the ordinary-occurrence A116 family. It stores no B or C value.

Every ordinary A path maps to exactly the following field or finite projection. Braced members expand individually; `predecessors[*]` projects the ordered slice without changing cardinality. These expansions total exactly 116. No B or C path has a field or projection.

| Exact A path(s) | Exact owner/view field or projection |
|---|---|
| `accounting.bytes.{value,available}` | `accounting.bytes.{value,available}` (`callHierarchyMeasure`) |
| `accounting.bytes.known_zero` | `accounting.bytes.available && accounting.bytes.value == 0` |
| `accounting.diagnostics.{value,available}` | `accounting.diagnostics.{value,available}` |
| `accounting.diagnostics.known_zero` | `accounting.diagnostics.available && accounting.diagnostics.value == 0` |
| `accounting.objects.{value,available}` | `accounting.objects.{value,available}` |
| `accounting.objects.known_zero` | `accounting.objects.available && accounting.objects.value == 0` |
| `accounting.work.{value,available}` | `accounting.work.{value,available}` |
| `accounting.work.known_zero` | `accounting.work.available && accounting.work.value == 0` |
| `commitments.selected.{identity,sha256,length,available}` | `commitments.selected.{identity,sha256,length,available}`; `sha256` projects lowercase hex of `[32]byte` |
| `commitments.handoff.{identity,sha256,length,available}` | `commitments.handoff.{identity,sha256,length,available}`; same digest projection |
| `commitments.predecessors[*].{identity,sha256,length,available}` | ordered `commitments.predecessors[*].{identity,sha256,length,available}`; same digest projection |
| `error.available` | `error.available` |
| `error.class` | `error.class` |
| `error.code` | `nil` unless `error.codeAvailable`, else `error.code` |
| `error.data_base64` | `nil` unless `error.dataAvailable`, else standard base64 of `error.data` |
| `error.data_form` | exact string projection of `error.dataForm` |
| `error.message` | `nil` unless `error.messageAvailable`, else `error.message` |
| `error.present` | `error.present` |
| `error.serialized_hex` | `nil` unless `error.serializedAvailable`, else lowercase hex of `error.serializedBytes` |
| `external.available` | `external.available` |
| `external.failure` | nullable string projection of `external.failure` (`nil` iff empty) |
| `external.notifications_hex` | lowercase hex of `external.notificationsBytes` |
| `external.responses_hex` | lowercase hex of `external.responsesBytes` |
| `frames.auxiliary` | ordered deep-copied `frames.auxiliary`; each element uses the same four frame projections below |
| `frames.request.available` | `frames.request.available` |
| `frames.request.frame_hex` | `nil` unless available, else lowercase hex of `frames.request.frameBytes` |
| `frames.request.frame_length` | `frames.request.frameLength` |
| `frames.request.frame_sha256` | lowercase hex of `frames.request.frameSHA256` |
| `frames.response.available` | `frames.response.available` |
| `frames.response.frame_hex` | `nil` unless available, else lowercase hex of `frames.response.frameBytes` |
| `frames.response.frame_length` | `frames.response.frameLength` |
| `frames.response.frame_sha256` | lowercase hex of `frames.response.frameSHA256` |
| `identity.binding.available` | `identity.binding != ""` |
| `identity.binding.value` | string projection of `identity.binding` |
| `identity.generation.available` | `identity.request.generation != 0` |
| `identity.generation.value` | `uint64(identity.request.generation)` |
| `identity.method.available` | `identity.method != ""` |
| `identity.method.value` | string projection of `identity.method` |
| `identity.profile.available` | `identity.profile != ""` |
| `identity.profile.value` | string projection of `identity.profile` |
| `identity.request_identity.generation` | `uint64(identity.request.generation)` |
| `identity.request_identity.request_key` | finite string/number projection of `identity.request.requestKey` preserving its existing kind |
| `identity.request_identity.session` | string projection of `identity.request.session` |
| `identity.request_identity.workspace_nonce_hex` | lowercase hex of `identity.request.workspace.managerNonce` |
| `identity.transaction.available` | `identity.transaction.request.generation != 0` |
| `identity.transaction.local_hex` | lowercase hex of `identity.transaction.local` |
| `refusal.reason` | nullable string projection of `refusalReason`; `refusalType` selects no text |
| `refusal.refused` | `refused` |
| `result.available` | `result.available` |
| `result.bytes_hex` | `nil` unless available, else lowercase hex of `result.bytes` |
| for each `d` in exactly `{callbacks,diagnostics,external_bytes,leases,reservations,transport_bytes,transport_messages}`, `round_trip_ledger.dimensions.d.{before,after,delta,available,known_zero}` | corresponding `roundTripLedger.dimensions.<camelD>.{before,after,delta,available,knownZero}` |
| `round_trip_ledger.terminal.before.{available,value}` | `roundTripLedger.terminal.before.available` and nullable projection of `.value` |
| `round_trip_ledger.terminal.after.{available,value}` | `roundTripLedger.terminal.after.available` and nullable projection of `.value` |
| `terminal.{available,value}` | `terminal.available` and nullable projection of `terminal.value` |
| `transport.bytes.{value,available,known_zero}` | `transport.bytes.{value,available,knownZero}` |
| `transport.messages.{value,available,known_zero}` | `transport.messages.{value,available,knownZero}` |
| `transport.request_bytes.{value,available,known_zero}` | `transport.requestBytes.{value,available,knownZero}` |
| `transport.request_messages.{value,available,known_zero}` | `transport.requestMessages.{value,available,knownZero}` |

The raw/semantic corrections are mandatory:

- malformed emitted response bytes are retained before parse normalization;
- control/read-error branches fabricate no response frame;
- no-response-frame events do not fabricate JSON-RPC error code/message/data/serialization;
- commitment metadata never selects refusal, result, error, terminal, lifecycle, external, or ledger policy;
- unavailable, available-empty, and available-known-zero remain distinct;
- the exact 17 removed/deferred request-body, request-payload, response-body, response-result, and `result.form` paths remain absent.

`finishRoundTrip` is the sole admitted terminal/error/result/accounting/ledger finalizer. It maps success, timeout, cancellation, crash, other failure, server error, and poison from current runtime branches only; it does not consult expected, C controls, world names, occurrence IDs, commitment content, or failure text.

## 9. B-only and C-only exclusions

The following remain qualification-owned and have production seam `NONE`:

- ordinary B20 lifecycle/control/external/identity assertions;
- the W24 root 35-field aggregation over seven dimensions and sixteen independently executed worker actual A ledgers;
- W25 disabled/enabled external and transport equality;
- all nine C fixture/control/organization paths.

W24 aggregation is **B-only**. No production type, field, owner, view, snapshot, helper, or method may store, project, request, or compute a root aggregate. The same ledger path names remain ordinary A fields for each worker and other ordinary occurrence.

W25 equality is **RED-only**. Disabled and enabled members execute independently. Equality is actual-to-actual, preserves unavailable/available-empty and nil/empty distinctions, and never borrows a sibling actual. It authorizes no equality field or comparison helper in production.

## 10. Immutable owner/view/lease behavior

The generic owner receives exactly one finalized observation and deep-copies all mutable slices/bytes. Views and callback views are fresh copies. Predecessor metadata is copied from caller to holder, holder to observation, and observation to each exposure without aliasing. Pointer identity has no metadata-equality meaning. Ordinary `RoundTrip` return leaves the owner open/acquirable and performs no terminal request or wait.

Acquisition is exactly:

```go
func (result RoundTripResult) privateCallHierarchyLayer1GenericLease() (any, bool)
```

The dynamic value is `callHierarchyLayer1GenericLease`. Disabled/no-owner returns `(nil, false)`. Closed/released owner returns `(nil, false)`. No zero lease is reported as available.

Ordinary `RoundTrip` ownership remains open until either an explicit `callHierarchyLayer1GenericLease.Release` or actual Manager/session teardown. Explicit `Release` and teardown `requestTerminal` both close admission, drain already admitted callbacks, and invoke the same `releaseLocked` transition; exactly one racer wins that helper, clears retained observation custody, and marks released, while every repeated or losing explicit release returns false. Teardown does not depend on an explicit actor: `requestTerminal` performs forced release itself, then `waitReleased` only observes the guaranteed transition. Callback panic is contained as the exact error in §5.3, callback errors return unchanged, and the active-count defer decrements exactly once and broadcasts at zero. Abandoned leases/views are invalid after teardown; later acquisition, view, and callback fail closed.

## 11. Exact closed C15 expected candidate set — unclassified

Canonical C15 MUST enumerate exactly the declarations and allocation sites below from the disposable patch. This is not a lower bound. Every classification remains `UNKNOWN`. Any extra or missing candidate is an immediate hard stop; no implementation, generator, or reviewer may silently normalize the delta.

### Field candidates

```text
callHierarchyLayer1RequestAttachment.predecessors *callHierarchyOpaqueCommitmentList
callHierarchyLayer1RequestAttachment.enabled bool
RoundTripRequest.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1Generic *callHierarchyLayer1GenericOwner
runtimeSession.callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner
```

### Type candidates

```text
callHierarchyOpaqueCommitmentList
callHierarchyLayer1GenericRefusalType and its exact twelve constants from §5.2
callHierarchyLayer1GenericAvailableUint64
callHierarchyLayer1GenericAvailableString
callHierarchyLayer1GenericLedgerDimension
callHierarchyLayer1GenericLedgerDimensions
callHierarchyLayer1GenericTerminalLedger
callHierarchyLayer1GenericRoundTripLedger
callHierarchyLayer1GenericCommitments
callHierarchyLayer1GenericFrame
callHierarchyLayer1GenericFrames
callHierarchyLayer1GenericError
callHierarchyLayer1GenericExternal
callHierarchyLayer1GenericResult
callHierarchyLayer1GenericTransport
callHierarchyLayer1GenericBeforeSnapshot
callHierarchyLayer1GenericAfterSnapshot
callHierarchyLayer1GenericObservation
callHierarchyLayer1GenericAttemptLineage
callHierarchyLayer1GenericOwner
callHierarchyLayer1GenericView
callHierarchyLayer1GenericLease
```

Every field declared in §4.2, §5.1, §5.2, and §6.2 is part of its containing type candidate; there are no anonymous or dynamically discovered field candidates. The exact carrier delta is the two named attempt fields above. `(*Manager).roundTrip` and `(*Manager).roundTripWithPrivate` have no carrier-caused declaration or body edit. No changed helper signature, explicit attempt parameter, `(*Manager).AdmitSeedBinding` call-site edit, `(*Manager).RoundTripPrivateB4` call-site edit, immutable direct-test call-site edit, or equivalent bridge declaration is a C15 candidate. Copying the request and performing the one transaction-result field initialization copy only the pointer value and introduce no allocation.

### Existing declarations expected to change

```text
(*Manager).privateCallHierarchyRequestIdentity
newCallHierarchyObservationLimits
callHierarchyObservationAccounting.validAgainst
callHierarchyObservationTerminal.valid
newCallHierarchyOriginalRequestFrameObservation
callHierarchyOriginalRequestFrameObservation.copyFrameBytes
newCallHierarchyOriginalResponseFrameObservation
callHierarchyOriginalResponseFrameObservation.copyFrameBytes
(*Manager).RoundTrip (body only; signature unchanged)
(*Manager).roundTripWithPrivateTransaction (body only; signature unchanged)
(*Manager).finishRoundTrip (body only; signature unchanged)
runtimeSession
RoundTripRequest
RoundTripResult
callHierarchyLayer1RequestAttachment
```

### New function/method candidates

```text
(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithPredecessors
(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithEnabled
newCallHierarchyLayer1GenericAttemptLineage
(*callHierarchyLayer1GenericAttemptLineage).recordRefusal
(*callHierarchyLayer1GenericAttemptLineage).finishRefusal
(*callHierarchyLayer1GenericAttemptLineage).admit
(*callHierarchyLayer1GenericAttemptLineage).captureRequestFrame
(*callHierarchyLayer1GenericAttemptLineage).captureResponseFrame
(*callHierarchyLayer1GenericAttemptLineage).captureResult
(*callHierarchyLayer1GenericAttemptLineage).finishAccepted
newCallHierarchyLayer1GenericOwner
publishCallHierarchyLayer1GenericOwner
(RoundTripResult).privateCallHierarchyLayer1GenericLease
(callHierarchyLayer1GenericLease).View
(callHierarchyLayer1GenericLease).Callback
(callHierarchyLayer1GenericLease).Release
(*callHierarchyLayer1GenericOwner).releaseLocked
(*callHierarchyLayer1GenericOwner).requestTerminal
(*callHierarchyLayer1GenericOwner).waitReleased
```

### Exact allocation/copy sites expected to enumerate

```text
privateCallHierarchyLayer1WithPredecessors: one &callHierarchyOpaqueCommitmentList{} allocation and one append([]callHierarchyOpaqueCommitmentMetadata(nil), values...) backing allocation only when len(values)>0
newCallHierarchyLayer1GenericAttemptLineage: one &callHierarchyLayer1GenericAttemptLineage{} allocation only when req.callHierarchyLayer1.enabled
captureRequestFrame: one append([]byte(nil), frame...) backing allocation when frame is available/nonempty
captureResponseFrame: one append([]byte(nil), raw...) backing allocation when raw is available/nonempty
captureResult: one append(json.RawMessage(nil), result...) backing allocation when result is available, including a non-nil zero-length slice for available-empty
finishRefusal: no allocation; it transfers the attempt's already detached predecessor slice into the by-value refusal observation and leaves every unavailable byte/slice member nil
finishAccepted: no allocation; it transfers the attempt's already detached predecessor/request-frame/response-frame/result custody and the current by-value snapshots into the observation
newCallHierarchyLayer1GenericOwner: one &callHierarchyLayer1GenericOwner{} allocation; one backing allocation for each non-nil mutable member among exactly observation.commitments.predecessors, observation.frames.auxiliary, each auxiliary[i].frameBytes, observation.frames.request.frameBytes, observation.frames.response.frameBytes, observation.error.data, observation.error.serializedBytes, observation.external.notificationsBytes, observation.external.responsesBytes, and observation.result.bytes
publishCallHierarchyLayer1GenericOwner: append of exactly one owner pointer to runtimeSession.callHierarchyLayer1GenericOwners; only the slice growth allocation induced by that append
callHierarchyLayer1GenericLease.View: exact deep-copy backing allocations for the same mutable observation members
callHierarchyLayer1GenericLease.Callback: exact deep-copy backing allocations for the same mutable observation members; errors.New allocates only on recovered panic
runLifecycle STOP registry detach: append([]*callHierarchyLayer1GenericOwner(nil), r.callHierarchyLayer1GenericOwners...) backing allocation, then nil the field
runLifecycle RESTART registry detach: append([]*callHierarchyLayer1GenericOwner(nil), r.callHierarchyLayer1GenericOwners...) backing allocation, then nil the field
Manager.Shutdown registry gather: append into one local []*callHierarchyLayer1GenericOwner from each session slice; only growth allocations of that one local slice
```

No allocation site exists in `releaseLocked`, `requestTerminal`, `waitReleased`, or `Release`. `releaseLocked` has only the exact observation-clear and release-bit effects specified in §5.3; `Release` and `requestTerminal` have only admission close, callback drain, and invocation of that helper, while `waitReleased` only observes/waits. No builder, holder, owner, view, lease, registry-entry, map, goroutine, channel, callback wrapper, or error allocation beyond the exact list above is permitted.

This is the exact expected C15 candidate set, not a C15 result. Canonical enumeration may attach generated identifiers, source locations, AST hashes, and `UNKNOWN` classifications to these exact candidates only. One extra or missing field, type, declaration, function, method, allocation/copy site, or candidate is:

```text
BLOCKED — canonical C15 candidate delta outside exact closed set: <exact extra-or-missing candidate>
```

No classification is assigned or implied by this document.

## 12. Forbidden semantics for every authorized symbol

No authorized symbol may:

- parse or classify call-hierarchy payload meaning;
- infer direction, target, occurrence, grouping, relation, method family, token form, semantic validity, feature identity, source completeness, authenticity, producer identity, qualification, publication, authority, completeness, or production use;
- retain selected-item, handoff, commitment, params, result, body, or payload preimage bytes beyond the explicitly authorized raw frame/result custody;
- use expected values, C controls, fixture/world/occurrence/pair names, registry identity, sibling actuals, or current/prototype output as runtime operands;
- synthesize response frames or detailed server errors;
- store B-only W24 aggregates or W25 equality;
- add public/exported API, persistence, replay, catalog, logging, telemetry, configuration, activation, CLI/MCP/schema behavior, diagnostics, retries, or parallel transaction/accounting/lifecycle machinery;
- repurpose existing `callHierarchyObservation` or `callHierarchyLayer1` result attachments; or
- mutate, suppress, rename, inline, relocate, or otherwise evade C15 candidates.

## 13. Immutable artifacts

During any later disposable experiment, all tests, testdata, corpora, fixtures, registries, manifests, digests, sidecars, generated artifacts, qualification documents, reviews, and historical C15 artifacts are immutable. V1, v2, and v3 artifacts are not rewrite targets. A failing immutable oracle is review evidence, not mutation authority.

This document is also immutable during the experiment. The prototype may write only the exact three production files after a separate gate names one disposable writer.

## 14. Mandatory experiment and custody sequence

The sequence is strict and non-circular:

1. **One disposable writer:** a separate review names exactly one disposable writer with exclusive custody of only the three files and exact symbols above in a newly created disposable worktree at the pinned HEAD.
2. **Qualified v3 GREEN:** that writer implements only enough of the finite boundary to make the immutable qualified current v3 RED compile and pass; no test/corpus/RED edit is permitted.
3. **Canonical C15 enumeration:** with the exact disposable patch still present, run the canonical enumerator and preserve the exact patch, implemented symbol set, baseline/generated candidate sets, locations, AST hashes, pre/post classifications, and complete `ADDED`, `REMOVED`, and `CHANGED` delta.
4. **Independent delta review:** a different reviewer accepts or rejects the finite symbol set, candidate set, complete delta, causal behavior, and hard-stop compliance. The prototype writer does not classify or bind the result.
5. **Dispose:** remove all prototype changes and delete the disposable worktree without commit; independently verify disposal.
6. **Later production gate:** only after accepted review and verified disposal may a separate gate freeze the final production file/symbol/candidate write-set and name exactly one production writer.

The disposable writer is not the production writer. The disposable patch has no presumptive reuse authority.

## 15. Hard stops

Stop immediately and report the exact blocker if any of the following is required:

- a fourth production file;
- any field, type, function, method, helper, alternate attachment, or carrier outside this finite list;
- any helper signature change or explicit attempt-pointer parameter on `roundTrip`, `roundTripWithPrivate`, `roundTripWithPrivateTransaction`, or `finishRoundTrip`;
- any edit to `(*Manager).AdmitSeedBinding`, `(*Manager).RoundTripPrivateB4`, their call sites, immutable direct helper-call tests, or the fourth production file `sessionruntime/b4_definition_private.go`;
- any `AdmitSeedBinding`, `RoundTripPrivateB4`, equivalent bridge interface, hidden/alternate attempt carrier, or second builder;
- failure to unconditionally overwrite the private request carrier at the first executable `RoundTrip` line, preserve exact pointer identity through ordinary by-value request transfer, initialize the transient result carrier exactly once after admission, or clear it in every `finishRoundTrip` return;
- any pre-admission return without refusal finalization;
- any post-admission return bypassing `finishRoundTrip`;
- any raw frame available only after parsing/normalization;
- any B/C value in production;
- any semantic requirement;
- any mutable test/corpus/RED/C15/document input;
- any inaccessible required input or ambiguous disposition; or
- any C15 delta outside the independently reviewed finite set.

Do not improvise, broaden an allowance, create a helper in another file, or weaken an immutable oracle.

## 16. Disposition

The exact three-file physical boundary remains the only candidate boundary. The finite proposed source-symbol and expected C15 candidate sets above are review inputs only. No disposable writer is named, no execution is authorized, no candidate is classified, and no production custody is issued.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL — REVIEW_REQUIRED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_WRITER — NOT NAMED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITESET_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITER — NOT NAMED
```

Disposition: **REVIEW_REQUIRED / NO GO**
