# ADR0011 synthetic source pin review candidate — 16b4621be6f4b8d1

Status: AUTHOR-PREPARED SCOPE-CORRECTED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED; UNSELECTED

## Candidate

- Predecessor selected manifest: `adr0011-synthetic-source-pin.f6f412ab299c27dc.manifest.json`
- Predecessor manifest SHA-256: `f6f412ab299c27dc38803ebcc12df2106fb39f4d49a385bb42ea820ff426ff37`
- Predecessor review: `adr0011-synthetic-source-pin.f6f412ab299c27dc.review.md`
- Successor manifest: `adr0011-synthetic-source-pin.16b4621be6f4b8d1.manifest.json`
- Successor manifest SHA-256: `16b4621be6f4b8d14623771bd581470ac5234f621898747c1131d97d2e778423`
- Successor aggregate digest: `sha256:864a1d9315d997972d8d1e722ccad5e9bef204dcd70370f5919fa5afaff45a18`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 133 files
- Current selected source bytes: 1255389
- Corrected source HEAD: `76000f8`

## Predecessor classification

The f6 manifest/review pair, its selector commit, and every earlier source-pin artifact remain immutable historical source-byte evidence. The selected f6 manifest is expected to remain executable-red after the verifier correction because its recorded verifier bytes predate that correction. This additive candidate supersedes none of them, does not alter `internal/adr0011acquisition/synthetic_source_pin_review_test.go`, and is not selected.

## Author-prepared reproducibility record

The successor preserves the f6 candidate's exact 133-path order: the historical 128-path directory profile followed by the five exact production paths admitted by the additive verifier profile:

- `cmd/lsp-trace/census_assembly.go`
- `cmd/lsp-trace/census_core.go`
- `cmd/lsp-trace/census_runner.go`
- `cmd/lsp-trace/census_chunking.go`
- `internal/censuschunking/chunking.go`

Every entry was recomputed from regular source bytes at corrected source HEAD `76000f8`; only `internal/adr0011acquisition/synthetic_source_pin.go` differs from f6. The aggregate uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed per path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. Canonical compact sorted-key JSON includes one trailing newline; the filename is the first 16 hexadecimal characters of the manifest SHA-256. Test files are intentionally excluded.

## Independent reviewer checklist

- [ ] Confirm all predecessor source-pin artifacts and the f6 selector remain byte-identical to committed versions.
- [ ] Independently verify the preserved 133-path order and exact five-path suffix.
- [ ] Independently recompute every entry byte length and SHA-256 from corrected source HEAD `76000f8`.
- [ ] Independently recompute the aggregate digest, manifest hash, and filename.
- [ ] Confirm legacy directory-only manifests remain structurally admissible and any partial exact-path extension fails closed.
- [ ] Select this successor only through a later independent review; this candidate is not selected here.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, public schema authority, or qualification.
