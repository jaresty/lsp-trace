# ADR 0007 / ADR 0008 source-projection qualification plan

Status: **MODEL_SELECTED — HYBRID_SHARED_ALGEBRA**

Qualification execution: **NOT_EXECUTED**

Machine-checkable matrix: [`qualification/adr0008-source-projection-matrix.v1.json`](../../qualification/adr0008-source-projection-matrix.v1.json)

## Claim boundary

This is a documentation-only qualification plan. It records the owner-selected design model but does not qualify implementation, report passing tests, authorize schema or runtime work, enable ADR 0007 inference/indexing, or authorize retained or transient projection. Matrix execution remains `NOT_EXECUTED`, `tests_pass_claimed=false`, and `implementation_qualified=false`; no qualification cell becomes `PASS`. Every implementation stage still requires attributable assertion-specific RED evidence and retained GREEN evidence at one reviewed revision.

## Why the barrier exists

ADR 0008 defines exact graph-bound source projection and two materially different custody modes: immutable retained objects bound to Graph Provenance V5, and possible future live-session projection on operations 43 then 36. ADR 0007 defines optional semantic consumers whose admissions, identities, caches, coverage claims, privacy, deletion, and authority ceilings must remain exact.

The unresolved design question is therefore not merely artifact-first versus query-first API shape. It is which shared projection-unit semantics, if any, should feed ADR 0007 TARGET, NEIGHBORHOOD, Describe, Embed, and index-build while retained and live resolvers preserve distinct custody. The dehydrated manifest may be the retained custody/availability index without owning general projection semantics. Bounded operation-43 live evidence may change the shared unit contract before freeze; retained-first is not a hard semantic dependency.

The reviewed comparison considered three models:

1. **Artifact-first retained:** exact units originate from retained V5 plus a manifest/availability index. This over-assigns the retained carrier unless projection semantics are separately factored out.
2. **Query-first shared:** one request algebra selects units and delegates to retained-object or live-session resolvers. This captures shared selection but under-specifies custody-specific physical identity and semantic admission.
3. **Hybrid shared algebra — selected:** shared logical projected-unit, selection, accounting, privacy, range, and citation semantics compose with and specialize **Select → Resolve → Assemble**. Retained V5 and live session-generation custody keep distinct `Resolve` contracts, physical projection identities, statuses, and ADR 0007 semantic-admission/cache identities.

The manifest owns retained custody and availability only; it does not own projection semantics. The same logical projected unit, and even identical source bytes, do not imply retained/live projection identity, semantic-admission identity, or cache equality. This selection changes the design barrier to `MODEL_SELECTED`; all five qualification tracks and all 23 cells retain their existing verdicts, while execution remains `NOT_EXECUTED`.

## Smallest next artifact

Before any wire contract is frozen, build one canonical cross-mode fixture containing exactly one endpoint and one server-reported `CALLS` relation represented twice:

- under exact retained Graph Provenance V5 custody, including its manifest availability binding and immutable source-object resolution; and
- under exact live managed-session-generation custody, including request-admitted document identity and bytes.

For both custody modes, the fixture must carry `TARGET` and bounded `NEIGHBORHOOD` outputs, proposed semantic-admission and cache preimages, metadata-only/body-eligible/restricted/withheld/unavailable privacy variants, and failure denominators that distinguish selected, omitted, evaluated, terminal, and unevaluated members. Golden vectors must make cross-mode non-equality visible even when the logical unit and source bytes are identical.

This fixture is the input for D1–D12 decisions about canonical bytes, projected-unit and citation preimages, aggregate status precedence, privacy vocabulary, and schema versions. Those decisions remain pending; this plan does not freeze detailed public schemas, identities, status names, precedence, privacy terms, or versions.

## Qualification architecture

### 1. Common projected-unit contract

Common qualification is conditional on model selection and covers only semantics that can remain custody-neutral:

- exact unit identity: evidence role, graph subject, logical source, range, encoding, digest/length, custody identity, privacy, availability, and policy;
- endpoint declaration versus server-reported relation call-site citations;
- canonical ordering, overlap attribution, whole-range admission, and `candidates = selected + omitted`;
- independent object/node, range, byte, work, response, and page limits;
- metadata-only defaults, explicit body eligibility, privacy exclusions, and discoverable omissions;
- malformed input rejection without repair, retry, fallback, hidden budget increase, or implicit continuation;
- `graph_facts_added: 0`, semantic `authority: 0`, `accepted: false`, and `source_graph_complete: UNKNOWN`.

Common semantics do not imply common custody. A shared logical unit under two custody modes is not cache-equal unless every custody-sensitive identity component is equal under the selected model.

### 2. Retained-object resolver qualification

Retained qualification binds exact admitted V5 bytes to immutable retrievable bytes. A manifest candidate is evaluated as a retained availability/custody index; broader semantic ownership is not presumed.

Required evidence covers exact commit/tree/blob retrieval, content-addressed dirty and non-Git objects, embedded immutable bytes, digest-only unavailability, owner-only pinned roots, canonical selectors, atomic no-replace publication, retention/leases/GC, and cold replay. Missing, corrupt, withheld, collected, shallow-history, rewritten-history, or neighbor bytes must yield typed terminal outcomes. No current-checkout, live-session, provider, network, alternate-resolver, or pathname fallback is permitted.

Graph and manifest custody metadata remain valid when source becomes unavailable. Unavailability, privacy withholding, corruption, and budget truncation remain noninterchangeable.

### 3. Live-session resolver qualification

Future transient qualification remains separate and does not become retained evidence. It binds reads to one exact managed session ID/generation and request-admitted document identity/version/bytes/encoding.

Operation 43 is evaluated first because it is the exact-symbol façade. Its bounded trials may reveal shared identity, citation, privacy, selection, or status requirements before contract freeze. Only after operation 43 qualifies may operation 36 reuse the selected semantics for exact-URI callers; operation 36 must not route through workspace-symbol lookup.

Live gates require:

- independent structural and projection budgets;
- exhausted-resource diagnostics with safe observed/limit values;
- explicit partial graph and frontier disposition;
- host-owned parameter help only for request-validation failures, never valid domain failures;
- body/snippet opt-in that changes projection only and cannot rewrite structural facts, status, frontier, authority, completeness, or budgets;
- omitted fields preserving legacy request/result bytes;
- no retained, replay, publication, or hydration eligibility for transient output.

### 4. ADR 0007 interoperability qualification

Exact projected units are candidate typed admissions, not semantic facts. Qualification must cover:

- **Acquisition mode:** one projected unit may be `TARGET`; declared structural expansion may form `NEIGHBORHOOD`; neither implies `CENSUS`.
- **Semantic operations:** exact units may feed item-independent Describe, Embed, and index-build only after typed admission. The worker receives only caller-admitted bytes and relationships.
- **Identity/cache:** equality requires exact admission, bytes or immutable selector, range, encoding, role, custody identity, revision or session generation, representation, prompt/model/runtime/policy, and ordered dependencies. Family, approximate, cross-revision, cross-generation, or cross-custody matches are misses.
- **Citations:** outputs cite exact endpoint or relation ranges and immutable admission IDs. Relation ranges remain attached only to admitted server-reported `CALLS`; endpoint ranges never substitute for call sites.
- **Authority:** retained V5-bound units remain revision-bound code/structural evidence; live units remain explicitly typed working context. Similarity cannot equalize them. Every generated description, embedding, index, search, group, and context packet remains `authority: 0`, `accepted: false`, and unable to add or repair graph facts or `CALLS`.
- **Coverage:** results expose TARGET/NEIGHBORHOOD/CENSUS, exact scope, denominator, evaluated count, terminal outcomes, failures, exclusions, revision policy, and coverage. Partial, bounded-zero, and non-complete results cannot imply absence, repository completeness, domain completeness, feature identity, or completed work.
- **Privacy/deletion:** only body-eligible units become semantic representations. Derived descriptions, embeddings, indexes, queries, caches, logs, and backups remain source-sensitive; deletion invalidates dependents and prevents stale cache reappearance.

