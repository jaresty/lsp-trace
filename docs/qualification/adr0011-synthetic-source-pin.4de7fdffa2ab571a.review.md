# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.cdcff32c898917cf.manifest.json`
- Predecessor manifest SHA-256: `cdcff32c898917cf19a6c9ae0cc702ab12650b9fe35c1e12f725f897000c0335`
- Successor manifest: `adr0011-synthetic-source-pin.4de7fdffa2ab571a.manifest.json`
- Successor manifest SHA-256: `4de7fdffa2ab571ae8edf859a4bfc9edfbe37c777e612b17d468bd89ebde3ef3`
- Successor aggregate digest: `sha256:7833f2bc23c6e211099999c33ad7b013bb0f11c5bd6348b7ffb97d1b8b8fb554`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; its ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly one byte-derived entry differs from the predecessor:

- `sessionruntime/provider_message_composition.go`
- Byte length: `4485` -> `4661`
- SHA-256: `sha256:3b3ca67dbcd2254c1673ea6e5542710b9fec6152f61a68f676663c870458e022` -> `sha256:bbff1d42e352a58eba1328f671381b197503fa8456d05860dfdea03b1f1ea3d3`

## Independent reviewer checklist

- [ ] Confirm the predecessor manifest and all prior review artifacts are byte-identical to their committed versions.
- [ ] Confirm both manifests contain the same 128 paths in the same order.
- [ ] Confirm only the entry named above differs.
- [ ] Independently recompute that file's byte length and SHA-256.
- [ ] Independently recompute the aggregate digest using the production verifier encoding.
- [ ] Independently recompute the successor manifest SHA-256 and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream final-issuance guards.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This candidate records source-byte reproducibility only. It does not establish binary identity, dependency closure, provider authentication, runtime behavior, production authority, Census execution, or independent acceptance.
