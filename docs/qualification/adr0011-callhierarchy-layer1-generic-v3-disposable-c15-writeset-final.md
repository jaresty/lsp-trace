# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Review Input

Status: **REVIEW_REQUIRED / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `b283d7ea6cf3d4da71cb0bed75517b9995d1922a`
- Required starting state: exact clean tracked worktree at the pinned HEAD; the refusal-owner-registry amendment is review input only
- Scope: documentation-only refresh of the finite disposable prototype write-set
- Source, test, corpus, RED, prototype, generated-artifact, execution, commit, activation, semantic, carrier-acceptance, production-writer, and production authority: **not issued**
- Mutation authorized by this task: this document only

## 1. Controlling inputs and supersession

This document refreshes the disposable C15 boundary from the following accepted current inputs:

1. the qualified current v3 RED and the reviewed predecessor disposable write-set;
2. the private attachment-enable carrier contract, limited to its still-current field/setter shape;
3. the accepted raw-response-custody correction;
4. the accepted semantic-residue correction; and
5. the accepted causal-lineage replacement-v3 and independent review; and
6. the qualified refusal-owner-registry amendment, which supersedes the prior session registry and its lifecycle collection rules.

The prior disposable effort stopped before mutation because the session-only registry could not retain enabled pre-admission refusal owners for absent or stale sessions. It produced no canonical C15 result. This integrated write-set is therefore new review input: no prior disposable authorization survives, and a new disposable writer authorization is required only after independent review accepts this exact revision.

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

The prototype, if separately reviewed and newly authorized, must satisfy this exact flow:

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
  -> one immutable owner publication under Manager lock
  -> retain in the Manager owner slice, or commit closed/retired refusal
  -> immediate outside-lock terminalize/join for refused publication
  -> (any, bool) lease acquisition
  -> copied View / copied Callback / Release / lifecycle drain
```

There is no package global, context value, side table, reconstructed builder, replacement builder, alternate owner, session registry, or second publication path. `Manager` has exactly the owner slice and exact-generation retired map specified in §6.2 and no other generic-owner field. The only attempt transfer is the qualified private `RoundTripRequest` field followed by the transient private `RoundTripResult` field specified in §6.2; neither field is serialized, exposed, retained after finalization, or usable as identity/equality evidence.

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
func (attempt *callHierarchyLayer1GenericAttemptLineage) admit(key lspwire.RequestKey, snapshot callHierarchyLayer1GenericBeforeSnapshot)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureRequestFrame(frame []byte, write RequestWriteObservation)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureResponseFrame(raw []byte, frame lspwire.ReadFrameObservation)
func (attempt *callHierarchyLayer1GenericAttemptLineage) captureResult(result json.RawMessage, serverError *lspwire.RPCError)
func (attempt *callHierarchyLayer1GenericAttemptLineage) finishAccepted(result RoundTripResult, failure session.Failure, terminal manageddiagnostic.Terminal, after callHierarchyLayer1GenericAfterSnapshot) callHierarchyLayer1GenericObservation
func newCallHierarchyLayer1GenericOwner(observation callHierarchyLayer1GenericObservation) *callHierarchyLayer1GenericOwner
func (m *Manager) finishCallHierarchyLayer1GenericRefusal(attempt *callHierarchyLayer1GenericAttemptLineage, result RoundTripResult, kind callHierarchyLayer1GenericRefusalType, reason session.Failure) RoundTripResult
func (m *Manager) publishCallHierarchyLayer1GenericOwnerLocked(result RoundTripResult, owner *callHierarchyLayer1GenericOwner) (RoundTripResult, *callHierarchyLayer1GenericOwner)
func (m *Manager) detachCallHierarchyLayer1GenericOwnersLocked(sessionID string, generation uint64, allGenerations bool) []*callHierarchyLayer1GenericOwner
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
- refusal finalizer: the Manager method owns `m.mu`; nil attempt returns the result unchanged without locking or allocation; enabled attempt records the exact typed refusal, freezes exactly one observation, constructs exactly one owner, and calls the locked publication helper in the same critical section while preserving public result/failure;
- admission: mutate the same pointer immediately after `pending.Begin`, recording the allocated key and before snapshot only;
- request capture: copy the exact emitted request frame from retained write custody, never params reserialization;
- response capture: copy `readResult.raw` before parsing-derived normalization; retain malformed emitted bytes; fabricate nothing for control/read-error/no-frame branches;
- result capture: preserve raw result and RPC-error availability after current result/error assignment and before any private clearing;
- accepted finalizer: normalize current terminal/error/result/accounting/ledger values only after current terminal mapping and under the exact snapshot custody described above;
- owner constructor/publication: deep-copy all mutable bytes/slices and initialize `cond.L = &owner.mu`; while holding `m.mu`, the locked helper assigns the owner to the result, clears the transient attempt, derives the exact generation key from immutable owner identity, and either appends that same pointer once to the Manager owner slice or, when `m.closed` or retired-key gated, returns it as the exact outside-lock terminal action;
- ordinary non-closed, non-retired `RoundTrip` finalization returns the registered owner open and acquirable; a closed/retired-gated publication MUST unlock and immediately call `requestTerminal` then `waitReleased` before returning the already released/fail-closed owner;
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

Every pre-admission return finalizes the request-carried attempt directly through `m.finishCallHierarchyLayer1GenericRefusal`; it never places the pointer in the transient result field and never calls `finishRoundTrip` for carrier transfer. Every admitted return reaches `finishRoundTrip`; it reads only `result.callHierarchyLayer1GenericAttempt`, performs the existing terminal/error/result/accounting/ledger finalization and at most one owner publication, assigns the final owner field, then unconditionally clears the transient field before returning. Disabled and direct legacy nil-attempt paths preserve current public behavior and return no owner. The owner field is not the attempt carrier and remains open/acquirable after successful publication. Neither carrier field has a JSON tag, public setter, exported alias, interface indirection, wrapper, serialization, or persistence semantics.

No field is added to `privateB4Reservation`, `privateB4Transaction`, `ownedTransport`, context, or another carrier. `Manager` receives exactly the two registry fields below. No `AdmitSeedBinding`, `RoundTripPrivateB4`, equivalent bridge interface, alternate private wrapper, or signature-level pointer parameter is proposed or implied.

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

The exact comparable key and only two proposed Manager fields are:

```go
type callHierarchyLayer1GenericGenerationKey struct {
    sessionID  string
    generation uint64
}

