# ADR 0011 Call Hierarchy observation API scaffold design

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Design HEAD:** `d6190bf2d7f12fd3d818c72308d6c9ac2de74d89`
- **Status:** design only; implementation not authorized
- **Scope:** inert private typed API scaffold for generic transaction custody
- **Authority:** `provider_authority=UNAVAILABLE`, `authority=0`, `accepted=false`, `completeness=UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`
- **Review gate:** `ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO` (not issued)

## 1. Decision and evidence ceiling

Design a compile-only, default-disabled, fail-closed, package-private scaffold inside the accepted `sessionruntime` reuse boundary. The scaffold gives later causal RED tests stable typed names for transaction identity, exact original request/response frame observation, generic predecessor linkage, selected-item handoff commitment, closed terminal state, and finite accounting. It does not observe a frame, select an item, interpret a method, emit a diagnostic, or alter a round trip.

This design is based on the accepted D/R recorder reuse assessment and its GO review. The managed exact-workspace session lookup for this design returned `APPLICABLE_SESSION_NOT_FOUND`; therefore this is explicitly a degraded filesystem-only design. Text search and source outlines are not treated as call-relationship evidence. No source completeness, runtime execution, provider authentication, CALLS truth, or production authority is claimed.

The scaffold is generic custody only. Incoming and outgoing profile ownership, direction, target meaning, caller/callee orientation, `fromRanges`, occurrence construction, normalization, qualification, evidence roles, and GO authority remain absent.

## 2. Hard boundary

The scaffold MUST be:

- unexported and package-private in `sessionruntime`;
- represented only by unexported fields marked `json:"-"` where attached to existing request/result structs;
- default disabled and fail closed;
- inert when absent and inert when supplied;
- unreachable from production constructors and registration;
- incapable of changing request bytes, response bytes, request IDs, scheduling, cancellation, errors, lifecycle, diagnostics, accounting, or returned production values; and
- incapable of producing an observation until a separately reviewed behavior phase.

The scaffold MUST NOT add or change:

- a public or exported type, constructor, method, option, interface, or field;
- CLI, MCP, schema, config, environment, build-tag, registry, capability, profile catalog, or `init` activation;
- production host wiring or a production constructor call;
- a new package, source file, test file, fixture, or public file;
- selection, normalization, occurrence construction, qualification, direction, caller/callee meaning, or `fromRanges` meaning;
- evidence, manifest, acquisition, publication, replay, admission, graph, grouping, Leiden, or semantic-vector artifacts; or
- a commit.

Supplying an internally constructed scaffold request is not enablement. Before a later behavior gate, it has exactly the same effect as omitting it.

## 3. Typed identity model

Every identity is a distinct defined type. No identity is an alias of another identity, and no plain `string` or `uint64` can cross a typed API boundary where one of these identities is required.

```go
type callHierarchyWorkspaceCustody struct {
    managerNonce [32]byte
}
type callHierarchySessionIdentity string
type callHierarchyGeneration uint64
type callHierarchyJSONRPCRequestKeyKind uint8
type callHierarchyJSONRPCRequestKey struct {
    kind        callHierarchyJSONRPCRequestKeyKind
    stringValue string
    numberBytes []byte
}
type callHierarchyRequestIdentity struct {
    workspace  callHierarchyWorkspaceCustody
    session    callHierarchySessionIdentity
    generation callHierarchyGeneration
    requestKey callHierarchyJSONRPCRequestKey
}
type callHierarchyProfileIdentity string
type callHierarchyBindingIdentity string
type callHierarchyTransactionIdentity struct {
    request callHierarchyRequestIdentity
    local   [32]byte
}
type callHierarchyPredecessorIdentity string
type callHierarchyMethodIdentity string
type callHierarchySelectedOrdinal uint64
```

`callHierarchyJSONRPCRequestKeyKind` is closed to `String` and `Number`; its zero value is invalid. `stringValue` contains the exact decoded JSON string value only for `String`. `numberBytes` contains a defensive copy of the exact JSON number token bytes only for `Number`, preserving distinctions such as string `"1"` versus number `1` and any distinct valid number token spellings accepted by the existing wire layer. There is no `String()`, formatting, coercion, decimal conversion, display-key constructor, or equality through normalized text. If a future `lspwire.RequestKey` does not preserve these distinctions, it MUST NOT be used as this identity; construction must occur at the exact parsed JSON-RPC ID custody point before information loss.

