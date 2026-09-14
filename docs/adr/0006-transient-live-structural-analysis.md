# ADR 0006: Add a distinct transient structural-context operation

- Status: Accepted
- Date: 2026-09-12

## Context

An LLM working in a codebase frequently needs a quick answer about one symbol:

- who calls it and what it calls;
- what bounded neighborhood surrounds it;
- which returned symbols are central, bridging, or boundary-crossing;
- what visible caller/callee frontier may be affected by a refactor;
- where source reading and tests should begin.

A READY managed language-server session can provide bounded server-reported call structure for these questions. Requiring Graph Provenance V5 source capture, retained source supplies, immutable publication, and offline admission before every exploratory answer adds latency and custody work that ordinary navigation and refactoring analysis do not always need.

A transient graph must not impersonate retained evidence. It may change with session generation, workspace state, server behavior, and traversal bounds. Without retained source and exact durable admission it is not hydration-capable, selector-qualified, publication-qualified, or replayable evidence.

ADR 0004 makes `trace` an acquisition operation and says no analysis runs implicitly during acquisition. This ADR preserves that boundary. It adds a separate structural-context operation; it does not add analysis to `trace` or `census`.

## Decision

Add a distinct intent-oriented structural-analysis façade named `context` and a canonical MCP operation named:

```text
lsp_trace_v1_structural_context
```

The operation supports bounded transient live analysis of one exact symbol or position without first producing Graph Provenance V5.

Durable analysis remains available through existing exact V5 and separately authorized composite admissions. Capture is an explicit escalation when retained source, replay, verification, review, or publication is material.

Admission controls claims, not access to numerical computation. Transient and durable paths may use the same numerical kernels, but they must have different admission types, result schemas, identities, bindings, and claim ceilings.

## Relationship to ADR 0004

This ADR refines ADR 0004 only by adding a separate analysis façade. It preserves these ADR 0004 decisions:

- `trace` is exact-target acquisition;
- `census` is accountable enumeration and batched acquisition;
- no derived analysis runs implicitly during `trace` or `census`;
- Graph Provenance V5 remains the authoritative production acquisition contract;
- intent-oriented operations compose existing transport-neutral capabilities rather than duplicate them.

Where ADR 0004 uses the earlier public name `discover`, the settled name is `census`.

## Current capability and proposed work

The accepted design above distinguishes exact-target `trace`, accountable `census`, and transient `context`. Current implementation status is narrower: the CLI ships `trace` and `census`, while operation 34 remains unregistered and `context`/operation 35 remain `FUTURE/PROPOSED`. The shipped census uses one exact session/generation, closed exclusions-first accounting, deterministic batches of at most 63, and a private capture-set selector; it does not add analysis or authorize direct Leiden.

The repository also retains bounded live `slice` and `incoming` traversal. Their ordinary modes do not capture source supplies. They emit acquisition-oriented Graph V3 data containing absolute URIs and other fields unsuitable for the proposed context result, and their MCP generation field is optional.

Program C currently admits exact V5 or opaque durable composites. Its public result and claim ceiling are V5-oriented. There is no transient Program C admission, transient result family, privacy projection, or combined live-analysis executor today.

Therefore the context portions of this ADR still specify proposed work. Existing Graph V3 bytes, census capture sets, and existing Program C result artifacts are not the transient result contract.

## Clarification adopted 2026-09-14: canonical transient session identity

This clarification is subsequent to the accepted decision and does not change this ADR's Accepted status. After manager admission succeeds, the manager generates the canonical logical-session identity from exactly 16 bytes obtained from `crypto/rand` and encodes it as `ts_` followed by exactly 32 lowercase hexadecimal digits. The token is stable for that logical session across generations and every use pairs it with the exact generation; the token alone never selects a generation.

The token and its collision index are process-local and nonpersistent. The manager retries a collision against every token in its live or tombstoned token index. Entropy failure prevents creation of any observable session. After final session removal the token is unavailable for lookup, even though a tombstoned index entry may remain for collision prevention during the process lifetime.

A caller cannot supply or select this token and it is not an authentication or authorization credential. Host-configured aliases, canonical token identity, and custody or authority claims are separate concerns. This clarification specifies the identity contract only: session-runtime token generation is not implemented by this increment, and operation 35 remains unregistered, unadvertised, and `FUTURE/PROPOSED`.

