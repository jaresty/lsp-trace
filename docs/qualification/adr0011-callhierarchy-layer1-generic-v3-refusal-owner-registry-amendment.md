# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Refusal-Owner Registry Amendment

Status: **REVIEW_REQUIRED / NO GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Pinned HEAD: `b283d7ea6cf3d4da71cb0bed75517b9995d1922a`
- Required starting state: exact clean worktree at the pinned HEAD
- Scope: documentation-only correction of the generic-owner publication and teardown registry proposed by `adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Mutation authorized by this task: this document only
- Source, tests, corpus, RED, prototype, generated artifacts, execution, commit, activation, classification, production write-set, production writer, and production authority: **not issued**

## 1. Hard stop resolved and narrow supersession

The prior session-only registry cannot retain enabled pre-admission refusal owners for the two branches in which the requested session is absent or stale. A refusal owner published before admission is still required to remain open and acquirable through the returned `RoundTripResult`; placing only admitted owners in `runtimeSession.callHierarchyLayer1GenericOwners` leaves those valid refusal owners outside lifecycle teardown.

This amendment supersedes only the following requirements of the prior final write-set and its reviews:

1. `runtimeSession.callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner`;
2. publication through a free `publishCallHierarchyLayer1GenericOwner` function that registers only against `m.sessions[id]`;
3. STOP, RESTART, and Shutdown collection from per-session owner slices; and
4. the corresponding `runtimeSession` field, declaration mutation, allocation sites, and C15 candidates.

All other qualified-v3 RED behavior remains open and acquirable, including enabled pre-admission refusals, the exact twelve refusal rows, admitted finalization through `finishRoundTrip`, immutable owner/view/lease behavior, explicit `Release`, callback drain, and fail-closed access after release. No fourth production file, signature change to an existing helper, test change, or prototype is authorized here.

## 2. Exactly one Manager-owned owner registry and one finite retired-generation set

The exact comparable generation key and the only two proposed Manager fields are:

```go
type callHierarchyLayer1GenericGenerationKey struct {
    sessionID  string
    generation uint64
}

