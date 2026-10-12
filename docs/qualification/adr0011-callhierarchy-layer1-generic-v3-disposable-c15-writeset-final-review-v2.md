# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review V2

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `83e43a14f3d0ec4f3f02f14eb8ff9930ded98fde`
- Reviewed input: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Supersedes: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final-review.md`
- Review mode: independent documentation-only review against the pinned production declarations and exact current call sites
- Tests, source, prototype, corpus, generated artifacts, and other documents: **not modified or executed**
- Production authority: **not issued**

## Findings

### 1. Prior criteria remain satisfied at the documentation boundary

The corrected write-set retains the coherent parts of the superseded review:

- exactly three proposed production files;
- one explicit enabled attempt pointer threaded through the ordinary/private transaction chain;
- typed finalization of every listed pre-key refusal and admitted completion through `finishRoundTrip`;
- raw request-frame custody from the retained write and raw response-frame custody from `readResult.raw` before parse-derived normalization;
- opaque commitment metadata and defensive predecessor copying without pointer-equality semantics;
- a closed A116 observation/view representation with no B20, C, W24 aggregate, or W25 equality storage;
- finite refusal normalization without failure-string parsing;
- no public API, persistence, replay, telemetry, semantic interpretation, hidden carrier, Manager-global owner, or second publication path; and
- exact `UNKNOWN`-only C15 intent rather than classification or production authority.

The pinned source contains the named request/result attachment, wrapper chain, pending-key allocation, raw read custody, terminal finalizer, runtime-session record, completed STOP/RESTART seams, and shutdown seam. No fourth physical production file is mechanically required for the proposed runtime custody and lifecycle design.

### 2. Corrected lifecycle liveness is coherent

The lifecycle correction resolves the deadlock identified by the superseded review.

#### Shared `releaseLocked` and exact-once release

Both explicit `Release` and teardown `requestTerminal` close admission, drain `active`, and invoke the same `releaseLocked` while holding `owner.mu`. `releaseLocked` is the sole transition that checks `releaseStarted || released`, clears the retained observation, sets both release bits, broadcasts, and reports whether it won. Therefore one explicit/teardown racer performs the release transition exactly once; repeated or losing explicit releases return false.

#### Explicit/teardown race and callback drain

Admission and `active` mutation are serialized by the owner mutex. Once either release path closes admission, no new callback can increment `active`. Every admitted callback has a panic-safe defer that decrements `active` exactly once and broadcasts at zero. Both release paths wait only for the finite set of callbacks admitted before closure. A callback error is preserved, a callback panic is contained, and neither path clears observation custody while an admitted callback remains active.

#### Abandoned owner or lease

Teardown does not require a retained lease or a user `Release`. `requestTerminal` itself performs forced release after callback drain. An ignored result, a never-acquired owner, an abandoned lease, and an abandoned copied view therefore all converge to teardown-owned release. After teardown wins, acquisition, `View`, and `Callback` fail closed.

#### `waitReleased` has no user dependency

`waitReleased` is observational only and is called after `requestTerminal`. Because `requestTerminal` itself guarantees the release transition once admitted callbacks drain, `waitReleased` cannot depend on a future user actor. Its only possible wait is bounded by callbacks already active when admission closes.

#### Ordinary acquirability

Ordinary `RoundTrip` publication does not call `requestTerminal` or `waitReleased`. The returned owner remains open and acquirable until explicit release or actual session/Manager teardown. The corrected teardown rule therefore preserves ordinary post-return acquisition while still guaranteeing forced cleanup.

#### Exact teardown seams

The pinned source provides the proposed integration points:

- completed STOP immediately before `delete(m.sessions, operation.SessionID)`;
- completed RESTART after old-generation retirement and before `m.starter.Start`; and
- `Manager.Shutdown` after `m.workers` reaches zero and before the existing final release sequence.

At each seam, the proposed slice detach under `m.mu`, unlock, `requestTerminal`/`waitReleased` join, and relock avoids holding the Manager mutex while callbacks drain. Failure branches that retain the session do not detach the registry.

The lifecycle correction is therefore not the blocker.

### 3. Blocking exact-call-site and C15-candidate mismatch

The write-set says every changed declaration and call site changes together and §11 claims an exact, closed C15 expected candidate set. The pinned production call sites require two changed enclosing declarations that §11 omits:

1. `(*Manager).AdmitSeedBinding` contains the production call to `m.roundTrip(...)` at `sessionruntime/sessionruntime.go:679`. Adding the proposed `attempt *callHierarchyLayer1GenericAttemptLineage` parameter to `roundTrip` requires this call expression, and therefore this declaration body, to change. `(*Manager).AdmitSeedBinding` is absent from “Existing declarations expected to change.”
2. `(*Manager).RoundTripPrivateB4` contains the production call to `m.roundTripWithPrivateTransaction(...)` at `sessionruntime/b4_definition_private.go:601`. Adding the proposed attempt parameter requires this call expression, and therefore this declaration body, to change. `(*Manager).RoundTripPrivateB4` is absent from “Existing declarations expected to change.”

Passing `nil` at those disabled direct routes does not avoid the declaration changes. Omitting the edits does not compile; making the edits creates `CHANGED` declaration candidates outside the exact closed set. Section 11 declares either outcome a hard stop.

This also means the claim that the exact proposed signatures cover “all current call sites” is false at the pinned HEAD.

### 4. Blocking immutable-test call-site contradiction

Current immutable package tests directly invoke the private methods whose signatures the write-set requires replacing:

- `roundtrip_test.go:176` calls `m.roundTrip(...)`;
- `b4_source_accounting_c13_c14_private_test.go:150`, `:182`, `:205`, and `:220` call `m.roundTripWithPrivate(...)`;
- `adr0011_c08_preflight_red_private_test.go:101`, `adr0011_c08_lifecycle_private_test.go:78`, and `adr0011_c07_integrated_private_test.go:85` call `m.roundTripWithPrivateTransaction(...)`.

Go does not support method overloading or optional parameters. Replacing the three method signatures exactly as proposed makes these immutable callers fail to compile. Updating them violates the document's immutable-test and three-production-file boundaries. Retaining compatibility methods or adding alternate helpers violates the exact proposed symbol/signature set and closed C15 candidate set.

The mandatory experiment requires qualified v3 GREEN without test edits. The current write-set does not provide a legal compiling route to that state.

## Required correction

Before a disposable writer can be named, the write-set must be revised and independently reviewed to:

1. account for every production declaration changed solely because of signature threading, including at least `(*Manager).AdmitSeedBinding` and `(*Manager).RoundTripPrivateB4`, in the exact C15 candidate set; and
2. provide a compile-valid attempt-threading design compatible with all immutable existing test call sites, without an unlisted helper, alternate carrier, second builder, fourth production file, or test mutation.

The lifecycle correction should be preserved: shared `releaseLocked`, exact-one release winner, admission closure before callback drain, teardown-forced release for abandoned owners/leases, observational `waitReleased`, and ordinary post-return acquirability.

## Disposition

```text
BLOCKED — proposed private signature changes require unlisted production declaration changes in AdmitSeedBinding and RoundTripPrivateB4, and they cannot compile with immutable existing direct test callers within the exact three-file/symbol/C15 boundary
```

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_BLOCKED
```

Disposition: **BLOCKED / NO GO**