## Operation numbering and rollout

MCP numbering is append-only:

```text
33  lsp_trace_v1_trace
34  lsp_trace_v1_census
35  lsp_trace_v1_structural_context
```

Operation 35 must not be registered before the finalized operation 33 and 34 contracts occupy those numbers. Historical operations are not renumbered.

`lsp_trace_v1_structural_context` is the only canonical operation for this behavior. A direct MCP tool and `lsp_trace_v1_execute` dispatch to that same registry entry and transport-neutral implementation.

## Primary transient workflow

```text
exact symbol or zero-based position
→ exact READY managed session generation
→ capability and position-encoding preflight
→ bounded CALLS-only live traversal
→ privacy-safe transient projection
→ opaque TransientLiveAdmission
→ shared numerical kernels
→ transient-specific structural result
→ lifecycle/generation recheck
→ non-artifact MCP delivery
```

The first bounded answer should not be delayed by durable capture unless the requested claim requires retained evidence.

## Executor ownership

Add a distinct executor family:

```text
transient-structural
```

It must not route through the offline, acquisition-v2, legacy slice, publication, hydration, custody, or selector executor families.

The executor owns orchestration and depends on narrow interfaces for:

1. exact managed-session resolution;
2. capability and negotiated-position-encoding preflight;
3. CALLS-only bounded traversal;
4. privacy projection and transient admission;
5. shared structural computation;
6. transient-specific presentation.

It must:

1. require an exact `session_id` and `generation`;
2. resolve aliases before execution and retain the canonical session ID;
3. freeze the READY metadata observed before traversal;
4. reject unsupported call hierarchy before traversal;
5. perform bounded traversal with `CaptureSupply: false`;
6. construct admission only after validation and privacy projection;
7. run analysis only on an admitted projection;
8. recheck the canonical session generation and lifecycle state before delivery;
9. convert restart, stop, or generation change during the operation into a typed non-success terminal outcome.

The executor cannot invoke source capture, Graph Provenance construction, artifact publication, selectors, hydration, retained-passage verification, or custody receipt creation.

## MCP request contract

The input schema is Draft 2020-12 and closed with `additionalProperties: false` at every object boundary.

Required common fields are:

- `session_id`: exact ID or host-configured alias;
- `generation`: integer `>=1`, mandatory for this operation;
- `uri`: absolute document URI used only as input to managed traversal;
- exactly one target selector;
- `analysis`: one analysis discriminator;
- explicit traversal and work limits.

Target selection is a strict `oneOf`:

```json
{"symbol":"ExactName"}
```

or:

```json
{"line":0,"character":0}
```

Symbol matching is exhaustive. Zero or multiple matches fail before call-hierarchy preparation. Position inputs are zero-based in MCP.

The initial traversal fields are:

- `down_depth`: integer `0..64`;
- `up_depth`: integer `0..64`;
- `max_nodes`: integer `1..10000`;
- `timeout_ms`: integer `1..60000`;
- `request_timeout_ms`: integer `1..60000` and no greater than `timeout_ms`;
- `max_messages`: integer `1..4096`;
- `max_bytes`: integer `1..16777216`.

The future decoder must make omitted-value behavior explicit before registration. The currently intended defaults are `max_messages: 64` and `max_bytes: 4194304`; this draft schema still requires callers to provide both values and does not itself implement decoder defaults.

The request is CALLS-only. It exposes no relation, adapter, or normalized-provider selector in version 1.

The `analysis` field is a strict discriminated union. Version 1 has no implicit analysis defaults:

- `{"kind":"NEIGHBORHOOD"}` returns the admitted target, node, edge, and direction counts plus opaque node/edge witnesses;
- `{"kind":"IMPACT"}` returns directed reachability from the admitted target within the already bounded projection; it does not expand traversal;
- `{"kind":"CENTRALITY","pagerank_top_k":N,"hub_top_k":N}` where each `N` is `1..10000`;
- `{"kind":"COMMUNITIES","seed":N,"pagerank_top_k":P,"hub_top_k":H}` where the seed is an unsigned 64-bit integer represented without JSON-number rounding and `P` and `H` are `1..10000`;
- `{"kind":"BOUNDARIES","seed":N}` uses the same seed representation and reports only witnesses derived from the resulting partition and admitted directed projection.

