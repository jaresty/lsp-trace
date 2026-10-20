# ADR0011 synthetic source pin review candidate — ef5d37d442b0e985

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.76fa35777392d0ed.manifest.json`
- Predecessor manifest SHA-256: `76fa35777392d0ed84237c1b6e532612cd81146b95320498203ffca16c38fc61`
- Predecessor review: `adr0011-synthetic-source-pin.76fa35777392d0ed.review.md`
- Successor manifest: `adr0011-synthetic-source-pin.ef5d37d442b0e985.manifest.json`
- Successor manifest SHA-256: `ef5d37d442b0e985fc2d22c9a4e40496b7bac08cb83612762060ade8307a66b2`
- Successor aggregate digest: `sha256:1e5544ef6c675b21d1a7e88f051f523aa2ff9b4a62becf784960f9576cf2fb88`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1173996

## Predecessor classification

The committed `76fa35777392d0ed` candidate and every earlier manifest/review remain immutable historical source-byte evidence. This candidate supersedes none of them, preserves C07–C12, and does not alter the selected reviewed pin. It records current corrected source bytes for the same established ordered path set.

- Modified paths: `sessionruntime/sessionruntime.go`
- Added paths: none
- Removed paths: none

## Author-prepared reproducibility record

The successor reuses the predecessor's exact 128-path order. Each entry was recomputed from the current regular source bytes. The aggregate uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed per path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. Canonical compact sorted-key JSON includes one trailing newline; the filename is the first 16 hexadecimal characters of the manifest SHA-256.

## Independent reviewer checklist

- [ ] Confirm all predecessor source-pin artifacts remain byte-identical to their committed versions.
- [ ] Independently compare all 128 ordered paths with the predecessor.
- [ ] Independently recompute every entry byte length and SHA-256.
- [ ] Independently recompute the aggregate digest, manifest hash, and filename.
- [ ] Review the Manager lease correction and causal tests separately.
- [ ] Select this successor only through a later independent review; this candidate is not selected here.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, or qualification.
