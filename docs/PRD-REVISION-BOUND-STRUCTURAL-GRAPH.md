# PRD: Revision-Bound Structural Graph Evidence and Analysis

**Status:** Revised proposed target contract; not an implementation-completion or qualification declaration

**Product:** `lsp-trace`  
**Audience:** Maintainers and downstream structural-graph consumers  
**Supersedes:** The initial revision-bound structural graph proposal (proposal lineage only)

**Inventory baseline:** `217fbb8` (2026-09-06)

**Revision basis:** Original PRD blob `2645f5685b5fa5b1495188b18c9ba23371671a01` at `81b70e3`, unchanged at the inventory baseline.

Approval of documentation revisions does not adopt the target contract, amend accepted ADRs, authorize the proposed next implementation milestone, or accept a delivery program. Adoption, normative amendment, and acceptance each require an explicit maintainer decision bound to the exact document/code revision, policy versions, evidence, and scope. Until then, requirement language below describes the proposed target, not delivered capability.

[ADR 0001](adr/0001-technical-evidence-packet-projections.md) and [ADR 0002](adr/0002-deterministic-seed-evidence-filtering.md) retain their accepted claim ceilings: inspection and set projection do not authenticate source or execution, increase native evidence authority, establish independent support, or adjudicate feature identity. New custody and analytical contracts must be independently versioned and explicitly adopted; this PRD does not reinterpret historical graph, inspection, filter, validation, or verification semantics.

## 1. Summary

Extend `lsp-trace` from an evidence-preserving LSP call-hierarchy tool into a revision-bound, domain-neutral structural graph evidence and offline analysis provider.

The work is deliberately ordered in two delivery programs:

1. **Trustworthy structural substrate:** authenticate contributing source snapshots and optional revision attestations; separate identity layers; publish capability, completeness, normalized relation, and projection contracts.
2. **Deterministic analysis:** add traversal, components, structural metrics, PageRank, and Personalized PageRank over the authenticated normalized graph.

Infomap, Leiden, and community-boundary reporting remain a later program gated by trustworthy inputs, stable projection semantics, qualified implementations, and a portability/licensing decision.

`lsp-trace` will continue to describe attributable structural evidence. It will not infer product features, business entities, runtime execution, or whole-source completeness from language-server output.

## 2. Foundation inventory — 2026-09-06, baseline `217fbb8`

This inventory describes the pinned repository, not any currently installed binary, provider, or live host configuration. The baseline provides these bounded foundations:

- deterministic bounded incoming traversal and outgoing-then-incoming slices;
- canonical caller-to-callee edge orientation;
- typed terminals, frontiers, diagnostics, and limits;
- conservative server-relative completeness;
- V3 semantic and exact-byte commitments;
- immutable publication generations and offline verification;
- canonical per-seed memberships and execution-bundle joins;
- structural and semantic schema validation;
- retained TypeScript and C# qualification evidence;
- explicit blocked ElixirLS qualification evidence.

Additional implemented precursors at this baseline include:

- `lsp-trace.graph.v4` and normalized provider relations (`internal/schema/schemas/lsp-trace.graph.v4.schema.json`, `internal/graph/relations.go`), retaining original evidence anchors rather than replacing their authority;
- host-provisioned provider collection and admission (`internal/provider/`, `docs/PROVIDERS.md`), with blocked diagnostics and partial relation coverage preserved by `217fbb8`;
- generic shared-operation and CLI/MCP parity infrastructure (`internal/operation/`, `internal/mcpcontract/`), not qualification of the proposed analytics;
- trust-provisioning, source-denominator, qualification-policy, support, and matrix validation primitives (`internal/schema/trust_provisioning_receipt.go`, `internal/schema/denominator.go`, `internal/qualificationpolicy/`, `internal/relations/support.go`, `internal/qualificationmatrix/`). Their presence is not independent operational proof or proof of complete production integration.

The baseline does **not establish completion** of:

- independently authenticated revision claims or custody for every contributing input;
- portable or revision-scoped semantic identity and acquisition/logical identity separation under FR5–FR6;
- FR6 lossless heterogeneous occurrence reconstruction, FR7 validated support dependence, or FR8 digest-bound analytical projection policy; graph.v4 normalized relations are a bounded precursor, not equivalence to `lsp-trace.relations.v1`;
- the proposed offline analytical artifact families or their qualified CLI/MCP operations;
- exact language/provider/version/native-input acceptance, including distinct JavaScript obligations, or integrated qualification of the proposed source and analysis artifacts.

Existing SCC-based cycle accounting is not publication of weak/strong components or a condensation DAG. Program admission must be established by the normative matrix, not inferred from package names, tests, schemas, or successful artifact delivery.

## 3. Problem

The original proposal identified the gaps below in the earlier call-hierarchy foundation. They remain the problem statement, not a claim that every precursor is absent at `217fbb8`: §2 records subsequent bounded implementation. Downstream consumers still need a qualified public contract for source identity, lossless normalization, projection choices, and analytical provenance.

Original limitations motivating the target contract are:

- source identity is limited to successfully resolved seed URI/content-digest pairs;
- `source_revision` and related provenance are caller-asserted;
- a version-control revision does not authenticate materialized bytes that differ from its recorded tree;
- traversal completeness is server-relative and must not be read as source completeness;
- capability advertisement, observed request outcome, and retained qualification are not one concept;
- node IDs are sensitive to absolute URI, ranges, server name, kind, and detail;
- documented relation identity currently conflicts with `canonicalRelationID` over whether source revision participates;
- call sites are losslessly retained but merged under caller/callee relations;
- relation families are distributed across implementation-specific collections;
- no public projection or analytical result contract exists;
- randomized and floating-point analysis needs explicit replay scope and resource policy.

## 4. Goals

### G1. Authenticated source-snapshot custody

Every retained source-backed node, relation occurrence, call site, association, or attributable diagnostic must reference authenticated source bytes in a canonical source snapshot. Version-control revision evidence is an optional typed attestation supplied by the selected custody adapter, not a prerequisite for graph identity.

### G2. Honest support and completeness

Consumers must independently observe protocol advertisement, request outcome, bounded traversal status, provider-relative completion, source-denominator coverage, custody completion, and retained qualification.

### G3. Explicit identity layers

Occurrence identity, portable location, revision-scoped semantic identity, logical relation identity, and cross-revision correspondence must be distinct and testable.

### G4. Lossless normalized relations

Consumers must be able to reconstruct every retained native relation, call-site occurrence, association, endpoint, and provenance reference without importing internal Go packages.

### G5. Transparent graph projections

Every analysis must declare how the directed multigraph was converted into its analytical input.

### G6. Deterministic offline analysis

Within a declared replay environment, identical verified input bytes, parameters, projection, implementation, and numeric contract must yield identical canonical output and logical digest.

### G7. Domain neutrality

Results describe structural topology and provider evidence. They do not identify product features or business meaning.

### G8. Compatibility

Existing V2 and V3 artifacts remain readable, and historical IDs are never silently rewritten or reinterpreted.

### G9. CLI and MCP parity

Normalized relation export and offline analysis must be available through both the CLI and MCP using the same operations, projection semantics, canonical result models, validators, and digest rules.

## 5. Non-goals

`lsp-trace` will not own:

- product-feature admission, naming, identity, or inventory membership;
- business entity resolution across repositories;
- UI-to-route, event, queue, topic, callback, or schema-contract inference;
- runtime execution, frequency, or deployed reachability claims;
- AST-level dataflow, taint, or general Code Property Graph semantics;
- domain-specific confidence thresholds or architectural decisions;
- automated service extraction;
- embeddings or communities as semantic identity authority;
- automatic cross-revision equivalence without explicit correspondence evidence.

## 6. Source snapshot, authority, and attestation model

### 6.1 Required distinctions

The product must distinguish:

1. **Content integrity:** bytes match a recorded SHA-256 digest.
2. **Source collection identity:** a logical collection groups source artifacts without requiring one repository or version-control system.
3. **Source snapshot identity:** a domain-separated digest identifies the canonical source manifest and admitted artifact receipts.
4. **Materialized-source binding:** the analyzed files match the recorded source snapshot, including locally modified or otherwise variant bytes where admitted.
5. **Revision attestation:** an optional typed adapter claim binds a source collection or snapshot to a VCS revision, archive, package, build input, or equivalent external identity.
6. **Manifest authentication:** a trusted expected digest or signature authenticates a supplied immutable manifest.
7. **Producer authentication:** optional signature or external custody proving who published an artifact.
8. **Provider evidence:** what a named language server reported during a named execution.

A digest alone proves integrity relative to itself; it does not authenticate the truth, origin, revision, or producer of the bytes. Copying a caller revision into a provider request, response, or graph retains caller authority; it is not provider verification. A separately attributable provider assertion must remain distinct from independent authentication. Hashing materialized bytes alone does not prove membership in the claimed revision: that requires evidence binding those bytes to the independently verified revision/tree or equivalent attestation. Package name, version, path, and hash identify and bind materialized inputs, not authenticated package origin.

Authentication states are closed and machine-validated:

```text
SELF_ASSERTED
INTEGRITY_VERIFIED
AUTHENTICATED
FAILED
```

`AUTHENTICATED` requires a successful verification chain terminating in a trust anchor provisioned independently of the presented manifest and claimant. The verifier must obtain the anchor through a trust-policy input that predates or is authenticated independently of the presented bundle; neither the producer, claimant, presented manifest, nor sibling metadata in the same custody bundle may establish or modify its own trust root. The source manifest must foreign-key a verifier-side provisioning receipt that records the trust-policy ID, anchor type and identity, provisioning authority and channel, provisioning event or trust-store version, verification method, and result. Semantic validation must reject claimant-controlled anchors, anchors introduced only by the presented bundle, and missing or unverifiable provisioning receipts. A self-recorded expected digest can establish integrity but can never produce `AUTHENTICATED`. Publication fails whenever policy requires `AUTHENTICATED` and the chain is absent or invalid.

### 6.2 Custody adapters