Every discriminator binds a versioned analysis-policy ID and digest in the result. Its ordering, tie-breaking, convergence, weight, isolate, and empty-graph rules must be frozen in a committed policy before the operation is enabled. All discriminators in the published version-1 schema must be implemented and qualified at launch. If implementation deliberately begins with fewer analyses, it must publish a narrower version-1 schema rather than advertise unsupported branches; adding branches later requires a new input-schema version and explicit capability declaration. An enabled policy cannot change silently.

Version 1 does not expose cross-generation instability. Seed-campaign instability may be added only after its transient operands and result claims are separately specified.

The schema does not contain, and therefore rejects:

- `output_selector`;
- `graph_provenance`;
- workspace revision or custody assertions;
- source-snapshot or capture-supply flags;
- publication, artifact, or retained-input selectors;
- hydration or body-selection fields;
- private roots or filesystem paths;
- provider adapters or non-`CALLS` relations.

## Opaque transient admission

Introduce a distinct `TransientLiveAdmission`. Its constructor is not exported outside the owning admission package. Raw graph JSON cannot construct it.

Admission validates and retains:

- canonical session ID and exact generation;
- READY lifecycle snapshot and negotiated position encoding;
- normalized exact target;
- traversal and resource policies;
- returned nodes and only server-reported `CALLS` occurrences;
- closed traversal terminal accounting;
- unsupported, empty, partial, truncated, timeout, cancellation, and resource-limit states;
- a privacy-safe node-identity map;
- a domain-separated deterministic graph digest;
- analysis policy identity and parameters;
- the constant evidence class `TRANSIENT_LIVE`.

Names, siblings, co-membership, proximity, source text, discovery, provider extensions, or model interpretation cannot create `CALLS` support.

The admission exposes a narrow read-only graph-projection interface to shared numerical kernels. It does not expose V5 bytes, retained source, source bindings, custody receipts, publication methods, or selectors.

A transient admission can never be converted into V5 by attaching metadata. Escalation performs a new native acquisition with its own bytes, identity, and receipts.

## Shared kernels and separate outcomes

Where a certified numerical kernel already exists, compatible transient and durable projections use that exact kernel. Leiden and its existing Program C analytics are the first reuse candidates. Bridges, articulation points, conductance, or other desired calculations must be implemented and qualified as shared kernels before their transient discriminator is enabled. There is no live-only algorithm fork and this ADR does not certify a kernel by naming it.

Admission-specific code constructs outcomes:

- the existing Program C durable outcome remains V5/composite-specific;
- a transient outcome uses a new schema and claim ceiling;
- computation equivalence for identical graph projections does not imply envelope, identity, custody, or claim equivalence.

Version 1 exposes only calculations whose operands are present in one admitted live projection. Cross-generation comparison, retained-source diagnostics, and durable-source completeness calculations are excluded.

## Transient result contract

Define a non-artifact result schema:

```text
lsp-trace.transient-structural-result.v1
```

Its top-level constants include:

```json
{
  "evidence_class": "TRANSIENT_LIVE",
  "authority": 0,
  "source_graph_complete": "UNKNOWN",
  "retained": false,
  "replayable": false,
  "publication_eligible": false,
  "hydration_eligible": false
}
```

These are schema constants, not caller-selected values.

The result binds:

- canonical session ID and generation;
- normalized target identity;
- negotiated position encoding;
- traversal/resource policy identities and exact limits;
- terminal accounting;
- privacy-safe graph digest;
- algorithm names, versions, policy digests, seeds, and parameters;
- bounded analysis values and witness node IDs;
- an explicit claim ceiling.

The result is delivered through an operation-specific non-artifact success envelope or an operation-specific domain-error envelope. It has no artifact schema identity, publication receipt, artifact selector, custody receipt, or publication-error variant.

Canonical execute wraps the exact delegated envelope in its existing gateway shape. The delegated envelope bytes and domain failures are identical to direct invocation.

## Terminal-state and accounting contract

The operation has closed phases: `PREFLIGHT`, `TRAVERSAL`, `ADMISSION`, `ANALYSIS`, and `DELIVERY_CHECK`. Each request ends in exactly one terminal state.

The non-artifact success envelope is used only for:

