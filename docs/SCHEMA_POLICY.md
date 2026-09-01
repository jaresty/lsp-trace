# Schema compatibility policy

Canonical output defaults to `schema_version: lsp-trace.graph.v3`. Explicit `--schema v1` and `--schema v2` retain their exact historical projections. Within each major, field meanings and enum meanings are stable. Additive optional fields may be introduced in a minor release only when existing consumers can ignore them. Removing or renaming fields, changing requiredness, changing edge orientation or identity rules, or changing an existing enum's meaning requires a new schema major version.

New enum values are additive but consumers must treat unknown values as forward-compatible rather than silently mapping them to an existing reason. Canonical ordering and the exclusion of wall-clock metadata remain compatibility obligations. Release review must identify schema-affecting changes and update golden tests, documentation, and the major version together when required.

## Schema v3

`lsp-trace.graph.v3` is the unconditional evidence-bundle contract. It records effective limits, global and request timeouts, concurrency, language ID, expansion and trace configuration, server command/arguments, explicit server-environment names with values omitted, output mode, and every original labeled seed. The sensitivity policy declares `automatic_redaction: false`: omission of environment values and the cwd path is a specific projection rule, not a general redaction guarantee. `process_context` uses role-specific `effective_environment_process_context_digest`, `working_directory_process_context_digest`, and per-name `environment_name_process_context_digest` fields without embedding their hidden inputs; offline verification checks structure and semantic commitment but cannot independently reconstruct or authenticate those inputs. The working-directory digest uses a cleaned absolute process path without symlink resolution, so cwd symlink aliases are not canonicalized. Resolved seeds additionally carry URI and SHA-256 content digest; their aggregate uses `resolved_seed_contents_digest` scoped only to `RESOLVED_SEED_CONTENTS`. Caller provenance is classified `CALLER_ASSERTED`. Every original `invocation.seeds` entry has exactly one same-label `seeds` result, including a failure result; failed seeds remain failures and are excluded from resolved-source identity. No Git, clock, tool-version, server-version, or whole-workspace verification is claimed.

Each seed result's `reached_relation_ids` are its primary relation IDs: direct canonical call-relation IDs whose relation is reached by that exact seed occurrence. They exclude transitive-only relations, sibling/dispatch discovery relations, and relations belonging only to another seed.

Before issuing its embedded semantic receipt, v3 rejects duplicate call-node IDs and dangling targets, edge endpoints, terminal/frontier IDs, and seed prepared/reached IDs. Sibling and dispatch discovery records contain independent embedded nodes; their IDs must match their embedded node identity, but those nodes intentionally need not appear in call `nodes`.

V3 may include an optional `slice` object for the `slice` command. It records the `from_file`, `at`, or `seed_file` start mode when produced by the CLI, the source/workspace scope URI, directional depth bounds, shortest-distance outgoing layers, exact `frontier_node_ids`, successful early-leaf `outgoing_terminal_node_ids`, their `upward_start_node_ids` union, and outgoing relation IDs. Every referenced node and relation must exist in the same native graph, layers must be contiguous from zero, and the frontier must equal the requested downward layer when that layer exists. The upward-start set must be the sorted deduplicated union of frontier and outgoing-terminal IDs. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not an outgoing terminal. New producers also populate each successful seed's bounded causal `reached_node_ids` and `reached_relation_ids`; shared membership is allowed, failed membership is empty, and semantic validation requires their union to equal the global slice graph. The two new upward-set fields are additive and omitted when reading/re-projecting historical v3 slices that predate them. V1 and v2 projections never include slice metadata.

The embedded receipt field `semantic_commitment_digest` uses domain `lsp-trace:semantic-bundle:v3` and scope `CANONICAL_SEMANTIC_BUNDLE_WITHOUT_RECEIPT`. File publication places an `exact_serialized_bytes_digest` receipt beside the artifact inside one immutable generation and atomically replaces `PATH` as the selector for that complete generation. That custody receipt's `directory_durability` is `CHECKED` when directory sync completed or `UNAVAILABLE_ON_PLATFORM` when the runtime cannot request it; the latter is a disclosed limitation and makes no directory-entry persistence claim. Replay artifacts use `replay_input_content_digest`, and their envelope uses `replay_input_manifest_digest`; these identify declared replay inputs and do not prove their origin or availability. The exact-byte digest uses domain `lsp-trace:serialized-output-bytes:v1` and scope `EXACT_SERIALIZED_OUTPUT_BYTES`. These digests are integrity/custody or identity commitments, not authentication, signatures, producer identity, source truth, or independent hidden-input verification. V3 also emits stable call, sibling, and dispatch relation identities plus per-seed-occurrence memberships; discovery evidence remains separate and contributes zero call support.

## Schema v2

