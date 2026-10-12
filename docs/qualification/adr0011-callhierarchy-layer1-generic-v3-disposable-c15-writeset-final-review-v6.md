# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review V6

Status: **GO for the exact finite disposable write-set boundary only**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `9e92fc2a6cdcb44715dc9038c4a87c6227c4295d`
- Reviewed modified input: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Reviewed input state: one tracked documentation modification over the pinned HEAD; `git diff --check` clean
- Source, tests, corpus, prototype, and generated artifacts were not modified or executed.
- This review supersedes prior write-set reviews only for this exact current-HEAD integrated revision. Prior disposable runs and candidate lists remain noncanonical.

## 1. Exact physical boundary

The complete candidate production write-set remains exactly these three existing files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

Pinned-source inspection confirms those files contain the existing request/result carriers, private wrapper chain, pending-key allocation, raw read custody, terminal finalizer, immutable-owner pattern, completed STOP/RESTART seams, and shutdown seam required by the proposal. No fourth production file, test file, corpus file, generated artifact, bridge interface, side table, or package global is required or admitted.

## 2. Complete symbol, field, type, signature, and call-site audit

The proposal closes every production declaration and exact Go type. The identity correction adds exactly these three comparable attachment fields:

```go
profile   callHierarchyProfileIdentity
binding   callHierarchyBindingIdentity
workspace callHierarchyWorkspaceCustody
```

At the pinned HEAD, `callHierarchyProfileIdentity` and `callHierarchyBindingIdentity` are defined `string` types, and `callHierarchyWorkspaceCustody` contains exactly `managerNonce [32]byte`. The additions therefore preserve attachment comparability and introduce no identity pointer, holder, map, parser, or allocation.

The exact setter signature is immutable-test compatible:

```go
func (attachment callHierarchyLayer1RequestAttachment) privateCallHierarchyLayer1WithIdentity(profile callHierarchyProfileIdentity, binding callHierarchyBindingIdentity, workspace callHierarchyWorkspaceCustody) callHierarchyLayer1RequestAttachment
```

Its authorized behavior is closed: value-return a copy; replace only `profile`, `binding`, and `workspace`; preserve `selected`, `handoff`, `ledger`, the exact `predecessors` pointer, `enabled`, and every other field; perform no allocation or semantic derivation.

The RED call order is exact and mechanically present in the immutable v3 test:

```text
privateCallHierarchyLayer1WithIdentity
→ privateCallHierarchyLayer1WithEnabled
→ privateCallHierarchyLayer1WithPredecessors
```

The constructor remains the single existing proposed function candidate with unchanged name and signature:

```go
func newCallHierarchyLayer1GenericAttemptLineage(req RoundTripRequest) *callHierarchyLayer1GenericAttemptLineage
```

Its revised body contract is verbatim value custody: when enabled, copy attachment `profile`, `binding`, and `workspace` into the existing attempt fields, with workspace copied into `requestIdentity.workspace`; preserve all other request identity, commitment, and predecessor custody. It must not derive any identity from method, session reconstruction, expected values, world/fixture/occurrence names, commitments, params, results, controls, or failure text. Disabled remains nil. The constructor allocates exactly one attempt only when enabled and introduces no second allocation or new type.

The private request-result carrier is exact:

```text
RoundTrip first executable line overwrites the request attempt carrier
→ unchanged by-value roundTrip / roundTripWithPrivate / roundTripWithPrivateTransaction flow
→ admission immediately after pending-key allocation
→ one pointer copy into the transient RoundTripResult carrier
→ every admitted terminal path reaches finishRoundTrip
→ finishRoundTrip finalizes and publishes at most one immutable owner
→ transient result carrier clears before every external return
```

The two transient carrier fields and separate owner field are:

```go
RoundTripRequest.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1GenericAttempt  *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1Generic         *callHierarchyLayer1GenericOwner
```

