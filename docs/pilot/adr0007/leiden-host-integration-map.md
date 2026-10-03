# ADR 0007 CALLS-only Leiden/host integration map

- **Scope:** historical server-reported `CALLS` only
- **Status:** historical reconciliation plus authorized integration-first execution sequence; no acceptance or public enablement asserted
- **Authority:** `0`; **accepted:** `false`; **completeness:** `UNKNOWN`

This map reconciles the implemented Program C path with ADR 0007's engineering-context index goal and the caller-provided-inference amendment. A Leiden community is a structural candidate group, not a feature. Host interpretation changes the interpretation provider; it does not select topology or membership and does not replace independent semantic review.

## Evidence basis and limits

The cited source and focused tests were inspected for this reconciliation; tests were **not rerun**. Managed LSP session `sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283`, generation `1`, was READY with `gopls`, `utf-16`, and dirty revision `40bac53c0b1b1dc3f4ec12653666c6b59ee148d2`. Managed evidence proved selected upstream `CALLS`. Some function probes failed or were partial, so direct source evidence below supports code connectivity but does not establish server-reported `CALLS` for every integration edge.

## End-to-end map at the original reconciliation checkpoint

The statuses below preserve the original inspection boundary, not a live implementation inventory. Subsequent parent reports describe private mechanical group-artifact/storage work and review defects, but no accepted source-backed group interpretation. The current execution sequence below supersedes the earlier fixture-first ordering without claiming those defects are resolved.