These are the private Go spellings for the required conceptual identities:

| Required identity | Private scaffold spelling | Meaning ceiling |
| --- | --- | --- |
| `WorkspaceIdentity` | `callHierarchyWorkspaceCustody` | Opaque manager-owned custody token derived from one exact managed workspace/session/generation record. Never caller constructed, asserted, decoded, or payload inferred. |
| `SessionIdentity` | `callHierarchySessionIdentity` | Exact managed session identity, present only inside manager-derived request identity. |
| `Generation` | `callHierarchyGeneration` | Exact managed generation, present only inside manager-derived request identity. |
| `JSONRPCRequestKey` / `RequestIdentity` | `callHierarchyJSONRPCRequestKey` / `callHierarchyRequestIdentity` | Exact typed JSON-RPC key joined to manager-owned workspace/session/generation custody; string and number remain distinct without display normalization. |
| `ProfileIdentity` | `callHierarchyProfileIdentity` | Opaque caller-supplied identity. The generic scaffold does not understand its direction or authority. |
| `BindingIdentity` | `callHierarchyBindingIdentity` | Opaque caller-supplied binding identity. It does not encode direction in the generic API. |
| `TransactionIdentity` | `callHierarchyTransactionIdentity` | Manager-derived request identity plus manager-local transaction discriminator; includes the exact typed JSON-RPC key. |
| `PredecessorIdentity` | `callHierarchyPredecessorIdentity` | Identity of an immutable predecessor commitment. |
| `MethodIdentity` | `callHierarchyMethodIdentity` | Exact method string identity; admission is only by the immutable caller-supplied allowlist. |
| `SelectedOrdinal` | `callHierarchySelectedOrdinal` | An opaque ordinal commitment supplied by a later owner; the scaffold does not perform selection. |

Zero values are invalid for construction. The only constructor for `callHierarchyWorkspaceCustody`, `callHierarchyRequestIdentity`, and `callHierarchyTransactionIdentity` is a private `Manager` method that reads the exact managed session record under manager custody; no package caller receives a constructor taking workspace, session, generation, request-key components, strings, or numbers. Callers may receive and pass only an already-issued lease or immutable commitment containing the opaque identity. The manager constructor validates a nonzero manager nonce, exact session-record membership, current generation, and a valid exact typed request key; it defensively copies request-key number bytes and returns `(value, false)` on zero, stale-generation, mismatched workspace/session/generation, foreign-manager token, malformed-key, or over-limit input. Validation conveys only well-formed local custody, never semantic truth.

Opaque token equality is exact field equality after kind-specific byte comparison. A zero token, a token issued by another manager, a token for a different managed workspace/session/generation record, or a token whose private bytes were not issued from the manager's record is refused before any request, predecessor, selected-item, handoff, observation, lease, commitment, or hash preimage can be constructed. No accessor exposes the manager nonce or mutable key bytes.

## 4. Immutable commitments

### 4.1 Predecessor commitment and chain

```go
type callHierarchyPredecessorCommitment struct {
    identity         callHierarchyPredecessorIdentity
    transaction      callHierarchyTransactionIdentity
    requestIdentity  callHierarchyRequestIdentity
    method           callHierarchyMethodIdentity
    requestOrdinal   uint64
    commitmentSHA256 [32]byte
}

type callHierarchyPredecessorChain struct {
    commitments []callHierarchyPredecessorCommitment
}

func newCallHierarchyPredecessorChain(
    in []callHierarchyPredecessorCommitment,
    limits callHierarchyObservationLimits,
) (callHierarchyPredecessorChain, bool)

func (c callHierarchyPredecessorChain) copyCommitments() []callHierarchyPredecessorCommitment
```

The constructor defensively copies the slice and every nested typed request-key byte sequence, rejects zero/duplicate identities, refuses any transaction/request-identity mismatch or zero/stale/foreign workspace custody token, rejects ordinal overflow or non-finite bounds, and preserves input order exactly. There is no append method. Access returns another deep defensive copy. A predecessor link is an immutable local linkage claim only; it does not prove that one method semantically follows another.