All existing helper signatures listed by the proposal match the pinned source and remain byte-for-byte unchanged. No edit is admitted for `(*Manager).roundTrip`, `(*Manager).roundTripWithPrivate`, `(*Manager).AdmitSeedBinding`, `(*Manager).RoundTripPrivateB4`, their direct call sites, or immutable direct helper-call tests.

## 3. Refusal closure and admitted finalization

The pinned source has exactly the twelve ordered enabled pre-key refusal rows named by the proposal:

1. private C16/C17 accounting not implemented;
2. C18 runtime gate;
3. context failure;
4. session absent;
5. generation mismatch;
6. lifecycle conflict;
7. private P2 deadline refusal;
8. owned-document mismatch;
9. non-`wireChild` process;
10. request/worker capacity;
11. readiness-history reader/borrower failure; and
12. `admitPrivateB4SourcesC17Locked` failure.

The two private-admission rows intentionally share one enum value, so the closed refusal type has zero plus eleven distinct nonzero constants for twelve branch rows. Every lock-held pre-key branch must unlock before invoking the Manager-owned refusal finalizer. No refusal reconstructs identity, parses failure text, uses the transient result carrier, or calls `finishRoundTrip` merely to transfer custody.

After `key := r.pending.Begin(req.Generation)`, every early return in the reviewed transaction path already funnels through `finishRoundTrip`; the proposal preserves that sole admitted finalizer. No post-key bypass or second publication path is admitted.

## 4. Manager registry, retirement race, and lifecycle

The only Manager additions are:

```go
type callHierarchyLayer1GenericGenerationKey struct {
    sessionID  string
    generation uint64
}

callHierarchyLayer1GenericOwners             []*callHierarchyLayer1GenericOwner
callHierarchyLayer1GenericRetiredGenerations map[callHierarchyLayer1GenericGenerationKey]struct{}
```

Both are exclusively guarded by `m.mu`. The exact generation key is comparable and has no wildcard/session-only form. Publication, retirement marking, and detach serialize under the same lock:

- publication before a retirement mark is selected by the subsequent detach;
- matching publication after retirement marking, or any publication after closure, is refused;
- refused publication returns the same owner for immediate outside-lock `requestTerminal` followed by `waitReleased`;
- no callback, terminal action, drain, or wait occurs while holding `m.mu`.

Completed STOP marks the exact current generation retired before detaching all owners for that session. Completed RESTART marks the exact old generation before detaching only that generation and drains before successor start. Failure before committed retirement does not mark or detach. After retirement/detach, owners are never reinserted. Shutdown, after `m.closed` is durable and workers reach zero, transfers the whole registry slice, nils the Manager field, clears the retired set, unlocks, and drains in registry order.

The detach allocation contract is exact and singular:

```go
detached := make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))
```

The helper performs one stable pass, appends only matching pointers to `detached`, compacts retained pointers in place through `m.callHierarchyLayer1GenericOwners[:0]`, nils every vacated tail slot, and assigns the compacted slice back. It performs no retained-slice backing allocation, alternate filtering allocation, owner lock, terminal action, wait, goroutine creation, accounting mutation, or session lookup.

## 5. Release and immutable exposure lifecycle

The owner constructor deep-copies every mutable slice/byte member and sets `cond.L = &owner.mu`. `View` and `Callback` return fresh deep copies. Pointer identity has no metadata-equality meaning.

Explicit `Release` and teardown `requestTerminal` both close admission, drain admitted callbacks, and invoke the same `releaseLocked` transition. `releaseLocked` is the sole observation-clear/release-bit transition and requires `active == 0`; exactly one racer wins, while repeated or losing explicit releases return false. Callback completion, including panic recovery, decrements `active` exactly once and broadcasts at zero. `waitReleased` is observational and is called only after forced `requestTerminal`, so teardown cannot wait on an absent user actor.

Ordinary non-closed, non-retired `RoundTrip` returns an open/acquirable owner. Closed or exact-retired publication attaches the owner only after outside-lock forced terminalization and join, so acquisition fails closed. The exact immutable-test acquisition signature remains:

