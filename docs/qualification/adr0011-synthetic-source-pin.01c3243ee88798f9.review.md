# ADR0011 synthetic source pin review candidate — 01c3243ee88798f9

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED

## Candidate

- Predecessor manifest: `adr0011-synthetic-source-pin.ef5d37d442b0e985.manifest.json`
- Predecessor manifest SHA-256: `ef5d37d442b0e985fc2d22c9a4e40496b7bac08cb83612762060ade8307a66b2`
- Predecessor review: `adr0011-synthetic-source-pin.ef5d37d442b0e985.review.md`
- Predecessor review SHA-256: `d944fd2abd170e60598bf6e2ec11924c845f0de8ff02b32d46e51be7bd0f5b9b`
- Successor manifest: `adr0011-synthetic-source-pin.01c3243ee88798f9.manifest.json`
- Successor manifest SHA-256: `01c3243ee88798f9b25c39c474ed8c9a5ff730853b0e23f16aaa1427725c2e11`
- Successor aggregate digest: `sha256:7644bef3db0e7a523935ea8ac1ff546653f9c7baf9bffbc4e20bff6a7a5bcf28`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 128 files
- Current selected source bytes: 1173938

## Predecessor classification

The committed `ef5d37d442b0e985` manifest/review pair and every earlier source-pin artifact remain immutable historical source-byte evidence. This additive candidate supersedes none of them and does not alter the selected reviewed pin.

## Author-prepared reproducibility record

The successor reuses the predecessor's exact 128-path order. Each entry was recomputed from current regular source bytes. The aggregate uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed per path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. Canonical compact sorted-key JSON includes one trailing newline; the filename is the first 16 hexadecimal characters of the manifest SHA-256.

Exactly 1 selected byte-derived entry differs from the predecessor:

- `sessionruntime/sessionruntime.go`: byte length `152098` -> `152040`; SHA-256 `sha256:dc4ad9852b4b2cc6cce37ff7986861775634021b0af8bdf7b8527d7b109477a6` -> `sha256:5f47bcb97e402c7562ba953c4407ec811347e24263552875b4f16836031cf866`.

The changed production bytes remove only the redundant post-retire algebra poison observation so confirmed owned-transport reap knowledge remains available to exact Restart reconciliation. The causal regression update is in `internal/transientstructural/target_resource_failure_test.go`, which is intentionally outside this production-only selected path set because `_test.go` files are excluded. No selected path was added, removed, or reordered; the denominator remains 128.

## Independent reviewer checklist

- [ ] Confirm all predecessor source-pin artifacts remain byte-identical to their committed versions.
- [ ] Independently compare all 128 ordered paths with the predecessor.
- [ ] Independently recompute every entry byte length and SHA-256.
- [ ] Independently recompute the aggregate digest, manifest hash, and filename.
- [ ] Review the Manager lifecycle correction and causal positive/negative controls separately.
- [ ] Select this successor only through a later independent review; this candidate is not selected here.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, or qualification.