Every commitment digest and identity digest uses an unambiguous, length-delimited binary preimage containing the workspace-custody bytes, exact session identity bytes, generation, JSON-RPC key kind, and the exact kind-specific request-key bytes before all remaining fields. The same complete `callHierarchyRequestIdentity` is included in predecessor, selected-item, handoff, request-frame, response-frame, aggregate transaction, and any future owner/lease commitment preimage. Hashing a display string, formatted number, normalized JSON, or an existing lossy `lspwire.RequestKey` is prohibited.

### 4.2 Selected-item commitment

```go
type callHierarchySelectedItemCommitment struct {
    ordinal          callHierarchySelectedOrdinal
    sourceRequest    callHierarchyRequestIdentity
    sourceFrame      callHierarchyTransactionIdentity
    sourceOffset     uint64
    sourceLength     uint64
    exactBytes       []byte
    exactSHA256      [32]byte
}

func newCallHierarchySelectedItemCommitment(
    ordinal callHierarchySelectedOrdinal,
    sourceRequest callHierarchyRequestIdentity,
    sourceFrame callHierarchyTransactionIdentity,
    sourceOffset, sourceLength uint64,
    exactBytes []byte,
    limits callHierarchyObservationLimits,
) (callHierarchySelectedItemCommitment, bool)

func (c callHierarchySelectedItemCommitment) copyBytes() []byte
```

This type records a supplied byte-span commitment. It does not locate, scan, choose, decode, normalize, or validate a Call Hierarchy item. The constructor requires `sourceLength == len(exactBytes)`, checked `sourceOffset + sourceLength`, matching nonzero manager-issued `sourceRequest` and `sourceFrame.request`, and an exact SHA-256 commitment over the full request-identity preimage plus the copied span bytes.

### 4.3 Handoff commitment

```go
type callHierarchyHandoffCommitment struct {
    selected         callHierarchySelectedItemCommitment
    successorRequest callHierarchyRequestIdentity
    successor        callHierarchyTransactionIdentity
    successorOffset uint64
    successorLength uint64
    successorBytes []byte
    successorSHA256 [32]byte
}

func newCallHierarchyHandoffCommitment(
    selected callHierarchySelectedItemCommitment,
    successorRequest callHierarchyRequestIdentity,
    successor callHierarchyTransactionIdentity,
    successorOffset, successorLength uint64,
    successorBytes []byte,
    limits callHierarchyObservationLimits,
) (callHierarchyHandoffCommitment, bool)

func (c callHierarchyHandoffCommitment) copySuccessorBytes() []byte
```

Construction succeeds only when the selected and successor request identities are nonzero manager-issued identities, `successor.request == successorRequest`, all workspace tokens belong to the constructing manager, copied successor bytes are byte-for-byte equal to the selected commitment bytes, lengths agree, offsets add without overflow, and full request-identity-prefixed digests agree. It establishes unchanged-byte handoff only. It does not establish selection correctness, direction, item meaning, or CALLS semantics.

## 5. Exact original frame observations

Request and response are separate types so they cannot be interchanged accidentally.

```go
type callHierarchyJSONTokenForm uint8

const (
    callHierarchyJSONTokenFormUnavailable callHierarchyJSONTokenForm = iota
    callHierarchyJSONTokenFormNull
    callHierarchyJSONTokenFormEmptyArray
    callHierarchyJSONTokenFormOther
)

type callHierarchyOriginalRequestFrameObservation struct {
    requestIdentity  callHierarchyRequestIdentity
    frameBytes       []byte
    frameLength      uint64
    frameSHA256      [32]byte
    bodyOffset       uint64
    bodyLength       uint64
    bodySHA256       [32]byte
    paramsOffset     uint64
    paramsLength     uint64
    paramsSHA256     [32]byte
    paramsForm       callHierarchyJSONTokenForm
}

type callHierarchyOriginalResponseFrameObservation struct {
    requestIdentity  callHierarchyRequestIdentity
    frameBytes       []byte
    frameLength      uint64
    frameSHA256      [32]byte
    bodyOffset       uint64
    bodyLength       uint64
    bodySHA256       [32]byte
    resultOffset     uint64
    resultLength     uint64
    resultSHA256     [32]byte
    resultForm       callHierarchyJSONTokenForm
}

func newCallHierarchyOriginalRequestFrameObservation(
    requestIdentity callHierarchyRequestIdentity,
    exactFrame []byte,
    bodyOffset, bodyLength, paramsOffset, paramsLength uint64,
    paramsForm callHierarchyJSONTokenForm,
    limits callHierarchyObservationLimits,
) (callHierarchyOriginalRequestFrameObservation, bool)

func newCallHierarchyOriginalResponseFrameObservation(
    requestIdentity callHierarchyRequestIdentity,
    exactFrame []byte,
    bodyOffset, bodyLength, resultOffset, resultLength uint64,
    resultForm callHierarchyJSONTokenForm,
    limits callHierarchyObservationLimits,
) (callHierarchyOriginalResponseFrameObservation, bool)

func (o callHierarchyOriginalRequestFrameObservation) copyFrameBytes() []byte
func (o callHierarchyOriginalRequestFrameObservation) copyParamsBytes() []byte
func (o callHierarchyOriginalResponseFrameObservation) copyFrameBytes() []byte
func (o callHierarchyOriginalResponseFrameObservation) copyResultBytes() []byte
```

