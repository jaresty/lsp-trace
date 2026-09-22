# 64MiB continuation-capture full-Pi-restart qualification

Status is not valid until `PREFLIGHT.json` says `STAGED_VALID`.

## User action

After confirming `STAGED_VALID`, **fully restart Pi**. Do not reconnect MCP and do not restart a managed session from the current Pi process; it retains stale in-memory configuration.

## Post-restart checks

Before the one-shot request:

1. Inspect `/Users/schwa/dev/lsp-trace/.mcp.json` and require this exact argv:

   `/Users/schwa/.local/bin/lsp-trace-mcp --bootstrap-config /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json --publication-root /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/base-publication --tool-profile compact`

2. Require exactly one `--publication-root` argument and no MCP `env` bindings; diagnostics remain default-off.
3. Require the live CLI and MCP SHA-256 values to match `PREFLIGHT.json`.
4. Require the base, continuation, trace, and ledger roots to be distinct, brand-new, empty, and `drwx------` before the request.
5. Require bootstrap continuation root `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/continuation-publication`, `max_object_bytes=67108864`, and sole capability `STOP_AFTER_DESCRIBE_REQUESTS`.
6. Require the capture defaults exposed by the restarted server to assert a 64MiB capture artifact limit with parent custody, `max_source_bytes=1048576`, and `max_total_source_bytes=8388608`. Stop if any default differs.
7. Call `lsp_session_v1_list` once. Require the exact `/Users/schwa/dev/lsp-trace` project session to be `READY`, workers to equal `0`, and the live server image digest to match `PREFLIGHT.json`. Substitute only its exact session ID and generation for `S` and `G` below.
8. Run the persistent validator:

   `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/validate-staged-request.py /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/REQUEST.json --handoff /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/HANDOFF.md`

   Stop unless it prints `REQUEST_FINGERPRINT_VALID sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`. The validator rejects `sources:["."]`, any occurrence of `"."`, and omitted/defaulted semantic fields.

Stop on any mismatch. Do not reconnect, retry, start a model, start a worker, or repair in-session.

## Exact one-shot request

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

Invoke `lsp_trace_v1_census` exactly once only after every post-restart check passes. Retain the response and stop. No retry, replay, reconnect, continuation resume, model invocation, worker start, packet inspection, commit, stash, reset, revert, or clean.

## Rollback

If any post-restart check fails, run:

`/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-64mib-20260921T134932Z/rollback.sh`

Then fully restart Pi again. Rollback restores the immediately preceding configuration and binaries; it does not delete any publication or evidence roots.
