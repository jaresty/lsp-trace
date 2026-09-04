# Production Acceptance Activation claim

Status: CLAIMED
Baseline: 4ce6e6df0ce97cdd0fd641fa046e68879cf3daae
Frame: Production Acceptance Activation

## Goal

Activate the committed real-process production acceptance guards using test-only changes: emit strict valid provider-observation envelopes with immutable original-document custody and exact anchors; exercise malformed, oversized, timeout, cancellation, and no-item modes; replace unconditional omission failure with marker-backed zero-start plus exact historical graph-v3 byte comparison; and update test bootstrap JSON to the host-owned provider declaration shape.

## Exact ownership

- `cmd/lsp-trace-mcp/testdata/fake-relation-provider/main.go`
- `cmd/lsp-trace-mcp/real_process_conformance_test.go`
- test bootstrap JSON consumed only by the conformance harness
- this claim file

No production implementation file or symbol is owned or may be edited.

## Acceptance boundary

1. The fake provider success mode emits a strict valid provider-observation envelope preserving immutable original document custody and exact anchors.
2. The fixture exposes malformed, oversized, timeout, cancellation, and no-item behavior.
3. `ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER` and `ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER` are precise joined-implementation acceptance guards and may remain compiling red on baseline.
4. `ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3` accepts marker-backed zero provider starts and requires byte-for-byte historical graph-v3 equality.
5. Test bootstrap JSON uses the intended host-owned provider declaration shape.
6. Direct transport guards remain green.
7. MCP publishes exactly 13 tools and Glint remains BLOCKED.
8. The final commit contains test/claim artifacts only and leaves the worktree clean.