The core graph model must not require Git, a mutable worktree, or even one repository. It depends only on source collections, canonical source snapshots, and artifact receipts.

Initial adapters:

- **Git worktree adapter:** accept repository root and optional expected revision; resolve the actual Git object when revision attestation is requested; reject mismatch; record clean, modified, and untracked state; hash all admitted contributing files.
- **Immutable manifest adapter:** accept a source manifest plus a trusted expected manifest digest or verifiable signature; no VCS metadata or mutable worktree is required.

Future adapters may cover signed archives, release packages, OCI artifacts, other version-control systems, remote build inputs, or generated-source bundles without changing normalized graph semantics.

Caller-supplied revision text without adapter evidence remains `CALLER_ASSERTED`, never authenticated. Source-byte authentication remains mandatory; revision attestation is conditional on the selected adapter and requested policy.

### 6.3 Core source identities

The core custody model defines:

```text
source_collection_id
source_snapshot_id
source_receipt_id
```

`source_snapshot_id` is a domain-separated SHA-256 digest of the versioned `SOURCE_SNAPSHOT_IDENTITY_V1` canonical projection. That projection includes source-collection membership and canonical admitted artifact receipts but excludes `source_snapshot_id`, signatures over the completed manifest, transport metadata, publication metadata, and derived authentication fields. Its domain separator, included fields, UTF-8 canonical JSON encoding, object-key ordering, array ordering, and verification algorithm are normative schema semantics. No producer may substitute an empty value, placeholder, or implementation-defined omission policy.

A graph may reference multiple source collections and snapshots when evidence spans repositories, generated inputs, or external admitted sources.

Typed attestations hang from collections or snapshots. Git-specific fields such as repository locator, expected revision, resolved object ID, and worktree state never participate directly in generic node, relation, projection, or analysis semantics.

### 6.4 Source scope policy

The source-manifest contract must classify each referenced URI or path as one of:

- repository file;
- admitted generated file;
- admitted dependency source;
- external file;
- virtual/non-file document;
- unavailable or excluded source.

Every exclusion must have a typed reason. Publication fails if required source-backed evidence lacks an authenticated receipt. Policies for generated, external, dependency, and virtual sources must be explicit rather than inferred from path shape.

Absolute paths may be retained only in sensitive execution evidence when necessary. Public analytical artifacts should use canonical repository-relative paths and repository IDs.

## 7. Functional requirements

### FR1. Complete contributing-source manifest

Publish `lsp-trace.source-manifest.v1` with:

- source collection IDs and custody-adapter kinds;
- canonical source snapshot ID;
- optional typed revision attestations;
- manifest authentication state;
- trust-policy ID, independent anchor reference, verification method, and verification result when authentication is claimed;
- dirty/untracked state;
- canonical workspace-relative path where applicable;
- file content SHA-256;
- source language;
- source classification;
- inclusion reason;
- provider and execution references;
- typed exclusions and failures;
- effective compiler configuration, inherited options and their configuration inputs, project references, declaration inputs, and analyzer overrides, with source receipts or immutable references sufficient to reconstruct the effective analysis environment.

Hash every file contributing a retained node, endpoint range, call-site occurrence, dispatch/type/document association, or source-attributable diagnostic.

Every source-backed graph record must reference one or more source receipts. Offline verification must validate manifest structure, semantics, digest, and graph foreign keys without reading a checkout.

### FR2. Source snapshot custody and revision attestation

1. Construct a canonical source snapshot for every published graph, independent of VCS availability.
2. Derive `source_snapshot_id` only from the normative, nonrecursive `SOURCE_SNAPSHOT_IDENTITY_V1` canonical projection and reject any other preimage policy.
3. Permit multiple source collections and typed custody adapters in one graph.
4. When the Git adapter is selected with revision attestation, accept a repository root and expected revision, resolve the actual Git object ID, and reject mismatch before publication.
5. Record whether Git-admitted bytes match the resolved tree, differ as modified tracked content, or are untracked.
6. Support immutable materialization with an independently provisioned trusted expected manifest digest or valid signature and no VCS metadata.
7. Classify self-consistent but independently unanchored manifests as `INTEGRITY_VERIFIED` or `SELF_ASSERTED`, never `AUTHENTICATED`.
8. Never upgrade caller-supplied revision text to authenticated attestation evidence.
9. Keep adapter-specific fields outside generic graph and relation identity inputs; generic records reference snapshots and receipts by foreign key.

### FR3. Capability, execution, and qualification

Publish three distinct layers.

#### Protocol capability

For each provider execution, retain advertised support for:

- document symbols;
- prepare call hierarchy;
- incoming calls;
- outgoing calls;
- type hierarchy prepare;
- supertypes;
- subtypes.

#### Observed request status

For every requested relation family and affected seed/file, report:

```text
SUCCEEDED
PARTIAL
FAILED
UNSUPPORTED
NOT_REQUESTED
```

Include provider response or error evidence and request accounting. Retain the effective compiler configuration, inherited options, project references, declaration inputs, analyzer overrides, and their provenance for each execution; distinguish requested configuration from the configuration actually used. Copied caller revision metadata remains `CALLER_ASSERTED`, provider assertions remain attributable provider evidence, and independent authentication requires the §6 chain. A package name/version/path/hash binding alone is not authenticated origin.

Analysis-only defaults are not categorically prohibited. Their policy and version, effective values, changes from native configuration, and evidence scope must be explicit and evaluated under the qualification policy. Modified source, substituted declarations, or modeled configuration must not be accepted as an unmodified-native run.

#### Retained qualification status

For each language/provider/version/operation combination, report:

```text
PASS
BLOCKED
FAIL
NOT_QUALIFIED
```

Provider advertisement or one successful request must not imply retained qualification. Every qualification result must foreign-key `lsp-trace.qualification-policy.v1`, an independently governed and authenticated policy artifact defining the exact language/provider/version/operation tuple, required positive and negative observations, minimum real-server evidence, repetition or sample requirements, pass predicates, failure predicates, adjudicating authority, policy version, and qualification-artifact signature or custody receipt. `PASS` is valid only when the retained evidence satisfies every applicable predicate and the policy authority resolves through a verifier-side trust input not controlled by the evidence producer. Semantic validation recomputes the result and rejects producer-authored policy substitution, stale policy versions, insufficient evidence, and self-issued status labels.

### FR4. Multidimensional completeness

Publish independent dimensions for:

- request completion;
- bounded traversal completion;
- provider-relative graph completion;
- source-denominator coverage;
- relation-family coverage;
- source-manifest completion;
- publication/custody completion.

Completeness scope is closed and explicit: `OPERATION_SCOPE`, `SOURCE_SNAPSHOT_SCOPE`, or `SOURCE_COLLECTION_SCOPE`. Every completeness value records exactly one scope class and its canonical member selector. A narrower scope may never be relabeled, aggregated, or promoted as a broader scope; repository- or snapshot-level claims require the corresponding full admitted-manifest universe independently of the request selector. `source_graph_complete` denotes `SOURCE_SNAPSHOT_SCOPE` and remains `UNKNOWN` unless it foreign-keys a verified `lsp-trace.source-denominator.v1` artifact. That artifact must contain scope inputs, an authoritative scope-policy ID and version, derivation policy and version, canonical denominator members or an independently verifiable membership commitment, typed exclusions and reasons, covered-member mappings, counts, logical digest, semantic-validator result, and extractor qualification reference. The scope policy must derive its universe from independently admitted source-manifest classifications and requested operation scope; it must not define the denominator from provider returns, retained graph members, or other observed successes. The extractor qualification reference must resolve to `PASS` for the exact extractor, version, scope policy, and denominator-derivation operation. Membership commitments prove committed membership only; they establish completeness only when their opening or independently verified derivation proves equality with the authoritative scope-policy universe. An opaque or self-narrowed extractor assertion is not a coverage proof.

No aggregate `complete=true` may obscure an incomplete, unsupported, failed, partial, or unknown dimension. Semantic validation must reject any non-`UNKNOWN` source-completeness status whose denominator proof is missing, whose extractor qualification is not exact-match `PASS`, whose scope universe is not independently bounded, or whose members and typed exclusions do not exhaust that universe.

### FR5. Identity semantics

Define and test these layers:

1. **Node occurrence ID:** exact source collection and snapshot context, source receipt, path, range, and provider-reported symbol occurrence; revision context is optional typed attestation, not a Git prerequisite.
2. **Portable locator:** language-aware locator used only for best-effort matching; includes normalization method and version.
3. **Revision-scoped semantic ID:** identity within one authenticated snapshot under a named derivation policy, with an explicit revision-attestation policy when a VCS revision is claimed.
4. **Cross-revision correspondence:** explicit evidence-bearing relation with `MATCHED`, `AMBIGUOUS`, `REJECTED`, or `UNRESOLVED` status.
5. **Relation occurrence ID:** exact acquisition/evidence occurrence, including execution, snapshot context, optional revision attestation, acquisition method, and call-site occurrence where applicable.
6. **Logical relation ID:** deduplicated semantic endpoints, relation kind, direction, and explicit snapshot/revision policy; optional during evidence collection, but mandatory for every record explicitly published as a logical relation under FR6 and for its linked occurrences.

Collection may retain unresolved or ungrouped occurrences without asserting logical identity; such records must remain explicitly typed and must not masquerade as published logical relations. Git is an optional custody adapter, not an identity requirement. Absolute filesystem URI must not be the only source-collection identity input.

Existing node and relation IDs remain historical V2/V3 identities. Correct the documentation/code inconsistency by documenting existing IDs exactly and introducing new IDs at a new schema boundary.

### FR6. Normalized relation export

Publish `lsp-trace.relations.v1` as an independently versioned, authoritative projection of an admitted graph bundle.

Each logical relation record must include:

```text
logical_relation_id
relation_kind
semantic_source_node_id
semantic_target_node_id
stored_direction
source_collection_ids
source_snapshot_ids
revision_policy
evidence_receipt_ids
contribution_record_ids
support_group_ids
support_observation_count
independent_support_group_count
```

