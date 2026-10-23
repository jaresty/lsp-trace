# ADR0011 synthetic source pin review — f5c4f40144023f97

**Verdict: ACCEPT — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `d369a46c9aa0e4c5bc565517f22556cc384b431d`
- Parent: `aebbaa037e08e5d9cf72113f0e38e5003fdfebe7`
- Tree: `b6c5e9795843782f26458915597b57ee09f5b183`
- Parent tree: `83a697fdc32c3bed61353902e52f3c14e558af40`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.f5c4f40144023f97.manifest.json`
- Manifest blob: `1753d48b97af93665664dd0ac06dfd19fea3916e`
- Manifest SHA-256: `f5c4f40144023f97d566543b927611de3f9d9c51621a7f018657689bd0501611`
- Manifest bytes: `26,119`
- Aggregate: `sha256:ffc65dafa92831dfc6f8ccd13b49522722fa4299c2022270ce0ccdcc3cbb04f9`

## Reproduction

The authoritative selection independently reproduced exactly: 128 sorted directory Go files, 32 ordered additional Go files, and four ordered schemas; 164 unique paths and 1,666,096 bytes. Canonical compact JSON plus final LF, order, coverage, per-file sizes and hashes all matched. Missing, extra, duplicate, test, unsafe, and symlink paths were zero. All selected leaves are tracked mode `100644`; tree, index, and worktree bytes agree.

Limits passed: 164/256 paths, 1,666,096/16,777,216 total bytes, and largest file 154,095/2,097,152 bytes.

## Lineage

Against predecessor `30fa39ac9c71ff3f…`: 162 unchanged, two changed, zero added, zero removed.

- `cmd/lsp-trace-mcp/census_completion.go`: 11,802 bytes, `212fdf90…` → 12,444 bytes, `85f4dce8…`
- `cmd/lsp-trace-mcp/census_runtime.go`: 17,770 bytes, `baff565f…` → 19,412 bytes, `2bffa5a6…`

The candidate commit adds only the manifest. `git diff --check` passed and the reviewed worktree was clean.

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

This establishes exact source-byte consistency only—not build identity, runtime behavior, provider authentication, deployment, production behavior, completeness, feature identity, or production admission.
