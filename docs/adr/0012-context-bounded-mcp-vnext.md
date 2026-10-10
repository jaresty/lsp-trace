# ADR 0012: Context-bounded MCP vNext

- **Status:** Proposed
- **Date:** 2026-10-10
- **Authority:** 0
- **Accepted:** false
- **Implementation authorization:** Not granted
- **Migration authorization:** Not granted
- **Related:** ADR 0004, ADR 0007 host-LLM bridge, ADR 0011

## Context

The current MCP surface has accumulated many independently versioned direct tools and large nested schemas. Advertising those definitions repeatedly consumes a material share of an LLM context window before any user evidence is loaded. The pressure has already caused practical context-limit failures and makes qualified capabilities harder, not easier, for an agent to discover and use.

The problem is transport presentation, not loss of product capability. Existing deterministic kernels, immutable artifacts, historical readers, custody rules, typed failures, and qualification boundaries remain valuable. A next-generation public surface may therefore be intentionally incompatible at the MCP request, response, tool-name, profile, and advertisement layers while preserving historical evidence and internal operation semantics.

## Decision

Design an additive, context-bounded MCP generation whose default advertised surface is small and whose detailed operation schemas and large artifacts are loaded only on demand. MCP vNext is the intended public-exposure vehicle for newly qualified ADR 0011 definition/reference grouping, successor cross-seed instability, relation-scoped design coupling, source-context/replay artifacts, and later authorized ADR 0007 candidate workflows. Those capabilities should not first enlarge the historical default direct-tool schemas and then require immediate transport migration.

The initial conceptual core is:

```text
lsp_capabilities
lsp_schema_get
lsp_execute
lsp_artifact_read
lsp_session
```

A later qualification may admit a few small workflow-oriented tools such as `lsp_inspect`, `lsp_trace`, or `lsp_discover`. Those tools are thin adapters over the same execution kernel and do not own independent semantics.

This ADR permits a breaking transport redesign. It does not permit rewriting retained artifacts, weakening validation, changing authority, silently widening qualified operations, or enabling public execution.

## Separation of contracts

MCP vNext separates three contracts that the historical surface often advertised together:

1. **Transport envelope:** a small stable request/result shape advertised to every client.
2. **Operation contract:** a strict versioned schema retrieved through `lsp_schema_get` and validated by `lsp_execute`.
3. **Artifact contract:** immutable canonical bytes addressed by identity and read through bounded projections or pages.

A small transport envelope never means permissive operation input. The executor resolves the exact operation and schema identity, validates the complete operation request, and fails closed before effects.

## Capability discovery

`lsp_capabilities` returns a compact bounded catalog. Each entry contains only:

- canonical operation name;
- supported contract versions;
- live, retained, or dual mode;
- concise purpose;
- exact input/result schema identities;
- authority and availability status;
- required session, custody, qualification, or execution gates.

It does not inline every operation schema. Catalog ordering, pagination, truncation, and total accounting are deterministic.

## Execution envelope

The conceptual envelope is:

```json
{
  "operation": "structural_context",
  "contract_version": "v3",
  "input": {},
  "input_ref": null,
  "request_id": "host-owned-id",
  "deadline_ms": 30000
}
```

Exactly one of `input` or `input_ref` is present. The advertised root schema remains `type: object`. Unknown fields, unknown operations, unsupported versions, ambiguous schema identity, unbounded input, and mismatched references fail closed.

Parallel requests have no implied semantic order. Dependent requests carry exact predecessor identities. Automatic host execution remains blocked by the parallel-MCP qualification and contingency gates in the ADR 0007 host-LLM bridge contract.

## Artifact references and bounded reads

Large input and output values default to immutable descriptors:

```json
{
  "artifact_id": "sha256:...",
  "schema_id": "https://...",
  "byte_length": 12345,
  "authority": 0,
  "completeness": "UNKNOWN",
  "summary": {},
  "next_page": null
}
```

`lsp_artifact_read` returns bounded pages or named projections under exact artifact identity, schema, privacy partition, and byte/work limits. Page continuation binds a stable snapshot. Ambient filesystem paths, mutable “latest” lookup, unbounded inline expansion, and cross-partition reads are forbidden.

## Live operations and feature parity

The compact surface must preserve qualified access to existing capabilities, including:

- managed session lifecycle;
- live trace and incoming relationships;
- Structural Context and source projection;
- workspace-symbol location and census;
- retained inspection, hydration, and verification;
- Program C grouping, composition, instability, and relation-scoped coupling when separately qualified;
- structural delta and retained artifact readers;
- future private ADR 0007 workflows when separately authorized.

