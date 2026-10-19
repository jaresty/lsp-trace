# ADR 0011 synthetic source pin successor review candidate

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.c6943ba2ad2bbe50.manifest.json`
- Predecessor manifest SHA-256: `c6943ba2ad2bbe5056747fef228594cc729d2da3d33e25305156cdd54b43d913`
- Successor manifest: `adr0011-synthetic-source-pin.c8779c7bab2b8d0f.manifest.json`
- Successor manifest SHA-256: `c8779c7bab2b8d0f48e9f5c8d5bf94f67aa933f138a660ca5f5786beae454370`
- Successor aggregate digest: `sha256:7d1ed75ed7472521a64240c281e9d1465b352914454ba86ebc00d710e744572e`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files

## Author-prepared reproducibility record

The successor was derived with the repository verifier's canonical `ADR0011_SYNTHETIC_SOURCE_SET_V1` length-prefixed aggregate encoding. The predecessor manifest was parsed without modification; all 128 ordered paths were retained exactly; current bytes were hashed for every selected path; compact sorted-key JSON was emitted without a trailing newline; and the manifest filename was derived from the first 16 hexadecimal characters of its SHA-256.

Exactly two byte-derived entries differ from the predecessor:

- `sessionruntime/owned_method_pair.go`: byte length `21263` -> `21434`; SHA-256 `sha256:8685b78ac7883263c7279ae68a543bb365ab7de0ca8610955eb90236ba5b70b8` -> `sha256:5d10a0ebfcc3c930f32784b105cc57a651ef35ddde809baa84f20eee28366fd9`.
- `sessionruntime/sessionruntime.go`: byte length `144899` -> `145871`; SHA-256 `sha256:0745c83c4d9e50a42ce5713c4298e4ba4e6681f8f73fc2960a049ab2f274bc0e` -> `sha256:0a50b65e2c7213157dd7b36eb053be6463f3ab5b9ccfb9c943ee7af13afffe36`.

The changed production bytes apply the existing internal `RoundTripRequest.MaxMessages` independently to decoded notification and decoded control/response classes while preserving aggregate decoded-frame byte and work accounting, preserve terminal allocation-refusal control flow without aggregate-message coupling, record the message-ceiling observation at the actual class overflow, and require private owned-method-pair validation to prove both class bounds. No selected path was added, removed, or reordered; the denominator remains 128.

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