- `COMPLETE`: traversal, admission, analysis, and delivery check completed;
- `EMPTY`: traversal completed with zero admitted call occurrences; the requested analysis returns its policy-defined empty value and no fabricated witnesses.

Every other state uses the operation-specific domain-error envelope and carries no completed analysis:

| Phase | Legal terminal states |
| --- | --- |
| `PREFLIGHT` | `UNSUPPORTED`, `AMBIGUOUS_TARGET`, `TARGET_NOT_FOUND`, `RESOURCE_LIMIT`, `TIMEOUT`, `CANCELLED`, `GENERATION_CHANGED`, `INVALID_SERVER_RESPONSE` |
| `TRAVERSAL` | `PARTIAL`, `TRUNCATED`, `RESOURCE_LIMIT`, `TIMEOUT`, `CANCELLED`, `GENERATION_CHANGED`, `INVALID_SERVER_RESPONSE` |
| `ADMISSION` | `RESOURCE_LIMIT`, `CANCELLED`, `GENERATION_CHANGED`, `INVALID_SERVER_RESPONSE` |
| `ANALYSIS` | `RESOURCE_LIMIT`, `TIMEOUT`, `CANCELLED`, `GENERATION_CHANGED`, `ANALYSIS_FAILED` |
| `DELIVERY_CHECK` | `CANCELLED`, `GENERATION_CHANGED` |

No other phase/state pair is valid. `COMPLETE` and `EMPTY` are assigned only after `DELIVERY_CHECK` succeeds.

Accounting uses non-negative integers and these equations:

```text
request_attempted = request_succeeded + request_failed + request_cancelled
prepared_attempted = prepared_returned + prepared_empty + prepared_failed
node_observed = node_admitted + node_rejected + node_omitted
occurrence_observed = occurrence_admitted + occurrence_rejected + occurrence_omitted
frontier_observed = frontier_expanded + frontier_unexpanded
```

`Accounting.Frontier` records expansion requests: `frontier_observed` counts requested expansions, partitioned into expanded and unexpanded requests. It is not boundary-node evidence and does not report the number of graph nodes at a traversal boundary.

Each omitted count has exactly one declared reason: depth bound, node bound, request bound, timeout, cancellation, unsupported response, malformed response, or deduplication. Deduplication retains both pre-deduplication and admitted denominators. `COMPLETE` and `EMPTY` require zero failed, cancelled, rejected, and non-deduplication-omitted counts, no unexpanded frontier within the requested depths, and no truncation flag. Any violation terminates as the matching domain error, with `PARTIAL` used only when at least one valid observation and at least one failed observation coexist.

A restart or stop observed after preflight but before delivery produces `GENERATION_CHANGED` or `CANCELLED`; it cannot yield a successful result from the obsolete generation.

## Privacy and disclosure

Current live Graph V3 serialization is not the MCP result because it contains absolute URIs and other acquisition detail.

The transient MCP result returns opaque, domain-separated node IDs and bounded analysis values by default. It must not return:

- absolute URIs or workspace paths;
- URI-derived raw node IDs;
- source text or retained bodies;
- opaque LSP `data`;
- symbol `detail` strings;
- raw diagnostics;
- server command arguments or environment;
- private publication roots.

If display labels or locations are later required, they must be a separately classified projection with root-confined workspace-relative logical locators, explicit sensitivity metadata, bounded cardinality, and leak tests. That extension is outside version 1.

Digests are not disclosure authorization and may permit guessing when their input domain is small. The graph digest therefore uses a domain-separated canonical representation and is not a substitute for redaction.

CLI presentation may resolve opaque IDs to local display information in-process for the invoking user, but machine JSON and MCP outputs remain governed by the privacy-safe result schema. Human presentation is not a second evidence artifact.

## Claim ceiling

A transient result may claim only:

> Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties.

It does not establish:

- retained source grounding;
- replayability after session or workspace change;
- runtime execution;
- design intent or correctness;
- architecture or ownership;
- semantic feature identity;
- whole-source or source-graph completeness;
- provider authentication;
- permission or production authority.

A community means “these symbols form a structurally notable set worth examining,” never “this community is a feature.”

Allowed phrasing includes “appears structurally central in this bounded live graph” and “is within the observed refactor-impact frontier.” Forbidden phrasing includes “is safe to refactor,” “is the architecture boundary,” “all callers are covered,” and “runs in production.”

