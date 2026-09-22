# Full-Pi-restart qualification handoff

## User action

**Fully restart Pi now.** Do not use `mcp connect`; the current process holds stale in-memory MCP configuration. Transient managed sessions are expected to restart.

## Post-restart pre-census verification

Run these checks before any census:

```sh
ROOT=/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-20260921T110946Z
python3 - <<'PY'
import json
p='/Users/schwa/dev/lsp-trace/.mcp.json'
s=json.load(open(p))['mcpServers']['lsp-trace']
print(' '.join([s['command'], *s['args']]))
print('publication-root-count=',s['args'].count('--publication-root'))
print('env=',s.get('env',{}))
PY
shasum -a 256 /Users/schwa/.local/bin/lsp-trace /Users/schwa/.local/bin/lsp-trace-mcp
stat -f '%Sp %N' "$ROOT" "$ROOT/base-publication" "$ROOT/continuation-publication" "$ROOT/trace" "$ROOT/ledger" "$ROOT/trace/runtime.trace" "$ROOT/ledger/publication-failures.ndjson"
```

Expected exact configured argv:

```text
/Users/schwa/.local/bin/lsp-trace-mcp --bootstrap-config /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json --publication-root /Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-20260921T110946Z/base-publication --tool-profile compact
```

Expected:

- `publication-root-count= 1`.
- `lsp-trace`: `612df012de2278083603b93dd71354d2c31dab39f6e6406bceb752cd8b833cd8`.
- `lsp-trace-mcp`: `686137d78737b66ee0f03e44bf474a99b3b202dfedcd76183c70b5df14173850`.
- Directories are `drwx------`; sink files are regular `-rw-------` files.
- Base root is `.../base-publication`; bootstrap continuation root is the distinct `.../continuation-publication`; `max_object_bytes` is `67108864` and capability is `STOP_AFTER_DESCRIBE_REQUESTS`.
- Diagnostic sinks are **default-off**: `.mcp.json` has no `env` bindings. Prepared activation values are in owner-only `diagnostic-sinks.env`; do not enable them during this run unless explicitly required.

Then use the restarted Pi MCP tools to call `lsp_session_v1_list` once. Require the exact `/Users/schwa/dev/lsp-trace` project session to be `READY`, report `Workers = 0`, and use generation `G` from that result. Confirm its server image digest is `sha256:686137d78737b66ee0f03e44bf474a99b3b202dfedcd76183c70b5df14173850`. Confirm the live child argv (for example with `ps -axo pid=,command= | grep '[l]sp-trace-mcp'`) exactly equals the argv above.

**Stop before census** on any argv, count, root, digest, mode, READY, generation, worker-count, continuation-capability, or sink-mode mismatch. Do not reconnect, restart a managed session, start a model, or start a worker to repair a mismatch.

## Exact one-shot census

Only after every precondition passes, call `lsp_trace_v1_census` exactly once with the exact project session ID `S` and generation `G` returned above:

```json
{
  "session_id": "S",
  "generation": G,
  "sources": ["."],
  "continuation": {
    "kind": "ADR_0007_FEATURE_CATALOG",
    "stop_after": "DESCRIBE_REQUESTS"
  }
}
```

Stop rules:

1. One invocation only: no retry, replay, reconnect, second census, PAUSED continuation, packet inspection, model sample, or worker start.
2. Stop immediately after retaining the complete census response and exact request parameters, regardless of `COMPLETE`, `PAUSED`, `COMMITTED_DEGRADED`, or domain error.
3. If transport/schema/configuration fails before an attributable census envelope is returned, retain the failure and stop; do not repair in-session.
4. Keep the prepared diagnostic sinks off. If they are unexpectedly active or non-empty before census, stop.

Rollback, if needed after restart or verification failure:

```sh
/Users/schwa/dev/lsp-trace/.pi/evidence/pi-full-restart-qualification-20260921T110946Z/rollback.sh
```
