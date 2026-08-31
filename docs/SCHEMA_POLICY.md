# Schema compatibility policy

Canonical output declares `schema_version: lsp-trace.graph.v1`. Within v1, field meanings and enum meanings are stable. Additive optional fields may be introduced in a minor release only when existing consumers can ignore them. Removing or renaming fields, changing requiredness, changing edge orientation or identity rules, or changing an existing enum's meaning requires a new schema major version.

New enum values are additive but consumers must treat unknown values as forward-compatible rather than silently mapping them to an existing reason. Canonical ordering and the exclusion of wall-clock metadata remain compatibility obligations. Release review must identify schema-affecting changes and update golden tests, documentation, and the major version together when required.

## Schema v2

`lsp-trace.graph.v2` is the default canonical output. It removes the unqualified `summary.complete` field and replaces it with:

- `traversal_complete`: whether every discovered server-reported branch was expanded or explicitly bounded.
- `source_graph_complete: "UNKNOWN"`: the client cannot prove that the language server reported the complete source graph.
- `completeness_scope: "SERVER_REPORTED_CALL_HIERARCHY"`: the exact evidence envelope.

Every v2 terminal and frontier boundary includes `provenance`, either `CLIENT_DERIVED` or `SERVER_REPORTED`. A successful empty incoming-call response is `SERVER_REPORTED_NO_INCOMING_CALLS`; client limits, validation, timeout, cancellation, capability, and transport classifications are client-derived.

V2 also includes language-neutral `capability_quality` observations: `advertised`, `prepare_succeeded`, `incoming_request_successes`, `incoming_edges`, `cross_file_edges`, `unresolved_calls`, and `dynamic_calls`. `cross_module_edges` is `"UNKNOWN"` because LSP Call Hierarchy has no language-neutral module boundary. The evidence counters are derived from diagnostics categorized as `UNRESOLVED_CALL` or `DYNAMIC_CALL`; they describe observed boundaries and never create call edges. Unresolved-call evidence makes traversal incomplete, while dynamic-call evidence alone is advisory. These counters describe observed protocol behavior, not source-graph completeness or language-server correctness.

## V1 compatibility

The implementation retains a v1 projection for callers that explicitly request `lsp-trace.graph.v1` through the traversal API. That projection preserves `summary.complete`, maps `SERVER_REPORTED_NO_INCOMING_CALLS` back to `NO_INCOMING_CALLS`, omits provenance and capability-quality fields, and otherwise retains v1 canonical field order and meanings. The CLI emits v2 by default; retained historical v1 artifacts remain valid v1 documents and are not rewritten.

Every v1 result contains `schema_version`, `invocation`, `capabilities`, `targets`, `nodes`, `edges`, `terminals`, `frontier`, `diagnostics`, and `summary`. `summary.complete` means only that all discovered server-reported branches reached successful expansion or a natural server response. It does not establish source-graph completeness. `summary.truncated` reports user-bound truncation.

## Reason enum

The v1 reason values are:

- `NO_INCOMING_CALLS`: the server reported no incoming calls for that expansion.
- `PREPARE_RETURNED_NO_ITEM`: target preparation returned no symbol.
- `INCOMING_RETURNED_NULL`: the server returned null instead of an incoming-call list.
- `EXTERNAL_URI`: the item cannot be expanded within the workspace envelope.
- `UNSUPPORTED_CALL_HIERARCHY`: initialize capabilities do not advertise Call Hierarchy.
- `SERVER_ERROR`: the server returned an operational error.
- `INVALID_SERVER_RESPONSE`: a response violated the expected protocol shape.
- `REQUEST_TIMEOUT`: an LSP request reached its local deadline.
- `GLOBAL_TIMEOUT`: the traversal-wide deadline expired.
- `CANCELLED`: traversal context was cancelled.
- `MAX_DEPTH`: the configured depth bound prevented expansion.
- `MAX_NODES`: the configured node bound prevented expansion.
- `NODE_ID_COLLISION`: distinct normalized items produced the same stable identity.

New values may be added within v1, so consumers must preserve or display unknown reasons rather than coercing them. Changing an existing value's meaning requires a schema-major change.
