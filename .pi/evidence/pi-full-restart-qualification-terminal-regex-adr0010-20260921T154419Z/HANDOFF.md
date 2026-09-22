# Full-Pi restart qualification: capture terminal-gap + regex locator phase 1 + ADR0010 contracts

Status is valid only when `PREFLIGHT.json` says `STAGED_VALID`.

## User action
Fully restart Pi. Do not reconnect MCP or restart a managed session in this process.

## Post-restart preflight (stop on any mismatch)
1. `.mcp.json` must contain exactly one base `--publication-root`: `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-terminal-regex-adr0010-20260921T154419Z/base-publication`; no MCP env or diagnostic sinks.
2. CLI/MCP hashes must match PREFLIGHT. Base, continuation, trace, ledger must be pairwise distinct, never reused, empty, mode 0700.
3. Bootstrap continuation must be `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-terminal-regex-adr0010-20260921T154419Z/continuation-publication`, max_object_bytes=67108864, sole capability `STOP_AFTER_DESCRIBE_REQUESTS`.
4. Production capture defaults: artifact=67108864, parent=67108864, bindings=1000, receipts=1000, source=1048576, total_source=8388608, work=10000; committed-workspace fallback capability required.
5. Regex defaults: document=61440, pattern=4096, matches=100, work=65536. Hard maxima: document=16777216, pattern=4096, matches=1000, work=536870912. Duplicate keys must reject.
6. Require current schema pins from PREFLIGHT, including V6 source snapshot, structural-context V6 input, and V4 domain-error envelope.
7. Call `lsp_session_v1_list` once; retain it; require exact project READY, workers=0, and MCP hash match. Substitute only returned session/generation below.
8. Run validator and require exact fingerprint `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`.

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
  "continuation": {"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}
}
```
Invoke census exactly once. Retain the public response first. Accept only progression beyond `PROGRAM_C_COMPUTED`, or exactly one typed `FAILED_CAPTURE` checkpoint. Diagnostics must be checkpoint-first and public; terminal diagnostics must not disappear after committed-workspace fallback. Stop with no retry/reconnect/model/worker/commit/stash/reset/revert/clean.

## Regex post-probe
A non-mutating regex probe may run only after the one-shot census result is retained. Because the one-shot discipline forbids any further request in this qualification, defer the live probe to a later fresh process; defaults/recovery are pinned here by bounded smoke only.

## Rollback
Run `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-terminal-regex-adr0010-20260921T154419Z/rollback.sh`, then fully restart Pi.