Each evidence occurrence must include:

```text
relation_occurrence_id
logical_relation_id
acquisition_method
source_node_occurrence_id
target_node_occurrence_id
source_snapshot_ids
source_receipt_ids
source_path
source_range
target_path
target_range
call_site_range
provider_id
provider_version
execution_bundle_id
capability_reference
request_status_reference
qualification_reference
completeness_reference
source_custody_reference
evidence_receipt_id
upstream_observation_ids
support_group_id
```

Call relations with multiple call sites must preserve one occurrence per distinct call site or an equivalent nested occurrence representation with canonical ordering and occurrence IDs.

Initial semantic relation kinds:

```text
CALLS
DISPATCH_ASSOCIATION
TYPE_RELATION
DOCUMENT_CONTAINS
```

Type relations must separately encode:

- semantic orientation;
- acquisition method (`SUPERTYPES` or `SUBTYPES`);
- direct, transitive, or unknown distance status.

`DOCUMENT_CONTAINS` may be emitted only from attributable hierarchical `DocumentSymbol` evidence. Flat `SymbolInformation` output does not establish hierarchy by itself.

The schema must define duplicate semantics, support counting, unresolved endpoints, canonical ordering, execution-bundle foreign keys, logical digest, and semantic validators. Acquisition-specific provenance belongs to each evidence occurrence or an occurrence-keyed contribution record; a logical relation may expose only aggregates canonically derived from all contributions. Semantic validation must reject heterogeneous occurrences whose provenance can be recovered only from one singular logical-record field.

### FR7. Support accounting

Do not use an ambiguous generic `support_count`.

Report separate counts where applicable:

- distinct evidence occurrences;
- distinct call sites;
- distinct executions;
- distinct providers/provider versions;
- distinct authenticated source receipts.

Co-bundled occurrences are not automatically independent support.

Every occurrence that contributes to a support claim must reference a canonical `support_group_id` and every attributable upstream observation through ordered `upstream_observation_ids`. `upstream_observation_id` is a domain-separated digest of a versioned canonical projection containing observation kind, admitted source/evidence receipt IDs, provider execution and acquisition-response identity where applicable, and parent upstream-observation IDs; the projection excludes its own ID and defines UTF-8 canonical JSON, key ordering, parent ordering, and cycle rejection. Upstream-observation records are authoritative foreign-key targets retained in the relation artifact or an immutable referenced evidence artifact.

Each group records its dependence basis, dependence kind, grouping policy and version, member occurrences, and relevant shared provider/execution/acquisition inputs. The schema defines a normative minimum dependence graph: occurrences are joined when they share a provider execution, acquisition response, generated evidence receipt, or canonical upstream-observation ID, and minimum dependence classes are the deterministic connected components of those unconditional edges, including transitive sharing through parent upstream observations. An independence policy may merge minimum classes conservatively but must never split one; CLI, MCP, and offline semantic validators must recompute identical classes from canonical foreign keys and reject missing targets, cycles, noncanonical ordering, inconsistent parent closure, or finer grouping. If required upstream provenance is unavailable, the occurrence may retain a typed `UPSTREAM_PROVENANCE_UNAVAILABLE` status but cannot contribute to `independent_support_group_count`. Publish that count only under a named validated independence policy whose groups are no finer than the recomputed floor. Raw occurrence, call-site, execution, provider, or receipt counts must never be labeled independent support.

### FR8. Projection contract

Every analysis must reference `lsp-trace.projection.v1`, declaring:

- included node and relation kinds;
- edge direction policy;
- multiedge collapse policy;
- edge-weight policy;
- self-loop policy;
- dispatch-edge treatment;
- unresolved-node treatment;
- excluded nodes and typed reasons;
- bounded-frontier treatment;
- conversion to undirected form, when applicable;
- exact normalized graph digest.

Changing any projection choice must change the projection digest. No command may silently simplify or reverse the graph.

### FR9. Deterministic traversal analytics

Provide offline operations for:

- directed BFS;
- incoming and outgoing reachability;
- bounded neighborhoods;
- shortest paths;
- capped path enumeration;
- explicitly defined path-diversity selection;
- weakly connected components;
- strongly connected components;
- condensation DAG.

Every path must preserve stored edge orientation and exact relation-occurrence witnesses.

Path enumeration must report configured caps, returned count, truncation status, and deterministic selection order.

Required-symbol acquisition under FR20 must reuse these shared directed path operations over its retained graph, not introduce a separate connection algorithm or infer missing edges. Path analysis remains offline; the acquisition coordinator owns any additional provider requests.

### FR10. Structural metrics

Provide:

- in-degree;
- out-degree;
- weighted degree;
- degree distribution;
- articulation points over an explicitly undirected projection;
- bridge edges over an explicitly undirected projection;
- betweenness centrality;
- optional HITS hub/authority scores.

Every result must identify its projection, denominator, disconnected-node policy, and approximation/resource policy. Hubs must be reported rather than silently removed.

Exact betweenness is required only below a configured resource ceiling. Approximate betweenness, if introduced, must use an explicit algorithm, deterministic seed, sample policy, error/coverage metadata, and distinct result status.

### FR11. PageRank and Personalized PageRank

Provide deterministic PageRank and PPR with:

- damping factor;
- seed occurrence or semantic IDs and seed weights;
- convergence tolerance and norm;
- maximum iterations;
- dangling-node policy;
- edge-weight policy;
- ordered summation policy;
- numeric representation and serialization policy;
- stable lexical tie-breaking.

Results must include convergence status, residual, iteration count, score sum, ranks, projection digest, and exact input graph digest.

Non-convergence fails closed unless explicitly permitted by a parameter retained in the result.

### FR12. Community diagnostics — deferred program

After the substrate and deterministic-analysis programs qualify successfully, evaluate:

- Infomap for directed-flow diagnostics;
- Leiden for an explicitly declared weighted projection.

Before implementation, approve:

- library and algorithm version;
- license compatibility;
- cross-platform build and packaging policy;
- random seed handling;
- input-order canonicalization;
- community-label canonicalization;
- repeated-seed instability diagnostics;
- resource ceilings.

Community labels are run-local. Schemas and CLI output must never call a community a feature.

### FR13. Boundary reports — deferred with communities

Given a qualified community artifact, report:

- intra-community and crossing relation occurrences;
- conductance or explicitly named equivalent;
- high-centrality crossing nodes;
- bridge edges and articulation points under the referenced projection;
- representative path witnesses;
- whether crossings pass through declared high-degree hubs.

Crossings remain structural observations, not business-boundary evidence.

### FR14. Analysis provenance

Every analysis artifact must include:

- schema version;
- tool version and build identity;
- runtime/architecture scope required for replay;
- input artifact and execution-bundle IDs;
- exact input graph semantic digest;
- source-manifest digest;
- projection ID and digest;
- algorithm and implementation version;
- complete parameters;
- resource limits and approximation status;
- output logical digest;
- warnings and incompleteness;
- deterministic replay command.

### FR15. CLI

Target command family:

```bash
lsp-trace relations export <graph>
lsp-trace analyze degree <relations> --projection <projection>
lsp-trace analyze components <relations> --mode weak|strong --projection <projection>
lsp-trace analyze paths <relations> --from <locator> [--to <locator>] \
  --direction incoming|outgoing|both --depth N --projection <projection>
lsp-trace analyze bridges <relations> --projection <projection>
lsp-trace analyze pagerank <relations> --projection <projection>
lsp-trace analyze ppr <relations> --seed <locator> [--seed <locator> ...] \
  --projection <projection>
```

Deferred commands:

```bash
lsp-trace analyze communities <relations> --algorithm infomap|leiden \
  --projection <projection>
lsp-trace analyze boundaries <relations> --communities <artifact>
```

All commands must support canonical JSON. Human-readable output must derive from the same result model.

The existing `slice` and `incoming` acquisition commands must expose FR20 required-symbol selectors, shared acquisition limits, and per-target coverage and connection results. Their adapters must use the same acquisition coordination and FR9 path semantics as the corresponding MCP operations; exact flag spelling belongs to the versioned parameter contract.

Before the command family grows materially, publish `lsp-trace.operation-registry.v1` and derive dispatch, help, capability reporting, and parity tests from it. Every operation entry contains a stable operation ID, lifecycle state, qualification state, required surfaces, parity applicability, input and result schema versions, parameter contract, resource contract, and publication modes. Lifecycle state is closed: `EXPERIMENTAL`, `SUPPORTED`, `DEPRECATED`, or `DISABLED`. `SUPPORTED` and `DEPRECATED` are parity-applicable and require implemented CLI and MCP surfaces over the same shared operation; `EXPERIMENTAL` is not a production-support claim; `DISABLED` is not exposed for execution. Every non-deferred operation required by FR9–FR11 must reach `SUPPORTED` before its delivery program is complete and cannot evade parity through lifecycle labeling. Runtime availability is a separate per-surface state: `ENABLED`, `RUNTIME_DISABLED`, `CONTAINMENT_UNAVAILABLE`, or `NOT_IMPLEMENTED`. `SUPPORTED` requires both required surfaces to be implemented, so `NOT_IMPLEMENTED` is invalid for either; environmental `RUNTIME_DISABLED` or `CONTAINMENT_UNAVAILABLE` may coexist with `SUPPORTED` only when the implementation is present, the disabling condition is reported, and capability metadata does not claim current executability. Registry validation rejects every other lifecycle, parity, required-surface, qualification, and runtime-state combination. Analysis logic must live in importable packages rather than CLI handlers.

### FR16. MCP analysis tools

Expose normalized relation export and every supported offline analysis operation through the `lsp-trace` MCP server.

MCP requirements:

1. MCP and CLI adapters must invoke the same shared operation packages and emit the same versioned result models; their exposed operation sets and status vocabulary derive from `lsp-trace.operation-registry.v1`.
2. MCP tool schemas must expose every projection choice, algorithm parameter, resource ceiling, and partial-result policy required by the corresponding CLI operation.
3. MCP tools must accept immutable published artifacts, selectors, or verified artifact bytes; they must not require a live language-server session after graph publication.
4. Custody, structural, semantic, foreign-key, and digest verification must run before analysis.
5. Tool capability metadata must separately report `ENABLED`, `RUNTIME_DISABLED`, `CONTAINMENT_UNAVAILABLE`, or `NOT_IMPLEMENTED` as applicable, without implying provider qualification.
6. Small canonical results may be returned inline. Results exceeding declared MCP response limits must be immutably published and returned by selector/receipt with byte length and digest; they must never be silently truncated.
7. Pagination or output selection may project a large admitted result for transport, but must preserve the authoritative artifact identity and disclose omitted sections, page accounting, and continuation state.
8. MCP defaults must not hide graph projection choices. Omitted required projection policy fails closed.
9. Tool names, schemas, publication support, and limits must be discoverable through the MCP capability surface.
10. MCP transport envelopes must not alter the logical digest of the underlying canonical result.

Initial MCP analysis family:

```text
lsp_trace_relations_export
lsp_trace_analyze_degree
lsp_trace_analyze_components
lsp_trace_analyze_paths
lsp_trace_analyze_bridges
lsp_trace_analyze_pagerank
lsp_trace_analyze_ppr
```

Deferred MCP tools:

```text
lsp_trace_analyze_communities
lsp_trace_analyze_boundaries
```

Exact public names may follow the repository's established MCP naming convention, but CLI and MCP operation semantics must remain equivalent.

The MCP `slice` and `incoming` acquisition operations must also expose FR20 required-symbol selection, limits, partial-result policy, and per-target resolution, expansion, and connection accounting. Capability discovery and input/result schemas must describe this support separately from offline path execution. CLI/MCP parity includes the combined acquisition result and replay of its path witnesses, not merely successful tool registration.

### FR17. Offline operation

After publication, relation export and every analysis must run without:

- source checkout;
- language server;
- network access;
- mutable cache;
- provider installation.

Offline operations must verify custody and all referenced input digests before execution.

### FR18. Qualification

Retain reviewable qualification by exact language, provider, and version for:

- C#;
- JavaScript separately from TypeScript;
- TypeScript;
- ElixirLS;
- each Elixir companion provider as a separately identified source.

Qualify, where supported:

- incoming calls;
- outgoing calls;
- bounded slices;
- document symbols;
- type hierarchy;
- V3 publication and verification;
- source-manifest custody;
- normalized relation export;
- at least one deterministic analysis replay.

A fixture does not establish PASS. A companion provider must not replace, rewrite, or upgrade blocked native-provider evidence.

Report these separately: artifact delivery (a result was retained), graph coverage (under its declared denominator and bounds), deployment integrity (the executed binary/provider/configuration matches the deployment claim), synthetic or modeled qualification (predicates passed in that declared environment), and native acceptance (the exact native-input/provider/operation policy is satisfied). None substitutes for another. Native acceptance requires retained effective compiler configuration, inherited options, project references, declaration inputs, and analyzer overrides, including their origins and changes. Modified or modeled inputs are not unmodified-native acceptance; analysis-only defaults are admissible only within an explicit policy and evidence scope, not by retroactively treating every default as forbidden or silently native.

### FR19. Cross-family domain neutrality

Domain neutrality is a semantic invariant over every public schema, field, enum, diagnostic, relation, projection, path, component, metric, ranking result, community result, boundary report, CLI rendering, MCP envelope, error, and generated documentation artifact. Public output must not assert product-feature, service, business-entity, ownership, or business-boundary identity from structural evidence. Opaque caller-supplied annotations may contain domain terms only when provenance marks them `CALLER_ASSERTED` and `NON_AUTHORITATIVE`.

Every public artifact family and transport requires positive and negative neutrality fixtures enforced by semantic validators and acceptance tests.

### FR20. Required-symbol acquisition and connection accounting

`slice` and `incoming` must accept caller-selected required symbol locators alongside a primary root. The operation must attempt resolution and bounded expansion of those targets, retain the resulting evidence, and reuse the shared FR9 directed path operation to report witnessed connections within the acquired graph. Required means **attempted and accounted for**, not guaranteed resolution, expansion, or connectivity. Presence in one graph does not establish a path.

1. **Exact selection:** accept source-qualified symbol-name or positional locators with explicit coordinate conventions and mutually exclusive selector forms. Retain requested locators and validated resolved node identities. Report missing, ambiguous, unsupported, failed, and not-attempted targets without selecting an arbitrary namesake or treating spelling as identity. Multiple distinct prepared identities require explicit disambiguation. Duplicate locators or aliases may share work under a declared policy, but every requested target retains an accounting record.
2. **Separate outcomes:** independently report each target's resolution, expansion by requested direction, and connection disposition. Expansion must distinguish successful-empty, successful-nonempty, partial, frontier/not-expanded, failed, unsupported, and budget-blocked outcomes as applicable, with attributable request evidence and limits. Discovery alone is not successful expansion; a failed request is not successful-empty. An unresolved or ambiguous endpoint makes its path query not evaluable, rather than disconnected.
3. **Shared path semantics:** compute a deterministic directed shortest-path witness for each evaluable root/target pair using FR9 over the combined retained graph. Retain the connecting intermediate nodes, relation occurrences, and provenance references in the returned evidence or resolvable immutable artifacts. Do not synthesize edges from co-membership, names, callback expectations, or requested inclusion. Alternative paths, when requested, use FR9's existing capped enumeration and selection policy rather than a new method.
4. **Explicit direction:** for `slice`, the default connection query is primary root → required symbol; for `incoming`, it is required symbol → primary root. Record the effective path direction and projection. Stored CALLS edges remain caller → callee regardless of acquisition direction. Slice membership through outgoing-then-incoming traversal is not itself a directed connection witness. A target resolving to the root may have a zero-hop witness without implying expansion succeeded.
5. **One bounded acquisition:** coordinate targets within one declared acquisition context, retaining exact provider execution/session-generation references where applicable. Root and required targets share declared global node, request, time, and evidence limits; per-target or per-direction limits and deterministic allocation rules must also be explicit. Adding targets must not silently multiply global budgets. Unattempted or unserved targets remain visible with reasons. A shared acquisition context does not imply frozen source bytes, compiler consumption, or authenticated analyzed-source identity.
6. **Scoped negative results:** report a witnessed path when found. Only an exhausted path search over the admitted retained graph may report not-found-in-retained-graph; path-search resource exhaustion yields incomplete, not not-found. Independently preserve acquisition partiality and provider omissions even when the offline graph search is exhaustive. Neither outcome establishes source-level or runtime unreachability. No aggregate success or completeness flag may hide these separate dispositions.
7. **Preservation and replay:** preserve coverage, graph records, source/evidence receipt references, and path witnesses through publication and the supported normalized-export/analysis pipeline. Derive mandatory coverage from the retained requested-target set, not only successful resolutions; semantic validation must reject missing target records and inconsistent identities, statuses, or path foreign keys. Introduce an explicit additive or versioned boundary where existing single-root contracts cannot represent the acquisition; do not silently relax historical admission or require consumers to discard coverage to analyze the graph. Published evidence must support offline path replay without reacquisition. Consistency verification does not independently authenticate provider or acquisition claims.
8. **Surface parity and authority:** CLI and MCP expose equivalent selectors, bounds, outcomes, and path semantics, including inline and immutable publication. Capability/schema discovery reports actual supported operation versions. Existing custody, qualification, neutrality, compatibility, and program-admission obligations continue to apply; this requirement does not itself authorize deployment, upgrade weaker provenance, or waive analytical admission gates.

9. **Seed manifest:** expose a versioned manifest input carrying the primary root, ordered required targets with unique caller IDs, exact locators, and declared shared/per-target limits. CLI file input and equivalent MCP structured input must resolve to the same effective request; do not implicitly read server-side paths supplied as MCP strings. Specify defaults and reject conflicting inline/manifest options, duplicate IDs, malformed selectors, and oversized manifests before acquisition. A structurally valid manifest retains every target's outcome even when individual resolution or expansion fails; successful targets must not disappear because another target fails. Manifest order and effective allocation policy remain replayable.

### FR21. Bounded retained-source context export

Provide an explicitly requested, deterministic offline view and export of the evidence retained for an admitted artifact, with reviewable source spans for caller-selected retained nodes, relationships, and declarations. Native and provider/sidecar evidence are first-class inputs under explicitly supported artifact contracts. Reuse admitted retained source bytes and provenance rather than reacquiring source. This requirement adds evidence presentation and context packaging, not feature-boundary selection, semantic relevance judgments, adjudication, or automatic discovery of containing language/framework constructs. An illustrative interface is `inspect ARTIFACT --hydrated`; final syntax remains an interface decision. The intended workflow is trace → inspect retained evidence → bounded review.

