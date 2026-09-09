# Topmost Siblings Repair

## Scope

This repair preserves frozen graph-v3 decoding, semantic replay, and historical sibling relation IDs while adding graph-v5 exact topmost-sibling correspondence. It does not alter graph-v4 normalized provider relations.

## Blocker repairs

- **Historical graph-v3 replay:** v3 sibling evidence no longer depends on producer-only seed URI/label fields absent from frozen JSON. Historical candidate-only origin shape is admitted. Existing serialized sibling relation IDs survive replay; both frozen and briefly emitted historical IDs are admitted without rewriting.
- **Honest graph-v5 identity:** topmost-sibling CLI acquisition selects `lsp-trace.graph.v5`. Every nomination carries exact seed URI/label/identity, distinct origin and candidate nodes, canonical direction/kind, provider identity, the exact `documentSymbol` and `prepareCallHierarchy` method references, source-byte digests, revision/custody, execution bundle, and a v2-domain sibling relation ID.
- **Canonical replay:** graph-v5 uses a distinct semantic receipt version/domain/scope. Semantic validation recomputes the bundle, relation identity, evidence receipt, memberships, and every correspondence coordinate. Field-specific diagnostics identify failed coordinates.
- **Closed nearest-container semantics:** document scope, nested callable scope, nearest type scope, constructors, and mixed callable kinds are covered. Non-callable, ambiguous, and unmatched seeds produce diagnostics and no fallback nominations.
- **No invented calls:** sibling nominations remain discovery-only relations with zero support contribution. CALLS remain only server-reported call-hierarchy edges. The end-to-end fixture retains its reported CALLS edge while separately proving exact `leaf -> peer` sibling correspondence.
- **Registration:** graph-v5 is embedded and registered additively in the graph schema family, semantic validation dispatch, CLI marshal/publication path, and therefore shared MCP `schema_get`/`validate` routing. Existing versions remain registered.
- **Mutation resistance:** focused tests reject mutation of seed identity/URI/label, endpoints, direction/kind, provider and LSP references, source digests, custody, execution bundle, relation ID, and seed membership.
- **Provider evidence:** fake-LSP advertises document-symbol support and provides a hierarchical fixture, allowing a real subprocess/process-boundary test without a real provider.

## Focused evidence

Executed without network, installs, deployment, real providers, the full suite, or full race:

```text
go test ./internal/graph ./internal/traverse ./internal/schema ./internal/operation ./cmd/lsp-trace ./cmd/fake-lsp -count=1
Go test: 559 passed in 6 packages
```

The end-to-end process test builds the CLI and fake server, captures exact protocol methods, validates emitted graph-v5 structurally and semantically, and asserts exact sibling endpoints independently from CALLS. A focused operation test executes MCP `schema_get` and `validate` for graph-v5.

`git diff --check` passed before final report creation.
