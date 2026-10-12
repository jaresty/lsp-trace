# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Three-World Causal-Outcome Amendment

Status: **REVIEW_REQUIRED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned starting HEAD: `382e06017c03bca5b145ebd6135c3eaafcbc1414`
- Required starting state: exact clean worktree at the pinned HEAD
- Exact scope: `world-04` / request key `104`, `world-05` / request key `105`, and `world-23` / request key `123`
- Mutation authority: this document only
- Corpus, RED, source, tests, prototype, generated artifact, execution, C15 enumeration, writer, commit, activation, carrier acceptance, and production authority: **not issued**

## 1. Decision

The three scoped occurrences require an independently authored causal-outcome driver input. It is not reconstructed from historical expected values or actual execution output.

The exact outcomes are:

| Occurrence | Request key | Exact causal outcome |
|---|---:|---|
| `world-04` | `104` | opaque typed accepted-terminal classification `successful-empty` |
| `world-05` | `105` | opaque typed accepted-terminal classification `successful-empty` |
| `world-23` | `123` | typed pre-effect refusal class `method-unavailable` and typed reason `method unavailable` |

For `world-04` and `world-05`, production consumes `successful-empty` verbatim only after transport has completed. It does not parse, decode, inspect, classify, or otherwise derive that classification from result bytes, response bytes, frame bytes, RPC error, method, request identity, world identity, or expected values.

For `world-23`, production consumes the explicit typed refusal directly before writer, transport, pending-key, admission, reservation, worker, lease, callback, frame, result, diagnostic, terminal, or accounting effects. The refusal branch does not parse a result and does not wait for, synthesize, or retain response/result bytes.

This is a causal-input correction, not semantic interpretation. The literals do not establish call-hierarchy meaning, relation identity, direction, result validity, source completeness, qualification, publication authority, or production use.

## 2. Exact independently authored driver schema

Every future v3 envelope remains self-contained. The only proposed schema addition is the optional member:

```text
driver.request.causal_outcome
```

Its value is exactly one of these two closed object shapes:

```json
{"class":"accepted-terminal","terminal":"successful-empty"}
```

```json
{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}
```

The member is authored independently of `expected`, runtime output, result bytes, response bytes, method, request ID, occurrence ID, and world identity. The future authoring ledger must identify the independently controlled source of each of the three values; the runtime envelope contains the complete value and requires no registry lookup or side table.

### 2.1 Strict decode

The future driver decoder is test-only and must:

1. use duplicate-aware JSON token processing for the complete envelope and every nested object;
2. reject duplicate `causal_outcome`, `class`, `terminal`, `refusal_class`, or `refusal_reason` members;
3. reject every unknown member at every causal-outcome object depth;
4. reject `null`, arrays, scalars, non-string members, empty strings, alternate spelling, alternate case, surrounding whitespace, Unicode lookalikes, and additional members;
5. accept `class == "accepted-terminal"` only with exactly `terminal == "successful-empty"` and with both refusal members absent;
6. accept `class == "pre-effect-refusal"` only with exactly `refusal_class == "method-unavailable"` and `refusal_reason == "method unavailable"`, with `terminal` absent; and
7. produce typed private values before any production call; production contains no JSON decoder, literal parser, normalizer, or validator for this driver input.

No open enum, fallback string, default terminal, default refusal class, default refusal reason, or unrecognized-value preservation is permitted.

### 2.2 Absence/default policy

`driver.request.causal_outcome` is required and non-null for exactly the three scoped occurrences. It is absent for exactly the other 82 accepted v3 occurrences.

Absence on those 82 occurrences maps to the private zero value `callHierarchyLayer1GenericCausalOutcome{}` and preserves the already qualified execution/finalization path unchanged. Absence on any of the scoped three is invalid. Presence on any of the other 82 is invalid. No absent value is inferred from or repaired using expected terminal/refusal fields, method, result, request key, occurrence ID, or world identity.

