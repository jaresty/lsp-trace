# V6 cardinality/checkpoint-diagnostic full-Pi restart qualification

Status is not valid until `PREFLIGHT.json` says `STAGED_VALID`.

## User action

After confirming `STAGED_VALID`, fully restart Pi. Do not reconnect MCP and do not restart a managed session from the current Pi process; it retains stale in-memory configuration.

## Exact post-restart checks

Before the one-shot request:

1. Inspect `/Users/schwa/dev/lsp-trace/.mcp.json` and require this exact argv:

   `/Users/schwa/.local/bin/lsp-trace-mcp --bootstrap-config /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json --publication-root /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/base-publication --tool-profile compact`

2. Require exactly one `--publication-root` argument, no MCP `env` bindings, and no diagnostic sink flags or environment variables; diagnostics remain default-off.
3. Require live CLI and MCP SHA-256 values to match `PREFLIGHT.json`.
4. Require base, continuation, trace, and ledger roots to be pairwise distinct, brand-new, empty, and `drwx------` before the request.
5. Require bootstrap continuation root `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/continuation-publication`, `max_object_bytes=67108864`, and sole capability `STOP_AFTER_DESCRIBE_REQUESTS`.
6. Require production capture defaults exposed by the restarted server to equal: `max_artifact_bytes=67108864`, `max_parent_bytes=67108864`, `max_bindings=1000`, `max_receipts=1000`, `max_source_bytes=1048576`, `max_total_source_bytes=8388608`, and `max_work=10000`. Stop if any default differs.
7. Require current V6 schema `lsp-trace.graph-v5-source-snapshot.v6.schema.json` to have SHA-256 `ba85dcb1546478eb7a6a79b146894a0ce52db9531fab7318312c44341bef85e4` in both registered copies.
8. Call `lsp_session_v1_list` once. Require the exact `/Users/schwa/dev/lsp-trace` project session to be `READY`, workers to equal `0`, and the live server image digest to match `PREFLIGHT.json`. Substitute only its exact session ID and generation for `S` and `G` below.
9. Run:

   `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/validate-staged-request.py /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/REQUEST.json --handoff /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/HANDOFF.md`

   Stop unless it prints `REQUEST_FINGERPRINT_VALID sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`. Reject `sources:["."]`, any occurrence of source `"."`, omitted/defaulted semantic fields, or any request/HANDOFF mismatch.

Stop on any mismatch. Do not reconnect, retry, start a model, start a worker, repair in-session, or issue another request.

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

`/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v6-cardinality-20260921T144639Z/rollback.sh`

Then fully restart Pi again. Rollback restores the immediately preceding configuration and binaries; it does not delete publication or evidence roots.
