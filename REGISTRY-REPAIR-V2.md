# Registry Review Repair V2

## Scope

Repaired the registry review findings from `28229f8` atop parent `1a8b2a7`. No delegation, network access, installation, deployment, product work, live D01, full suite, or full race was performed. Cleanup failures were not addressed.

## Repairs

- Restored `internal/mcp/registry.go` byte-for-byte to `1a8b2a7`; SHA-256 is `f9b99893c09bfa014986ee56fa7b4064f53576a3cde7f9dd36b723842d97444b` for both files.
- Removed the exported production `NewHistoricalAlwaysLocalRegistry` and its production helper refactor by restoring the parent production file.
- Froze historical exact12 evidence in `internal/integratedconformance/testdata/historical-always-local-registry.v1.json` as canonical names plus input-schema identities.
- Added a test-local registry adapter constructed only from public `mcp.Tool` fields. The adapter gates the executable unsupported-start check, which retains `PROCESS_CONTAINMENT_UNAVAILABLE` and zero census effects.
- Kept the current assertion on `mcp.NewRegistry(true)` at exactly 28 advertised tools.
- Replaced source-text separation checking with executable set/cardinality separation between immutable historical evidence and the current runtime registry.
- Added independent real-state `rejectPerturbation` paths for historical exact12, current28, and historical/current separation.
- Existing historical 13/20/21/22/23/25 contracts remain unchanged and passing.

## Falsification witnesses

- Historical exact12 perturbation removes one fixture entry and is rejected with `advertised=11 want=12`; the unperturbed assertion passes.
- Current28 perturbation removes one runtime-advertised tool and is rejected with `advertised=27 want=28`; the unperturbed assertion passes.
- Separation perturbation substitutes historical evidence for current evidence and is rejected with `historical=12 current=12 current_only=0`; the unperturbed assertion passes.

## Focused verification

Passed:

- Exact production byte comparison against `1a8b2a7`.
- Focused normal registry/manifest/integrated-conformance tests.
- Focused race tests over `internal/mcpcontract`, `internal/mcp`, and the three integrated registry assertions.
- Independent mutation executions for exact12, current28, and separation, followed by satisfying replays.
- `./scripts/release-check.sh` (`RELEASE CHECK PASS`).
- `go vet ./internal/integratedconformance ./internal/mcp ./internal/mcpcontract`.
- `go build ./cmd/lsp-trace ./cmd/lsp-trace-mcp`.
- `git diff --check`.
