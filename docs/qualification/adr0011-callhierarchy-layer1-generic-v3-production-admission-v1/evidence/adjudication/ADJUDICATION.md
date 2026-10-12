# ADR-0011 C15 ALL33 Semantic Adjudication

- Schema: `lsp-trace/adr0011-c15-semantic-adjudication-v1`
- Base: `ce4a1838f8a10bafc13429031e55df7e4dd812c6`
- Patch SHA-256: `e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0`
- Counts: total=33 NECESSARY=28 REUSE=4 UNRESOLVED=1
- All33 unique equality: `True` (33 IDs, 33 unique)
- Disposition: `C15_DELTA_BLOCKED_UNRESOLVED_1`
- Proposed `C15_DELTA_GO`: not emitted because unresolved != 0.

## Evidence ceiling
Static adjudication of the immutable baseline, accepted patch, and enumeration evidence only. It does not prove runtime frequency, allocator choices, production execution, or unknown slice capacity.

## Rows

### 1. `alloc.b4.owned.method.pair.lease.admit.builtin-make.make.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:549:12-549:27`; AST `93e3af7d1f4df6e8ed36c89ef8c6aebcd94cd54ae1235a8ff373208cd1bebad1` (builtin-make)
- Allocation: `make([]byte, 8)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 2. `alloc.b4.owned.method.pair.lease.captureresult.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:576:53-576:92`; AST `e4f0159479758e4db5adb4d41fa92a902df13e5f353d2e0bcfa8da1ac1352330` (builtin-append)
- Allocation: `append(json.RawMessage(nil), result...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 3. `alloc.b4.owned.method.pair.lease.captureresult.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:579:12-579:61`; AST `5b5e5a627193866f7c655c46d3e70304fa5e27712f0c91bb001976399e18b7e0` (builtin-append)
- Allocation: `append(json.RawMessage(nil), serverError.Data...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 4. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:584:31-584:114`; AST `8aa5d1a70547d4d1578faa2ceb288571174c4d91a8ff565c55812067d3aec273` (builtin-append)
- Allocation: `append([]callHierarchyOpaqueCommitmentMetadata(nil), o.commitments.predecessors...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 5. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:585:17-585:53`; AST `e70379f24edfe2dfbddb10f4f4c45f851a29bd5a962c091432b127a1724474f3` (builtin-append)
- Allocation: `append([]byte(nil), o.error.data...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 6. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.3` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:586:28-586:75`; AST `7c8b0e5f2ff0a68f1671ed0012cefca88dd079c53b15fd738a3c000c2d6b60bc` (builtin-append)
- Allocation: `append([]byte(nil), o.error.serializedBytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 7. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.4` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:587:34-587:87`; AST `2c4505de6c3d383701298218f748455d084206123c0b0b2eda2e3846f439b34e` (builtin-append)
- Allocation: `append([]byte(nil), o.external.notificationsBytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 8. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.5` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:588:30-588:79`; AST `11e85c11b34e99af694384bc376c03d6c92b4f4496022346a63d45e49e1b7228` (builtin-append)
- Allocation: `append([]byte(nil), o.external.responsesBytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 9. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.6` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:589:32-589:83`; AST `d101a08908237fcbaa48801f021c8aaf0f771ca16fe83fe24a91a87225e7871d` (builtin-append)
- Allocation: `append([]byte(nil), o.frames.request.frameBytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 10. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.7` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:590:33-590:85`; AST `55c456880d42f6016845580daef68472a5375d8de858e34e6daaf7dcbffaf69f` (builtin-append)
- Allocation: `append([]byte(nil), o.frames.response.frameBytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 11. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.8` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:591:19-591:66`; AST `a244dd6a6afea9b921ef43897296abe5de77dd0dc20695ecbdad0fbf8e768c75` (builtin-append)
- Allocation: `append(json.RawMessage(nil), o.result.bytes...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 12. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-append.append.1` — REUSE
- Source: `sessionruntime/b4_owned_method_pair_lease.go:857:15-857:34`; AST `da6c79e56f5882c3a5962b086224b95225502f68a72af3baaa152a3dd184d373` (builtin-append)
- Allocation: `append(detached, o)`
- Reservation/accounting provenance: Uses the fresh detached slice reserved at this function with capacity len(manager owners); appends cannot exceed that bound.
- Initial owner: detachCallHierarchyLayer1GenericOwnersLocked local detached slice.
- Transfer/custody: Carries detached owner pointers out of the manager lock to lifecycle terminalization.
- Release/retirement: Each detached owner receives requestTerminal then waitReleased on stop/restart.
- Refusal/terminal: Not on request refusal; lifecycle-only partitioning.
- Concurrency/exact-once: Manager mutex serializes partition; owner mutex serializes terminalization.
- Defensive copy: No payload copy; pointer list isolates lifecycle work after unlock.
- Backing: Reuses the fresh detached-slice backing allocated by the paired make site.
- Rationale: Exact evidence proves reuse of an existing backing or a pre-existing allocation expression; no new C15 allocation class is introduced.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 13. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-append.append.2` — REUSE
- Source: `sessionruntime/b4_owned_method_pair_lease.go:859:15-859:34`; AST `d9b16894f27398fc270820361bba2c42c6d1b35d14d2ed530344f7008acb980a` (builtin-append)
- Allocation: `append(retained, o)`
- Reservation/accounting provenance: retained is m.callHierarchyLayer1GenericOwners[:0], so append reuses the manager slice backing within its existing capacity.
- Initial owner: Manager retained-owner table.
- Transfer/custody: Retains nonmatching owner pointers in manager custody.
- Release/retirement: Later stop/restart/shutdown detaches and terminalizes retained owners.
- Refusal/terminal: Not on request refusal; lifecycle-only partitioning.
- Concurrency/exact-once: Manager mutex encloses compaction and publication.
- Defensive copy: No payload copy; in-place pointer compaction.
- Backing: Reuses existing manager owner-slice backing.
- Rationale: Exact evidence proves reuse of an existing backing or a pre-existing allocation expression; no new C15 allocation class is introduced.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 14. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-make.make.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:852:14-852:98`; AST `9bec583d09a7958e4cbe8cb5a835fcdde4607130356312c62e2e382d4490c6c0` (builtin-make)
- Allocation: `make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Lifecycle detach local.
- Transfer/custody: Receives matching manager-held owner pointers before unlock.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required to separate detached worklist from in-place retained compaction.
- Backing: New pointer-slice backing with capacity equal to current manager owner count.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 15. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:759:54-759:91`; AST `e553b6969f1ad694842cb2fcdab6de9618dd41859cf2f6cb786a20b85b27f5ec` (builtin-append)
- Allocation: `append(json.RawMessage(nil), body...) [accepted-terminal result]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 16. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:773:55-773:92`; AST `e553b6969f1ad694842cb2fcdab6de9618dd41859cf2f6cb786a20b85b27f5ec` (builtin-append)
- Allocation: `append(json.RawMessage(nil), body...) [successful-nonempty result]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 17. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.3` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:782:118-782:146`; AST `62b2f95ff20cb11660049a9645a7a31b7022324bd19d128f03bea91f4bacf609` (builtin-append)
- Allocation: `append([]byte(nil), body...) [external responses]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 18. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.4` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:788:112-788:142`; AST `45004ae9e9fcd911c3eb5d0e6190f0871932cb7626b561f98d836fed041eda46` (builtin-append)
- Allocation: `append([]byte(nil), e.Data...) [protocol error data]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 19. `alloc.b4.owned.method.pair.lease.finishaccepted.call.json.marshal.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:789:16-793:39`; AST `091674c004b49dfb632e304b7a0a18c8c69d85967880044979b252288de955c6` (call)
- Allocation: `json.Marshal(protocol RPC error projection)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 20. `alloc.b4.owned.method.pair.lease.finishaccepted.call.json.marshal.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:802:16-806:42`; AST `005bcba3245100e312afe9288e5baa1e008a84ac136b28f1c6d12348cb561617` (call)
- Allocation: `json.Marshal(fixed malformed-response error projection)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 21. `alloc.b4.owned.method.pair.lease.genericframe.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:559:7-559:34`; AST `76117c73d3671cc00146f0f188bb50d958500b2df87ea09aa978c5116cfead72` (builtin-append)
- Allocation: `append([]byte(nil), raw...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 22. `alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:535:38-535:116`; AST `9deaf0dbc751fdf0302b52ecf3b0944cb398197af051a7c10197d50f38b04c38` (builtin-append)
- Allocation: `append([]callHierarchyOpaqueCommitmentMetadata(nil), a.predecessors.values...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 23. `alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.builtin-append.append.1` — UNRESOLVED
- Source: `sessionruntime/b4_owned_method_pair_lease.go:848:39-848:88`; AST `3d56ac62d23567e84550e61da9f9ed98418a1d034d52e8ebdab7071d4c85968f` (builtin-append)
- Allocation: `append(m.callHierarchyLayer1GenericOwners, owner)`
- Reservation/accounting provenance: Publication is under m.mu, but neither patch nor enumeration proves spare capacity or forced growth at this append.
- Initial owner: Manager callHierarchyLayer1GenericOwners table if admitted.
- Transfer/custody: Result owner is transferred to manager custody unless manager is closed or generation retired; otherwise returned for immediate terminalization.
- Release/retirement: Manager detaches on completed stop/restart/shutdown; rejected publication is terminalized immediately.
- Refusal/terminal: Closed/retired publication does not retain owner and returns it for synchronous terminalization.
- Concurrency/exact-once: m.mu serializes publication/detach; owner terminal path drains active callbacks exactly once.
- Defensive copy: Payload was already copied into owner; this site only stores an owner pointer.
- Backing: Could reuse existing slice capacity or allocate a new backing array; evidence does not decide.
- Rationale: Allocation occurrence is capacity-dependent and the immutable evidence contains no capacity invariant proving reuse or growth; convenience cannot be inferred.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 24. `alloc.owned.method.pair.copycommitments.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:395:9-395:80`; AST `ac2f65cb87a172cfbcb3521f5ad092bd6db9d3a84cadae147a21119a232deedb` (builtin-append)
- Allocation: `append([]callHierarchyPredecessorCommitment(nil), chain.commitments...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 25. `alloc.owned.method.pair.copyframebytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:431:9-431:45`; AST `4a2889cd2a5f2adec0a7c4f6d20b066abba76326b39e9b3613a5d09f5deb8f17` (builtin-append)
- Allocation: `append([]byte(nil), o.frameBytes...) [request frame]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 26. `alloc.owned.method.pair.copyframebytes.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:437:9-437:45`; AST `4a2889cd2a5f2adec0a7c4f6d20b066abba76326b39e9b3613a5d09f5deb8f17` (builtin-append)
- Allocation: `append([]byte(nil), o.frameBytes...) [response frame]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 27. `alloc.owned.method.pair.copyparamsbytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:434:9-434:91`; AST `b12ed674de1226b727225838febb9ca4ae51ae848c11c18def1eed3ba6bb1741` (builtin-append)
- Allocation: `append([]byte(nil), o.frameBytes[o.paramsOffset:o.paramsOffset+o.paramsLength]...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 28. `alloc.owned.method.pair.copyresultbytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:440:9-440:91`; AST `add1a810585a039a3dc85723c0774e1a5b76db0899375e3176a8c26dc3d3f2d1` (builtin-append)
- Allocation: `append([]byte(nil), o.frameBytes[o.resultOffset:o.resultOffset+o.resultLength]...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 29. `alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:418:11-418:40`; AST `b1ab03459d0ab43e1623b408e8e9e9e3cfd3bbdda6af8e0837678de361a9f866` (builtin-append)
- Allocation: `append([]byte(nil), frame...) [original request]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 30. `alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:426:11-426:40`; AST `b1ab03459d0ab43e1623b408e8e9e9e3cfd3bbdda6af8e0837678de361a9f866` (builtin-append)
- Allocation: `append([]byte(nil), frame...) [original response]`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 31. `alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:487:72-487:135`; AST `8b6cd8f336f3bf2015380fbf7408c338be0739d9113022262ad880ad97872466` (builtin-append)
- Allocation: `append([]callHierarchyOpaqueCommitmentMetadata(nil), values...)`
- Reservation/accounting provenance: New retained or returned value created from patch input at this exact site; nil-based append/make/marshal gives independent storage when nonempty.
- Initial owner: Initially the constructing attempt/observation/returned-copy local; when embedded in an owner, manager-held callHierarchyLayer1GenericOwner custody.
- Transfer/custody: Attempt data is copied into observation, then defensively copied into owner; View/Callback return additional defensive copies where applicable.
- Release/retirement: Attempt temporaries become unreachable after finish; owner backing is cleared by releaseLocked after admission closes and active callbacks drain; manager detaches on stop/restart/shutdown.
- Refusal/terminal: Pre-effect refusals still construct/publish an observation owner; closed/retired manager state returns it for immediate requestTerminal/waitReleased.
- Concurrency/exact-once: Owner mutex gates admission and active callbacks; releaseStarted/released makes terminal release exactly-once; callback drain precedes clearing.
- Defensive copy: Required where bytes/slices cross caller, transport, attempt, owner, or view boundaries; prevents post-call mutation and alias retention.
- Backing: New independent backing for nonempty nil-based append/make/marshal result; zero-length append may remain nil and is a known zero.
- Rationale: Introduces independent storage required by the accepted write-set ownership/defensive-copy semantics and therefore requires explicit C15 treatment.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 32. `alloc.sessionruntime.runlifecycle.builtin-make.make.3` — REUSE
- Source: `sessionruntime/sessionruntime.go:2961:129-2961:166`; AST `0ea68843a9a50cc154711a531af54cc7e981a0207c34876ac7d461c556bd2f0a` (builtin-make)
- Allocation: `make(map[lspwire.RequestKey]*Request) [pre-existing successor startup site, ordinal shifted]`
- Reservation/accounting provenance: Exact AST hash matches a pre-patch runLifecycle allocation that was renumbered by inserted syntax; no new allocation semantics.
- Initial owner: Existing successor runtime-session startup state.
- Transfer/custody: Unchanged pre-existing startup custody.
- Release/retirement: Unchanged pre-existing runtime-session lifecycle.
- Refusal/terminal: Unchanged pre-existing startup failure behavior.
- Concurrency/exact-once: Unchanged pre-existing manager lifecycle synchronization.
- Defensive copy: No new defensive copy; classification is site reuse by exact AST identity.
- Backing: Existing allocation expression, renumbered; not a newly introduced backing class.
- Rationale: Exact evidence proves reuse of an existing backing or a pre-existing allocation expression; no new C15 allocation class is introduced.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

### 33. `alloc.sessionruntime.runlifecycle.builtin-make.make.4` — REUSE
- Source: `sessionruntime/sessionruntime.go:2961:168-2961:197`; AST `fc84b204c50e9cebd8d9f1bf471f2d6e6f9df91a08473b5accc57d44b377393f` (builtin-make)
- Allocation: `make(chan struct{}, 1) [pre-existing successor startup site, ordinal shifted]`
- Reservation/accounting provenance: Exact AST hash matches a pre-patch runLifecycle allocation that was renumbered by inserted syntax; no new allocation semantics.
- Initial owner: Existing successor runtime-session startup state.
- Transfer/custody: Unchanged pre-existing startup custody.
- Release/retirement: Unchanged pre-existing runtime-session lifecycle.
- Refusal/terminal: Unchanged pre-existing startup failure behavior.
- Concurrency/exact-once: Unchanged pre-existing manager lifecycle synchronization.
- Defensive copy: No new defensive copy; classification is site reuse by exact AST identity.
- Backing: Existing allocation expression, renumbered; not a newly introduced backing class.
- Rationale: Exact evidence proves reuse of an existing backing or a pre-existing allocation expression; no new C15 allocation class is introduced.
- Evidence ceiling: Static baseline+accepted-patch+enumeration evidence only; establishes source/lifecycle structure, not runtime allocation frequency, allocator behavior, production execution, or unobserved capacity state.

## Input hashes

- `ADDED.tsv`: `f7c8e6259ba1cc396d77c871ee54669b93c68f501458da973b4b2a6565646dbc`
- `CHANGED.tsv`: `8ef2257684ee18cccc271ea82a4c32a43d06b5041afb652024329fe1c3f69198`
- `candidates.tsv`: `4e1ec4a7cf20bf0d3a74b7b4e7206a05c7f14efd9a8dc91961eaa30c7bf080e4`
- `classifications.tsv`: `6b7d05e6ed36bbb38cbd9100e50e3473e858f0b295d96894eb4c6a47a7110ffd`
- `metadata.json`: `e1e25cdeb870dbe36a4694ecfddb3e2b5fd80d5d20903e307396af177beaf344`
