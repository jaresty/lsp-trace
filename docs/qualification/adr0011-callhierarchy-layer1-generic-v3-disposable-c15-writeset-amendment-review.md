# ADR-0011 Call-Hierarchy Layer-1 Generic V3 Disposable C15 Write-Set Amendment — Independent Review

Status: **GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `1419784160f199f9de2f1ee1e784d34e8e0ddd0f`
- Reviewed amendment: `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-disposable-c15-writeset-amendment.md`
- Review mode: independent read-only architecture review
- Production implementation authority: **not issued**

## Findings

The exact three-file disposable boundary is mechanically sufficient and minimal:

1. `sessionruntime/sessionruntime.go`
2. `sessionruntime/owned_method_pair.go`
3. `sessionruntime/b4_owned_method_pair_lease.go`

The `owned_method_pair.go` authority is finite and exact: the previously accepted eight inert symbols plus only:

- the `callHierarchyLayer1RequestAttachment.predecessors` field; and
- `callHierarchyLayer1RequestAttachment.privateCallHierarchyLayer1WithPredecessors([]callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment`.

The setter is value-returning, defensively copies the ordered opaque metadata, and grants no raw-preimage or semantic authority. The revised causal RED can use this exact method to transfer independently authored predecessor inputs before `Manager.RoundTrip`; no test edit, unsafe mutation, hidden table, or fourth production file is required after this correction.

The `sessionruntime.go` attachment/plumbing seam and `b4_owned_method_pair_lease.go` owner/view/lease seam remain finite. No additional production symbol is admitted. Any additional symbol or file is a hard stop.

The ordering is coherent:

```text
disposable isolated prototype
→ canonical C15 enumeration
→ independent finite-delta review
→ prototype disposal
→ separate production write-set/writer gate
```

Tests, v1/v2/v3 corpora, historical C15 artifacts, and all non-admitted files remain immutable. This review grants disposable prototype and enumeration authority only; it grants no production implementation, commit, activation, release, or admission authority.

## Disposition

```text
ADR0011_CALLHIERARCHY_LAYER1_GENERIC_V3_DISPOSABLE_C15_WRITESET_GO
```
