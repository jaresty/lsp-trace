# ADR 0007: Append-only transient structural delta

## Status

Accepted and implemented as canonical operation 37.

## Decision

`lsp_trace_v1_structural_delta` compares exactly two strict inline `lsp-trace.transient-structural-result.v2` values. The CLI spelling is `lsp-trace context-delta --before FILE --after FILE --machine`; both inputs must be bounded local regular files. MCP direct calls, execute-gateway calls, and the CLI use the same transport-neutral comparison kernel.

Symbols match only by `(workspace-relative path, kind, name)`. Opaque IDs and declaration ranges are excluded. Duplicate canonical keys are `AMBIGUOUS_SYMBOL_KEY`; canonical target or position-encoding mismatch is rejected. Calls are aggregated by canonical caller, callee, and call-site relative path. Shifted individual call-site ranges are not paired.

The result reports symbol and call changes, multiplicity changes, Robert Martin coupling/instability transitions, SCC cycle and articulation membership transitions, PageRank, HITS hub/authority, and weak-bridge membership. `affected_callers` is only the unique set of observed caller keys incident to changed calls or calls to added/removed callees.

Every result states authority `0`, `source_graph_complete: UNKNOWN`, comparison scope `TWO_BOUNDED_LOCAL_RESULTS`, `acquisition_scope_comparable: UNKNOWN`, node-universe equality, analytics-policy equality, and PageRank/HITS scope sensitivity. It never claims repository equivalence, completeness, architecture, safety, ownership, community, or Leiden conclusions.

## Compatibility

Operation 37 is append-only. Operation 35 V1 and operation 36 V2 semantics are unchanged. The compact profile still advertises exactly 10 tools; operation 37 remains hidden but dispatchable there.
