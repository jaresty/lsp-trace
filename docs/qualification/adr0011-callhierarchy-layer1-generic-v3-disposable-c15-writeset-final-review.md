# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `83e43a14f3d0ec4f3f02f14eb8ff9930ded98fde`
- Reviewed input: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Review mode: independent documentation-only review against the pinned production declarations
- Tests, source, prototype, corpus, generated artifacts, and other documents: **not modified or executed**
- Production authority: **not issued**

## Findings

### 1. Exact physical boundary

The proposed causal plumbing is located in exactly the three named existing production files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

At the pinned HEAD, those files contain the request/result attachment, ordinary/private wrapper chain, pending-key allocation, raw read custody, terminal finalizer, existing immutable-owner pattern, completed STOP/RESTART teardown points, and shutdown point needed by the proposal. No fourth physical file is mechanically indicated by the reviewed design. The boundary is sufficient and minimal as a proposed file boundary.

### 2. Closed production representation and A116 ceiling

The proposed private types enumerate every field and exact Go type. The observation and view close over the A116 projections listed in §8; no B20 field, C control/fixture/organization field, W24 root aggregate, or W25 equality field/helper is admitted. The mapping explicitly distinguishes unavailable, available-empty, and available-known-zero states and preserves raw response-frame custody before parse-derived normalization.

The refusal enum is finite, the twelve ordered refusal branches are typed without failure-string parsing, and disabled requests carry no generic owner. Commitment metadata remains opaque; selected, handoff, and predecessor preimages are not admitted.

This portion is closed and contains no open semantic allowance.

### 3. Closed C15 candidate proposal

Section 11 uses exact declarations, exact changed declarations, exact new functions/methods, and exact allocation/copy descriptions; it explicitly says the set is not a lower bound and requires every classification to remain `UNKNOWN`. It supplies hard stops for every extra or missing candidate and does not authorize implementation-side normalization.

The set therefore has no “minimum” ambiguity at the documentation boundary. Canonical mechanical agreement remains a later disposable-experiment result and is not classified or accepted by this review.

### 4. Lineage, refusal, wrappers, enablement, predecessor custody, and raw custody

The proposal closes the causal chain as one enabled-bit read at the first executable `RoundTrip` line, one attempt pointer, explicit threading through every ordinary/private wrapper, admission immediately after `pending.Begin`, typed refusal finalization before every pre-key return, admitted completion through `finishRoundTrip`, one owner publication, and no hidden carrier or second builder.

Predecessor metadata is copied into an immutable comparable holder and copied again at owner/view boundaries; pointer identity has no metadata meaning. Request-frame custody comes from retained writer output, response-frame custody comes from `readResult.raw` before normalization, and result/error custody is captured before private clearing. No semantic parsing or inferred authority is admitted.

### 5. Blocking lifecycle liveness defect

The proposed lifecycle cannot establish the required no-deadlock property.

The final write-set requires all of the following simultaneously:

1. ordinary `RoundTrip` returns the owner open and acquirable;
2. only the winning explicit `callHierarchyLayer1GenericLease.Release` sets `releaseStarted`, waits for active callbacks, clears the observation, and sets `released=true`;
3. teardown-owned `requestTerminal` only closes admission and explicitly does **not** start release, clear custody, or set `released`;
4. teardown-owned `waitReleased` waits until `released==true`; and
5. completed STOP, completed RESTART, and `Manager.Shutdown` synchronously call `requestTerminal` followed by `waitReleased`.

No actor is guaranteed to call explicit `Release`. In particular, an enabled `RoundTripResult` may be ignored, may never acquire a lease, or may be abandoned without releasing its acquired lease. Once teardown calls `requestTerminal`, new acquisition fails closed, so an owner for which no lease was previously retained can no longer obtain the only object whose `Release` method performs the `released=true` transition. The teardown goroutine then waits forever. The same failure exists when a previously acquired lease is abandoned.

This is not merely an implementation choice: §§5.3, 6.2, and 10 expressly forbid `requestTerminal` from completing release and forbid any alternate release winner. Consequently the finite authorized state machine contains a reachable state with no transition to `released`:

```text
owner open, active=0, no retained lease
  -> teardown requestTerminal
  -> admissionClosed=true, released=false
  -> acquisition fails closed
  -> waitReleased waits forever
```

The pinned source confirms that the proposed waits occur on lifecycle-completion and shutdown paths. Blocking there can prevent STOP completion, RESTART successor startup, and `Shutdown` return.

The write-set must be revised and independently reviewed to name an exact bounded ownership rule that guarantees a release transition without weakening callback drain, single-winner behavior, ordinary post-return acquisition, or teardown closure. That revision will necessarily change the currently closed lifecycle semantics and may change the exact C15 function/allocation candidate set; this review cannot invent the correction.

## No-go boundary

The three-file boundary, closed A116 representation, finite refusal table, exact lineage, predecessor/raw custody, and closed `UNKNOWN` C15 proposal are otherwise coherent. They do not cure the lifecycle deadlock, and this review does not broaden any allowance to do so.

```text
BLOCKED — teardown can wait forever because requestTerminal closes acquisition without performing release, while explicit lease Release is the sole transition to released and no release actor is guaranteed
```

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_BLOCKED
```

Disposition: **BLOCKED / NO GO**
