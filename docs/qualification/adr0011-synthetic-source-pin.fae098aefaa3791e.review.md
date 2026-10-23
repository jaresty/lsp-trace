# ADR0011 synthetic source pin review — fae098aefaa3791e

**Verdict: ACCEPT — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `98252b800e28e493610a607c898beed8d8555753`
- Parent: `98914cf0ac063d3342e11539085c05f10e9ee327`
- Tree: `f605c645e0a497513dc00ab717646769ebc9d05c`
- Parent tree: `4abb7ba1d5983ad38af31c6da648ccab9e8a1b68`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.fae098aefaa3791e.manifest.json`
- Manifest SHA-256: `fae098aefaa3791ea520d99ff6955d29ca2b6d56fcec88d7c400fc2b7ae11feb`
- Manifest bytes: `26,119`
- Aggregate: `sha256:40a50dab6fd883ed8ee8fbed982dfbbd0762f0800b4e8e79dd172f2016c89390`

## Independent reproduction

The authoritative selection was independently enumerated from `internal/adr0011acquisition/synthetic_source_pin.go`: 128 sorted directory Go files, 32 ordered additional Go files, and four ordered schemas. Exact path order, coverage, byte lengths, every per-file SHA-256, aggregate framing, and compact canonical JSON plus one final LF reproduced byte-for-byte.

- Selection: `164` unique paths, `1,662,549` bytes
- Count limit: `164 / 256`
- Total-byte limit: `1,662,549 / 16,777,216`
- Largest file: `154,095 / 2,097,152`
- No duplicates, omissions, extras, test files, symlink leaves, or symlink ancestors
- All selected paths are tracked mode `100644`; index blobs equal filesystem bytes

## Lineage

Against immutable predecessor `d894fac416e29a01…`: 162 unchanged, two changed, zero added, zero removed.

- `cmd/lsp-trace/private_provider_probe.go`: `67,125` bytes, `037ae512…` → `68,764` bytes, `91e5c404…`
- `sessionruntime/provider_message_composition.go`: `4,661` bytes, `bbff1d42…` → `6,511` bytes, `41454311…`

The candidate commit adds only the manifest.

## Checks

Independent enumeration, hashing, aggregate framing, canonicalization, focused Go tests, `git diff --check`, index/filesystem equality, and clean status passed.

## Authority ceiling

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
reviews=NOT_REQUESTED
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
```

This review establishes exact source-byte consistency only. It establishes no build, runtime, provider authentication, production behavior, feature identity, completeness, deployment, or production admission.
