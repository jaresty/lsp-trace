# ADR0011 synthetic source pin review — 71f9d77c4860911e

**Verdict: REPIN_VERIFY_GO — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `3753e10ef2cfa0a8906571b303f13cb106ce1cbf`
- Parent: `3d76ec235dc260f9be525b5b2000eb0ab34f01d7`
- Tree: `48d48164f480e230a751c8bf0e4f5f9eb5b82a3b`
- Parent tree: `ed11420937d0f83610d2c69ebf0be0a038d64051`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.71f9d77c4860911e.manifest.json`
- Manifest blob: `f8bebd2374918e83347c6c2d4ab5b77216c2a7f4`
- Manifest SHA-256: `71f9d77c4860911e6e9fcf7e413573d6e665e2df42839536d7653039bfa3870c`
- Manifest bytes: `26,726`
- Aggregate: `sha256:0629c4a52ba0d4185d907c6c7b9f76917fc41ef659212bad08614370b3511ce9`

## Reproduction

Independent canonical-policy reconstruction reproduced the manifest byte-for-byte: compact recursively sorted-key JSON followed by one LF. The filename prefix, ordered unique selection, all 168 per-file lengths and SHA-256 values, and aggregate matched repository bytes. The selection contains 1,723,026 bytes, zero `_test.go` paths, and authority 0.

Limits passed: 168/256 paths, 1,723,026/16,777,216 total bytes, and largest file 154,095/2,097,152 bytes.

Verification:

```text
focused source-pin policy tests → 7 passed
internal/censusacquisition      → 157 passed
internal/censussourceplan       → 23 passed
cmd/lsp-trace-mcp               → 698 passed
git diff --check                → clean
```

The predecessor-bound selectors remain intentionally stale and fail closed until a separate selector update.

## Lineage

Against immediate reviewed source-pin predecessor `fc825861052f8f4fc2bed9faf8fecfa3d9bcf9293ed6dd125c20b6b2c1cdc652` (166 paths, 1,686,174 bytes, aggregate `sha256:12436ecd7d4a9bd7b079c7c8fe1a7e59309b82fd6f1a989b7258f5ec7f3461b5`): 162 unchanged, four changed, two added, zero removed. Its paired review SHA-256 is `952dbeac3ef32ffe9f5eb3e93bf7528d99f97629d9cfb6024acb215237539487`.

Added:
- `cmd/lsp-trace-mcp/census_discovery_terminal.go`
- `internal/censusacquisition/core.go`

Changed:
- `cmd/lsp-trace-mcp/census_completion.go`
- `cmd/lsp-trace-mcp/census_runtime.go`
- `internal/censusacquisition/discovery.go`
- `internal/adr0011acquisition/synthetic_source_pin.go`

The manifest commit adds only the manifest, tracked mode `100644`; HEAD and worktree bytes agree. The reviewed worktree was otherwise clean apart from this untracked review document.

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

This establishes exact repository-visible source selection, Git custody, source bytes, framing, and bounded test results only—not build or binary identity, dependency closure, runtime completeness, provider authentication, deployment fitness, feature identity, acceptance, or production admission.
