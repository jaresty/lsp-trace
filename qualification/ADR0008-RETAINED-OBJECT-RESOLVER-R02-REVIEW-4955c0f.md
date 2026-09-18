# ADR 0008 retained object resolver R02 qualification review

- Reviewed revision: `4955c0fc87cef6867b8c19e6c197285341022b68`
- Execution record: `qualification/adr0008-retained-object-resolver.execution.4955c0fc87cef6867b8c19e6c197285341022b68.json`
- R01 predecessor: `qualification/adr0008-retained-object-resolver.execution.3edd45a9e5fab548eef62700138236056d9868a9.json` (unchanged)
- Common projection execution: `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json` (unchanged)
- Governing matrix: `qualification/adr0008-source-projection-matrix.v1.json` (unchanged)
- Result: `R01–R02=PASS`; `R03–R04=BLOCKED`; `RETAINED_OBJECT_RESOLVER=BLOCKED`; `implementation_qualified=false`

## R02 evidence

The focused corpus resolves committed source from a real temporary Git repository using an exact commit, root tree, repository-relative path, and blob binding. After commit, the working-tree file is replaced with different dirty bytes; resolution still returns the exact committed blob. Manifest identity, entry identity, storage class, custody identity, source length, and SHA-256 are verified before bytes are returned.

Dirty and non-Git cases use real owner-private `sourceobject.Store` publication and retrieval for both `CONTENT_ADDRESS` and `EMBEDDED_IMMUTABLE`. The tests prove these classes do not invoke the Git resolver. Supplying a Git binding for an immutable-object class fails before object lookup, so another storage class cannot satisfy retrieval.

A digest-only dirty source represented by an unavailable manifest entry returns the generic `UNAVAILABLE` outcome and invokes neither Git nor immutable-object resolution. This review does not assign R03-specific missing, corrupt, withheld, collected, lease, GC, deletion, shallow-history, rewritten-history, or historical-fallback semantics.

The strict mutation corpus rejects substitutions of manifest identity, entry identity, commit, tree, blob, and path. It also rejects a lookup returning neighbor bytes. An exact-HEAD mutation/restore execution independently introduced and detected Git checkout fallback, alternate-class fallback, unavailable-entry lookup, and manifest-binding removal; each restored state passed afterward.

`internal/retainedresolver/` is registered in the integrated ownership inventory. Focused tests, adjacent custody packages, the full repository suite, and the release check all passed at the reviewed revision. The execution record retains each log path and SHA-256 digest; the release record ends `RELEASE CHECK PASS`.

## Retained-track disposition

| Cell | Verdict | Basis | Ceiling |
|---|---|---|---|
| R01 | **PASS** | Carried from its immutable execution record. | This review does not rewrite or requalify R01. |
| R02 | **PASS** | Every exact Git, dirty/non-Git immutable-object, digest-only unavailable, and pre-return identity/custody clause executes at `4955c0fc87cef6867b8c19e6c197285341022b68`. | No R03 lifecycle or R04 publication/replay claim is made. |
| R03 | **BLOCKED** | No complete lifecycle and no-fallback corpus exists. | Missing, corrupt, withheld, collected, shallow, rewritten-history, lease, GC, and deletion outcomes remain unqualified. |
| R04 | **BLOCKED** | No complete immutable-publication race and cold-replay packet exists. | Owner-only no-replace publication and offline replay remain unqualified. |

Because R03 and R04 remain BLOCKED, `RETAINED_OBJECT_RESOLVER=BLOCKED`.

## Carried and historical outcomes

`COMMON_PROJECTION=PASS` is carried only from immutable execution record `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json`.

Historical `L04=FAIL` is unchanged. Its external release exception does not alter the verdict. The governing matrix, R01 execution, common-projection execution, and historical qualification records remain byte-unchanged.

## Claim ceiling

This review qualifies R02 only and carries R01. It does not establish lease retention, garbage-collection behavior, deletion, historical fallback handling, immutable publication races, cold replay, source-graph completeness, semantic authority, semantic acceptance, deployment, or production qualification. `source_graph_complete=UNKNOWN`, `authority=0`, and `implementation_qualified=false` remain required.
