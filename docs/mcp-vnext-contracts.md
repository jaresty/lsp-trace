# MCP vNext contract draft

Status: unqualified design draft governed by [ADR 0012](adr/0012-context-bounded-mcp-vnext.md). No implementation or public enablement is authorized.

## Design goals

- Preserve access to qualified live and retained operations.
- Keep the default advertised definition set within a frozen context budget.
- Retrieve exact operation schemas only when needed.
- Carry large evidence by immutable reference and bounded page.
- Share one operation kernel across compact tools, generic execution, CLI adapters, and any retained compatibility adapter.

## Conceptual tools

### `lsp_capabilities`

Input:

```json
{"cursor":null,"limit":50}
```

Result entries contain operation name, versions, mode, summary, schema IDs, availability, and required gates. They do not contain full schemas.

### `lsp_schema_get`

Input:

```json
{"schema_id":"https://example/schema","page":1,"snapshot":null}
```

Page 1 returns a stable snapshot identity, total pages, exact schema byte length and digest, and the first bounded segment. Later pages require that snapshot. Complete reconstruction is required before validation claims.

### `lsp_execute`

Input:

```json
{
  "operation":"structural_context",
  "contract_version":"v3",
  "input":{},
  "input_ref":null,
  "request_id":"request-identity",
  "deadline_ms":30000
}
```

Exactly one of `input` or `input_ref` is present. The executor resolves and validates the exact operation schema before effects. Results are inline only below a qualified bound; otherwise they return an artifact descriptor.

### `lsp_artifact_read`

Input names an immutable artifact, projection, page size, and continuation snapshot. Reads are bounded, privacy-partitioned, and content-addressed. There is no ambient path or mutable-latest mode.

### `lsp_session`

Conceptual lifecycle envelope for list, status, start/derive, stop, and restart. Exact action shape remains unresolved. Lifecycle operations preserve C18 ownership, collision, refusal-before-effect, stale-generation, and fresh-owner restart behavior.

## Workflow shortcuts

Candidate shortcuts are `lsp_inspect`, `lsp_trace`, and `lsp_discover`. A shortcut is admitted only if user testing shows material discoverability benefit and its complete advertised definition fits the profile budget. It delegates to the same execution kernel and cannot change defaults, limits, failures, or authority.

## Live inspection example

```json
{
  "operation":"structural_context",
  "contract_version":"v3",
  "input":{
    "session_id":"sk1:...",
    "generation":4,
    "target":{
      "uri":"file:///repo/foo.go",
      "position":{"line":42,"character":7}
    },
    "projection":"bounded_source"
  },
  "request_id":"inspect-42",
  "deadline_ms":30000
}
```

The result preserves session generation, position encoding, graph/source custody, limits, omissions, and typed outcomes. Parallel invocation carries no semantic order.

## Error model

Transport failures are distinct from operation failures. Closed transport codes include malformed envelope, unknown operation, unsupported contract version, schema unavailable, input-reference mismatch, deadline before admission, context/resource limit, and artifact-page snapshot mismatch. Operation failures retain their existing typed domain codes and authority.

Errors never inline unbounded schemas, artifacts, source, stderr, paths, prompts, or transcripts.

## Context accounting

Every profile build records exact serialized bytes for:

- tool names and descriptions;
- advertised input schemas;
- capability page exemplars;
- schema pages;
- inline examples if advertised;
- maximum bounded diagnostics.

The default profile has one aggregate ceiling and per-tool ceilings. Exact limits require retained measurements before acceptance.

## Compatibility

The compact transport may be incompatible with current direct MCP tools. Historical artifacts, canonical readers, selectors, replay, and internal operations remain independently versioned. A compatibility adapter must prove result and failure equivalence; it cannot become a second semantic implementation.
