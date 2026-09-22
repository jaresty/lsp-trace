# ADR 0010: Add provenance-bounded feature attribution as post-inventory processing

- **Status:** Proposed
- **Date:** 2026-09-20
- **Decision owners:** LSP Trace maintainers
- **Depends on:** ADR 0007 and ADR 0008
- **Implementation priority:** Immediately after the ADR 0007 feature inventory; before ADR 0009

## Context

ADR 0007 produces an immutable, local, offline, provisional feature inventory from bounded structural and source evidence. Its feature candidates remain `authority=0`, `accepted=false`, and `completeness=UNKNOWN`. ADR 0008 preserves the separation between server-reported graph custody and deterministic source projection.

Once an inventory exists, downstream tools need to associate independently discovered things with the feature candidates whose constituents contain those things. Examples include:

- database tables referenced by feature code;
- API routes implemented or called by feature code;
- queues, topics, files, configuration keys, and external services;
- tests, requirements, diagnostics, or operational observations anchored to source;
- organization-specific resource types unknown to `lsp-trace`.

The domain-specific extractor is best positioned to establish what a subject means. A SQL analyzer can establish that an exact source occurrence references `public.orders`; an OpenAPI analyzer can establish that an exact handler implements a route. Those tools should not have to reproduce Program C membership, retained source custody, catalog identity, correction history, or immutable replay rules.

Conversely, `lsp-trace` should not embed SQL, ORM, API, queue, or infrastructure semantics into feature capture. Feature capture must remain independent of downstream resource technologies. Resource extraction may improve, be rerun, or use a different revision without rerunning the local Describe model or changing the underlying inventory.

The missing capability is therefore a generic, deterministic postprocessing join:

```text
external subject
  -> exact structural or retained-source anchor
  -> inventory constituent
  -> provisional feature candidate ID
```

This operation maps evidence to candidate identities. It does not infer feature identity, ownership, purpose, runtime use, or completeness.

ADR 0009 proposes a broader semantic discovery and change-planning system. Exact feature attribution is narrower, deterministic, useful without a model, and a prerequisite for aggregating later discovery results by feature candidate. It will be implemented first.

## Decision

Add an optional **provenance-bounded feature attribution** operation after feature inventory publication.

The offline attribution operation consumes:

1. one immutable ADR 0007 feature-inventory selector;
2. the exact immutable constituent-membership and source-custody artifacts on which that inventory depends;
3. one caller-supplied batch of independently identified subjects whose inputs are exact immutable anchors or selectors into immutable locator-resolution receipts;
4. one closed, versioned attribution policy and bounded resource limits.

Two caller modes share this offline kernel:

- **Exact mode:** a caller submits exact immutable anchors directly, or selects exact resolved anchors from an immutable `LocatorResolutionReceipt`.
- **Convenience live mode:** a caller submits a bounded batch of symbol, position, or document-regex locators to the live resolver. The resolver freezes every successful resolution into an immutable `LocatorResolutionReceipt`, and only that receipt is passed to offline attribution. A convenience resolve-then-attribute interface returns the receipt alongside the attribution result.

The live resolver and offline attribution kernel are separate contracts. `ResolveLocatorRequest`, `LocatorResolutionReceipt`, `AttributionRequest`, and `AttributionResult` are distinct canonical artifacts with distinct identities. `AttributionRequest` never contains a raw live locator. The offline kernel never reads workspace, session, language-server, process, or runtime state.

It produces one immutable attribution artifact that maps each submitted subject to zero, one, or several provisional feature candidate IDs with exact mechanical witnesses and complete terminal accounting.

Release one performs only exact deterministic joins. It does not use an LLM, embeddings, lexical similarity, names, descriptions, arbitrary graph proximity, or ambient workspace inspection.

Every derived mapping remains:

```text
authority = 0
accepted = false
completeness = UNKNOWN
```

The operation is postprocessing. It does not run during census, Program C, source capture, Describe, or catalog construction. Failure cannot invalidate or roll back the previously committed inventory.

## Goals

- Provide one generic way to map arbitrary externally identified subjects to provisional feature candidate IDs.
- Reuse exact constituent, graph-subject, source-range, revision, and custody identities already owned by `lsp-trace`.
- Keep domain-specific extraction outside the feature-inventory pipeline.
- Preserve one-to-many mappings for shared code rather than forcing one feature.
- Represent unmapped and invalid anchors explicitly.
- Support byte-identical replay from immutable artifacts without a live session or workspace.
- Let resource extractors evolve without rerunning feature description or changing inventory identities.
- Provide a stable foundation for table-to-feature, route-to-feature, test-to-feature, and later ADR 0009 discovery aggregation.

