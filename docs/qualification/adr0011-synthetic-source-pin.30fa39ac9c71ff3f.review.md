# ADR0011 synthetic source pin review — 30fa39ac9c71ff3f

**Verdict: ACCEPT — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `d7a947db7f45dda9700f8a733b4fca0ef659b79b`
- Parent: `271e404c3460bc4e8dd3dbaaac5aad4dbc73c4a2`
- Tree: `1ec15eb1ab49032fdc26f945733cc404b7a87d77`
- Parent tree: `58c912cf99feac8b99c78074ed49583272f45d65`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.30fa39ac9c71ff3f.manifest.json`
- Manifest SHA-256: `30fa39ac9c71ff3f0769f6772fd29f6e0ca20f8d0bd573597747682e27768de0`
- Manifest bytes: `26,119`
- Aggregate: `sha256:99bb53df0b5cb23b5a4304c7ca7f349ea3880cb6cca4d128bb8d70ddd73254ce`

## Independent reproduction

The authoritative selection was independently enumerated from `internal/adr0011acquisition/synthetic_source_pin.go`: 128 sorted directory Go files, 32 ordered additional Go files, and four ordered schemas. Exact order, coverage, lengths, per-file SHA-256 values, aggregate framing, and compact canonical JSON plus final LF reproduced byte-for-byte.

- Selection: `164` unique paths, `1,663,812` bytes
- Limits: `164 / 256` paths; `1,663,812 / 16,777,216` total bytes
- Largest: `sessionruntime/sessionruntime.go`, `154,095 / 2,097,152` bytes
- Missing, extra, and duplicate paths: zero
- All leaves mode `100644`; ancestors are trees; no test paths, unsafe lexical paths, or symlink objects
- Index, worktree, and tree bytes agree

## Lineage

Against immutable predecessor `fae098aefaa3791e…`: 160 unchanged, four changed, zero added, zero removed.

- `internal/lspwire/read_frame.go`: `9,741`, `c189b521…` → `9,861`, `df8a3e20…`
- `internal/lspwire/wire.go`: `18,412`, `90640b31…` → `18,939`, `19856930…`
- `sessionruntime/provider_message_composition.go`: `6,511`, `41454311…` → `7,004`, `c7f41494…`
- `cmd/lsp-trace/private_provider_probe.go`: `68,764`, `91e5c404…` → `68,887`, `331f8367…`

The candidate commit adds only the manifest. Existing selectors intentionally remain on the predecessor until this review is committed.

## Checks

Independent enumeration, hashing, framing, canonicalization, limit and ceiling guards, index/worktree/tree equality, and `git diff --check` passed.

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

This establishes exact source-byte consistency only. It establishes no build identity, runtime behavior, provider authentication, deployment, production behavior, completeness, feature identity, or production admission.
