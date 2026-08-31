# Graph semantics

An edge means the configured language server reported a static incoming Call Hierarchy relation. It does not prove runtime execution, reachability on a deployed configuration, frequency, or whole-program completeness. Discovery proceeds from callee to callers, while each edge is serialized in execution orientation: caller to callee.

Completeness is bounded by server capability and responses plus visible depth, node, timeout, cancellation, and error frontiers. Unsupported Call Hierarchy never authorizes a generic-reference fallback. Cycles describe graph structure; they are not roots or proof of runtime recursion.

The qualification fixtures include `StaticButNotExecuted`, whose call is behind a branch normal fixture execution does not take. A server may still report that edge, demonstrating the distinction between static evidence and observed execution.

## Completeness and bounds

`summary.complete` is true only when every discovered node was expanded successfully or reached a natural terminal. It is false when capability, cancellation, timeout, server error, invalid response, or a safety bound prevents complete expansion. `summary.truncated` is narrower: it reports depth or node-count truncation, not every cause of incompleteness. Consumers must inspect `terminals`, `frontier`, and `diagnostics` rather than infer a reason from either Boolean alone.

A terminal records a node that was expanded or intentionally stopped; a frontier records a discovered node that remains unexpanded. Reasons are stable machine-readable values defined by the v1 schema policy. `MAX_DEPTH` and `MAX_NODES` identify user bounds. Deadline errors currently surface as `REQUEST_TIMEOUT`; the global `--timeout` bounds the whole command, while `--request-timeout` bounds each LSP request. `GLOBAL_TIMEOUT` remains reserved v1 reason vocabulary and is not currently emitted as a distinct reason. Cancellation surfaces as `CANCELLED`, and an interrupt causes exit status `130` after JSON emission when possible.

## Traversal completeness and provenance

Traversal completeness is server-relative, not source completeness. For example, source inspection may show `CrossModuleCallers.aliased_cross_file/1` calling `Calls.leaf/1`, yet a server may return zero callers for `leaf/1`. In that result, `NO_INCOMING_CALLS` means only that the configured server reported an empty incoming-call list for that prepared item; it does not erase the known source caller or prove no caller exists.

Terminal provenance is carried by the terminal's `node_id`, `reason`, and optional `message`, interpreted alongside the invocation's server command, capabilities, target, and retained server version. The v1 graph does not embed a source-inspection oracle or server version, so those belong in qualification evidence rather than new graph fields. A complete traversal can therefore be incomplete relative to source truth, and a source-known/server-zero-caller discrepancy blocks the relevant interoperability qualification without changing `lsp-trace.graph.v1` semantics.