| Edge | Status | Evidence and boundary |
|---|---|---|
| Historical server-reported `CALLS` → admitted graph | **Implemented; focused tests inspected** | Composition preserves bounded nodes, edges, occurrences, identities, completeness, and a CALLS-only claim ceiling ([compose.go:20-30](../../../internal/programccompose/compose.go#L20-L30), [compose.go:260-318](../../../internal/programccompose/compose.go#L260-L318)). Admission revalidates the composite and projects exact CALLS endpoints/occurrences ([admission.go:130-199](../../../internal/programcadmission/admission.go#L130-L199)). Inspected tests cover multiplicity, direction, loops, isolates, and stable IDs ([admission_test.go:145-173](../../../internal/programcadmission/admission_test.go#L145-L173), [core_test.go:167-180](../../../internal/programc/core_test.go#L167-L180)). |
| Admitted `CALLS` → Leiden partition | **Implemented; focused tests inspected** | Program C profile `calls-v1` uses Gonum Leiden with an explicit seed and conservative claim ceiling ([core.go:24-31](../../../internal/programc/core.go#L24-L31), [core.go:328-330](../../../internal/programc/core.go#L328-L330), [core.go:374-395](../../../internal/programc/core.go#L374-L395)). The partition is structural and is not a feature. |
| Partition → representative nominations | **Implemented; focused tests inspected** | The adapter sends every community to mechanical representative qualification ([adapter.go:124-147](../../../internal/censusprogramc/adapter.go#L124-L147)). Selection preserves complete `CommunityMembers`, can remain unresolved, and establishes no semantic identity ([representative.go:1-3](../../../internal/programcrepresentative/representative.go#L1-L3), [representative.go:80-139](../../../internal/programcrepresentative/representative.go#L80-L139), [representative.go:336-375](../../../internal/programcrepresentative/representative.go#L336-L375)). Inspected tests cover nearest/SCC selection, unresolved/empty outcomes, determinism, and cloning ([representative_test.go:72-103](../../../internal/programcrepresentative/representative_test.go#L72-L103), [representative_test.go:130-153](../../../internal/programcrepresentative/representative_test.go#L130-L153), [representative_test.go:262-275](../../../internal/programcrepresentative/representative_test.go#L262-L275)). Representatives are navigation anchors, not exhaustive evidence. |
| Crossing edges/helpers → structural presentation | **Implemented; focused tests inspected; presentation-only** | The non-authoritative view joins every member and presents crossing calls while disclaiming feature identity and completeness ([presentation.go:19-22](../../../internal/programcpresentation/presentation.go#L19-L22), [presentation.go:189-229](../../../internal/programcpresentation/presentation.go#L189-L229)). Inspected mutation cases fail closed on incomplete or inconsistent joins ([join_test.go:37-50](../../../internal/programcpresentation/join_test.go#L37-L50), [join_test.go:91-112](../../../internal/programcpresentation/join_test.go#L91-L112)). This does not provide semantic grouping. |
| Representative nomination → bounded target packet | **Implemented; focused tests inspected** | Packet construction iterates nominations, reconciles retained projection/custody, and hydrates the selected representative plus selected consumer alternatives ([packet.go:183-225](../../../internal/targetpacket/packet.go#L183-L225), [packet.go:270-305](../../../internal/targetpacket/packet.go#L270-L305), [packet.go:314-355](../../../internal/targetpacket/packet.go#L314-L355), [packet.go:623-650](../../../internal/targetpacket/packet.go#L623-L650)). It does **not** hydrate every community member. Inspected custody tests bind and reject altered graph bytes/schema ([adr0007_custody_test.go:70-160](../../../internal/targetpacket/adr0007_custody_test.go#L70-L160)). |
| Target packets → per-target descriptions → V2 provisional catalog | **Pipeline-integrated; semantic quality unqualified** | Continuation invokes the V2 worker per request and stores a V2 catalog artifact ([pipeline.go:680-724](../../../internal/censuscontinuation/pipeline.go#L680-L724), [pipeline.go:845-875](../../../internal/censuscontinuation/pipeline.go#L845-L875)). V2 copies validated response records without semantic merge/reinterpretation ([catalog_v2.go:35-132](../../../internal/provisionalfeaturecatalog/catalog_v2.go#L35-L132)); the production prompt remains an experimental target-level baseline ([v2_production.go:3-8](../../../internal/describerequest/v2_production.go#L3-L8)). The catalog is response/nomination-level, not group-level. |
| Group-level, member-complete evidence/interpretation artifact | **Unavailable: first missing integration edge** | No located artifact assembles every Leiden member and its terminal hydration outcome into one bounded group packet for host interpretation. |
| Engineering-context indexed artifact | **Proposed; no located implementation** | ADR 0007 specifies immutable admission/derived identity, invalidation, and correction lineage ([ADR 0007:121-146](../../adr/0007-optional-local-semantic-feature-index.md#L121-L146)) plus terminal accounting ([ADR 0007:222-239](../../adr/0007-optional-local-semantic-feature-index.md#L222-L239)). This reconciliation does not claim those requirements are implemented for CALLS-only groups. |
| Search/drill-down | **Proposed; no located CALLS-only implementation** | The ADR states the engineering retrieval ceiling ([ADR 0007:106-117](../../adr/0007-optional-local-semantic-feature-index.md#L106-L117)) and closed search/group accounting ([ADR 0007:226-239](../../adr/0007-optional-local-semantic-feature-index.md#L226-L239)). No semantic search is included in the next slice. |
| Correction/supersession | **Implemented and tested only for V1; not integrated with active V2** | V1 defines append-only correction/predecessor structures and successor construction ([catalog.go:150-205](../../../internal/provisionalfeaturecatalog/catalog.go#L150-L205), [catalog.go:631-687](../../../internal/provisionalfeaturecatalog/catalog.go#L631-L687)); the inspected test preserves predecessor bytes and rejects foreign/semantic-equivalence corrections ([catalog_test.go:215-240](../../../internal/provisionalfeaturecatalog/catalog_test.go#L215-L240)). V2's envelope has no correction fields ([catalog_v2.go:35-45](../../../internal/provisionalfeaturecatalog/catalog_v2.go#L35-L45)); do not pretend V1/V2 equivalence. |

## Host inference for candidate-group indexing

The host receives an assembled, bounded group packet; it does not choose topology, community membership, or the representative. Group instructions ask for candidate responsibility/coherence and evidence tensions, and permit unnamed or unresolved results.

The packet must account for every community member: complete bounded body or an explicit unavailable/omitted terminal outcome, plus helper and crossing-edge treatment. It retains internal and crossing `CALLS`, existing crossing witnesses, hub/high-centrality crossing nodes, bridges, articulation points, partition/profile/seed/community identity, custody, coverage, and the representative only as a navigation anchor. Any later semantic helper classification is proposed interpretation, not an existing structural field.

Mechanical validation checks citation membership, exact quotation, cited identity, and accounting. Independent review judges semantic support. Retention binds the exact admitted graph; partition policy/profile/seed; community identity and complete membership; source projections; host/model identity where exposed; instructions and context limitations; output; review; and corrections. Correction lineage appends a successor and never rewrites partition or evidence; acceptance remains external.

Reuse existing admission, Program C, representative lineage, retained projection, continuation store/artifact identity, and compatible V1 correction concepts. This reconciliation proposes no new public API or schema.

## Dependency separation

The next slice consumes only admitted historical server-reported `CALLS`. It does not depend on ADR 0011, Unit 2, D/R, mixed grouping, or any other relation family. Later families may enter only after their own admission and qualification; they cannot bypass this boundary.

## Current integration-first vertical slice

**Decision basis:** the user authorized a purpose-built source fixture and then approved earlier end-to-end integration. Parent reports found no eligible source-complete multi-member community among the inspected existing captures. That is a bounded dataset finding, not absence of source-capture infrastructure. Reuse the existing retained path (`ReplaySnapshots` → `retainedprojection.Admit` → `DisplayKeys` → `Select` → `Resolve` → `AssembleV2Bounded`); verify its applicability to the selected inputs rather than create another acquisition system.

**Execution spine:** actual managed CALLS/source capture → admitted graph → frozen seeded Leiden partition → one member-accounted, source-backed community packet → host interpretation → validated immutable candidate artifact → exact-ID retrieval.

1. Create or reuse the authorized small source fixture with cohesive interactions and external boundary relationships. Freeze exact fixture bytes, partition profile/seed, acquisition and resource bounds before capture. Acquire actual managed-LSP relationships and source bindings. Do not hand-author CALLS or membership, hunt seeds, or silently modify the fixture to obtain a preferred partition.
2. Establish that the observed partition contains one eligible multi-member community and that its identities join to retained bodies. If not, record the fixed-run outcome and stop this demonstration before polishing downstream artifacts. The `ValidV5` graph fixture remains useful for mechanical tests; it cannot supply missing source custody.
3. Assemble one bounded packet accounting for every member with a custody-bound body or explicit terminal unavailable/omitted outcome. Preserve internal/crossing CALLS and existing boundary fields with their provenance; representatives remain navigation only. An oversized group must fail or disclose omissions without shrinking membership. Unavailable bodies do not permit a source-supported DESCRIPTION claim merely because accounting balances.
4. Pass this exact packet through host interpretation and complete-evidence independent review, then immutable storage and exact-ID retrieval. Unnamed/unresolved outcomes are legitimate, but an all-unavailable artifact closes only a mechanical boundary, not the source-backed semantic demonstration. Host output cannot mutate membership or introduce semantic edges.
5. Exercise missing-source, forged/mismatched source identity, invalid citation/accounting, packet-capacity, boundary-loss, and publication-failure cases through this same assembled path. Check actual bounds and partial-publication behavior; no externally usable candidate may survive a failed publication as if committed. Preserve immutable predecessors and truthful terminal outcomes. Component tests support this path; they are not substitutes for it.

**Completion boundary:** one synthetic, source-backed community has traversed the full spine and its critical failure cases have passed independent review. Record exact capture/partition/source/output/storage identities, the selection rationale, semantic review, and unresolved limitations. This is synthetic integration evidence, not real-world feature quality or Leiden qualification. Evaluate usefulness on a real community separately; do not start broad qualification or another model-tuning loop to close this increment.

**Deferred:** generic/semantic search, broader corpora, local-model quality, mixed relations, production/public surfaces, and feature acceptance. Add correction/supersession only when existing compatible lineage can be reused; otherwise name that subsequent missing edge without equating V1 and V2.

### Authorized private implementation scope (standing user direction)

- internal group-packet assembly near `targetpacket`/continuation;
- host request/response orchestration through the existing host path;
- a group candidate artifact in existing storage; and
- focused tests and documentation.

Explicitly excluded: public APIs, a generic search/index backend, ADR 0011, Unit 2, a new model/harness, and feature acceptance.

### Acceptance criteria

- Exactly one community is processed, and every member has one accounted terminal outcome.
- Bodies are bounded and custody-bound; internal and crossing `CALLS` are preserved.
- The representative is not treated as sole evidence, and the host cannot change membership.
- Citations are exact; unresolved output is allowed; semantic support receives independent review.
- The artifact is retrievable by exact ID, while predecessor evidence remains immutable.
- The artifact remains `authority=0`, `accepted=false`, `completeness=UNKNOWN`.
- No feature acceptance, completeness, runtime, production, or public-surface claim is made.

## Explicit non-claims

This map does not establish feature identity, ownership, architecture, runtime execution, whole-workspace or source completeness, producer authentication, permission, production authority, stakeholder acceptance, semantic quality, public enablement, or successful implementation of the proposed slice. Leiden communities remain candidate structural groups. Presentation views and per-target descriptions are supporting evidence, not accepted group semantics. Direct source inspection does not upgrade partially observed managed-LSP relationships into server-reported `CALLS` for every edge.
