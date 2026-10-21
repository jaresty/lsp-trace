# ADR0011 synthetic source-pin successor candidate — 278b9e64d953acc5

Status: INDEPENDENT SOURCE-BYTE REVIEW PASS / UNSELECTED

## Candidate identity

- Target parent/source commit: `ed06af956b64974594d54b6ee049466e6d823817`
- Target implementation tree: `ec536ca92de726f232f31789800093bed68a0290`
- Producer commit: `050b3ecc5f8ea4d5c1d8a4b195546019b086ee79`
- Producer tree: `89eff132b866807ae13cbc75385c66cd134774da`
- Predecessor manifest: `adr0011-synthetic-source-pin.450cebe06c081c5d.manifest.json`
- Predecessor manifest SHA-256: `450cebe06c081c5db73f7c94978902480e0d198ea4c1c28a99a79efb024d603d`
- Predecessor aggregate digest: `sha256:53d91cc911f7e53e5d16a5bba7ff27277e75f4265bf9baf8583573a4033da981`
- Successor manifest: `adr0011-synthetic-source-pin.278b9e64d953acc5.manifest.json`
- Successor manifest SHA-256: `278b9e64d953acc58952bdc9e61489db4df04efa971ef061d3b1df7cb51de8cc`
- Successor aggregate digest: `sha256:5e6f604fcc3f3de948c9ded11b54735ec1a6962b811e1a2c1c8bfe05b1cb9561`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `157` files
- Selected source bytes: `1,503,212`
- Canonical manifest bytes: `25,040`

The producer commit has exactly three changed paths from target `ed06af956b64974594d54b6ee049466e6d823817`: addition of the successor manifest and bounded updates to the source-pin profile and producer-candidate guard. It changes no predecessor manifest, predecessor review, reviewed selector, authority ceiling, schema, provider behavior, or Census behavior.

## Immutable predecessor

The predecessor manifest and its review remain byte-identical. Its recorded identity remains 156 files, 1,497,243 selected bytes, manifest SHA-256 `450cebe06c081c5db73f7c94978902480e0d198ea4c1c28a99a79efb024d603d`, and aggregate `sha256:53d91cc911f7e53e5d16a5bba7ff27277e75f4265bf9baf8583573a4033da981`. Neither predecessor artifact was rewritten.

## Independent selection and byte recomputation

An independent read-only reviewer reconstructed the selection from exact commit `050b3ecc5f8ea4d5c1d8a4b195546019b086ee79`. The resulting 157 paths are unique and match the manifest exactly, with no omission, extra path, duplicate, symlink leaf, order difference, byte-length mismatch, or SHA-256 mismatch.

Relative to the predecessor manifest, 152 entries are byte-identical, four existing entries changed, one entry was added, and zero entries were removed.

Changed entries:

1. `cmd/lsp-trace/census_assembly.go`
2. `cmd/lsp-trace/census_core.go`
3. `cmd/lsp-trace/census_runner.go`
4. `internal/adr0011acquisition/synthetic_source_pin.go`

Added entry:

1. `cmd/lsp-trace/census_private_discovery_causes.go`

The added entry occurs exactly once, immediately after `cmd/lsp-trace/census_core.go` and before `cmd/lsp-trace/census_runner.go` in the required additional-source order.

Every selected producer file was independently read and matched to its manifest byte length and SHA-256. Re-encoding `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then each ordered UTF-8 path and record with uint64-big-endian path length, path bytes, uint64-big-endian file length, and raw SHA-256 bytes reproduced aggregate `sha256:5e6f604fcc3f3de948c9ded11b54735ec1a6962b811e1a2c1c8bfe05b1cb9561`. Compact sorted-key JSON plus one trailing LF reproduced exactly 25,040 bytes, SHA-256 `278b9e64d953acc58952bdc9e61489db4df04efa971ef061d3b1df7cb51de8cc`, and filename prefix `278b9e64d953acc5`.

A mismatch in path class, count, order, uniqueness, per-file length, per-file hash, aggregate, canonical JSON, manifest hash, or filename prefix would have failed the review; none occurred.

## Test interpretation

The producer candidate, ceilings, and additional-path guards passed, and the complete `TestSyntheticSourcePin` family passed. Source-pin ceilings remain 2 MiB per file, 16 MiB total, and 256 entries.

`TestReviewedSyntheticSourcePin` fails at the producer commit because the reviewed selector intentionally remains pinned to the historical 156-entry predecessor. This is the expected unselected boundary, not a candidate defect. The selector may advance only after this additive independent review artifact is integrated.

## Closure and authority ceiling

Verdict: **PASS for independent manifest review readiness only**.

This source pin authenticates only the explicitly selected checkout bytes. It does not authenticate build inputs outside the selected files, tests, other JSON, documentation, generated or untracked content, Go toolchain or dependency closure, compiled binary identity, process or host state, runtime behavior, provider authentication, live analyzed version, Census execution or results, semantic completeness, feature identity, occurrence qualification, or production behavior.

This PASS does not itself select or advance the reviewed pointer and grants no provider authority, Census execution or result authority, acceptance authority, self-acceptance, feature-inventory authority, capability, final-issuance authority, production admission, deployment authority, or production change authority. Selector advancement is a separate additive integration act.

No provider operation, Census run, selector advancement, prior-artifact rewrite, policy change, manifest change, or push was performed by the independent review.