1. **Retained inputs and authority:** declare the admitted artifact families and versions, bind the exact input digest, and preserve selected record IDs, receipt references, content hashes, and supply/capture classifications. Do not fall back to a checkout, provider, or network when bytes are absent. Successful document supply does not prove compiler consumption or analyzed-version identity; post-traversal captures remain separately attributed. Packaging must not upgrade custody or completeness claims.
2. **Explicit selection and boundaries:** accept explicit source-bound spans and retain every requested origin. Expansion to a complete containing function, declaration, enclosing comment, or template boundary requires an explicit request and an attributable boundary tied to the exact retained source version. Language/framework boundary discovery remains provider-owned; caller-supplied boundaries remain `CALLER_ASSERTED` and `NON_AUTHORITATIVE`. A CallHierarchyItem range or structurally valid range alone does not establish a complete containing function. “Relevant declarations” means an explicit list or named, recorded selection policy, not an inferred semantic judgment. Unknown boundaries remain explicit rather than guessed.
3. **Deduplicated spans and origin mappings:** deterministically union overlapping half-open byte intervals within the same source identity and coordinate encoding, preserving mappings from every selected node, relationship, declaration, and requested span to the exported intervals. URI equality alone is insufficient for merging. Different same-URI versions must remain distinct; identical bytes may share storage only if their separate receipt and acquisition provenance remain recoverable.
4. **Byte-exact coordinates:** retain original coordinates, declared UTF-8/UTF-16/UTF-32 position encoding, and corresponding original-source byte offsets. Validate conversions against the exact retained bytes; preserve BOM and line-ending bytes, including CRLF. Reject out-of-bounds coordinates, split code points or surrogate pairs, incompatible encodings, and mismatched coordinate/byte mappings. Missing encoding is not permission to guess.
5. **Bounds and omissions:** declare input/output byte, span-count, work, and applicable time limits, deterministic selection/allocation order, and partial-result policy. Account for every requested origin, including absent bytes, unknown boundaries, privacy exclusions, invalid coordinates, and budget failures as distinct dispositions. Empty readable content is not unavailable content. Never silently truncate a span or label a fragment complete. A complete-context claim requires the whole requested span and, for containing-construct claims, the declared attributable boundary; it does not establish source, graph, or analyzed-version completeness.
6. **Replay, privacy, and compatibility:** expose equivalent CLI/MCP selection, limits, omission accounting, inline output, and immutable publication through an explicit additive or versioned contract. Structural and semantic validation must check hashes, coordinate mappings, source-version joins, origin coverage, unions, and completeness claims. Identical admitted bytes and parameters must replay without source access. Source-body inclusion remains explicitly opt-in under §12. Existing identity, qualification, neutrality, and program-admission obligations remain unchanged. Adoption of this requirement does not claim implementation, authorize deployment, or add automatic semantic context discovery to scope.

7. **Stable paging:** bind pages to one immutable input/selection/policy/result snapshot with explicit page ordering, continuation, total span/origin counts, and completion metadata. Reject stale or mismatched continuations. Exhaustive page retrieval must reconstruct the same logical context bundle as unpaged export, with no missing or duplicate spans or origin mappings; transport pagination must not restart acquisition, reset global export budgets, or imply source completeness. Preserve whole-span semantics: if a span cannot fit the declared page limit, report an explicit omission or an immutable whole-span reference under the declared policy, never an unlabeled fragment.

8. **Hydrated evidence inspection:** provide both human-readable and machine-readable artifact-centered views. Within the declared selection and bounds, account for every retained source fragment and evidence record, including native records and provider/sidecar contributions. Preserve and display origin, authority, file/source identity, available revision and content hash, exact range and coordinate encoding when present, and attributable links to the relationships supported or claimed by each fragment. Missing revision/range/hash information stays explicitly unavailable; no inferred value may fill it. A fragment's attribution is not independent proof of the relationship it accompanies.
9. **Evidence states and authority separation:** distinguish retained bytes from reported references without bytes, unavailable or missing context, truncated retained context, and view-level budget omissions. Show shared fragments once under FR21's source/version-scoped deduplication policy, with references from every associated relationship and separate authority/provenance records intact. Label sidecar claims as sidecar claims; never convert them into native server-reported CALLS, independent support, or stronger custody. Unsupported evidence families must be explicitly rejected or accounted as unsupported, never silently dropped. Hydration must perform no working-tree, provider, or network acquisition. Resolving references that require new acquisition is a separate explicitly requested operation with its own provenance; it must not mutate or retroactively complete the original artifact.

10. **Focused public review selection:** CLI/MCP must accept explicit native node and relationship IDs and deterministically resolve their attributable retained source-bearing records into an exposed, digest-bound selection manifest. Default to retained ranges; whole-file expansion and source-body inclusion each require explicit opt-in. Preserve receipt identity, source classification, coordinate authority, native/sidecar distinctions, and mappings back to every requested ID. Native relationship selection must retain attributable call-site context, not merely endpoint declarations. Unsupported, ambiguous, or unresolvable selections receive explicit dispositions rather than disappearing.
11. **Bookkeeping versus missing context:** exclude explicitly NON_SOURCE bookkeeping from focused source selection without deleting or relabeling it in the underlying artifact. NON_SOURCE is not UNKNOWN_SOURCE, missing bytes, or failed source resolution and must not generate missing-source warnings merely because it has no source receipt. Preserve the exhaustive catalog as an advanced inspectable diagnostic surface. Focused output must exclude unrelated acquisition bookkeeping while retaining the provenance references needed to validate its selected evidence, and must report measured output bytes separately from unique retained source bytes. Existing offline, privacy, resource, and snapshot-paging rules apply unchanged.

### FR22. Discoverable pipeline compatibility and empty-result contracts

Publish a machine-readable, version-bound compatibility matrix and corresponding documentation for acquisition → provenance → retained export → analysis. Each supported transition must name exact operation and input/output family versions, relevant limits and prerequisites, and CLI/MCP production and consumption surfaces. Distinguish internal-only implementation, public production, public consumption/validation, and observed deployed availability; schema validation support alone does not imply an artifact can be publicly produced or analyzed. Deployment observations must identify the inspected instance/version and observation scope; absent live inspection is unknown, not deployed support. Unsupported transitions must fail explicitly rather than silently convert families or discard coverage.

Document nullability per artifact family/version, including valid empty and partial-empty native graphs. Preserve historical native `nodes: null` / `edges: null` behavior where admitted by that contract; consumers must not infer failure from nullable collections. Normalized retained exports must provide their declared collection tables as arrays, including `[]` for empty collections, without altering embedded historical graph bytes. Failure and unresolved-seed dispositions remain distinct from successful empty acquisition.

Audit existing capability/completeness requirements and public surfaces before adding new semantics: separately expose native relation support, installed/enabled framework adapters, unsupported boundaries, observed execution, and qualified coverage under their exact policies. Zero candidates or an empty graph must not imply absence of relationships or exhaustive source coverage. Retain discovered gaps as explicit acceptance work; this requirement does not claim that every existing surface is deficient.

### FR23. Bounded managed preflight and failure diagnostics

Provide a shared machine-readable diagnostic receipt or explicitly enabled bounded diagnostic output for managed acquisition failures, with equivalent CLI/MCP classifications. Preserve historical evidence output and error compatibility through an additive or versioned boundary; diagnostics must not corrupt stdout evidence or turn initialization failure into a successful empty graph.

- Identify the observed phase: spawn, initialize-write, initialize-response, document-supply, or capability-check. Distinguish process exit, timeout, transport error, protocol error, unsupported capability, and unknown causes without inferring unobserved causality. Unresolved seeds and successful empty results remain separate acquisition outcomes, not initialization errors.
- Record observed exit status/signal, elapsed time and declared limits when available; represent unavailable or unobserved fields explicitly. Distinguish configured launch/runtime metadata from independently observed runtime versions.
- Bind a safe launch/profile identity and redacted configuration/environment provenance. Any retained stderr or richer diagnostic reference must be explicitly enabled, access-controlled under the declared host policy, bounded in bytes/time/storage, and report truncation or collection failure. Do not dump credentials, source bodies, or raw sensitive arguments/environment by default; arbitrary stderr cannot be assumed safe merely because it is bounded.
- Diagnostic collection must respect cancellation and resource limits, preserve managed transport ownership and safe teardown, and never block indefinitely to obtain exit or stderr details. The current private startup sink performs synchronous forensic publication bounded to 64 records and 65,536 serialized bytes; filesystem publication may add latency and is not claimed nonblocking. Publication remains secondary and cannot override the primary failure. No startup retry or timeout changes follow.
- **Per-request preparation accounting:** retain bounded diagnostics for every initialization and `textDocument/prepareCallHierarchy` request in graph-provenance v2 acquisition accounting and equivalent public CLI/MCP results. Bind the method, target ID, deterministic request sequence, available protocol request ID, observed start/end timestamps, elapsed duration and terminal state: request not written, request written awaiting response, response received and matched, response received but unmatched, response received after timeout/cancellation, protocol error, transport closed/unreadable, cancelled, deadline exceeded, or process exited. Retain request write start/completion, response-read and dispatch state, unmatched/late response accounting, observed process exit/lifecycle state, transport read/write-loop state at termination, relevant negotiated capabilities including call-hierarchy support, and whether document preparation completed before the request. Distinguish process state observed during the request from cleanup-induced exit; do not infer root cause from timeout, EOF, or empty stderr. Missing facts, byte/time ceilings, truncation, withholding, and diagnostic-collection failure remain explicit. Partial graph output remains valid and replayable; no automatic retry, timeout increase, or completeness promotion is permitted.
- **Initial delivery scope:** retain safe structured phase/reason codes, observed timing, limits, transport/lifecycle state, request identifiers, negotiated capability facts, and exit facts, with raw server stderr, raw errors/RPC messages, environment values, and arguments withheld. Report withheld diagnostic content explicitly; bounded capture alone does not make content safe to release. Bounded stderr capture status, limits, truncation, and missing/withheld disclosures may be retained, but release of stderr content or a content-bearing reference requires a separately approved host access/redaction policy and is deferred.
- **Delivery boundaries:** provide an opt-in restricted CLI diagnostic file and equivalent host-configured MCP bootstrap sink. **Current FR23 delivery is deliberately narrower:** only the explicit private CLI startup sink `managed-startup-diagnostics/v1` is implemented; no MCP sink, registry/public schema, default write, provider qualification, or deployment claim follows. Bootstrap may fail before MCP serves requests; do not promise a tool response in that case. Live MCP access uses an explicit versioned structured surface bound to the exact session/generation, with bounded retention and explicit unavailable/evicted outcomes, rather than changing historical error envelopes silently. Diagnostic destinations must reject stdout, unsafe paths/symlinks, and collisions with evidence output; failed delivery must preserve the primary failure. No default diagnostic file writes.
- Use deterministic local servers to qualify classifications. This requirement neither attributes an observed Elixir initialization failure to lsp-trace nor authorizes dependency installation, source mutation, or automatic environment/server repair. Recording this initial scope is PRD adoption only, not implementation or deployment approval; public FR20 integration and FR21 context export remain the preceding priorities unless maintainers explicitly reprioritize.

## 8. Schemas

### Trustworthy substrate

