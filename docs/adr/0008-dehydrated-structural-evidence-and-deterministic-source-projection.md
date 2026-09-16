# ADR 0008: Separate retained structural evidence from deterministic source projection

- **Status:** Accepted
- **Date:** 2026-09-16
- **Decision owners:** LSP Trace maintainers
- **Scope:** Graph Provenance V5 source availability, deterministic source projection, and hydration

## Context

Structural evidence and source presentation serve different purposes. Graph Provenance V5 retains acquisition evidence for custody, verification, replay, and later inspection. An LLM answering a bounded code question usually needs only the exact target, admitted server-reported relations, and a small set of corresponding source ranges. Presenting every source document observed during acquisition can increase privacy exposure, payload size, token pressure, and semantic noise without adding graph authority.

The current transient structural-context path does not itself retain a large source bundle. Operations 36 and 43 return bounded source-locating structural results: workspace-relative paths, declaration ranges, call-site ranges, and bounded analytics. The underlying transient executor uses `CaptureSupply: false`; the result remains non-retained, non-replayable, publication-ineligible, and hydration-ineligible. ADR 0006 requires escalation to create a new durable acquisition identity rather than upgrading transient bytes in place.

Graph Provenance V5 remains the authoritative production acquisition contract. V2 and V3 remain historical readers. Existing V5, transient-result, source-snapshot, hydration, and publication schema bytes are immutable and cannot be broadened by reinterpretation.

The repository already has two relevant foundations:

1. private, content-addressed, no-replace publication with bounded selectors and no pathname fallback;
2. deterministic offline hydrated inspection with explicit selection, body opt-in, stable ordering, bounded accounting, and no acquisition or filesystem fallback.

A digest can verify bytes but cannot reconstruct bytes that were never retained. Exact deferred hydration therefore requires immutable retrievable bytes, not only a URI, document version, or digest.

## Decision

Introduce a V5-bound source-availability layer that is separate from graph authority and from LLM presentation.

The initial implementation will:

1. leave operations 36 and 43 and their existing input/result bytes unchanged;
2. define a new immutable `lsp-trace.dehydrated-source-manifest.v1` artifact bound to exact admitted Graph Provenance V5 bytes;
3. define a new deterministic `lsp-trace.source-projection.v1` result family;
4. expose projection through additive versions of the existing `lsp_trace_v1_inspect_hydrated` dispatch and canonical execute path;
5. preserve exactly 43 canonical operations and 13 compact-advertised tools;
6. add no operation 44;
7. keep operation-43 source-mode flags deferred until the offline artifact, storage, and hydration contracts are independently qualified.

This ADR does not authorize runtime or schema implementation by itself. Each implementation stage requires assertion-specific RED evidence before production changes.

## Current contracts preserved

The design preserves these existing boundaries:

- Graph Provenance V5 is authoritative for retained graph acquisition.
- V2 and V3 remain historical readers and are never silently upgraded.
- Existing immutable schema identities and bytes do not change.
- `CALLS` is exclusively server-reported.
- Source text, co-presence, shared files, names, overlap, proximity, containment, hydration, and model interpretation cannot create nodes or relations.
- `authority` remains `0` where currently required.
- `source_graph_complete` remains `UNKNOWN`.
- A transient result cannot be converted into durable evidence by attaching source metadata.
- Additional acquisition creates a new evidence identity and never silently extends an existing capture.
- No automatic retry, alternate resolver, current-checkout fallback, range repair, encoding guess, malformed-output repair, hidden budget increase, or implicit resumability is permitted.

## Terminology and evidence roles

Every source binding has exactly one role:

- **`ENDPOINT`**: an exact source range associated with an admitted graph node.
- **`RELATION`**: an exact server-reported call-site range associated with an admitted `CALLS` relation.
- **`ANCILLARY`**: source retained because acquisition supplied, opened, or captured it, but not selected as endpoint or call-site evidence.

A source binding contributes zero graph support. `ENDPOINT` and `RELATION` identify where admitted facts can be inspected; they do not replace the server evidence that established those facts. `ANCILLARY` means acquisition context, not semantic support.

