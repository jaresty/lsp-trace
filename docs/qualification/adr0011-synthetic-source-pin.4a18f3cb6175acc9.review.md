# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.c8779c7bab2b8d0f.manifest.json`
- Predecessor manifest SHA-256: `c8779c7bab2b8d0f48e9f5c8d5bf94f67aa933f138a660ca5f5786beae454370`
- Successor manifest: `adr0011-synthetic-source-pin.4a18f3cb6175acc9.manifest.json`
- Successor manifest SHA-256: `4a18f3cb6175acc998f325b1150bd1a7f695fe918208b94335c3d14ee88bf857`
- Successor aggregate digest: `sha256:546328b0b08ca05435db2ba1f382ed1bcacab861a6326a9ebb3376aeedfee8aa`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; all 128 ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly two byte-derived entries differ from the predecessor:

- `sessionruntime/method_frame_correspondence.go`: byte length `3834` -> `4033`; SHA-256 `sha256:ffefe67b49952ab05218ef57e89455a960a221e80bfc9fe8ba46ecf5be63627f` -> `sha256:182e3846230e4b47b0ae74493c9f8b45f06908bfffc68083d8a264fc4943bf3c`.
- `sessionruntime/sessionruntime.go`: byte length `145871` -> `146033`; SHA-256 `sha256:0a50b65e2c7213157dd7b36eb053be6463f3ab5b9ccfb9c943ee7af13afffe36` -> `sha256:07c0fa75857034bf7fd4110e45828cae7c31c51f982bedbe997138d8b61243ad`.

The changed production bytes adapt method-frame correspondence to the existing private per-class message proof and document that diagnostic EffectiveMaxMessages remains the configured per-class value when total ResponseMessages spans classes. No selected path was added, removed, or reordered; the denominator remains 128.

## Independent reviewer checklist

- [ ] Confirm the predecessor manifest and every prior source-pin review artifact remain byte-identical to their committed versions.
- [ ] Confirm both manifests contain the same 128 paths in the same order.
- [ ] Confirm only the two entries named above differ.
- [ ] Independently recompute both changed files' byte lengths and SHA-256 values.
- [ ] Independently recompute the aggregate digest using the production verifier encoding.
- [ ] Independently recompute the successor manifest SHA-256 and filename.
- [ ] Run `TestReviewedSyntheticSourcePin` and downstream compatibility guards.
- [ ] Record an independent disposition separately; this candidate is not that disposition.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It does not grant independent acceptance and does not establish binary identity, dependency closure, provider authentication, runtime behavior, production authority, Census execution, or qualification beyond the exact local source-byte checks described above.
