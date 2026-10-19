# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.4de7fdffa2ab571a.manifest.json`
- Predecessor manifest SHA-256: `4de7fdffa2ab571ae8edf859a4bfc9edfbe37c777e612b17d468bd89ebde3ef3`
- Successor manifest: `adr0011-synthetic-source-pin.33e3f6654709c47e.manifest.json`
- Successor manifest SHA-256: `33e3f6654709c47e6b0c484590cbb897b9d04114ec1f550bb3625ec086ac10c9`
- Successor aggregate digest: `sha256:ea0cf4c563dfccd00b55398a5a6830c2d6f28d5a2d8b5c84357972d1b695c50c`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; its ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly one byte-derived entry differs from the predecessor:

- `sessionruntime/sessionruntime.go`
- Byte length: `141966` -> `144797`
- SHA-256: `sha256:fb03e7a923cdbccb955dcc166c91845f556b3b4f7336ae57b8638d9260507e99` -> `sha256:1e5f2ab9d787aab177f168f17c41b378909e977eb9ac05ba38a88b87273ea950`

The v5 scalar observation declarations were relocated from the newly introduced `sessionruntime/provider_transaction_observation.go` into the already selected `sessionruntime/sessionruntime.go`, and the separate file was removed. This preserves the predecessor's exact 128 paths, order, and denominator rather than broadening the selected source set.

## Independent reviewer checklist

- [ ] Confirm the predecessor manifest and every prior source-pin review artifact are byte-identical to their committed versions.
- [ ] Confirm both manifests contain the same 128 paths in the same order.
- [ ] Confirm only the entry named above differs.
- [ ] Independently recompute that file's byte length and SHA-256.
- [ ] Independently recompute the aggregate digest using the production verifier encoding.
- [ ] Independently recompute the successor manifest SHA-256 and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream final-issuance guards.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It does not grant independent acceptance and does not establish binary identity, dependency closure, provider authentication, runtime behavior, production authority, Census execution, or qualification beyond the exact local source-byte checks described above.