Hydration is a pure projection over an already admitted capture:

```text
hydrate(capture, selection) -> source projection
```

It adds zero graph facts and leaves graph bytes, identities, node IDs, relation IDs, authority, completeness, and support accounting unchanged.

Additional acquisition is distinct:

```text
acquire(seed, revision, policy) -> new capture identity + explicit lineage
```

A new traversal, provider request, Git read admitted as evidence, or workspace read cannot be returned under the original capture identity.

## Dehydrated source manifest

`lsp-trace.dehydrated-source-manifest.v1` binds source availability to exact Graph Provenance V5 bytes without embedding every source body in the graph artifact.

The manifest records at least:

- exact Graph Provenance V5 schema ID, digest, and byte length;
- capture identity and workspace identity;
- revision-binding kind, identity, and custody;
- source-object identity, content digest, and byte length;
- storage class and bounded selector;
- privacy classification;
- availability and qualification;
- position encoding;
- graph-subject binding and exact range;
- evidence role.

Storage classes are:

- `GIT_BLOB`: exact commit/tree/blob custody;
- `CONTENT_ADDRESS`: immutable host-pinned content-addressed object;
- `EMBEDDED_IMMUTABLE`: exact bytes retained in an admitted immutable carrier;
- `UNAVAILABLE`: no retrievable exact bytes.

Qualification distinguishes at least:

- `EXACT_BYTES`;
- `DIGEST_ONLY`;
- `UNVERIFIED`.

`DIGEST_ONLY` can verify candidate bytes but cannot satisfy deferred exact hydration by itself.

Selectors are bounded names beneath an already pinned private root. They are not caller-provided absolute paths, arbitrary filesystem paths, or ambient workspace lookups.

## Source storage and resolver model

The durable model is hybrid.

### Immutable Git source

Committed source may be represented by an exact commit/tree/blob binding. Hydration verifies the blob identity, byte length, and content digest before range extraction. A shallow clone, rewritten history, or garbage-collected object yields `SOURCE_UNAVAILABLE`; the hydrator does not read the current checkout instead.

### Content-addressed source

Dirty buffers, generated documents, non-Git source, and source requiring explicit retention use private content-addressed immutable objects. Resolution verifies selector, byte length, digest, custody root, and privacy policy before range extraction.

### Dirty or live buffers

A document version and digest cannot reconstruct a dirty buffer after it changes. Exact deferred hydration is available only when exact bytes were retained in an immutable carrier or content-addressed object during capture. Otherwise the manifest records `SOURCE_UNAVAILABLE`.

### Current workspace source

The current filesystem path is never an exact hydrator for a historical capture. Reading it is a new acquisition with a new identity and explicit `derived_from` lineage.

Resolvers do not fall through from Git to content-addressed storage to live files. The admitted storage class selects one resolver. Failure is typed and terminal for that resolution attempt.

## Projection modes

`lsp-trace.source-projection.v1` supports these explicit modes:

- **`NONE`**: no source bodies; return discoverability, availability, privacy, and omission accounting.
- **`TARGET`**: return the exact target endpoint range. Incident relation ranges are included only when the request explicitly selects a policy that names them.
- **`PROJECTED`**: return deterministic graph-bound endpoint and relation ranges for selected nodes and relations.
- **`COMPLETE_CAPTURE`**: return every permitted retained source object, including ancillary source. This is an explicit high-disclosure mode and is never the default.

Ancillary source is excluded from normal LLM projection unless explicitly requested. Its count, retained-byte total, privacy disposition, availability, and bounded hydration selector remain discoverable.

Operation 43 continues to return its existing transient result during the initial stages. Client policy may recommend `TARGET` for later retained inspection, but the server does not infer a projection mode.

## Deterministic selection and budgets

Selection is deterministic and independent of source-object input order. Canonical ordering is:

1. evidence role: `ENDPOINT`, `RELATION`, `ANCILLARY`;
2. graph subject identity;
3. canonical logical source identity;
4. exact range start and end;
5. source-object digest.

