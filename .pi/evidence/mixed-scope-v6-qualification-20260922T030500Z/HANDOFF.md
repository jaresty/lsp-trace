# Mixed-scope V6 qualification restart handoff

## Restart boundary

A full Pi process restart is required. Reconnect, MCP reconnect, managed-session restart, or reuse of the process that staged these files is not equivalent.

Project configuration now points at this qualification root.

## Pinned files

- CLI: `artifacts/lsp-trace`
  - SHA-256 `6d832fdf1b6268536034368bb25bcceb8a505099795d85a9979a807e4f55c93c`
- MCP: `artifacts/lsp-trace-mcp`
  - SHA-256 `ff1c89760a778c27124cde862e6c6018de6c7725f1f6b7a618bff30eb3d358b0`
- Bootstrap config: `BOOTSTRAP_CONFIG.json`
  - SHA-256 `4cb301870d81d61e262d34dbe6af7144a5fca1078cd4113eb5d01d97ae7b4c90`
- MCP config: `MCP_CONFIG.json`
  - SHA-256 `c497f5c31b66b866ab0bfd4569758d237d10073762b7efd9b1b178524aec655c`

## Fresh custody roots

All directories were created mode `0700` and pairwise distinct:

- `base-publication`
- `continuation-publication`
- `trace-root`
- `ledger-root`
- `managed-preparation-private`

`managed-preparation-private/diagnostic.ndjson` exists at mode `0600`; private diagnostics are explicitly enabled for this qualification.

## Exact one-shot request

```json
{
  "session_id": "sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283",
  "generation": 1,
  "sources": [
    "internal/censuscontinuation/pipeline.go",
    "internal/censuscontinuation/capture.go"
  ],
  "down_depth": 1,
  "up_depth": 0,
  "max_nodes": 10000,
  "batch_targets": 16,
  "timeout_ms": 60000,
  "request_timeout_ms": 30000,
  "continuation": {
    "kind": "ADR_0007_FEATURE_CATALOG",
    "stop_after": "DESCRIBE_REQUESTS"
  }
}
```

## Required post-restart sequence

1. Call `lsp_session_v1_list` exactly once and retain the complete response as `PRE_SESSION_LIST.json`.
2. Require `project` and `nais-discovery-cue` to be `READY`, workers zero, and each retained server instance to report MCP executable SHA-256 `ff1c89760a778c27124cde862e6c6018de6c7725f1f6b7a618bff30eb3d358b0`.
3. Substitute only the exact READY `project` session ID and generation into `REQUEST.json`.
4. Run `preflight.py` with the pinned files, fresh roots, retained session-list response, and explicit diagnostics enablement.
5. Send the exact stop-after-Describe census request once through the delegated gateway. Do not retry unchanged.
6. Retain the complete public response, one post-result session-list response, and execution accounting separately.
7. Run `check-result.py`; require workers `0` and model invocations `0`.
8. Accept only strict `PAUSED` at `DESCRIBE_REQUESTS` or a typed immutable failure. Do not run a worker or model.
9. If `PAUSED`, inspect V6 snapshots, TARGET packets, and Describe requests using `V6-PACKET-CHECKLIST.md`.

Semantic request fingerprint remains `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9` after substituting session custody only.
