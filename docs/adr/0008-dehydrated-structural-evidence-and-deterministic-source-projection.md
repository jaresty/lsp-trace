# ADR 0008: Separate retained structural evidence from deterministic source projection

- **Status:** Accepted
- **Date:** 2026-09-16
- **Decision owners:** LSP Trace maintainers
- **Scope:** Graph Provenance V5 source availability, deterministic source projection, and hydration

## Context

Structural evidence and source presentation serve different purposes. Graph Provenance V5 retains acquisition evidence for custody, verification, replay, and later inspection. An LLM answering a bounded code question usually needs only the exact target, admitted server-reported relations, and a small set of corresponding source ranges. Presenting every source document observed during acquisition can increase privacy exposure, payload size, token pressure, and semantic noise without adding graph authority.

The current transient structural-context path does not itself retain a large source bundle. Historical operations 36 and 43 return bounded source-locating structural results: workspace-relative paths, declaration ranges, call-site ranges, and bounded analytics. The underlying transient executor uses `CaptureSupply: false`; the result remains non-retained, non-replayable, publication-ineligible, and hydration-ineligible. ADR 0006 requires escalation to create a new durable acquisition identity rather than upgrading transient bytes in place.

Graph Provenance V5 remains the authoritative production acquisition contract. V2 and V3 remain historical readers. Existing V5, transient-result, source-snapshot, hydration, and publication schema bytes are immutable and cannot be broadened by reinterpretation.

The repository already has two relevant foundations:

1. private, content-addressed, no-replace publication with bounded selectors and no pathname fallback;
2. deterministic offline hydrated inspection with explicit selection, body opt-in, stable ordering, bounded accounting, and no acquisition or filesystem fallback.

A digest can verify bytes but cannot reconstruct bytes that were never retained. Exact deferred hydration therefore requires immutable retrievable bytes, not only a URI, document version, or digest.

## Decision

Introduce a V5-bound source-availability layer that is separate from graph authority and from LLM presentation.

The implementation will:

1. preserve historical operation and schema bytes as immutable readers without requiring those historical names to remain advertised;
2. expose one canonical transient product operation, `lsp_trace_v2_structural_context`, with an exclusive target union of either one exact symbol or one exact URI/line/character position;
3. remove `lsp_trace_v1_structural_context_symbol` and `lsp_trace_v2_structural_context_symbol` from product discovery rather than retain legacy aliases;
4. target exactly 41 canonical operations and 12 compact-advertised tools, with the unified operation replacing both symbol-only entries in compact discovery;
5. define a new immutable `lsp-trace.dehydrated-source-manifest.v1` artifact bound to exact admitted Graph Provenance V5 bytes;
6. define a deterministic source-projection result family shared by retained and live custody modes;
7. distinguish exact evidence ranges, server item ranges, and provenance-qualified structural display ranges without allowing any source range to establish graph facts;
8. permit live projection to acquire an explicit, bounded, request-ephemeral set of additional caller/callee document supplies from the same managed session generation when projection policy requests cross-document bodies; these bytes are never retained, published, cached, or converted into source objects;
9. add no operation 44;
10. preserve operation 33 as retained acquisition and keep semantic Describe, Embed, and indexing outside this implementation.

This ADR does not authorize runtime or schema implementation by itself. Each implementation stage requires assertion-specific RED evidence before production changes.

## Owner selection amendment: hybrid shared algebra

The owner selects **HYBRID_SHARED_ALGEBRA** from the qualification plan's reviewed three-candidate comparison. The selected design composes with and specializes:

```text
Select(logical request) → Resolve(custody contract) → Assemble(projected records)
```

The shared algebra owns logical projected-unit, selection, accounting, privacy, range, and citation semantics. `Resolve` is custody-specific: retained projection binds exact admitted Graph Provenance V5, manifest availability, and immutable source objects, while live projection binds one exact managed session generation and an explicitly selected, bounded set of request-admitted document supplies. The target document is mandatory; additional documents are separately selected and acquired only for already-admitted projected units. Physical projection identities, statuses, and downstream ADR 0007 admission/cache identities remain custody-specific.

The dehydrated manifest owns retained custody and availability only. It is not the owner of projection semantics. A shared logical unit does not create a shared physical identity: even byte-identical retained and live source selections are not thereby projection-equal, admission-equal, or cache-equal.