// in Manager; both guarded exclusively by m.mu
callHierarchyLayer1GenericOwners             []*callHierarchyLayer1GenericOwner
callHierarchyLayer1GenericRetiredGenerations map[callHierarchyLayer1GenericGenerationKey]struct{}
```

The key uses exact `string` session identity and `uint64` generation value equality. The retired set is initialized lazily or in the existing Manager constructor, contains only exact generations committed retired by completed STOP/RESTART, has no wildcard/session-only key, survives until Shutdown, and is bounded by one Manager lifetime. The Manager slice contains every retained refusal or admitted owner exactly once and contains no disabled, nil, duplicate, wrapper, session bucket, generation bucket, or secondary index. Selection uses immutable owner request identity, never current-session reconstruction. Every owner-slice or retired-map operation occurs under `m.mu`.

`publishCallHierarchyLayer1GenericOwnerLocked` requires `m.mu`: assign the owner to the result, clear the transient attempt, derive the exact key, and append once unless `m.closed` or that key is retired. A refused append returns the same owner as the second result. After unlocking, the caller immediately calls `requestTerminal()` then `waitReleased()` before returning; no owner callback, drain, terminal request, or wait runs under `m.mu`.

`detachCallHierarchyLayer1GenericOwnersLocked` requires `m.mu` and performs one exact stable pass. It must allocate `detached := make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))` at the single named site, compact retained pointers in place into `m.callHierarchyLayer1GenericOwners[:0]`, append matching pointers to `detached`, nil every vacated tail element, and then assign the compacted retained slice back to the Manager field. `allGenerations=true` detaches every immutable-owner identity for the session ID; otherwise it detaches only the exact session/generation. It preserves detached and retained order and performs no retained-slice backing allocation, alternate filtering allocation, owner locking, terminal action, waiting, goroutine creation, accounting mutation, or session lookup.

Exact invocation ownership and integration call sites are:

1. `(*Manager).finishCallHierarchyLayer1GenericRefusal`, called exactly once by each §7 pre-key branch and never while its caller holds `m.mu`, records/freezes/constructs/publishes atomically under `m.mu`, unlocks, and immediately executes any returned terminal action before returning the result.
2. `(*Manager).finishRoundTrip`, with unchanged signature, remains the sole admitted finalizer. After terminal mapping and after-snapshot normalization while holding `m.mu`, it freezes, constructs, and calls the locked publication helper; it completes existing locked accounting, unlocks at its existing handoff, immediately terminalizes/joins any refused publication, and only then returns.
3. Completed STOP inserts the exact current-generation retired key before detach, then detaches all generations for the session ID under `m.mu`; it unlocks, drains each detached owner in registry order, relocks, and only then deletes/completes the session. `SessionNotFound` STOP marks and detaches nothing; absent-session owners remain Manager-lifetime bounded.
4. Completed RESTART inserts the exact old-generation retired key before detach, detaches only that generation under `m.mu`, unlocks and drains in registry order, relocks, and only then starts the successor. The successor has no session registry.
5. `(*Manager).Shutdown`, after worker drain reaches zero and with `m.closed` durably true under `m.mu`, moves the complete Manager slice to a local, nils the field, and clears the retired set; it unlocks, drains every owner in order, relocks, and continues the existing shutdown sequence.
6. Failure branches before committed STOP/RESTART retirement do not mark, detach, or terminalize owners. After retirement/detach, owners are never reinserted even if later lifecycle work fails.

Publication, retirement marking, and detach serialize under `m.mu`: publication before a mark is selected by detach; matching publication after a mark or after closure is refused and terminalized outside the lock. No other function may call `requestTerminal` or `waitReleased`. No alternate transaction path, parser, serializer, retry, diagnostic, transport behavior, public result behavior, public error behavior, public accounting behavior, timing change, feature flag, CLI/MCP/schema behavior, or `internal/lspwire` mutation is permitted.

## 7. Complete pre-admission refusal closure

For every enabled pre-key return, the exact sequence is:

```text
attempt := req.callHierarchyLayer1GenericAttempt
result = m.finishCallHierarchyLayer1GenericRefusal(attempt, <current public result>, <exact enum>, <exact typed private reason>)
return result
```

At call sites currently holding `m.mu`, the branch unlocks before invoking the lock-owning finalizer and returns directly without relocking. No effect may enter that unlock/finalize boundary. The direct return never uses `RoundTripResult.callHierarchyLayer1GenericAttempt` and never calls `finishRoundTrip` for carrier transfer. No helper parameter, alternate bridge, reconstruction, failure-string parsing, or second publication helper is permitted. For disabled requests the overwritten nil request carrier preserves the current return and publishes no owner.

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

Any early return after key allocation is admitted and must funnel through `finishRoundTrip`; it must not use `finishCallHierarchyLayer1GenericRefusal`.

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

Ordinary non-closed, non-retired `RoundTrip` ownership remains open until either an explicit `callHierarchyLayer1GenericLease.Release` or matching Manager lifecycle teardown. Closed or exact-retired publication attaches the owner but returns only after immediate outside-lock terminalization/join, so acquisition fails closed. Explicit `Release` does not remove its pointer from the Manager registry; it clears observation custody through `releaseLocked`, and later lifecycle drain is idempotent. Explicit `Release` and teardown `requestTerminal` both close admission, drain already admitted callbacks, and invoke the same `releaseLocked` transition; exactly one racer wins that helper, clears retained observation custody, and marks released, while every repeated or losing explicit release returns false. Teardown does not depend on an explicit actor: `requestTerminal` performs forced release itself, then `waitReleased` only observes the guaranteed transition. Callback panic is contained as the exact error in §5.3, callback errors return unchanged, and the active-count defer decrements exactly once and broadcasts at zero. Abandoned leases/views are invalid after teardown; later acquisition, view, and callback fail closed.

## 11. Exact closed C15 expected candidate set — unclassified

Canonical C15 MUST enumerate exactly the declarations and allocation sites below from the disposable patch. This is not a lower bound. Every classification remains `UNKNOWN`. Any extra or missing candidate is an immediate hard stop; no implementation, generator, or reviewer may silently normalize the delta.

### Field candidates

```text
callHierarchyLayer1RequestAttachment.predecessors *callHierarchyOpaqueCommitmentList
callHierarchyLayer1RequestAttachment.enabled bool
RoundTripRequest.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1Generic *callHierarchyLayer1GenericOwner
Manager.callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner
Manager.callHierarchyLayer1GenericRetiredGenerations map[callHierarchyLayer1GenericGenerationKey]struct{}
```

### Type candidates

```text
callHierarchyOpaqueCommitmentList
callHierarchyLayer1GenericGenerationKey
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

