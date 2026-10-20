# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.21608a677850a1d9.manifest.json`
- Predecessor manifest SHA-256: `21608a677850a1d95302aa30d66cff1e78905d8e253da81b81704016ea325f50`
- Predecessor review: `adr0011-synthetic-source-pin.21608a677850a1d9.review.md`
- Predecessor review SHA-256: `d2f414e7d4ac4e25850a64ccf2eeb919a43b1bcd27cc800a5cc036d439bf3fd7`
- Successor manifest: `adr0011-synthetic-source-pin.7de70622758e21aa.manifest.json`
- Successor manifest SHA-256: `7de70622758e21aa3276850914b34d816ac0f6f6ba86b85ca245ec7b2d30bae8`
- Successor aggregate digest: `sha256:385c39c17200bca3b3a07be64d566ce4c390c143e5feed10d2f605bc205ec219`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1169929

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; all 128 ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly 2 byte-derived entries differ from the predecessor:

- `sessionruntime/diagnostic_backing_c15_private.go`: byte length `5285` -> `5339`; SHA-256 `sha256:34db153fd4c9052f57cdb390864ce255b5fbbd7fad42b4c20c04a15c2d0c6e79` -> `sha256:cb17be6b67695f99c44d51472c6549801bdceb6d547ae730b72e77331b653c02`.
- `sessionruntime/diagnostic_history_storage_c15_private.go`: byte length `4827` -> `4881`; SHA-256 `sha256:f39e5e9f7fc83b49452ec42c96100170bc5fb1803066d7b062e84e6dba27b3e6` -> `sha256:fc0afdba8151943071ab9f5691ca192d02c13715bb3b735066bfa3f4e9e44535`.

These changed production bytes are the CI portability repair that aligns fail-closed diagnostic backing and history storage with the manageddiagnostic authoritative platform representation. This source-pin successor records those existing bytes only; it adds no capability or authority.

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