## Non-goals

This decision does not authorize:

- accepted or canonical feature identity;
- database-, ORM-, API-, queue-, file-, or infrastructure-specific extraction;
- ownership, product-purpose, runtime-use, value, or completeness claims;
- choosing one candidate when an anchor belongs to several candidates;
- inferring an anchor from a subject name or description;
- semantic similarity, embedding search, or model-selected mappings;
- creation or repair of `CALLS` or any other structural relation;
- ambient checkout, language-server, process, network, or current-session fallback during replay;
- mutation, merge, split, rename, or acceptance of feature candidates;
- proof that an external extractor correctly interpreted its domain;
- proof that a referenced resource exists or is used in production.

## Terminology

### Subject

A **subject** is an externally identified thing to map, such as `database-table:public.orders`. Its namespace, identity, and domain meaning belong to the caller or extractor. `lsp-trace` preserves but does not reinterpret them.

### Anchor

An **anchor** is an exact immutable reference connecting a subject to admitted structural or source evidence. Release one supports only the closed anchor kinds defined below.

### Constituent

A **constituent** is a mechanically retained member of an ADR 0007/Program C feature candidate. Constituent membership is not semantic acceptance.

### Feature candidate ID

A **feature candidate ID** is the immutable candidate identity in the selected inventory. The API must not shorten this to an accepted `feature_id` unless a later contract introduces independently accepted feature identities.

### Locator-resolution receipt

A **locator-resolution receipt** is the immutable boundary between optional live locator resolution and offline exact attribution. For each submitted locator it records exactly one terminal disposition: `RESOLVED`, `UNRESOLVED`, or `AMBIGUOUS`. A resolved entry freezes the admitted anchor plus its exact graph-subject or relation-occurrence identity, evidence/item/selection/display ranges as applicable, logical URI, source digest and byte length, position encoding, document version, managed session and generation, workspace revision and custody, and provider identity. Session and provider fields establish how the receipt was produced; they are immutable receipt evidence, not runtime dependencies during replay.

Ambiguous resolution preserves bounded candidate accounting and identities but produces no selected anchor. Input order, provider order, or server response order must never silently choose the first candidate. Unresolved and ambiguous entries remain terminal and cannot be upgraded by attribution.

### Witness

A **witness** is the exact chain of immutable identities and ranges that mechanically establishes one subject-to-candidate mapping.

## Input contract

### Locator resolution request

`ResolveLocatorRequest` is the only contract that admits live symbol, position, or document-regex locators. It names an exact managed session and generation, bounded provider policy, expected workspace revision/custody when supplied, privacy identity, and hard limits. Its output is a `LocatorResolutionReceipt`; it is not an attribution request and cannot select an inventory or feature candidate.

Resolution is fail-closed. Every submitted locator receives one terminal disposition, all candidate and work accounting is bounded, and ambiguity never selects a first match. Canonical receipt identity binds the complete request identity, terminal entries, admitted anchors, provenance, limits, and accounting.

### Attribution request

A canonical request contains:

- request schema version and request identity;
- immutable feature-inventory selector, digest, schema identity, and byte length;
- exact constituent-membership artifact selector and identity;
- exact graph/source custody selectors required by submitted anchors;
- attribution-policy identity;
- caller-supplied batch identity, digest, canonical byte length, and subject count;
- privacy-policy identity;
- hard limits for subjects, anchors, mappings, ranges, bytes, and work;
- `authority=0`, `accepted=false`, and `completeness=UNKNOWN`.

The request contains no workspace path, executable path, model path, network endpoint, or live-session fallback.

### Subject record

Each subject record contains:

- caller-unique `subject_id`;
- nonempty, versioned `subject_kind` namespace;
- optional opaque canonical metadata under a declared caller schema;
- one or more exact anchors, each supplied inline or selected by immutable receipt identity and resolved-entry identity;
- optional extractor identity, version, policy identity, and source-evidence digest;
- the caller's own bounded interpretation, if any, kept distinct from `lsp-trace` mapping facts.

Subject metadata cannot participate in release-one mapping decisions. Raw symbol, position, and regex locators are invalid in `AttributionRequest`; callers that have locators must resolve them first.

### Anchor kinds

Release one supports these exact anchor kinds:

#### `GRAPH_SUBJECT`

