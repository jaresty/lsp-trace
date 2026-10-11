# ADR-0011 Call-Hierarchy Causal RED Boundary Review

Status: **BLOCKED**

Review target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- pinned HEAD: `66d11b6a87078d9210c71e3d6ff189c9905aa25a`
- reviewed boundary: `docs/qualification/adr0011-callhierarchy-causal-red-boundary.md`
- preserved pre-existing delta: exactly 110 added lines in `sessionruntime/b4_owned_method_pair_lease_test.go`

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_BLOCKED
```

## Exact blocking defects

1. **Layer 1 still admits target-selection artifacts.** The proposed shared-runtime boundary permits “opaque selected-item and handoff commitments” and requires selected/handoff custody in the shared RED. Types named `callHierarchySelectedItemCommitment` and `callHierarchyHandoffCommitment`, including a selected ordinal and selected bytes, are products of target selection rather than generic transport custody. Calling them opaque does not make their production direction-neutral. Under the required strict split, `sessionruntime` may retain exact transported request/response frames and generic custody joins, but the directional adapter must own selection, selected-item commitments, selection ordinals, and semantic handoff construction. Remove selected/handoff commitments from the Layer-1 allowed facts, shared obligations, views, and scaffold; later adapters may derive and retain them from exact transport evidence.

2. **`callHierarchyExternalResultSnapshot` does not preserve exact errors.** It contains only `errorClass` plus `errorAvailable`. That is insufficient to prove exact enabled/disabled equality of externally returned errors: distinct errors can share a class while differing in code, message, data, failure text, or exact encoded bytes. Add a private, immutable, availability-bearing exact external-error snapshot (or exact error bytes/value fields with length/digest as appropriate) and require exact equality without semantic interpretation. Error classification may be retained only in addition to, not instead of, the exact externally observable error.

3. **The stated external-equivalence obligations inherit that insufficiency.** “Error classification and its availability” and “field-for-field equivalence” cannot establish unchanged external errors when the snapshot omits their exact representation. The shared RED obligations must explicitly compare exact returned result bytes, exact externally observable error value/bytes, terminal state, transport-message delta, transport-byte delta, and the bounded external byte surface, each with unavailable distinct from known zero/empty.

## Findings that pass

- HEAD is exactly `66d11b6a87078d9210c71e3d6ff189c9905aa25a`.
- Before this review file, the worktree contained only the untracked boundary document and the pre-existing 110-line test addition; the test delta is preserved unchanged by this review.
- The boundary correctly forbids normalization, occurrence construction, grouping, relation admission, qualification, publication, downstream ADR-0007 semantics, public API/schema changes, CLI/MCP changes, catalog regeneration, and semantic-vector expansion.
- The proposed ledger dimensions are otherwise generic and typed: messages, bytes, external bytes, reservations, leases, callbacks, diagnostics, and terminal transition. Its availability rule correctly distinguishes unavailable from known zero, and its immutable ingress/egress requirement is appropriate.
- Directional obligations are explicitly deferred.
- The existing 110-line test is correctly treated as partial RED rather than accepted complete RED. It preserves useful generic causal intent but presently crosses the boundary through binding/method allowlists, selected-item meaning, token-form/successful-empty interpretation, and refusal assertions.
- Manager publication remains an intentionally RED boundary. Existing owner/copy/lifecycle constructors in `sessionruntime/b4_owned_method_pair_lease.go` are not themselves inert, so the corrected document must continue to scope “inert” specifically to Manager production/publication of the new ledger, snapshot, owner, and lease effects.
- The proposed source/test write-set is private and bounded to `sessionruntime/b4_owned_method_pair_lease.go`, `sessionruntime/manager.go`, `sessionruntime/b4_owned_method_pair_lease_test.go`, and conditionally `sessionruntime/manager_test.go`; no public, CLI, MCP, generated catalog, semantic-vector, adapter, provider, or publication path is authorized.

## Required correction before GO

Revise only the boundary specification so that Layer 1 contains no selected-item or semantic handoff artifact and so that the external snapshot and RED obligations preserve and compare exact externally observable errors. No implementation, source edit, test edit, regeneration, evidence production, publication, or commit is authorized by this review.
