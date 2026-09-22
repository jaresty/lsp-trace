# Coherent full-Pi qualification: CUE source-only TARGET + final V2 census admission

Status is valid only when `PREFLIGHT.json` says `STAGED_VALID`.

## User action

Fully restart Pi. Do not reconnect MCP or restart a managed session in this process.

## Post-restart preflight

1. `.mcp.json` contains one base `--publication-root`: `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-coherent-20260921T172613Z/base-publication`.
2. Installed CLI/MCP and configuration hashes match `PREFLIGHT.json`.
3. Base, continuation, trace, and ledger roots are pairwise distinct, empty, and mode `0700`.
4. `.lsp-trace-mcp-bootstrap.json` is mode `0600`; continuation root is `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-coherent-20260921T172613Z/continuation-publication`, max object bytes `67108864`, sole capability `STOP_AFTER_DESCRIBE_REQUESTS`.
5. Capture defaults remain artifact/parent `67108864`, bindings/receipts `1000`, source `1048576`, total source `8388608`, work `10000`.
6. Regex defaults remain document `61440`, pattern `4096`, matches `100`, work `65536`; hard maxima remain document `16777216`, pattern `4096`, matches `1000`, work `536870912`.
7. Call `lsp_session_v1_list` once. Require exact project and `nais-discovery-cue` sessions `READY`, workers `0`, and the expected MCP hash.
8. Validate `REQUEST.json`; require semantic fingerprint `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`.

## Exact one-shot census

```json
{
  "session_id": "S",
  "generation": "G",
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

Substitute only the returned project session ID and generation into `REQUEST.json`, then invoke census exactly once. Retain the public response before inspecting private artifacts. Accept only progression beyond `PROGRAM_C_COMPUTED`, or exactly one typed `FAILED_CAPTURE` checkpoint. Stop without retry on any other result.

Do not run a model or worker. Do not commit, stash, reset, revert, or clean.

## After the census result is retained

The CUE source-only behavior may be qualified separately with position, symbol+URI, and regex zero-depth TARGET requests. These are not part of the one-shot census qualification and must not be used as fallback or retry for it.

## Rollback

Run `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-coherent-20260921T172613Z/rollback.sh`, then fully restart Pi.