References one exact `graph_subject_id` under the graph/custody identity used by the selected inventory.

#### `SOURCE_RANGE`

References:

- exact logical source URI;
- exact revision or retained-source custody identity;
- position encoding;
- exact half-open range;
- optional expected source digest;
- optional exact graph subject when already known.

A source range is eligible only when its custody and revision are identity-compatible with the inventory's retained source evidence.

#### `RELATION_OCCURRENCE`

References one exact retained relation-occurrence ID and its custody bundle. Release one maps its exact endpoint constituents independently and preserves both when both are eligible.

Unknown anchor kinds fail closed. Anchor fields are closed; duplicate keys, duplicate anchors, trailing content, invalid ranges, and identity substitution are rejected.

## Attribution rules

Release one freezes a deterministic policy with the following mapping kinds.

### `EXACT_CONSTITUENT`

A `GRAPH_SUBJECT` anchor exactly equals a constituent graph-subject identity in the selected inventory.

### `RANGE_CONTAINED`

A `SOURCE_RANGE` is fully contained within one retained constituent display range under the exact same logical source, revision/custody identity, and position encoding.

Containment does not cross files, custody objects, revisions, position encodings, or unresolved display ranges. Partial overlap is not containment.

### `RELATION_ENDPOINT`

A retained relation occurrence names an endpoint that is an exact constituent of a candidate. Caller and callee endpoint mappings remain separate witnesses; the relation does not imply that both endpoints belong to one feature.

### Multiplicity

If one anchor maps to constituents in several candidates, the result contains every mapping and uses terminal disposition `MAPPED_MULTIPLE`. Candidate order is deterministic. The operation must not choose, rank, merge, or semantically reconcile them.

If several anchors establish the same subject-to-candidate mapping, the mapping is emitted once with every distinct witness retained up to the declared witness bound. Truncation is explicit and accounted.

### No transitive attribution in release one

Release one does not map through callers, callees, shared helpers, package membership, file co-location, repository path, lexical similarity, catalog prose, or community adjacency. Such rules require a successor policy and separate evaluation.

## Terminal accounting

Every admitted subject receives exactly one terminal disposition:

- `MAPPED_ONE`;
- `MAPPED_MULTIPLE`;
- `UNMAPPED`;
- `ANCHOR_INVALID`;
- `ANCHOR_CUSTODY_MISMATCH`;
- `ANCHOR_UNAVAILABLE`;
- `RESOURCE_LIMIT`;
- `DUPLICATE_SUBJECT`.

Every admitted anchor receives exactly one terminal disposition:

- `EXACT_CONSTITUENT`;
- `RANGE_CONTAINED`;
- `RELATION_ENDPOINT`;
- `NO_MATCH`;
- `INVALID`;
- `CUSTODY_MISMATCH`;
- `UNAVAILABLE`;
- `RESOURCE_LIMIT`.

Operation outcomes are:

- `COMPLETE` — all admitted subjects reached terminal disposition;
- `COMPLETE_DEGRADED` — all admitted subjects reached terminal disposition and at least one subject or anchor failed independently;
- `INVALID_INPUT`;
- `POLICY_MISMATCH`;
- `ARTIFACT_UNAVAILABLE`;
- `RESOURCE_LIMIT`;
- `PUBLICATION_FAILURE`;
- `REPLAY_FAILURE`.

`UNMAPPED` means no eligible exact mapping was found within the selected inventory and admitted anchors. It does not mean the subject is unrelated to every feature or that the inventory is complete.

Accounting includes:

- submitted, admitted, mapped-one, mapped-multiple, unmapped, failed, and truncated subject counts;
- submitted, admitted, matched, unmatched, failed, and truncated anchor counts;
- unique candidates and mappings emitted;
- witness counts and truncation;
- exact work and byte consumption.

All equalities are validated before publication.

## Output artifact

The canonical attribution artifact contains:

- schema and artifact identities;
- request and input-batch identities;
- exact selected inventory/catalog, constituent-membership, graph, source-custody, and policy identities;
- ordered subject results;
- ordered mappings for each subject;
- mapping kind and exact witnesses;
- terminal dispositions, limitations, and accounting;
- `authority=0`, `accepted=false`, and `completeness=UNKNOWN`.

A mapping resembles:

```json
{
  "subject_id": "database-table:public.orders",
  "feature_candidate_id": "sha256:...",
  "mapping_kind": "EXACT_CONSTITUENT",
  "witnesses": [
    {
      "anchor_id": "sha256:...",
      "graph_subject_id": "sha256:...",
      "constituent_membership_id": "sha256:..."
    }
  ],
  "authority": 0,
  "accepted": false,
  "completeness": "UNKNOWN"
}
```

