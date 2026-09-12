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

Treat the transient live graph as suitable for bounded questions about callers and callees, likely impact, centrality, architectural boundaries, and refactor risk. Its claim ceiling remains low: it is not retained, replayable, or source-grounded evidence, and it does not establish runtime behavior, complete source coverage, or feature identity.

Escalate optionally to Capture/V5 when the work needs durable review, retained hydration, publication, replay, or stronger source-bearing evidence. That escalation is never the default prerequisite for live structural orientation.

## Traversal selection

Use `incoming` when exact supplied positions are callees and only upward caller expansion is needed. Use `slice` when bounded outgoing discovery must choose the nodes from which incoming traversal begins.

For slice, exact-depth frontier nodes and genuine successful empty outgoing leaves form the sorted deduplicated upward-start union. Failed, null, timed-out, canceled, or budget-truncated outgoing requests are not leaves. Depths count edges; zero disables that direction. Set explicit depth, node, request-timeout, and global-timeout bounds.

Legacy `slice --from-file PATH` recursively enumerates server-reported document symbols and attempts preparation. Implemented Production V5 slice also accepts repeatable file/directory `--from-file`, `--include`, and `--exclude` scopes using the documented workspace-relative gitignore-style subset. Exclusions win. Every document symbol is only a candidate; only successful non-empty preparation makes it callable. Exact operational denominators do not prove endpoint, source, callable, or feature completeness.

Current `trace` is an implemented exact-target facade. Exact symbol mode fails on zero or multiple matches; use `--at` to resolve ambiguity. Repeated positions remain separate target occurrences even when native nodes deduplicate. Topmost sibling enrichment is opt-in.

Future `discover`, richer census, or context interfaces in ADRs remain proposals unless current binary help/capabilities document implementation. Describe desired behavior without inventing syntax.

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
