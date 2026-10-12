# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Attempt-Carrier Compatibility Amendment

Status: **REVIEW_REQUIRED / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `83e43a14f3d0ec4f3f02f14eb8ff9930ded98fde`
- Amends: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Responds to: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final-review-v2.md`
- Scope: documentation-only replacement of replacement-v3's explicit helper-parameter transfer
- Tests, source, prototype, corpus, generated artifacts, execution, commit, activation, classification, carrier acceptance, production writer, and production authority: **not issued**
- Mutation authorized by this task: this document only

## 1. Narrow supersession

This amendment supersedes only replacement-v3's requirement to add an explicit `attempt *callHierarchyLayer1GenericAttemptLineage` parameter to `roundTrip`, `roundTripWithPrivate`, `roundTripWithPrivateTransaction`, and `finishRoundTrip`, and the associated requirement to edit every caller for parameter threading.

All other requirements of the amended write-set remain unchanged, including the 85-occurrence mapping, finite refusal table, exact snapshots and raw-frame custody, A116-only observation/view representation, owner/lease lifecycle, teardown behavior, immutable artifacts, exact three-production-file boundary, and `UNKNOWN`-only C15 posture.

This amendment is a candidate write-set correction for review. It does not accept the carrier, classify any candidate, authorize a disposable prototype, or authorize production mutation.

## 2. Exact compile-compatible carrier

### 2.1 Exact request and transient result fields

Add exactly this private field to the existing `RoundTripRequest` declaration in `sessionruntime/sessionruntime.go`:

```go
callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
```

Add exactly the same private transient field to the existing `RoundTripResult` declaration in `sessionruntime/sessionruntime.go`:

```go
callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
```

The transient result field is distinct from the final `callHierarchyLayer1Generic` owner/publication field. No JSON tag, public setter, exported alias, interface indirection, wrapper, or other carrier is admitted.

These fields are transport-local package-private custody. They are not request or response content, wire data, persistent state, replay state, telemetry, configuration, or authority. Ordinary by-value request and result passing preserves the exact pointer identity without changing any method signature.

### 2.2 Public `RoundTrip` entry and anti-forgery overwrite

At the first executable line of `(*Manager).RoundTrip`, overwrite the field unconditionally:

```go
req.callHierarchyLayer1GenericAttempt = newCallHierarchyLayer1GenericAttemptLineage(req)
```

The constructor remains default-off: it returns nil when the explicit Layer-1 generic attachment is disabled and returns one newly allocated attempt pointer only when enabled.

The assignment occurs before the existing private C16/C17 accounting refusal check and before every other refusal or route selection. Therefore:

- disabled entry overwrites the field with nil;
- enabled entry overwrites the field with the one builder-created pointer;
- any caller-supplied value, including package-internal input, is ignored and cannot inject, substitute, pre-populate, or reuse an attempt;
- the builder still reads only the already-authorized request identity and explicit attachment inputs and never reads expected values, world/occurrence state, commitment content, failure text, or the incoming carrier field.

The accounting refusal branch must use that overwritten field to record the already-selected finite refusal kind and typed failure, then return through `finishRefusal`. Nil remains a no-op and preserves the existing public result. The refusal path does not allocate a key and does not call `admit`.

Every other listed pre-key refusal behaves identically: read the exact request-carried pointer, record the exact finite refusal kind and existing typed failure, and finalize once through `finishRefusal`. No refusal is inferred from strings or semantic content.

### 2.3 Unchanged by-value transfer

The existing signatures remain byte-for-byte unchanged:

```go
func (m *Manager) RoundTrip(parent context.Context, req RoundTripRequest) RoundTripResult
func (m *Manager) roundTrip(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool) RoundTripResult
func (m *Manager) roundTripWithPrivate(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, private *privateB4Reservation) RoundTripResult
func (m *Manager) roundTripWithPrivateTransaction(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool, transaction *privateB4Transaction) RoundTripResult
func (m *Manager) finishRoundTrip(id string, owner *ownedTransport, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult
```

`RoundTrip` passes `req` by value through the existing `roundTrip` / `roundTripWithPrivate` / `roundTripWithPrivateTransaction` chain. Each copy contains the same pointer value; no stage rebuilds, clears, replaces, compares, indexes, registers, or derives another attempt.

Immediately after the existing key allocation:

```go
key := r.pending.Begin(req.Generation)
```

and before request registration, identity capture, protocol ownership, worker allocation, or `m.mu.Unlock()`, `roundTripWithPrivateTransaction` reads exactly:

```go
attempt := req.callHierarchyLayer1GenericAttempt
```

and invokes `attempt.admit(key, beforeSnapshot)` under the unchanged nil-safe contract. The transaction's one `RoundTripResult` initialization copies that exact pointer exactly once into:

```go
callHierarchyLayer1GenericAttempt: req.callHierarchyLayer1GenericAttempt,
```

All later request-frame, response-frame, result/error, refusal, and terminal mutations consume the pointer retained in that transient result field. Every existing `finishRoundTrip` call already carries the `RoundTripResult` unchanged; no helper signature, call site, context, side table, or replacement carrier is required.

On every return branch, `finishRoundTrip` reads `result.callHierarchyLayer1GenericAttempt`, performs the existing normalized finalization and publication on that exact pointer, sets the final `result.callHierarchyLayer1Generic` owner field from that finalization, clears `result.callHierarchyLayer1GenericAttempt` to nil, and only then returns the result. Nil remains disabled and finalizes to no owner/publication. No branch may return a result retaining the transient pointer.

Pre-admission refusal branches that return directly before the transaction result initialization continue to finalize the request-carried pointer directly through `finishRefusal`; they never place it in a `RoundTripResult` transient field and never call `finishRoundTrip` for carrier transfer.

There is exactly one builder call, one enabled allocation, one pointer identity, one request carrier field, and one finite transient result carrier field.

## 3. Compatibility and exact non-edits

### 3.1 Legacy internal callers

The private field is inaccessible outside package `sessionruntime` and omitted values are nil. Existing package-internal callers that invoke `roundTrip`, `roundTripWithPrivate`, or `roundTripWithPrivateTransaction` directly continue to compile unchanged. Their request values carry nil unless package code explicitly supplied a value; under this amendment direct legacy routes are not public `RoundTrip` entries and nil retains the prior behavior of no Layer-1 generic observation.

No compatibility overload, variadic parameter, alternate helper, wrapper method, or test-only seam is required or permitted.

### 3.2 Exact declarations requiring no edit

The following declarations require no edit:

- `(*Manager).AdmitSeedBinding`; its existing call to `m.roundTrip(...)` remains compile-compatible and its ordinary request copy carries nil, so it produces no generic observation.
- `(*Manager).RoundTripPrivateB4` in `sessionruntime/b4_definition_private.go`; its existing call to `m.roundTripWithPrivateTransaction(...)` remains compile-compatible and its ordinary request copy carries nil, so it produces no generic observation.
- every immutable direct test call to `roundTrip`, `roundTripWithPrivate`, or `roundTripWithPrivateTransaction`; all existing calls compile without modification.

`RoundTripPrivateB4` remains in its current fourth physical file but is not edited. Its existence does not expand the three-file mutation boundary.

### 3.3 Exact method-signature rule

No method signature changes. In particular, no attempt parameter is added to:

- `roundTrip`;
- `roundTripWithPrivate`;
- `roundTripWithPrivateTransaction`; or
- `finishRoundTrip`.

Any such signature change is outside this amendment and is an immediate hard stop.

## 4. Refusal and accepted behavior

### 4.1 Refused before key allocation

For an enabled request refused before `pending.Begin`:

1. the first-line builder creates the sole attempt and stores it in the overwritten request field;
2. the refusal site reads that same field;
3. `recordRefusal` records only the exact finite refusal enum and already-selected typed failure;
4. `finishRefusal` freezes and publishes exactly one refusal observation while preserving the existing public result and failure;
5. the direct return never uses the transient `RoundTripResult.callHierarchyLayer1GenericAttempt` field; and
6. no key, admission snapshot, request frame, response frame, result bytes, or fabricated availability is recorded.

For a disabled request, the overwritten field is nil and the public refusal behavior is unchanged with no generic observation.

The initial accounting refusal is covered by this same rule; it is not an exception and cannot return before the overwrite/finalization opportunity.

### 4.2 Accepted after key allocation

For an enabled request that reaches key allocation:

1. all by-value request copies carry the sole builder-created pointer;
2. `roundTripWithPrivateTransaction` reads that exact pointer after `pending.Begin`;
3. `admit` records the allocated key and exact before snapshot on that pointer;
4. the transaction result initialization copies that pointer exactly once into the transient result field;
5. every existing `finishRoundTrip` call carries that result unchanged;
6. every `finishRoundTrip` branch reads the transient pointer, finalizes normalized owner/publication, sets the final `callHierarchyLayer1Generic` owner field, clears the transient pointer, and then returns; and
7. the existing accepted finalizer publishes exactly one owner under the unchanged lifecycle contract.

For a disabled request or a direct legacy internal route carrying nil, `admit` and all later attempt operations are nil-safe no-ops and no generic observation is published.

## 5. Anti-forgery and forbidden alternatives

The public `RoundTrip` overwrite is the anti-forgery boundary. The carrier does not trust caller-supplied pointer custody, even from same-package construction: the entry always replaces it with nil or a fresh builder result selected solely by the authorized enabled bit.

The following remain forbidden:

- globals, Manager-global attempt state, package registries, maps, side tables, context values, goroutine-local state, or key-indexed lookup;
- any second builder, alternate owner, alternate publication path, pointer reconstruction, or result initialization/copy beyond the one exact transient-field initialization;
- expected values, expected observations, world/occurrence data, corpus knowledge, commitment-content interpretation, or failure-string parsing;
- pointer-equality metadata or semantic meaning derived from pointer identity;
- public/exported carrier access, wire serialization, persistence, replay, logging, telemetry, or diagnostics;
- caller-supplied field values surviving the public `RoundTrip` entry;
- edits to immutable tests, `AdmitSeedBinding`, `RoundTripPrivateB4`, or a fourth production file; and
- any method-signature change.

## 6. Exact lineage implications

This carrier correction changes transfer mechanics only. The lineage implications are exact:

- **Origin:** the public `RoundTrip` first-line builder is the sole enabled origin.
- **Disabled identity:** nil is the sole disabled representation.
- **Request transfer:** ordinary Go struct copying transfers one pointer value through the unchanged request parameter.
- **Admission:** the transaction reads the request field only after key allocation and admits that same pointer.
- **Result transfer:** the transaction result initialization copies that pointer exactly once into the transient result field, and every existing `finishRoundTrip` call carries the result unchanged.
- **Refusal:** pre-admission direct-return sites finalize the request-carried pointer without admission and never use the result transient.
- **Accepted completion:** every `finishRoundTrip` branch reads the transient pointer, finalizes normalized owner/publication, assigns the final `callHierarchyLayer1Generic` owner field, clears the transient pointer, and returns no transient custody.
- **Forgery resistance:** public entry overwrites all incoming values before refusal or dispatch.
- **Legacy compatibility:** direct internal callers may carry nil and retain no generic observation.
- **Non-semantic identity:** pointer sameness proves only custody continuity inside this bounded attempt; it does not classify feature identity, expected/world agreement, occurrence identity, or authority.

The replacement-v3 occurrence mapping and all observation semantics remain unchanged. This amendment neither adds nor removes an enabled occurrence; it only supplies a compile-compatible custody path for the already-proposed attempt.

## 7. Exact C15 amendment — unclassified

The exact closed C15 candidate proposal is amended by exactly two field candidates:

```text
RoundTripRequest.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
```

`RoundTripRequest` and `RoundTripResult` are correspondingly added to the existing declarations expected to change. The transient `RoundTripResult` field is separate from the already-proposed final `RoundTripResult.callHierarchyLayer1Generic` owner field. `(*Manager).RoundTrip` remains in the set because it performs the first-line overwrite and direct pre-admission refusal handling; `(*Manager).roundTripWithPrivateTransaction` performs the sole transient result initialization; and `(*Manager).finishRoundTrip` consumes and clears that transient field on every return branch.

The following are removed from the proposed `CHANGED` implications caused solely by explicit parameter threading:

```text
no signature change: (*Manager).roundTrip
no signature change: (*Manager).roundTripWithPrivate
no signature change: (*Manager).roundTripWithPrivateTransaction
no signature change: (*Manager).finishRoundTrip
no call-site change: (*Manager).AdmitSeedBinding
no call-site change: (*Manager).RoundTripPrivateB4
```

The helper bodies may still contain already-proposed lineage capture/finalization mutations where named by the controlling write-set; this amendment removes only signature and call-site changes. No new function, method, type, allocation, side table, or fourth-file candidate is introduced. The constructor's one conditional attempt allocation remains the sole carrier allocation; copying `RoundTripRequest` and the one exact transaction-result field initialization copy only the pointer value and create no pointed-to object.

Every C15 classification remains `UNKNOWN`. This is an exact candidate proposal, not a C15 result. Any extra carrier field, helper, signature delta, call-site edit, allocation, or physical production file is:

```text
BLOCKED — attempt-carrier compatibility delta outside exact amendment: <exact extra candidate or edit>
```

## 8. Exact physical mutation boundary

The controlling three-file boundary remains unchanged:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The new request field, new transient result field, public-entry overwrite/direct-refusal handling, unchanged by-value request transfer, post-key field read, sole transaction-result initialization, and `finishRoundTrip` consume/set/clear behavior all fit in `sessionruntime/sessionruntime.go`. The remaining owner/lease and lineage work stays in the other two already-named files.

No edit is required in `sessionruntime/b4_definition_private.go` or any test file. Requiring one is a hard stop and requires a new write-set amendment and independent review.

## 9. Review disposition

This amendment resolves the compile-compatibility contradiction identified by the final independent review V2 at the proposal level: immutable direct callers retain their existing arity, and the two legacy production callers require no edit. It does not itself establish that the complete amended write-set is accepted, that canonical C15 matches, or that any prototype behavior is qualified.

```text
REVIEW_REQUIRED — exact private request and transient result fields replace explicit helper-parameter threading; public RoundTrip overwrites caller input, the transaction copies the pointer once into its result, every finishRoundTrip branch finalizes/sets/clears it, direct pre-admission refusals finalize without the result transient, and legacy direct callers remain compile-compatible
```

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_ATTEMPT_CARRIER_COMPATIBILITY_AMENDMENT_REVIEW_REQUIRED
```

Disposition: **REVIEW_REQUIRED / NO GO / NO AUTHORITY**