This means only that the selected feature candidate mechanically contains code identified by the submitted anchor. It does not transfer the extractor's domain claim into `lsp-trace` authority and does not establish resource ownership or runtime use.

## Artifact and identity separation

The following remain distinct:

- feature inventory and catalog;
- constituent-membership artifact;
- external subject/evidence batch;
- attribution request;
- attribution result;
- optional domain-specific presentation or aggregation.

The attribution-result identity binds all of them by immutable digest. A new inventory, corrected membership, revised source custody, changed external evidence batch, or changed attribution policy produces a new result identity. Existing artifacts remain immutable.

An extractor may reuse one evidence batch against several inventories. One inventory may be joined with several resource batches. Neither operation mutates the other artifact.

## Replay

Replay uses only retained immutable artifacts named by the attribution result or request:

- exact inventory/catalog;
- exact constituent membership;
- exact graph/source custody;
- exact external subject batch;
- exact policy and bounds.

Replay must produce byte-identical canonical attribution bytes or a typed failure. It cannot read the ambient checkout, query a language server, rerun census or Describe, invoke the extractor, access the network, or substitute a current inventory.

## Privacy and security

External subjects and metadata may reveal private schema, customer, infrastructure, or operational names. The operation therefore:

- remains local and offline;
- admits only caller-supplied bounded records;
- partitions publication by privacy-policy identity;
- never interprets opaque subject metadata for mapping;
- exposes source-safe typed failures without source bodies or private raw paths;
- publishes atomically with no replacement;
- does not commit raw external evidence batches to Git by default.

Sanitized qualification fixtures may be committed separately after review.

## Interfaces

### CLI

After qualification, expose both modes without collapsing their contracts:

```text
lsp-trace feature-map \
  --inventory SELECTOR \
  --subjects FILE \
  --policy exact-v1 \
  [--machine]

lsp-trace feature-map-resolve \
  --session SESSION \
  --generation GENERATION \
  --locators FILE \
  [--then-attribute REQUEST]
```

The exact command accepts only exact anchors or immutable receipt selectors and returns an immutable attribution-result selector. The convenience command first publishes or returns an immutable locator-resolution receipt; when attribution is requested it returns both the receipt and attribution result. Host-controlled configuration owns publication and privacy paths. The attribution subject file cannot select executables, models, sessions, or ambient workspaces.

### MCP

After qualification, expose distinct canonical operations for resolution and attribution, plus an optional convenience composition that preserves both outputs:

```text
lsp_trace_v1_resolve_feature_locators
lsp_trace_v1_feature_map
lsp_trace_v1_resolve_and_feature_map
```

The resolver accepts `ResolveLocatorRequest` and returns `LocatorResolutionReceipt`. The exact mapper accepts `AttributionRequest` and returns `AttributionResult`. The convenience operation performs those two steps in order and returns both immutable artifacts; it does not create a fifth combined contract or permit the attribution kernel to inspect live state.

CLI and MCP share canonical validators and identities. All attribution transports share one workspace-free kernel and publication implementation; only the resolver may use the managed live session named by its request.

### Library kernel

The transport-neutral kernel should also be callable directly by domain-specific tools. A database mapper may extract table occurrences itself, submit exact anchors, and render a table-centric view without adding SQL semantics to `lsp-trace`.

## Qualification

Before public enablement, qualification must cover:

- exact graph-subject mapping;
- exact range containment with UTF-8 and UTF-16 positions;
- boundary, empty, partial-overlap, and invalid ranges;
- exact relation endpoint mapping;
- shared constituents mapping to several candidates;
- repeated anchors and witness deduplication;
- unmapped, unavailable, and custody-mismatch cases;
- empty batches and zero mappings;
- mixed success and failure accounting;
- duplicate keys, subjects, anchors, unknown fields, and trailing content;
- cross-revision, cross-catalog, and coordinated identity substitution;
- truncation and every hard resource bound;
- byte-identical workspace-free replay;
- privacy partitioning and source-safe diagnostics;
- historical ADR 0007 inventory and catalog bytes remaining unchanged.

At least one end-to-end fixture should use an external database-table extractor, but SQL or ORM interpretation remains outside the `lsp-trace` acceptance boundary. The fixture verifies only that exact submitted anchors map mechanically to expected candidate memberships.