Required invariants:

1. `requestIdentity` is a deep immutable copy of the nonzero manager-issued identity for this exact frame; zero, stale, mismatched, or foreign-manager custody refuses construction.
2. `frameBytes` is a defensive copy of the exact original framed bytes, including framing; it is never reconstructed or remarshaled.
3. `frameLength == len(frameBytes)` and `frameSHA256` commits to the complete length-delimited request-identity preimage followed by `frameBytes`.
4. Every span is frame-relative, checked for finite bounds and overflow, and contained in the copied frame.
5. Body and params/result digests use the same complete request-identity prefix and exact copied subspans; digests never replace bytes.
6. `Null` means the exact trimmed token bytes are `null`; `EmptyArray` means the exact trimmed token bytes are `[]`; they remain distinct even if both imply zero later semantic rows.
7. `Unavailable` is valid only when the corresponding payload span is absent with zero offset/length. It is not equal to `Null`, `EmptyArray`, or an empty byte slice.
8. `Other` preserves all exact bytes without semantic classification.
9. Constructors reject inconsistent form declarations rather than normalize whitespace, repair JSON, infer semantics, or derive identity from a display string.

The scaffold phase defines these constructors but does not call them from runtime behavior.

## 6. Immutable method allowlist without direction knowledge

```go
type callHierarchyMethodAllowlist struct {
    methods []callHierarchyMethodIdentity
}

func newCallHierarchyMethodAllowlist(
    methods []callHierarchyMethodIdentity,
    limits callHierarchyObservationLimits,
) (callHierarchyMethodAllowlist, bool)

func (a callHierarchyMethodAllowlist) permits(method callHierarchyMethodIdentity) bool
func (a callHierarchyMethodAllowlist) copyMethods() []callHierarchyMethodIdentity
```

The caller supplies the exact allowlist. The constructor:

- requires a positive finite method-count limit;
- rejects empty methods, duplicates, and over-limit input;
- preserves exact strings and caller order;
- defensively copies input; and
- exposes no mutation.

`permits` performs exact equality only. It has no knowledge of incoming, outgoing, prepare, document symbols, profiles, binding semantics, method sequence, or a union of directional methods. The scaffold does not contain built-in Call Hierarchy method constants and does not invoke `permits` from production behavior.

A later directional owner may supply exactly its reviewed three-method list. That later fact is outside this generic scaffold.

## 7. Closed terminal enum

```go
type callHierarchyObservationTerminal uint8

const (
    callHierarchyObservationTerminalUnavailable callHierarchyObservationTerminal = iota
    callHierarchyObservationTerminalComplete
    callHierarchyObservationTerminalSuccessfulEmpty
    callHierarchyObservationTerminalTransportError
    callHierarchyObservationTerminalProtocolError
    callHierarchyObservationTerminalMissingPayload
    callHierarchyObservationTerminalMalformed
    callHierarchyObservationTerminalPartialCapture
    callHierarchyObservationTerminalLimitRefused
    callHierarchyObservationTerminalCancelled
)

func (t callHierarchyObservationTerminal) valid() bool
```

No string conversion accepts arbitrary values. `valid` is a closed switch. The scaffold's only returned terminal is `Unavailable`, because it produces no attempts or observations. `SuccessfulEmpty` preserves the ability to distinguish an exact reviewed empty-success state later, but this scaffold does not decide whether `null` or `[]` maps to it; that decision remains outside generic custody.

There is deliberately no ambiguous-selection, target-refusal, incoming, outgoing, caller, callee, occurrence, or qualification terminal in this generic scaffold.

