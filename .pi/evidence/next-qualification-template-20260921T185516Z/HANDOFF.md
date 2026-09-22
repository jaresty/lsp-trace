# ADR0007 next qualification handoff

This staging template is inert. It requires a **full Pi process restart** after an operator has copied it to a final run directory, supplied final binaries/configuration/schema pins and fresh empty roots, and completed `preflight.py`. Reconnect, managed-session restart, or reuse of the process that prepared this template is not equivalent.

## Exact one-shot request

Substitute only the exact READY `project` session ID and generation retained by the single post-restart session-list call. Do not alter any other field. Invoke census once; retain the outer delegated-gateway response before any private-artifact inspection; never retry unchanged.

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

Semantic fingerprint: `sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9`.

## Required sequence after the full restart

1. Retain exactly one pre-result session-list JSON response; do not perform session mutation.
2. Run `preflight.py` with the final CLI/MCP builds, final MCP/bootstrap configs, final schema files and expected hashes, four or more final empty runtime roots, retained session-list JSON, and this request/validator.
3. Require exactly one `routing.alias` each for `project` and `nais-discovery-cue`, both with `routing.readiness=READY`; permit other sessions such as `d01-csharp`. Require `Census.Workers=0` and every retained session's associated `server_instance.executable_sha256` to equal the pinned MCP digest. Also require no configured diagnostic sink unless the operator explicitly chose `--diagnostics-enabled`, and a valid request fingerprint.
4. Send the exact census request once through the delegated gateway.
5. Separately retain: (a) the complete public outer gateway result, (b) one post-result session-list JSON, and (c) a host/ledger execution-accounting JSON that identifies source and custody. Do not add accounting fields to the public response.
6. Run `check-result.py PUBLIC_RESPONSE.json --post-session-list POST_SESSION_LIST.json --execution-accounting EXECUTION_ACCOUNTING.json`. Require zero workers in both retained accounting sources and zero model invocations in the execution-accounting source.
7. Accept only strict composite V2 PAUSED at the described request/preparation boundary, strict historical V1 typed immutable failure, or strict additive V2 typed immutable failure.
8. Perform the V6/packet checklist manually against retained artifacts. Do not run a worker or model.

No step in this template authorizes execution, installation, configuration changes, session changes, retries, or cleanup.
