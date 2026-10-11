# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set Amendment — Independent Review V2

Status: **GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `5fc218c677a0caf4537b0c2f475d8d1f3800755f`
- Review mode: independent read-only architecture review
- Supersedes the representation conclusion in the prior review
- Production authority: **not issued**

## Findings

The exact three-file disposable boundary remains sufficient and minimal:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The exact eleven-authority `owned_method_pair.go` ceiling is coherent: the prior eight symbols plus:

- private immutable holder type `callHierarchyOpaqueCommitmentList`;
- comparable attachment field `predecessors *callHierarchyOpaqueCommitmentList`;
- value-returning `privateCallHierarchyLayer1WithPredecessors` setter with the exact RED signature.

The holder defensively owns the ordered metadata slice and is never mutated after construction. The attachment remains comparable because it stores only the holder pointer. Pointer identity conveys no commitment equality or meaning; views and observations compare/copied metadata values. The representation resolves the disposable prototype’s immutable-test compilation blocker without changing tests or granting raw-preimage or semantic authority.

No fourth production file or additional symbol is admitted. The disposable prototype → canonical C15 enumeration → independent finite-delta review → disposal → separate production gate ordering remains mandatory. Any additional requirement is a hard stop.

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_GO
```
