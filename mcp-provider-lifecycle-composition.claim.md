# MCP Provider Lifecycle Composition claim

Status: CLAIMED
Baseline: 4ce6e6df0ce97cdd0fd641fa046e68879cf3daae
Frame: MCP Provider Lifecycle Composition

## Goal

Compose an already provisioned provider registry, admission resolver, and semantic adapter into exactly one bounded internal provider runtime/collector lifecycle, attach that collector to `hostSelectorRuntime`, and do so before production incoming and slice executors are constructed.

## Exact ownership

Preferred new production and test files:

- `cmd/lsp-trace-mcp/provider_lifecycle_composition.go`
  - one narrow lifecycle composition function
  - narrow local consumer interfaces only when required by unavailable sibling APIs
- `cmd/lsp-trace-mcp/provider_lifecycle_composition_test.go`
  - assertion-specific in-process composition guards

Existing files may be changed only at the narrow `hostSelectorRuntime` construction seam needed to invoke composition before incoming/slice executor construction. Exact files and symbols will be recorded after baseline interface inspection.

Explicitly unclaimed:

- bootstrap declaration parsing;
- provider provisioning and semantic validation rules;
- provider runtime transport semantics;
- incoming/slice relation or graph semantics;
- MCP schema publication;
- real-process fixtures and conformance tests;
- broader graph analytics, Glint activation, and NAIS files.

## Behavioral dimensions

1. Admitted configuration constructs exactly one bounded provider runtime and collector lifecycle.
2. The collector is attached to `hostSelectorRuntime` before incoming and slice executor construction.
3. Omitted relations bypass collector resolution and start.
4. Successful provider evidence cannot overwrite independent CALLS gaps or completeness.
5. No-bootstrap behavior remains unchanged.
6. Composition consumes existing provisioning, admission, runtime, collector, and semantic-adapter interfaces without copying sibling logic.

## Enforcement sequence

1. Inspect baseline interfaces and narrow the claimed seam if necessary.
2. Add committed assertion-specific guards using in-process fakes only.
3. Exercise present-but-wrong perturbations so each retained behavioral property emits its own red result.
4. Block production implementation until the focused red result names the assertion-specific failures.
5. Implement only the narrow composition function and construction-seam attachment.
6. Rerun the identical focused guard green, then full tests, `go vet ./...`, and diff hygiene.
7. Record exact red/green evidence and commit a clean worktree.
