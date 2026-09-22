# pi-mcp-adapter publication-root qualification

Date: 2026-09-21
Workspace: `/Users/schwa/dev/lsp-trace`
Result: **STOPPED BEFORE CENSUS**

## Intended supported change

The supported configuration seam is the project-local, machine-local `.mcp.json` server definition. `pi-mcp-adapter` passes `definition.args` directly to `StdioClientTransport`; launcher-prepended duplicate flags are neither needed nor appropriate.

The attempted config replacement changed the existing single pair:

```text
--publication-root /Users/schwa/dev/lsp-trace/.lsp-trace-publication
```

to exactly one pair:

```text
--publication-root /Users/schwa/dev/lsp-trace/.pi/evidence/pi-mcp-adapter-publication-root-qualification-20260921T110258Z/base-publication
```

It also independently changed bootstrap continuation publication to:

```text
/Users/schwa/dev/lsp-trace/.pi/evidence/pi-mcp-adapter-publication-root-qualification-20260921T110258Z/continuation-publication
```

with `max_object_bytes = 67108864`.

## Blocker

The adapter-supported `mcp connect` reconnect refreshed the server connection using the adapter's already-loaded in-memory `state.config` definition. It did not reload `.mcp.json` from disk.

Observed live argv after reconnect:

```text
/Users/schwa/.local/bin/lsp-trace-mcp --bootstrap-config /Users/schwa/dev/lsp-trace/.lsp-trace-mcp-bootstrap.json --publication-root /Users/schwa/dev/lsp-trace/.lsp-trace-publication --tool-profile compact
```

Therefore the required postcondition failed: the live argv contained one `--publication-root`, but it did not equal the intended fresh base root. Per the no-census stop rule, no census request, PAUSED replay, packet inspection, or local-model sample was executed.

## Exact supported action still needed

After applying the same `.mcp.json` and bootstrap replacements, perform a **full Pi session reload/restart that reloads project MCP configuration from disk**, then connect `lsp-trace`. A plain adapter server reconnect is insufficient because it reuses the loaded server definition.

Before census, verify:

1. The lsp-trace child argv contains exactly one `--publication-root`.
2. Its value equals the intended fresh base root.
3. Bootstrap continuation root equals the separate intended continuation root.
4. Runtime trace and publication-failure ledger files exist as regular 0600 files under 0700 parents.
5. The project session is READY on the installed current binary digest and `Workers = 0`.

## Smoke/build evidence

Focused smoke: `1125 passed in 7 packages`.

Built current artifacts:

- `artifacts/lsp-trace.new`: `sha256:612df012de2278083603b93dd71354d2c31dab39f6e6406bceb752cd8b833cd8`
- `artifacts/lsp-trace-mcp.new`: `sha256:686137d78737b66ee0f03e44bf474a99b3b202dfedcd76183c70b5df14173850`

The current binaries were briefly installed and the current MCP image reached READY with digest `sha256:686137d78737b66ee0f03e44bf474a99b3b202dfedcd76183c70b5df14173850`, but the live argv remained bound to the legacy root.

## Fresh roots and sinks

- Base: `base-publication/` (0700)
- Continuation: `continuation-publication/` (0700)
- Runtime trace: `trace/runtime.trace` (0600; empty)
- Failure ledger: `ledger/publication-failures.ndjson` (0600; empty)
- Maximum continuation object: 64 MiB

These are preserved as evidence but are default-off because the active `.mcp.json` was restored and contains no diagnostic sink environment bindings.

## Restoration

Restored byte-for-byte from owner-only rollback copies:

- `.mcp.json`: `sha256:d4c8595e89731e7ca48ba631d30ed26661a427611cc4f69df1525fe0f27fc471`
- `.lsp-trace-mcp-bootstrap.json`: `sha256:e5c823c423c9acbbd45d8b6b9eef12f5b24aeba568f74a9299fbfd9a76477072`
- installed `lsp-trace`: `sha256:d0e1d716e46167f7aa300b5834e91874b0b2fec65d70e219d2f653aa112e3412`
- installed `lsp-trace-mcp`: `sha256:00da842f664f572932895b561a693c415740bebcaaea048526f792b0395136d9`

Final live state: project READY, D01 READY, workers 0, original single legacy publication-root argv restored. The NAIS workspace was not modified.
