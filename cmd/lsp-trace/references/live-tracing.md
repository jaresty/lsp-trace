# Live tracing and census

## Automatic managed-session prerequisite

Before `incoming`, `slice`, `trace`, `census`, or transient structural-context analysis, call `lsp_session_v1_list`. Reuse an exact-workspace session only when its exact generation is `READY`. If no exact match exists and the target is already an exact registered Git worktree, identify exactly one READY parent session from the same Git worktree registry and call `lsp_session_v1_derive_workspace` with only `session_id`, `generation`, and the target's canonical absolute local `file:` `workspace_uri`. Continue only when the result itself is `READY`; use its returned opaque session identity and generation.

Do not create, move, or remove a worktree as an implicit analysis prerequisite. Do not automatically retry another parent, increase bounds, start an independently configured replacement language server, or infer a session from language, path similarity, source text, or repository proximity. An absent, ambiguous, non-READY, unregistered, or failed derivation is an explicit unmet prerequisite. Direct CLI server launch remains available only when the caller explicitly supplies or selects trusted launch configuration; it is not an automatic fallback from managed-session setup.

This setup grants no additional evidence authority. It only aligns the host-owned launch profile and provider configuration with one registered worktree so subsequent live operations can observe that workspace.

## Profiles and coordinates

Prefer an explicitly selected named profile:

```sh
lsp-trace incoming --workspace /path/to/workspace --profile typescript --at path/to/file.ts:LINE:COLUMN
```

Profiles are never inferred from language IDs, paths, or extensions. Without `--profile`, config files are ignored and legacy flags retain their behavior. `--server` may override a selected profile command. Default configuration loads user then workspace TOML; `--config PATH` replaces both. CLI command fields override project profile fields, which override user fields. Unknown fields, malformed TOML, missing profiles, plaintext profile environment values, and missing referenced environment variables fail closed.

CLI `--at` is one-based. MCP and retained LSP positions are zero-based. Character offsets use the generation's negotiated encoding (`utf-8` bytes, `utf-16` code units, or `utf-32` code points), never visual columns. The server owns the encoding; omission defaults to `utf-16`.

A direct trusted server command remains available:

```sh
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server language-server \
  --server-arg --stdio \
  --at path/to/file.ts:LINE:COLUMN
```

`language-server` is a placeholder. Pass required arguments and environment explicitly; do not infer trusted execution configuration.

## Exact seeds

`--at` is repeatable. A seed file preserves distinct labels:

```json
{"seeds":[
  {"label":"report-download","at":"src/reports.ts:42:8"},
  {"label":"scheduled-export","at":"src/export.ts:19:4"}
]}
```

Paths resolve against the workspace. Labels are unique and match `[A-Za-z][A-Za-z0-9._-]*`. Unknown fields fail. Failed seeds remain represented rather than disappearing. Use each seed's stored result, membership, reached IDs, and native references; never attribute the deduplicated union graph to every seed.

## Multiple exact symbols

A semantic symbol operation accepts one exact symbol per call. For multiple symbols, make one independent semantic call per symbol rather than combining their names into one request:

```text
results = []
for each exact_symbol in exact_symbols:
    envelope = semantic_call(exact_symbol)
    results.append({symbol: exact_symbol, envelope: envelope})
return results
```

Keep each symbol's typed envelope. Do not collapse the results into a successful-only list: absent, ambiguous, `PARTIAL`, `TRUNCATED`, and failed outcomes remain independent and do not disappear because another symbol succeeded. Do not replace this semantic fan-out with combined textual search. Text may locate declarations or tests, but it cannot establish `CALLS`.

When the exact callee URI and position are already known and only callers are needed, use `incoming` for that symbol instead of resolving its document through an exact-symbol operation. Keep that `incoming` result independent under the same per-symbol accounting.

## Transient structural design orientation

An LLM may start from one symbol and quickly obtain live design or refactoring context from a READY managed LSP session. Graph Provenance and source capture are not prerequisites for this bounded orientation. Route through the available `context`, incoming, or slice operation appropriate to the question, using exact session and generation identity.

Treat `slice` and `incoming` as bounded orientation over server-reported callers and callees. They do not compute centrality, establish architectural boundaries, or by themselves justify refactor-safety conclusions. Their claim ceiling remains low: it is not retained, replayable, or source-grounded evidence, and it does not establish runtime behavior, complete source coverage, or feature identity. ADR 0006's transient structural-context operation is implemented as CLI `context --machine` and unified MCP `lsp_trace_v2_structural_context`; use current binary help or capabilities for its exact syntax.

Escalate optionally to Capture/V5 when the work needs durable review, retained hydration, publication, replay, or stronger source-bearing evidence. That escalation is never the default prerequisite for live structural orientation.

### Source-bearing structural context

Omit `projection` for relationship-only context. When the declaration URI is already known, prefer its exact zero-based position; symbol mode expects the exact server-reported symbol name, not a package-qualified identifier. For implementation ownership, source-flow, or seam analysis, request bounded layered source explicitly. A practical starting request is:

