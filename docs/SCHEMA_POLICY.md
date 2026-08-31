# Schema compatibility policy

Canonical output declares `schema_version: lsp-trace.graph.v1`. Within v1, field meanings and enum meanings are stable. Additive optional fields may be introduced in a minor release only when existing consumers can ignore them. Removing or renaming fields, changing requiredness, changing edge orientation or identity rules, or changing an existing enum's meaning requires a new schema major version.

New enum values are additive but consumers must treat unknown values as forward-compatible rather than silently mapping them to an existing reason. Canonical ordering and the exclusion of wall-clock metadata remain compatibility obligations. Release review must identify schema-affecting changes and update golden tests, documentation, and the major version together when required.

## Schema v2

`lsp-trace.graph.v2` is the default canonical output. It removes the unqualified `summary.complete` field and replaces it with:

- `traversal_complete`: whether every discovered server-reported branch was expanded or explicitly bounded.
- `source_graph_complete: "UNKNOWN"`: the client cannot prove that the language server reported the complete source graph.
- `completeness_scope: "SERVER_REPORTED_CALL_HIERARCHY"`: the exact evidence envelope.

Every v2 terminal and frontier boundary includes `provenance`, either `CLIENT_DERIVED` or `SERVER_REPORTED`. A successful empty incoming-call response is `SERVER_REPORTED_NO_INCOMING_CALLS`; client limits, validation, timeout, cancellation, capability, and transport classifications are client-derived.

V2 also includes language-neutral `capability_quality` observations: `advertised`, `prepare_succeeded`, `incoming_request_successes`, `incoming_edges`, and `cross_file_edges`. `cross_module_edges` is `"UNKNOWN"` because LSP Call Hierarchy has no language-neutral module boundary. These counters describe observed protocol behavior, not source-graph completeness or language-server correctness.

## V1 compatibility

The implementation retains a v1 projection for callers that explicitly request `lsp-trace.graph.v1` through the traversal API. That projection preserves `summary.complete`, maps `SERVER_REPORTED_NO_INCOMING_CALLS` back to `NO_INCOMING_CALLS`, omits provenance and capability-quality fields, and otherwise retains v1 canonical field order and meanings. The CLI emits v2 by default; retained historical v1 artifacts remain valid v1 documents and are not rewritten.
