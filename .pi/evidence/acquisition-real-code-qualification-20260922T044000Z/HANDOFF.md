# Real-operation-code acquisition diagnostic qualification

## Mandatory boundary

Perform a full Pi process restart. MCP reconnect or managed-session restart is not equivalent.

## Pins

- CLI SHA-256: `6d832fdf1b6268536034368bb25bcceb8a505099795d85a9979a807e4f55c93c`
- MCP SHA-256: `fc158a6edab4552597c2636ee61bcc52588dba73a1501d4ee7b116da766c67dc`
- Bootstrap SHA-256: `aca7ff4ba449480c1a0978768782f130cdca2adf2a30e6fff3d822555e6b0cf4`
- MCP config SHA-256: `5f5f550418b61f7d28d80af37ff3ecb7da421d4d1d769c3210484c00a88287ed`
- Active .mcp.json SHA-256: `5f5f550418b61f7d28d80af37ff3ecb7da421d4d1d769c3210484c00a88287ed`
- Request fingerprint: `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`

## Exact one-shot request

```json
{
  "session_id": "sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283",
  "generation": 1,
  "sources": ["internal/censuscontinuation/pipeline.go", "internal/censuscontinuation/capture.go"],
  "down_depth": 1,
  "up_depth": 0,
  "max_nodes": 10000,
  "batch_targets": 16,
  "timeout_ms": 60000,
  "request_timeout_ms": 30000,
  "continuation": {"kind": "ADR_0007_FEATURE_CATALOG", "stop_after": "DESCRIBE_REQUESTS"}
}
```

## Sequence

1. One full session list; require READY project/CUE, workers zero, MCP digest above.
2. Substitute only project session ID/generation into request and handoff request block.
3. Run 192-schema preflight and exact fingerprint validation.
4. Send exactly one changed diagnostic census. Never retry.
5. Retain public response, private ledgers, one post-result session list, and accounting.
6. Require workers/model invocations zero.
