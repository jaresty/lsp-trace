# Generic Provider Conformance claim

Status: CLAIMED
Baseline: f5362543ac86e1355d51b899d3d482e7ae267343
Frame: Generic Provider Conformance

## Goal

Own a framework-neutral core command and API that runs generic protocol conformance against one host-supplied absolute executable with declared provider identity, protocol identity, capabilities, and limits. The harness performs no executable discovery or download and makes no framework-correctness inference.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-external-provider-f536254-v1/FRAMEWORK.md`
- every tracked `*.claim.md` at baseline, including provider admission, runtime, executable, custody, adaptation, and qualification claims

## Exact ownership

- `generic-provider-conformance.claim.md`
- `internal/providerconformance/`
- `cmd/lsp-trace/provider_conformance_command.go`
- `cmd/lsp-trace/provider_conformance_command_test.go`
- generic conformance request/report schemas under `schema/schemas/`

No provider package, configured inventory/capability implementation, release policy, framework analyzer, semantic adapter, or qualification artifact is owned or modified.

## Retained behavior

1. The API accepts only an explicit absolute executable plus declared identity, protocol, capabilities, and positive bounded limits; it performs no discovery or download.
2. A run writes exactly one bounded frame and accepts exactly one bounded response frame, with strict request and response schemas.
3. Provider, protocol, and request identities match exactly; original custody is authoritative and immutable, while virtual custody is subordinate and linked to the declared original.
4. Time, frame, observation, and diagnostic bounds fail closed.
5. Cancellation terminates the child and the API does not return before process reap is observed.
6. Two fresh runs with the same request must produce byte-identical response payloads.
7. Reports use a generic JSON schema and explicitly distinguish passed, unsupported, unavailable, partial, empty, and failed outcomes.
8. Conformance validates generic protocol behavior only and never infers framework correctness.

## Enforcement

Add persistent assertion-specific tests first and observe named failures against compile-valid present-but-wrong behavior. Only then implement the smallest core API/command and schemas, run the identical focused tests green, attempt reductions and distinguishing-state checks, run full tests and `go vet ./...`, verify forbidden-path diffs are empty, update this claim with evidence, and commit cleanly.

## Evidence

- Assertion-specific red: `go test ./internal/providerconformance -run TestGenericProviderConformance -count=1 -v` — 0 passed, 6 failed; output named host declaration, one-frame/schema/identity/custody, bounds/cancel/reap, deterministic replay, and explicit outcome assertions.
- Focused green after adding malformed custody, duplicate-frame, cancellation, timeout, and item-bound distinguishing states: `go test ./internal/providerconformance ./cmd/lsp-trace -run 'TestGenericProviderConformance|TestFrameworkNeutralSource|TestProviderConformanceCommand' -count=1 -v` — 9 passed in 2 packages.
- Ownership guard: `go test ./internal/integratedconformance -run 'TestDisabledIntegratedConformance/ASSERT_PACKAGE_OWNERSHIP_ONLY' -count=1 -v` — 2 passed.
- Full suite: 3255 passed, 1 failed, 2 skipped in 44 packages. The sole failure is the baseline-assigned `TestProductionEmberProviderCompletesManagedB05Lifecycle` production qualification family, outside this frame and unchanged.
- `go vet ./...` — pass, no diagnostics.
- Both new schema documents parse as JSON; `git diff --check` passes.
- Forbidden-path diff across provider package, MCP provider inventory/composition, qualification, release configuration, and provider documentation is empty.
