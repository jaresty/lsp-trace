# Atomic same-provider seed binding claim

## Baseline and scope

Revised on branch `pi-agent-c46f80e2-a3af-42f` from the prior pre-provider external-validator design. Work remained repository-local: no delegation, network, install, deploy, product access, live D01, real LSP provider launch, full suite, or full race.

## Delivered boundary

- The closed `lsp-trace.seed-binding.v2` manifest remains opt-in on current-only v3 CLI private rooted selectors and optional MCP bootstrap process configuration. Omission preserves historical/default behavior and registry operation schemas.
- Mechanical preflight runs before provider-attempt allocation. Host-owned workspace Git custody authenticates `source_revision`; the manifest cannot supply both sides of the comparison. The canonical workspace file must be regular and contained, declaring-file identity must match, max-source+1 detects oversize input, one exact descriptor snapshot is retained, SHA-256 is verified, and locator/declaration-range endpoints must be ordered, in bounds, and exact UTF-8/UTF-16/UTF-32 code-unit boundaries.
- Semantic admission uses no external validator executable, protocol, CLI selector pair, or MCP `seed_validator`. After one managed provider generation initializes, the manager supplies the exact retained bytes with `textDocument/didOpen`, then issues one bounded `textDocument/documentSymbol` request on that exact generation before acquisition target resolution.
- The semantic parser strictly accepts one homogeneous LSP array-level form: recursive `DocumentSymbol[]` or flat `SymbolInformation[]`; mixed arrays and malformed/unknown members are INVALID. A hierarchical match requires exact expected name, locator containment by selection/name and declaration ranges, exact declaration range, valid negotiated-encoding boundaries, and the canonical request document. A flat match requires exact name, same canonical location URI, exact containing declaration range, and records the qualification that no distinct name range exists. Zero or multiple matches are MISMATCH; there is no name-only fallback.
- Only MATCH latches admission and permits `prepareCallHierarchy`. MISMATCH, UNAVAILABLE, and INVALID occur after one provider attempt and issue no prepare/incoming/outgoing traversal requests. The document-symbol request is deterministically charged as one request from the shared acquisition request/time/response limits.

## Compatibility decision

No frozen graph artifact family, public MCP operation input, registry tool count, or default/v1 behavior is extended. The existing manifest validator identity fields remain accepted for manifest compatibility but identify the selected managed LSP authority; they no longer configure or launch another executable.

## Focused evidence

- RED retained at `/tmp/lsp-trace-seed-binding/semantic-replacement-red.log`: the prior implementation reported `ASSERT_SEED_BINDING_OVERSIZE_REJECTED: {Status:MATCH` for a max+1 source.
- Mechanical/semantic parser guard: `go test ./internal/seedbinding -count=1` — 17 passed.
- Same-generation ordering guard: `go test ./acquisitionops -run TestSemanticSeedAdmissionPrecedesPrepareAndBlocksMismatch -count=1` — 3 passed; MATCH records documentSymbol before prepare, MISMATCH records no prepare, and both retain one provider generation.
- Focused normal: `go test ./internal/seedbinding ./sessionruntime ./acquisitionops ./cmd/lsp-trace ./cmd/lsp-trace-mcp -count=1` — 545 passed in 5 packages.
- Focused race: selected binding/parser/order/bootstrap tests in three packages — 21 passed.
- Focused vet: the five touched package groups — `FOCUSED_VET_PASS`.
- Focused build: `./cmd/lsp-trace` and `./cmd/lsp-trace-mcp` to `/tmp/lsp-trace-seed-binding` — `FOCUSED_BUILD_PASS`.
- No full suite, full race, D01, or real provider was used.

## C# and D01

A separate C# adapter is not required. Readiness depends on the selected managed csharp-ls generation returning sufficient document-symbol semantics; unsupported documentSymbol or insufficient protocol availability yields `SEED_BINDING_UNAVAILABLE`, not an observed mismatch. D01 is not ready, not authorized, and was not rerun.