The authoring/rematerialization validator, not production, enforces the exact three-member presence set. Runtime neither knows nor dispatches on that set.

## 3. Exact finite private production carrier

The physical production boundary remains exactly:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

No fourth production file is admitted.

### 3.1 Types and fields

Add exactly these private comparable declarations in `sessionruntime/b4_owned_method_pair_lease.go`:

```go
type callHierarchyLayer1GenericCausalOutcomeClass uint8

const (
	callHierarchyLayer1GenericCausalOutcomeUnavailable callHierarchyLayer1GenericCausalOutcomeClass = iota
	callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal
	callHierarchyLayer1GenericCausalOutcomePreEffectRefusal
)

type callHierarchyLayer1GenericCausalOutcome struct {
	class         callHierarchyLayer1GenericCausalOutcomeClass
	terminal      manageddiagnostic.Terminal
	refusalType   callHierarchyLayer1GenericRefusalType
	refusalReason session.Failure
}
```

Extend the existing refusal enum with exactly:

```go
callHierarchyLayer1GenericRefusalMethodUnavailable
```

Add exactly one field to the comparable request attachment:

```go
causalOutcome callHierarchyLayer1GenericCausalOutcome
```

Add exactly one field of the same type to `callHierarchyLayer1GenericAttemptLineage`:

```go
causalOutcome callHierarchyLayer1GenericCausalOutcome
```

No causal-outcome field is added to `RoundTripRequest`, `RoundTripResult`, `Manager`, owner, observation, view, lease, snapshot, transaction, reservation, transport, context, registry entry, or public type. Existing request/result attempt pointers remain the sole cross-wrapper carrier. The new struct is comparable and contains no pointer, slice, map, interface, function, byte custody, result custody, or allocation.

Exact zero defaults are:

```text
class         = callHierarchyLayer1GenericCausalOutcomeUnavailable
terminal      = manageddiagnostic.Terminal("")
refusalType   = callHierarchyLayer1GenericRefusalUnavailable
refusalReason = session.Failure("")
```

That zero value is the only value carried by the other 82 occurrences and has no behavioral effect.

The two nonzero values supplied by strict test-only decode are exactly:

```go
callHierarchyLayer1GenericCausalOutcome{
	class:    callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal,
	terminal: manageddiagnostic.Terminal("successful-empty"),
}
```

```go
callHierarchyLayer1GenericCausalOutcome{
	class:         callHierarchyLayer1GenericCausalOutcomePreEffectRefusal,
	refusalType:   callHierarchyLayer1GenericRefusalMethodUnavailable,
	refusalReason: session.Failure("method unavailable"),
}
```

No production symbol constructs either nonzero literal from strings. The RED supplies the already typed value.

### 3.2 Setter and exact invocation order

Add exactly this value-returning private setter in `sessionruntime/owned_method_pair.go`:

```go
func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithCausalOutcome(
	outcome callHierarchyLayer1GenericCausalOutcome,
) callHierarchyLayer1RequestAttachment
```

It replaces only `causalOutcome`. It preserves `selected`, `handoff`, `ledger`, `predecessors` including exact pointer identity, `enabled`, `profile`, `binding`, `workspace`, and every other attachment field byte-for-byte. It performs no allocation and no validation, parsing, derivation, or dispatch.

The first executable RED attachment invocation sequence becomes exactly:

```text
privateCallHierarchyLayer1WithIdentity(profile, binding, workspace)
-> privateCallHierarchyLayer1WithEnabled(enabled)
-> privateCallHierarchyLayer1WithCausalOutcome(causalOutcome)
-> privateCallHierarchyLayer1WithPredecessors(predecessors)
```

Thus the new setter is inserted strictly after `identity -> enabled` and strictly before `predecessors`. Reordering, omission, duplicate invocation, a second setter, or direct field mutation is invalid.

## 4. Constructor, refusal, admission, and finalizer consumption

