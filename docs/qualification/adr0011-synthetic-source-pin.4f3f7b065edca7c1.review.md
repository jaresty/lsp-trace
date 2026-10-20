# ADR0011 synthetic source-pin successor candidate — 4f3f7b065edca7c1

Status: AUTHOR-PREPARED / INDEPENDENT ACCEPTANCE NOT CLAIMED / UNSELECTED

## Candidate identity

- Base commit: `262c54b7c83ef114d8a6f0f9a5fac99e3028962a`
- Predecessor manifest: `adr0011-synthetic-source-pin.16b4621be6f4b8d1.manifest.json`
- Predecessor manifest SHA-256: `16b4621be6f4b8d14623771bd581470ac5234f621898747c1131d97d2e778423`
- Predecessor review: `adr0011-synthetic-source-pin.16b4621be6f4b8d1.review.md`
- Predecessor review SHA-256: `e59bae7c3569ce6638f0dab6909be5c0727ad4bb8bfb1b69f4af2f1defaf8e8a`
- Candidate manifest: `adr0011-synthetic-source-pin.4f3f7b065edca7c1.manifest.json`
- Candidate manifest SHA-256: `4f3f7b065edca7c104f15e88ed16af726296ba6fbb8f539a8d2312b4cd60e829`
- Candidate aggregate digest: `sha256:5d1e36a57e7b043a4ece612184ad14fbaccd7a4fa886edd4bbde0438a96d1fe1`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 144 files
- Selected source bytes: 1370506
- Manifest bytes: 22966

## Exact additive delta

The predecessor's directory-selected production `.go` paths and ordered five-path suffix remain in place. `cmd/lsp-trace/census_chunking.go` is rehashed from the base commit through normal selection. The candidate appends these exact production `.go` paths, in this order:

1. `cmd/lsp-trace-mcp/census_chunking.go`
2. `cmd/lsp-trace-mcp/census_completion.go`
3. `cmd/lsp-trace-mcp/census_mcp_binding.go`
4. `cmd/lsp-trace-mcp/census_runtime.go`
5. `cmd/lsp-trace-mcp/main.go`
6. `cmd/lsp-trace/census_chunking_adapters_private.go`
7. `internal/censuschunkingruntime/adapters.go`
8. `internal/censuschunkingruntime/translate.go`
9. `internal/mcpcontract/census_future.go`
10. `internal/operation/types.go`

The final typed embedded-schema suffix path is:

11. `internal/mcpcontract/testdata/schemas/envelope-census-chunk-terminal-result.v1.schema.json`

No other path class is added. The aggregate remains `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed for each ordered entry by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. The manifest is compact sorted-key JSON with one trailing newline; its filename uses the first 16 hexadecimal characters of its SHA-256.

## Reproducibility commands

From base commit `262c54b7c83ef114d8a6f0f9a5fac99e3028962a`:

```sh
shasum -a 256 docs/qualification/adr0011-synthetic-source-pin.16b4621be6f4b8d1.manifest.json docs/qualification/adr0011-synthetic-source-pin.16b4621be6f4b8d1.review.md
shasum -a 256 docs/qualification/adr0011-synthetic-source-pin.4f3f7b065edca7c1.manifest.json
go test ./internal/adr0011acquisition -run '^TestSyntheticSourcePin($|AdditionalProductionPaths$|CeilingsUnchanged$)' -count=1
git diff --check
```

Independently enumerate immediate non-test `.go` files from the twelve directory allowlist entries, sort those paths, append the preserved five-path suffix, then append the eleven paths above. Require exactly 144 unique regular non-symlink files. Recompute each byte length and SHA-256, the length-prefixed aggregate, compact manifest bytes, manifest SHA-256, and filename.

## Authority ceiling

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, selection, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, public schema authority, or qualification. A later independent act is required before any pointer change.
