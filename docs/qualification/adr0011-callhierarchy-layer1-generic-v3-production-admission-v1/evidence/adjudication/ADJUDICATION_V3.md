# ADR-0011 C15 exact33 source-site adjudication V3

- Schema: `lsp-trace/adr0011-c15-semantic-adjudication-v3`
- Counts: total=33 NECESSARY=29 REUSE=4 UNRESOLVED=0
- Disposition: `C15_DELTA_GO`
- `C15_DELTA_GO`: proposed because all 33 are complete, unresolved is zero, and the exact complement is 319.

## Independent 352 delta accounting

- Universe provenance: `ADDED.tsv` + `CHANGED.tsv` + `REMOVED.tsv` candidate IDs (not `all-source-locations.tsv`).
- Universe: 352 unique IDs.
- Exact adjudicated set: 33 unique IDs.
- FP complement: 319 unique IDs.
- Intersection(exact33, complement): 0.
- Union count: 352; union equals universe: `true`.
- Canonical FP complement SHA-256: `0bdfdeb69508e0f3e21b4d5287f7d18fdba2f7b7b78d0d2db014558a79e223b9` (sorted IDs, LF-terminated).

## V3 corrections

- Restored NOACC to accounting_reservation_rule and exact site owner to initial_owner for eight NECESSARY rows.
- Corrected RESTART make.3/make.4 to the exact requests/documents map expressions, fields, owners, and transfers; retained REUSE only because the expressions pre-exist base.
- Corrected complement-universe provenance to ADDED.tsv + CHANGED.tsv + REMOVED.tsv.
- Adjudicated publication append NECESSARY for capacity-growing occurrences without claiming every append allocates.

## Evidence ceiling

Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

## Rows

