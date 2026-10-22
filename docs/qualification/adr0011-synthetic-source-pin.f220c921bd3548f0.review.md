# ADR0011 synthetic source-pin successor candidate — f220c921bd3548f0

Status: INDEPENDENT SOURCE-BYTE REVIEW PASS / UNSELECTED

## Candidate identity

- Target parent/source commit: `8ef322fa23907f261be4325ba86ae959937e01e6`
- Target implementation tree: `df4b403a2782b86285fe9465cb2fc9461a8f6704`
- Producer commit: `3e770c287fa3dce7ea2844ea1b9feb328a4b7e1b`
- Producer tree: `3d563f21ba80cf1c76f1db2555a4e3eebcbb83d7`
- Predecessor manifest: `adr0011-synthetic-source-pin.278b9e64d953acc5.manifest.json`
- Predecessor manifest SHA-256: `278b9e64d953acc58952bdc9e61489db4df04efa971ef061d3b1df7cb51de8cc`
- Predecessor aggregate digest: `sha256:5e6f604fcc3f3de948c9ded11b54735ec1a6962b811e1a2c1c8bfe05b1cb9561`
- Successor manifest: `adr0011-synthetic-source-pin.f220c921bd3548f0.manifest.json`
- Successor manifest SHA-256: `f220c921bd3548f0a781e5b40de2916a0c477d2ce0eadc4b3541dd8e14da5219`
- Successor aggregate digest: `sha256:a40edc0a604d4217ed67be1641660bb4eae969bba069188137d029ca93e5fbe3`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: `157` files
- Selected source bytes: `1,503,852`
- Canonical manifest bytes: `25,040`

The producer commit has exactly two changed paths from target `8ef322fa23907f261be4325ba86ae959937e01e6`: addition of the successor manifest and a bounded update to the producer-candidate guard. It changes no predecessor manifest, predecessor review, reviewed selector, authority ceiling, schema, provider behavior, runtime behavior, or Census behavior.

## Immutable predecessor

The predecessor manifest and its review remain byte-identical. Its recorded identity remains 157 files, 1,503,212 selected bytes, manifest SHA-256 `278b9e64d953acc58952bdc9e61489db4df04efa971ef061d3b1df7cb51de8cc`, and aggregate `sha256:5e6f604fcc3f3de948c9ded11b54735ec1a6962b811e1a2c1c8bfe05b1cb9561`. Neither predecessor artifact was rewritten.

## Independent selection and byte recomputation

An independent read-only reviewer reconstructed the selection from exact source commit `8ef322fa23907f261be4325ba86ae959937e01e6`. The resulting 157 paths are unique and match the manifest exactly, with no omission, extra path, duplicate, symlink leaf, mode difference, order difference, byte-length mismatch, or SHA-256 mismatch. Every selected entry is a regular non-symlink file with mode `100644`.

Relative to the predecessor manifest, 155 entries are byte-identical, two existing entries changed, zero entries were added, and zero entries were removed.

Changed entries:

1. `cmd/lsp-trace/census_private_discovery_causes.go`
2. `internal/censusacquisition/discovery.go`

Every selected source file was independently read and matched to its manifest byte length and SHA-256. Re-encoding `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00`, then each ordered UTF-8 path and record with uint64-big-endian path length, path bytes, uint64-big-endian file length, and raw SHA-256 bytes reproduced aggregate `sha256:a40edc0a604d4217ed67be1641660bb4eae969bba069188137d029ca93e5fbe3`. Compact sorted-key JSON plus one trailing LF reproduced exactly 25,040 bytes, SHA-256 `f220c921bd3548f0a781e5b40de2916a0c477d2ce0eadc4b3541dd8e14da5219`, and filename prefix `f220c921bd3548f0`.

The mandatory path set and ordering are exact. Source-pin ceilings remain unchanged at 2 MiB per file, 16 MiB total, and 256 entries. A mismatch in path class, count, order, uniqueness, mode, symlink status, per-file length, per-file hash, aggregate, canonical JSON, manifest hash, filename prefix, mandatory paths, or ceilings would have failed the review; none occurred.

## Test interpretation

The independent review reproduced the candidate manifest identity, source-set aggregate, path count, selected-byte count, canonical-manifest byte count, lineage, modes, hashes, ordering, mandatory paths, and unchanged ceilings. It found no findings.

The reviewed selector remains pinned to the accepted predecessor at this review boundary. This is the expected unselected state, not a candidate defect. Selector advancement is a separate additive integration act permitted only after this review artifact is committed.

## Closure and authority ceiling

Verdict: **PASS for independent manifest review readiness only**.

This source pin authenticates only the explicitly selected source bytes at the stated source commit. It does not authenticate a compiled binary, dependency or toolchain closure, provider identity or behavior, runtime behavior or state, Census execution or results, inventory completeness or identity, semantic correctness or completeness, production behavior or state, deployment readiness, or deployment state.

This PASS grants source-byte authority only. It grants no binary authority, dependency authority, provider authority, runtime authority, Census authority, inventory authority, semantic authority, production authority, deployment authority, acceptance authority, self-acceptance, capability, final-issuance authority, production admission, or production change authority. Selector advancement is a separate additive integration act.

No provider operation, Census run, selector advancement, prior-artifact rewrite, policy change, manifest change, source change, production action, deployment action, or push was performed by the independent review.