`lsp-trace.graph.v2` is preserved when explicitly selected and retains the d153ce8 canonical behavior. It removes the unqualified `summary.complete` field and replaces it with:

- `traversal_complete`: whether every discovered server-reported branch was expanded or explicitly bounded.
- `source_graph_complete: "UNKNOWN"`: the client cannot prove that the language server reported the complete source graph.
- `completeness_scope: "SERVER_REPORTED_CALL_HIERARCHY"`: the exact evidence envelope.

Every v2 receipt includes `tool` identity and the effective invocation parameters. The stable tool name is `lsp-trace`; `tool.version` comes only from `--provenance-tool-version` and is otherwise `"UNKNOWN"`. `invocation.provenance` has the fixed keys `invocation_id`, `caller`, `source`, `source_revision`, `server_version`, and `timestamp`. Their values come only from matching caller-supplied CLI flags; each omitted value is serialized as `"UNKNOWN"`. The CLI never fills these fields from environment variables, user identity, CI metadata, Git state, the current clock, resolved paths, or server responses.

Every v2 document includes `evidence_semantics`, a machine-readable claim ceiling. `call_edges` supports only `server_reported_caller_callee_relation`; it explicitly does not support runtime execution, feature identity, whole-source completeness, or independent source confirmation. `discovery_relations` supports only nomination for separate investigation and has `support_contribution: 0`.

Every v2 document also includes `trace_receipt`. `receipt_version` is `lsp-trace.receipt.v1`. `content_digest` is `sha256:` plus lowercase SHA-256 over the bytes `lsp-trace:trace-receipt:v1`, a zero byte, and the canonical compact v2 JSON document with `trace_receipt` omitted. `digest_scope` names that exclusion as `CANONICAL_GRAPH_WITHOUT_TRACE_RECEIPT`, avoiding a self-referential digest. The digest commits to receipt content; it does not strengthen any evidence class or establish source truth.

Every v2 terminal and frontier boundary includes traversal `provenance`, either `CLIENT_DERIVED` or `SERVER_REPORTED`. This boundary classification is distinct from caller-supplied invocation provenance. A successful empty incoming-call response is `SERVER_REPORTED_NO_INCOMING_CALLS`; client limits, validation, timeout, cancellation, capability, and transport classifications are client-derived.

V2 may include `dispatch_relationships` and `sibling_candidates` when their explicit expansion flags are enabled. These are discovery relationships, not call edges: dispatch relationships assert only a server-reported Type Hierarchy association, while sibling candidates assert only top-level document-symbol membership. Both extensions are omitted from the v1 projection.

When either discovery extension is present, v2 also emits `evidence_receipt`. Its `relations` array projects each unique nomination or association as `evidence_class: "DISCOVERY_NOMINATION"`, `evidence_role: "DISCOVERY_ONLY"`, and `support_contribution: 0`; the envelope's `support_total` is therefore zero. `relation_kind` is `SIBLING_CANDIDATE` or `DISPATCH_ASSOCIATION`. Each relation also records its direction, locator, and caller-supplied source revision. `relation_id` is `sha256:` plus lowercase SHA-256 over the bytes `lsp-trace:evidence-relation:v1`, a zero byte, and canonical compact JSON for the version, evidence class, relation kind, source revision, direction, locator, and semantic endpoints. Relations are ordered by `relation_id` and duplicate semantic tuples collapse. The receipt records evidence provenance and accounting only: it creates no call edge and contributes no support to traversal completeness, source completeness, capability quality, or interoperability claims. The whole `evidence_receipt` envelope is omitted from v1.

V2 also includes language-neutral `capability_quality` observations: `advertised`, `prepare_succeeded`, `incoming_request_successes`, `incoming_edges`, `cross_file_edges`, `unresolved_calls`, and `dynamic_calls`. `cross_module_edges` is `"UNKNOWN"` because LSP Call Hierarchy has no language-neutral module boundary. The evidence counters are derived from diagnostics categorized as `UNRESOLVED_CALL` or `DYNAMIC_CALL`; they describe observed boundaries and never create call edges. Unresolved-call evidence makes traversal incomplete, while dynamic-call evidence alone is advisory. These counters describe observed protocol behavior, not source-graph completeness or language-server correctness.

## V1 compatibility

The implementation retains a v1 projection for callers that explicitly request `lsp-trace.graph.v1` through the traversal API. That projection preserves `summary.complete`, maps `SERVER_REPORTED_NO_INCOMING_CALLS` back to `NO_INCOMING_CALLS`, omits tool identity, invocation and boundary provenance, and capability-quality fields, and otherwise retains v1 canonical field order and meanings. The CLI emits v3 by default; retained historical v1 and v2 artifacts remain valid documents and are not rewritten.

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
