# ADR-0011 Call-Hierarchy Causal RED Boundary Rereview v4

Status: **GO**

Review target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- pinned HEAD: `a4cdf704085c2878a29848c568caa8a32e01c7d3`
- corrected boundary: `docs/qualification/adr0011-callhierarchy-causal-red-boundary.md`
- preserved prior reviews: `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review.md`, `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review-v2.md`, and `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review-v3.md`
- preserved partial RED: exactly 110 additions and zero deletions in `sessionruntime/b4_owned_method_pair_lease_test.go`, SHA-256 `dc32d3f75c32c4b3fdd2a479c09408b11866db08efff00da85e36359c682899b`
- authority: independent boundary rereview only; no implementation or commit authority

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO
```

## Decision

The corrected boundary is sufficient for the frozen declaration-only causal-RED scaffold. This GO authorizes only that boundary. It does not approve the preserved partial RED, authorize implementation or GREEN behavior, authorize directional adapters or downstream effects, authorize evidence production or publication, or authorize a commit.

## Go declaration ownership and exact write-set

`RoundTripRequest` and `RoundTripResult` are declared in `sessionruntime/sessionruntime.go`; Go does not permit another file to extend those struct declarations. Therefore private request/result attachment fields necessarily belong in `sessionruntime/sessionruntime.go`. Their private field types may be declared in another file in the same `sessionruntime` package.

`sessionruntime/manager.go` does not exist and is removed from the boundary. `sessionruntime/manager_test.go` is also excluded. The maximal future scaffold write-set is exactly:

1. `sessionruntime/sessionruntime.go` — private attachment fields on the existing request/result declarations only;
2. `sessionruntime/b4_owned_method_pair_lease.go` — private generic owner/lease/view declarations and the two specified private snapshot/ledger types only; and
3. `sessionruntime/b4_owned_method_pair_lease_test.go` — same-package causal RED and preservation controls only.

Expansion requires a new boundary review.

## Declaration-only attachment feasibility

The scaffold can attach private request and result data without helper constructors, helper accessors, global maps, side tables, or parallel storage. Same-package RED may populate and inspect the private fields directly. A private field declared in `sessionruntime.go` may name a private type declared in `b4_owned_method_pair_lease.go` because both files compile as one package.

The boundary keeps `Manager.RoundTrip` behavior unchanged: it ignores the new fields and emits no owner, ledger, snapshot, lease, callback, reservation, diagnostic, or other observation effect. This is a declaration-only inert scaffold, not Manager plumbing or publication behavior.

## Generic versus directional ownership

The corrected boundary preserves the strict split:

- Layer 1 owns only generic Manager-produced transport custody, exact request/response and external-result/error representations, generic accounting and lifecycle facts, and the closed Layer-2-supplied opaque correlation tuple: typed opaque identity, SHA-256 digest, exact byte length, and availability where needed.
- Layer 1 receives no selected-item or handoff payload/preimage bytes, spans, ranges, decoded or semantic fields, serialization, handles, or recovery callbacks; it may not construct, recompute, independently validate, or semantically interpret the commitment.
- Layer 2 exclusively owns target selection, raw selected/handoff preimages, preimage-to-commitment proof, method-family policy, incoming/outgoing orientation, normalization, occurrence production, grouping, admission, qualification, publication, ADR-0007 composition, and every production-visible semantic effect.

The preserved 110-line partial RED remains unapproved because its semantic binding, allowlist, selected-item bytes/meaning, token-form/successful-empty interpretation, and refusal assertions cross this split. It is preserved unchanged and does not become implementation authority.

## Preserved boundary semantics

The correction retains every substantive commitment accepted or required by the prior reviews:

- exact byte-for-byte and value-for-value enabled/disabled external equivalence;
- exact externally observable error presence and concrete class, message, data, code, serialized bytes, and independent availability wherever applicable;
- unavailable distinct from present-empty and present-zero;
- immutable defensive custody for permitted mutable transport data when later behavior is separately authorized;
- closed generic ledger dimensions and exclusion of correlation, target, direction, occurrence, grouping, admission, publication, and downstream counters;
- zero transport-side refusal, retry, rewrite, normalization, diagnostic, or external-byte effects for semantically unsuitable directional attachments;
- callback-error transparency, panic containment, terminal admission closure, callback drain, exactly-once release, and concurrent lifecycle obligations; and
- opaque-correlation restrictions with raw-preimage proof remaining adapter-owned.

No public API/schema, exported identifier, command, MCP schema, retained graph schema, adapter, provider, generated catalog, testdata corpus, qualification artifact, ADR-0007 path, or unrelated source/test is admitted.

## C15 v2 candidate preservation

The active C15 v2 discovery code enumerates production `*ast.CallExpr` and `*ast.CompositeLit` nodes. Private struct field declarations and private type declarations add neither calls nor composite literals. Under the exact declaration-only scaffold, no new candidate-producing AST is introduced and the C15 v2 candidate surface remains unchanged. Any initializer, constructor, accessor body, map allocation, helper call, composite literal, or other executable/storage mechanism would exceed this reviewed boundary and require renewed review.

## Preservation and authority

Before this v4 file was created:

- HEAD was exactly `a4cdf704085c2878a29848c568caa8a32e01c7d3`;
- Git reported only the corrected boundary document and the preserved 110-line partial RED as modified;
- `sessionruntime/manager.go` and `sessionruntime/manager_test.go` were absent;
- the specified ledger, external-result snapshot, external-error snapshot, and measured-delta scaffold types were absent; and
- `git diff --check` passed.

This rereview creates only this documentation file. It performs no implementation, source edit, test edit, regeneration, evidence production, publication, or commit.

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO
```
