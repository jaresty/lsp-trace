# Acquisition diagnostic qualification restart handoff

## Mandatory boundary

Perform a full Pi process restart. MCP reconnect, server reconnect, managed-session restart, or reuse of the staging process is not equivalent.

`.mcp.json` now points to this qualification's MCP binary and includes the independent `--acquisition-diagnostic-path`.

## Pins

- CLI SHA-256: `6d832fdf1b6268536034368bb25bcceb8a505099795d85a9979a807e4f55c93c`
- MCP SHA-256: `7d9bb48a6a2e17b04817664765a21695e4183dc97db53c026a986eec864dd78f`
- Bootstrap config SHA-256: `c19839c73c391678cd6a42575473d057437a08ba4983b8933cf89fe66dbf6651`
- MCP config SHA-256: `c4852f35b5ff82fcc64bd16e33ab8ff22df6ad2640927c9d8fa144a6a3a7cb0d`
- Active `.mcp.json` SHA-256: `09de506588c753251839282140873da363d97dba8876a562465975ff3bd34157`
- Expected semantic request fingerprint: `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`

## Fresh custody

All roots are pairwise distinct and mode `0700`. Both private ledgers exist at mode `0600`:

- `base-publication/`
- `continuation-publication/`
- `managed-preparation-private/diagnostic.ndjson`
- `acquisition-private/acquisition.ndjson`

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

## Exact post-restart sequence

1. Call `lsp_session_v1_list` exactly once; retain the complete result as `PRE_SESSION_LIST.json`.
2. Require project and CUE sessions READY, workers zero, and loaded MCP SHA-256 `7d9bb48a6a2e17b04817664765a21695e4183dc97db53c026a986eec864dd78f`.
3. Substitute only the exact READY project session ID and generation into `REQUEST.template.json`, saving `REQUEST.json`.
4. Run `preflight.py`; require all pins, roots, schemas, diagnostics, and fingerprint.
5. Send the request exactly once through `lsp_trace_v1_execute → lsp_trace_v1_census`. This is a changed diagnostic run, not an unchanged retry.
6. Do not retry regardless of `retry` metadata.
7. Retain complete public response, one post-result session list, execution accounting, and both private ledgers.
8. Require workers zero and model invocations zero.
9. Use the acquisition private record to identify the actual subtype if public result remains `ACQUISITION_FAILED`; otherwise accept only strict `PAUSED` at `DESCRIBE_REQUESTS` or another typed immutable terminal.
