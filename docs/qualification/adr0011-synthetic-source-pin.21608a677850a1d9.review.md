# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.4a18f3cb6175acc9.manifest.json`
- Predecessor manifest SHA-256: `4a18f3cb6175acc998f325b1150bd1a7f695fe918208b94335c3d14ee88bf857`
- Predecessor review: `adr0011-synthetic-source-pin.4a18f3cb6175acc9.review.md`
- Predecessor review SHA-256: `295b44ae121f6ade5d82a6642b516458cedc5f05efb544e2fcc984cdaa080569`
- Successor manifest: `adr0011-synthetic-source-pin.21608a677850a1d9.manifest.json`
- Successor manifest SHA-256: `21608a677850a1d95302aa30d66cff1e78905d8e253da81b81704016ea325f50`
- Successor aggregate digest: `sha256:e52841839a7c6149ca53455df15e42773c0fe307fd6935ffe15aab5c88736318`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1169821

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; all 128 ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly 1 byte-derived entry differs from the predecessor:

- `sessionruntime/sessionruntime.go`: byte length `146033` -> `148071`; SHA-256 `sha256:07c0fa75857034bf7fd4110e45828cae7c31c51f982bedbe997138d8b61243ad` -> `sha256:4f5346d4a09acf86b1676b6e4fda5d321a19cf9723ccfc91ddb96e36e89e5210`.

These changed production bytes are the committed private provider-probe v6 Manager notification-policy and scalar transaction-observation implementation. No selected path was added, removed, or reordered; the denominator remains 128.

## Independent reviewer checklist

- [ ] Confirm the predecessor manifest, predecessor review, and every earlier source-pin artifact remain byte-identical to their committed versions.
- [ ] Confirm both manifests contain the same 128 paths in the same order.
- [ ] Confirm only the entries named above differ.
- [ ] Independently recompute every changed file's byte length and SHA-256 value.
- [ ] Independently recompute the aggregate digest using the production verifier encoding.
- [ ] Independently recompute the successor manifest SHA-256 and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream final-issuance guards.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It does not grant independent acceptance and does not establish binary identity, dependency closure, provider authentication, runtime behavior, production authority, Census execution, or qualification beyond the exact local source-byte checks described above.