```text
lsp-trace.source-manifest.v1
lsp-trace.capabilities.v1
lsp-trace.qualification-policy.v1
lsp-trace.qualification-matrix-profile.v1
lsp-trace.completeness.v1
lsp-trace.source-denominator.v1
lsp-trace.relations.v1
lsp-trace.projection.v1
lsp-trace.operation-registry.v1
```

### Deterministic analysis

```text
lsp-trace.paths.v1
lsp-trace.components.v1
lsp-trace.centrality.v1
lsp-trace.pagerank.v1
```

### Deferred community program

```text
lsp-trace.communities.v1
lsp-trace.boundaries.v1
```

Schema changes must be additive or receive a new major version. Every family requires structural validation followed by family-specific semantic validation.

## 9. Determinism contract

Determinism is scoped to identical:

- admitted input bytes;
- source-manifest and graph digests;
- projection artifact;
- parameters and resource ceilings;
- algorithm and implementation version;
- declared runtime/architecture constraints where numeric equivalence requires them.

Within that scope:

- canonical JSON bytes are reproducible;
- map iteration cannot affect results;
- node, edge, and occurrence ordering is stable;
- floating-point summation and serialization are specified;
- non-finite values are rejected;
- negative zero is canonicalized;
- ties use lexical stable IDs;
- randomized algorithms require and record a seed;
- replay reproduces the output logical digest.

Provider acquisition remains nondeterministic evidence collection and is separated from deterministic offline analysis.

## 10. Resource policy

Every potentially expensive operation must define and record applicable limits:

- maximum admitted nodes and relation occurrences;
- maximum traversal depth;
- maximum paths and path length;
- maximum iterations;
- timeout and memory policy;
- exact-versus-approximate threshold;
- partial-result policy.

A reached resource ceiling produces typed incomplete or failed output. It must not silently change algorithms, sample inputs, or truncate results.

## 11. Failure semantics

Fail closed on:

- expected/resolved revision mismatch;
- invalid or unauthenticated required source manifest;
- unauthenticated contributing source files;
- stale graph, relation, projection, or analysis digest;
- unsupported explicitly required capability;
- malformed or semantically invalid artifact;
- ambiguous locator where exact selection is required;
- absent projection policy;
- non-converged PageRank unless explicitly permitted;
- unavailable community implementation;
- digest or replay mismatch;
- exceeded resource ceiling where partial output was not explicitly permitted.

Partial output may be retained only with typed status, denominator accounting, limits, and warnings.

## 12. Security and privacy

- Do not embed source bodies in analysis artifacts unless explicitly requested.
- Prefer repository-relative canonical paths.
- Treat absolute paths, diagnostics, source excerpts, provider opaque data, and arguments as sensitive.
- Source manifests normally contain hashes and locations, not source bodies.
- Offline verification and analysis must not contact providers or external services.
- Signatures and trusted expected digests must identify their trust policy and verification method.

## 13. Delivery plan

### Program A: Trustworthy structural substrate

#### A0. Identity and semantic corrections

- Correct the documented relation/revision inconsistency.
- Publish an identity ADR defining all identity layers.
- Freeze historical V2/V3 identity semantics.
- Add migration and compatibility tests.

#### A1. Source snapshot custody and revision attestations

- Implement the VCS-neutral source collection, snapshot, and receipt model.
- Implement Git worktree and immutable-manifest custody adapters.
- Publish complete contributing-source manifests.
- Add snapshot and source-receipt foreign keys.
- Add multi-collection, mutation, dirty-state, exclusion, and revision-mismatch tests.

#### A2. Capability and completeness

- Separate advertisement, observed request outcome, and retained qualification.
- Publish multidimensional completeness.
- Preserve `source_graph_complete: UNKNOWN` unless proven by a qualified denominator.
- Add FR20 required-symbol selection and per-target resolution/expansion accounting under shared acquisition limits.

#### A3. Normalized relations and projections

- Publish lossless logical relations and evidence occurrences.
- Define support accounting and type acquisition semantics.
- Publish projection artifacts and digests.
- Add export reconstruction and semantic-validation tests.
- Preserve FR20 requested-target coverage, graph/source joins, and connection witnesses through versioned export and offline replay.

#### A4. Substrate qualification matrix gate

Publish `lsp-trace.qualification-matrix-profile.v1`, an independently governed normative profile listing mandatory axis members, mandatory Cartesian products, permitted equivalence classes, profile authority, version, and authenticated custody receipt. The profile must include every initial relation family, both initial custody adapters, every required status class, every materially distinct projection class, CLI and MCP, inline and immutable publication, and required provider/provider-version classes; generated evidence cannot select or reduce these obligations. Publish a versioned Program-B entry matrix plus a canonical required-cell generator bound to that profile. The generator defines explicit axes for initial relation family, custody adapter, required complete/partial/failed/unsupported/unknown state, materially distinct projection class, CLI/MCP transport, inline/immutable publication mode, and provider/provider-version class. Its versioned rules state which axes form a Cartesian product. A cell covers exactly one generated tuple. Any equivalence reduction must identify the omitted tuples, state a versioned equivalence rule, and carry retained evidence proving that the exercised tuple validates the same behavior; labels that merely list multiple axis values are not cells. The matrix validator recomputes the required tuples and rejects missing, duplicate, composite, or unsupported reductions.

The gate defines a versioned set of non-waivable foundational cells, including independently anchored custody, source-receipt foreign keys, identity compatibility, occurrence-provenance reconstruction, denominator validation with exact-match `PASS` extractor qualification, minimum support-dependence grouping, operation-registry parity, and neutrality validation for every artifact family and transport published by the Program-B admission milestone. Deferred families are outside this cell until publication; each deferred family has an equivalent non-waivable neutrality gate before its schema, CLI, MCP, or generated documentation becomes public. Other cells may carry an `APPROVED_WAIVER` only when the waiver names its exact generated tuples, rationale, an approving principal authorized by an independently authenticated verifier-side waiver policy, distinct from the artifact producer and evidence producer, with a foreign-keyed policy provisioning receipt, a bounded expiry or objectively testable revalidation condition, and specific blocked claims and operations. A waiver cannot claim `PASS`, cannot hide `BLOCKED`, and is invalid after expiry. Validators and command admission must enforce the blocked claims and operations. Companion-provider evidence cannot replace a native-provider cell.

`PROGRAM_B_ADMITTED` is the single gate predicate used by both Program-B entry and first-milestone completion. It is true only when every non-waivable cell is `PASS`, every other required cell is `PASS` or has a currently valid `APPROVED_WAIVER`, all required real-server evidence is retained, and no requested Program-B operation depends on a claim or operation blocked by a waiver. Program B remains disabled otherwise.

B05 provider admission's `all_requested_relations_supported` flag is not `PROGRAM_B_ADMITTED`. No equivalence may be claimed without a checked mapping from the exact B05 evidence and policy versions to the normative profile's generated tuples and pass predicates. Provider support is not substrate acceptance.

Generic graph algorithms have no blanket Ember prerequisite. The independently governed profile must retain every non-waivable foundational cell and specify operation-specific qualified dependencies; native-provider requirements cannot be removed by an implementation or replaced by companion evidence. A generic operation may proceed only after `PROGRAM_B_ADMITTED` validates for its declared scope and no dependency is blocked. This clarification is not an alternative admission route or a waiver of AC12.

### Program B: Deterministic structural analysis

#### B1. Graph primitives

- degree metrics;
- weak and strong components;
- condensation DAG;
- directed BFS, reachability, and shortest paths, reused for FR20 required-symbol connection witnesses;
- capped path enumeration;
- bridges and articulation points under explicit undirected projections.

#### B2. Ranking

- PageRank;
- Personalized PageRank;
- convergence and numeric contracts;
- multi-seed tests;
- deterministic replay.

#### B3. CLI and MCP adapters

Expose each qualified analysis operation through thin CLI and MCP adapters over the same shared packages. Retain parity fixtures proving equivalent canonical result models and logical digests. Exercise inline and immutable-published MCP result paths. Include FR20 acquisition-to-path composition and its separately reported coverage outcomes; do not implement a second path algorithm in either adapter.

#### B4. Cross-language qualification

Retain V3 evidence for C#, JavaScript, TypeScript, ElixirLS where available, and separately identified companions. Blocked providers remain blocked.

### Program C: Community diagnostics

Run a library, licensing, portability, determinism, and resource spike. Only then decide whether to implement Infomap, Leiden, community boundaries, and instability diagnostics.

## 14. Acceptance criteria

### AC1. Complete authenticated source custody

For a traversal discovering evidence in five non-seed repository files, all five appear in the source manifest and every dependent record resolves to their receipts. The snapshot reaches `AUTHENTICATED` only when its verification chain terminates in an anchor obtained by the verifier from an independently authenticated trust-policy input and resolves a valid provisioning receipt; an anchor introduced or controlled solely by the claimant or presented bundle is rejected. The same self-consistent manifest without that evidence remains `INTEGRITY_VERIFIED` or `SELF_ASSERTED`.

### AC2. Revision-attestation mismatch

When Git revision attestation is requested, an expected revision differing from the resolved Git object fails before publication. A valid non-Git snapshot requires no Git metadata.

### AC3. Materialized-byte honesty

Every admitted artifact is represented by its actual content hash. A Git-modified tracked file additionally records its modified state and is never represented as matching the clean Git tree.

### AC4. Honest completeness

A provider omission yields provider-relative status plus source completeness `UNKNOWN` or `PARTIAL`, never global complete. Operation-scoped completeness is labeled `OPERATION_SCOPE` and cannot satisfy or be rendered as `source_graph_complete`. Any non-`UNKNOWN` source completeness references a validated, digest-bound denominator-proof artifact whose independently bounded scope universe is exhausted by members and typed exclusions and whose exact extractor/version/scope-policy qualification resolves to `PASS`. References resolving to `BLOCKED`, `FAIL`, or `NOT_QUALIFIED`, self-narrowed scopes derived from provider returns, and unopened commitments fail semantic validation.

### AC5. Identity separation