## 8. Finite limits and accounting with availability

Known zero and unavailable are distinct.

```go
type callHierarchyMeasure struct {
    available bool
    value     uint64
}

type callHierarchyObservationLimits struct {
    maxBytes       uint64
    maxObjects     uint64
    maxWork        uint64
    maxDiagnostics uint64
}

type callHierarchyObservationAccounting struct {
    bytes       callHierarchyMeasure
    objects     callHierarchyMeasure
    work        callHierarchyMeasure
    diagnostics callHierarchyMeasure
}

func newCallHierarchyObservationLimits(
    maxBytes, maxObjects, maxWork, maxDiagnostics uint64,
) (callHierarchyObservationLimits, bool)

func unavailableCallHierarchyObservationAccounting() callHierarchyObservationAccounting
func knownZeroCallHierarchyObservationAccounting() callHierarchyObservationAccounting
func (a callHierarchyObservationAccounting) validAgainst(l callHierarchyObservationLimits) bool
```

Rules:

- each limit is independently positive, finite, at or below a package-private hard maximum, and frozen by value;
- there is no unlimited sentinel;
- all additions and multiplications use checked arithmetic before mutation;
- no dimension borrows from another;
- `{available:false,value:0}` means unavailable, not observed zero;
- `{available:true,value:0}` means observed and known zero;
- `{available:false,value:N}` for `N != 0` is invalid;
- accounting must be either independently available per dimension or explicitly unavailable per dimension; and
- diagnostics accounting is generic recorder accounting only and does not authorize emitting a diagnostic.

The scaffold phase returns unavailable accounting. It does not reserve, charge, release, hash, allocate observation storage, or emit diagnostics during round trips.

## 9. Aggregate private API

```go
type callHierarchyObservationIdentity struct {
    request     callHierarchyRequestIdentity
    profile     callHierarchyProfileIdentity
    binding     callHierarchyBindingIdentity
    transaction callHierarchyTransactionIdentity
    method      callHierarchyMethodIdentity
}

type callHierarchyObservationRequest struct {
    identity     callHierarchyObservationIdentity
    allowlist    callHierarchyMethodAllowlist
    predecessors callHierarchyPredecessorChain
    selected     *callHierarchySelectedItemCommitment
    handoff      *callHierarchyHandoffCommitment
    limits       callHierarchyObservationLimits
}

type callHierarchyObservation struct {
    identity     callHierarchyObservationIdentity
    predecessors callHierarchyPredecessorChain
    request      *callHierarchyOriginalRequestFrameObservation
    response     *callHierarchyOriginalResponseFrameObservation
    selected     *callHierarchySelectedItemCommitment
    handoff      *callHierarchyHandoffCommitment
    terminal     callHierarchyObservationTerminal
    accounting   callHierarchyObservationAccounting
}

type callHierarchyObservationView struct {
    Identity     callHierarchyObservationIdentity
    Predecessors []callHierarchyPredecessorCommitment
    Request      *callHierarchyOriginalRequestFrameObservation
    Response     *callHierarchyOriginalResponseFrameObservation
    Selected     *callHierarchySelectedItemCommitment
    Handoff      *callHierarchyHandoffCommitment
    Terminal     callHierarchyObservationTerminal
    Accounting   callHierarchyObservationAccounting
}

type callHierarchyObservationLease struct {
    owner *callHierarchyObservationOwner
}

func newCallHierarchyObservationRequest(
    identity callHierarchyObservationIdentity,
    allowlist callHierarchyMethodAllowlist,
    predecessors callHierarchyPredecessorChain,
    selected *callHierarchySelectedItemCommitment,
    handoff *callHierarchyHandoffCommitment,
    limits callHierarchyObservationLimits,
) (callHierarchyObservationRequest, bool)

func (l *callHierarchyObservationLease) withObservation(
    fn func(callHierarchyObservationView) error,
) error

func (l *callHierarchyObservationLease) release() bool
```

`newCallHierarchyObservationRequest` is callable only with an identity already minted by the manager from its exact managed session record; it has no workspace/session/generation/request-key component parameters and cannot mint or repair identity. It refuses zero, stale-generation, mismatched transaction/request, and foreign-manager custody. All pointer inputs are copied into private owned values during construction. Every nested `numberBytes`, frame byte slice, commitment byte slice, and slice reachable from a callback view is a defensive copy scoped to the callback. No callback-owned alias remains usable after return. Nil pointers mean unavailable; they never mean empty, null, successful, or zero.

