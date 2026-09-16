# Transport and routing

## Choose CLI or MCP

Use CLI for direct local invocations, files/stdin, schema retrieval, rendering, and explicit trusted server launch. Use MCP when a host already provisions a trusted managed session or when registry-described offline tools fit the task.

For MCP, call `lsp_trace_v1_capabilities` before relying on operation names, schema IDs, publication support, limits, or hidden-operation routes. The embedded manifest and schemas are authoritative. Canonical operation `lsp_trace_v1_trace` accepts one exact symbol or one zero-based `line`/`character` position against a host-managed READY session; default, advanced, and full advertise it. A compact tool-advertisement profile remains exactly ten tools and hides trace from `tools/list`, but does not change dispatch, behavior, authority, or managed sessions, so trace remains callable directly from a cached registration and through canonical execute. It is unrelated to CLI `--profile NAME`.

Choose the transport explicitly:

- **Direct MCP operation**: invoke the operation's MCP tool and pass that operation's arguments directly.
- **MCP gateway `lsp_trace_v1_execute`**: operation is nested beneath `request`. Copy-ready example:

  ```json
  {"request":{"operation":"lsp_trace_v2_structural_context","arguments":{...}}}
  ```

- **CLI `lsp-trace execute`**: accepts production execution requests and is not an MCP dispatcher. Do not pass an MCP operation envelope to it.

For the MCP gateway, operation fields belong in `request.arguments`. MCP wire and server operation names are canonical unqualified names; host adapters may display namespace-qualified names. `request.operation` always uses the canonical unqualified operation name.

Client metadata is a separate boundary from server behavior. If capabilities or the installed revision advertise an operation but the client does not list it or its tool schema rejects it before dispatch, reconnect or refresh MCP metadata before diagnosing the language server. The MCP gateway is a dispatch interface and does not bypass stale client-side schema validation. Distinguish client or adapter schema validation from server dispatch, structural preflight, and traversal typed failures. A gateway retry is not a remedy for stale client metadata.

For hidden artifact producers, a delegated graph `output_selector` belongs in `request.arguments`; an outer gateway selector publishes the execution artifact instead.

## Managed lifecycle

The host, not the caller, supplies bootstrap execution authority. Stdio serving begins only after configured sessions reach correlated READY. Call session list to obtain the exact session ID and generation, then bind traversal to both. Callers cannot select executable commands through traversal inputs.

Lifecycle result envelopes, not transport success alone, determine categorical success. Cancellation stops the caller observation; an already accepted lifecycle intent may continue. Restart and stop are generation-sensitive. Never infer readiness or authority from stale IDs, logs, or private diagnostics.

The recommended Pi integration uses the standard `pi-mcp-adapter` and machine-local `.mcp.json`; do not add a repository-specific bridge. Preserve host command, bootstrap, publication root, working directory, lifecycle, timeout, direct-tool, and search configuration.

## Coordinates and limits

CLI target positions are one-based; MCP positions and retained LSP ranges are zero-based. Use the managed generation's negotiated encoding. MCP traversal requires exact READY generation and retained call-hierarchy/encoding evidence.

Inline artifacts are bounded. Supply a caller-chosen output destination before broad acquisition. `max_messages` and `max_bytes` apply independently to each individual LSP wire request, not to an aggregate slice. Pagination and publication limits come from capabilities, not copied assumptions.

## Evidence envelopes

Every completed MCP call returns one versioned envelope in `structuredContent`; outer content is non-authoritative and may be empty. Distinguish domain errors, publication receipts, and inline artifacts. Compact responses do not summarize away or upgrade retained evidence.

Tool descriptions and capabilities are discovery metadata, not session discovery, source acquisition, or permission. Use the current runtime registry for canonical names rather than relying on a frozen list in this reference.
