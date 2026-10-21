# ADR0011 synthetic source-pin successor candidate — 450cebe06c081c5d

Status: INDEPENDENT SOURCE-BYTE REVIEW PASS / UNSELECTED

## Candidate identity

- Target parent/source commit: `90085ea50a6f2c3e2b3ea66c9276d36b646b7550`
- Target implementation tree: `f6e71eb744c501f40a02f1ddfe7a3da59e20ed65`
- Producer commit: `dbd0d3b40b4979a958001ec220879f8c6ccca18c`
- Producer tree: `c273e94024c2f2d050e5a4dd042d41108f58e4b1`
- Predecessor manifest: `adr0011-synthetic-source-pin.6054423fc7d6c991.manifest.json`
- Predecessor manifest SHA-256: `6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6`
- Predecessor aggregate digest: `sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339`
- Successor manifest: `adr0011-synthetic-source-pin.450cebe06c081c5d.manifest.json`
- Successor manifest SHA-256: `450cebe06c081c5db73f7c94978902480e0d198ea4c1c28a99a79efb024d603d`
- Successor aggregate digest: `sha256:53d91cc911f7e53e5d16a5bba7ff27277e75f4265bf9baf8583573a4033da981`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `156` files
- Selected source bytes: `1,497,243`
- Canonical manifest bytes: `24,878`

The producer commit has exactly one path change from target `90085ea50a6f2c3e2b3ea66c9276d36b646b7550`: addition of the successor manifest. It changes no manifest predecessor, verifier policy, reviewed selector, production source, or schema.

## Immutable predecessor

The predecessor manifest remains byte-identical. Independent historical reproduction read all 148 entries from its exact producer checkout `d2f9808a53a157c3f4816023ccb69c31658a6f5d` (tree `ebba29da9862fd1fb2f507418a4dbf12436d6a20`), matched every recorded length and SHA-256, and reproduced 1,448,693 selected bytes, aggregate `sha256:6f475e674272e18d6b5fee5a2a23803225788e999ae1de84c050fb12c491e339`, canonical 23,579-byte JSON, and manifest SHA-256 `6054423fc7d6c991d7343b036ebec90b0d74f007801459ad58f96a0e666f68b6`. Its independent review remains an immutable later artifact; neither predecessor file was rewritten.

## Independent selection and byte recomputation

The selection was independently reconstructed from the verifier policy at producer HEAD. It contains exactly 128 canonically sorted immediate-directory non-test production `.go` paths, followed by the exact ordered 24 additional production `.go` paths and the exact ordered four admitted schema paths. The resulting 156 paths are unique and match the manifest exactly, with no omission, extra path, duplicate, symlink, or order difference.

Relative to the predecessor manifest, 142 entries are byte-identical, six existing entries changed, eight entries were added, and zero entries were removed.

Changed entries:

1. `cmd/lsp-trace/census_assembly.go`
2. `cmd/lsp-trace/census_core.go`
3. `cmd/lsp-trace/census_discovery_adapter.go`
4. `cmd/lsp-trace/census_runner.go`
5. `internal/adr0011acquisition/synthetic_source_pin.go`
6. `internal/censusacquisition/discovery.go`

Added entries:

1. `cmd/lsp-trace/census_command.go`
2. `internal/censuspreflight/result.go`
3. `internal/censusrequest/receipt_v2.go`
4. `internal/schema/schema.go`
5. `internal/schema/schemas/lsp-trace.census-enumeration-diagnostic.v1.schema.json`
6. `internal/schema/schemas/lsp-trace.census-enumeration-preflight-result.v1.schema.json`
7. `internal/schema/schemas/lsp-trace.census-request-receipt.v2.schema.json`
8. `internal/source/discovery.go`

Every selected producer file was independently read and matched to its manifest byte length and SHA-256. Re-encoding `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then each ordered UTF-8 path and record with uint64-big-endian path length, path bytes, uint64-big-endian file length, and raw SHA-256 bytes reproduced aggregate `sha256:53d91cc911f7e53e5d16a5bba7ff27277e75f4265bf9baf8583573a4033da981`. Compact sorted-key JSON plus one trailing LF reproduced exactly 24,878 bytes, SHA-256 `450cebe06c081c5db73f7c94978902480e0d198ea4c1c28a99a79efb024d603d`, and filename prefix `450cebe06c081c5d`.

A mismatch in path class, count, order, uniqueness, per-file length, per-file hash, aggregate, canonical JSON, manifest hash, or filename prefix would have failed this review; none occurred.

## Test interpretation

The focused structural verifier tests passed except for `TestSyntheticSourcePinProducerCandidate`, and `TestReviewedSyntheticSourcePin` also failed. Both tests are intentionally hard-coded to the older `6054423fc7d6c991` manifest and aggregate, while the checkout now contains changed and added selected source. These are expected old-current stale assertions demonstrating that no selector or old candidate test was advanced. They are not evidence that the immutable predecessor manifest or review is corrupt: the predecessor independently reproduces at its exact historical checkout as recorded above.

No test or selector at producer HEAD references `450cebe06c081c5d` or its aggregate. Independent byte and canonical reconstruction, rather than a self-selecting test update, is the basis for this PASS.

## Closure and authority ceiling

Verdict: **PASS for independent manifest review readiness only**.

This source pin closes only the explicitly selected checkout bytes. It excludes tests, other JSON, documentation, generated or untracked content, build inputs outside the selected files, Go toolchain and module/dependency closure, compiled binary identity, process and host state, runtime behavior, provider authentication, live analyzed version, occurrence qualification, and production behavior. Source completeness outside the exact 128-directory-Go + 24-additional-Go + four-schema policy is unknown; downstream causal and runtime completeness are not established.

This PASS does not select or advance the reviewed pointer and grants no provider authority, Census execution or result authority, acceptance authority, self-acceptance, selector authority, feature-inventory authority, capability, final-issuance authority, production admission, deployment authority, or production change authority. Any later selector advancement or acceptance is a separate authorized act.

No provider operation, Census run, selector advancement, prior-artifact rewrite, policy change, production-code change, manifest change, or push was performed by this review.
