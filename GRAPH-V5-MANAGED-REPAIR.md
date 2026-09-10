# Graph V5 Managed Repair

Base: `99798f9`

## Repair

- Wired `expansion.topmost_siblings` into `internal/acquisition.Acquire` after resolution/admission and before call traversal.
- Routed `textDocument/documentSymbol` and sibling `textDocument/prepareCallHierarchy` through the coordinator's existing `invoke`, sharing request, evidence, node, prepare-probe, and timeout accounting across all targets.
- Added deterministic hierarchical symbol selection and exact prepared sibling correspondence without producing or inferring CALLS edges.
- Preserved discovery relations through deterministic acquisition replay and promoted them only into native `lsp-trace.graph.v5` output.
- Added exact V5 provenance fields, source digests, seed identity, custody evidence, canonical relation IDs, and nonempty-output rejection.
- Added CLI `--output-version lsp-trace.graph-provenance.v5`; it is accepted only with acquisition v3 and `expansion.topmost_siblings=true`, and publication validates as V5.
- Added a hierarchical fake-runtime specimen with one exact seed, a broad root, a sibling method, observed LSP requests, nonempty exact relations, and repeated-route byte parity.
- VERIFIED_HOST now requires an exact host receipt identifier; CALLER_ASSERTED_LOCAL is not promoted. Existing V5 semantic validation retains the LSP-supplied, retained-byte-consistency, UNKNOWN, and MISSING class rules and rejects authority mutations.
- V5 capture now branches directly from the acquired graph and never passes through the V2 carrier's mutable typed graph or V2 graph-byte remarshal-equality contract.
- V5 marshaling canonicalizes only a detached deep working copy. Verification checks the exact base64-decoded `graph_v5` bytes and SHA-256 first, then creates a fresh typed decode solely for structural and semantic validation.
- V5 semantic receipt verification hashes the exact selected semantic byte prefix. It does not establish authority by repeatedly marshaling `EvidenceV2.Acquisition.Graph` or by sharing typed storage with immutable carrier bytes.
- Sibling relation IDs include origin, declaration, prepared candidate, correspondence evidence, and custody, preserving identity across reordering of three or more siblings while retaining zero-CALLS semantics.
- `lsp-trace verify --family graph-provenance --version v5 PATH` verifies a direct immutable V5 carrier and reports `verified immutable carrier`; selector-based historical verification continues to report publication custody.

## Validation

The final implementation is gated by focused immutable-carrier, mutation, sibling-reordering, acquisition, CLI, schema, custody, and privacy tests; the complete suite, changed-package race tests, vet, builds, and the prepared live C# acceptance are run before commit. The live acceptance requires nonempty origin-excluded siblings, distinct declaration/prepared evidence, zero CALLS, exact embedded-byte digest, and successful V5 verification.

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