```json
{
  "session_id": "READY_SESSION_OR_ALIAS",
  "generation": 1,
  "uri": "file:///absolute/path/to/file.go",
  "line": 0,
  "character": 0,
  "down_depth": 1,
  "up_depth": 1,
  "max_nodes": 80,
  "timeout_ms": 60000,
  "request_timeout_ms": 30000,
  "max_messages": 1024,
  "max_bytes": 8388608,
  "analysis": {"kind": "NEIGHBORHOOD"},
  "projection": {
    "mode": "PROJECTED",
    "body": "INCLUDE",
    "include_relation_occurrences": true,
    "include_ancillary": false,
    "display_range_policy": "FULL_DEFINITION",
    "limits": {
      "max_objects": 80,
      "max_ranges": 80,
      "max_source_bytes": 2097152,
      "max_work": 10000,
      "max_response_bytes": 8388608,
      "max_additional_documents": 20,
      "max_document_requests": 21,
      "max_document_bytes": 1048576,
      "max_total_document_bytes": 8388608,
      "max_document_messages": 32,
      "max_document_acquisition_work": 21,
      "max_display_resolution_work": 21
    },
    "privacy_policy_id": "public"
  }
}
```

Use `TARGET` instead of `PROJECTED` when only the selected definition is needed. `PROJECTED` returns exact endpoint and relation evidence ranges together with complete containing definitions resolved through server-reported `textDocument/documentSymbol`. Repeated logical units may share deduplicated emitted spans. These bodies remain request-ephemeral, authority-zero, and non-retainable. Keep the initial depth and object bounds narrow; an oversized projected response is not a reason to increase limits silently.

## Traversal selection

Use `incoming` when exact supplied positions are callees and only upward caller expansion is needed. Use `slice` when bounded outgoing discovery must choose the nodes from which incoming traversal begins.

For slice, exact-depth frontier nodes and genuine successful empty outgoing leaves form the sorted deduplicated upward-start union. Failed, null, timed-out, canceled, or budget-truncated outgoing requests are not leaves. Depths count edges; zero disables that direction. Set explicit depth, node, request-timeout, and global-timeout bounds.

Legacy `slice --from-file PATH` recursively enumerates server-reported document symbols and attempts preparation. Implemented Production V5 slice also accepts repeatable file/directory `--from-file`, `--include`, and `--exclude` scopes using the documented workspace-relative gitignore-style subset. Exclusions win. Every document symbol is only a candidate; only successful non-empty preparation makes it callable. Exact operational denominators do not prove endpoint, source, callable, or feature completeness.

Current `trace` is the implemented exact-target facade. Use it for one exact symbol or repeated exact positions. Exact symbol mode fails on zero or multiple matches; use `--at` to resolve ambiguity. Repeated positions remain separate target occurrences even when native nodes deduplicate. Topmost sibling enrichment is opt-in.

Current CLI `census` is AVAILABLE for accountable source-symbol enumeration and deterministic batched acquisition. Use only syntax printed by `lsp-trace census --help`: it requires `--workspace`, either `--server` or `--profile` (with optional `--config`), and a private `--publication-root`; it accepts repeatable `--source`, `--include`, `--exclude`, and `--server-arg`, bounded traversal/resource options, and `--machine`. The default source is `.`, depths default down/up `1`/`0`, and exclusions win. One invocation uses one initialized session and exact generation, closes file/symbol accounting before acquisition, partitions deterministic non-empty batches at 63 targets, and publishes one private atomic capture-set bundle selector. Precommit failure publishes nothing. Machine output is closed JSON/JSONL. Committed degradation is `SUCCEEDED_DEGRADED` and non-retryable.

Census authority remains zero and source-graph completeness remains `UNKNOWN`. The capture set has no native aggregate custody, infers no cross-capture `CALLS`, and is not directly Leiden-admissible. The `context` interface accepted in ADR 0006 is AVAILABLE for bounded transient live analysis; it remains non-retained, non-replayable, non-publishable, authority-zero, and `source_graph_complete=UNKNOWN`. The earlier ADR 0004 name `discover` is superseded by the accepted `census` name.

## Status and retention

- `0`: requested bounded traversal completed relative to server and bounds; warnings may remain.
- `2`: structured incomplete evidence may exist; preserve graph/selector, stderr, diagnostics, failed seeds, and boundaries.
- `1`: invocation, unrecoverable server, inspection/verification, or publication failure; complete marshaled graph JSON may remain on stdout after publication failure.
- `130`: interrupted `incoming`; retain emitted output as interrupted evidence. Slice cancellation follows status `2`.

A selector's existence never overrides status. Preserve invocation scope/revision assertions, server/workspace configuration, seeds and reasons, bounds, artifact bytes or selector, stdout, stderr, exit status, verification, inspections, and unresolved authority limits. `--provenance-source-revision` is caller-asserted, not authenticated.

Reconcile every requested seed:

```text
requested_seed_count = successful_seed_count + failed_seed_count
successful_seed_count = successful_seed_with_membership_count + successful_seed_without_membership_count
```

Global counts must equal array lengths; per-seed reference occurrence counts may exceed deduplicated global records. Diagnostic indexes are correlations, not custody or causation. Preserve `UNKNOWN`, successful-empty, failed, bounded-zero, and untouched ambiguity as distinct states.
