# ADR0011 synthetic source pin review — 38487f72b1a9111e

**Verdict: REPIN_VERIFY_GO — independently reproduced within the source-pin boundary; authority 0.**

## Identity

- Source HEAD: `d62659dd2e803e3965bd7e3f53138498904424fb`
- Manifest: `docs/qualification/adr0011-synthetic-source-pin.38487f72b1a9111e.manifest.json`
- Manifest SHA-256: `38487f72b1a9111edb11df7984175603271572e1ffe2d131f93ff9c4e12784c9`
- Manifest bytes: `28124`
- Aggregate: `sha256:7f6f268fb6850068fc7e4f7281464b61bdfa7d9a8fba6bc2c3ac175c82b4e19e`
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Selection: 177 paths, 1,782,318 bytes
- Canonical segments: `128 + 45 + 4`

## Predecessor identity and preservation

- Predecessor manifest: `docs/qualification/adr0011-synthetic-source-pin.f428950e7f2081ad.manifest.json`
- Predecessor manifest SHA-256: `f428950e7f2081ad6c86c4c80ce118418067a7800cc586eb57c2a0a1770b6d96`
- Predecessor manifest bytes: `27047`
- Predecessor review: `docs/qualification/adr0011-synthetic-source-pin.f428950e7f2081ad.review.md`
- Predecessor review SHA-256: `418e37f0355936cb944c98750a959f426a620e59ad0840508b37b44516453936`
- Predecessor review bytes: `4440`

Independent byte comparison against the corresponding committed `HEAD` blobs confirmed that both predecessor artifacts are preserved exactly. They remain immutable lineage evidence; this review neither rewrites nor retroactively strengthens them.

## Independent enumeration and recomputation

The successor was independently enumerated in the implementation's canonical three-segment order:

1. 128 direct regular non-symlink `.go` files from the twelve authoritative directories, excluding `_test.go`, in strictly increasing lexical path order;
2. 45 additional Go paths in the exact declaration order of `syntheticSourceAdditionalGoPaths`;
3. four additional schema paths in the exact declaration order of `syntheticSourceAdditionalSchemaPaths`.

For every selected path, the current regular source bytes were read independently and their exact byte length and SHA-256 were compared with the manifest entry. All 177 entries matched.

The aggregate was independently recomputed as SHA-256 over `ADR0011_SYNTHETIC_SOURCE_SET_V1\x00` followed, for every ordered entry, by uint64-be path length, UTF-8 path bytes, uint64-be source byte length, and the raw 32-byte file digest. This produced `sha256:7f6f268fb6850068fc7e4f7281464b61bdfa7d9a8fba6bc2c3ac175c82b4e19e` over exactly 177 paths and 1,782,318 source bytes.

Canonical compact sorted-key JSON with one trailing newline was independently hashed. It is exactly 28,124 bytes, hashes to `38487f72b1a9111edb11df7984175603271572e1ffe2d131f93ff9c4e12784c9`, and is bound to the filename prefix `38487f72b1a9111e`.

## Exact predecessor delta

Using tuple order **unchanged / added / changed / removed**, the exact delta is `162 / 7 / 8 / 0`.

Added paths, in successor order:

- `internal/censussourcedriver/driver.go`
- `internal/censussourcedriver/step.go`
- `internal/censussourceassembly/stores.go`
- `internal/censussourceassembly/terminal.go`
- `internal/censussourceassembly/run.go`
- `cmd/lsp-trace-mcp/census_source_continuation_store.go`
- `cmd/lsp-trace/census_native_catalog.go`

Changed paths, in successor order:

- `internal/adr0011acquisition/synthetic_source_pin.go`
- `cmd/lsp-trace/census_core.go`
- `cmd/lsp-trace/census_runner.go`
- `cmd/lsp-trace-mcp/census_completion.go`
- `cmd/lsp-trace-mcp/census_discovery_terminal.go`
- `cmd/lsp-trace-mcp/census_runtime.go`
- `cmd/lsp-trace-mcp/census_discovery_resume.go`
- `internal/censussourcecontinuation/coordinator.go`

Removed paths: none.

## Focused verification

At exact source HEAD `d62659dd2e803e3965bd7e3f53138498904424fb`:

- `go test ./internal/adr0011acquisition -run '^(TestSyntheticSourcePin|TestSyntheticSourcePinCensusSourcePlanProductionSelection|TestSyntheticSourcePinAdditionalProductionPaths|TestSyntheticSourcePinCeilingsUnchanged)$' -count=1` passed (`7 passed`);
- `go vet ./internal/adr0011acquisition` passed;
- the broader focused selector-bearing run, `go test ./internal/adr0011acquisition -run 'SyntheticSourcePin' -count=1`, produced the expected selector RED classification: 7 passed and exactly `TestReviewedSyntheticSourcePin` plus `TestSyntheticSourcePinProducerCandidate` failed closed because the reviewed selector and producer-candidate expectation still bind the predecessor.

That selector RED is expected evidence of an unselected successor, not runtime or semantic failure. Selector mutation, commit, publication, and push are outside this review.

## Authority ceiling and explicit exclusions

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
implementationDecision=UNISSUED
runtime_acceptance=NOT_CLAIMED
semantic_acceptance=NOT_CLAIMED
production_admission=REFUSED_MISSING_EVIDENCE
```

`REPIN_VERIFY_GO` accepts only this manifest's bounded source-byte reproducibility, canonical ordering, predecessor delta, and focused verifier evidence at the stated source HEAD. It grants no runtime execution claim, semantic correctness, feature identity, source completeness outside the bounded policy, build or binary identity, dependency closure, provider authentication, implementation acceptance, selector admission, deployment fitness, production behavior, or production acceptance. No runtime, semantic, or production acceptance is claimed.