Moving a line changes occurrence identity where required but does not silently assert or deny semantic correspondence. Cross-revision matches require explicit correspondence evidence.

### AC6. Relation identity

Changing revision context, execution evidence, acquisition method, or call site changes the relation occurrence ID. Logical identity remains separate and follows its documented revision policy.

### AC7. Lossless export and support provenance

Normalized export reconstructs all retained call sites, calls, dispatch/type/document associations, endpoints, occurrence-level provider/request/capability/qualification/completeness/custody provenance, and support-group membership without implementation-specific collections. Semantic validation resolves every `upstream_observation_id`, rejects missing or cyclic parent graphs, and recomputes normative minimum dependence classes as deterministic connected components over shared execution, response, receipt, and transitive upstream-observation edges. No grouping policy may split a class, unavailable required upstream provenance cannot contribute to independent support, and no raw diversity count is represented as independent support.

### AC8. Projection transparency

Every analysis references a projection artifact. Changing direction, weight, multiedge, unresolved-node, or frontier policy changes its digest.

### AC9. Path correctness

Directed shortest paths retain stored orientation and exact relation-occurrence witnesses.

### AC10. PPR determinism

Repeated offline PPR under the declared replay environment produces identical canonical bytes and logical digest.

### AC11. Resource honesty

A resource ceiling yields a typed incomplete/failure result with exact limit accounting and no silent fallback.

### AC12. Substrate qualification matrix

Program B remains disabled until `PROGRAM_B_ADMITTED` validates. The canonical generator must produce the expected atomic tuples; composite cells, missing tuples, unsupported equivalence reductions, and waivers on foundational cells are rejected. Every non-waivable cell is `PASS`; every other cell is `PASS` or has a valid authorized waiver whose blocked claims and operations are enforced. Required provider/language/version evidence remains real-server evidence, blocked providers remain visible, and companion evidence does not replace native-provider cells.

### AC13. Cross-family domain neutrality

No public artifact family or surface asserts product-feature, service, business-entity, ownership, or business-boundary identity from structural evidence. Semantic validation and negative fixtures cover relations, projections, paths, components, metrics, rankings, communities, boundaries, diagnostics, CLI, MCP, errors, and generated documentation.

### AC14. Compatibility

Existing V2/V3 artifacts remain readable and preserve their original IDs and semantics.

### AC15. MCP analysis parity

For identical admitted input, projection, parameters, and implementation version, CLI and MCP execution produce the same canonical result model and logical digest. Registry validation rejects a `SUPPORTED` operation with a missing required implementation; environmental runtime disablement remains distinct from implementation support and must not claim current executability. An oversized MCP result is returned through immutable publication with verified byte length and digest, never silent truncation.

### AC16. Required symbols and witnessed connections

For both `slice` and `incoming`, caller-selected required symbols are attempted under shared acquisition limits and every request retains independent resolution, expansion, and connection accounting:

- **Connected:** the root and required implementation resolve, and provider-reported edges establish a directed chain. The result includes its intermediate nodes and exact retained relation-occurrence witnesses, matching the shared FR9 path result on the same admitted graph and parameters.
- **Unresolved callback connection:** `StartImport` and a caller-selected concrete `ImportSurveyFromWorkbook` implementation both resolve and expand, but no connecting callback edge is reported. Both targets remain represented; no edge is invented. An exhaustive offline search reports no connection in the retained graph, with acquisition incompleteness preserved separately. Caller-provided names do not establish business identity or callback semantics.
- **Direction and identity:** reverse-only connectivity does not satisfy the requested direction. Ambiguous namesakes are not arbitrarily selected, aliases preserve every request's accounting, and a resolved root-equal target may have a zero-hop witness without an expansion-success claim.
- **Partial acquisition and bounded search:** missing, unsupported, failed, frontier-bound, and budget-blocked targets remain explicit; adding targets never silently multiplies the global budget. Acquisition success, expansion success, graph-search exhaustion, and path existence remain distinct. A path-search limit yields incomplete, never not-found.
- **Compatibility and replay:** equivalent admitted evidence and parameters yield matching CLI/MCP coverage models and path witnesses. Omitted required-symbol options preserve historical behavior and identities. Published coverage survives supported export and validates/replays offline without the source checkout or server. Missing target records, substituted endpoints, lost intermediate witnesses, reversed paths, and inconsistent statuses fail semantic validation.

### AC17. Bounded retained-source context export

- **Offline and version-bound:** after deleting or changing the checkout, equivalent admitted artifacts and parameters yield identical context bytes and logical digests. A supplied version A and later captured version B at the same URI remain separately attributed; neither is silently chosen as the analyzed version.
- **Selection and deduplication:** overlapping, nested, repeated, and disjoint spans preserve exact bytes and every requested origin mapping. Sharing storage never erases distinct source provenance. Identifier-only ranges and absent boundaries cannot produce a complete-function or complete-template claim; caller-asserted boundaries retain their authority label.
- **Coordinates:** UTF-8/UTF-16/UTF-32 fixtures cover non-BMP characters, BOM, LF/CRLF, empty files, and exact endpoints. Tampered hashes, source-version references, split characters, and inconsistent coordinate/byte mappings fail validation.
- **Omissions and budgets:** privacy opt-out, missing bodies, unknown boundaries, invalid coordinates, and exact byte/span/work budget edges produce distinct, exhaustive accounting. No dropped origin, silent truncation, or fragment labeled complete is accepted. Whole-span completion remains separate from graph/source completeness.
- **Surface parity and neutrality:** CLI/MCP inline and published results agree and replay offline. Historical artifacts and default behavior remain unchanged. No result asserts feature boundaries, business identity, semantic relevance, or adjudication from the exported context.

- **Hydrated artifact coverage:** a mixed native/provider/sidecar fixture renders every selected retained fragment and evidence record with origin and authority labels, exact available source anchors, and relationship references. Shared fragments render once without losing distinct provenance or counting duplicate support. Retained bytes, references without bytes, unavailable/truncated context, and view-budget omissions remain distinguishable in human and machine output. Missing metadata is explicit. Sidecar claims cannot become native CALLS through rendering, deduplication, or export.
- **No implicit hydration acquisition:** inspect after changing/deleting the checkout and with providers/network unavailable; retained output remains stable under snapshot paging. Reference-only records stay reference-only. Any separately authorized reference acquisition creates new provenance without rewriting the original artifact. Unsupported families and omitted selections remain visible rather than disappearing.

- **Focused review replay:** qualify explicit native node/relationship selection through CLI and MCP with identical exposed selection manifests and equivalent validated evidence. Requested relationships remain identifiable with their retained call-site context; unknown IDs, unsupported selection types, missing contexts, and NON_SOURCE records have distinct outcomes. Whole-file/body opt-in recovers exact retained bytes; default selection uses retained ranges without implicit source acquisition. Exhaustive catalog inspection remains available.
- **D01 regression target (reported, not yet qualified here):** the supplied trial reports seven audited relationships, five files totaling approximately 47 KB of unique source, a 419 KB all-record text view, and 116 UNKNOWN_SOURCE origins for explicitly NON_SOURCE metadata. Retain the exact artifact, baseline file hashes, selection/formatting parameters, and byte-count conventions before asserting replay acceptance. With checkout access denied, verify all seven relationship contexts, byte-identical whole-file recovery of the five baseline files, absence of misleading missing-source warnings for NON_SOURCE bookkeeping, CLI/MCP equivalence, and explicit unresolved selections. Measure focused output against the pinned all-record baseline and explain retained overhead; do not claim a universal reduction ratio or silently trim required evidence. These tests establish neither production-sidecar qualification, runtime behavior, nor inventory admission.

### AC18. Manifest, paging, compatibility, and diagnostics ergonomics

- **Manifests:** equivalent CLI/MCP inputs yield the same effective ordered acquisition request and shared limits. Invalid/conflicting manifests fail before requests; mixed successful, empty, failed, and partial targets retain exhaustive per-target accounting without multiplying budgets.
- **Paging:** complete page retrieval matches unpaged logical output. Replayed pages are stable; stale/wrong-snapshot continuations reject. Cross-page deduplication, origin coverage, whole-span limits, and explicit over-page-size omissions/references hold without resetting budgets.
- **Compatibility:** every advertised pipeline transition has an executable producer/consumer fixture for its declared surfaces and versions. Internal-only or validate-only support is never advertised as a complete public path. Observed deployment is distinguished from source implementation. Legacy empty/partial-empty graph bytes remain unchanged while normalized collection arrays remain non-null.
- **Failure diagnostics:** deterministic local servers distinguish: request not written; request written but no response while the process remains alive; protocol-error response; received but unmatched response; late response after timeout/cancellation; process exit during preparation; closed/unreadable transport or read-loop failure; explicit call-hierarchy unsupported; and successful preparation after document supply. Spawn, initialize-write/response, and document-supply failures also receive distinct phase/reason records. For every case, method/target/sequence, available request ID, timing/limits, terminal state, lifecycle/exit and transport state, negotiated capability facts, document-preparation prerequisite, and bounded stderr status are attributable or explicitly unavailable. Artifacts remain schema-valid and replayable offline; partial graphs are preserved; no retry occurs. CLI/MCP classifications agree, current filesystem/process state is not consulted during replay, V1 bytes remain unchanged, and valid empty/unresolved-seed outcomes are not relabeled initialization failures.
- **Privacy and bounds:** secret-bearing stderr/configuration fixtures cannot leak through default errors or stdout evidence; explicit diagnostic policy controls richer retention. Byte/time/storage ceilings, truncation, cancellation, failed collection, and unavailable process status are tested without masking the primary failure or leaving an unmanaged blocked writer.
- **Coverage audit:** map existing public native/framework capability and coverage reporting to existing requirements, retain identified gaps, and test that unsupported boundaries and zero candidates never become source-absence or completeness claims.

## 15. Required tests