Requests declare independent hard limits for:

- selected source objects or nodes;
- selected ranges;
- returned source bytes;
- total work;
- response and page bytes where paging is explicitly requested.

Selection admits whole exact ranges in canonical order. It never silently shortens a range, splits a code point, increases a budget, or retries with a broader policy. Zero is valid where omission-only accounting is desired.

Accounting includes:

- candidate count;
- selected count;
- omitted count;
- selected byte count;
- one exact omission reason for every omitted candidate.

Required reconciliation is:

```text
candidates = selected + omitted
```

Omission reasons include privacy exclusion, source unavailable, node/object budget, range budget, byte budget, invalid binding, and failed exact-byte verification. Reasons remain mutually exclusive for accounting.

## Status and failure semantics

Graph acquisition status and source-projection status are separate.

- `COMPLETE`: every requested and permitted exact source selection was returned.
- `PARTIAL`: at least one requested exact selection was returned and at least one was unavailable, withheld, or failed verification.
- `TRUNCATED`: caller-declared projection limits prevented otherwise eligible selections from being returned.
- `SOURCE_UNAVAILABLE`: no required exact selection can be resolved from its admitted immutable source.

`PARTIAL` and `TRUNCATED` are not interchangeable. Privacy exclusion, missing historical objects, digest mismatch, and unavailable dirty-buffer bytes do not become `TRUNCATED`. Budget exhaustion does not become `SOURCE_UNAVAILABLE`.

A structurally incomplete transient or durable graph retains its existing typed graph failure. Source projection cannot convert it into successful structural evidence.

No cursor is returned unless paging was requested explicitly. A cursor continues the same immutable projection only; it does not resume acquisition or increase limits.

## Projection result

A projection result binds:

- exact graph digest and capture identity;
- manifest identity;
- projection mode and policy identity;
- source entries with role, graph subject, source identity, exact range, position encoding, digest, and optional body;
- complete selection and omission accounting;
- `graph_facts_added: 0`;
- authority and completeness ceilings inherited without strengthening.

Bodies are separately opt-in. Metadata-only projection remains available when source disclosure is not authorized.

## Privacy and security

Source objects are stored separately from graph and manifest bytes so retention or disclosure policy can withhold source without rewriting graph evidence.

Private source storage requires:

- owner-only permissions;
- host-pinned roots;
- canonical relative selectors;
- no request-supplied arbitrary paths;
- exact digest and length verification;
- no-replace immutable publication;
- bounded reads and responses;
- explicit privacy classification;
- no raw private root in public results.

Restricted and withheld bytes are excluded from default projections. Digests are not disclosure authorization and may enable guessing over small domains.

`COMPLETE_CAPTURE` is a high-disclosure request and remains subject to privacy policy. Its name describes capture coverage, not permission to disclose restricted bytes.

## Retention and garbage collection

Published manifests and source objects have independent retention. A source object may be collected only when:

1. no retained manifest or active lease requires it; and
2. the configured retention policy permits deletion.

After permitted collection, the manifest remains valid structural and custody metadata. Hydration returns `SOURCE_UNAVAILABLE`; it does not rewrite the manifest or fetch replacement bytes.

Retention guarantees for historical Git source require pinning or archiving the referenced objects. Repository reachability at capture time does not guarantee future availability.

The placement and implementation of source-object leases remain an implementation decision. This ADR does not decide whether they live in `internal/publication` or a dedicated source-object package.

## Immutable publication and offline replay

Publication is content-addressed, atomic no-replace, and verified from exact bytes. A public or cross-process artifact identity cannot be derived from mutable paths.

Complete offline replay requires:

1. exact admitted Graph Provenance V5 bytes;
2. exact dehydrated manifest bytes;
3. every referenced immutable source object required by the selected projection;
4. the projection policy and limits.

A manifest plus digests is verifiable but not source-replayable. Missing objects cause typed unavailability and never trigger live acquisition.

## Compatibility and operation strategy

Existing schema bytes remain registered and valid. New contracts are additive:

