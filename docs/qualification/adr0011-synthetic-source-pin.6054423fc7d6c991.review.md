# ADR0011 synthetic source-pin successor candidate — 6054423fc7d6c991

Status: INDEPENDENT SOURCE-BYTE REVIEW PASS / UNSELECTED

## Candidate identity

- Producer commit: `d2f9808a53a157c3f4816023ccb69c31658a6f5d`
- Target parent/source commit: `025ffaa2349a3dd245122029e8051e4025365315`
- Target implementation tree: `28ee6d9239a16d0d18038e42fa429af0bb69acc3`
- Producer tree: `ebba29da9862fd1fb2f507418a4dbf12436d6a20`
- Predecessor manifest: `adr0011-synthetic-source-pin.4f3f7b065edca7c1.manifest.json`
- Predecessor manifest SHA-256: `4f3f7b065edca7c104f15e88ed16af726296ba6fbb8f539a8d2312b4cd60e829`
- Predecessor manifest bytes: `22,966`
- Predecessor review: `adr0011-synthetic-source-pin.4f3f7b065edca7c1.review.md`
- Predecessor review SHA-256: `15d8135333b92eb248d202c126780d3fd440e19096c20e36a352393adf5b9505`
- Predecessor review bytes: `6,244`
- Candidate manifest: `adr0011-synthetic-source-pin.6054423fc7d6c991.manifest.json`
- Candidate manifest SHA-256: `6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6`
- Candidate aggregate digest: `sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `148` files
- Selected source bytes: `1,448,693`
- Manifest bytes: `23,579`

The predecessor manifest and review were hashed directly at the producer commit and remain byte-identical to their committed objects. This review does not rewrite either predecessor artifact.

## Independent selection and byte recomputation

The selection was independently reconstructed from the verifier's twelve immediate-directory production `.go` allowlists, excluding `_test.go`, followed by the exact ordered additional-path suffix. It produced 128 canonically sorted directory paths and 20 suffix paths. Comparison with the candidate yielded 148 unique paths in exact order, with no omitted path, no extra path, and no duplicate.

Relative to the predecessor manifest, the candidate has exactly four additions and zero removals:

1. `cmd/lsp-trace/acquisition_v2.go`
2. `cmd/lsp-trace/census_discovery_adapter.go`
3. `internal/censusacquisition/discovery.go`
4. `internal/censusacquisition/request_budget.go`

Exactly four pre-existing selected entries changed bytes:

1. `cmd/lsp-trace-mcp/census_chunking.go`: `5,553` -> `5,641` bytes
2. `cmd/lsp-trace-mcp/census_runtime.go`: `16,981` -> `17,770` bytes
3. `cmd/lsp-trace/census_runner.go`: `11,438` -> `11,806` bytes
4. `internal/adr0011acquisition/synthetic_source_pin.go`: `7,555` -> `7,730` bytes

The target-source comparison from prior selected source target `4cb53722698a63d0c501b4ec0fd7ee4755debb41` to target parent/source `025ffaa2349a3dd245122029e8051e4025365315` contains 18 changed paths. All 7 changed production paths are selected: the three changed pre-existing Census paths above plus the four additions. All 9 changed `_test.go` paths, `.github/workflows/ci.yml`, and `scripts/check-ci.sh` are excluded. The sole admitted JSON path is exactly `internal/mcpcontract/testdata/schemas/envelope-census-chunk-terminal-result.v1.schema.json`; no arbitrary, sibling, renamed, or additional schema is selected.

Every selected file was read from the producer checkout and independently checked against its manifest byte length and SHA-256; all 148 entries matched. Re-encoding `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then each ordered UTF-8 path and file record with uint64-big-endian lengths and raw SHA-256 bytes, reproduced aggregate `sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339`. Compact sorted-key JSON plus one trailing LF reproduced exactly 23,579 manifest bytes, SHA-256 `6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6`, and filename prefix `6054423fc7d6c991`.

A contrary result in any path count, order, uniqueness check, allowlist reconstruction, byte length, per-entry hash, aggregate, canonical encoding, complete manifest hash, or filename prefix would have failed this review; none occurred.

## Verification

The following bounded gates were run from clean exact producer HEAD `d2f9808a53a157c3f4816023ccb69c31658a6f5d`:

- `go test ./internal/adr0011acquisition -run '^TestSyntheticSourcePin($|AdditionalProductionPaths$|CeilingsUnchanged$|ProducerCandidate$)' -count=1` — PASS, 4 tests.
- `go test ./internal/adr0011acquisition -run '^TestReviewedSyntheticSourcePin$' -count=1` — expected FAIL, 1 test: the reviewed selector intentionally remains pinned to predecessor manifest `4f3f7b065edca7c1`; no selector advancement was performed.
- `go test ./internal/censusacquisition -count=1` — PASS, 124 tests.
- `go test ./internal/censuschunking -count=1` — PASS, 154 tests.
- `go test ./cmd/lsp-trace -run 'Census|Acquisition|Discovery|Budget' -count=1` — PASS, 223 tests.
- `go test ./cmd/lsp-trace-mcp -run 'Census|Acquisition|Gateway|Chunk' -count=1` — PASS, 221 tests.
- `go vet ./internal/adr0011acquisition ./internal/censusacquisition ./internal/censuschunking ./cmd/lsp-trace ./cmd/lsp-trace-mcp` — PASS.
- Independent DuckDB manifest projection/join — 148 current unique paths and 1,448,693 bytes; 144 predecessor unique paths and 1,370,506 bytes; 4 additions; 0 removals.
- Independent per-entry, aggregate, canonical-JSON, complete-file, and filename-prefix recomputation — PASS.
- `git diff --check` and final one-file diff/cleanliness checks — PASS; exactly this review artifact was untracked before commit.

One combined affected-package `go test` invocation reached its 240-second harness timeout. It is not counted as a semantic failure or success; the same bounded package coverage was rerun as the four passing shards listed above. These tests and byte checks establish the stated source-set and local verifier properties only. They do not establish complete downstream causal equivalence for every runtime behavior affected by the seven production paths.

## Authority ceiling

Verdict: **PASS for independent manifest review readiness only**.

This PASS establishes only independently reviewed source-byte manifest integrity at the named commits and trees. It is **NOT selector acceptance**, does not update or select the reviewed pointer, and does not authorize Census execution, inventory inference, production admission, or any production change.

It grants no binary identity, build or dependency closure, provider authentication, runtime qualification, complete downstream causal equivalence, Census result authority, feature-inventory authority, selector authority, production authority, capability, final issuance, or deployment authority. Any later selector advancement or acceptance remains a separate act by an authorized writer/reviewer.

No Census run, inventory inference, selector advancement, prior-artifact rewrite, production-code change, or push was performed by this review.