The conceptual request/result attachment is:

```go
type RoundTripRequest struct {
    // existing fields unchanged
    callHierarchyObservation *callHierarchyObservationRequest `json:"-"`
}

type RoundTripResult struct {
    // existing fields unchanged
    callHierarchyObservation *callHierarchyObservationOwner `json:"-"`
}

func (r RoundTripResult) privateCallHierarchyObservationLease() (
    callHierarchyObservationLease,
    bool,
)
```

During this scaffold phase:

- `Manager.RoundTrip` and every private B4 path ignore `req.callHierarchyObservation`;
- `RoundTripResult.callHierarchyObservation` is always `nil`;
- `privateCallHierarchyObservationLease` always returns the zero lease and `false`;
- `withObservation` on a zero or released lease returns the existing package-private unavailable/closed error chosen at implementation review;
- `release` on a zero lease returns `false`;
- no owner is constructed;
- no observation is produced;
- no callback runs;
- no accounting becomes available; and
- no existing behavior or result changes.

These zero semantics are intentional. They let later tests compile but cannot make a semantic RED pass.

## 10. Explicitly prohibited semantics

The following names and behaviors MUST NOT appear in the scaffold source API:

- `Incoming`, `Outgoing`, `Direction`, `Caller`, `Callee`, `FromRange`, or occurrence types;
- a selector, scanner, normalizer, classifier, assembler, qualification record, or relation builder;
- a method-sequence rule or built-in three-method binding;
- profile or binding registries;
- interpretation of `ProfileIdentity`, `BindingIdentity`, `SelectedOrdinal`, predecessor order, selected bytes, or handoff bytes;
- conversion from retained bytes into a `CallHierarchyItem` or other semantic LSP value; or
- a success, accepted, authority, evidence-role, admission, or GO field.

Exact-byte validation needed to construct a custody commitment is not semantic selection or normalization. It must remain local, bounded, and side-effect free.

## 11. Why the causal RED still fails

A later behavior phase needs a causal RED that crosses the real API rather than testing a detached helper. The scaffold makes that test compile:

```go
func TestCallHierarchyObservation_CausalRED_NoObservationYet(t *testing.T) {
    allowlist, ok := newCallHierarchyMethodAllowlist(callerMethods, limits)
    if !ok { t.Fatal("test setup") }

    exactIdentity, ok := manager.privateCallHierarchyRequestIdentity(
        managedSessionLease,
        exactTypedJSONRPCRequestKey,
    )
    if !ok { t.Fatal("test setup") }

    request, ok := newCallHierarchyObservationRequest(
        exactIdentity,
        allowlist,
        immutablePredecessors,
        selectedCommitment,
        handoffCommitment,
        limits,
    )
    if !ok { t.Fatal("test setup") }

    got := manager.RoundTrip(ctx, RoundTripRequest{
        // existing valid managed request fields
        callHierarchyObservation: &request,
    })

    lease, ok := got.privateCallHierarchyObservationLease()
    if !ok {
        t.Fatalf("RED: observations=0, want observations>0")
    }
    defer lease.release()

    count := 0
    if err := lease.withObservation(func(v callHierarchyObservationView) error {
        count++
        return nil
    }); err != nil {
        t.Fatal(err)
    }
    if count == 0 {
        t.Fatalf("RED: observations=0, want observations>0")
    }
}
```

With scaffold semantics, `manager.RoundTrip` ignores the request, the result owner is nil, lease acquisition returns `false`, and the test fails exactly at `observations=0, want observations>0`. Thus the test crosses request construction, the actual manager round-trip, result retrieval, and lease API while still proving behavior is absent.

This scaffold MUST NOT add that causal RED test yet. It MUST NOT special-case the test, create a placeholder observation, return a known-zero observation, or make the assertion pass. The behavior test belongs only after a separate behavior-design and RED-writing gate.

## 12. Frozen scaffold implementation write-set

If and only if the design gate is issued, the scaffold implementation proposal is frozen to this strict subset of the accepted seven source files and existing focused tests:

### Source files