- `lsp-trace.dehydrated-source-manifest.v1`;
- `lsp-trace.source-projection.v1`;
- additive inspect-hydrated input and output versions.

The existing `lsp_trace_v1_inspect_hydrated` operation and `lsp_trace_v1_execute` gateway route the new additive contracts. Direct and gateway paths return identical delegated envelopes and typed failures.

No operation 44 is added. Canonical and compact counts remain exactly 43 and 13.

The existing sibling-oriented `lsp-trace.graph-v5-source-snapshot.v1` bytes and semantics are not broadened. The new manifest is a distinct family.

Operation-43 request flags or result versions are deferred. They may be considered only after the offline manifest, immutable storage, deterministic resolver, projection accounting, privacy behavior, and direct/gateway parity are qualified. Existing omitted-field operation-43 requests must remain byte-identical if such a later extension is proposed.

## Migration stages

1. **Semantic contract:** freeze evidence roles, status algebra, ordering, budgets, privacy classes, and no-fallback rules in this ADR.
2. **Schema families:** add new immutable manifest and projection schemas without changing predecessor bytes.
3. **Offline core:** extend deterministic hydrated-evidence admission and projection for the new manifest.
4. **Immutable storage:** qualify Git bindings, private content-addressed objects, dirty-buffer snapshots, retention, and GC behavior.
5. **MCP dispatch:** add versioned inspect-hydrated and canonical-execute routing while preserving 43/13 counts.
6. **Presentation:** qualify quiet metadata-only and target/projected LLM-facing policies with explicit omission accounting.
7. **Optional transient integration:** separately decide whether an additive operation-43 contract provides sufficient value without weakening ADR 0006.

Each stage is independently reviewable and does not imply authorization for the next.

## Qualification requirements

Implementation begins with assertion-specific RED tests. At minimum, qualification must establish:

1. `ASSERT_DEHYDRATED_SOURCE_NEW_FAMILY_NO_EXISTING_SCHEMA_MUTATION` — predecessor schema hashes remain unchanged and new IDs are additive.
2. `ASSERT_SOURCE_PROJECTION_PRESERVES_43_FULL_13_COMPACT` — no operation 44 and exact profile counts remain.
3. `ASSERT_ANCILLARY_SOURCE_SUPPORT_CONTRIBUTION_ZERO` — ancillary bindings cannot contribute graph support.
4. `ASSERT_SOURCE_COPRESENCE_CANNOT_CREATE_CALLS` — adding or changing source objects cannot alter graph facts.
5. `ASSERT_PROJECTED_SOURCE_CANONICAL_ORDER_AND_EXACT_RANGES` — input permutation produces byte-identical output.
6. `ASSERT_SOURCE_BYTE_NODE_RANGE_BUDGETS_RECONCILE` — candidates equal selected plus omitted and ranges remain whole.
7. `ASSERT_SOURCE_PARTIAL_AND_TRUNCATED_ARE_NONINTERCHANGEABLE` — unavailability and budget exhaustion remain distinct.
8. `ASSERT_DIGEST_ONLY_DIRTY_BUFFER_SOURCE_UNAVAILABLE` — digest-only dirty source cannot hydrate.
9. `ASSERT_GIT_HYDRATION_BINDS_EXACT_COMMIT_BLOB` — current workspace bytes cannot satisfy historical bindings.
10. `ASSERT_CONTENT_ADDRESS_HYDRATION_REJECTS_NEIGHBOR_BYTES` — wrong length or digest fails before extraction.
11. `ASSERT_SAME_CAPTURE_HYDRATION_ADDS_ZERO_GRAPH_FACTS` — graph bytes and identities remain unchanged.
12. `ASSERT_ADDITIONAL_ACQUISITION_HAS_NEW_IDENTITY_AND_LINEAGE` — reacquisition cannot reuse the original capture identity.
13. `ASSERT_SOURCE_PROJECTION_NO_RETRY_FALLBACK_REPAIR_BUDGET_INCREASE_RESUME` — resolver attempts and limits remain exactly declared.
14. `ASSERT_DEFAULT_LLM_PROJECTION_EXCLUDES_ANCILLARY_AND_RESTRICTED_BYTES` — defaults remain quiet while omissions stay discoverable.
15. `ASSERT_PUBLISHED_MANIFEST_SURVIVES_SOURCE_OBJECT_GC_WITH_TYPED_UNAVAILABLE` — graph verification survives allowed object collection.
16. `ASSERT_OFFLINE_REPLAY_REQUIRES_EXACT_V5_MANIFEST_AND_SOURCE_OBJECTS` — missing objects cannot trigger live substitution.
17. `ASSERT_V2_V3_READERS_UNCHANGED_AND_V5_AUTHORITATIVE` — historical fixtures remain valid and new production manifests bind V5.
18. `ASSERT_SOURCE_PROJECTION_DIRECT_EXECUTE_EXACT_PARITY` — direct and gateway envelopes and failures are identical.

