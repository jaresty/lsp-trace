# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review V5

Status: **GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `b283d7ea6cf3d4da71cb0bed75517b9995d1922a`
- Reviewed inputs: the modified `adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`, the untracked `adr0011-callhierarchy-layer1-generic-v3-refusal-owner-registry-amendment.md`, and `adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final-review-v4.md`
- Review mode: superseding independent documentation-only final review
- Source, tests, corpus, RED, prototype, generated artifacts, and other documents: **not modified or executed**
- Production authority: **not issued**

## Findings

### 1. Prior V4 coherent findings remain accepted

The V4 findings remain coherent and unchanged except for its sole blocking finding, which the amended controlling write-set closes:

1. the physical mutation boundary remains exactly the three existing production files `sessionruntime/sessionruntime.go`, `sessionruntime/owned_method_pair.go`, and `sessionruntime/b4_owned_method_pair_lease.go`;
2. the private request/result attempt-carrier flow, unchanged helper signatures, admitted finalization through `finishRoundTrip`, immutable owner publication, and unconditional transient-carrier clearing remain closed and compile-compatible as specified;
3. the exact twelve pre-key refusals publish through the lock-owning Manager finalizer, while every admitted terminal path reaches `finishRoundTrip`;
4. Manager publication, exact-generation retirement, detach, closed/retired refusal, outside-lock terminalization/join, STOP, RESTART, and Shutdown ordering remain mutually coherent;
5. absent-session and unretired stale-generation owners remain Manager-lifetime bounded, while explicit release and forced teardown share the sole release transition and later access fails closed;
6. no stale operative session registry, free publication helper, explicit attempt-parameter requirement, fourth production file, second index, owner back-pointer, registry wrapper, or additional map is admitted; and
7. the prior blocked path remains correctly noncanonical and grants no surviving disposable authority.

Nothing in the final amendment disturbs those findings.

### 2. V4's sole C15 blocker is closed exactly

V4 blocked because the former text allowed either in-place retained compaction or a retained-slice backing allocation, making the supposedly exact C15 allocation/copy set conditional on a later implementation choice.

The amended controlling write-set now selects one implementation shape unconditionally:

```text
detached := make([]*callHierarchyLayer1GenericOwner, 0, len(m.callHierarchyLayer1GenericOwners))
```

`detachCallHierarchyLayer1GenericOwnersLocked` must allocate that detached slice at the single named site, append every matching owner to it, compact every retained pointer in place through `m.callHierarchyLayer1GenericOwners[:0]`, nil every vacated tail slot, and assign the compacted retained slice back to the Manager field.

The allocation registry repeats the same closed rule: exactly one unconditional detached-slice allocation site, no retained-slice backing allocation, and in-place retained compaction. STOP and RESTART add no owner-slice copy allocation beyond that one detached-slice site; Shutdown transfers the complete Manager slice directly and performs no owner-slice copy allocation.

The former phrases permitting a retained backing allocation “if selected” or “if required,” and the former instruction that canonical C15 choose or report between two filtering shapes, are absent from the controlling integrated write-set.

### 3. No remaining conditional candidate or open implementation choice

The integrated field, type, changed-declaration, new-function/method, and allocation/copy registries are finite and mutually consistent. The detach contract and candidate registry now agree on the same mandatory allocation and compaction shape.

Remaining conditions such as enabled versus disabled requests, nil versus non-nil mutable members, available versus unavailable bytes, closed/retired publication, panic-only error creation, map initialization/growth, and append-induced slice growth are runtime occurrence conditions attached to named candidate sites. They do not permit the disposable implementation to choose a different declaration, allocation/copy site, helper, data structure, filtering strategy, or canonical C15 candidate set.

No operative text leaves an alternate retained-slice allocation, alternate detach algorithm, optional helper, optional registry representation, or other implementation-selected C15 delta. Any extra or missing candidate remains an explicit hard stop.

## Authority boundary

This review accepts only the exact documentation-defined disposable write-set and closed expected C15 candidate boundary. It does not name a disposable writer, execute a prototype, classify a candidate, accept a future canonical delta, authorize production mutation, or issue production custody. The mandatory new disposable authorization, qualified v3 GREEN, canonical enumeration, independent delta review, and verified disposal remain required in order.

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_V2_GO
```

Disposition: **GO for a separately authorized new disposable run; production authority not issued**