1. `sessionruntime/owned_method_pair.go`
   - add the unexported typed identities, immutable allowlist, predecessor/selection/handoff commitments, exact request/response frame observation value types, closed terminal, limits/accounting, aggregate request/observation types, pure constructors, validation, checked arithmetic, and defensive-copy helpers;
   - add no runtime invocation and no Call Hierarchy semantics.
2. `sessionruntime/b4_owned_method_pair_lease.go`
   - add the unexported observation owner/view/lease shape and inert zero/released lease behavior;
   - construct no owner and publish no lease.
3. `sessionruntime/sessionruntime.go`
   - add only the private `json:"-"` request pointer, private `json:"-"` result owner pointer, and private result lease accessor;
   - do not branch on the request, install an owner, reserve accounting, or alter `RoundTrip` behavior.

### Existing focused test files

4. `sessionruntime/owned_method_pair_test.go`
   - compile-only constructor/type conformance, distinct-type assignments, defensive-copy checks performed only on pure values, null-versus-empty exact-byte value construction, closed-enum validity, immutable allowlist copying, immutable predecessor copying, and unavailable-versus-known-zero representation.
5. `sessionruntime/b4_owned_method_pair_lease_test.go`
   - compile-only zero-lease and unpopulated-result accessor conformance; no manager execution and no observation-production assertion.

No other accepted source file is needed for the inert scaffold. In particular, the scaffold MUST NOT modify:

- `sessionruntime/b4_request_frame_lease.go`;
- `sessionruntime/b4_response_frame_lease.go`;
- `sessionruntime/method_frame_correspondence.go`; or
- `sessionruntime/b4_definition_private.go`.

Those four remain reserved for a later behavior phase that actually connects original-frame custody, correspondence, reservation, lifecycle, and cleanup. Touching one of them during scaffold implementation invalidates this frozen write-set and returns to review.

No new test, fixture, package, source file, or generated file is permitted. Any need to touch a file outside the five named files invalidates the scaffold proposal.

## 13. Compile-only conformance test plan

The scaffold implementation phase may add only tests that establish API shape and pure value ownership:

1. **Distinct identity compile checks:** constructor signatures require the exact typed `JSONRPCRequestKey`, `RequestIdentity`, and `TransactionIdentity` and reject accidental interchange at compile time through typed helper declarations. String key `"1"` and number key `1` construct unequal identities; exact number-token bytes remain unchanged. Do not add intentionally uncompilable repository code.
2. **Zero semantics:** zero `RoundTripResult` yields `(zeroLease, false)`; zero lease release is `false`; no callback runs.
3. **Request/result privacy:** reflection over `RoundTripRequest` and `RoundTripResult` confirms the private fields carry `json:"-"`; JSON marshaling remains unchanged.
4. **Manager custody and defensive copies:** no caller-facing component constructor can mint workspace/request/transaction identity; zero, stale-generation, mismatched-session/workspace, and foreign-manager tokens refuse. Mutating request-key bytes, constructor inputs, or accessor outputs does not mutate identities, immutable allowlists, predecessor chains, selected bytes, handoff bytes, or exact frame bytes.
5. **Exact-byte identity and preimages:** every transaction, predecessor, selection, handoff, request-frame, and response-frame commitment preimage includes the complete typed request identity; lengths and SHA-256 values match copied exact bytes and exact subspans. Display strings, string/number coercion, normalized numbers, and decode/remarshal are not accepted as identity or original input.
6. **Null versus empty:** exact `null`, exact `[]`, unavailable, and other bytes remain four distinct token-form states.
7. **Closed terminal:** every declared terminal validates and out-of-range values refuse; runtime still returns no terminal because it returns no observation.
8. **Availability:** unavailable and known zero differ for bytes, objects, work, and diagnostics.
9. **Finite limits:** zero, over-hard-max, and checked-overflow inputs refuse independently; exact finite boundaries construct.
10. **Immutable allowlist:** exact caller order and exact strings are copied; duplicates, empty methods, and over-limit lists refuse; there is no direction-sensitive behavior.
11. **Immutable predecessor chain:** exact order is copied; zero/duplicate identities and over-limit chains refuse; there is no semantic chain interpretation.
12. **Commitments:** selected and handoff constructors enforce lengths, spans, digests, and byte equality without scanning or semantic interpretation.
13. **Static symbol check:** no exported scaffold symbol and no forbidden semantic identifier is introduced in the diff.

