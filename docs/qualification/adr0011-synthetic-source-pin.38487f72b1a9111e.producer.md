# ADR0011 synthetic source-pin successor producer record — 38487f72b1a9111e

Status: PRODUCER CANDIDATE / UNREVIEWED / UNSELECTED

## Target and immutable lineage

- Exact source HEAD: `d62659dd2e803e3965bd7e3f53138498904424fb`
- Predecessor manifest: `docs/qualification/adr0011-synthetic-source-pin.f428950e7f2081ad.manifest.json`
- Predecessor manifest SHA-256: `f428950e7f2081ad6c86c4c80ce118418067a7800cc586eb57c2a0a1770b6d96`
- Predecessor review: `docs/qualification/adr0011-synthetic-source-pin.f428950e7f2081ad.review.md`
- Predecessor review SHA-256: `418e37f0355936cb944c98750a959f426a620e59ad0840508b37b44516453936`
- Predecessor manifest and review remain byte-identical; neither is rewritten by this producer.

## Candidate identity

- Manifest: `docs/qualification/adr0011-synthetic-source-pin.38487f72b1a9111e.manifest.json`
- Manifest SHA-256: `38487f72b1a9111edb11df7984175603271572e1ffe2d131f93ff9c4e12784c9`
- Manifest bytes: 28,124
- Aggregate: `sha256:7f6f268fb6850068fc7e4f7281464b61bdfa7d9a8fba6bc2c3ac175c82b4e19e`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Ordered paths: 177 (derived and asserted; expected count was 177)
- Selected source bytes: 1,782,318
- Canonical segments: 128 direct directory Go paths + 45 declaration-ordered additional Go paths + 4 declaration-ordered schema paths.

## Exact predecessor delta

- Unchanged paths: 162
- Added paths: 7
- Changed paths: 8
- Removed paths: 0

Added paths, lexical for reporting only:

1. `cmd/lsp-trace-mcp/census_source_continuation_store.go`
2. `cmd/lsp-trace/census_native_catalog.go`
3. `internal/censussourceassembly/run.go`
4. `internal/censussourceassembly/stores.go`
5. `internal/censussourceassembly/terminal.go`
6. `internal/censussourcedriver/driver.go`
7. `internal/censussourcedriver/step.go`

Changed paths, lexical for reporting only:

1. `cmd/lsp-trace-mcp/census_completion.go`
2. `cmd/lsp-trace-mcp/census_discovery_resume.go`
3. `cmd/lsp-trace-mcp/census_discovery_terminal.go`
4. `cmd/lsp-trace-mcp/census_runtime.go`
5. `cmd/lsp-trace/census_core.go`
6. `cmd/lsp-trace/census_runner.go`
7. `internal/adr0011acquisition/synthetic_source_pin.go`
8. `internal/censussourcecontinuation/coordinator.go`

## Independent producer implementation

The one-off generator was independently authored for this production run and does not import or call production verifier helpers. It hard-codes the independently transcribed twelve authoritative directories, the `syntheticSourceAdditionalGoPaths` declaration order, and the `syntheticSourceAdditionalSchemaPaths` declaration order. It rejects symlink ancestors/leaves and non-regular files, derives direct non-test `.go` membership lexically, asserts exactly 177 unique paths, records exact byte lengths and `sha256:<hex>` digests, and computes the aggregate over `ADR0011_SYNTHETIC_SOURCE_SET_V1\0` followed per entry by uint64-be path length, path bytes, uint64-be file length, and raw 32-byte digest.

- Generator implementation: `/private/tmp/adr0011-v8-source-pin-generator.py`
- Generator SHA-256: `893585551cdf8b2c731c972ca6bd32152ea2823b2d11266e80d899af4e1100fa`
- Generator command: `python3 /private/tmp/adr0011-v8-source-pin-generator.py /private/tmp/adr0011-v8-local-main-integration /private/tmp/adr0011-v8-local-main-integration/docs/qualification`
- Serialization: canonical compact sorted-key JSON plus one trailing LF.
- Filename binding: first 16 hexadecimal characters of the complete manifest SHA-256.

## Verification disposition

Observed producer gates at exact source HEAD:

- Independent recomputation: `ADR0011_INDEPENDENT_RECOMPUTE_PASS`, confirming 177 paths, 1,782,318 selected bytes, manifest SHA-256 `38487f72b1a9111edb11df7984175603271572e1ffe2d131f93ff9c4e12784c9`, and aggregate `sha256:7f6f268fb6850068fc7e4f7281464b61bdfa7d9a8fba6bc2c3ac175c82b4e19e`.
- Independent declaration-order extraction: `ADR0011_DECLARATION_ORDER_PASS additional_go=45 schemas=4`.
- Focused non-selector source-pin tests: 7 passed.
- Production verifier candidate test: failed with `producer candidate: synthetic implementation source pin mismatch` because `TestSyntheticSourcePinProducerCandidate` explicitly opens the predecessor `f428950e7f2081ad` manifest; the mechanism does not discover additive candidates.
- Reviewed selector test: failed with the same expected current-source mismatch because it remains explicitly bound to predecessor manifest/review bytes.
- No production test, reviewed selector, or review artifact was modified to force candidate admission.
- Predecessor-byte preservation and `git diff --check` are final producer gates recorded by the producing session.

## Authority boundary

```text
authority=0
accepted=false
review=PENDING
selector=PENDING
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
production_admission=REFUSED_MISSING_EVIDENCE
```

This record is producer-authored source-byte provenance only. It is not an independent review, acceptance, selector update, production admission, binary identity, dependency closure, provider authentication, runtime qualification, semantic correctness claim, or authority grant. No review artifact is created and no selector is updated.
