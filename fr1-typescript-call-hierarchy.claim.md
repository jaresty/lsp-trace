# FR1 generic TypeScript call-hierarchy compatibility

## Claim

At baseline `269f850bc4fdc5f1354b7de98d24784be6beef3a`, managed-session initialization sent no `textDocument.callHierarchy` client capability. FR1 must advertise generic LSP Call Hierarchy support with `dynamicRegistration:false`, accept a static `callHierarchyProvider`, avoid claiming unsupported dynamic-registration state that the runtime does not own, and leave a genuinely unsupported server explicit without issuing prepare/incoming/outgoing hierarchy requests.

For supported servers, a plain-JavaScript caller/callee fixture must derive `CALLS` edges only from exact server responses to `textDocument/prepareCallHierarchy`, `callHierarchy/outgoingCalls`, and `callHierarchy/incomingCalls`, preserving every server-returned URI, range, selection range, call-site range, and opaque item data. FR1 adds no source-derived calls and no TypeScript- or Angular-specific semantics.

## Evidence

- Baseline revision: `269f850bc4fdc5f1354b7de98d24784be6beef3a`
- Required real server: `typescript-language-server 6.0.0` with its locally resolved TypeScript dependency.
- RED before production mutation: `go test ./sessionruntime -run TestReadinessCallHierarchyCapabilityModes -count=1` failed four test nodes: static and unsupported omitted the client capability; dynamic registration remained unsupported. A separate command then printed `DURABLE_FR1_RED_ARTIFACTS_CONFIRMED`.
- Focused GREEN after production mutation: the same command passed all four test nodes; `go test ./sessionruntime -count=1` passed 44 tests; `go test ./cmd/lsp-trace -count=1` passed 219 tests.
- Wire environment: `/opt/homebrew/bin/typescript-language-server --version` reported `6.0.0`. Global TypeScript `7.0.2` was present but unavailable to TLS because it provides no `tsserver.js`; an already-local nested TypeScript `5.9.3` was exposed only in the disposable `/tmp` fixture.
- Exact initialize request advertised `textDocument.callHierarchy.dynamicRegistration:false`; TLS 6.0.0 returned `callHierarchyProvider:true`. It emitted no `client/registerCapability` or unregister event.
- Exact hierarchy traffic: prepare at zero-based `(4,9)` returned `caller` range `(4,0)-(6,1)` and selection `(4,9)-(4,15)`; outgoing returned `callee` range `(0,0)-(2,1)`, selection `(0,9)-(0,15)`, and call site `(5,9)-(5,15)`; incoming returned the matching caller and call site. Graph-v3 retained two nodes and one deduplicated server-reported `CALLS` edge.
- Final GREEN gates: `go test ./...` and `go test -race ./...` each passed 3,325 tests across 41 packages; `scripts/check-ci.sh` passed format/test/vet/build/Python/shell/release/clean/release-dry-run; `scripts/release-check.sh` passed schemas, custody, MCP contract, release binaries, and archive exclusion; `goreleaser check` passed; focused `sliceops` passed 12 tests; focused MCP/operation parity passed 175 tests; explicit unsupported mode passed with zero hierarchy requests.
- DASL JS, Survey Center JS, and Market View JS replay status: unavailable in this worktree/environment because no configured paths were present; no project-specific capability, prepare, traversal, empty, or error conclusion is inferred, and no Angular coverage is claimed.

## Derivation

1. The managed runtime, rather than the separate legacy client, owns active session initialization and transport-neutral operation metadata.
2. Capability support must therefore be derived from the exact initialize exchange plus registration state owned by that session generation.
3. Traversal remains server-authoritative: capability state only admits requests; it does not synthesize relations.
4. Fake-server modes independently falsify static support, dynamic registration, and honest unsupported behavior before real TLS replay.
5. Real-process evidence distinguishes environment availability, capability-level support, prepare quality, traversal output, and project-specific empty/error outcomes without implying Angular coverage.
