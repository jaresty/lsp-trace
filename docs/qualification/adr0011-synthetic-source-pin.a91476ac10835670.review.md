# ADR0011 synthetic source pin review — a91476ac10835670

**Verdict: REPIN_VERIFY_GO — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `915b022be28d197baa0005d31e8607e476580f10`
- Parent: `65bec46d9932eb732978e0f6586905e9b2a7e5f3`
- Tree: `ffb72a9149d8c9940175690cb61fc72b03c7f91c`
- Parent tree: `faa59e800ed668bc08c9b313948bdebf33db842e`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.a91476ac10835670.manifest.json`
- Manifest blob: `3642228a828bf83b9775e6f76375b9228d20e3b5`
- Manifest SHA-256: `a91476ac108356703fbd8468a6eb9063277844967da0c62ae78dd8595daa7114`
- Manifest bytes: `26,417`
- Aggregate: `sha256:20fc2873e8d0452b40251f9dc9a4361058465d6f6125a329af79da59354ed915`

## Reproduction

Independent canonical-policy reconstruction reproduced the manifest bytes exactly: compact recursively sorted-key JSON followed by one LF. The filename prefix, ordered unique selection, every per-file byte length and SHA-256, and the aggregate all matched current repository bytes. The selection contains 166 paths and 1,684,501 bytes; all selected leaves are regular non-symlink files. The five expected production deltas appear exactly once and no `_test.go` path is selected.

The aggregate was independently reproduced as:

```text
SHA256("ADR0011_SYNTHETIC_SOURCE_SET_V1\0" ||
  each ordered [BE64(path-byte-length) || path bytes ||
                BE64(file-byte-length) || raw file SHA-256])
= sha256:20fc2873e8d0452b40251f9dc9a4361058465d6f6125a329af79da59354ed915
```

Limits passed: 166/256 paths, 1,684,501/16,777,216 total bytes, and largest file 154,095/2,097,152 bytes.

Focused verification:

```text
go test ./internal/adr0011acquisition -run '^TestSyntheticSourcePin($|CensusSourcePlanProductionSelection$|AdditionalProductionPaths$|CeilingsUnchanged$)' -count=1
→ 7 passed

go test ./internal/censussourceplan -count=1
→ 23 passed

go test ./cmd/lsp-trace-mcp -count=1
→ 642 passed
```

The predecessor-bound reviewed and producer selectors remain intentionally stale and fail closed until a separate selector update.

## Lineage

Against immediate reviewed predecessor `b4b72af800bfb0759a52acd1f22c1facd1fcc1be4acc8ad5de36d9b32f733784` (165 paths, 1,672,527 bytes, aggregate `sha256:3dde225912bce1d424e12baab4f8fd4ac68c8d1a0b9880dbfe2a0c3048680a1b`): 161 unchanged, four changed, one added, zero removed. Its paired review SHA-256 is `179b99add5036047982e51a5b942479cc45df58a0d2ebacb1a2ea5ed7f1a346e`.

- Added `cmd/lsp-trace-mcp/census_executor.go`.
- Changed `cmd/lsp-trace-mcp/census_completion.go`.
- Changed `cmd/lsp-trace-mcp/census_runtime.go`.
- Changed `internal/censussourceplan/plan.go`.
- Changed `internal/adr0011acquisition/synthetic_source_pin.go`.

The manifest commit adds only the manifest. The manifest is tracked mode `100644`; its HEAD and worktree bytes agree. `git diff --check` passed, and the reviewed worktree was otherwise clean apart from this untracked review document.

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
