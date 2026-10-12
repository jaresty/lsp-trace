# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review V3

Status: **GO for the finite disposable write-set boundary only**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `83e43a14f3d0ec4f3f02f14eb8ff9930ded98fde`
- Reviewed input: `adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`
- Supersedes the blocked v1 and v2 final reviews.
- Source, tests, corpus, and prototypes were not modified or executed.

## Findings

The finite mutation boundary is exactly:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

No fourth production file, helper-signature change, compatibility overload, global state, side table, or test mutation is required.

The qualified carrier is exact:

```text
RoundTrip overwrites private request carrier
→ unchanged by-value request chain
→ transaction copies pointer once into transient result carrier
→ every admitted terminal path reaches finishRoundTrip
→ finishRoundTrip finalizes and publishes immutable owner
→ transient result carrier is cleared before return
```

The two transient carrier fields are:

```go
RoundTripRequest.callHierarchyLayer1GenericAttempt *callHierarchyLayer1GenericAttemptLineage
RoundTripResult.callHierarchyLayer1GenericAttempt  *callHierarchyLayer1GenericAttemptLineage
```

The immutable publication field is separate:

```go
RoundTripResult.callHierarchyLayer1Generic *callHierarchyLayer1GenericOwner
```

All existing helper signatures and direct callers remain unchanged. `AdmitSeedBinding`, `RoundTripPrivateB4`, and immutable direct tests require no edits. All twelve pre-key refusal branches finalize directly from the request carrier. Every admitted branch carries the transient result through `finishRoundTrip`; no post-key bypass was found.

The representation is closed over A116 only. It admits no B20, C, World-24 aggregate, World-25 equality, semantic parsing, or deferred body/payload storage. The exact C15 candidate set is closed; all classifications remain `UNKNOWN`, and every extra or missing candidate is a hard stop.

Lifecycle liveness is coherent. Explicit `Release` and teardown `requestTerminal` close admission, drain admitted callbacks, and race through the same exact-once `releaseLocked` transition. Teardown therefore force-releases ignored or abandoned owners and leases without depending on a user actor. `waitReleased` is observational after `requestTerminal`; ordinary `RoundTrip` leaves the owner open and acquirable until explicit release or actual teardown.

The exact teardown seams are completed STOP, completed RESTART before successor start, and `Manager.Shutdown` after workers reach zero. Owner slices are detached under the Manager mutex and drained outside it.

No operative stale explicit-parameter requirement remains. Historical parameter-threading text is retained only as superseded history or an explicit hard stop.

## Authority boundary

This review authorizes only the finite disposable write-set boundary. It does not authorize production mutation, production admission, C15 classification, activation, release, deployment, or directional CALLS qualification.

The remaining sequence is:

```text
one disposable writer
→ v3 GREEN
→ canonical C15 enumeration
→ independent finite-delta review
→ dispose worktree
→ later production gate
```

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_GO
```