### 1. `alloc.b4.owned.method.pair.lease.admit.builtin-make.make.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:549:12-549:27`; AST `93e3af7d1f4df6e8ed36c89ef8c6aebcd94cd54ae1235a8ff373208cd1bebad1` (builtin-make)
- Exact expression/input: make([]byte, 8), populated big-endian from admitted key.ID
- Allocation/nonempty behavior: Unconditional once non-nil attempt.admit is called; length is always 8, so the language creates an eight-byte slice value (heap placement is not proved).
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: admit local number.
- Transfer/custody: number local -> a.requestIdentity.requestKey.numberBytes -> finishAccepted identity -> owner observation (the patch does not make a second request-key-byte copy).
- Natural release/retirement: Becomes unreachable when the attempt/observation chain is discarded; if published, clearing owner.observation at terminal drops the final retained reference.
- Refusal/terminal: Not reached for pre-effect refusals because they finish before pending.Begin/admit; accepted transactions only. Terminal handling applies later to the containing owner, not this local. 
- Concurrency/exact-once: The expression operates on an attempt/function-local value before publication; no owner mutex or exact-once release claim applies at this site. After publication, only the containing owner lifecycle is synchronized.
- Necessity/reuse reason: Necessary fixed-width materialization of numeric RequestKey identity; no existing []byte input/backing is available to reuse.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 2. `alloc.b4.owned.method.pair.lease.captureresult.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:576:53-576:92`; AST `e4f0159479758e4db5adb4d41fa92a902df13e5f353d2e0bcfa8da1ac1352330` (builtin-append)
- Exact expression/input: append(json.RawMessage(nil), result...); input is decoded result bytes
- Allocation/nonempty behavior: captureResult on a non-nil attempt; the expression always executes, allocating new backing only when len(result)>0 (nil/empty remains zero-length).
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: captureResult local result value assigned to a.result.bytes.
- Transfer/custody: a.result.bytes -> finishAccepted observation fields -> new owner defensive copy -> lease View/Callback defensive copies.
- Natural release/retirement: Attempt-held copy is unreachable after finish; owner-held copy is dropped by releaseLocked; returned view copies die with caller references.
- Refusal/terminal: Not used by pre-effect refusals. For admitted terminal results it preserves decoded result/error evidence; owner terminal refusal only governs later access.
- Concurrency/exact-once: The expression operates on an attempt/function-local value before publication; no owner mutex or exact-once release claim applies at this site. After publication, only the containing owner lifecycle is synchronized.
- Necessity/reuse reason: Necessary ingress snapshot: caller/decoder-owned bytes may be reused or mutated; nil-based append isolates the attempt.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 3. `alloc.b4.owned.method.pair.lease.captureresult.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:579:12-579:61`; AST `5b5e5a627193866f7c655c46d3e70304fa5e27712f0c91bb001976399e18b7e0` (builtin-append)
- Exact expression/input: append(json.RawMessage(nil), serverError.Data...); input is RPC error Data bytes
- Allocation/nonempty behavior: captureResult on a non-nil attempt; the expression executes only when serverError != nil, allocating new backing only when len(serverError.Data)>0.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: captureResult local result value assigned to copied e.Data retained via a.serverError.
- Transfer/custody: copied e.Data retained via a.serverError -> finishAccepted observation fields -> new owner defensive copy -> lease View/Callback defensive copies.
- Natural release/retirement: Attempt-held copy is unreachable after finish; owner-held copy is dropped by releaseLocked; returned view copies die with caller references.
- Refusal/terminal: Not used by pre-effect refusals. For admitted terminal results it preserves decoded result/error evidence; owner terminal refusal only governs later access.
- Concurrency/exact-once: The expression operates on an attempt/function-local value before publication; no owner mutex or exact-once release claim applies at this site. After publication, only the containing owner lifecycle is synchronized.
- Necessity/reuse reason: Necessary ingress snapshot: caller/decoder-owned bytes may be reused or mutated; nil-based append isolates the attempt.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 4. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:584:31-584:114`; AST `8aa5d1a70547d4d1578faa2ceb288571174c4d91a8ff565c55812067d3aec273` (builtin-append)
- Exact expression/input: append([]callHierarchyOpaqueCommitmentMetadata(nil), o.commitments.predecessors...); exact source field o.commitments.predecessors
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.commitments.predecessors)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.commitments.predecessors: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 5. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:585:17-585:53`; AST `e70379f24edfe2dfbddb10f4f4c45f851a29bd5a962c091432b127a1724474f3` (builtin-append)
- Exact expression/input: append([]byte(nil), o.error.data...); exact source field o.error.data
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.error.data)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.error.data: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 6. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.3` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:586:28-586:75`; AST `7c8b0e5f2ff0a68f1671ed0012cefca88dd079c53b15fd738a3c000c2d6b60bc` (builtin-append)
- Exact expression/input: append([]byte(nil), o.error.serializedBytes...); exact source field o.error.serializedBytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.error.serializedBytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.error.serializedBytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 7. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.4` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:587:34-587:87`; AST `2c4505de6c3d383701298218f748455d084206123c0b0b2eda2e3846f439b34e` (builtin-append)
- Exact expression/input: append([]byte(nil), o.external.notificationsBytes...); exact source field o.external.notificationsBytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.external.notificationsBytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.external.notificationsBytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 8. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.5` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:588:30-588:79`; AST `11e85c11b34e99af694384bc376c03d6c92b4f4496022346a63d45e49e1b7228` (builtin-append)
- Exact expression/input: append([]byte(nil), o.external.responsesBytes...); exact source field o.external.responsesBytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.external.responsesBytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.external.responsesBytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 9. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.6` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:589:32-589:83`; AST `d101a08908237fcbaa48801f021c8aaf0f771ca16fe83fe24a91a87225e7871d` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frames.request.frameBytes...); exact source field o.frames.request.frameBytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.frames.request.frameBytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.frames.request.frameBytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 10. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.7` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:590:33-590:85`; AST `55c456880d42f6016845580daef68472a5375d8de858e34e6daaf7dcbffaf69f` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frames.response.frameBytes...); exact source field o.frames.response.frameBytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.frames.response.frameBytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.frames.response.frameBytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 11. `alloc.b4.owned.method.pair.lease.copygenericobservation.builtin-append.append.8` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:591:19-591:66`; AST `a244dd6a6afea9b921ef43897296abe5de77dd0dc20695ecbdad0fbf8e768c75` (builtin-append)
- Exact expression/input: append(json.RawMessage(nil), o.result.bytes...); exact source field o.result.bytes
- Allocation/nonempty behavior: Unconditional when copyGenericObservation runs; new backing only if len(o.result.bytes)>0, otherwise the result is nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: The local by-value observation copy c/o returned by copyGenericObservation.
- Transfer/custody: source observation field -> independent field in constructor copy (owner.observation) or genericView copy (returned View/Callback value), depending on the exact caller.
- Natural release/retirement: Constructor copy is dropped when releaseLocked zeros owner.observation; View/Callback copy becomes unreachable when the callback/returned value is released.
- Refusal/terminal: Refusal observations can contain this field only when populated; copying itself does not accept/refuse or terminalize. Post-terminal calls are rejected before genericView. 
- Concurrency/exact-once: copyGenericObservation executes while its caller holds the relevant context (constructor before publication, or owner mutex in View/Callback); this local copy has no independent mutex or exact-once release operation.
- Necessity/reuse reason: Necessary alias break for o.result.bytes: the containing struct copy is shallow for slices, so this exact append prevents owner/view mutation from sharing that field backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 12. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-append.append.1` — REUSE
- Source: `sessionruntime/b4_owned_method_pair_lease.go:857:15-857:34`; AST `da6c79e56f5882c3a5962b086224b95225502f68a72af3baaa152a3dd184d373` (builtin-append)
- Exact expression/input: append(detached, o); input o is each matching owner pointer from m.callHierarchyLayer1GenericOwners
- Allocation/nonempty behavior: Conditional on a matching owner. The append count is at most the number iterated; detached was made with cap=len(m.callHierarchyLayer1GenericOwners), so this append cannot grow its backing.
- Accounting/reservation: Capacity is reserved structurally by the paired make, not by a C15 byte account; no payload bytes are copied.
- Initial owner: The function-local detached slice created by the paired make.
- Transfer/custody: m.callHierarchyLayer1GenericOwners element -> detached local -> return value -> runLifecycle local genericOwners -> requestTerminal/waitReleased for each owner.
- Natural release/retirement: The detached list becomes unreachable after the lifecycle loop; each pointed-to owner is retired through requestTerminal then waitReleased.
- Refusal/terminal: Request refusal is inapplicable: this is stop/restart lifecycle partitioning. Terminalization is the purpose of the returned list.
- Concurrency/exact-once: m.mu is held while partitioning; list construction itself needs no owner mutex. Each owner independently enforces terminal exact-once.
- Necessity/reuse reason: Reuses the paired make backing; exact capacity bound proves no append growth.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 13. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-append.append.2` — REUSE
- Source: `sessionruntime/b4_owned_method_pair_lease.go:859:15-859:34`; AST `d9b16894f27398fc270820361bba2c42c6d1b35d14d2ed530344f7008acb980a` (builtin-append)
- Exact expression/input: append(retained, o); retained := m.callHierarchyLayer1GenericOwners[:0]
- Allocation/nonempty behavior: Conditional on a nonmatching owner. In-place compaction appends no more elements than the original slice length, so capacity is sufficient and no new backing is allocated.
- Accounting/reservation: No C15 reservation: this is pointer-table compaction into already owned manager backing.
- Initial owner: Manager field m.callHierarchyLayer1GenericOwners backing, viewed through local retained.
- Transfer/custody: manager table -> retained in-place compaction -> reassigned to m.callHierarchyLayer1GenericOwners.
- Natural release/retirement: Backing remains manager-owned until later detach or Shutdown sets the table nil; dropped tail entries are explicitly nilled.
- Refusal/terminal: Request refusal is inapplicable; retained owners remain live. Terminal behavior occurs only on a later lifecycle detach/shutdown.
- Concurrency/exact-once: m.mu serializes compaction/publication; no per-owner terminal or exact-once action occurs here.
- Necessity/reuse reason: Reuses the existing manager-table backing by exact full-capacity reslice semantics.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 14. `alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.builtin-make.make.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:852:14-852:98`; AST `9bec583d09a7958e4cbe8cb5a835fcdde4607130356312c62e2e382d4490c6c0` (builtin-make)
- Exact expression/input: make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))
- Allocation/nonempty behavior: Unconditional per detach invocation. Capacity equals the current manager-owner count; when count is zero no element backing need be allocated, while nonzero reserves one bounded pointer worklist.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: detachCallHierarchyLayer1GenericOwnersLocked local detached worklist.
- Transfer/custody: fresh worklist -> matching owner pointers appended -> returned to runLifecycle genericOwners -> terminalization loop.
- Natural release/retirement: Worklist naturally dies after the lifecycle loop; pointed-to owners release separately through requestTerminal/waitReleased.
- Refusal/terminal: Request refusal is inapplicable. The worklist exists only to execute lifecycle terminal retirement outside m.mu.
- Concurrency/exact-once: m.mu protects snapshot/partition; the list has one goroutine-local owner. No exact-once release applies to list backing.
- Necessity/reuse reason: Necessary independent worklist so terminal waits occur after unlocking without aliasing the manager table being compacted.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 15. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:759:54-759:91`; AST `e553b6969f1ad694842cb2fcdab6de9618dd41859cf2f6cb786a20b85b27f5ec` (builtin-append)
- Exact expression/input: append(json.RawMessage(nil), body...)
- Allocation/nonempty behavior: Only in causalOutcome AcceptedTerminal; allocates backing iff body is nonempty.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local observation o.result.bytes.
- Transfer/custody: response-frame body slice -> o.result.bytes -> owner constructor defensive copy -> view copies.
- Natural release/retirement: Local observation copy dies after owner construction; owner/view copies retire by their own paths.
- Refusal/terminal: Applies to accepted-terminal causal outcomes, including failed-* terminal mapping; it is not the pre-effect-refusal branch.
- Concurrency/exact-once: finishAccepted mutates one attempt/local observation before publication; no owner mutex/exact-once action here.
- Necessity/reuse reason: Necessary to separate retained result bytes from a.responseFrame backing.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 16. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:773:55-773:92`; AST `e553b6969f1ad694842cb2fcdab6de9618dd41859cf2f6cb786a20b85b27f5ec` (builtin-append)
- Exact expression/input: append(json.RawMessage(nil), body...)
- Allocation/nonempty behavior: Only when failure=="", serverError==nil, and computed terminal is successful-nonempty; body is expected nonempty by classification, and append allocates iff len(body)>0.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local observation o.result.bytes.
- Transfer/custody: successful response body -> o.result.bytes -> owner constructor defensive copy -> view copies.
- Natural release/retirement: Local observation dies after construction; owner copy clears at terminal; view copies die naturally.
- Refusal/terminal: Only successful-nonempty accepted execution; refusal and failed terminal paths do not execute this site.
- Concurrency/exact-once: Pre-publication local operation; owner synchronization/exact-once is inapplicable at this expression.
- Necessity/reuse reason: Necessary result snapshot distinct from captured response frame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 17. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.3` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:782:118-782:146`; AST `62b2f95ff20cb11660049a9645a7a31b7022324bd19d128f03bea91f4bacf609` (builtin-append)
- Exact expression/input: append([]byte(nil), body...) assigned to o.external.responsesBytes
- Allocation/nonempty behavior: Unconditional in finishAccepted after body selection; allocates new backing iff body is nonempty.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local o.external.responsesBytes.
- Transfer/custody: selected response body -> external.responsesBytes -> owner constructor defensive copy -> view copies.
- Natural release/retirement: Local dies after construction; owner copy clears at terminal; view copies die naturally.
- Refusal/terminal: Runs for accepted and pre-effect-refusal observations; refusals generally have empty body, yielding nil/no backing.
- Concurrency/exact-once: Pre-publication local operation; no mutex/exact-once release applies here.
- Necessity/reuse reason: Necessary independent external-response projection rather than aliasing responseFrame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 18. `alloc.b4.owned.method.pair.lease.finishaccepted.builtin-append.append.4` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:788:112-788:142`; AST `45004ae9e9fcd911c3eb5d0e6190f0871932cb7626b561f98d836fed041eda46` (builtin-append)
- Exact expression/input: append([]byte(nil), e.Data...) assigned to o.error.data
- Allocation/nonempty behavior: Only when a.serverError != nil; allocates backing iff e.Data is nonempty.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local o.error.data.
- Transfer/custody: attempt-owned copied serverError.Data -> o.error.data -> owner constructor defensive copy -> view copies.
- Natural release/retirement: Same local/owner/view natural retirement chain.
- Refusal/terminal: Only protocol-error accepted finish; pre-effect refusal and non-protocol terminals do not execute it.
- Concurrency/exact-once: Pre-publication local operation; no owner mutex/exact-once action here.
- Necessity/reuse reason: Necessary because error.data is a separately exposed projection and must not alias attempt/serverError Data.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 19. `alloc.b4.owned.method.pair.lease.finishaccepted.call.json.marshal.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:789:16-793:39`; AST `091674c004b49dfb632e304b7a0a18c8c69d85967880044979b252288de955c6` (call)
- Exact expression/input: json.Marshal(struct{Code int64; Data json.RawMessage; Message string}{int64(e.Code), e.Data, e.Message})
- Allocation/nonempty behavior: Only when a.serverError != nil; on marshal success b is nonempty JSON with fresh encoder-owned bytes; on error no bytes are retained.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local b, then o.error.serializedBytes.
- Transfer/custody: protocol error fields -> json.Marshal result b -> o.error.serializedBytes -> owner defensive copy -> view copies.
- Natural release/retirement: b/o local dies after owner construction; owner/view copies retire naturally/terminally.
- Refusal/terminal: Only protocol-error terminal; refusal and non-protocol paths are inapplicable.
- Concurrency/exact-once: Pre-publication local encoding; exact-once terminal synchronization does not govern this allocation.
- Necessity/reuse reason: Necessary canonical serialized error projection; no pre-existing serialized buffer exists.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 20. `alloc.b4.owned.method.pair.lease.finishaccepted.call.json.marshal.2` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:802:16-806:42`; AST `005bcba3245100e312afe9288e5baa1e008a84ac136b28f1c6d12348cb561617` (call)
- Exact expression/input: json.Marshal(struct{Code int64; Data json.RawMessage; Message string}{o.error.code, data, o.error.message})
- Allocation/nonempty behavior: Only malformed-failure branch; successful marshal returns nonempty fresh JSON, marshal error retains nothing.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: finishAccepted local b, then o.error.serializedBytes.
- Transfer/custody: fixed malformed fields/data literal -> marshal b -> o.error.serializedBytes -> owner defensive copy -> view copies.
- Natural release/retirement: Local/owner/view copies retire through natural scope and owner terminal clearing.
- Refusal/terminal: Only malformed terminal; refusal, success, and protocol-error branches are inapplicable.
- Concurrency/exact-once: Pre-publication local encoding; no independent mutex/exact-once release.
- Necessity/reuse reason: Necessary serialized malformed-error representation; not reusable from the distinct data token.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 21. `alloc.b4.owned.method.pair.lease.genericframe.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:559:7-559:34`; AST `76117c73d3671cc00146f0f188bb50d958500b2df87ea09aa978c5116cfead72` (builtin-append)
- Exact expression/input: append([]byte(nil), raw...)
- Allocation/nonempty behavior: genericFrame first returns zero when len(raw)==0; expression therefore runs only for nonempty raw and necessarily creates independent nonempty slice backing.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: genericFrame local b.
- Transfer/custody: caller frame bytes -> b -> attempt requestFrame/responseFrame -> finishAccepted observation -> owner defensive copy -> view copies.
- Natural release/retirement: Attempt/local becomes unreachable after finish; owner/view copies retire as specified.
- Refusal/terminal: Not reached for absent frames/pre-effect refusal; terminal semantics only later classify the captured frame.
- Concurrency/exact-once: Attempt-local copy; no owner mutex or exact-once release applies at capture.
- Necessity/reuse reason: Necessary capture snapshot because raw belongs to framed transport capture and cannot be retained by alias.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 22. `alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:535:38-535:116`; AST `9deaf0dbc751fdf0302b52ecf3b0944cb398197af051a7c10197d50f38b04c38` (builtin-append)
- Exact expression/input: append([]callHierarchyOpaqueCommitmentMetadata(nil), a.predecessors.values...)
- Allocation/nonempty behavior: Only when attachment predecessors pointer is non-nil; allocates backing iff values is nonempty (a nonnil pointer may still hold empty values).
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: new attempt local attempt.commitments.predecessors.
- Transfer/custody: request attachment predecessor list -> attempt -> finishAccepted observation -> owner defensive copy -> view copies.
- Natural release/retirement: Attempt list dies after finish; owner list clears terminally; view list dies naturally.
- Refusal/terminal: Can occur before a later pre-effect refusal because attempt construction precedes refusal checks; terminal/refusal does not alter this copy.
- Concurrency/exact-once: Construction-local copy; no owner synchronization/exact-once release at this site.
- Necessity/reuse reason: Necessary ingress isolation from the mutable request attachment predecessor slice.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 23. `alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/b4_owned_method_pair_lease.go:848:39-848:88`; AST `3d56ac62d23567e84550e61da9f9ed98418a1d034d52e8ebdab7071d4c85968f` (builtin-append)
- Exact expression/input: append(m.callHierarchyLayer1GenericOwners, owner)
- Allocation/nonempty behavior: Only after owner != nil and the manager is neither closed nor generation-retired. Allocation is capacity-dependent: append reuses backing when len < cap and allocates replacement backing when growth is required. No capacity invariant or preallocation exists, so this does not claim every append allocates.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: On a capacity-growing occurrence, manager registry field m.callHierarchyLayer1GenericOwners owns the replacement backing after assignment; without growth it continues to own the existing backing.
- Transfer/custody: owner pointer -> append result assigned to manager registry field m.callHierarchyLayer1GenericOwners under m.mu -> detached worklist on STOP/RESTART or Shutdown local -> requestTerminal/waitReleased for the pointed owner.
- Natural release/retirement: On growth, the superseded backing becomes naturally unreachable after assignment. The active registry backing becomes naturally unreachable when STOP/RESTART detach or Shutdown clears/transfers the registry; explicit owner Release does not remove the registry entry.
- Refusal/terminal: Closed or generation-retired publication skips append and returns the owner for synchronous terminalization. Admitted publication retains the pointer in the Manager registry; explicit Release alone does not remove it.
- Concurrency/exact-once: m.mu serializes registry append, detach, and Shutdown transfer. Owner terminal exact-once behavior governs the pointed owner, not whether this append grows its backing.
- Necessity/reuse reason: NECESSARY on capacity-growing occurrences: replacement backing is required to retain the complete Manager registry pointer set after append. Because no capacity invariant or preallocation exists, growth occurrence frequency is unproved and no claim is made that every append allocates.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity. For this append, the evidence proves the conditional growth rule and absence of a capacity guarantee; it does not prove which runtime invocations grow or how often growth occurs.

### 24. `alloc.owned.method.pair.copycommitments.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:395:9-395:80`; AST `ac2f65cb87a172cfbcb3521f5ad092bd6db9d3a84cadae147a21119a232deedb` (builtin-append)
- Exact expression/input: append([]callHierarchyPredecessorCommitment(nil), chain.commitments...)
- Allocation/nonempty behavior: Unconditional when copyCommitments is called; allocates iff chain.commitments is nonempty.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: copyCommitments return slice local/result.
- Transfer/custody: chain.commitments -> returned independent slice -> caller-owned value.
- Natural release/retirement: Returned copy becomes unreachable with caller; source chain lifetime is independent.
- Refusal/terminal: Generic refusal/terminal behavior is inapplicable to this value helper.
- Concurrency/exact-once: Pure value-copy helper; no owner mutex or exact-once release exists.
- Necessity/reuse reason: Necessary caller copy because returning chain.commitments directly would alias the chain.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 25. `alloc.owned.method.pair.copyframebytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:431:9-431:45`; AST `4a2889cd2a5f2adec0a7c4f6d20b066abba76326b39e9b3613a5d09f5deb8f17` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frameBytes...) in request copyFrameBytes
- Allocation/nonempty behavior: Unconditional per method call; valid request observations have nonempty frameBytes, so a call on valid data creates independent backing.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: request copyFrameBytes method return slice.
- Transfer/custody: request observation frameBytes -> caller-owned returned copy.
- Natural release/retirement: Returned slice dies with caller; observation retains its own backing.
- Refusal/terminal: Refusal/terminal behavior is inapplicable; this is an accessor on immutable observation data.
- Concurrency/exact-once: No shared mutation or exact-once release; method-local return copy only.
- Necessity/reuse reason: Necessary defensive accessor copy to prevent caller mutation of stored request frame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 26. `alloc.owned.method.pair.copyframebytes.builtin-append.append.2` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:437:9-437:45`; AST `4a2889cd2a5f2adec0a7c4f6d20b066abba76326b39e9b3613a5d09f5deb8f17` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frameBytes...) in response copyFrameBytes
- Allocation/nonempty behavior: Unconditional per method call; valid response observations have nonempty frameBytes, so valid calls create independent backing.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: response copyFrameBytes method return slice.
- Transfer/custody: response observation frameBytes -> caller-owned returned copy.
- Natural release/retirement: Returned slice dies with caller; observation backing remains.
- Refusal/terminal: Refusal/terminal behavior is inapplicable.
- Concurrency/exact-once: No owner mutex/exact-once release; standalone returned value.
- Necessity/reuse reason: Necessary defensive accessor copy to prevent mutation of stored response frame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 27. `alloc.owned.method.pair.copyparamsbytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:434:9-434:91`; AST `b12ed674de1226b727225838febb9ca4ae51ae848c11c18def1eed3ba6bb1741` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frameBytes[o.paramsOffset:o.paramsOffset+o.paramsLength]...)
- Allocation/nonempty behavior: Unconditional per call after slicing; allocates iff paramsLength>0, while a valid zero-length params span returns nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: copyParamsBytes method return slice.
- Transfer/custody: validated params subspan of stored request frame -> independent caller slice.
- Natural release/retirement: Returned slice naturally dies with caller.
- Refusal/terminal: Refusal/terminal behavior is inapplicable.
- Concurrency/exact-once: Pure accessor; no mutex or exact-once release.
- Necessity/reuse reason: Necessary to expose params without aliasing the complete stored frame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 28. `alloc.owned.method.pair.copyresultbytes.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:440:9-440:91`; AST `add1a810585a039a3dc85723c0774e1a5b76db0899375e3176a8c26dc3d3f2d1` (builtin-append)
- Exact expression/input: append([]byte(nil), o.frameBytes[o.resultOffset:o.resultOffset+o.resultLength]...)
- Allocation/nonempty behavior: Unconditional per call after slicing; allocates iff resultLength>0; zero-length valid result yields nil/zero-length.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: copyResultBytes method return slice.
- Transfer/custody: validated result subspan of response frame -> independent caller slice.
- Natural release/retirement: Returned copy dies with caller.
- Refusal/terminal: Refusal/terminal behavior is inapplicable.
- Concurrency/exact-once: Pure accessor; no mutex or exact-once release.
- Necessity/reuse reason: Necessary to expose result token without aliasing stored response frame.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 29. `alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:418:11-418:40`; AST `b1ab03459d0ab43e1623b408e8e9e9e3cfd3bbdda6af8e0837678de361a9f866` (builtin-append)
- Exact expression/input: append([]byte(nil), frame...) after all length/offset/limit guards
- Allocation/nonempty behavior: Conditional on constructor validation success; guard requires len(frame)>0, so every successful constructor executes a nonempty copy.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: constructor local owned, then returned request observation field frameBytes.
- Transfer/custody: caller frame -> owned local -> returned request-frame observation.
- Natural release/retirement: On validation failure no allocation; on success backing lives with returned observation then becomes GC-eligible.
- Refusal/terminal: Refusal/terminal behavior is inapplicable to this constructor.
- Concurrency/exact-once: Construction is local; no mutex or exact-once release.
- Necessity/reuse reason: Necessary retained immutable request frame used for hashes and subspan access.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 30. `alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:426:11-426:40`; AST `b1ab03459d0ab43e1623b408e8e9e9e3cfd3bbdda6af8e0837678de361a9f866` (builtin-append)
- Exact expression/input: append([]byte(nil), frame...) after all length/offset/limit guards
- Allocation/nonempty behavior: Conditional on constructor validation success; len(frame)>0 is required, so successful construction copies nonempty bytes.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: constructor local owned, then returned response observation field frameBytes.
- Transfer/custody: caller frame -> owned local -> returned response-frame observation.
- Natural release/retirement: Failure allocates nothing; successful backing dies with returned observation.
- Refusal/terminal: Refusal/terminal behavior is inapplicable.
- Concurrency/exact-once: Construction-local; no mutex or exact-once release.
- Necessity/reuse reason: Necessary retained immutable response frame for hashes/result subspan.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 31. `alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.builtin-append.append.1` — NECESSARY
- Source: `sessionruntime/owned_method_pair.go:487:72-487:135`; AST `8b6cd8f336f3bf2015380fbf7408c338be0739d9113022262ad880ad97872466` (builtin-append)
- Exact expression/input: append([]callHierarchyOpaqueCommitmentMetadata(nil), values...)
- Allocation/nonempty behavior: Only when len(values)!=0; therefore every execution has nonempty input and creates independent backing.
- Accounting/reservation: No privateB4ByteAccountV2 reservation/lease is called by this generic-layer site. finishAccepted explicitly emits the reservations ledger dimension as available known-zero (before=after=delta=0); under the accepted C15 semantics this is non-custodial generic observation storage, so no C15 byte reservation/release symbol applies.
- Initial owner: new callHierarchyOpaqueCommitmentList stored in the returned attachment field predecessors.
- Transfer/custody: caller values -> copied list -> attachment.predecessors -> later generic attempt makes its own ingress copy.
- Natural release/retirement: Attachment copy dies with request/attachment; attempt copy lifetime is separate.
- Refusal/terminal: May precede either execution or pre-effect refusal; it does not itself refuse/terminalize.
- Concurrency/exact-once: Builder is value-local; no owner mutex or exact-once release.
- Necessity/reuse reason: Necessary request-builder isolation from caller mutation before RoundTrip consumes predecessors.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 32. `alloc.sessionruntime.runlifecycle.builtin-make.make.3` — REUSE
- Source: `sessionruntime/sessionruntime.go:2961:129-2961:166`; AST `0ea68843a9a50cc154711a531af54cc7e981a0207c34876ac7d461c556bd2f0a` (builtin-make)
- Exact expression/input: make(map[lspwire.RequestKey]*Request) assigned to restarted runtimeSession field r.requests
- Allocation/nonempty behavior: Unconditional when the successful RESTART assignment executes; the empty map value is newly constructed for the restarted runtimeSession. REUSE is retained only because this exact expression and field assignment pre-exist base ce4a1838f8a10bafc13429031e55df7e4dd812c6; the patch only shifts its enumeration ordinal.
- Accounting/reservation: Unchanged baseline restarted-runtimeSession allocation; no generic-observation privateB4ByteAccountV2 reservation or lease is introduced.
- Initial owner: Restarted runtimeSession field r.requests (requests map).
- Transfer/custody: fresh requests map -> assignment to restarted runtimeSession field r.requests -> runtime request/document lifecycle operations.
- Natural release/retirement: The map remains reachable through the restarted runtimeSession until that session is stopped, restarted again, shut down, or otherwise becomes unreachable under the pre-existing runtimeSession lifecycle.
- Refusal/terminal: Generic observation refusal and owner terminalization are inapplicable; this is pre-existing RESTART initialization of runtimeSession state.
- Concurrency/exact-once: The assignment occurs in the pre-existing lifecycle restart path; this adjudication does not infer additional synchronization or exact-once behavior from the allocation expression.
- Necessity/reuse reason: REUSE only in delta-adjudication terms: the exact allocation expression and ownership field pre-exist the base. The expression still constructs a fresh empty map when RESTART executes.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

