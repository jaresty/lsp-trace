# ADR 0008 retained object resolver R03 qualification review

- Reviewed revision: `e42ff78805fd3838cb81ef2cd154ec166cfd3066`
- Execution record: `qualification/adr0008-retained-object-resolver.execution.e42ff78805fd3838cb81ef2cd154ec166cfd3066.json`
- R02 predecessor: `qualification/adr0008-retained-object-resolver.execution.4955c0fc87cef6867b8c19e6c197285341022b68.json` (unchanged)
- Common projection execution: `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json` (unchanged)
- Governing matrix: `qualification/adr0008-source-projection-matrix.v1.json` (unchanged)
- Result: `R01–R03=PASS`; `R04=BLOCKED`; `RETAINED_OBJECT_RESOLVER=BLOCKED`; `implementation_qualified=false`

## R03 evidence

The focused corpus assigns exact terminal codes to `MISSING`, `CORRUPT`, `WITHHELD`, `COLLECTED`, `SHALLOW_HISTORY`, `REWRITTEN_HISTORY`, `GC_COLLECTED`, and `DELETED`. None is `LIMIT`, all terminal paths return the zero result, manifest-withheld/collected and exact lifecycle states terminate before storage lookup, and an unknown lifecycle does not relabel a generic selected-resolver failure.

Real private `sourceobject.Store` interventions delete and corrupt the exact retained object. Resolution returns `MISSING` and `CORRUPT` respectively, without alternate storage-class or checkout recovery. The manifest and graph byte slices remain unchanged across terminal manifest outcomes.

A private Git lease manager publishes exact refs beneath `refs/lsp-trace/leases/`. In real temporary repositories, an active lease preserves an otherwise unreachable commit through reflog expiry and `git gc --prune=now`. Explicit release and deterministic expiry sweep remove only the exact ref; subsequent GC collects the unreferenced commit. Lease management neither resolves source bytes nor adds retry, network, checkout, provider, live-session, or alternate-resolver capability.

The isolated mutation/restore corpus detects terminal-code collapse, lifecycle fallthrough into Git lookup, loss of active-lease retention, failure to release collection eligibility, and graph/manifest input mutation. Every mutation produces its assertion-specific failure and every restored exact revision passes.

`internal/retainedlifecycle/` and `internal/retainedresolver/` are registered in the integrated ownership inventory. Focused tests, adjacent custody packages, race tests, the full repository suite, and the release check passed at the reviewed revision. The release record ends `RELEASE CHECK PASS`.

## Retained-track disposition

| Cell | Verdict | Basis | Ceiling |
|---|---|---|---|
| R01 | **PASS** | Carried from its immutable execution record. | This review does not rewrite or requalify R01. |
| R02 | **PASS** | Carried from its immutable execution record. | This review does not broaden exact retrieval custody. |
| R03 | **PASS** | Exact lifecycle codes, no-fallback cardinality, real deletion/corruption, real lease/GC/release/expiry, immutable custody bytes, and mutation witnesses execute at `e42ff78805fd3838cb81ef2cd154ec166cfd3066`. | No R04 publication/replay claim is made. |
| R04 | **BLOCKED** | No complete immutable-publication race and cold-replay packet exists. | Owner-only no-replace publication and offline replay remain unqualified. |

Because R04 remains BLOCKED, `RETAINED_OBJECT_RESOLVER=BLOCKED`.

## Carried and historical outcomes

`COMMON_PROJECTION=PASS` is carried only from immutable execution record `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json`.

Historical `L04=FAIL` is unchanged. Its external release exception does not alter the verdict. The governing matrix, R01/R02 executions, common-projection execution, and historical qualification records remain byte-unchanged.

## Claim ceiling

This review qualifies R03 only and carries R01–R02. It does not establish immutable publication races, cold offline replay, production deployment, secure erasure, source-graph completeness, semantic authority, or semantic acceptance. `source_graph_complete=UNKNOWN`, `authority=0`, `accepted=false`, and `implementation_qualified=false` remain required.
