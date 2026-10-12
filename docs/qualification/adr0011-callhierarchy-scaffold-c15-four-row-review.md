# ADR-0011 Call-Hierarchy Scaffold C15 Four-Row Independent Review

Status: **V3 NOT REQUIRED**

Review target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`;
- exact reviewed HEAD: `9df7fdcf99bae8d6253eef601c39be92eb6b3c9b`;
- reviewed untracked assessment: `docs/qualification/adr0011-callhierarchy-scaffold-c15-four-row-assessment.md`;
- preserved pre-existing partial RED: exactly 110 additions and zero deletions in `sessionruntime/b4_owned_method_pair_lease_test.go`;
- authority: independent cleanup-plan qualification only; no source, test, assessment, regeneration-in-worktree, implementation, semantic behavior, or commit authority.

```text
CALLHIERARCHY_SCAFFOLD_C15_V3_NOT_REQUIRED
```

## Decision

No new C15 v3 semantic decision is required for the proposed cleanup plan. The four mechanically discovered rows are already retained in C15 v2 with exact typed `excluded` classifications. Removing the unnecessary unreachable constructor/accessor/lifecycle behavior would remove mechanically visible source expressions; it would not reclassify an owner, copy, lease, or reservation operation and would not create or alter Manager propagation.

C15 v2 remains immutable historical evidence at its exact retained bytes. A later source cleanup must not regenerate, overwrite, normalize, delete, or otherwise revise those v2 artifacts merely to make them describe a newer source state. This review does not authorize implementation.

## Preserved Manager scaffold evidence

Independent verification reproduced the assessment's exact Manager evidence:

| Field | Exact value |
|---|---|
| Path | `internal/session/manager.go` |
| Byte length | `36640` |
| SHA-256 | `214d42c0da0f6203df1734b87702b7e67f221485260ad5575411d993b21f6a1e` |

`internal/session/manager.go` has no worktree diff and contains no `callHierarchyObservation` occurrence. The reviewed production path therefore does not construct, attach, publish, return, reserve for, lease, copy for, or otherwise propagate the call-hierarchy observation scaffold.

## Exact four-row reproduction

The retained candidate and classification files independently reproduce exactly these four rows:

| Candidate ID | Exact source location | Candidate hash | Exact retained classification |
|---|---|---|---|
| `alloc.b4.owned.method.pair.lease.newcallhierarchyobservationowner.composite-literal.composite.literal.1` | `sessionruntime/b4_owned_method_pair_lease.go:181` | `3511521e0bd66a5f5302ec727743a36659e5fda015217e2946e46e5cb5abf4ab` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.1` | `sessionruntime/sessionruntime.go:738` | `9cb9b449c194ce0f142bc395860b47df1ff301d77ecd6dd364584636b5d23cf5` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.2` | `sessionruntime/sessionruntime.go:745` | `9cb9b449c194ce0f142bc395860b47df1ff301d77ecd6dd364584636b5d23cf5` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.3` | `sessionruntime/sessionruntime.go:747` | `2efdb4a4aa0681e0a69d7b09a625c4d0520b90f4fd3e610cf72b44f290a027c1` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |

These are AST-derived row identities, not runtime allocation counts or accepted semantic identities. At each site there is no `privateB4ByteAccountV2` owner operation, accounted byte copy, capacity reservation, acquired accounted lease, or corresponding release obligation. The direct `owner` field assignment in the fourth expression does not itself establish any of those semantics.

## Honest functional-removal boundary

The cleanup is honest only if it deletes unnecessary behavior rather than renaming, respelling, wrapping, reordering, or otherwise hiding census-visible constructs.

The current source contains unreachable behavior beyond the inert scaffold boundary: `newCallHierarchyObservationOwner`, observation deep-copy/view helpers, callback admission and panic handling, terminal closure, callback drain, release behavior, and `privateCallHierarchyObservationLease`. Manager never invokes or publishes this path. A permitted cleanup plan may remove those unnecessary constructor/accessor/lifecycle functions and their helper composite allocations.

The cleanup plan may retain only the minimal typed attachment shapes needed by the frozen private scaffold through existing private structs and the existing private `RoundTripRequest`/`RoundTripResult` fields. Ordinary language zero values and direct same-package private-field access may remain; they need no zero-value constructor, accessor, owner, lease, reservation, callback, or helper composite allocation. The cleanup must not introduce replacement wrappers, aliases, factories, spelling changes, reflection, indirect construction, or any other census-evasion technique.

This plan grants no authority to add owner construction, copying, reservations, accounting, lease acquisition, callbacks, publication, Manager propagation, or semantic call-hierarchy behavior.

## C15 v2 preservation

A disposable archive of exact HEAD was regenerated with:

```sh
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' -count=1
```

The command passed, and both generated files compared byte-for-byte equal to the retained worktree artifacts:

| Artifact | Rows | SHA-256 |
|---|---:|---|
| `sessionruntime/testdata/c15-census-v2/candidates.tsv` | 3847 | `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2` |
| `sessionruntime/testdata/c15-census-v2/classifications.tsv` | 3847 | `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730` |

The disposable reproduction establishes exact current pre-cleanup correspondence. It does not authorize regeneration after cleanup or mutation of the retained v2 evidence. Source evolution may make a historical catalog non-current without changing its retained historical bytes; that fact alone does not require a new semantic decision.

## Preserved partial RED

The partial RED remains exactly 110 additions and zero deletions. Its worktree SHA-256 remains:

```text
dc32d3f75c32c4b3fdd2a479c09408b11866db08efff00da85e36359c682899b
```

`git diff --check` passes. The targeted test remains RED at the intended absent Manager-publication seam: seven refusal-path subtests pass, while the incoming and outgoing publication subtests fail because the result observation lease is missing. This review neither edits nor approves that partial RED and does not convert it into implementation authority.

## Cleanup-plan conditions

Any separately authorized cleanup must:

1. change only the separately authorized source paths;
2. remove the unnecessary unreachable constructor/accessor/lifecycle behavior rather than disguise it;
3. preserve the minimal private typed attachment fields and ordinary zero behavior without helper composite allocations;
4. add no owners, copies, leases, reservations, callbacks, accounting, publication, or Manager propagation;
5. leave all retained C15 v2 files byte-identical at the exact hashes above;
6. leave the existing partial RED byte-identical unless a separate RED-edit authority is issued; and
7. return for review if the plan expands into semantic behavior or evidence mutation.

No source cleanup, test change, assessment edit, regeneration in the reviewed worktree, commit, or semantic behavior is authorized by this document.

```text
CALLHIERARCHY_SCAFFOLD_C15_V3_NOT_REQUIRED
```
