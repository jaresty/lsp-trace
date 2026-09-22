# Full-Pi one-shot qualification: V2 batch-target admission + accepted capture/regex/ADR0010 changes

Status is valid only when `PREFLIGHT.json` says `STAGED_VALID`.

## User action
Fully restart Pi. Do not reconnect MCP or restart a managed session in this process.

## Post-restart preflight (stop on any mismatch)
1. `.mcp.json` contains exactly one base `--publication-root`: `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v2-admission-20260921T160121Z/base-publication`; it contains no MCP env or diagnostic sinks.
2. CLI/MCP and config hashes match `PREFLIGHT.json`.
3. Base, continuation, trace, and ledger roots are pairwise distinct, brand-new, empty, owner-only mode 0700.
4. `.lsp-trace-mcp-bootstrap.json` is owner-only mode 0600. Its continuation root is `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v2-admission-20260921T160121Z/continuation-publication`, `max_object_bytes=67108864`, and its sole capability is `STOP_AFTER_DESCRIBE_REQUESTS`.
5. Production capture defaults are artifact=67108864, parent=67108864, bindings=1000, receipts=1000, source=1048576, total_source=8388608, work=10000; committed-workspace fallback capability is present.
6. Regex defaults are document=61440, pattern=4096, matches=100, work=65536. Hard maxima are document=16777216, pattern=4096, matches=1000, work=536870912. Duplicate keys reject.
7. Current schema pins match `PREFLIGHT.json`, including V6 source snapshot, structural-context V6 input, and V4 domain-error envelope.
8. Call `lsp_session_v1_list` once; retain it; require exact project `READY`, workers=0, and MCP hash match. Substitute only its returned session and generation below.
9. Run `validate-staged-request.py REQUEST.json --handoff HANDOFF.md`; require exact fingerprint `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`.

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
Invoke census exactly once. Retain the public response before inspecting any private artifacts. Accept only progression beyond `PROGRAM_C_COMPUTED`, or exactly one typed `FAILED_CAPTURE` checkpoint. Diagnostics must be checkpoint-first and public. Terminal diagnostics must remain present after committed-workspace fallback. Stop with no retry, reconnect, model, worker, commit, stash, reset, revert, or clean.

## Regex post-probe
Do not make a second live request in this qualification. Regex defaults, hard maxima, duplicate rejection, and recovery behavior are pinned by bounded smoke tests recorded in `PREFLIGHT.json`.

## Rollback
Run `/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-v2-admission-20260921T160121Z/rollback.sh`, then fully restart Pi.
