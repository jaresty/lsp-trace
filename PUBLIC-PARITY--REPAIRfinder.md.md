# Public analytics v2 parity repair

## Scope

Completed the additive layer-28 public analytics v2 repair for analysis, metrics, and ranking CLI/MCP operations. No network, install, deploy, product, live-D01, full-suite, full-race, or delegated execution was used.

## Rooted input

- Added bounded no-follow regular-file reads beneath the process-pinned immutable publication root.
- Added mutually exclusive inline `input` and rooted `publication_selector` carriers.
- A rooted selector binds relative selector, exact graph schema ID, SHA-256 digest, and byte length.
- Admission rejects absolute, escaping, missing, symlinked, wrong-family, wrong-length, and wrong-digest inputs before analytics.
- The pinned root is passed as host-only operation context and cannot be supplied by MCP JSON.

## Output and provenance

- Results expose exactly `UNVERIFIED_LOCAL` or `VERIFIED_PROVENANCE`.
- Inline/raw bytes remain `UNVERIFIED_LOCAL`.
- `VERIFIED_PROVENANCE` requires verifier evidence for rooted exact bytes, schema, digest, and length; digest alone cannot elevate a claim.
- Existing output publication remains atomic, owner-rooted, reread-verified, and no-replace.

## MCP and parity

- Added five immutable v2-specific envelope schemas: artifact, publication, compact publication, publication error, and domain error.
- Added the three v2 operations to manifest-backed offline input validation.
- Added exact artifact-family identification for analysis, metrics, and ranking.
- Added a real-process guard comparing CLI and MCP bytes for all three operations, COMPLETE/LIMIT, inline string/raw JSON, and rooted selectors.

## Compatibility

- Historical schemas and aliases were not edited.
- Historical 25-tool composition remains owned by its prior layer.
- Current additive composition remains 28 canonical tools.
- Existing four v2 schema families remain available through the shared schema registry and validation/verification mechanisms.

## Focused evidence

- `go test -race ./cmd/lsp-trace-mcp -run '^TestPublicAnalyticsV2ProcessExactParity$' -count=1` — 1 passed.
- Affected focused packages — 52 passed in 7 packages.
- `go test ./internal/schema ./internal/mcpcontract -count=1` — 168 passed in 2 packages.

## Claim boundary

This is source-ready, synthetic local qualification only. It does not grant Program B admission, producer authenticity, source truth, production authority, deployment, or shipment status.
