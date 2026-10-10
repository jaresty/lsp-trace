# MCP vNext qualification plan

State: UNFROZEN DRAFT. This plan does not qualify or authorize MCP vNext.

## Delivery boundary

MCP vNext qualification is part of exposing newly qualified ADR 0011 and later authorized ADR 0007 functionality. Private artifact qualification remains prior and independent. New D/R grouping, successor instability, relation-scoped coupling, source-context/replay, and candidate-workflow capabilities target vNext first rather than expanding the historical default direct-tool schemas. Public advertisement still requires a separate final authorization.

## Required evidence

### Context-budget corpus

Measure exact advertised bytes using representative MCP clients and target model contexts. Include default startup, capability discovery, one schema retrieval, one live inspection, and one artifact-read workflow. Record serialization overhead and repeated-turn cost.

### Feature-parity matrix

Map every qualified current operation to one outcome:

- compact direct shortcut;
- `lsp_execute` operation;
- retained compatibility-only operation;
- intentionally retired operation with explicit authority and migration evidence.

Parity is behavioral and includes canonical outputs, failures, limits, custody, authority, and empty/partial outcomes.

### Live operations

Qualify managed session lifecycle, trace, Structural Context, census, source projection, cancellation, deadlines, stale generation, STOP/restart, and parallel MCP behavior. Use the ADR 0007 bridge parallel-request matrix where host execution is in scope.

### Schema retrieval

Prove exact reconstruction, digest/length binding, page ordering, snapshot staleness, unknown schema/version refusal, duplicate-key rejection, bounded pages, and no partial-schema validation claim.

### Artifact reads

Prove content identity, projection identity, privacy partition, page accounting, continuation snapshots, retention/tombstone behavior, range and byte limits, and no ambient filesystem fallback.

### Kernel parity

For each operation, compare compact transport, CLI where applicable, and retained compatibility adapter against one shared kernel. Require byte-identical canonical artifacts or an explicitly versioned projection with exact predecessor binding.

### Failure and security matrix

Cover malformed envelopes, open/unknown fields, unsupported versions, schema substitution, artifact substitution, oversized inline data, page amplification, cancellation at each effect boundary, source-safe diagnostics, and authority preservation.

### Migration

Prove that historical artifacts and readers remain usable independently of direct-tool advertisement. Default-profile change, compatibility duration, and any removal require separate acceptance.

## Qualification acceptance matrix

Every row is mandatory before ADR 0012 acceptance. “Owner” names an independent acceptance role, not the producing implementation worker. Evidence paths are contract names to freeze during campaign design; their exact digest-bound filenames and schemas must be admitted before execution.

| Concern | Required retained evidence | Independent acceptance owner | Blocking gate |
|---|---|---|---|
| Discoverability | Representative-agent task corpus; capability-catalog transcripts; success, wrong-operation, abandonment, and schema-fetch counts; comparison against the qualified legacy surface | Product/workflow usability reviewer | All required qualified operations are findable within frozen task and round-trip bounds without relying on hidden prompt knowledge |
| Schema retrieval round trips | Exact schema-page bytes, digest/length manifest, reconstruction receipts, client request count and context-byte measurements, stale/duplicate/reordered/missing-page mutation results | Schema and parser reviewer | Complete reconstruction validates exactly; partial, substituted, stale, or reordered schemas fail closed; round trips remain within the frozen budget |
| Context budgets | Exact serialized default advertisement, descriptions, input schemas, capability pages, schema pages, diagnostics, and repeated-turn measurements for each representative client/model pair | Context-budget reviewer independent of transport implementation | Aggregate and per-component numeric ceilings are frozen and every representative environment passes |
| Artifact paging | Immutable artifact/projection fixtures; page manifests; stable-snapshot receipts; privacy-partition, range, amplification, tombstone, retention, and substitution mutation results | Artifact custody and privacy reviewer | Exact reconstruction/accounting passes and every cross-partition, stale, substituted, ambient-path, or over-budget read refuses before disclosure |
| Behavioral parity | Exhaustive operation inventory; compact/CLI/compatibility-adapter result and failure comparisons; canonical-byte or explicitly versioned projection receipts; authority and limit comparisons | Compatibility reviewer independent of adapter implementation | Every qualified operation is mapped and equivalent or has an explicitly authorized retirement/migration disposition |
| Migration and retained readers | Historical artifact corpus; reader/replay matrix; default-profile transition rehearsal; rollback evidence; compatibility-window and removal decision | Migration authority holder plus retained-evidence reviewer | Historical artifacts remain readable/replayable; no default or removal changes without separately accepted migration evidence |
| ADR 0007 parallel MCP | Exact concurrent request matrix; predecessor-identity cases; C18 collision/restart/refusal accounting; arrival/completion reorderings; failed-qualification contingency result | Live runtime/concurrency reviewer independent of bridge producer | Parallel execution passes its bridge contract; failure leaves automatic execution disabled and activates no fallback without separate qualification and authority |

The consolidated enablement reviewer verifies that every row refers to one frozen campaign, exact source revision, complete retained evidence set, and independently attributable acceptance. Passing one row cannot compensate for a missing or failed row.

## Preimplementation contract-freeze gate

No vNext implementation begins until one accepted contract revision resolves and freezes all of:

1. exact public tool and operation names plus versioning rules;
2. default tool set and any workflow shortcuts;
3. numeric aggregate, per-tool, page, inline, diagnostic, and work budgets;
4. schema-page and artifact-page continuation/snapshot contracts;
5. inline-versus-reference thresholds;
6. `lsp_session` lifecycle shape;
7. artifact storage, retention, tombstone, and privacy-partition owner;
8. concurrency, cancellation, request identity, idempotency, and predecessor-binding semantics;
9. compatibility-adapter ownership and the legacy transition/removal decision process;
10. discoverability acceptance corpus, bounds, and scoring rules.

Documentation experiments, measurements, inventories, and qualification design may continue before this freeze. Runtime implementation, schema registration, profile advertisement, migration, and host execution may not.

## Acceptance boundaries

A candidate fails if:

- advertised definitions exceed any frozen byte budget;
- a qualified operation becomes undiscoverable or inaccessible without an explicit retirement decision;
- generic execution accepts input not valid under the exact operation schema;
- large values bypass artifact limits through descriptions, errors, capabilities, or examples;
- live and retained custody is weakened;
- compatibility adapters diverge from shared-kernel behavior;
- parallel host execution lacks its separate runtime qualification;
- authority, acceptance, completeness, or feature identity is promoted by transport.

## Required independent reviews

- architecture and public-contract review;
- context-budget and representative-client review;
- schema/parser/resource review;
- live runtime/custody/concurrency review;
- compatibility and migration review;
- final consolidated enablement review.

Before ADR 0012 acceptance, add a backlink from the active ADR 0011 future CLI/MCP section after its Stage 7/8 text is integrated; do not apply that edit from an older branch and overwrite newer qualification contracts.