### 33. `alloc.sessionruntime.runlifecycle.builtin-make.make.4` — REUSE
- Source: `sessionruntime/sessionruntime.go:2961:168-2961:197`; AST `fc84b204c50e9cebd8d9f1bf471f2d6e6f9df91a08473b5accc57d44b377393f` (builtin-make)
- Exact expression/input: make(map[string]openDocument) assigned to restarted runtimeSession field r.documents
- Allocation/nonempty behavior: Unconditional when the successful RESTART assignment executes; the empty map value is newly constructed for the restarted runtimeSession. REUSE is retained only because this exact expression and field assignment pre-exist base ce4a1838f8a10bafc13429031e55df7e4dd812c6; the patch only shifts its enumeration ordinal.
- Accounting/reservation: Unchanged baseline restarted-runtimeSession allocation; no generic-observation privateB4ByteAccountV2 reservation or lease is introduced.
- Initial owner: Restarted runtimeSession field r.documents (documents map).
- Transfer/custody: fresh documents map -> assignment to restarted runtimeSession field r.documents -> runtime request/document lifecycle operations.
- Natural release/retirement: The map remains reachable through the restarted runtimeSession until that session is stopped, restarted again, shut down, or otherwise becomes unreachable under the pre-existing runtimeSession lifecycle.
- Refusal/terminal: Generic observation refusal and owner terminalization are inapplicable; this is pre-existing RESTART initialization of runtimeSession state.
- Concurrency/exact-once: The assignment occurs in the pre-existing lifecycle restart path; this adjudication does not infer additional synchronization or exact-once behavior from the allocation expression.
- Necessity/reuse reason: REUSE only in delta-adjudication terms: the exact allocation expression and ownership field pre-exist the base. The expression still constructs a fresh empty map when RESTART executes.
- Evidence ceiling: Static evidence from base ce4a1838f8a10bafc13429031e55df7e4dd812c6, accepted PATCH.diff SHA-256 e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0, exact callers/contracts visible there, and frozen enumeration only. It proves source-level control flow and custody; it does not prove runtime frequency, escape/heap placement, allocator behavior, production execution, or unobserved slice capacity.