// in Manager; both guarded exclusively by m.mu
callHierarchyLayer1GenericOwners            []*callHierarchyLayer1GenericOwner
callHierarchyLayer1GenericRetiredGenerations map[callHierarchyLayer1GenericGenerationKey]struct{}
```

`callHierarchyLayer1GenericGenerationKey` is private, contains exactly the existing comparable session ID type (`string`) and generation type (`uint64`), and is compared by Go value equality. The set is initialized lazily or in the existing Manager construction path, contains only exact generations committed retired by completed STOP or RESTART, never uses a wildcard/session-only key, and persists until Manager Shutdown clears it. A successor generation has a different key and is therefore unaffected. Its size is finite over the finite lifetime of one Manager; no process-global or cross-Manager retention is admitted.

The finite owner slice contains every successfully retained generic owner exactly once:

- every enabled pre-admission refusal owner, including session-absent and stale-generation refusals;
- every enabled admitted owner finalized by `(*Manager).finishRoundTrip`; and
- no disabled request, nil attempt, unpublished owner, duplicate pointer, registry-entry wrapper, map entry, session bucket, generation bucket, or secondary index.

The owner already carries immutable request identity, including session, generation, and request identity. Registry selection uses that immutable owner identity; it must not consult the current `m.sessions` entry to reconstruct or overwrite identity. No identity field, registry-entry type, owner back-pointer, package global, context value, or side table is added.

The slice is append-only between lifecycle detach operations. Its backing and entries are private. Every owner-registry read, append, filter, replacement, or nil assignment and every retired-set read, insert, clear, or replacement occurs while holding `m.mu`.

Add exactly this lock-required publication helper:

```go
func (m *Manager) publishCallHierarchyLayer1GenericOwnerLocked(
    result RoundTripResult,
    owner *callHierarchyLayer1GenericOwner,
) (RoundTripResult, *callHierarchyLayer1GenericOwner)
```

The caller holds `m.mu`. The helper assigns `owner` to `result.callHierarchyLayer1Generic`, clears `result.callHierarchyLayer1GenericAttempt`, and computes `key` from the owner's immutable session ID and generation. If `m.closed` is true or `key` is present in `m.callHierarchyLayer1GenericRetiredGenerations`, it does not append and returns `(result, owner)`; otherwise it appends exactly once and returns `(result, nil)`. The second result is an exact outside-lock terminal action, not an error or ownership transfer: after unlocking `m.mu`, the caller must immediately call `requestTerminal()` and then `waitReleased()` on that pointer before returning the `RoundTripResult`. Neither the helper nor any caller may invoke an owner callback, callback drain, `requestTerminal`, or `waitReleased` while holding `m.mu`.

## 3. Exact pre-admission refusal finalizer

Replace the prior attempt-only refusal publication path and free publication helper with exactly this Manager method:

```go
func (m *Manager) finishCallHierarchyLayer1GenericRefusal(
    attempt *callHierarchyLayer1GenericAttemptLineage,
    result RoundTripResult,
    kind callHierarchyLayer1GenericRefusalType,
    reason session.Failure,
) RoundTripResult
```

The method owns its lock discipline; callers must not hold `m.mu`. Its behavior is exact:

1. If `attempt == nil`, return `result` unchanged without locking, allocation, owner construction, publication, or registry mutation.
2. Lock `m.mu`.
3. While still holding `m.mu`, call `attempt.recordRefusal(kind, reason)`, freeze exactly one refusal observation, construct exactly one `callHierarchyLayer1GenericOwner`, and call `publishCallHierarchyLayer1GenericOwnerLocked(result, owner)`.
4. Unlock `m.mu`.
5. If the helper returned a non-nil terminal-action owner, immediately call `requestTerminal()` and then `waitReleased()` on it, in that order, before returning `result`; otherwise perform no owner lifecycle action.
6. Return `result` with all existing public result/failure behavior unchanged. A retained owner is open/acquirable. A closed/retired-gated owner is returned already released and therefore fails closed on acquisition.

For an enabled attempt, refusal recording, observation freeze, owner construction, result publication, transient-carrier clearing, and the retain-versus-terminal decision are one atomic Manager critical section. No owner pointer becomes externally observable before either registry append or commitment to the exact outside-lock terminal action. No registry entry exists without the same finalized owner being assigned to the returned result. The method does not call `finishRoundTrip`, `releaseLocked`, pending accounting, worker accounting, transport accounting, ledger decrement, `pending.End`, worker decrement, session poisoning, or any public behavior path. Its only terminal calls are the required immediate outside-lock `requestTerminal`/`waitReleased` pair.

The attempt remains single-finalization: a second freeze/publication is forbidden and is a hard stop rather than a second owner.

## 4. Exact twelve pre-admission call sites

Each of the following twelve enabled pre-key return branches must call `m.finishCallHierarchyLayer1GenericRefusal(...)` exactly once, in current evaluation order, with the listed enum and typed reason:

| # | Current branch | `kind` | `reason` | Existing public behavior preserved |
|---:|---|---|---|---|
| 1 | `req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting` | `callHierarchyLayer1GenericRefusalPrivateAccountingNotImplemented` | `session.Failure("private-accounting-not-implemented")` | `ToolNotImplemented` |
| 2 | C18 runtime gate not reached | `callHierarchyLayer1GenericRefusalRuntimeGate` | existing typed `ToolNotImplemented` branch value | unchanged |
| 3 | `contextFailure(parent) != ""` | `callHierarchyLayer1GenericRefusalContext` | exact typed context failure | unchanged |
| 4 | session absent | `callHierarchyLayer1GenericRefusalSession` | `SessionNotFound` | unchanged |
| 5 | generation mismatch | `callHierarchyLayer1GenericRefusalGeneration` | `StaleGeneration` | unchanged |
| 6 | not Ready or protocol-owned | `callHierarchyLayer1GenericRefusalLifecycle` | `LifecycleConflict` | unchanged |
| 7 | private P2 cannot start before deadline | `callHierarchyLayer1GenericRefusalDeadline` | `RequestTimeout` | unchanged |
| 8 | owned document mismatch | `callHierarchyLayer1GenericRefusalDocument` | `DocumentSupplyUnavailable` | unchanged |
| 9 | process is not `wireChild` | `callHierarchyLayer1GenericRefusalProcess` | `SessionPoisoned` | unchanged |
| 10 | request/worker limit exceeded | `callHierarchyLayer1GenericRefusalCapacity` | `ResourceExhausted` | unchanged |
| 11 | readiness-history reader/borrower failure | `callHierarchyLayer1GenericRefusalPrivateAdmission` | `ResourceExhausted` | unchanged |
| 12 | `admitPrivateB4SourcesC17Locked` failure | `callHierarchyLayer1GenericRefusalPrivateAdmission` | exact typed returned failure | unchanged |

At call sites currently holding `m.mu`, the caller must unlock before invoking the lock-owning finalizer. No effect may be introduced into that unlock/relock boundary: the branch is already terminal and pre-admission, no key has been allocated, and no pending, worker, protocol, transport, reservation, or accounting ownership may have begun. The finalizer returns directly; callers do not relock after it.

Failure-string parsing, attempt reconstruction, transient result-carrier use, `finishRoundTrip`, or a second publication helper is forbidden at all twelve sites. Any enabled pre-key return outside this exact set, or any listed branch that cannot invoke this method without an intervening effect, is a hard stop requiring write-set revision and independent review.

## 5. Admitted publication through `finishRoundTrip`

The existing signature remains byte-for-byte unchanged:

```go
func (m *Manager) finishRoundTrip(
    id string,
    owner *ownedTransport,
    result RoundTripResult,
    failure session.Failure,
    poison bool,
) RoundTripResult
```

`finishRoundTrip` remains the sole admitted terminal/error/result/accounting/ledger finalizer. While already holding `m.mu`, and only after current terminal mapping and after-snapshot normalization, it freezes the admitted attempt, constructs exactly one generic owner, and calls `publishCallHierarchyLayer1GenericOwnerLocked(result, owner)`. Its existing signature does not change. It completes all existing Manager-locked accounting, records the returned terminal-action pointer in a local, unlocks `m.mu` at its existing handoff, immediately performs `requestTerminal()` then `waitReleased()` outside `m.mu` when that pointer is non-nil, and only then returns the result. It must neither reacquire `m.mu` for the owner action nor run any owner callback or callback drain under `m.mu`.

Admitted publication does not require a current session-registry entry after owner identity has been frozen, does not append to `runtimeSession`, and does not call the pre-admission finalizer. In the ordinary non-closed, non-retired case the helper appends and no terminal action occurs, so the owner remains open and acquirable until explicit release or matching lifecycle teardown. If Shutdown closure or matching retirement won the `m.mu` serialization before this late admitted publication, the helper refuses the append and `finishRoundTrip` returns only after the owner is terminal/released.

## 6. Exact lifecycle detach helper

Add exactly this private helper in `sessionruntime/sessionruntime.go`:

```go
func (m *Manager) detachCallHierarchyLayer1GenericOwnersLocked(
    sessionID string,
    generation uint64,
    allGenerations bool,
) []*callHierarchyLayer1GenericOwner
```

The caller must hold `m.mu`. The helper performs a stable single pass over `m.callHierarchyLayer1GenericOwners`:

- if `allGenerations` is true, detach every owner whose immutable request identity session equals `sessionID`, regardless of generation;
- otherwise detach every owner whose immutable request identity session equals `sessionID` and generation equals `generation`;
- preserve registry order in both the detached and retained slices;
- replace the Manager field with the retained slice while still under `m.mu`;
- return the detached owner pointers; and
- perform no owner lock, terminal request, wait, goroutine creation, accounting mutation, session lookup, or public behavior change.

For `Manager.Shutdown`, the exact all-owner detach is represented by:

```go
owners := m.callHierarchyLayer1GenericOwners
m.callHierarchyLayer1GenericOwners = nil
```

under `m.mu`; Shutdown does not call the selector once per session.

## 7. STOP, RESTART, and Shutdown ordering

For every completed lifecycle path, retirement marking plus selection/removal occur under `m.mu`; release and join occur outside `m.mu`; existing lifecycle mutation resumes only after `m.mu` is reacquired. The exact handoff is: mutate retirement state and detach while locked; store detached pointers locally; unlock once; call each owner's `requestTerminal()` and `waitReleased()` in registry order with no Manager lock held; relock once; continue existing lifecycle state mutation. No owner callback, callback drain, terminal request, or release wait may execute under `m.mu`.

### 7.1 Completed STOP

A completed STOP for an existing session retires its exact current generation and detaches all generations for `operation.SessionID`:

1. while holding `m.mu`, form `callHierarchyLayer1GenericGenerationKey{sessionID: operation.SessionID, generation: currentGeneration}` and insert it into `m.callHierarchyLayer1GenericRetiredGenerations` before any owner detach;
2. still holding `m.mu`, call `detachCallHierarchyLayer1GenericOwnersLocked(operation.SessionID, 0, true)`;
3. unlock `m.mu`;
4. for each detached owner in registry order, call `owner.requestTerminal()` and then `owner.waitReleased()`;
5. relock `m.mu`;
6. perform the existing completed-STOP session deletion and completion sequence.

A STOP that returns `SessionNotFound` has no current generation to retire, does not insert a tombstone, does not detach or drain absent-session owners, and must not claim that it did. Such owners remain retained until Manager Shutdown, explicitly bounded by that Manager's lifetime.

All generations are selected because absent/stale refusal owners may name the stopped session while not belonging to its currently admitted `runtimeSession` generation.

### 7.2 Completed RESTART

A completed RESTART detaches only the retired old generation:

1. after old-generation retirement is committed and before successor startup, while holding `m.mu`, insert `callHierarchyLayer1GenericGenerationKey{sessionID: operation.SessionID, generation: oldGeneration}` into `m.callHierarchyLayer1GenericRetiredGenerations` before any owner detach;
2. still holding `m.mu`, call `detachCallHierarchyLayer1GenericOwnersLocked(operation.SessionID, oldGeneration, false)`;
3. unlock `m.mu`;
4. for each detached owner in registry order, call `owner.requestTerminal()` and then `owner.waitReleased()`;
5. relock `m.mu`;
6. call the existing successor-start sequence.

Owners for other generations, including a later generation if one can already be represented, remain registered. The successor does not initialize a session registry because none exists.

### 7.3 Manager Shutdown

`(*Manager).Shutdown` detaches all remaining owners, including owners for absent session IDs and stale generations:

1. after existing worker drain reaches zero and while holding `m.mu`, assign the full Manager slice to a local `owners`, set `m.callHierarchyLayer1GenericOwners = nil`, and clear the retired set only after `m.closed` is durably true under the same lock;
2. unlock `m.mu`;
3. for every detached owner in registry order, call `owner.requestTerminal()` and then `owner.waitReleased()`;
4. relock `m.mu`;
5. continue the existing shutdown-owned release/finalization sequence and final unlock.

The amendment does not assume current source has an `m.closed` admission check in `RoundTrip`. A `RoundTrip` that continues concurrently with or starts after Shutdown may still reach either finalizer. Its publication helper observes `m.closed` under `m.mu`, does not append, and returns the exact terminal-action owner; the finalizer unlocks, immediately requests terminal state, waits for release, and only then returns the owner-bearing result. Thus no post-Shutdown result can leave an unjoined owner: it may be returned already released/fail-closed under concurrent or completed teardown. In nonconcurrent RED worlds this gate is not reached and expected open/acquirable behavior is unchanged; therefore the existing nonconcurrent RED impact is **none**.

### 7.4 Failure branches and publication-after-detach rule

STOP or RESTART branches that fail before completed lifecycle retirement do not detach owners: the session/lifecycle remains active or poisoned under existing behavior, and its owners remain open/acquirable and registered. If failure occurs only after detach and after the outside-lock joins, detached owners remain terminal/released even if successor startup or later lifecycle completion fails; they must not be reinserted because the retired lifecycle boundary already occurred.

For completed STOP and RESTART, correctness does not depend on lifecycle exclusion preventing late publication. A publication serialized before retirement marking may append and is then selected by the detach. A matching publication serialized after marking hits the exact retired-generation key, does not append, and is terminalized/joined outside `m.mu` before return. This applies both to the lifecycle-conflict refusal reached after session detach and to an admitted request that finishes late. A request for a genuinely new successor generation has a different key and is unaffected.

No owner may be appended concurrently with a matching detach because publication, retirement marking, and detach serialize under `m.mu`. The persistent tombstone closes the former after-detach gap without holding `m.mu` during owner drain.

## 8. Explicit Release and retention bound

`callHierarchyLayer1GenericLease.Release` does not remove the owner pointer from `m.callHierarchyLayer1GenericOwners`. Immediate removal would require a Manager back-pointer or a second Manager mutation path and would make release/teardown races more complex. Neither is admitted.

Explicit release still closes owner admission, drains active callbacks, and invokes `releaseLocked` exactly once. The registry retains only the released owner pointer; `releaseLocked` has already cleared the retained observation and mutable evidence custody. Later teardown calls `requestTerminal` and `waitReleased` idempotently on that owner and completes immediately. Repeated explicit release returns false as before.

Registry pointer retention is bounded by lifecycle teardown:

- matching completed STOP for an existing session removes all generations for the session ID;
- matching completed RESTART removes the retired generation;
- absent-session owners are retained until Manager Shutdown; `SessionNotFound` STOP performs no detach and claims no drain;
- Manager Shutdown removes all entries, including absent/stale-session refusals, so their maximum retention is the finite Manager lifetime; and
- there is no time-based cleanup, opportunistic compaction, result-GC inference, finalizer, or immediate-release deletion.

## 9. Exact absent/stale and Shutdown behavior

For an enabled request naming an absent session ID, the session-refusal owner is atomically published into the Manager registry and returned open/acquirable unless `m.closed` already requires immediate outside-lock terminalization. It is not rejected merely because `m.sessions[sessionID]` is absent. A STOP returning `SessionNotFound` does not detach it; Manager Shutdown detaches it. This retention is explicitly bounded by the Manager lifetime.

For an enabled request naming a stale generation whose exact key has not been retired by this Manager, the generation-refusal owner is atomically published with that immutable stale generation and returned open/acquirable. Once a completed STOP or RESTART marks that exact key retired, a matching stale-generation refusal serialized after the mark is not appended and is returned only after outside-lock terminalization/join. Stale owners for other, unmarked generations remain registered until a completed STOP for an existing session detaches the session's owners or Manager Shutdown consumes all retained owners.

During or after Manager Shutdown, existing public shutdown/lifecycle failure classification remains controlling. If an enabled pre-admission refusal or admitted completion reaches publication after `m.closed` is true, publication is permitted only as result attachment plus refusal to append: the finalizer must terminalize and join the owner outside `m.mu` before returning. Disabled requests never publish. The returned owner is released/fail-closed rather than leaked or falsely retained.

## 10. Exact amended C15 candidate delta — all `UNKNOWN`

This section describes the expected amendment to the prior closed candidate set. It is not a C15 result. Every added, removed, or changed candidate remains **`UNKNOWN`**.

### 10.1 Field candidates

Remove/supersede:

```text
runtimeSession.callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner — UNKNOWN
```

Add:

```text
Manager.callHierarchyLayer1GenericOwners []*callHierarchyLayer1GenericOwner — UNKNOWN
Manager.callHierarchyLayer1GenericRetiredGenerations map[callHierarchyLayer1GenericGenerationKey]struct{} — UNKNOWN
```

No other field candidate is added or removed.

### 10.2 Type candidates

Add:

```text
callHierarchyLayer1GenericGenerationKey struct{ sessionID string; generation uint64 } — UNKNOWN
```

No other type candidate is added or removed.

### 10.3 Helper/method candidates

Remove/supersede:

```text
(*callHierarchyLayer1GenericAttemptLineage).finishRefusal(RoundTripResult) RoundTripResult — UNKNOWN
publishCallHierarchyLayer1GenericOwner(RoundTripResult, *callHierarchyLayer1GenericOwner) RoundTripResult — UNKNOWN
```

Add exactly:

```text
(*Manager).finishCallHierarchyLayer1GenericRefusal(*callHierarchyLayer1GenericAttemptLineage, RoundTripResult, callHierarchyLayer1GenericRefusalType, session.Failure) RoundTripResult — UNKNOWN
(*Manager).publishCallHierarchyLayer1GenericOwnerLocked(RoundTripResult, *callHierarchyLayer1GenericOwner) (RoundTripResult, *callHierarchyLayer1GenericOwner) — UNKNOWN
(*Manager).detachCallHierarchyLayer1GenericOwnersLocked(string, uint64, bool) []*callHierarchyLayer1GenericOwner — UNKNOWN
```

`(*Manager).finishRoundTrip` remains an existing changed body with an unchanged signature — `UNKNOWN`.

### 10.4 Existing declaration/body candidates

Remove/supersede:

```text
runtimeSession — UNKNOWN
```

Add or retain as changed:

```text
Manager — UNKNOWN
(*Manager).RoundTrip — body only; signature unchanged — UNKNOWN
(*Manager).roundTripWithPrivateTransaction — body only; signature unchanged — UNKNOWN
(*Manager).finishRoundTrip — body only; signature unchanged — UNKNOWN
(*Manager).runLifecycle — body only; signature unchanged — UNKNOWN
(*Manager).Shutdown — body only; signature unchanged — UNKNOWN
```

The exact twelve refusal call-site edits listed in §4 are body changes within the existing qualified request path; each is `UNKNOWN`. No edit to `(*Manager).AdmitSeedBinding`, `(*Manager).RoundTripPrivateB4`, direct helper-call tests, or `sessionruntime/b4_definition_private.go` is admitted.

### 10.5 Allocation/copy candidates

Remove/supersede:

```text
append one owner pointer to runtimeSession.callHierarchyLayer1GenericOwners — UNKNOWN
STOP copy of runtimeSession owner slice — UNKNOWN
RESTART copy of runtimeSession owner slice — UNKNOWN
Shutdown gather from all runtimeSession owner slices — UNKNOWN
```

Add exactly:

```text
pre-admission finalizer append of exactly one owner pointer to Manager.callHierarchyLayer1GenericOwners; only append-induced Manager-slice growth — UNKNOWN
finishRoundTrip append of exactly one admitted owner pointer to Manager.callHierarchyLayer1GenericOwners; only append-induced Manager-slice growth — UNKNOWN
publishCallHierarchyLayer1GenericOwnerLocked append only for non-closed, non-retired owners; no append for terminal-action return — UNKNOWN
detachCallHierarchyLayer1GenericOwnersLocked stable retained-slice backing, if required by the implementation selected in the disposable run — UNKNOWN
retired-generation exact-key map initialization and insert growth — UNKNOWN
Manager.Shutdown direct whole-slice detach and retired-set clear; no owner-slice copy allocation required — UNKNOWN
```

The final exact allocation shape for stable filtering is not accepted by this document; canonical C15 must report whether in-place stable compaction or one retained backing allocation is used. The exact retired-generation map above is the sole admitted map addition. Any other map, registry-entry, wrapper, goroutine, channel, owner, view, lease, callback, or error allocation remains outside the candidate set and is a hard stop.

### 10.6 Write-set impact

The physical candidate write-set remains exactly three existing production files:

1. `sessionruntime/sessionruntime.go` — exact generation-key type, two Manager fields, dedicated refusal finalizer, locked publication helper, twelve refusal call sites, admitted publication gate, detach helper, retirement marking, and STOP/RESTART/Shutdown lifecycle integration;
2. `sessionruntime/owned_method_pair.go` — unchanged from the prior qualified finite boundary; and
3. `sessionruntime/b4_owned_method_pair_lease.go` — owner identity/freeze/lifecycle symbols remain within the prior qualified finite boundary; no Manager back-pointer or registry-entry type.

No fourth source file, test file, corpus, fixture, generated artifact, schema, CLI/MCP file, or additional qualification document is part of the proposed write-set. If compilation or the disposable C15 run requires one, stop and revise/review this amendment.

## 11. Preserved authority ceiling and hard stops

This amendment does not establish feature identity, semantic meaning, source completeness, producer authentication, provider authenticity, qualification, publication authority, completeness, production safety, or production use. It changes no public API, result/error semantics, transport, retries, persistence, replay, telemetry, diagnostics, configuration, activation, CLI/MCP/schema behavior, or `internal/lspwire` behavior.

Hard-stop before C15 if any implementation requires:

- an owner registry anywhere except the single Manager slice;
- any map except the exact private retired-generation set, or any registry-entry type, owner back-pointer, session registry, package global, or second index;
- publication outside the dedicated pre-admission finalizer or admitted `finishRoundTrip`;
- fewer or more than the exact twelve pre-admission refusal call sites;
- pending, worker, protocol, transport, reservation, or accounting decrement on pre-admission refusal;
- making an owner-bearing result externally observable before the same `m.mu` critical section has either appended the owner or committed the non-nil outside-lock terminal action;
- holding `m.mu` while an owner callback, `requestTerminal`, callback drain, or `waitReleased` runs;
- immediate registry deletion by explicit `Release`;
- appending an owner when `m.closed` is true or its exact generation key is retired;
- returning a closed/retired-gated owner before its outside-lock `requestTerminal`/`waitReleased` action completes;
- a fourth production file, helper-signature change, test mutation, or C15 candidate outside the independently reviewed finite set.

## 12. Required next sequence

This document remains review input only. The required order is:

1. independently review this exact amendment, including the twelve call sites, exact new helper signatures, closed/retired publication dispositions, retirement-before-detach ordering, outside-lock handoff, absent-session Manager-lifetime bound, failure ordering, and C15 delta;
2. update and freeze the final finite disposable write-set to supersede the session-registry requirements;
3. name exactly one disposable writer in a new clean disposable worktree at the reviewed HEAD;
4. obtain qualified v3 GREEN without changing RED/tests/corpus;
5. run a **new** canonical disposable C15 enumeration and preserve the exact patch, candidates, locations, AST hashes, and complete `ADDED`/`REMOVED`/`CHANGED` delta, all initially `UNKNOWN`;
6. independently review that new finite delta and the final write-set together;
7. dispose of the prototype and verify disposal; and
8. only then consider a separate production write-set and writer gate.

The prior disposable run, if any, cannot be reused as evidence for this changed registry/lock/lifecycle candidate set.

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_REFUSAL_OWNER_REGISTRY_AMENDMENT — REVIEW_REQUIRED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_UPDATE — REQUIRED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_NEW_DISPOSABLE_RUN — REQUIRED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITESET_GO — NOT ISSUED
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_PRODUCTION_WRITER — NOT NAMED
```

Disposition: **REVIEW_REQUIRED / NO GO**