### 4.1 Constructor

`newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest) *callHierarchyLayer1GenericAttemptLineage` retains its exact signature, default-off allocation rule, and single-attempt identity.

When enabled, it copies `req.callHierarchyLayer1.causalOutcome` verbatim into `attempt.causalOutcome` together with the already qualified identity and attachment values. When disabled, it returns nil and consumes nothing. It does not validate or reinterpret the outcome and does not read method, expected values, world/occurrence/request identifiers, result, response, or transport state to create or modify it.

The first executable line of `(*Manager).RoundTrip` continues to overwrite caller-supplied attempt custody with the sole constructor result. Existing by-value request flow carries that pointer unchanged through unchanged helper signatures.

### 4.2 Typed pre-effect refusal

Immediately after attempt construction and before the currently first writer/transport/pending/admission effect, `(*Manager).RoundTrip` examines only `attempt.causalOutcome.class`:

- nil attempt or `Unavailable`: continue the existing path unchanged;
- `PreEffectRefusal`: require the already typed carrier to contain exactly `callHierarchyLayer1GenericRefusalMethodUnavailable`, `session.Failure("method unavailable")`, and zero terminal, then call the existing Manager-owned refusal finalizer directly and return;
- `AcceptedTerminal`: continue toward admission/transport without changing request, method, writer, transport, pending, result, or expected data.

The exact W23 branch is therefore:

```text
constructor
-> typed causal-outcome class read
-> finishCallHierarchyLayer1GenericRefusal(
     attempt,
     current public result,
     callHierarchyLayer1GenericRefusalMethodUnavailable,
     session.Failure("method unavailable"),
   )
-> return
```

It occurs before `pending.Begin`, request registration, writer invocation, transport read/write, worker/reservation/lease/callback activity, frame capture, result capture, or admitted finalization. It performs no result parse. Public behavior must be specified by the later RED/corpus review from independently authored driver requirements; this document does not authorize source behavior or invent a public failure mapping.

The existing refusal finalizer remains the sole pre-admission finalizer. No second finalizer, parser, method switch, world switch, request-key switch, or result-dependent refusal helper is added.

### 4.3 Admission and accepted finalization

For W04 and W05, admission remains at the exact existing boundary immediately after `key := r.pending.Begin(req.Generation)`. The same attempt pointer is copied once into the transient result carrier and follows the existing request/response/result capture path.

`captureResult` may continue to retain raw result custody, but it must not inspect or translate result bytes to choose the terminal classification. After actual transport completion and existing current terminal/error mapping, `finishAccepted` consumes `attempt.causalOutcome.terminal` verbatim when and only when `class == AcceptedTerminal`. For both W04 and W05 that typed value is exactly `manageddiagnostic.Terminal("successful-empty")`.

The accepted-terminal carrier cannot cause admission, transport success, a response, a result, or an owner to exist. It only supplies the scoped terminal classification after the actual transport path has reached accepted finalization. If the attempt does not reach that finalizer, the carrier cannot manufacture success or bypass the actual failure path. The future RED must freeze the exact precedence for an unexpected transport failure; this documentation amendment does not silently convert failure into success.

For the other 82 occurrences, the zero carrier preserves the existing finalizer mapping byte-for-byte.

## 5. Anti-dispatch and mutation guards

The future corpus/RED must prove all of the following before source authority can be considered:

1. Renaming any scoped world or occurrence while retaining the same typed causal outcome does not change runtime behavior.
2. Changing request keys `104`, `105`, or `123` while retaining the same typed causal outcome does not change runtime behavior.
3. Swapping incoming/outgoing methods or binding labels while retaining the same typed causal outcome does not change runtime behavior.
4. Mutating expected terminal, refusal, result, error, frame, control, or external values does not change the runtime carrier or branch.
5. Mutating result bytes for W04/W05 does not change their accepted terminal classification; no result decoder/parser is invoked for that classification.
6. Supplying the W23 typed pre-effect refusal prevents writer, transport, pending-key, reservation, worker, lease, callback, frame, result, admitted snapshot, and admitted-finalizer effects.
7. Supplying `Unavailable` to any scoped occurrence is rejected by strict driver validation; production does not repair it.
8. Supplying a nonzero causal outcome to any of the other 82 occurrences is rejected by strict driver validation; production has no ID/world exception.
9. Duplicate or unknown driver members reject before execution.
10. No expected value, historical corpus value, method, ID, world, request key, result, response, or error can construct, select, overwrite, or normalize the carrier.
11. The setter changes only `causalOutcome`; constructor consumption changes no other attachment field; finalizer consumption changes no raw custody.
12. The W23 refusal uses the typed enum and typed `session.Failure` directly; failure-string parsing is forbidden.

No ID-, world-, method-, expected-, result-, response-, error-, or terminal-derived dispatch is permitted anywhere in production or RED execution construction.

## 6. A-path effects and accounting remain exact

This amendment adds no expected path, removes no expected path, and changes no ownership category. The accepted schema/path accounting remains:

```text
A production generic owner/view fields                     116
B RED-derived assertions from actual executions             20
C fixture/control-only assertions and metadata                9
                                                          -----
retained expected paths                                     145
removed/deferred paths remain absent                         17
```

Occurrence-sensitive totals remain:

```text
84 ordinary occurrences: 116/20/9
root/24:                  81/55/9

A = 84×116 + 81 =  9,825
B = 84×20  + 55 =  1,735
C = 85×9        =    765
union              = 12,325 = 85×145
```

The causal-outcome carrier itself is driver input and is not an A, B, or C expected path. No production observation/view field is added for it.

For W04/W05, actual writer/transport/result/accounting effects continue to come only from actual execution; the carrier changes no effect count or raw custody. For W23, the typed branch is refusal-before-effect: existing A116 fields express the exact refusal, unavailable admitted/terminal/result/frame state, and only those no-effect dimensions independently proven untouched as available-known-zero. No new accounting field, path, denominator, aggregate, or fabricated zero is introduced.

## 7. Exact C15 candidate additions and future write-set update

Relative to the current closed write-set at the pinned HEAD, the future canonical C15 candidate set must add or change exactly the following and nothing else. Every candidate remains `UNKNOWN` until a separately authorized canonical run and independent review.

### 7.1 Field candidates

Add exactly two field candidates:

```text
callHierarchyLayer1RequestAttachment.causalOutcome callHierarchyLayer1GenericCausalOutcome
callHierarchyLayer1GenericAttemptLineage.causalOutcome callHierarchyLayer1GenericCausalOutcome
```

Closed field-candidate total: **12** (previously 10).

### 7.2 Type/constant candidates

Add exactly two type candidates:

```text
callHierarchyLayer1GenericCausalOutcomeClass
callHierarchyLayer1GenericCausalOutcome
```

Change the existing `callHierarchyLayer1GenericRefusalType` candidate only by adding `callHierarchyLayer1GenericRefusalMethodUnavailable` to its closed constant block.

Closed type-candidate total: **25** (previously 23). The changed refusal constant block does not create a third type.

### 7.3 Function/method candidates

Add exactly one new method candidate:

```text
(callHierarchyLayer1RequestAttachment).privateCallHierarchyLayer1WithCausalOutcome(callHierarchyLayer1GenericCausalOutcome) callHierarchyLayer1RequestAttachment
```

Closed new function/method-candidate total: **22** (previously 21).

The existing `newCallHierarchyLayer1GenericAttemptLineage` candidate changes body only to copy the comparable carrier. Existing `(*Manager).RoundTrip` changes body only for the one typed pre-effect branch. Existing `(*callHierarchyLayer1GenericAttemptLineage).finishAccepted` changes body only for accepted-terminal consumption after transport. Existing `(*Manager).finishRoundTrip` remains the sole admitted finalizer and changes only as required to pass the already carried typed classification to `finishAccepted`; its signature remains unchanged.