## Implementation phases

### Phase 0 — Inventory prerequisite

- complete and publish the ADR 0007 provisional feature inventory;
- expose immutable candidate-to-constituent membership and exact dependency selectors;
- confirm stable feature candidate identities and correction behavior.

ADR 0010 implementation begins immediately after this prerequisite. ADR 0009 implementation remains deferred.

### Phase 1 — Dual-interface contracts and offline exact kernel

- freeze the four distinct `ResolveLocatorRequest`, `LocatorResolutionReceipt`, `AttributionRequest`, and `AttributionResult` contracts;
- freeze subject, closed anchor union, receipt selector, witness, accounting, and failure schemas;
- require the resolver to preserve exact provenance and terminal `RESOLVED`/`UNRESOLVED`/`AMBIGUOUS` accounting without first-match selection;
- prohibit raw live locators and runtime dependencies from the attribution request and kernel;
- implement `GRAPH_SUBJECT` and `SOURCE_RANGE` mapping;
- implement strict canonical validation and workspace-free replay;
- add property, adversarial, and identity-substitution tests.

### Phase 2 — Relation occurrence and publication

- add `RELATION_OCCURRENCE` mapping;
- implement immutable publication and selector verification;
- add mixed, shared, zero-result, truncation, and privacy fixtures.

### Phase 3 — CLI and MCP

- expose `lsp-trace feature-map` and `lsp_trace_v1_feature_map` over the common kernel;
- qualify machine and human rendering;
- retain public opt-in status until end-to-end qualification passes.

### Phase 4 — Domain adapters

- publish examples for database tables and one additional domain;
- keep extractors separate from core;
- use their findings to evaluate whether a successor attribution policy is needed.

Every phase requires assertion-specific RED evidence before production correction. Completing one phase does not silently authorize the next.

## Consequences

### Positive

- Feature IDs become a reusable structural integration surface.
- Database and other domain tools can map their findings without understanding Program C internals.
- Resource extraction can improve independently of feature generation.
- Exact, shared, and unmapped results remain inspectable and replayable.
- ADR 0009 can later aggregate discovery evidence by feature candidate using a deterministic primitive.
- No model or prompt qualification is required for release-one attribution.

### Costs and risks

- The inventory must publish stable, sufficient constituent membership and custody references.
- Source-range containment requires careful position-encoding and revision handling.
- Shared helper code may map to many candidates and produce large results.
- Users may mistake containment for ownership or runtime use despite explicit labels.
- External extractors may submit incorrect anchors or domain identities; exact mapping cannot validate their domain semantics.
- Catalog correction creates new attribution results and may require downstream rebuilds.

These costs are accepted under immutable identity binding, exact-only release-one rules, complete terminal accounting, and explicit authority ceilings.

## Alternatives considered

### Build database-table mapping directly into feature capture

Rejected. It couples feature inventory to one resource technology, expands the critical census/Describe path, and forces resource improvements to rerun feature generation.

### Let every extractor independently interpret feature membership

Rejected. It duplicates Program C, custody, replay, and correction logic and will produce inconsistent feature mappings.

### Use catalog prose or semantic similarity

Rejected for release one. Generated descriptions are provisional and may be generic or corrected. Similarity would nominate candidates rather than establish exact constituent membership.

### Attribute through the call graph transitively

Deferred. Call proximity does not establish feature membership and can greatly broaden shared infrastructure. A successor policy would require separate bounds, accounting, evaluation, and terminology.

### Implement ADR 0009 first

Rejected as the immediate sequence. ADR 0009 is broader, model-dependent, and evaluation-heavy. Exact feature attribution is smaller, deterministic, directly useful to external tooling, and can become an input to ADR 0009.

## Relationship to ADR 0007, ADR 0008, and ADR 0009

ADR 0007 remains the owner of inventory construction, provisional feature candidates, constituent membership, Describe, catalog correction, and all feature-authority boundaries. ADR 0010 consumes an immutable inventory and never changes or accepts it.

ADR 0008 remains the owner of graph/source custody separation and deterministic source projection. ADR 0010 consumes exact admitted graph subjects, relation occurrences, retained source ranges, and custody identities. Source text does not manufacture structural evidence.

ADR 0009 remains the proposed owner of natural-language discovery, ranking, fusion, and bounded synthesis. Its implementation is sequenced after ADR 0010. ADR 0009 may consume attribution artifacts as typed evidence, but semantic discovery cannot upgrade, repair, or silently replace exact attribution mappings.
