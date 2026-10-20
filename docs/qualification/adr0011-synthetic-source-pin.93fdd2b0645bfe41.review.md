# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.7de70622758e21aa.manifest.json`
- Predecessor manifest SHA-256: `7de70622758e21aa3276850914b34d816ac0f6f6ba86b85ca245ec7b2d30bae8`
- Predecessor review: `adr0011-synthetic-source-pin.7de70622758e21aa.review.md`
- Predecessor review SHA-256: `269b9632b65de5b763107002078236067c4faade507e509d59c06d9c2cec9dbc`
- Successor manifest: `adr0011-synthetic-source-pin.93fdd2b0645bfe41.manifest.json`
- Successor manifest SHA-256: `93fdd2b0645bfe41366d4abd718daef2145112651e27d3e3ad48213b5b94a998`
- Successor aggregate digest: `sha256:b2649c7039d879554702b6fc57b775bbfd03a2b4106ae73ac07161785709fefe`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1170235

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; all 128 ordered paths were retained exactly; current bytes were hashed only for the two named changed paths; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly 2 byte-derived entries differ from the predecessor:

- `sessionruntime/diagnostic_runtime.go`: byte length `17436` -> `17476`; SHA-256 `sha256:4090aed6b0d4276e7cc5d1abc32b41da192726b83b4c176f48aa1824a1a4305e` -> `sha256:7a0058883dd9164b6eb1c665c1949640c3f30b71b6cbe94dd211f26841887ee7`.
- `sessionruntime/sessionruntime.go`: byte length `148071` -> `148337`; SHA-256 `sha256:4f5346d4a09acf86b1676b6e4fda5d321a19cf9723ccfc91ddb96e36e89e5210` -> `sha256:b4c0aebfa53d41780bfbcdb6864c7448f67c63c669c2aee10e6fe7019237f611`.

These changed production bytes make Manager diagnostics optional at startup, retain portable stores on unsupported representations without allocating fixed private diagnostic resources, and require initialized qualified owner/history state before private event operations. Supported qualified representations retain the existing allocation and terminal lifecycle. This source-pin successor records those bytes only; it adds no capability or authority.

## Independent reviewer checklist

- [ ] Confirm the predecessor manifest, predecessor review, and every earlier source-pin artifact remain byte-identical to their committed versions.
- [ ] Confirm both manifests contain the same 128 paths in the same order.
- [ ] Confirm only the two entries named above differ.
- [ ] Independently recompute each changed file's byte length and SHA-256 value.
- [ ] Independently recompute the aggregate digest using the production verifier encoding.
- [ ] Independently recompute the successor manifest SHA-256 and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream final-issuance guards.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It does not grant independent acceptance, capability, or authority, and does not establish binary identity, dependency closure, provider authentication, runtime behavior, production authority, Census execution, or qualification beyond the exact local source-byte checks described above.
