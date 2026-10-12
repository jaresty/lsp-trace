# ADR-0011 Generic V3 Lease Acquisition and Disposable C15 Write-Set — Independent Review V3

Status: **GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `8311f0ac40816e506147d7a0aa0ec1fcf2ae6638`
- Review mode: independent read-only architecture and targeted normal/race verification
- Production authority: **not issued**

## Findings

The v3 RED now discovers the package-private acquisition seam through the compile-safe interface:

```go
privateCallHierarchyLayer1GenericLease() (any, bool)
```

The returned dynamic concrete lease is inspected only through exported `View`, `Callback`, and `Release` methods. This removes impossible reflection over an unexported method without adding a public API or weakening lifecycle assertions.

The disposable boundary remains exactly three implementation files and the exact eleven `owned_method_pair.go` authorities, plus the already finite owner/acquisition/lease/lifecycle seams in `b4_owned_method_pair_lease.go` and result plumbing in `sessionruntime.go`.

Targeted normal and race runs compiled and reached the intended causal predecessor-setter RED with 85 attempts, zero executions, and 85 pre-effect refusals. Comparator tests passed. Pin and prototype → C15 → review → disposal → production-gate ordering are correct.

## Disposition

```text
V3_CAUSAL_RED_GO
V3_DISPOSABLE_C15_WRITESET_GO
```
