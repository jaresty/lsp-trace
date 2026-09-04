# Bootstrap Provider Declarations claim

Status: CLAIMED
Baseline: 4ce6e6df0ce97cdd0fd641fa046e68879cf3daae
Frame: Bootstrap Provider Declarations

## Goal

Extend the host-owned bootstrap configuration with strict, versioned provider declarations and deterministic conversion into `internal/provider.Provision` input and output. Preserve the existing managed-LSP-only bootstrap path. This frame declares providers but does not attach collectors or adapt provider semantics.

## Claims read before mutation

- `observation-adaptation.claim.md` (adjacent nested frame; explicitly leaves MCP bootstrap and provider process execution unclaimed)
- `internal/provider/provisioning.go` (adjacent exported Provision contract)
- `cmd/lsp-trace-mcp/bootstrap.go` (existing bootstrap contract)

The external FRAMEWORK hierarchy named by the task is not present inside this isolated worktree, so no claim beyond the checked-out baseline files above is represented as read.

## Exact ownership

Preferred new bootstrap-specific files:

- `cmd/lsp-trace-mcp/bootstrap_provider.go`
- `cmd/lsp-trace-mcp/bootstrap_provider_test.go`

Narrow existing edit:

- `cmd/lsp-trace-mcp/bootstrap.go` only to add the optional provider declarations field and permit provider-only or mixed version-1 bootstrap configs while retaining managed-process behavior.

Explicitly unclaimed: collector attachment, provider invocation, semantic adapters, observation adaptation, MCP tool behavior, and changes to `internal/provider`.

## Retained behavioral properties

1. Strict decode: a successful bootstrap decode represents exactly one JSON value and rejects unknown fields at every decoded level.
2. Declaration validity: every configured provider declaration has the supported declaration schema version, absolute executable and optional working directory, stable identity and provider version, protocol identity/version, explicit non-empty capabilities, and positive limits no larger than `provider.MaxProviderLimits`.
3. Deterministic conversion: valid bootstrap provider declarations convert to exact `provider.Declaration` values and `provider.Provision` returns identity-sorted canonical declarations independent of input order.
4. Immutable copying: conversion and provisioning do not retain aliases to bootstrap declaration argument, environment, or capability slices.
5. Managed-only compatibility: existing version-1 configurations containing only managed LSP processes retain their decode, preparation, and runtime behavior.
6. Scope exclusion: this frame does not attach collectors and does not implement semantic adapters.

## Enforcement sequence

1. This claim is the first repository mutation.
2. Add assertion-specific bootstrap provider tests and observe named failures before production implementation.
3. Implement the smallest bootstrap-only schema/conversion surface and narrow bootstrap loader change.
4. Run focused tests, full tests, `go vet ./...`, and `git diff --check`.
5. Verify the diff contains no collector or semantic-adapter wiring, record exact evidence, and commit with a clean worktree.

## Red evidence

Before implementation, `go test ./cmd/lsp-trace-mcp -run 'TestBootstrapProviderDeclaration' -count=1 -v` reported `Go test: 0 passed, 7 failed in 1 packages`. The output named `ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_DETERMINISTIC_PROVISION_INPUT`, `ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_DISALLOW_UNKNOWN_FIELDS`, `ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_ABSOLUTE_AND_HOST_BOUNDED`, and `ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_IMMUTABLE_COPY`.

## Implementation

- Added a strict `lsp-trace.bootstrap-provider.v1` declaration schema in a bootstrap-specific file.
- Added immutable conversion into `provider.Declaration` and bootstrap-owned invocation of `provider.Provision`, preserving its deterministic canonical output and host maxima.
- Added optional `providers` to version-1 bootstrap configuration; configurations remain valid with managed processes only, providers only, or both.
- Retained the existing decoder's `DisallowUnknownFields` and second-decode EOF check.
- Added no collector attachment, provider invocation, or semantic adapter.

## Verification evidence

- Assertion-specific focused: `go test ./cmd/lsp-trace-mcp -run 'TestBootstrapProviderDeclaration' -count=1 -v` — `Go test: 13 passed in 1 packages`.
- Bootstrap compatibility focus: `go test ./cmd/lsp-trace-mcp -run 'TestBootstrapProviderDeclaration|TestBootstrapConfigIsStrictAndHostOwned|TestBootstrapHostOwnedAliases|TestPrepareBootstrapRejectsDuplicateSessionIdentity|TestBootstrapRollbackAndShutdownOwnEveryStartedSession' -count=1` — `Go test: 16 passed in 1 packages`.
- Adjacent provider: `go test ./internal/provider -count=1` — `Go test: 27 passed in 1 packages`.
- Integrated ownership assertion: `go test ./internal/integratedconformance -run 'TestDisabledIntegratedConformance/ASSERT_PACKAGE_OWNERSHIP_ONLY' -count=1` — `Go test: 2 passed in 1 packages`.
- Final full suite excluding the baseline's intentionally unimplemented real-provider collector composition: `go test ./... -skip TestProductionMCPRealProviderConformance -count=1` — `Go test: 3199 passed in 40 packages`.
- Final unexcluded full suite was run: `go test ./... -count=1` — `Go test: 3198 passed, 5 failed, 2 skipped in 40 packages`; four failures are the existing `TestProductionMCPRealProviderConformance` family whose diagnostics explicitly require the collector boundary forbidden by this claim, and one transient `TestSliceManagedFakeProviderProcess` failure passed immediately in isolation as `Go test: 1 passed in 1 packages`.
- `go vet ./...` — PASS, no diagnostics.
- `git diff --check` — PASS, no diagnostics.