- unit tests for identity, canonicalization, and source receipt rules;
- line movement, rename, unchanged-symbol, and semantic-only-change identity tests;
- historical V2/V3 compatibility and migration tests;
- property tests for deterministic ordering;
- golden schema and canonical JSON tests;
- source mutation, multi-collection, non-Git snapshot, dirty-state, external-source, trust-anchor, authentication-state, digest-preimage, and revision-attestation mismatch tests;
- qualification-policy authority, evidence-minimum, positive/negative predicate, stale-policy, self-issued-PASS, and exact-tuple tests;
- capability/completeness red fixtures, including scope-escalation, opaque, stale, self-narrowed, unqualified-extractor, non-exhaustive-commitment, and wrong-scope denominator-proof rejection;
- relation reconstruction, occurrence-provenance, upstream-observation identity/canonical-order/foreign-key/cycle/parent-closure fixtures, unavailable-upstream-provenance rejection, support-group independence, cross-CLI/MCP/offline minimum-dependence recomputation, illegal class-splitting, and multiedge/call-site tests;
- direction and projection-preservation tests;
- FR20 required-symbol resolution/expansion/connection separation, ambiguous and aliased selectors, connected and disconnected implementations, reverse-only and zero-hop paths, shared-budget exhaustion, missing-accounting rejection, source/version uncertainty, versioned export preservation, omitted-option compatibility, and CLI/MCP acquisition-to-offline-path replay tests;
- FR21 offline source-context selection, source-version isolation, interval union/origin coverage, boundary-authority preservation, UTF-8/UTF-16/UTF-32 byte-coordinate conversion, BOM/CRLF, privacy opt-in, explicit omissions, whole-span budget edges, tamper rejection, and CLI/MCP publication/replay parity tests;
- FR20–FR23 manifest parity/conflict rejection, stable snapshot paging, cross-page coverage and budgets, executable versioned compatibility-matrix transitions, legacy-null/normalized-array empty results, bounded privacy-safe phase/reason and per-preparation request diagnostics, deterministic unsupported/protocol-error/no-response/process-exit/transport-close/success controls, and native/framework coverage-reporting audit fixtures under AC18;
- weak/strong component and condensation fixtures;
- bridge, articulation, disconnected, and singleton fixtures;
- PageRank convergence and dangling-node tests;
- PPR multi-seed and weight tests;
- numeric serialization and replay tests;
- corrupted digest and stale artifact tests;
- bounded-resource and huge-graph tests;
- retained real-server qualification tests for each language/provider combination;
- CLI/MCP parity tests for every analysis operation;
- MCP capability, schema, limit, pagination, and output-selector tests;
- oversized-result immutable-publication and no-silent-truncation tests;
- operation-registry closed-lifecycle, required-operation promotion, runtime-availability compatibility, denominator, and surface-parity tests;
- independently governed matrix-profile, mandatory-member/product, canonical qualification-cell generation, Cartesian-axis, composite-cell rejection, equivalence-reduction, non-waivable-cell, independently provisioned waiver-authority, expiry, and blocked-operation tests;
- deferred-family publication-gate neutrality tests;
- verifier-side trust-anchor provisioning, claimant-separation, and self-anchor rejection tests;
- cross-family domain-neutrality negative fixtures for every public artifact and transport.

Deferred community tests include seed replay, label canonicalization, instability diagnostics, and boundary accounting.

## 16. Migration and compatibility

- Existing V2 and V3 artifacts remain readable.
- Existing commands retain behavior unless a correctness defect requires a versioned change.
- Historical IDs are never silently rewritten.
- New identity fields use a new schema boundary or independently versioned artifacts.
- Existing retained qualification remains historical evidence and is not relabeled under new semantics.
- Derived normalized relations retain foreign keys to their admitted source graph and execution bundle.

## 17. Risks and mitigations

### General CPG scope expansion

Restrict owned relations to attributable provider structural evidence and generic analysis.

### Authentication overclaim

Name integrity, source-snapshot identity, optional revision attestation, materialized-byte binding, manifest authentication, and producer authentication separately.

### Version-control coupling

Keep source collection, snapshot, and receipt identities VCS-neutral. Isolate Git object IDs and worktree state in the Git custody adapter.

### Server omission interpreted as absence

Preserve multidimensional completeness and retained qualification evidence.

### Identity stability overclaim

Separate occurrence, portable, revision-scoped, logical, and correspondence identities.

### Projection ambiguity

Require a digest-bearing projection artifact for every analysis.

### Hub domination

Publish degree and hub diagnostics; require explicit weighting; never silently remove hubs.

### Floating-point or randomized nondeterminism

Specify replay environment, ordering, numeric policy, seeds, versions, and logical digests.

### Community labels interpreted semantically

Use neutral schemas and defer community work until inputs and implementations are qualified.

### Large-graph resource exhaustion

Require explicit ceilings and typed incomplete results; never silently approximate.

## 18. Success metrics

- 100% of retained source-backed graph records resolve to source receipts, and every `AUTHENTICATED` snapshot verifies to an independently provisioned trust anchor.
- 100% of analysis artifacts reference exact graph and projection digests.
- 100% of `SUPPORTED` and `DEPRECATED` operations have implemented CLI and MCP surfaces with the same logical result digest; current environmental availability is reported separately.
- Byte-identical replay within every declared deterministic fixture environment.
- No qualified language/provider claim without retained evidence.
- Zero cases where provider-relative completion is represented as source completeness.
- No undocumented graph reconstruction required by downstream consumers.
- No public artifact family or transport makes a product-feature or business-identity claim from structural evidence.
- Existing V2/V3 fixtures retain their historical IDs and meaning.

## 19. Open decisions

1. Concrete trust-anchor distribution and optional signature mechanisms satisfying verifier-side provisioning, claimant separation, and provisioning-receipt requirements.
2. Source collection identity across hosts, mirrors, archives, and VCS adapters.
3. Canonical multi-collection snapshot membership and ordering.
4. Treatment of admitted generated, dependency, external, and virtual sources.
5. Whether normalized relations are independently published or embedded additively in a future graph version.
6. Revision-attestation policy for logical relation IDs.
7. Ownership and implementation of cross-revision correspondence.
8. Numeric replay scope across operating systems and architectures.
9. Exact and approximate betweenness thresholds.
10. Path-diversity definition and output cap policy.
11. Whether hierarchical document containment reaches initial substrate qualification.
12. Infomap and Leiden implementation, licensing, and packaging.
13. FR21 admitted artifact versions, source-bound boundary-record contract, and concrete privacy/whole-span budget defaults; automatic semantic context discovery remains deferred.

## 20. First delivery milestone

The first milestone is complete exactly when `PROGRAM_B_ADMITTED` is true. The versioned substrate qualification matrix must therefore validate every generated cell under the non-waivable and waiver rules above, including retained real-server V3 evidence demonstrating:

1. independently anchored authenticated source-snapshot and materialized-source custody, with Git revision attestation when requested;
2. complete contributing-source receipts;
3. corrected and versioned identity semantics;
4. separate capability, observed execution, qualification, and completeness artifacts;
5. lossless normalized relation occurrences;
6. an explicit projection artifact;
7. offline verification and deterministic replay preparation;
8. unchanged historical V2/V3 identities;
9. occurrence-level acquisition provenance and validated support groups;
10. denominator-proof validation for every non-`UNKNOWN` source-completeness claim;
11. registry-defined CLI/MCP parity and cross-family domain neutrality.

No PageRank, PPR, Infomap, or Leiden implementation is required for this milestone. Its purpose is to make every later analytical result trustworthy and unambiguous.

Neither artifact delivery nor B05 `all_requested_relations_supported` establishes completion of this milestone. Acceptance requires a revision-bound decision supported by the checked normative matrix mapping in A4; this document records no such acceptance.

## 21. Remaining work and proposed next milestone

### Remaining-work inventory

- Close contributing-input custody and effective-configuration provenance gaps, with independent authentication and operational integration evidence.
- Complete FR5–FR7 identity separation, heterogeneous occurrence reconstruction, canonical upstream provenance, and support-dependence validation rather than treating normalized provider records as the finished substrate.
- Adopt and qualify the versioned projection, operation-registry, denominator, trust, and matrix policies; retain exact native and modeled evidence separately and prove CLI/MCP/offline enforcement.
- Deliver and qualify the FR9–FR11 analytical artifacts, replay/resource contracts, and transport parity. Shortest paths, capped enumeration and explicit path diversity, structural metrics, PageRank, and PPR remain required for Program B completion. Program C remains deferred.

### PROPOSED: Versioned analytical projection and first topology operations

This is a sequencing proposal for maintainer approval, **not approved implementation work or a declaration of Program A/B qualification**. Preserve §20 as the substrate-admission milestone; do not replace it with a smaller algorithm demonstration.

After its prerequisites and `PROGRAM_B_ADMITTED` are satisfied, the proposed next increment would publish an explicit versioned analytical projection contract under FR8, then deterministic incoming/outgoing reachability, bounded neighborhoods, weak and strong components, and condensation. Projection policy and output must bind the exact admitted graph digest, direction, multiedges, weights, self-loops, unresolved endpoints, exclusions, and frontier handling. Component membership and condensation edges require canonical ordering and attributable relation witnesses; existing SCC cycle accounting does not satisfy this publication contract.

Require offline CLI/MCP execution over the same shared operations, schemas, semantic validators, canonical results, and digests; include inline/immutable publication parity, corruption and policy-mismatch rejection, empty/disconnected/self-loop/multiedge fixtures, deterministic replay, and typed resource-limit outcomes. Inputs must satisfy custody, occurrence reconstruction, support accounting, denominator-claim, compatibility, and neutrality obligations. Qualification must cover the exact operations, projection classes, and native/provider dependencies selected by the independently governed matrix, without a blanket Ember dependency or any admission bypass.

Before authorizing this increment, maintainers must record the exact projection/operation versions, normative matrix mapping and dependency scope, qualification policy and evidence scope for analysis-only defaults, and revision-bound acceptance authority. This proposal neither drops the remaining FR9 paths/path-diversity requirements nor FR10 metrics or FR11 PageRank/PPR, and it does not accelerate deferred communities.
