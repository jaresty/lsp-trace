# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set — Final Independent Review V4

Status: **BLOCKED**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `b283d7ea6cf3d4da71cb0bed75517b9995d1922a`
- Reviewed inputs: the modified `adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-final.md`, the untracked `adr0011-callhierarchy-layer1-generic-v3-refusal-owner-registry-amendment.md`, prior final reviews, and pinned production declarations
- Review mode: independent documentation-only final review
- Source, tests, corpus, RED, prototype, generated artifacts, and other documents: **not modified or executed**
- Production authority: **not issued**

## Findings

### 1. Exact physical boundary and finite declarations

The proposed production mutation boundary remains exactly these three existing files:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The integrated document closes the proposed fields, types, existing changed declarations, new methods/functions, and forbidden additions. No fourth production file, test edit, helper-signature change, registry-entry wrapper, owner back-pointer, package global, second index, or additional map is admitted.

### 2. Request/result carrier and finalization closure

The carrier is coherent and compile-compatible:

```text
first executable RoundTrip overwrite
→ private RoundTripRequest attempt field
→ unchanged by-value helper chain
→ one transient RoundTripResult attempt-field initialization after pending.Begin
→ every admitted terminal path through finishRoundTrip
→ immutable owner publication
→ unconditional transient-field clearing before return
```

The final owner field remains distinct from both attempt-carrier fields. Existing helper signatures, `AdmitSeedBinding`, `RoundTripPrivateB4`, their call sites, and direct immutable test callers remain unchanged. No stale operative explicit attempt-parameter requirement remains.

### 3. Twelve refusals and Manager publication

Pinned source contains the twelve proposed pre-key refusal branches in the documented order. The integrated write-set requires each enabled branch to call exactly one lock-owning `(*Manager).finishCallHierarchyLayer1GenericRefusal` after releasing any currently held `m.mu`; nil attempts preserve public behavior without allocation or publication.

The finalizer records the typed refusal, freezes one observation, constructs one owner, and calls `publishCallHierarchyLayer1GenericOwnerLocked` in one Manager critical section. The helper attaches the owner, clears the transient carrier, and either appends exactly once to the Manager owner slice or returns that same owner as the closed/retired outside-lock terminal action. Every post-key return observed in the pinned transaction body reaches `finishRoundTrip`.

### 4. Closed/retired race gate and lifecycle ordering

The Manager-owned slice plus exact-generation retired set closes the former publication-after-detach race:

- publication before retirement is appended and selected by detach;
- matching publication after retirement, or publication after `m.closed`, is not appended;
- refused publication is terminalized and joined outside `m.mu` before its result returns;
- completed STOP marks the exact current generation before detaching all generations for the session ID;
- completed RESTART marks and detaches only the old generation before successor startup; and
- Shutdown, after worker drain with `m.closed` durably true, transfers the complete Manager slice, clears registry state, drains outside the lock, then resumes shutdown finalization.

No owner callback, callback drain, terminal request, or release wait is permitted under `m.mu`. Failed lifecycle branches before committed retirement do not detach; owners detached after committed retirement are never reinserted.

### 5. Absent/stale owners, release, and forced teardown

Absent-session refusal owners and unretired stale-generation refusal owners are retained in the Manager registry rather than a nonexistent/current session registry. `SessionNotFound` STOP does not claim to detach absent-session owners; Shutdown provides the finite Manager-lifetime bound.

Explicit `Release` and teardown `requestTerminal` both close admission, drain already admitted callbacks, and race through the same sole `releaseLocked` transition. `releaseLocked` alone clears observation custody and sets the release bits; exactly one racer wins. `waitReleased` is observational after forced teardown and does not depend on a user actor. Later acquisition, `View`, and `Callback` fail closed.

### 6. No stale registry or parameter contradiction

Search of the controlling integrated document found no operative session-owned registry, free refusal/publication helper, or explicit attempt-pointer parameter requirement. Session-registry and parameter-threading mentions occur only as superseded history or explicit prohibitions. The Manager fields, generation-key type, helper signatures, C15 field/type/function lists, allocation discussion, and hard stops consistently use the amended model.

### 7. Prior blocked path is accurately noncanonical

The prior independent reviews were documentation blockers: first teardown liveness, then compile compatibility from explicit parameter threading. They do not record a completed disposable mutation or canonical C15 enumeration. The integrated document therefore accurately states that no prior canonical C15 result or surviving disposable authority exists and requires a new writer, new disposable run, new canonical enumeration, independent delta review, and disposal.

### 8. Blocking C15 closure contradiction

The proposed C15 boundary is not fully closed over allocation/copy candidates.

Section 11 calls the candidate set exact and says any extra or missing allocation/copy site is an immediate hard stop. But it specifies:

```text
detachCallHierarchyLayer1GenericOwnersLocked: stable retained-slice backing allocation only if selected by the disposable implementation; canonical C15 must report in-place compaction versus allocation
```

This leaves two admissible implementations with different canonical candidate sets:

1. in-place stable compaction, with no retained-slice backing allocation candidate; or
2. stable filtering with one retained-slice backing allocation candidate.

The later canonical run cannot choose between them without changing whether the supposedly exact expected candidate exists. That is an implementation-selected C15 delta, contrary to the document's statements that the set is closed, is not a lower bound, and permits no silent extra or missing candidate.

Before a disposable writer can be authorized, the controlling write-set must select one exact detach implementation shape and make the allocation/copy candidate list unconditional. This correction can remain within the same three files and does not disturb the otherwise coherent carrier, refusal, Manager publication, retirement, teardown, absent-owner, or release contracts, but it requires a revised review input.

## Disposition

```text
BLOCKED — the exact closed C15 candidate set is conditional: detachCallHierarchyLayer1GenericOwnersLocked may use either in-place stable compaction or a retained-slice backing allocation, so the proposal does not determine whether that allocation candidate is present
```

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_FINAL_V2_BLOCKED
```

Disposition: **BLOCKED / NO GO**