## FP complement IDs

Canonical hash: `0bdfdeb69508e0f3e21b4d5287f7d18fdba2f7b7b78d0d2db014558a79e223b9`

```text
alloc.b4.owned.method.pair.lease.admit.call.byte.1
alloc.b4.owned.method.pair.lease.admit.call.len.1
alloc.b4.owned.method.pair.lease.admit.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.callback.call.errors.new.1
alloc.b4.owned.method.pair.lease.callback.call.expression.1
alloc.b4.owned.method.pair.lease.callback.call.fn.1
alloc.b4.owned.method.pair.lease.callback.call.genericview.1
alloc.b4.owned.method.pair.lease.callback.call.o.cond.broadcast.1
alloc.b4.owned.method.pair.lease.callback.call.o.mu.lock.1
alloc.b4.owned.method.pair.lease.callback.call.o.mu.lock.2
alloc.b4.owned.method.pair.lease.callback.call.o.mu.unlock.1
alloc.b4.owned.method.pair.lease.callback.call.o.mu.unlock.2
alloc.b4.owned.method.pair.lease.callback.call.o.mu.unlock.3
alloc.b4.owned.method.pair.lease.callback.call.recover.1
alloc.b4.owned.method.pair.lease.capturerequestframe.call.genericframe.1
alloc.b4.owned.method.pair.lease.captureresponseframe.call.genericframe.1
alloc.b4.owned.method.pair.lease.captureresult.call.json.rawmessage.1
alloc.b4.owned.method.pair.lease.captureresult.call.json.rawmessage.2
alloc.b4.owned.method.pair.lease.captureresult.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.1
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.2
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.3
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.4
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.5
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.6
alloc.b4.owned.method.pair.lease.copygenericobservation.call.expression.7
alloc.b4.owned.method.pair.lease.copygenericobservation.call.json.rawmessage.1
alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.call.len.1
alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.call.len.2
alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.call.len.3
alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.call.string.1
alloc.b4.owned.method.pair.lease.detachcallhierarchylayer1genericownerslocked.call.uint64.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.byte.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.bytes.contains.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.bytes.equal.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.bytes.equal.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.bytes.index.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.5
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.6
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.7
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.8
alloc.b4.owned.method.pair.lease.finishaccepted.call.expression.9
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericavailable.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericavailable.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericavailable.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericavailable.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericmeasure.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericmeasure.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericmeasure.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.genericmeasure.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.int64.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.int64.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.json.rawmessage.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.json.rawmessage.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.5
alloc.b4.owned.method.pair.lease.finishaccepted.call.len.6
alloc.b4.owned.method.pair.lease.finishaccepted.call.session.failure.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.session.failure.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.session.failure.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.session.failure.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.string.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.string.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.string.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.string.4
alloc.b4.owned.method.pair.lease.finishaccepted.call.strings.hasprefix.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.strings.trimprefix.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.uint64.1
alloc.b4.owned.method.pair.lease.finishaccepted.call.uint64.2
alloc.b4.owned.method.pair.lease.finishaccepted.call.uint64.3
alloc.b4.owned.method.pair.lease.finishaccepted.call.uint64.4
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.10
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.11
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.12
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.13
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.14
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.15
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.16
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.17
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.18
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.19
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.20
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.21
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.22
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.23
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.24
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.25
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.26
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.27
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.28
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.29
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.3
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.30
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.4
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.5
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.6
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.7
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.8
alloc.b4.owned.method.pair.lease.finishaccepted.composite-literal.composite.literal.9
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.a.finishaccepted.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.a.recordrefusal.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.genericmeasure.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.m.mu.lock.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.m.mu.unlock.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.m.publishcallhierarchylayer1genericownerlocked.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.newcallhierarchylayer1genericowner.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.terminal.requestterminal.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.call.terminal.waitreleased.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.3
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.4
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.5
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.6
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.7
alloc.b4.owned.method.pair.lease.finishcallhierarchylayer1genericrefusal.composite-literal.composite.literal.8
alloc.b4.owned.method.pair.lease.genericavailable.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.genericdelta.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.genericdelta.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.genericframe.call.expression.1
alloc.b4.owned.method.pair.lease.genericframe.call.len.1
alloc.b4.owned.method.pair.lease.genericframe.call.len.2
alloc.b4.owned.method.pair.lease.genericframe.call.sha256.sum256.1
alloc.b4.owned.method.pair.lease.genericframe.call.uint64.1
alloc.b4.owned.method.pair.lease.genericframe.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.genericframe.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.genericmeasure.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.genericview.call.copygenericobservation.1
alloc.b4.owned.method.pair.lease.genericview.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.call.callhierarchygeneration.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.call.callhierarchymethodidentity.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.call.callhierarchysessionidentity.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.call.expression.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericattemptlineage.composite-literal.composite.literal.3
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericowner.call.copygenericobservation.1
alloc.b4.owned.method.pair.lease.newcallhierarchylayer1genericowner.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.privatecallhierarchylayer1genericlease.call.o.mu.lock.1
alloc.b4.owned.method.pair.lease.privatecallhierarchylayer1genericlease.call.o.mu.unlock.1
alloc.b4.owned.method.pair.lease.privatecallhierarchylayer1genericlease.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.call.string.1
alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.call.uint64.1
alloc.b4.owned.method.pair.lease.publishcallhierarchylayer1genericownerlocked.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.release.call.o.cond.broadcast.3
alloc.b4.owned.method.pair.lease.release.call.o.cond.wait.2
alloc.b4.owned.method.pair.lease.release.call.o.mu.lock.2
alloc.b4.owned.method.pair.lease.release.call.o.mu.unlock.3
alloc.b4.owned.method.pair.lease.release.call.o.releaselocked.1
alloc.b4.owned.method.pair.lease.releaselocked.call.o.cond.broadcast.1
alloc.b4.owned.method.pair.lease.releaselocked.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.requestterminal.call.o.cond.broadcast.2
alloc.b4.owned.method.pair.lease.requestterminal.call.o.cond.wait.1
alloc.b4.owned.method.pair.lease.requestterminal.call.o.mu.lock.2
alloc.b4.owned.method.pair.lease.requestterminal.call.o.mu.unlock.2
alloc.b4.owned.method.pair.lease.requestterminal.call.o.releaselocked.1
alloc.b4.owned.method.pair.lease.view.call.genericview.1
alloc.b4.owned.method.pair.lease.view.call.o.mu.lock.1
alloc.b4.owned.method.pair.lease.view.call.o.mu.unlock.1
alloc.b4.owned.method.pair.lease.view.composite-literal.composite.literal.1
alloc.b4.owned.method.pair.lease.view.composite-literal.composite.literal.2
alloc.b4.owned.method.pair.lease.waitreleased.call.o.cond.wait.2
alloc.b4.owned.method.pair.lease.waitreleased.call.o.mu.lock.2
alloc.b4.owned.method.pair.lease.waitreleased.call.o.mu.unlock.2
alloc.owned.method.pair.copycommitments.call.expression.1
alloc.owned.method.pair.copyframebytes.call.expression.1
alloc.owned.method.pair.copyframebytes.call.expression.2
alloc.owned.method.pair.copyparamsbytes.call.expression.1
alloc.owned.method.pair.copyresultbytes.call.expression.1
alloc.owned.method.pair.newcallhierarchyobservationlimits.composite-literal.composite.literal.1
alloc.owned.method.pair.newcallhierarchyobservationlimits.composite-literal.composite.literal.2
alloc.owned.method.pair.newcallhierarchyobservationrequest.call.allowlist.permits.1
alloc.owned.method.pair.newcallhierarchyobservationrequest.composite-literal.composite.literal.1
alloc.owned.method.pair.newcallhierarchyobservationrequest.composite-literal.composite.literal.2
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.expression.1
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.1
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.2
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.3
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.4
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.5
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.6
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.len.7
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.sha256.sum256.1
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.sha256.sum256.2
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.sha256.sum256.3
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.1
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.2
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.3
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.4
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.5
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.call.uint64.6
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.composite-literal.composite.literal.1
alloc.owned.method.pair.newcallhierarchyoriginalrequestframeobservation.composite-literal.composite.literal.2
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.expression.1
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.1
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.2
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.3
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.4
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.5
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.6
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.len.7
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.sha256.sum256.1
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.sha256.sum256.2
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.sha256.sum256.3
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.1
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.2
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.3
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.4
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.5
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.call.uint64.6
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.composite-literal.composite.literal.1
alloc.owned.method.pair.newcallhierarchyoriginalresponseframeobservation.composite-literal.composite.literal.2
alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.call.expression.1
alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.call.len.1
alloc.owned.method.pair.privatecallhierarchylayer1withpredecessors.composite-literal.composite.literal.1
alloc.sessionruntime.finishroundtrip.call.attempt.finishaccepted.1
alloc.sessionruntime.finishroundtrip.call.genericterminal.requestterminal.1
alloc.sessionruntime.finishroundtrip.call.genericterminal.waitreleased.1
alloc.sessionruntime.finishroundtrip.call.m.publishcallhierarchylayer1genericownerlocked.1
alloc.sessionruntime.finishroundtrip.call.newcallhierarchylayer1genericowner.1
alloc.sessionruntime.finishroundtrip.call.string.1
alloc.sessionruntime.finishroundtrip.composite-literal.composite.literal.4
alloc.sessionruntime.finishroundtrip.composite-literal.composite.literal.5
alloc.sessionruntime.finishroundtrip.composite-literal.composite.literal.6
alloc.sessionruntime.roundtrip.call.m.finishcallhierarchylayer1genericrefusal.1
alloc.sessionruntime.roundtrip.call.m.finishcallhierarchylayer1genericrefusal.2
alloc.sessionruntime.roundtrip.call.newcallhierarchylayer1genericattemptlineage.1
alloc.sessionruntime.roundtrip.call.session.failure.1
alloc.sessionruntime.roundtrip.call.session.failure.2
alloc.sessionruntime.roundtrip.composite-literal.composite.literal.1
alloc.sessionruntime.roundtrip.composite-literal.composite.literal.2
alloc.sessionruntime.roundtrip.composite-literal.composite.literal.3
alloc.sessionruntime.roundtripwithprivatetransaction.call.attempt.admit.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.child.stdout.3
alloc.sessionruntime.roundtripwithprivatetransaction.call.expression.before.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.genericresponsecapture.retainedframe.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.genericresponsecapture.sha256.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.int64.10
alloc.sessionruntime.roundtripwithprivatetransaction.call.int64.11
alloc.sessionruntime.roundtripwithprivatetransaction.call.int64.13
alloc.sessionruntime.roundtripwithprivatetransaction.call.int64.6
alloc.sessionruntime.roundtripwithprivatetransaction.call.int64.7
alloc.sessionruntime.roundtripwithprivatetransaction.call.io.reader.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.io.teereader.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.json.marshal.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.json.marshal.2
alloc.sessionruntime.roundtripwithprivatetransaction.call.json.marshal.3
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.10
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.11
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.12
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.13
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.14
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.6
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.7
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.8
alloc.sessionruntime.roundtripwithprivatetransaction.call.len.9
alloc.sessionruntime.roundtripwithprivatetransaction.call.lspwire.newreader.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.lspwire.newreaderobservedatdecode.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.m.finishcallhierarchylayer1genericrefusal.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.m.finishcallhierarchylayer1genericrefusal.2
alloc.sessionruntime.roundtripwithprivatetransaction.call.m.finishcallhierarchylayer1genericrefusal.3
alloc.sessionruntime.roundtripwithprivatetransaction.call.m.finishcallhierarchylayer1genericrefusal.4
alloc.sessionruntime.roundtripwithprivatetransaction.call.m.finishcallhierarchylayer1genericrefusal.5
alloc.sessionruntime.roundtripwithprivatetransaction.call.newframedwritecapturebounded.2
alloc.sessionruntime.roundtripwithprivatetransaction.call.read.err.error.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.req.deadline.iszero.4
alloc.sessionruntime.roundtripwithprivatetransaction.call.req.deadline.sub.2
alloc.sessionruntime.roundtripwithprivatetransaction.call.result.callhierarchylayer1genericattempt.capturerequestframe.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.result.callhierarchylayer1genericattempt.captureresponseframe.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.result.callhierarchylayer1genericattempt.captureresult.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.session.failure.1
alloc.sessionruntime.roundtripwithprivatetransaction.call.writecapture.retainedframe.3
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.15
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.16
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.17
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.18
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.19
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.20
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.21
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.22
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.23
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.24
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.25
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.26
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.27
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.28
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.29
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.30
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.31
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.32
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.33
alloc.sessionruntime.roundtripwithprivatetransaction.composite-literal.composite.literal.34
alloc.sessionruntime.runlifecycle.builtin-make.make.1
alloc.sessionruntime.runlifecycle.builtin-make.make.2
alloc.sessionruntime.runlifecycle.call.genericowner.requestterminal.1
alloc.sessionruntime.runlifecycle.call.genericowner.requestterminal.2
alloc.sessionruntime.runlifecycle.call.genericowner.waitreleased.1
alloc.sessionruntime.runlifecycle.call.genericowner.waitreleased.2
alloc.sessionruntime.runlifecycle.call.m.detachcallhierarchylayer1genericownerslocked.1
alloc.sessionruntime.runlifecycle.call.m.detachcallhierarchylayer1genericownerslocked.2
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.10
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.11
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.4
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.5
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.6
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.7
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.8
alloc.sessionruntime.runlifecycle.composite-literal.composite.literal.9
alloc.sessionruntime.shutdown.call.genericowner.requestterminal.1
alloc.sessionruntime.shutdown.call.genericowner.waitreleased.1
```