Qualification also includes schema mutation fixtures, property/permutation tests, privacy markers, adversarial object-store cases, missing historical Git objects, dirty-buffer fixtures, no-replace publication races, and exact-byte golden vectors. Tests must not repair malformed input or increase limits to reach GREEN.

## Alternatives considered

### Fully hydrated V5-adjacent artifact

This provides simple offline replay but is rejected as the default because it maximizes privacy exposure, publication size, retention coupling, and prompt noise. Exact embedded bytes remain available as an explicit storage class and under `COMPLETE_CAPTURE` where authorized.

### Git-only dehydrated manifest

This is compact and deduplicated but is rejected as the sole storage model because Git history may be shallow, rewritten, or garbage-collected, and dirty buffers have no blob.

### Content-addressed storage only

This provides exact deferred hydration but duplicates immutable Git storage and introduces retention obligations for every committed file. The hybrid model is preferred.

### Flags directly on current operation 43

This is rejected for the initial implementation because operation 43 is a transient locator façade governed by ADR 0006 and closed immutable schemas. It remains a separately reviewed future possibility after offline qualification.

### New operation 44

Rejected because fixed operation counts are compatibility constraints and existing inspect-hydrated and canonical-execute dispatch can carry additive versions.

### Broaden the existing source-snapshot schema

Rejected because its immutable bytes and sibling-specific semantics cannot be repurposed.

### Hydrate from the current checkout

Rejected because mutable workspace bytes cannot establish historical identity. Such a read is a new acquisition.

### Treat acquisition-touched source as supporting evidence

Rejected because co-presence is ancillary acquisition context and cannot establish graph relevance or relations.

### Silent Git, content-store, or live fallback

Rejected because it obscures custody and revision origin and can substitute different bytes.

## Consequences

Routine LLM prompts can remain small and structurally focused while exact retained source stays available under explicit policy. Durable artifacts preserve custody and discoverability without requiring all captured bytes to travel with every response.

The design adds a new manifest family, source-object lifecycle, resolver admission, privacy policy, projection algebra, and GC obligations. Exact deferred hydration for dirty buffers requires retaining immutable bytes, so storage cannot be eliminated where replay is promised.

Separating graph evidence from source availability makes source collection and deletion more explicit. A graph artifact can remain valid after source becomes unavailable, but complete source replay cannot.

## Non-goals

This ADR does not:

- change operations 36 or 43;
- add operation 44;
- change the 43/13 tool counts;
- modify existing schema bytes;
- infer `CALLS` or any other relation from source;
- make transient results durable or hydration-eligible;
- claim source-graph completeness;
- authorize arbitrary filesystem reads;
- require source bodies in ordinary LLM prompts;
- implement or select a public source-object service;
- decide the package location of retention leases;
- authorize automatic fallback, retry, repair, or resumability.

## Open questions

- Which existing dispatch-version mechanism should select additive inspect-hydrated input and result contracts?
- Should source-object leases extend `internal/publication` or live in a dedicated package?
- Which source privacy classes and redaction projections are stable enough for public registration?
- What retained-object guarantees are required for historical Git hydration on Linux and macOS?
- Does a later additive operation-43 projection provide enough value to justify a new contract while preserving byte-identical legacy requests?
