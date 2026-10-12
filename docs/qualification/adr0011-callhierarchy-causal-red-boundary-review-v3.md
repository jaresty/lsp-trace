# ADR-0011 Call-Hierarchy Causal RED Boundary Rereview v3

Status: **GO**

Review target:

- repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- pinned HEAD: `66d11b6a87078d9210c71e3d6ff189c9905aa25a`
- corrected boundary: `docs/qualification/adr0011-callhierarchy-causal-red-boundary.md`
- preserved blocked reviews: `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review.md` and `docs/qualification/adr0011-callhierarchy-causal-red-boundary-review-v2.md`
- preserved pre-existing test delta: exactly 110 additions and zero deletions in `sessionruntime/b4_owned_method_pair_lease_test.go`
- authority: independent boundary rereview only; no implementation or commit authority

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO
```

## Decision

The corrected boundary resolves the v2 blocker and is sufficient for the frozen causal-RED boundary. This GO authorizes only that boundary. It does not approve the preserved partial RED, authorize implementation or GREEN behavior, authorize directional adapters or downstream effects, or authorize a commit.

## Layer ownership

Layer 1 selected-item/handoff correlation is exhaustively restricted to a Layer-2-supplied typed opaque identity, SHA-256 digest, exact byte length, and availability where needed. The tuple is closed: Layer 1 may receive, retain, expose, and compare only those opaque metadata values. It may not receive or expose raw bytes, payload bytes, preimage bytes, spans, ranges, tokens, decoded or semantic fields, serialization, payload/preimage handles, recovery callbacks, or any mechanism that reconstructs the committed material. It may not compute or independently validate the digest, infer target or direction, interpret equality semantically, or confer evidence or authority.

Layer 2 exclusively owns the selected-item/handoff raw payload and preimage bytes, construction and semantic validation of those artifacts, and independent proof that each preimage corresponds to its supplied SHA-256 commitment. The shared-runtime RED cannot satisfy or imply that proof.

Exact external request frames, response frames, returned result bytes, error bytes/components, and the finite externally visible byte surface remain correctly in Layer 1. They are Manager-produced transport-custody facts, not selected-item/handoff correlation preimages. The boundary keeps this distinction explicit.

## Exact external equivalence

`callHierarchyExternalResultSnapshot` is generic and sufficient for the stated RED. It preserves returned result bytes and availability; exact externally observable error presence and concrete class; exact message, data, code, and serialized bytes wherever applicable; independent availability for each optional component; terminal state; transport-message and transport-byte measures; and the bounded externally visible byte surface.

The enabled/disabled obligation requires byte-for-byte and value-for-value equality across those representations. Classification-only, normalization, reconstruction, or semantic equivalence is expressly insufficient. Present-empty and present-zero remain distinct from unavailable, and permitted mutable transport storage must be defensively copied on ingress and egress.

## Generic ledger and snapshot

`callHierarchyTransportEffectLedger` is closed and direction-neutral: transport messages, transport bytes, externally visible bytes, reservations, leases, callbacks, transport diagnostics, and terminal before/after state. Its measurements preserve availability independently from value and do not infer deltas from unavailable endpoints. It excludes correlation commitments, selected-item/handoff payload or preimage bytes, target, direction, method family, occurrence, member, range, grouping, relation, admission, publication, and downstream counters.

The external snapshot likewise contains no correlation commitment or selected-item/handoff payload/preimage field and performs no JSON token-form, successful-empty, returned-item, or protocol-field interpretation.

## Inert Manager boundary

The boundary correctly scopes inertness to the new Manager production/publication behavior: before separate behavior authority, Manager emits no new call-hierarchy owner, ledger, snapshot, lease, callback, reservation, diagnostic, or observation effect. It does not incorrectly declare the repository's existing private owner/copy/lifecycle scaffolding globally inert. The future causal RED must fail only because the narrowly named private scaffold or inert Manager publication behavior is absent.

## Shared RED obligations and deferred effects

The shared obligations are complete for the frozen boundary: exact Manager custody identity and joins; exact request/response frame custody; the closed opaque correlation tuple and API exclusions; terminal and generic resource accounting; defensive copying; callback-error transparency and panic containment; terminal admission closure, callback drain, exact-once release, and concurrent lifecycle behavior; exact enabled/disabled external equivalence; exact generic ledger accounting; zero transport-side refusal/retry/rewrite/normalization/diagnostic/external-byte effects for semantically unsuitable attachments; independent Layer-2 preimage proof; and inert Manager publication.

Directional effects remain deferred to Layer 2: target discovery and selection, method-family and allowlist policy, semantic normalization, incoming/outgoing orientation, range interpretation, occurrence construction, ordinals and multiplicity, zero-site handling, parallel edges, self-loops, grouping, relation admission, qualification, publication, ADR-0007 composition, and every production-visible semantic effect. No passing shared-runtime test authorizes them.

## Preserved partial RED

The existing 110-line addition remains unapproved partial RED. It still contains raw selected/handoff bytes and semantic binding, method allowlist, token-form/successful-empty, and refusal expectations that cross the corrected boundary. The boundary explicitly requires any later authorized RED revision to preserve only its generic causal intent and move raw-preimage proof and directional semantics to Layer 2. This rereview neither edits nor accepts that test.

## Write-set exclusions

The prospective shared-runtime write-set remains maximally bounded to:

1. `sessionruntime/b4_owned_method_pair_lease.go`;
2. `sessionruntime/manager.go`;
3. `sessionruntime/b4_owned_method_pair_lease_test.go`; and
4. conditionally, `sessionruntime/manager_test.go`.

Only private generic Layer-1 scaffolding and causal RED within the corrected contract are contemplated. Public package paths, exported identifiers, public request/result schemas, commands, MCP schemas, retained graph schemas, adapters, providers, qualification artifacts, generated catalogs, testdata corpora, ADR-0007 paths, unrelated source/tests, semantic-vector expansion, evidence production, publication, and production admission remain excluded. Expansion requires a new boundary review.

## Preservation verification

Before this v3 file was created:

- HEAD was exactly `66d11b6a87078d9210c71e3d6ff189c9905aa25a`;
- the corrected boundary SHA-256 was `e98866e38be02fd4711c5f4aff9476621b111fd390ca0d59260326e82b6e4c6e`;
- the original blocked review SHA-256 remained `fb2ef39e326e28c45e844a0db5e0bbdd4b61eb1f165f2f13b94d8b676684490a`;
- the v2 blocked review SHA-256 was `ae4014fbb654ab3aa49527f441169588395396b000b88d21518a56a717fd66dc`;
- `sessionruntime/b4_owned_method_pair_lease_test.go` remained SHA-256 `dc32d3f75c32c4b3fdd2a479c09408b11866db08efff00da85e36359c682899b`; and
- Git reported that test delta as exactly 110 additions and zero deletions.

No implementation, source edit, test edit, regeneration, evidence production, publication, or commit is authorized or performed by this rereview.

```text
ADR0011_CALLHIERARCHY_CAUSAL_RED_BOUNDARY_GO
```
