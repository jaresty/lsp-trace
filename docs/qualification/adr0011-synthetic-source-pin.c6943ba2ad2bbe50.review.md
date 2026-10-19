# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.33e3f6654709c47e.manifest.json`
- Predecessor manifest SHA-256: `33e3f6654709c47e6b0c484590cbb897b9d04114ec1f550bb3625ec086ac10c9`
- Successor manifest: `adr0011-synthetic-source-pin.c6943ba2ad2bbe50.manifest.json`
- Successor manifest SHA-256: `c6943ba2ad2bbe5056747fef228594cc729d2da3d33e25305156cdd54b43d913`
- Successor aggregate digest: `sha256:b43f593376bb04da83402157d1a12ae14a154c989f4fb6af3ce5b435816ca925`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; its ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly one byte-derived entry differs from the predecessor:

- `sessionruntime/sessionruntime.go`
- Byte length: `144797` -> `144899`
- SHA-256: `sha256:1e5f2ab9d787aab177f168f17c41b378909e977eb9ac05ba38a88b87273ea950` -> `sha256:0745c83c4d9e50a42ce5713c4298e4ba4e6681f8f73fc2960a049ab2f274bc0e`

The changed production bytes rename the private v5 scalar from a provider-write claim to `SelectedResponseReadCompleted` and define it solely from authoritative Manager-side completed selected response-frame read evidence. No provider instrumentation or hook was added. The source set, order, and denominator remain unchanged.

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