Acquisition, projection, and semantic accounting are independent. Structural acquisition status and graph/frontier accounting cannot be rewritten by projection; projection selection, omissions, privacy, availability, and limits cannot be rewritten by semantic processing; semantic Describe, Embed, index, search, or grouping outcomes cannot alter either structural or projection accounting. Every layer retains its own denominator, terminal disposition, identity, and authority ceiling.

Qualification execution remains `NOT_EXECUTED`; no qualification cell is `PASS` and no runtime implementation is qualified. The canonical cross-mode fixture now supports owner adjudication of D3–D12: domain-separated logical-unit, occurrence, citation, and custody-specific physical identities; explicit UTF-16 half-open ranges; finite projection dispositions and omission causes; body-versus-policy privacy separation; whole-range range/byte accounting; overlap accounting; successful-empty semantics; canonical ordering; and semantic-cache deferral. Fixture hash values remain noncanonical until regenerated from the frozen preimages. D1–D2 are revised by the unified operation decision and still require exact request/result schema identities and golden bytes.

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

Historical operation-43 schema bytes remain immutable inputs for historical readers, but the operation is removed from product discovery and routing. `lsp_trace_v2_structural_context` owns both exact-symbol and exact-position targets and does not infer a projection mode; callers select projection explicitly.

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
- source entries with role, graph subject, source identity, exact evidence range, optional server item range, optional structural display range and provenance, position encoding, digest, document custody binding, and optional body;
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

## Product-surface and operation strategy

Existing schema bytes remain registered and valid as historical readers. Product discovery is independently consolidated:

- `lsp_trace_v2_structural_context` is the sole canonical transient context operation;
- its target is an exclusive union of one exact symbol or one exact URI/line/character position;
- `lsp_trace_v1_structural_context_symbol` and `lsp_trace_v2_structural_context_symbol` are removed from advertised product discovery rather than retained as aliases;
- full discovery targets 41 canonical operations;
- compact discovery targets 12 tools by replacing both symbol-only entries with `lsp_trace_v2_structural_context`;
- no operation 44 is added.

The new immutable contract families remain additive to historical schema bytes:

- `lsp-trace.dehydrated-source-manifest.v1`;
- a source-projection request/result family shared by retained and live custody;
- additive inspect-hydrated input and output versions where retained projection remains exposed.

The existing `lsp_trace_v1_inspect_hydrated` operation and `lsp_trace_v1_execute` gateway route retained additive contracts. Unified live direct, gateway, and CLI-context paths must return identical delegated projection records and typed failures. The existing sibling-oriented `lsp-trace.graph-v5-source-snapshot.v1` bytes and semantics are not broadened.

## Migration stages

1. **Product surface:** replace historical symbol-only discovery with one unified context target union; preserve historical schema readers without advertising compatibility aliases.
2. **Projection algebra:** freeze D3–D12 identities, statuses, ordering, budgets, privacy, overlap, empty-result, and semantic-deferral semantics from the canonical fixture; regenerate fixture hashes from the frozen preimages.
3. **Schema families:** add new immutable unified-context, manifest, and projection schemas without changing predecessor bytes. Structural-display and multi-document acquisition require successor request, source-projection result, unified-result, success-envelope, and domain-error identities.
4. **Range preservation and syntax resolution:** preserve server selection, item, and call-site ranges independently; resolve optional structural display ranges only from admitted bytes through qualified deterministic adapters.
5. **Shared core:** reuse deterministic hydrated-evidence selection, extraction, overlap, privacy, and accounting beneath custody-specific retained and live resolvers.
6. **Bounded ephemeral live document acquisition:** select target-first canonical document sets after traversal, acquire each exact in-memory supply once under independent limits, retain only typed per-document outcomes in the response, and erase references to raw supplies when the request ends.
7. **Immutable storage:** qualify Git bindings, private content-addressed objects, dirty-buffer snapshots, retention, and GC behavior.
8. **Unified transports:** qualify direct MCP, canonical execute gateway, and CLI context routing for the unified V2 operation, targeting 41 canonical / 12 compact discovery.
9. **Presentation:** qualify quiet metadata-only and target/projected LLM-facing policies with explicit omission accounting.

Each stage is independently reviewable and does not imply authorization for the next.

## Repository-grounded review: interactive and retained projection

A review of the current production paths supports one projection algebra serving two distinct custody modes, but not one interchangeable evidence class:

- **Transient live bounded projection** is an interactive presentation over one exact managed session generation. Historical operations 36 and 43 already return workspace-relative paths plus endpoint item ranges and relation call-site ranges; real managed `gopls` evidence demonstrates that those item ranges may be identifier-sized and cannot be assumed to contain full declarations. The unified product operation retains that structural core while accepting an exclusive symbol-or-position target union. The target document is prepared and captured first. When an explicit projection policy requests cross-document structural bodies, the operation may deterministically select and prepare a bounded set of additional documents referenced only by already-admitted server-reported endpoints or relation occurrences. Every document supply remains request-scoped, version-bound, independently accounted, and ephemeral. It is discarded when the request terminates and is never written to retained artifacts, manifests, source-object stores, publication roots, semantic caches, logs, or qualification fixtures. The result remains `authority: 0`, `source_graph_complete: UNKNOWN`, non-replayable, publication-ineligible, and unable to add graph facts.
- **Retained V5-bound offline projection** is hydration over immutable bytes bound to an admitted Graph Provenance V5 capture and dehydrated manifest. The existing hydrated-inspection path is offline, path-free after ingress, explicitly body-gated, deterministically ordered, overlap-aware, bounded, and independently validated. Missing retained bytes remain typed and never trigger current-checkout or live-session acquisition.

These modes may share ordering, range extraction, overlap handling, privacy vocabulary, omission reasons, and projection accounting only if their custody and result identities remain explicit. A transient projection cannot become retained evidence, and retained hydration cannot consult the live workspace. Source text remains a zero-authority view over already admitted server-reported `CALLS`; it cannot create, repair, complete, or strengthen graph facts.

### Range and disclosure contract

Location-only output remains the default in both modes. Bodies or snippets require an explicit opt-in plus independent byte, range, object, work, response, and, where applicable, page limits. The projection contract distinguishes:

- endpoint server item and selection ranges, attached to admitted nodes; and
- relation call-site ranges, attached to admitted server-reported `CALLS` occurrences.

Neither range kind substitutes for the other. Each projected unit distinguishes:

- an **evidence range**: the exact server-reported selection or call-site range that anchors the citation;
- an optional **server item range**: the `CallHierarchyItem.range` admitted during traversal, preserved without claiming that it encloses a declaration body; and
- an optional **structural display range**: a broader range resolved from already-admitted source bytes by a language-qualified deterministic syntax adapter and used only for presentation.

A structural display range must contain or otherwise exactly bind its evidence range under the adapter contract. Its provenance and kind are explicit, for example `GO_AST_ENCLOSING_DECLARATION`; it cannot add, repair, infer, merge, or strengthen nodes or relations. If no qualified adapter or admitted bytes exist, the evidence range remains valid while display expansion receives a typed terminal disposition.

Overlapping selected display ranges are processed in canonical order and may share one emitted span only when every original evidence selection, display selection, and citation remains separately attributable and accounting remains exact. Projection status and accounting are independent of structural status: unavailable, withheld, invalid, or budget-truncated source cannot rewrite structural success or failure. Projection candidates must reconcile as selected plus omitted, with one mutually exclusive reason per omission.

### Bounded multi-document live acquisition

Cross-document live source is available only through an explicit acquisition policy on the unified request. Multi-document document supplies are exclusively a transient live concern. Retained projection never performs this acquisition: it may resolve only immutable source objects already admitted by its existing V5/manifest custody contract, and a live request can neither create nor populate those objects. Structural traversal completes first. The operation then selects document URIs solely from already-admitted endpoint and server-reported relation units. Canonical document ordering is:

1. the target document;
2. remaining selected document URIs in lexicographic order.

Selection and acquisition are distinct. A document excluded by structural or projection-unit selection is never prepared merely because traversal observed it. Each admitted document is prepared at most once for the exact session generation through an explicit bounded `sessionruntime` live-document operation and captured in memory as a full-text `LSP_SUPPLIED` document supply. That managed preparation may source the selected URI from the exact current workspace solely to synchronize it into the same live session generation; this is the primary authorized live acquisition, not a fallback and not retained evidence. The supply lifetime is bounded to that operation invocation and ends after response assembly or failure. Projection and syntax packages may not open paths directly. After managed preparation fails, no current-checkout retry, Git, retained-object, network, alternate-session, or alternate-resolver path may satisfy the document.