A transport redesign cannot claim feature parity from operation-name counts. Qualification uses behavior, canonical artifact, failure, limit, custody, and authority equivalence.

## Context budgets

The public contract freezes explicit limits for:

- total bytes of the default advertised tool definitions;
- each tool description and input schema;
- capability catalog pages;
- schema pages;
- inline request and result bytes;
- artifact-read page bytes and work;
- diagnostics and terminal accounting.

CI and qualification fail when a profile exceeds its frozen context budget. Budgets include exact serialization overhead rather than only payload fields. No tool may evade a budget by placing a large schema in descriptions, examples, capability prose, or error text.

Exact numeric budgets remain unresolved until representative MCP clients and target models are measured. No default profile is accepted without those retained measurements.

## Profiles and compatibility

The design distinguishes:

- **compact-vnext:** small default surface governed by this ADR;
- **legacy-compatible:** existing direct tools where a bounded transition is separately authorized;
- **hidden-legacy:** historical operations not advertised but retained for qualified artifact compatibility where required.

Legacy dispatch is not automatically permanent. Removal requires migration evidence, retained-reader coverage, and explicit authorization. Historical artifact schemas and readers are not removed merely because their original direct MCP tool is hidden or retired.

## Validation and authority

All advertised tools have root `type: object`. Schema retrieval and generic execution do not weaken closed unions, duplicate-key rejection, canonical identity, typed limits, refusal-before-effect, exact terminal accounting, source-safe diagnostics, or authority ceilings.

Every result preserves the underlying operation's authority, acceptance, completeness, feature-identity, custody, and source-graph limits. The transport cannot promote a result.

## Migration

Migration is staged:

1. inventory current qualified behaviors and artifact readers;
2. freeze compact envelopes and context budgets;
3. implement vNext behind a non-default private profile;
4. prove kernel and artifact parity;
5. qualify live, retained, failure, paging, privacy, and concurrency behavior;
6. expose newly authorized ADR 0011 and ADR 0007 functionality through vNext rather than expanding the historical default direct-tool surface;
7. separately authorize default advertisement;
8. retain or remove legacy direct tools only under an explicit compatibility decision.

No stage rewrites historical artifacts or selectors. No default changes silently.

## Consequences

### Positive

- Tool advertisement consumes materially less LLM context.
- Agents fetch only schemas and artifacts relevant to the current task.
- Large evidence stays immutable and pageable.
- Existing operation kernels can remain authoritative.
- Breaking transport cleanup is possible without historical artifact migration.

### Costs and risks

- Generic execution can reduce discoverability if the capability catalog is weak.
- Schema and artifact retrieval add round trips.
- Artifact storage, paging, privacy, retention, and stale-snapshot behavior require qualification.
- Supporting both compact and legacy profiles during transition increases test scope.
- An overly generic envelope could hide operation semantics unless strict retrieved-schema validation is preserved.

## Rejected alternatives

### Continue adding direct tools

Rejected. It compounds context pressure and repeats large schemas for every session.

### One permissive generic JSON tool

Rejected. Context reduction does not justify weak validation or open-ended inputs.

### Remove historical readers with old tools

Rejected. Transport cleanup does not invalidate immutable evidence or replay obligations.

### Put complete schemas in capability descriptions

Rejected. It moves rather than solves the context-budget problem.

## Unresolved preimplementation gates

The following are not implementation-time discretion. One accepted contract revision must resolve and freeze them before runtime implementation, schema registration, or private-profile wiring begins:

1. exact default tools and workflow shortcuts;
2. exact context budgets and representative client/model matrix;
3. artifact storage, privacy partition, retention, tombstone, and acceptance owner for public vNext;
4. inline-versus-reference thresholds;
5. schema-page and artifact-page continuation and stable-snapshot contracts;
6. legacy transition duration, compatibility-adapter ownership, rollback, and removal criteria;
7. whether `lsp_session` is one action-discriminated tool or a small lifecycle tool family;
8. exact public names and versioning convention;
9. request identity, cancellation, idempotency, concurrency, and predecessor-binding semantics;
10. discoverability corpus, success criteria, and maximum schema-fetch/operation-selection round trips.

Measurements, inventories, contract drafts, and qualification planning may continue while these gates are unresolved. Their existence grants no implementation or exposure authority.

## Authorization boundary

This proposed ADR authorizes documentation and bounded design/qualification planning only. It does not authorize implementation, schema registration, advertisement changes, public execution, legacy removal, migration, release, deployment, or host-LLM execution.