```go
func (result RoundTripResult) privateCallHierarchyLayer1GenericLease() (any, bool)
```

The dynamic value is `callHierarchyLayer1GenericLease`; disabled, absent-owner, closed, and released cases return `(nil, false)`.

## 6. A116 and forbidden semantic derivation

The observation and view are closed over the ordinary A116 family. The listed finite projections reconcile to exactly 116 and preserve unavailable, available-empty, and available-known-zero distinctions. The three corrected identity projections come only from the typed request attachment through attempt custody:

- `identity.profile` from attachment `profile`;
- `identity.binding` from attachment `binding`;
- `identity.request_identity.workspace_nonce_hex` from the exact `[32]byte` attachment workspace nonce, hex-projected only at exposure.

No production field or helper stores or computes B20, any C path, the W24 root aggregate, or W25 equality. No method/expected/world/fixture/occurrence/control value, commitment content, params/result meaning, failure text, current session reconstruction, sibling actual, or prototype output may select or derive identity, refusal, terminal, result, error, lifecycle, external, ledger, or accounting semantics. Raw response bytes remain captured from `readResult.raw` before parse-derived normalization; no-frame branches fabricate nothing.

## 7. Exact closed C15 reconciliation

A deterministic recount of every explicit row in §11 produces exactly:

| Candidate class | Exact total | Duplicate rows |
|---|---:|---:|
| field candidates | 10 | 0 |
| type candidates | 23 | 0 |
| existing declarations expected to change | 17 | 0 |
| new function/method candidates | 21 | 0 |
| allocation/copy-site rows | 16 | 0 |

The current-HEAD identity delta is completely represented:

- three field candidates: `profile`, `binding`, `workspace`;
- one new method candidate: `privateCallHierarchyLayer1WithIdentity`;
- one changed constructor body: `newCallHierarchyLayer1GenericAttemptLineage`, whose name/signature and single function-candidate identity remain unchanged.

The setter and constructor-body additions are comparable value copies and introduce no allocation/copy-site row. The constructor's pre-existing conditional single-attempt allocation remains enumerated exactly once. The request pointer transfer and one result-initialization pointer copy introduce no allocation. The generation key remains one type candidate rather than an anonymous field or duplicate map-key candidate.

No candidate is omitted, conditional, optional, implementation-selected, duplicated, or misclassified within the proposal's closed categories. All eventual canonical classifications remain `UNKNOWN`; this review does not classify them. Any generated C15 extra, missing candidate, category mismatch, location mismatch, or changed AST delta is a hard stop rather than normalization authority.

## 8. Immutable inputs and prior-run boundary

The qualified v3 tests and their private interface signatures remain immutable. No test, corpus, fixture, RED, registry, manifest, digest, sidecar, generated artifact, prior C15 artifact, or qualification input is a mutation target. The reviewed source signatures and current immutable RED require no compatibility edit.

The two earlier disposable paths stopped before source mutation and produced no canonical C15. Their GO/BLOCKED records, candidate lists, and noncanonical runs grant no authority and cannot be reused. This review accepts only the exact current-HEAD closed write-set proposal. A fresh user authorization must separately name one new disposable writer before any prototype mutation.

## 9. Authority boundary and disposition

This review authorizes only proceeding to a separately authorized disposable experiment constrained to the exact three files and closed symbol/candidate set above. It does not authorize source mutation by itself, production mutation, production admission, C15 classification, activation, release, deployment, or production custody.

The mandatory sequence remains:

```text
fresh explicit user authorization naming one disposable writer
→ exact three-file disposable GREEN against immutable v3 RED
→ new canonical C15 enumeration
→ independent complete delta review
→ verified disposal
→ separate later production gate
```

No blocker was found in the exact current-HEAD physical boundary, identity custody, request-result carrier, twelve-refusal closure, Manager registry/retirement race, release lifecycle, detach allocation, A116 ceiling, immutable signatures, or reconciled C15 total inventory.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_V3_GO
```