The request declares independent hard limits for additional documents, per-document bytes, total acquired source bytes, document requests, protocol messages, acquisition work, projection work, returned source bytes, and complete response bytes. Each document is admitted atomically; limits never cause partial document custody or a hidden increase. Target-document failure retains the existing structural failure. Additional-document failures are terminal for that document but do not rewrite already established structural facts.

Document accounting exposes candidates, selected, acquired, unavailable, withheld, limit-omitted, total acquired bytes, and one terminal disposition per selected document. Required typed causes include `DOCUMENT_LIMIT`, `SOURCE_BYTE_LIMIT`, `DOCUMENT_UNAVAILABLE`, `DOCUMENT_VERSION_CHANGED`, and `POLICY_WITHHELD`. No retry or implicit continuation is permitted.

Structural display resolution runs only after document admission. For the first qualified adapter, Go declarations are resolved with `go/parser` over the captured in-memory bytes; parsing does not read imports or the filesystem and contributes zero graph authority. Other languages use explicitly qualified adapters or exact-evidence fallback. Relation call-site evidence remains the server-reported `fromRange`; caller declaration display is a separate presentation range.

The repository review also exposed a bounded live-tracing usability risk. Valid exact-symbol requests at bounded depth returned typed `TRAVERSAL/TRUNCATED` and `ADMISSION/RESOURCE_LIMIT` outcomes, but the responses did not identify the exhausted resource, report observed-versus-limit accounting, state whether any partial graph or frontier remained available, or name an actionable safe adjustment. The host rendering additionally appended `Expected parameters` after these valid domain failures, making them resemble input-schema errors. Subsequent depth-1 requests succeeded against the same managed session, so this is evidence about bounded-limit diagnostics and usability, not evidence of a dead session or unsupported operation.

Source-projection qualification must treat that risk explicitly. Projection accounting and status remain separate from structural traversal accounting and status. Opting into bodies or snippets must not silently consume structural request, message, node, depth, or traversal-byte budgets, and a projection limit or source failure must not convert structural success into structural failure. Conversely, projection must not hide a structural `TRUNCATED` or `RESOURCE_LIMIT` outcome. Typed failures must identify the exhausted budget, report observed and declared-limit values where safely available, state the disposition of any partial result or frontier, and distinguish actionable bounded adjustments from forbidden hidden retries or budget increases. Parameter-help text must not be appended to valid domain failures; schema guidance is reserved for actual request-validation errors.

Both custody modes require deterministic canonical ordering, whole-range admission, position-encoding-aware extraction, no code-point splitting, explicit privacy classification, and terminal typed outcomes for unavailable or unverifiable bytes. Explicit bounded live preparation of a selected workspace document is permitted only through `sessionruntime`; direct path access by projection code and any retry, resolver fallthrough, post-failure current-checkout substitution, range repair, encoding guess, hidden budget increase, or implicit continuation remain prohibited. Revision custody is exact and mode-specific: the live session generation qualifies transient reads, while the V5 capture, manifest, and immutable source object qualify retained reads.

### Staged recommendation

Retained and live qualification are independent custody tracks under the selected shared algebra; retained qualification is not a semantic prerequisite for bounded live projection:

1. Preserve the implemented V1 canonical fixture for D3–D12, then add an immutable successor fixture to freeze D13–D16 for evidence/display separation, display provenance, bounded document acquisition, and independent document accounting.
2. Define one unified request/result contract for `lsp_trace_v2_structural_context`, with exactly one symbol or position target and explicit projection options.
3. Route symbol targets through locator-only workspace-symbol resolution, then delegate the exact URI target to the same structural/projection core used by position targets.
4. Remove both symbol-specific names from full discovery; replace both compact symbol entries with the unified context operation.
5. Keep operation 33 on retained post-acquisition projection. Source presentation occurs from the resulting retained identity and manifest, not through a competing transient body channel.
6. Retain immutable historical schema readers even when their legacy operation names are no longer advertised or routed.

This ordering adds no operation 44, targets exactly 41 canonical and 12 compact-advertised tools, and preserves immutable predecessor schema bytes. It does not itself authorize runtime or schema implementation. Every implementation stage begins with assertion-specific RED evidence. Qualification must prove independent structural/projection budgets, explicit partial/frontier disposition, no parameter-help decoration on valid domain failures, and direct/gateway/CLI parity for the unified operation.

## Qualification requirements

Implementation begins with assertion-specific RED tests. At minimum, qualification must establish:

