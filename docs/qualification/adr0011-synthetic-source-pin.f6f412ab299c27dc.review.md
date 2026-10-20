# ADR0011 synthetic source pin review candidate — f6f412ab299c27dc

Status: AUTHOR-PREPARED REVIEW CANDIDATE; INDEPENDENT ACCEPTANCE NOT CLAIMED; UNSELECTED

## Candidate

- Predecessor selected manifest: `adr0011-synthetic-source-pin.01c3243ee88798f9.manifest.json`
- Predecessor manifest SHA-256: `01c3243ee88798f9b25c39c474ed8c9a5ff730853b0e23f16aaa1427725c2e11`
- Predecessor review: `adr0011-synthetic-source-pin.01c3243ee88798f9.review.md`
- Successor manifest: `adr0011-synthetic-source-pin.f6f412ab299c27dc.manifest.json`
- Successor manifest SHA-256: `f6f412ab299c27dc38803ebcc12df2106fb39f4d49a385bb42ea820ff426ff37`
- Successor aggregate digest: `sha256:8dedd90198d0cec64706e1c6333bc067f11224f6f9d6b5257b1a4ee58ccb1354`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered selection: 133 files
- Current selected source bytes: 1254358

## Predecessor classification

The committed selected manifest/review pair and every earlier source-pin artifact remain immutable historical source-byte evidence. This additive candidate supersedes none of them and does not alter `internal/adr0011acquisition/synthetic_source_pin_review_test.go`; it is not selected.

## Author-prepared reproducibility record

The successor preserves the predecessor's exact 128-path order and appends five production paths required by the private CLI chunking route:

- `cmd/lsp-trace/census_assembly.go`
- `cmd/lsp-trace/census_core.go`
- `cmd/lsp-trace/census_runner.go`
- `cmd/lsp-trace/census_chunking.go`
- `internal/censuschunking/chunking.go`

Each entry was recomputed from current regular source bytes. The aggregate uses `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed per path by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and raw SHA-256 digest. Canonical compact sorted-key JSON includes one trailing newline; the filename is the first 16 hexadecimal characters of the manifest SHA-256. Test files are intentionally excluded.

## Independent reviewer checklist

- [ ] Confirm all predecessor source-pin artifacts remain byte-identical to committed versions.
- [ ] Independently verify the preserved 128-path order and five appended production paths.
- [ ] Independently recompute every entry byte length and SHA-256.
- [ ] Independently recompute the aggregate digest, manifest hash, and filename.
- [ ] Review compatibility-first routing, fail-closed chunked translation, Manager lease custody, terminal privacy, and post-release emission separately.
- [ ] Select this successor only through a later independent review; this candidate is not selected here.

## Authority boundary

This author-prepared candidate records source-byte reproducibility only. It grants no independent acceptance, capability, final-issuance authority, production authority, Census execution, provider authentication, binary identity, dependency closure, runtime behavior, public schema authority, or qualification.
