# ADR-0011 Call-Hierarchy Scaffold C15 Four-Row Assessment

Status: **NO**

Assessment target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`;
- requested pinned assessment HEAD: `66d11b6a87078d9210c71e3d6ff189c9905aa25a`;
- observed worktree HEAD during this documentation-only assessment: `9df7fdcf99bae8d6253eef601c39be92eb6b3c9b`, whose only committed change after `66d11b6a…` is the causal-boundary rereview document;
- preserved pre-existing worktree delta: exactly 110 additions and zero deletions in `sessionruntime/b4_owned_method_pair_lease_test.go`;
- authority: bounded C15 assessment only; no source, test, regeneration, implementation, or commit authority.

```text
ADR0011_CALLHIERARCHY_SCAFFOLD_C15_FOUR_ROW_NO
```

## Preserved Manager boundary

The Manager file is preserved exactly:

| Field | Exact value |
|---|---|
| Path | `internal/session/manager.go` |
| Byte length | `36640` |
| SHA-256 | `214d42c0da0f6203df1734b87702b7e67f221485260ad5575411d993b21f6a1e` |

The worktree bytes and the bytes at `66d11b6a87078d9210c71e3d6ff189c9905aa25a` have that same length and digest. The file has no worktree diff. This assessment therefore records **Manager nonpropagation**: Manager does not construct, attach, publish, reserve for, or return a Call Hierarchy observation owner or lease. The retained v2 manifest independently records `"manager_propagation_claim": false`.

## Disposable command and result

The assessment used a disposable archive and regenerated the current source catalog only inside a temporary directory; it did not mutate the reviewed worktree:

```sh
src=/Users/schwa/dev/lsp-trace-adr0011-c17
tmp=$(mktemp -d /tmp/adr0011-c15-four-row.XXXXXX)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/repo"
(cd "$src" && git archive HEAD) | (cd "$tmp/repo" && tar -xf -)
cd "$tmp/repo"
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' -count=1
```

The command passed. Comparing the disposable generated candidate and classification rows with the retained `sessionruntime/testdata/c15-census-v2/` artifacts produced:

- candidates: `3847 -> 3847`;
- removed: `0`;
- changed candidate rows: `0`;
- changed classification rows: `0`.

Thus the retained C15 v2 candidate and classification files remain **3,847 rows and byte-identical** at the assessed source state. Their exact digests remain:

- `candidates.tsv`: `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2`;
- `classifications.tsv`: `494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730`.

The pre-existing partial RED is in an excluded `*_test.go` file and therefore creates no production-source C15 candidate delta.

## Exact four-row assessment

The proposed cleanup would touch exactly these four retained v2 candidate rows:

| Candidate ID | Exact source location | Candidate hash | Retained classification |
|---|---|---|---|
| `alloc.b4.owned.method.pair.lease.newcallhierarchyobservationowner.composite-literal.composite.literal.1` | `sessionruntime/b4_owned_method_pair_lease.go:181` | `3511521e0bd66a5f5302ec727743a36659e5fda015217e2946e46e5cb5abf4ab` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.1` | `sessionruntime/sessionruntime.go:738` | `9cb9b449c194ce0f142bc395860b47df1ff301d77ecd6dd364584636b5d23cf5` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.2` | `sessionruntime/sessionruntime.go:745` | `9cb9b449c194ce0f142bc395860b47df1ff301d77ecd6dd364584636b5d23cf5` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |
| `alloc.sessionruntime.privatecallhierarchyobservationlease.composite-literal.composite.literal.3` | `sessionruntime/sessionruntime.go:747` | `2efdb4a4aa0681e0a69d7b09a625c4d0520b90f4fd3e610cf72b44f290a027c1` | `excluded` / `external-library` — `composite.literal is library behavior outside privateB4ByteAccountV2 custody` |

These are exact retained row identities and source locations, not accepted feature identities or evidence of runtime allocation volume.

## Semantic adjudication

All four rows are **false positives for C15 ownership**.

1. `newCallHierarchyObservationOwner`'s struct literal merely assembles an unexported Go value. At this site there is no `privateB4ByteAccountV2` owner operation, no byte copy attributed to that account, no lease or reservation acquisition, no capacity charge, and no retained-byte release obligation.
2. The two empty `callHierarchyObservationLease{}` returns are zero-value sentinels. They allocate or retain no payload, establish no owner, acquire no lease, perform no reservation, and create no release obligation.
3. The `callHierarchyObservationLease{owner: owner}` return is a direct private handle value. It does not copy owner-held observation bytes, reserve capacity, acquire a separately accounted lease, or create a `privateB4ByteAccountV2` release obligation.

Accordingly, none of the four rows is a C15 owner, copy, lease, or reservation site. Their existing typed `excluded` classifications are semantically correct. AST presence and matching names confer neither ownership nor Manager propagation.

## Honest cleanup boundary

An honest cleanup must not rename, disguise, wrap, reorder, or otherwise obscure expressions merely to evade the census. It may remove only unnecessary inert zero-value constructor/accessor functions and their composite literals. It must retain:

- the minimal private type shapes required by the frozen scaffold;
- the direct private `RoundTripRequest` and `RoundTripResult` attachment fields;
- direct same-package access to those private fields needed by a future authorized RED; and
- ordinary language zero values, used directly without helper construction where no behavior exists.

Cleanup must not add owner construction, defensive copying, leasing, reservation, accounting, callback, publication, or Manager behavior. It must not claim semantic improvement from reducing mechanically discovered rows.

## Decision

**v3 decision: NO.** No cleanup implementation is authorized by this assessment.

Although the four rows are semantic false positives, deleting or rewriting their expressions would mechanically change the frozen 3,847-row C15 v2 candidate bytes. C15 v2 must remain byte-identical. A source cleanup therefore cannot proceed under this assessment merely because the affected rows are correctly excluded.

Any later implementation proposal must, before acceptance:

1. independently regenerate and compare the catalog;
2. verify **no candidate delta**, no removed or renumbered row, no changed hash, and no reclassification;
3. preserve the 3,847-row v2 byte boundary and both retained artifact digests;
4. preserve Manager nonpropagation and the direct private request/result attachment boundary; and
5. stop and return to review if any candidate or classification byte changes.

Because a cleanup that literally removes these four composite literals is expected to remove or alter four catalog rows, that proposal is presently incompatible with the no-delta boundary. The appropriate next step is an **independent cleanup review**, separate from this assessment, which must reconcile the desired source simplification with the immutable C15 v2 evidence boundary without renaming or obscuring census-visible constructs.

No v3 gate is required or claimed here. This document itself grants no implementation, regeneration, source/test mutation, evidence-production, publication, or commit authority.

```text
ADR0011_CALLHIERARCHY_SCAFFOLD_C15_FOUR_ROW_NO
```