## Capture escalation

The default coding-assistant policy is:

```text
use transient context
→ answer the bounded question and disclose material limits
→ escalate only when stronger evidence changes the decision
```

Escalate to native V5 capture when any of these is material:

- review or reproduction after the live session;
- retained enclosing bodies for semantic adjudication;
- consequential or cross-session decisions;
- immutable publication or selector-based handoff;
- distinguishing provider/session drift from source change;
- retained-passage verification or hydration;
- promotion of a technical community into feature-inventory preparation.

A durable V5 analysis may still occur without publication. “No publication” and “no capture” remain distinct.

## Feature-inventory boundary

Transient communities are investigation leads. Before promotion to a provisional feature candidate, the workflow normally escalates to exact durable graph admission and retained-source coverage for all semantically relevant members.

The semantic pipeline remains:

```text
structural community nomination
→ source-complete bounded packet
→ technical-behavior classification
→ task/outcome and infrastructure assessment
→ reversible feature-boundary hypothesis
→ independent and stakeholder adjudication
```

Mechanical preparation and structural computation cannot accept feature identity.

## MCP registry, profiles, and parity

The operation has one canonical registry entry and one canonical-execute branch. Profile filtering changes advertisement only; it never changes dispatch, schema, limits, or behavior.

After operations 33–35 are implemented:

- default advertises `trace`, `census`, `structural_context`, verification, hydration inspection, Program C durable analysis, capabilities, and canonical execute as defined by the then-current profile ADR/tests;
- advanced also advertises `structural_context`;
- compact remains exactly its compatibility-defined tool set and does not silently gain operation 35;
- full advertises all canonical operations;
- hidden legacy traversal remains callable through canonical dispatch.

Exact profile membership must be frozen by regression tests when operations 33–35 land. This ADR does not silently alter the current pre-33 installed profile counts.

## Verification requirements

Before accepting the implementation, tests must establish:

1. operation-number and profile-partition guards for append-only operations 33–35;
2. strict-schema mutation rejection for every forbidden capture, source, publication, hydration, provider, and custody field;
3. one exact symbol or position reaches analysis without V5 construction or publication;
4. `CaptureSupply` is always false and no publisher, selector, artifact store, custody, hydration, or retained-passage dependency is called;
5. executor-family routing never delegates to offline, acquisition-v2, publication, or legacy slice handlers;
6. direct and canonical-execute delegated-envelope and domain-failure parity;
7. required-generation omission, stale generation, alias resolution, restart during each phase, stop, and pre-delivery mismatch behavior;
8. capability and position-encoding preflight occurs before traversal or analysis;
9. absolute path, URI, diagnostics, opaque data, symbol detail, command argument, environment, and source-body leak tests;
10. exact terminal-state exclusivity and denominator reconciliation;
11. no complete analysis is emitted from incomplete or truncated traversal;
12. server-reported `CALLS` cannot be synthesized by sibling, name, source, discovery, or community logic;
13. transient and durable adapters produce computation-equivalent numerical results for identical admitted graph projections while retaining different schemas, identities, bindings, and claims;
14. transient output cannot be admitted by V5, durable Program C, hydration, passage-verification, publication, custody, or selector APIs;
15. empty, unsupported, null, malformed, partial, timeout, cancellation, and node-limit provider behavior is typed;
16. existing V5 admission, Program C claim ceiling, custody, publication, and qualification remain unchanged;
17. representative live providers pass in addition to synthetic fixtures.

## Consequences

LLMs can answer ordinary structural design and refactoring questions with lower latency and without creating durable evidence unnecessarily. The same certified numerical kernels guide source reading, test selection, impact analysis, and capture decisions.

The cost is a new executor family, opaque admission, privacy projection, non-artifact result family, and stricter language discipline. Some questions require a second native acquisition rather than upgrading transient bytes.

## Non-goals

This ADR does not:

- change `trace` or `census` into analysis commands;
- change certified numerical algorithms;
- infer calls;
- authorize feature identity, architecture, ownership, or design verdicts;
- make transient results durable, hydrated, published, or replayable;
- weaken Graph Provenance V5 admission;
- authorize capture-set Leiden admission or cross-capture calls;
- change installation or managed-service lifecycle behavior.
