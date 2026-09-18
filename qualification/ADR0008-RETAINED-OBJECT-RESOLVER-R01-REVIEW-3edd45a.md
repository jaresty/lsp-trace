# ADR 0008 retained object resolver R01 qualification review

- Reviewed revision: `3edd45a9e5fab548eef62700138236056d9868a9`
- Execution record: `qualification/adr0008-retained-object-resolver.execution.3edd45a9e5fab548eef62700138236056d9868a9.json`
- Predecessor execution: `qualification/adr0008-source-projection-matrix.execution.5d392f9710225bd1761af67fd9d851c2ff4b2f82.json` (unchanged)
- Common projection execution: `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json` (unchanged)
- Governing matrix: `qualification/adr0008-source-projection-matrix.v1.json` (unchanged)
- Result: `R01=PASS`; `R02–R04=BLOCKED`; `RETAINED_OBJECT_RESOLVER=BLOCKED`; `implementation_qualified=false`

## R01 evidence

The new private `internal/retainedmanifest` package builds and admits one canonical retained-availability manifest bound to exact admitted Graph Provenance V5 bytes and capture identity. The manifest owns custody and availability identity only. It cannot add graph facts, semantic authority, semantic acceptance, resolver outcomes, lease state, GC state, publication state, or replay claims.

The focused corpus establishes:

- exact V5 byte digest, byte length, schema identity, capture identity, and ordered availability-entry binding;
- sensitivity to source digest, source length, storage class, qualification, privacy, availability, role, graph subject, logical source, range, position encoding, request policy, and custody binding;
- byte-identical canonical output under semantically irrelevant entry permutation;
- rejection of unknown or missing JSON members, semantic-owner or authority substitution, non-canonical ordering, and entry-identity substitution;
- exact preservation of the Graph Provenance V2 and V3 predecessor schema bytes and independent historical readers;
- registration of `internal/retainedmanifest/` in the integrated ownership inventory.

The focused suite, full repository suite, and release check all passed at the reviewed revision. Their log paths and SHA-256 digests are recorded in the execution record. The release check ended `RELEASE CHECK PASS`.

## Counterfactual evidence

Each of the six R01 properties was independently mutated during implementation review. The guards detected exact-binding substitution, entry-identity substitution, non-canonical order, semantic-ceiling expansion, permissive unknown-member admission, and predecessor-byte mutation. The files were restored before the committed revision. These counterfactuals discriminate the R01 properties; they do not qualify R02–R04.

## Retained-track disposition

| Cell | Verdict | Basis | Ceiling |
|---|---|---|---|
| R01 | **PASS** | Exact V5 binding, complete availability identity, canonical determinism, custody/availability-only semantics, strict admission, predecessor preservation, ownership registration, full-suite and release evidence all execute at `3edd45a9e5fab548eef62700138236056d9868a9`. | No runtime resolver, lease, GC, deletion, publication, replay, CLI, MCP, or operation behavior is claimed. |
| R02 | **BLOCKED** | No qualifying complete real Git/content-addressed resolver corpus exists. | Commit/tree/blob, dirty worktree, non-Git, digest-only, and alternate storage-class outcomes remain to be executed. |
| R03 | **BLOCKED** | No qualifying complete lifecycle corpus exists. | Lease retention, GC, deletion, withheld/collected source, shallow clone, rewritten history, and all no-fallback outcomes remain to be executed. |
| R04 | **BLOCKED** | No qualifying retained publication/replay corpus exists. | No-replace race receipts, immutable publication, and cold replay remain to be executed, including real Linux evidence where required. |

Because R02–R04 remain BLOCKED, `RETAINED_OBJECT_RESOLVER=BLOCKED`.

## Carried and historical outcomes

`COMMON_PROJECTION=PASS` is carried only from immutable execution record `qualification/adr0008-common-projection.execution.5e624471011cb679f731bbc4079a4229f2f986b7.json`; this review does not requalify or rewrite C01–C06.

Historical `L04=FAIL` is unchanged. Its accepted external release exception does not alter that verdict. The governing matrix and all predecessor execution and review records remain byte-unchanged.

## Claim ceiling

This review qualifies R01 only. It does not establish source availability at replay time, real Git object retention, lease or GC behavior, deletion, publication atomicity, cold replay, source-graph completeness, semantic authority, semantic acceptance, deployment, or production qualification. `source_graph_complete=UNKNOWN`, `authority=0`, and `implementation_qualified=false` remain required.