ADR 0007 prerequisite schemas, policies, owners, evaluation thresholds, and execution authority remain separately required. Passing source-projection qualification would not authorize the pilot.

## Compatibility boundaries

The barrier rejects any proposal that:

- changes operations 33, 36, 41, or 43 outside separately qualified additive versions;
- adds operation 44 or changes the exact 43 canonical / 13 compact counts;
- mutates predecessor schema bytes or broadens `lsp-trace.graph-v5-source-snapshot.v1`;
- gives operation 41 source flags, operation 33 a competing transient body channel, or ADR 0007 a core CLI/MCP surface;
- treats hydration, source text, co-presence, names, proximity, embeddings, or model output as graph support;
- conflates retained-object custody with live-session working context.

Direct inspect-hydrated and canonical execute paths must return byte-identical delegated envelopes and typed failures for any additive retained contract. Any future live direct/gateway exposure requires an independent parity claim; host rendering is tested separately from operation result parity.

## Evidence and verdict rules

Each matrix cell names exact PASS evidence, discriminating FAIL evidence, blockers, and upstream dependencies.

- **PASS:** attributable artifacts prove the named property at one reviewed revision. Required evidence includes exact commands, inputs, outputs, assertion reports, digests, and environment/custody identity appropriate to the cell.
- **BLOCKED:** prerequisite evidence, authority, policy, model selection, platform capability, or required artifact is absent. BLOCKED is never promoted because a fixture or document exists.
- **FAIL:** the qualifying execution ran and violated the named assertion, or a persisted mutation escaped its intended guard.
- **NOT_RUN:** no qualifying execution occurred.

Assertion-specific RED must compile and execute far enough to fail the intended assertion. Setup failures and unrelated assertion failures are not RED evidence. Structural invariants already passing before repair must be labeled invariant guards rather than invented RED. Tests must not repair malformed input, broaden policy, change resolver, or increase limits to reach GREEN.

## Integration barrier procedure

1. Preserve the reviewed comparison and `HYBRID_SHARED_ALGEBRA` selection as design input, not qualification evidence.
2. Produce and review the canonical cross-mode fixture and D1–D12 golden vectors before freezing any common projection-unit wire contract.
3. Verify manifest and object-store barrier inputs independently; the manifest remains only a retained custody/availability index and does not own projection semantics.
4. Execute common, retained-object, and live-session tracks independently. Custody-specific PASS cannot be borrowed across modes.
5. Execute ADR 0007 interoperability only after its own frozen prerequisites exist. Projection PASS is necessary input evidence, not semantic-pilot authorization.
6. Require every applicable cell to be PASS at one reviewed revision. Any BLOCKED, FAIL, NOT_RUN, mixed revision, count drift, predecessor mutation, or claim-ceiling violation blocks integration.
7. Preserve the matrix and evidence packet as barrier inputs; do not rewrite historical outcomes after model selection.

## Critical blockers now

- The semantic model is selected, but the canonical cross-mode fixture and D1–D12 golden vectors have not been executed or reviewed.
- Common projected-unit and citation wire identities are not frozen.
- Manifest and object-store tracks have not supplied canonical bytes, validators, immutable-object custody receipts, GC/lease evidence, or adversarial outcomes.
- No authorized operation-43 live projection trial or host-rendering evidence exists.
- ADR 0007 typed admission, identity, privacy/deletion, terminal-accounting, ownership, and evaluation prerequisites remain unexecuted.

Documentation resolves only the model-selection barrier. It resolves none of the execution, fixture, wire-contract, custody-specific qualification, ADR 0007 prerequisite, or implementation blockers.
