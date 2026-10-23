# ADR0011 synthetic source pin review — fc825861052f8f4f

**Verdict: REPIN_VERIFY_GO — independently reproduced within source-pin boundary; authority 0.**

## Identity

- HEAD: `92adbcc9ba7a48a99aef6c06e2d6a3cdf57314f9`
- Parent: `4053965131934c731fa1faa2b8431501ed4194dd`
- Tree: `28a889c6587388b67ed80e26cd6444588e5f4da1`
- Parent tree: `c437c277352c8c6e2625a81d29a0e760652e2ab3`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.fc825861052f8f4f.manifest.json`
- Manifest blob: `afc40585f56f77d86d30e2e1d6b262ed3a5fa187`
- Manifest SHA-256: `fc825861052f8f4fc2bed9faf8fecfa3d9bcf9293ed6dd125c20b6b2c1cdc652`
- Manifest bytes: `26,417`
- Aggregate: `sha256:12436ecd7d4a9bd7b079c7c8fe1a7e59309b82fd6f1a989b7258f5ec7f3461b5`

## Reproduction

Independent canonical-policy reconstruction reproduced the manifest byte-for-byte: recursively sorted-key compact JSON followed by one LF. The filename prefix, ordered unique selection, every per-file byte length and SHA-256, and the aggregate matched current repository bytes. The selection contains 166 paths and 1,686,174 bytes, with zero `_test.go` paths and authority 0.

The aggregate framing independently reproduced as:

```text
SHA256("ADR0011_SYNTHETIC_SOURCE_SET_V1\0" ||
  each ordered [BE64(path-byte-length) || path bytes ||
                BE64(file-byte-length) || raw file SHA-256])
= sha256:12436ecd7d4a9bd7b079c7c8fe1a7e59309b82fd6f1a989b7258f5ec7f3461b5
```

Limits passed: 166/256 paths, 1,686,174/16,777,216 total bytes, and largest file 154,095/2,097,152 bytes.

Verification:

```text
focused source-pin policy tests → 7 passed
focused supplier/runtime tests  → 18 passed
internal/censusacquisition      → 134 passed
internal/censussourceplan       → 23 passed
cmd/lsp-trace-mcp               → 660 passed
narrow supplier race            → 18 passed
git diff --check                → clean
```

The predecessor-bound reviewed and producer selectors remain intentionally stale and fail closed until a separate selector update.

## Lineage

Against immediate reviewed source-pin predecessor `a91476ac108356703fbd8468a6eb9063277844967da0c62ae78dd8595daa7114` (166 paths, 1,684,501 bytes, aggregate `sha256:20fc2873e8d0452b40251f9dc9a4361058465d6f6125a329af79da59354ed915`): 165 unchanged, one changed, zero added, zero removed. Its paired review SHA-256 is `b516dd61c778411d584a6e49382f0d0d5aa6020fac5ef3ead9b4c6b729230860`.

- Changed `cmd/lsp-trace-mcp/census_runtime.go`: 20,744 bytes, `sha256:995c5053b721ab95458831b205087260886403836cbcb374f3ffa0140a7bcb15` → 22,417 bytes, `sha256:99e4c91a23110775a34ac39374f879e988889da6a08be6a984a4dae6a5295a02`.

The manifest commit adds only the manifest. It is tracked mode `100644`; its HEAD and worktree bytes agree. The reviewed worktree was otherwise clean apart from this untracked review document.

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
