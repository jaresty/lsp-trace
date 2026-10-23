# ADR0011 synthetic source pin review — b4b72af800bfb075

**Verdict: REPIN_VERIFY_GO — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `df33e2a13d744b13e41d3a2e912a0de5d17a080b`
- Parent: `af42e15efa24262f092217e1d5915a2832be1659`
- Tree: `4e9cce3a0cd7c8da2236986fc26e2b5c683948aa`
- Parent tree: `9b5b0dec7088886eb297a099d78aaee7127a38ae`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.b4b72af800bfb075.manifest.json`
- Manifest blob: `ff9ac72d85d97afd4ca1609cc0dd6321dcac4291`
- Manifest SHA-256: `b4b72af800bfb0759a52acd1f22c1facd1fcc1be4acc8ad5de36d9b32f733784`
- Manifest bytes: `26,266`
- Aggregate: `sha256:3dde225912bce1d424e12baab4f8fd4ac68c8d1a0b9880dbfe2a0c3048680a1b`

## Reproduction

The authoritative selection independently reproduced exactly: 128 sorted directory Go files, 33 ordered additional Go files, and four ordered schemas; 165 unique paths and 1,672,527 bytes. Canonical compact JSON plus final LF, filename prefix, ordering, coverage, per-file sizes and hashes all matched. Missing, extra, duplicate, test, unsafe, and symlink paths were zero. All selected leaves are stage 0, tracked mode `100644`; implementation HEAD, index, and worktree bytes agree. `internal/censussourceplan/plan.go` appears exactly once and no `_test.go` path is selected.

Limits passed: 165/256 paths, 1,672,527/16,777,216 total bytes, and largest file 154,095/2,097,152 bytes.

Focused verification:

```text
go test ./internal/adr0011acquisition -run '^TestSyntheticSourcePin($|CensusSourcePlanProductionSelection$|AdditionalProductionPaths$|CeilingsUnchanged$)' -count=1 -v
→ 6 passed

go test ./internal/censussourceplan -count=1
→ 14 passed
```

The predecessor-bound `TestReviewedSyntheticSourcePin` and `TestSyntheticSourcePinProducerCandidate` remain intentionally stale and fail closed until a separate selector update.

## Lineage

Against immediate reviewed predecessor `f5c4f40144023f97d566543b927611de3f9d9c51621a7f018657689bd0501611` (164 paths, 1,666,096 bytes, aggregate `sha256:ffc65dafa92831dfc6f8ccd13b49522722fa4299c2022270ce0ccdcc3cbb04f9`): 162 unchanged, two changed, one added, zero removed.

- Added `internal/censussourceplan/plan.go`: 5,372 bytes, `sha256:257e1df459be49ef2b5aa2c904a4ec7f4376eb6f034ab3b83d537095fa2814f8`.
- Changed `cmd/lsp-trace-mcp/census_runtime.go`: 19,412 bytes, `2bffa5a6…` → 20,433 bytes, `43794fec…`.
- Changed `internal/adr0011acquisition/synthetic_source_pin.go`: 8,605 bytes, `3dd9032f…` → 8,643 bytes, `e748e74f…`.

The candidate commit adds only the manifest. The manifest is tracked mode `100644`, its HEAD and worktree bytes agree, `git diff --check` passed, and the reviewed worktree was otherwise clean apart from this untracked review document.

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

This establishes exact repository-visible source selection, Git custody, source bytes, framing, and focused test results only—not build or binary identity, dependency closure, runtime behavior, provider authentication, deployment fitness, feature identity, completeness, acceptance, or production admission.
