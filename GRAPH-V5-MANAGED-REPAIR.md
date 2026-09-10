# Graph V5 Managed Repair

Base: `3a527fcf38e37be43542a5ab141ec4de5c6e8c7d` (`pi-agent-1fa12b24-648f-440`)

## Repair

- Wired `expansion.topmost_siblings` into `internal/acquisition.Acquire` after resolution/admission and before call traversal.
- Routed `textDocument/documentSymbol` and sibling `textDocument/prepareCallHierarchy` through the coordinator's existing `invoke`, sharing request, evidence, node, prepare-probe, and timeout accounting across all targets.
- Added deterministic hierarchical symbol selection and exact prepared sibling correspondence without producing or inferring CALLS edges.
- Preserved discovery relations through deterministic acquisition replay and promoted them only into native `lsp-trace.graph.v5` output.
- Added exact V5 provenance fields, source digests, seed identity, custody evidence, canonical relation IDs, and nonempty-output rejection.
- Added CLI `--output-version lsp-trace.graph-provenance.v5`; it is accepted only with acquisition v3 and `expansion.topmost_siblings=true`, and publication validates as V5.
- Added a hierarchical fake-runtime specimen with one exact seed, a broad root, a sibling method, observed LSP requests, nonempty exact relations, and repeated-route byte parity.
- VERIFIED_HOST now requires an exact host receipt identifier; CALLER_ASSERTED_LOCAL is not promoted. Existing V5 semantic validation retains the LSP-supplied, retained-byte-consistency, UNKNOWN, and MISSING class rules and rejects authority mutations.

## Focused validation

- Normal: 1120 tests passed across acquisition, graph, graph provenance, schema, seedbinding, acquisitionops, CLI, and MCP packages.
- Race: 13 focused managed-V5/budget/custody/authority/parity tests passed.
- Process/schema: 56 focused CLI/MCP process, managed-V5, acquisition, and schema tests passed.
- Vet: focused packages passed.
- Build: `cmd/lsp-trace`, `cmd/lsp-trace-mcp`, and `cmd/fake-lsp` passed.
- Post-custody focused rerun: 232 tests passed.

No full suite or full race was run. No delegation, network, install, deploy, product, D01, or export action was performed.

## Independent re-review

Reviewed the final diff in-thread independently from implementation sequencing. Confirmed:

1. Expansion executes inside the coordinator and uses its sole `runner.result.Usage` and context.
2. Multi-target work does not instantiate per-seed budgets; duplicate admitted seed identities are deduplicated.
3. Budget-blocked/capture-incomplete requests remain explicit request records and mark acquisition partial.
4. Sibling candidates are discovery relations only; no call edge is synthesized.
5. V3 callers cannot select V5 output accidentally; CLI and executor reject unsupported combinations.
6. V5 output rejects an empty sibling set and validates exact embedded graph bytes.
7. Custody promotion is fail-closed and VERIFIED_HOST cannot be emitted without a receipt.

## Documentary limitation

`GRAPH-V5-MANAGED-CERTIFICATION.md` was not present in the exact commit tree or tracked local history available to this isolated worktree. The repair therefore addresses every blocker explicitly supplied in the task statement; it cannot quote or cross-check additional text from an unavailable untracked certification artifact.
