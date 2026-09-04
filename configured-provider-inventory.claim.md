# Configured Provider Inventory claim

Status: CLAIMED
Baseline: f5362543ac86e1355d51b899d3d482e7ae267343
Frame: Configured Provider Inventory

## Goal

Expose a generic, process-lifetime inventory of host-configured providers through server capabilities without launching providers. Publish provider identity, protocol, capabilities, separately represented configuration/executable/conformance facts, and one closed readiness state: `DECLARED`, `EXECUTABLE_AVAILABLE`, `CONFORMANCE_VERIFIED`, or `READY`.

Extend provider selection so `auto` succeeds only when exactly one configured provider satisfies every requested relation, language, and framework constraint; zero or multiple complete matches fail closed. Requests remain unable to introduce execution authority.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-external-provider-f536254-v1/FRAMEWORK.md`
- every tracked `*.claim.md` at baseline, especially `bootstrap-provider-declarations.claim.md` and `provider-admission-semantic-binding.claim.md`

## Exact ownership

- generic core inventory types and construction under `internal/provider`
- generic provider admission constraint extensions under `internal/provider`
- capabilities publication in the existing MCP/server capability surface
- focused and compatibility tests for those surfaces
- this claim file

Explicitly unclaimed: provider-package files, framework-specific names or branching in core logic, provider launch/transport changes, downloadable or implicit executable discovery, and release documentation.

## Retained properties

1. Inventory entries are derived only from immutable host-provisioned declarations and publish identity, protocol, capabilities, configuration, executable availability, conformance evidence, and readiness without launching providers.
2. Configuration, executable availability, conformance evidence, and readiness are distinct facts; readiness belongs to the closed state set `DECLARED|EXECUTABLE_AVAILABLE|CONFORMANCE_VERIFIED|READY`.
3. Inventory and execution authority are immutable for the process lifetime; request payloads cannot create or expand executable authority.
4. Auto-selection matches all requested relations, languages, and frameworks conjunctively and succeeds only for exactly one complete match; zero and ambiguous matches fail closed.
5. Existing omitted-relation and compatibility behavior is preserved.
6. Core logic remains framework-neutral; provider packages and release documentation remain unchanged.

## Enforcement sequence

1. This claim is the first repository mutation.
2. Add assertion-specific tests against compile-valid present-but-wrong behavior.
3. Observe focused red results for inventory publication/readiness and exact conjunctive selection before production implementation.
4. Implement the smallest generic inventory, selection, and capabilities changes.
5. Run focused, compatibility, full-suite, vet, formatting, and diff checks.
6. Verify forbidden paths and framework-name constraints, update this claim with exact evidence, commit once, and leave a clean worktree.

## Required assertion identities

- `ASSERT_CONFIGURED_PROVIDER_INVENTORY_CAPABILITIES`
- `ASSERT_CONFIGURED_PROVIDER_INVENTORY_NO_LAUNCH`
- `ASSERT_CONFIGURED_PROVIDER_READINESS_CLOSED_AND_DISTINCT`
- `ASSERT_CONFIGURED_PROVIDER_PROCESS_LIFETIME_IMMUTABLE`
- `ASSERT_PROVIDER_AUTO_ALL_CONSTRAINTS`
- `ASSERT_PROVIDER_AUTO_EXACTLY_ONE`
- `ASSERT_PROVIDER_REQUEST_NO_EXECUTION_AUTHORITY`
- `ASSERT_PROVIDER_INVENTORY_COMPATIBILITY`

## Red evidence

The compile-valid present-but-wrong inventory returned no entries and admission ignored language/framework constraints. The focused guard reported `Go test: 2 passed, 5 failed in 2 packages`, including `ASSERT_CONFIGURED_PROVIDER_INVENTORY_CAPABILITIES: entries=[]`, `ASSERT_CONFIGURED_PROVIDER_READINESS_CLOSED_AND_DISTINCT`, and `ASSERT_PROVIDER_AUTO_ALL_CONSTRAINTS`.

Counterfactual perturbations independently changed declared-only readiness to `READY` and removed framework matching. Each produced its assertion-specific failure; restoration returned the focused guard to green.

## Implementation

- Added immutable generic inventory snapshots derived only from canonical host-provisioned declarations.
- Published identity, provider version, protocol, relation/language/framework capabilities, configured state, executable availability, conformance verification, and derived closed readiness through capabilities.
- Added optional host-declared executable-availability and conformance-verification facts; inventory construction performs no process launch, executable probe, download, or implicit discovery.
- Extended admission and strict collector input to conjunctively require every requested relation, language, and framework while preserving omitted language/framework compatibility.
- Constructed the MCP registry with its provider inventory before serving; no runtime inventory mutation surface exists.
- Preserved selector-only request authority: requests cannot supply executable paths, arguments, environment, or other execution authority.

## Verification evidence

- Focused inventory/admission/capabilities guard: `Go test: 11 passed in 2 packages`.
- Adjacent provider, MCP, MCP contract, command, and ownership compatibility with the pre-existing production-real-provider test excluded: `Go test: 246 passed in 5 packages`.
- Provider and MCP race suites: `Go test: 98 passed in 2 packages`.
- Full repository with only the two baseline later-frame production qualification tests excluded: `Go test: 3250 passed in 43 packages`.
- Unexcluded adjacent run: `Go test: 219 passed, 2 failed in 4 packages`; only the baseline `TestProductionMCPRealProviderConformance` family failed.
- Unexcluded full run: the only non-ownership baseline failure was `TestProductionEmberProviderCompletesManagedB05Lifecycle`; this frame fixed its temporary ownership-allowlist failure.
- `go vet ./...`: pass.
- `git diff --check`: pass.
- Diff contains no `provider/` package path and no release documentation.