1. `ASSERT_DEHYDRATED_SOURCE_NEW_FAMILY_NO_EXISTING_SCHEMA_MUTATION` — predecessor schema hashes remain unchanged and new IDs are additive.
2. `ASSERT_UNIFIED_CONTEXT_41_FULL_12_COMPACT_NO_OPERATION_44` — both symbol-only names are absent, the unified context operation is compact-advertised, and exact profile counts are 41/12.
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
19. `ASSERT_EVIDENCE_ITEM_DISPLAY_RANGES_DISTINCT_AND_PROVENANCED` — exact citations remain stable while optional display ranges are independently identified and provenance-qualified.
20. `ASSERT_GOPLS_ITEM_RANGE_NOT_ASSUMED_DECLARATION_BODY` — identifier-sized server item ranges cannot be relabeled as declaration bodies.
21. `ASSERT_LIVE_DOCUMENT_SELECTION_TARGET_FIRST_CANONICAL` — selected document acquisition is target-first and permutation-invariant.
22. `ASSERT_CROSS_DOCUMENT_CALLER_BODY_REQUIRES_EXACT_SUPPLY` — caller bodies require an exact request-admitted supply for that URI.
23. `ASSERT_MULTI_DOCUMENT_LIMITS_ATOMIC_AND_RECONCILED` — document, per-document byte, total-byte, request, message, work, projection, and response limits remain independent and exactly accounted.
24. `ASSERT_ADDITIONAL_DOCUMENT_FAILURE_CANNOT_REWRITE_CALLS` — additional-document failures leave server-reported structural facts unchanged.
25. `ASSERT_SYNTAX_DISPLAY_RESOLUTION_READS_ADMITTED_BYTES_ONLY` — syntax adapters cannot read imports, files, alternate resolvers, or ambient workspace bytes.
26. `ASSERT_DIRECT_GATEWAY_CLI_MULTI_DOCUMENT_PARITY` — all transports preserve document custody, display ranges, omissions, and typed failures.

Qualification also includes schema mutation fixtures, property/permutation tests, privacy markers, adversarial object-store cases, missing historical Git objects, dirty-buffer fixtures, no-replace publication races, and exact-byte golden vectors. Tests must not repair malformed input or increase limits to reach GREEN.

## Alternatives considered

### Fully hydrated V5-adjacent artifact

This provides simple offline replay but is rejected as the default because it maximizes privacy exposure, publication size, retention coupling, and prompt noise. Exact embedded bytes remain available as an explicit storage class and under `COMPLETE_CAPTURE` where authorized.

### Git-only dehydrated manifest

This is compact and deduplicated but is rejected as the sole storage model because Git history may be shallow, rewritten, or garbage-collected, and dirty buffers have no blob.

### Content-addressed storage only

This provides exact deferred hydration but duplicates immutable Git storage and introduces retention obligations for every committed file. The hybrid model is preferred.

### Preserve separate symbol-only product operations

Rejected because there are no external users requiring compatibility aliases. Locator-only symbol resolution becomes one target arm of the unified context operation; historical schemas may remain readable without remaining advertised.

### New operation 44

Rejected because the existing `lsp_trace_v2_structural_context` name owns the unified capability; a new number would recreate the duplication this amendment removes.

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

- add operation 44;
- retain either symbol-specific operation as an advertised compatibility alias;
- reduce compact discovery below 12 tools;
- modify existing schema bytes or delete historical readers;
- infer `CALLS` or any other relation from source;
- make transient results durable or hydration-eligible;
- claim source-graph completeness;
- authorize arbitrary filesystem reads;
- require source bodies in ordinary LLM prompts;
- implement or select a public source-object service;
- retain, publish, cache, log, or write live multi-document supplies into manifests, source-object stores, qualification artifacts, or any durable carrier;
- decide the package location of retention leases;
- authorize automatic fallback, retry, repair, or resumability.

## Open questions

- Which existing dispatch-version mechanism should select additive inspect-hydrated input and result contracts?
- Should source-object leases extend `internal/publication` or live in a dedicated package?
- Which source privacy classes and redaction projections are stable enough for public registration?
- What retained-object guarantees are required for historical Git hydration on Linux and macOS?
- What exact immutable successor request/result/envelope identities should encode range policy, bounded multi-document acquisition, document accounting, and structural display provenance?
- Which language adapters beyond the first Go `go/parser` implementation are eligible for qualification, and what exact fallback vocabulary applies when no adapter is available?
