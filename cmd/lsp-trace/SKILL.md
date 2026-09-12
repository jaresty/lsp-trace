---
name: lsp-trace
description: Structural code analysis with lsp-trace: route live or offline tracing, census, inspection, hydration, verification, and CLI/MCP operations while preserving evidence boundaries. Use for exact caller/callee and bounded code-neighborhood questions; use lsp-trace-feature-inventory for feature inventory work.
---

# lsp-trace

Use this skill for structural code analysis backed by bounded language-server or retained evidence. It dispatches to detailed references; load only the reference needed for the current task.

Use only trusted language-server binaries and workspaces. They run with the developer's permissions, are not sandboxed, and may access local files, builds, restores, or the network.

## Route the task

- Exact live target or callers: read [Live tracing and census](references/live-tracing.md).
- Existing artifact, selector, inspection, comparison, hydration, or verification: read [Offline evidence operations](references/offline-evidence.md).
- CLI versus MCP, profiles, coordinates, publication, or lifecycle: read [Transport and routing](references/transport-routing.md).
- Authority, completeness, attribution, capture sets, communities, or instability: read [Evidence boundaries](references/evidence-boundaries.md).
- Feature inventory preparation or adjudication: load the separate `lsp-trace-feature-inventory` skill. Structural analysis may supply evidence to it but cannot accept feature identity.

## Command router

Prefer a matching READY managed language-server session for code-relationship questions. Discover the exact target textually only when it is not yet known; after structural tracing, read source bodies for semantic interpretation.

- `trace`: intent-oriented exact managed symbol or repeated exact-position facade; current implemented CLI syntax is documented in `--help` and the repository README.
- `incoming`: start from exact callee positions and trace callers upward.
- `slice`: discover bounded outgoing nodes, then trace incoming callers from the exact frontier and genuine server-reported empty outgoing leaves.
- Legacy `slice --from-file PATH`: server-reported document-symbol census. It is not complete callable, source, or feature coverage.
- `inspect`: admit and project one seed or all retained seeds without changing evidence authority.
- `filter`: mechanically compare exactly two seeds from an admitted all-seeds inspection.
- `inspect-hydrated`: retrieve retained context without reacquiring source.
- `verify`: audit selector custody or an exact retained passage, depending on the implemented subcommand.
- `validate`: validate the selected family/version contract without rewriting input.
- `export-retained-calls` and bounded retained analytics: offline structural projections over admitted retained CALLS.

Future intent-oriented census or context interfaces described in ADRs are proposals unless the current binary help, capability registry, or README explicitly documents them as implemented. Do not invent finalized syntax.

## Shared evidence boundary

CALLS means server-reported LSP Call Hierarchy evidence only. Dispatch, siblings, communities, centrality, hubs, and instability are separate structural evidence and contribute no feature identity, purpose, runtime proof, or acceptance.

Preserve bounded zero, `UNKNOWN`, failed/null/incomplete outcomes, untouched ambiguity, and per-seed attribution explicitly. Never turn a successful bounded walk into source completeness or absence.

Capture sets preserve exact constituents. They do not infer cross-capture CALLS and do not acquire native single-capture custody. Composition, transport, validation, verification, inspection, hydration, rendering, or publication never upgrades authenticity, producer identity, source truth, runtime execution, coverage, confidence, feature identity, or acceptance.

## Retrieval and authority

`lsp-trace skill get` prints this embedded dispatcher. Detailed references and the separate feature-inventory skill are source-tree resources; current binary retrieval does not list, install, or emit them. Use the current binary's `--help`, `lsp-trace info`, MCP capabilities, embedded schemas, and repository contracts as syntax/schema authority.
