# Live tracing and census

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

## Transient structural design orientation

An LLM may start from one symbol and quickly obtain live design or refactoring context from a READY managed LSP session. Graph Provenance and source capture are not prerequisites for this bounded orientation. Route through the available live MCP incoming and slice operations, using exact session and generation identity; future orchestration may compose those existing operations but must not invent a new command.

Treat `slice` and `incoming` as bounded orientation over server-reported callers and callees. They do not compute centrality, establish architectural boundaries, or by themselves justify refactor-safety conclusions. Their claim ceiling remains low: it is not retained, replayable, or source-grounded evidence, and it does not establish runtime behavior, complete source coverage, or feature identity. ADR 0006 accepts a separate transient structural-context operation that may compute bounded structural measures; that operation and its syntax are proposed until current binary help or capabilities document implementation.

Escalate optionally to Capture/V5 when the work needs durable review, retained hydration, publication, replay, or stronger source-bearing evidence. That escalation is never the default prerequisite for live structural orientation.

## Traversal selection

Use `incoming` when exact supplied positions are callees and only upward caller expansion is needed. Use `slice` when bounded outgoing discovery must choose the nodes from which incoming traversal begins.

For slice, exact-depth frontier nodes and genuine successful empty outgoing leaves form the sorted deduplicated upward-start union. Failed, null, timed-out, canceled, or budget-truncated outgoing requests are not leaves. Depths count edges; zero disables that direction. Set explicit depth, node, request-timeout, and global-timeout bounds.

Legacy `slice --from-file PATH` recursively enumerates server-reported document symbols and attempts preparation. Implemented Production V5 slice also accepts repeatable file/directory `--from-file`, `--include`, and `--exclude` scopes using the documented workspace-relative gitignore-style subset. Exclusions win. Every document symbol is only a candidate; only successful non-empty preparation makes it callable. Exact operational denominators do not prove endpoint, source, callable, or feature completeness.

Current `trace` is the implemented exact-target facade. Use it for one exact symbol or repeated exact positions. Exact symbol mode fails on zero or multiple matches; use `--at` to resolve ambiguity. Repeated positions remain separate target occurrences even when native nodes deduplicate. Topmost sibling enrichment is opt-in.

Current CLI `census` is AVAILABLE for accountable source-symbol enumeration and deterministic batched acquisition. Use only syntax printed by `lsp-trace census --help`: it requires `--workspace`, either `--server` or `--profile` (with optional `--config`), and a private `--publication-root`; it accepts repeatable `--source`, `--include`, `--exclude`, and `--server-arg`, bounded traversal/resource options, and `--machine`. The default source is `.`, depths default down/up `1`/`0`, and exclusions win. One invocation uses one initialized session and exact generation, closes file/symbol accounting before acquisition, partitions deterministic non-empty batches at 63 targets, and publishes one private atomic capture-set bundle selector. Precommit failure publishes nothing. Machine output is closed JSON/JSONL. Committed degradation is `SUCCEEDED_DEGRADED` and non-retryable.

Census authority remains zero and source-graph completeness remains `UNKNOWN`. The capture set has no native aggregate custody, infers no cross-capture `CALLS`, and is not directly Leiden-admissible. The `context` interface accepted in ADR 0006 remains `FUTURE/PROPOSED`; describe desired context behavior without presenting proposed syntax as live. The earlier ADR 0004 name `discover` is superseded by the accepted `census` name.

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
