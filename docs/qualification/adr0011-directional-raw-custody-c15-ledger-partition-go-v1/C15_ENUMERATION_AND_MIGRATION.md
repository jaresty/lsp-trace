# C15 Stage A / Stage B and migration contract

## Prior evidence

The immutable predecessor chain remains evidence: C15 two-stage contract V2, lease-source V2, private-interface V1/V2/V3 blocked packages, two-stage bridge/binding packages, production-interface V1 proposal, and the externally retained blocked-v7 bundle. This successor neither edits nor normalizes them.

## Stage A — pre-implementation

Preserve the existing 20 planned obligations and predecessor edges: six component families each have reservation, owned-copy, and zero-charge alias rows; one whole-acquisition transfer row; one terminal release row. They remain `PLANNED_RESERVATION|PLANNED_COPY|PLANNED_ALIAS|PLANNED_TRANSFER|PLANNED_RELEASE`, never actual source identities or `NECESSARY|REUSE` findings.

Add planned accounting obligations without rewriting old IDs:

1. directional partition identity and one-slot reservation;
2. existing/general partition identity binding;
3. checked combined Manager ceiling (`67,108,864`);
4. cross-partition transfer refusal;
5. same-partition alias binding;
6. partition-scoped release/reuse and fresh identity;
7. `sessionruntime/b4_explicit_bytes_c15_private.go` migration seam preserving legacy layout and general cap.

These are additive planned successors whose predecessor sets reference the applicable prior 20 rows. Old planned rows remain historical evidence; no in-place ID replacement is allowed.

## Migration consequences

- `privateB4ByteLedger` remains 24,600 bytes, 1,024 entries, and bounded by 33,554,432 existing/general bytes.
- `privateB4ByteLedgerV2` and existing leases retain current layout/semantics unless a separately reviewed representation successor is unavoidable.
- The new directional partition does not debit, expand, or borrow from the general ledger.
- Manager admission must observe both partition snapshots under one checked transaction while preserving partition-local counters.
- Existing C15 census rows keep their actual source identities. New directional/accounting sites receive new planned IDs in Stage A and source-derived IDs only in Stage B.
- Prior `47 + 1` identity accounting and 20-wrapper mapping remain predecessor evidence, not the new denominator; the next generator must explicitly reconcile preserved, added, changed, removed, and unchanged rows.

## Stage B — post-implementation only

Stage B is forbidden until separately authorized implementation is committed. A canonical deterministic enumerator must derive actual IDs from exact path, symbol, operation, component, partition, backing-identity rule, and source digest. It must emit complete before/after sets and classify every actual row independently as exactly `NECESSARY`, `REUSE`, `FALSE_POSITIVE`, or `UNRESOLVED`; `UNRESOLVED` must be zero before any later implementation GO.

Planned-to-actual lineage is many-to-many and correction-safe: `REALIZED`, `DISAPPEARED`, `SPLIT`, `MERGED`, or `EMERGENT`, with complete predecessor/successor sets. Planned IDs never become actual IDs. Two identical runs from the same source manifest must be byte-identical.

Required Stage B reconciliation proves separately:

```text
existing/general live <= 33,554,432
directional live <= 33,554,432
general + directional <= 67,108,864
one live backing identity -> one C15 ownership classification
one attempt/backing -> at most one byte-bearing directional component
alias bytes = 0
cross-partition transfer count = 0
all releases and reuse identities reconcile exactly
```

AUTHORED_NOT_GO