Every field declared in §4.2, §5.1, §5.2, and §6.2 is part of its containing type candidate; there are no anonymous or dynamically discovered field candidates. The exact carrier delta is the two named attempt fields above. The generation key is a type candidate, not a field candidate or anonymous map-key spelling. `(*Manager).roundTrip` and `(*Manager).roundTripWithPrivate` have no carrier-caused declaration or body edit. No changed helper signature, explicit attempt parameter, `(*Manager).AdmitSeedBinding` call-site edit, `(*Manager).RoundTripPrivateB4` call-site edit, immutable direct-test call-site edit, or equivalent bridge declaration is a C15 candidate. Copying the request and performing the one transaction-result field initialization copy only the pointer value and introduce no allocation.

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
(*Manager).runLifecycle (body only; signature unchanged)
(*Manager).Shutdown (body only; signature unchanged)
Manager
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
(*Manager).finishCallHierarchyLayer1GenericRefusal
(*callHierarchyLayer1GenericAttemptLineage).admit
(*callHierarchyLayer1GenericAttemptLineage).captureRequestFrame
(*callHierarchyLayer1GenericAttemptLineage).captureResponseFrame
(*callHierarchyLayer1GenericAttemptLineage).captureResult
(*callHierarchyLayer1GenericAttemptLineage).finishAccepted
newCallHierarchyLayer1GenericOwner
(*Manager).publishCallHierarchyLayer1GenericOwnerLocked
(*Manager).detachCallHierarchyLayer1GenericOwnersLocked
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
finishCallHierarchyLayer1GenericRefusal: one owner construction for an enabled attempt, then at most one append-induced Manager-slice growth; no allocation for nil attempt; it transfers already detached refusal custody and leaves unavailable byte/slice members nil
finishAccepted: no allocation; it transfers the attempt's already detached predecessor/request-frame/response-frame/result custody and the current by-value snapshots into the observation
newCallHierarchyLayer1GenericOwner: one &callHierarchyLayer1GenericOwner{} allocation; one backing allocation for each non-nil mutable member among exactly observation.commitments.predecessors, observation.frames.auxiliary, each auxiliary[i].frameBytes, observation.frames.request.frameBytes, observation.frames.response.frameBytes, observation.error.data, observation.error.serializedBytes, observation.external.notificationsBytes, observation.external.responsesBytes, and observation.result.bytes
publishCallHierarchyLayer1GenericOwnerLocked: append of exactly one owner pointer to Manager.callHierarchyLayer1GenericOwners only when not closed/retired; only append-induced Manager-slice growth
detachCallHierarchyLayer1GenericOwnersLocked: exactly one unconditional detached-slice `make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))` allocation site; retained pointers compact in place with no retained-slice backing allocation, and every vacated tail slot is nilled
retired-generation set: exact-key map initialization and insert growth only
callHierarchyLayer1GenericLease.View: exact deep-copy backing allocations for the same mutable observation members
callHierarchyLayer1GenericLease.Callback: exact deep-copy backing allocations for the same mutable observation members; errors.New allocates only on recovered panic
runLifecycle STOP Manager detach: no additional owner-slice copy; retirement-key insert plus the single detached-slice allocation above
runLifecycle RESTART Manager detach: no additional owner-slice copy; retirement-key insert plus the single detached-slice allocation above
Manager.Shutdown registry detach: direct whole-slice transfer, nil assignment, and retired-set clear; no owner-slice copy allocation required
```

No allocation site exists in `releaseLocked`, `requestTerminal`, `waitReleased`, or `Release`. The exact retired-generation map is the sole admitted map addition; no session registry, registry-entry type, wrapper, second index, owner back-pointer, goroutine, or channel is admitted. `releaseLocked` has only the exact observation-clear and release-bit effects specified in §5.3; `Release` and `requestTerminal` have only admission close, callback drain, and invocation of that helper, while `waitReleased` only observes/waits. No builder, holder, owner, view, lease, registry-entry, map, goroutine, channel, callback wrapper, or error allocation beyond the exact list above is permitted.

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

1. **New review and authorization:** independently review this integrated Manager-owner-slice and exact-generation-retired-map revision; the prior pre-mutation disposable blocker produced no canonical C15 and grants no authority.
2. **One new disposable writer:** only after that review, a new authorization names exactly one disposable writer with exclusive custody of only the three files and exact symbols above in a newly created disposable worktree at the pinned HEAD.
3. **Qualified v3 GREEN:** that writer implements only enough of the finite boundary to make the immutable qualified current v3 RED compile and pass; no test/corpus/RED edit is permitted.
4. **New canonical C15 enumeration:** with the exact disposable patch still present, run the canonical enumerator and preserve the exact patch, implemented symbol set, baseline/generated candidate sets, locations, AST hashes, pre/post classifications, and complete `ADDED`, `REMOVED`, and `CHANGED` delta.
5. **Independent delta review:** a different reviewer accepts or rejects the finite symbol set, candidate set, complete delta, causal behavior, and hard-stop compliance. The prototype writer does not classify or bind the result.
6. **Dispose:** remove all prototype changes and delete the disposable worktree without commit; independently verify disposal.
7. **Later production gate:** only after accepted review and verified disposal may a separate gate freeze the final production file/symbol/candidate write-set and name exactly one production writer.

The disposable writer is not the production writer. The disposable patch has no presumptive reuse authority. The prior blocked disposable path and any noncanonical candidate list cannot be reused; this changed registry/lock/lifecycle set requires the new reviewed authorization and new canonical C15 run.

## 15. Hard stops

Stop immediately and report the exact blocker if any of the following is required:

- a fourth production file;
- any field, type, function, method, helper, alternate attachment, or carrier outside this finite list, including any session registry, registry-entry type, owner back-pointer, second index, or map other than the exact generation-key retired set;
- any helper signature change or explicit attempt-pointer parameter on `roundTrip`, `roundTripWithPrivate`, `roundTripWithPrivateTransaction`, or `finishRoundTrip`;
- any edit to `(*Manager).AdmitSeedBinding`, `(*Manager).RoundTripPrivateB4`, their call sites, immutable direct helper-call tests, or the fourth production file `sessionruntime/b4_definition_private.go`;
- any `AdmitSeedBinding`, `RoundTripPrivateB4`, equivalent bridge interface, hidden/alternate attempt carrier, or second builder;
- failure to unconditionally overwrite the private request carrier at the first executable `RoundTrip` line, preserve exact pointer identity through ordinary by-value request transfer, initialize the transient result carrier exactly once after admission, or clear it in every `finishRoundTrip` return;
- any of the exact twelve pre-admission returns without exactly one call to the Manager refusal finalizer, or any additional pre-key return;
- any owner append while `m.closed` or its exact generation key is retired, or return before the required outside-lock terminalize/join completes;
- any STOP/RESTART detach before exact retirement marking, or any owner callback/drain/terminal/wait while holding `m.mu`;
- any post-admission return bypassing `finishRoundTrip`;
- any raw frame available only after parsing/normalization;
- any B/C value in production;
- any semantic requirement;
- any mutable test/corpus/RED/C15/document input;
- any inaccessible required input or ambiguous disposition; or
- any C15 delta outside the independently reviewed finite set.

Do not improvise, broaden an allowance, create a helper in another file, or weaken an immutable oracle.

## 16. Disposition

The exact three-file physical boundary remains the only candidate boundary. The finite proposed source-symbol and expected C15 candidate sets above are review inputs only. No disposable writer is named, no execution is authorized, no candidate is classified, and no production custody is issued. The prior disposable path was blocked before mutation and produced no canonical C15; independent review of this integrated revision and a new explicit disposable authorization are required before any experiment.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL — REVIEW_REQUIRED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_WRITER — NOT NAMED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITESET_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITER — NOT NAMED
```

Disposition: **REVIEW_REQUIRED / NO GO**
