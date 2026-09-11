# Program C multi-capture composition contract v1

Status: internal, conflict-isolated, not registered, not a producer artifact.

## Boundary

`internal/programccompose` is a stage before Leiden. It composes only structurally validated Graph Provenance V5 captures and does not perform community detection. It returns a strict private composite plus exact constituent envelopes. It does not manufacture a Graph Provenance receipt and is therefore not directly accepted by `programc.Compute`; the deferred integration seam is an explicit Program C projection admission path that consumes `Artifact.Nodes` and `Artifact.Edges` while validating this composite and every constituent with `programccompose.Validate`.

## Resource and policy identity

- Composite schema: `lsp-trace.private.program-c-multi-capture-composite.v1`.
- Policy: `program-c-multi-capture-policy.v1`.
- Policy bytes are exported as `programccompose.PolicyBytes`; `PolicyDigest()` is domain-separated SHA-256 over those exact bytes.
- Composite identity binds the policy identity, canonical compatibility record, canonical digest-ordered constituents, exact preserved constituent bytes, merged graph records, conservative completeness, and claim ceiling.
- Output digest binds the full artifact except its own digest field. Replay recomputes both identities and validates every original V5 envelope again.

## Conservative caps

Inputs are 2 through 16. Total exact envelope bytes are at most 256 MiB. Work units (`input bytes + admitted nodes + admitted occurrences`) are at most 400,000. The merged graph is at most 10,000 nodes and 100,000 directed occurrences. Sixteen inputs allows bounded multi-seed composition while limiting validation amplification; 256 MiB is below two maximum 192 MiB V5 envelopes and prevents multiplying the existing per-envelope ceiling; the work cap bounds canonical map/sort work independently of byte size. All caps fail before Leiden. There is no truncation, sampling, or partial result.

## Exact compatibility matrix

Every input must match exactly on:

| Coordinate | Source |
|---|---|
| envelope schema/version and embedded graph schema ID/version | validated V5 carrier |
| session ID and generation | V5 envelope |
| source revision and invocation ID | embedded invocation provenance |
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

## Union and conflict rules

Node membership is exactly the union of `graph_v5.nodes`; retained source or diagnostic bytes never select nodes. CALLS evidence is exactly the union of `graph_v5.edges` and their reported call-site occurrences. No edge is inferred from names, siblings, source proximity, callbacks, or co-membership.

Nodes key by stable node ID; edges key by relation ID. Equal IDs deduplicate only when canonical typed JSON is byte-equivalent. Any unequal content for an equal ID fails. Distinct relation IDs and distinct call-site occurrences remain distinct, including parallel occurrences and self-loops. Production V5 validation normally makes content-derived ID collisions unreachable; the composer still checks them defensively.

Each exact original envelope is retained as base64 together with caller identity, artifact digest, byte length, envelope schema, embedded graph digest/length/schema ID, session, and generation. Duplicate artifact bytes with distinct caller identities remain distinct constituents, preserving capture/receipt multiplicity. Conflicting caller identity-to-digest mappings fail.

## Completeness and claims

Per-input summary and outer diagnostics remain in exact constituent bytes; each graph summary is additionally preserved in canonical constituent order. `AllTraversalComplete` is conjunction, `AnyTruncated` is disjunction, and `WholeWorkspace` is always false. Union never establishes whole-workspace completeness.

The composite claim ceiling is structural server-reported CALLS union only. It cannot establish feature identity, ownership, architecture, runtime execution, whole-source/workspace completeness, producer authentication, permission, or production authority. Original custody and receipts remain constituent-local and are never collapsed into a stronger composite receipt.

## Deferred integration

Do not pass composite bytes to current `programc.Compute`: it correctly requires one genuine Graph Provenance V5 source binding. A later non-overlapping integration should add a Program C admission function accepting a validated `programccompose.Artifact` (or a narrow projection interface), preserve all constituent bindings in Program C output, and reuse existing occurrence/cap checks. CLI, MCP registry/contracts, shared schemas, presentation, README, and general docs are intentionally deferred.