These are compile/conformance tests, not behavior tests. They MUST NOT run a manager transaction, assert an observation count above zero, capture a frame, exercise production reachability, or claim enabled/disabled equivalence. Existing focused package tests may be run only to confirm compilation and unchanged compatibility.

## 14. Review checklist

An independent reviewer should issue GO only if every item is true:

- [ ] The reviewed file is exactly this design at its recorded SHA-256 and descends from the stated HEAD.
- [ ] The proposal is generic custody only and contains no direction knowledge.
- [ ] All required conceptual identities, including exact typed `JSONRPCRequestKey`, `RequestIdentity`, and `TransactionIdentity`, have distinct private typed spellings.
- [ ] String and number JSON-RPC keys remain distinct exact identities with no display-string or numeric normalization.
- [ ] Workspace/request/transaction identity can be minted only by a private manager constructor from the exact managed workspace/session/generation record; callers can only hold a lease or immutable commitment.
- [ ] Zero, stale-generation, mismatched workspace/session/generation, and foreign-manager custody tokens refuse, and every nested mutable byte sequence is defensively copied.
- [ ] Every immutable predecessor, selected-item, handoff, request/response-frame, aggregate transaction, and future owner/lease hash preimage includes the complete exact typed request identity.
- [ ] Request and response frame types preserve exact bytes, length, SHA-256, and frame-relative spans.
- [ ] `null`, `[]`, unavailable, and other exact payload bytes remain distinct.
- [ ] Terminal values are closed and the scaffold returns no observation terminal.
- [ ] Bytes, objects, work, and diagnostics distinguish unavailable from known zero.
- [ ] Limits are positive, finite, independent, hard-capped, and overflow checked.
- [ ] The exact caller-supplied allowlist is immutable and has no direction or built-in method knowledge.
- [ ] Predecessor links/chains, selected-item commitments, and handoff commitments are immutable and defensively copied.
- [ ] Every request/result attachment is private and `json:"-"`.
- [ ] Default and supplied scaffold requests are behaviorally inert and fail closed.
- [ ] No owner, observation, callback, diagnostic, accounting charge, reservation, hash, or allocation occurs during a round trip.
- [ ] The causal RED can cross the API later and still fails with expected nonzero observations.
- [ ] No selection, normalization, occurrence, qualification, or direction enters the source API.
- [ ] There is no production constructor, registration, CLI, MCP, config, environment, build-tag, schema, or public API route.
- [ ] The implementation write-set is exactly the three named existing source files and two named existing tests.
- [ ] No behavior test is authorized in the scaffold phase.
- [ ] No evidence, manifest, fixture, other document, generated artifact, or commit is authorized.

The only review token is:

```text
review_gate = ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO
status = NOT_ISSUED
```

The token is not issued by this document. It authorizes only a separately executed inert scaffold implementation within the frozen five-file write-set. It does not authorize behavior, causal RED writing, acquisition, evidence, qualification, profile GO, production admission, or a commit.

## 15. Post-gate sequence

After, and only after, an independent review issues the exact design GO over this file hash:

1. verify exact repository, ancestry, clean state, and design hash;
2. implement only the three named source-file scaffold changes;
3. add only compile/conformance coverage to the two named existing test files;
4. run `gofmt` only on changed Go files;
5. run the two focused test selections and package compilation, without adding behavior tests;
6. run static diff checks for exported symbols, forbidden semantic identifiers, production activation surfaces, and files outside the frozen write-set;
7. run `git diff --check`;
8. obtain an independent scaffold implementation review;
9. stop with no commit and no behavior claim;
10. prepare a separate behavior-design proposal for original-frame attachment, owner construction, reservation, lifecycle, and cleanup across the remaining accepted files;
11. only after that separate behavior gate, add the causal RED shown above and verify it fails specifically with `observations=0, want observations>0`; and
12. only after independent RED-causality review may a later implementation attempt to produce observations.

No step in this sequence authorizes selection, normalization, occurrence construction, direction semantics, qualification, evidence promotion, profile admission, or production reachability.

## 16. Current disposition

Current disposition is **STOP**:

- `ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO` is not issued;
- no source or test implementation is authorized;
- no behavior test is authorized;
- no observation exists;
- no semantic RED can pass;
- no evidence or authority changes;
- production remains unreachable and `REFUSED_MISSING_EVIDENCE`; and
- no commit is authorized.
