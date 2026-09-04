# Observation Adaptation claim

Status: CLAIMED
Baseline: d9f2c890d0bf100a1b9ce471551a16a1054668f8
Frame: Observation Adaptation

## Goal

Provide a provider-neutral, fail-closed adaptation boundary from strictly validated provider-reported observation envelopes to deterministic graph-v4 normalized relations. Preserve provider/version/protocol and adapter identity, original and virtual document custody, explicit coverage and failure distinctions, every contributing observation identity, and all focused-frame non-entailments. Go treats Ember/Glimmer source and provider payload semantics as opaque and never parses framework syntax.

## Focused claims read before mutation

- `b05-qualification.claim.md`
- `document-custody.claim.md`
- `incoming-composition.claim.md`
- `mcp-contract.claim.md`
- `normalized-relation-model.claim.md`
- `observation-semantics.claim.md`
- `provider-qualification.claim.md`
- `provider-runtime.claim.md`
- `slice-composition.claim.md`

## Exact ownership

New files only:

- `internal/observationadapter/adapter.go`
  - provider response/envelope identity types
  - strict adapter input validation
  - deterministic provider-reported observation identity
  - original/virtual anchor and document custody projection
  - explicit coverage and failure projection
  - deterministic many-observation graph-v4 derivation
- `internal/observationadapter/adapter_test.go`
  - assertion-specific adaptation and non-entailment tests
- `internal/integratedconformance/harness_test.go`
  - only the `ASSERT_PACKAGE_OWNERSHIP_ONLY` allowlist entry for this claim and package

Explicitly unclaimed and read-only: MCP bootstrap, production collector wiring, provider runtime/process execution, incoming/slice composition, graph-v4 core vocabulary/schema, observation-semantic core, document-custody core, provider qualification, B05 fixtures, and all NAIS files.

## Behavioral dimensions

1. Validate exact supported provider identity, provider version, protocol identity/version, adapter identity/version, and semantic schema version; reject unknown or mismatched values.
2. Emit only `PROVIDER_REPORTED` observations from provider-reported fields; never promote transport, parsing, callback passage, state update, or template syntax into stronger semantic claims.
3. Derive observation IDs deterministically from canonical semantic inputs, excluding request IDs, timestamps, aliases, and traversal order.
4. Preserve canonical original-document custody and original anchors; preserve virtual document/anchor plus mapping identity when reported; virtual coordinates never replace original custody and synthetic provenance is forbidden.
5. Keep coverage (`COMPLETE_WITHIN_BOUNDS`, `PARTIAL`, `UNKNOWN`) distinct from failure/empty outcomes; unsupported, unavailable, malformed, transport failure, bounded truncation, and no observations are not interchangeable.
6. Derive deterministic graph-v4 normalized relations from one or many observations, independent of input order, retaining sorted unique IDs of every contributing observation and exact adapter identity/version.
7. Preserve graph-v4 non-entailments: non-CALLS evidence never becomes CALLS; passage never entails invocation; update never entails render; parse/analyze never entails runtime execution, feature identity, repaint, or whole-source completeness.
8. Do not parse Ember/Glimmer syntax in Go; the adapter validates and canonicalizes provider-reported envelopes only.

## Enforcement sequence

1. Commit this claim as the first mutation after reading the frame and all focused claims.
2. Inspect only the adjacent exported contracts needed by the new package.
3. Add assertion-specific tests and capture named red failures against a deliberately incomplete adapter surface.
4. Implement one adapter-owned behavior at a time without modifying adjacent-frame files.
5. Run focused package tests, adjacent contract tests, and the full repository suite.
6. Verify no MCP bootstrap or production collector wiring diff, update this claim with evidence, and commit a clean worktree.

## Red evidence

Against a compile-valid adapter surface that returned an empty result, `go test ./internal/observationadapter -count=1 -v` reported `Go test: 0 passed, 5 failed in 1 packages`. Named failures included `ASSERT_ADAPTER_IDENTITY_FAILS_CLOSED_PROVIDER`, `ASSERT_ADAPTER_IDENTITY_FAILS_CLOSED_PROTOCOL`, `ASSERT_ADAPTER_IDENTITY_FAILS_CLOSED_ADAPTER`, and the missing-observation panic exposed the absent adaptation result. The incomplete surface was then replaced by the implementation below.

## Implementation

- Added strict provider/protocol/adapter identity and `PROVIDER_REPORTED` envelope-authority validation.
- Added deterministic observation identities over provider, protocol, adapter, and canonical semantic observation identity while excluding request ID, timestamp, source bytes, and traversal order.
- Delegated immutable original/virtual document validation to the existing custody contract and retained both anchor coordinate spaces plus mapping identity.
- Preserved coverage independently from the closed failure vocabulary.
- Delegated endpoint-role and prohibited-claim enforcement to existing observation semantics; provider observations cannot produce CALLS.
- Grouped equivalent reported observations into deterministic graph-v4 relations, retaining sorted unique contributor IDs and adapter identity/version.
- Kept provider source bytes opaque; no Ember/Glimmer parser or syntax interpretation exists.
- Added only the exact new paths to the integrated-conformance ownership allowlist.

## Tests

- Focused: `go test ./internal/observationadapter -count=1 -v` — `Go test: 9 passed in 1 packages`.
- Adjacent: `go test ./internal/relations ./internal/graph ./internal/observationadapter -count=1` — `Go test: 150 passed in 3 packages`.
- Full: `go test ./... -count=1` — `Go test: 3161 passed in 40 packages`.
- `go vet ./...` — PASS, no diagnostics.
- `git diff --check` — PASS.
- Forbidden-path diff over `cmd/lsp-trace-mcp`, `internal/mcp`, `incomingops`, and `sliceops` — empty.

## Commit

This commit — `feat: add provider observation adaptation boundary`.
