# ADR0011 synthetic source-pin successor producer record — 6054423fc7d6c991

Status: PRODUCER CANDIDATE / UNREVIEWED / UNSELECTED

## Target and lineage

- Target implementation commit: `025ffaa2349a3dd245122029e8051e4025365315`
- Target implementation tree: `28ee6d9239a16d0d18038e42fa429af0bb69acc3`
- Prior selected source target: `4cb53722698a63d0c501b4ec0fd7ee4755debb41`
- Prior manifest: `docs/qualification/adr0011-synthetic-source-pin.4f3f7b065edca7c1.manifest.json`
- Prior manifest SHA-256: `4f3f7b065edca7c104f15e88ed16af726296ba6fbb8f539a8d2312b4cd60e829`
- Prior aggregate: `sha256:5d1e36a57e7b043a4ece612184ad14fbaccd7a4fa886edd4bbde0438a96d1fe1`

## Candidate identity

- Manifest: `docs/qualification/adr0011-synthetic-source-pin.6054423fc7d6c991.manifest.json`
- Manifest SHA-256: `6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6`
- Manifest bytes: 23579
- Aggregate: `sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered paths: 148
- Selected source bytes: 1448693

## Exact path-set delta

Added production Go paths, in suffix order:

1. `cmd/lsp-trace/acquisition_v2.go`
2. `cmd/lsp-trace/census_discovery_adapter.go`
3. `internal/censusacquisition/discovery.go`
4. `internal/censusacquisition/request_budget.go`

Removed paths: none.

The preceding 144-path policy remains intact: twelve directory-selected direct regular non-symlink `.go` sets excluding `_test.go`, the historical five-path Go suffix, the previously authorized ten-path Census successor Go suffix, and the single authorized `internal/mcpcontract/testdata/schemas/envelope-census-chunk-terminal-result.v1.schema.json` suffix. No test, documentation, CI, script, arbitrary JSON, sibling schema, or other changed path is admitted. The successor envelope schema bytes and path are unchanged.

## Encoding

The aggregate is SHA-256 over `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then for each ordered entry: uint64-be UTF-8 path-byte count, path bytes, uint64-be file-byte count, and raw 32-byte file SHA-256. The manifest is compact sorted-key JSON plus one LF; its filename uses the first 16 hexadecimal characters of its complete-byte SHA-256.

## Authority boundary

This is producer-authored source-byte provenance only. It is not an independent review, acceptance, selector, production admission, Census invocation, binary identity, dependency closure, provider authentication, runtime qualification, or authority grant. Prior manifest and review bytes remain immutable. Independent review and any later selection are separate acts owned by other writers.