No helper signature changes. No new refusal finalizer, result parser, dispatch helper, decoder, validator, owner/view field, Manager field, registry field, wrapper, map, pointer, side table, or public symbol is a candidate.

The closed existing-declaration set remains **17** because `RoundTrip`, `finishRoundTrip`, `callHierarchyLayer1RequestAttachment`, and the relevant containing declarations are already members of that set. The closed allocation/copy-site row count remains **16**: the comparable field, setter, constructor copy, branch, and finalizer consumption allocate nothing and add no defensive-copy site.

### 7.4 Future write-set amendment

After documentation review, and only after corpus bytes and RED receive their separate authorities and RED GO, the finite disposable/production write-set documentation must be refreshed to incorporate exactly this delta while preserving the same three-file physical boundary. A canonical C15 run must compare the actual future patch against the refreshed closed set; this document is not a C15 result and does not authorize that run.

Any extra or missing field, type, constant, declaration, function/method, allocation/copy site, source file, or candidate is a hard stop requiring a new amendment and independent review.

## 8. Rejected patch as negative evidence only

The rejected patch hash is:

```text
9e92fc2a6cdcb44715dc9038c4a87c6227c4295d
```

That hash is recorded only as negative evidence. No code, test result, carrier shape, causal conclusion, authority, writer custody, or reusable implementation is inherited from it.

## 9. Required authority sequence

Authority is strict, non-transitive, and must occur in this order:

1. **Documentation review:** independently review this exact document and issue or deny documentation GO.
2. **Corpus bytes:** only after documentation GO, separately authorize and review exact rematerialized bytes for the three scoped driver additions, preserving all unrelated values and immutable prior versions.
3. **RED GO:** only after exact corpus-byte review, separately authorize and review RED that proves strict decode, anti-dispatch, accepted-terminal opacity, W23 pre-effect refusal, mutation guards, unchanged accounting, and all-82 default behavior.
4. **Write-set refresh:** only after RED GO, update and independently review the closed finite three-file symbol/C15 write-set described in §7.
5. **Implementation:** not authorized by this document. Any later disposable experiment or production mutation requires a separate explicit gate, named writer, exact clean pinned input, and independently reviewed custody.

No later step may be inferred from an earlier GO. Corpus bytes do not authorize RED; RED GO does not authorize source; write-set review does not name a writer; a rejected patch grants no authority.

## 10. Hard stops

Stop and report the exact blocker if any later work requires:

- a fourth production file;
- any driver literal or object shape outside §2;
- causal-outcome presence outside the exact three scoped occurrences;
- any nonzero default for the other 82 occurrences;
- dispatch by ID, world, method, expected, result, response, error, terminal, or historical fixture content;
- result parsing for W04/W05 classification;
- any W23 writer, transport, pending-key, admission, or result-parse effect before refusal;
- a public/exported field or method;
- a second carrier, builder, refusal finalizer, admitted finalizer, registry, map, side table, context value, or parser;
- a helper signature change;
- an A/B/C path or accounting-total change;
- a C15 candidate outside the exact delta in §7;
- corpus, RED, source, test, prototype, or generated-artifact mutation under this document; or
- authority inferred from the rejected patch.

## 11. Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_THREE_WORLD_CAUSAL_OUTCOME_AMENDMENT — REVIEW_REQUIRED
DOCUMENTATION REVIEW GO — NOT ISSUED
CORPUS BYTES AUTHORITY — NOT ISSUED
RED GO — NOT ISSUED
WRITE-SET REFRESH GO — NOT ISSUED
IMPLEMENTATION / SOURCE / TEST / WRITER / COMMIT / PRODUCTION AUTHORITY — NOT ISSUED
```

Disposition: **REVIEW_REQUIRED / NO GO**
