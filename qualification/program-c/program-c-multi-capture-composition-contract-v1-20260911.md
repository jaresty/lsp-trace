# Program C multi-capture composition contract v1

Status: internal, conflict-isolated, not registered, not a producer artifact.

## Boundary

`internal/programccompose` is a stage before Leiden. It composes only structurally validated Graph Provenance V5 captures and does not perform community detection. It returns a strict private composite plus exact constituent envelopes. It does not manufacture a Graph Provenance receipt and is therefore not directly accepted by `programc.Compute`; the deferred integration seam is an explicit Program C projection admission path that consumes `Artifact.Nodes` and `Artifact.Edges` while validating this composite and every constituent with `programccompose.Validate`.

## Resource and policy identity

- Composite schema: `lsp-trace.private.program-c-multi-capture-composite.v1`.
- Policy: `program-c-multi-capture-policy.v1`.
- Policy bytes are exported as `programccompose.PolicyBytes`; `PolicyDigest()` is domain-separated SHA-256 over those exact bytes: `sha256:600a185953a61a0448a57b2ec00ac75341c361f4b9214848dbf32573b9d366b7`.
- Composite identity binds the policy identity, canonical compatibility record, canonical invocation-ID-then-envelope-digest-then-caller-identity ordered constituents, every constituent invocation identity, exact preserved constituent bytes and projections, semantic-work accounting, merged graph records, conservative completeness, and claim ceiling.
- Output digest binds the full artifact except its own digest field. Replay recomputes both identities, validates every original V5 envelope again, and recomposes the complete canonical artifact from those exact envelopes.

## Conservative caps

Inputs are 2 through 16. Total exact envelope bytes are independently capped at 256 MiB (268,435,456 bytes); bytes never count as semantic work. Semantic work is independently capped at 400,000 units with the exact deterministic formula `input_count + traversed_node_records + traversed_edge_records + traversed_call_site_occurrences + traversed_receipt_or_source_supply_records`. Each concrete evidence-receipt container and each relation record within it counts once in the current adapter. Once the converged V5 source-supply contract lands, each concrete receipt, binding, or equivalent source-supply record visited by the adapter must count once. Containers and byte length do not otherwise add work.

The merged graph is at most 10,000 distinct nodes and 100,000 admitted directed call-site occurrences. Traversed records count before deduplication, so repeated exact records still consume semantic work. The 400,000 cap can admit the Program C maxima (16 inputs + 10,000 node records + 100,000 edge records + 100,000 occurrences + 189,984 receipt/source-supply records) without hiding a smaller byte cap. Equality is admitted and +1 fails for both independent caps. All caps fail before Leiden. There is no truncation, sampling, or partial result.

## Exact compatibility matrix

Every input must match exactly on:

| Coordinate | Source |
|---|---|
| envelope schema/version and embedded graph schema ID/version | validated V5 carrier |
| session ID and generation | V5 envelope |
| source revision | embedded invocation provenance |
| provider command and provider version | embedded invocation |
| language | embedded invocation, or unanimous seed language |
| evidence semantics | exact canonical JSON |
| sensitivity policy | exact canonical JSON |
| workspace identity | mandatory caller-bound immutable metadata |
| revision custody | mandatory caller-bound immutable metadata |
| position encoding | mandatory caller-bound immutable metadata |
| acquisition semantics | mandatory caller-bound immutable metadata |
| privacy policy | mandatory caller-bound immutable metadata |

The caller metadata fills coordinates unavailable from Graph Provenance V5. It is preserved and digest-bound but remains caller asserted; it is not producer authentication. Empty, mixed, unknown, or ambiguous values fail closed. Cross-session, cross-generation, and cross-revision composition is prohibited.

Invocation ID is deliberately not a homogeneous compatibility coordinate: separate frozen acquisitions from the same compatible managed session naturally have distinct invocation IDs. Every non-empty invocation ID remains constituent-local, is preserved exactly beside the exact envelope, is the first canonical constituent sort coordinate (followed by envelope digest and caller identity), and is bound into composite identity. The compatibility record contains no single-invocation claim, and the composite does not represent itself as one native capture.

## Union and conflict rules

Node membership is exactly the union of `graph_v5.nodes`; retained source or diagnostic bytes never select nodes. CALLS evidence is exactly the union of `graph_v5.edges` and their reported call-site occurrences. No edge is inferred from names, siblings, source proximity, callbacks, or co-membership.

Nodes key by stable node ID; edges key by relation ID. Equal IDs deduplicate only when canonical typed JSON is byte-equivalent. Any unequal content for an equal ID fails. Distinct relation IDs and distinct call-site occurrences remain distinct, including parallel occurrences and self-loops. Production V5 validation normally makes content-derived ID collisions unreachable; the composer still checks them defensively.

Each exact original envelope is retained as base64 together with caller identity, artifact digest, byte length, envelope schema, embedded graph digest/length/schema ID, session, generation, and exact invocation ID. Canonical raw projections additionally preserve invocation/seeds, frontier, diagnostics, summary/completeness, and slice/traversal state per constituent; replay checks every projection against the retained envelope. Duplicate artifact bytes with distinct caller identities remain distinct constituents, preserving capture/receipt multiplicity. Conflicting caller identity-to-digest mappings fail.

## Completeness and claims

Per-input invocation/seeds, traversal/slice, frontier, summary/completeness, diagnostics, and outer diagnostics remain in exact constituent bytes; graph-level projections are additionally preserved in canonical constituent order. `AllTraversalComplete` is conjunction, `AnyTruncated` is disjunction, and `WholeWorkspace` is always false. Union never establishes whole-workspace completeness.

The composite claim ceiling is structural server-reported CALLS union only. It cannot establish feature identity, ownership, architecture, runtime execution, whole-source/workspace completeness, producer authentication, permission, or production authority. Original custody and receipts remain constituent-local and are never collapsed into a stronger composite receipt.

## Deferred integration

Do not pass composite bytes to current `programc.Compute`: it correctly requires one genuine Graph Provenance V5 source binding. This implementation predates the converged V5 source-supply contract and does **not** claim source-supply composition complete. Its parser enumerates the currently known V5 projection, rejects unknown/future fields, marks every constituent `BLOCKED_PENDING_CONVERGED_V5_CONTRACT`, and specifically fails closed on source-supply-shaped fields.

Integration blocker: after the converged V5 source-supply contract lands, the local V5 adapter must be revised explicitly to parse, preserve, count, canonical-order, and exact-conflict-check every source-supply receipt and binding before such captures can be admitted. It must also prove that retained source bytes introduce no graph nodes. No guessed schema or silent unknown-field acceptance is permitted.

A later non-overlapping integration should add a Program C admission function accepting a validated `programccompose.Artifact` (or a narrow projection interface), preserve all constituent bindings in Program C output, and reuse existing occurrence/cap checks. CLI, MCP registry/contracts, shared schemas, presentation, README, and general docs are intentionally deferred.
