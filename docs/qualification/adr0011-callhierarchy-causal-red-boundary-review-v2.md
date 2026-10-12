# ADR-0011 Call-Hierarchy Causal RED Boundary Rereview v2

Status: **BLOCKED**

Review target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- pinned HEAD: `66d11b6a87078d9210c71e3d6ff189c9905aa25a`
- corrected boundary: `docs/qualification/adr0011-callhierarchy-causal-red-boundary.md`
- preserved original blocked review: `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review.md`
- preserved pre-existing test delta: exactly 110 added lines in `sessionruntime/b4_owned_method_pair_lease_test.go`
- authority: independent boundary rereview only; no implementation or commit authority

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_BLOCKED
```

## Governing interpretation

The governing split permits `sessionruntime` to custody only generic transport facts and opaque, direction-supplied selected/handoff correlation **digest, length, and identity tokens**. Those tokens remain uninterpreted: Layer 1 may not construct them, assign or recover meaning, inspect target or direction, derive occurrences, validate suitability, or confer evidentiary or production authority. Layer 2 exclusively owns selection/handoff semantics and every resulting directional effect.

This rereview treats that supplied boundary as narrower than custody of raw selected-item or handoff bytes. A digest may commit to direction-owned bytes without admitting those bytes themselves into shared-runtime custody.

## Blocking defect

The corrected boundary still permits raw selected/handoff commitment bytes in Layer 1:

- line 24 admits opaque correlation commitments as “exact bytes, lengths, digests, and uninterpreted identities”; and
- line 85 again permits `sessionruntime` to store and copy “bytes, length, digest, and opaque identity.”

That exceeds the governing allowance of uninterpreted direction-supplied digest/length/identity tokens. Raw selected or handoff bytes are the direction-owned artifact preimage, not merely an opaque generic correlation token. Their admission also makes the private shared-runtime scaffold capable of retaining target-bearing material even while semantic inspection is forbidden.

The correction must remove raw selected/handoff bytes from Layer-1 allowed facts, obligations, views, constructors, copy requirements, and future scaffold. Layer 1 may retain only the direction-supplied digest, length, and opaque identity tokens and may compare or defensively copy those tokens without constructing, decoding, validating, or authorizing their meaning.

## Findings that pass

### Exact external-error representation

The prior blocker is corrected. The boundary now requires `callHierarchyExternalErrorSnapshot` to preserve error presence and concrete class plus exact externally observable message, data, code, and serialized bytes wherever applicable, with independent availability for each component. Enabled/disabled comparison is expressly byte-for-byte and value-for-value rather than classification-only, normalized, reconstructed, or merely semantically equivalent. This is sufficient for exact external-error equality at the stated private RED boundary.

### Generic ledger and snapshot sufficiency

Apart from the raw correlation-byte defect above, the transport ledger is generic and exhaustive for the stated shared boundary: transport messages, transport bytes, externally visible bytes, reservations, leases, callbacks, diagnostics, and terminal before/after state. Availability is distinct from known zero, and typed before/after values or exact available deltas avoid inference from unavailable endpoints.

The external snapshot is likewise sufficient for generic enabled/disabled comparison: result bytes and availability, exact external-error representation and component availability, terminal state, transport message/byte measures, and the bounded externally visible byte surface. It forbids JSON-token interpretation, successful-empty classification, item inspection, and protocol-field semantics.

### Inert Manager RED boundary

The document correctly keeps the new Manager boundary inert: before separate behavior authority, Manager emits no new call-hierarchy owner, ledger, snapshot, lease, callback, reservation, diagnostic, or observation effect. The future RED must fail only because the narrowly named private scaffold or Manager production is absent. Existing owner/copy/lifecycle behavior is not incorrectly declared globally inert.

### Shared obligations and deferred directional effects

The shared obligations appropriately cover Manager custody joins, exact frames, terminal/resource accounting, defensive copying, callback containment, lifecycle drain and release, exact enabled/disabled external equivalence, ledger accounting, and zero transport-side semantic refusal or rewrite effects.

Target selection, method-family policy, semantic normalization, incoming/outgoing orientation, range and occurrence production, multiplicity, grouping, relation admission, qualification, publication, ADR-0007 composition, and production-visible semantic effects remain explicitly deferred to Layer 2. No shared-runtime passing test can satisfy or authorize them.

The sole exception is the blocking raw-byte overreach: the otherwise valid opaque-correlation obligation must be narrowed to direction-supplied digest/length/identity tokens only.

### Write-set and public exclusions

The prospective implementation write-set is bounded to private `sessionruntime` source and test paths:

1. `sessionruntime/b4_owned_method_pair_lease.go`;
2. `sessionruntime/manager.go`;
3. `sessionruntime/b4_owned_method_pair_lease_test.go`; and
4. conditionally, `sessionruntime/manager_test.go`.

Public package paths, exported identifiers, public request/result schemas, commands, MCP schemas, retained graph schemas, adapters, providers, qualification artifacts, generated catalogs, testdata corpora, ADR-0007 paths, and unrelated files remain excluded. This rereview does not authorize that future write-set.

## Preservation verification

Before creating this rereview:

- HEAD was exactly `66d11b6a87078d9210c71e3d6ff189c9905aa25a`;
- the original blocked review SHA-256 was `fb2ef39e326e28c45e844a0db5e0bbdd4b61eb1f165f2f13b94d8b676684490a`;
- the corrected boundary SHA-256 was `0afdece1cbd851786234f6eef0531d831ca55e84e23a6f4b84b9d320f995ba96`;
- `sessionruntime/b4_owned_method_pair_lease_test.go` had SHA-256 `dc32d3f75c32c4b3fdd2a479c09408b11866db08efff00da85e36359c682899b`; and
- Git reported its delta as exactly 110 additions and zero deletions.

The test is still partial RED and still crosses the intended boundary through semantic binding, allowlists, selected-item meaning, successful-empty/token-form meaning, and refusal assertions. This rereview neither edits nor approves it.

## Required correction before GO

Revise only the boundary specification so Layer 1 admits no raw selected-item or handoff bytes and retains only uninterpreted, direction-supplied digest/length/identity correlation tokens. Preserve all current prohibitions on construction, meaning, target, direction, occurrence, suitability validation, evidence, and authority, with Layer 2 owning all semantics and effects.

No implementation, source edit, test edit, regeneration, evidence production, publication, or commit is authorized.

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_BLOCKED
```
