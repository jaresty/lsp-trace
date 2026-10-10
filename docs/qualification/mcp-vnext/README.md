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
