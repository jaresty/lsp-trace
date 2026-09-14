# ADR 0007: Pilot an optional local engineering-context index

- **Status:** Accepted
- **Date:** 2026-09-13
- **Accepted:** 2026-09-14

## Context

Engineering work repeatedly needs bounded semantic retrieval while a design or change is still in progress: the governing decision for one symbol, related tests and prior failures for one range, analogues for one proposal, or unresolved assumptions around one requirement. Requiring a repository or source census before answering one exact-item question makes that support unavailable at the point of use.

Revision-bound structural evidence can enumerate source symbols and preserve server-reported `CALLS`. Accepted ADRs and requirements can state governing decisions. Working proposals, tasks, and generated summaries can preserve useful current context. These sources have different authority. Lexical overlap, embeddings, generated descriptions, structural adjacency, communities, and model confidence cannot equalize them or establish design correctness, ownership, production use, completion, or feature identity.

Yzma is a possible inference backend because it exposes local in-process `llama.cpp` inference, including constrained-generation and embedding APIs. Its installer verifies downloaded native-library files against release-published SHA-256 digests, and tagged releases identify compatible `llama.cpp` versions. Model verification, exact model and runtime artifact pinning, network prohibition, and process isolation are requirements imposed by this pilot wrapper, not guarantees supplied by Yzma.

This decision is independent of canonical structural MCP operations 33–35. This ADR does not register or renumber operations 33–35, must not delay or alter them, and must not become part of `lsp-trace-mcp`. Yzma is not integrated into `lsp-trace`, its CLI, or `lsp-trace-mcp`. Acceptance authorizes prerequisite work and, only after every frozen gate in this ADR is satisfied, implementation and execution of the isolated pilot. It does not authorize shipment, a public CLI or MCP surface, registry changes, or integration into core binaries.

## Decision

Propose an optional, network-denied, local offline engineering-context index behind a closed, versioned protocol. It is usable continuously during design and engineering over exactly admitted material. Feature inventory is one downstream workflow, not the primary product.

The worker may use Yzma as one replaceable inference backend. It is not an approved daemon, shipped service, CLI surface, MCP service, autonomous engineering agent, or repository crawler. Public CLI, MCP, operation, and schema names are deferred. No public surface is designed by this ADR.

Do not add Yzma to the `lsp-trace` module, `lsp-trace` CLI, `lsp-trace-mcp`, reusable feature-inventory workflow, release archives, installers, or restart paths. Do not require Go, Yzma, or Bar to consume the protocol.

Process separation contains dependency, resource, upgrade, lifecycle, and failure risk; it is not evidence of semantic independence. Item-independent generation and independent adjudication remain separate relationships. The pilot cannot execute until its evaluation plan, typed admission and lineage schemas, terminal-state enums, privacy and deletion policy, supply-chain policy, and ownership assignments are frozen and referenced by immutable digest.

## Strictly typed corpora

Every admission belongs to exactly one corpus and one finite item type within that corpus:

1. **Revision-bound code and structural evidence** — source files, symbol or range representations, tests, and retained structural evidence bound to an exact repository/source revision and evidence digest.
2. **Accepted decision and requirement evidence** — accepted ADRs, requirements, policies, and stakeholder decisions whose source authority and acceptance metadata are preserved without reinterpretation.
3. **Working context** — proposals, tasks, review notes, unresolved assumptions, and generated summaries. Working or generated status must remain explicit.

Every admission and result exposes:

- corpus and finite item type;
- immutable source identity and exact passage, range, symbol, file, or document selector;
- source revision and/or content digest plus exact byte length and canonicalization policy;
- source acceptance state and source authority classification;
- supersession links and an explicit current, superseded, unresolved, or unknown currentness state;
- complete provenance and derivation references.

Cross-corpus search and similarity preserve these distinctions in every returned member. Similarity, rank, or grouping never equalizes authority, upgrades acceptance, resolves supersession, or converts working context into accepted decision evidence. Accepted source ADRs and requirements retain their own source authority metadata. A generated description, embedding, summary, ranking, or projection always has `authority=0` and `accepted=false`; it cannot modify or inherit the source's authority or acceptance.

## Acquisition modes and coverage

Every operation records exactly one acquisition mode and its coverage statement:

1. **TARGET** — admit one exact symbol, range, file, document, or passage. TARGET has no census prerequisite. One admitted target may immediately be described, embedded, and indexed.
2. **NEIGHBORHOOD** — begin from explicit targets and perform only a declared, bounded structural or provenance expansion. The record identifies expansion rules, bounds, admitted members, exclusions, failures, and an accountable admitted denominator.
3. **CENSUS** — enumerate a declared closed repository or source scope and account for every member of that closed scope.

A result reports its exact acquisition mode, scope, denominator, evaluated count, terminal counts, revision policy, and coverage state. `TARGET` means exact one-item coverage, not repository coverage. `NEIGHBORHOOD` means only its recorded bounded expansion. `CENSUS` means only its declared closed scope. Partial indexes and bounded zero results never imply absence outside the admitted index, completeness of a repository/source, or completeness of an engineering domain.

## Operations and item-independent Describe

The proposed protocol has four conceptual operation families. Their public names and schemas are deferred:

1. **Describe** — generate a bounded item-independent description for one admitted item, or close it with one terminal outcome.
2. **Embed and index** — embed one admitted representation and append it to one exact index product under frozen policies, or close it with one terminal outcome.
3. **Search** — close one query record and account for every member of one exact admitted index while ranking eligible members.
4. **Nominate groups** — emit reversible candidate groups, explicit unmatched members, and rationale references from one exact admitted index and policy.

Describe is independent of corpus size and acquisition mode. It receives exactly one admitted item representation plus frozen system instructions. It receives no corpus statistics, neighboring items, search results, groups, stakeholder labels, prior descriptions, or persistent conversational/model state. TARGET can therefore immediately describe, embed, and index one item. Corpus search and grouping operate over whatever exact admitted index exists and expose that index's acquisition mode and coverage; they do not wait for or imply census.

Search and grouping never rewrite admissions, item evidence, descriptions, source authority, source acceptance, or prior judgments.

## Practical engineering use and capability ceiling

The index may retrieve governing decisions and requirements, related symbols and tests, prior failures, analogues, unresolved assumptions, and relevant working context. It may emit bounded work-context packets, candidate change sites, and review targets with exact provenance and coverage.

These products may guide, rank, and navigate human-authorized engineering work. They cannot decide:

- design correctness or architectural fitness;
- edit or execution authorization;
- safety, security, privacy, or operational acceptability;
- ownership or production use;
- feature identity or stakeholder acceptance;
- whether work is complete.

Execution remains external and separately authorized. The worker must not edit files, execute code, select or invoke tools, traverse a repository, resolve arbitrary paths, fetch external context, or retrieve beyond exactly admitted material. It receives only caller-admitted bytes and declared relationships.

## Immutable admission, identity, cache, and invalidation

Every accepted or rejected admission attempt creates an immutable append-only record containing an admission ID, corpus/type, exact submitted-byte SHA-256 and length, schema/media identity, canonicalization policy, source selector and identity, source revision/digest, authority and acceptance metadata, performed verification, and terminal disposition. Digest and length establish byte integrity only; they do not establish producer authentication, source truth, execution, completeness, custody, ownership, or feature identity.

Every derived record and index product has an identity computed from immutable references to:

```text
record_identity = digest(
  admission identities and ordered dependency identities,
  representation digest,
  prompt/system/template/grammar digests,
  model/tokenizer/chat-template digests,
  policy digests,
  runtime/backend/native-library/environment digests
)
```

Records are immutable and append-only. A cache hit is permitted only for exact identity equality; approximate, partial, model-family, prompt-family, or policy-family matches are cache misses. A changed admission, source revision, representation, prompt, model, policy, runtime, dependency, correction, deletion, or revocation creates a new record and invalidates or rebuilds every affected index, search, ranking, group, and work-context product. Prior records remain replayable and visibly superseded.

Multiple source and repository revisions may coexist. Every query pins exact revision/index identities or states a frozen, digest-identified selection policy such as current accepted source revision. Results disclose the selected policy and all resolved identities; they never silently mix revisions.

## Normative correction lineage

Every record carries immutable predecessor and `supersedes` references; correction-event ID, actor identity, actor authority classification, and reason; a finite versioned `context_state_delta` enum; affected admission, record, dependency, and index identities; and downstream invalidation/rebuild dispositions.

An inventory-specific downstream workflow may define an additional `inventory_state_delta` subtype, but the core engineering-context protocol uses neutral `context_state_delta`. Every correction appends a record. No correction mutates or erases predecessor history, structural evidence, source authority, or source acceptance.

## Authority and independent adjudication

Every generated semantic product carries:

```json
{
  "authority": 0,
  "accepted": false
}
```

Generated products may nominate, retrieve, compare, or challenge candidates. They may not establish semantic truth, accepted design, feature identity, ownership, service boundaries, runtime behavior, production use, source authority, or acceptance. They may not manufacture `CALLS`, suppress terminal outcomes, or overwrite evidence or lineage.

Acceptance remains an explicit external action by an independently identified authority. Independent adjudication must use a separately defined review relationship and authority boundary; a different process or person alone does not establish independence. Evaluation success and ADR approval grant neither implementation nor shipment authority.

## Closed terminal states and denominator accounting

The closed protocol versions these exact finite, uppercase, mutually exclusive operation/member outcome enums. Adding or removing a state requires a protocol version change.

**Describe operation outcome v1:**

`COMPLETE | MODEL_UNAVAILABLE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`

**Describe member outcome v1:**

`COMPLETE | ABSTAINED | INVALID_INPUT | MODEL_UNAVAILABLE | CONTEXT_LIMIT | OUTPUT_INVALID | TIMEOUT | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`

**Embed operation outcome v1:**

`COMPLETE | MODEL_UNAVAILABLE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`

**Embed member outcome v1:**

`COMPLETE | ABSTAINED | INVALID_INPUT | MODEL_UNAVAILABLE | CONTEXT_LIMIT | OUTPUT_INVALID | TIMEOUT | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`

**Index-build operation outcome v1:**

`COMPLETE | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`

**Index-build member outcome v1:**

`INCLUDED | INVALID_INPUT | EMBEDDING_UNAVAILABLE | CANCELLED | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH | DUPLICATE_INPUT`

**Search operation outcome v1:**

`COMPLETE | INVALID_QUERY | INDEX_UNAVAILABLE | INDEX_MISMATCH | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`

**Search result-member outcome v1:**

`RETURNED | BELOW_THRESHOLD | FILTERED_BY_POLICY | DUPLICATE_MEMBER | INVALID_MEMBER`

**Group operation outcome v1:**

`COMPLETE | INDEX_UNAVAILABLE | INDEX_MISMATCH | CANCELLED | TIMEOUT | RESOURCE_LIMIT | BACKEND_FAILURE | POLICY_MISMATCH`

**Group member outcome v1:**

`GROUPED | UNMATCHED | FILTERED_BY_POLICY | DUPLICATE_MEMBER | INVALID_MEMBER`

For Describe, Embed, Index-build, Search, and Group, whole-operation outcome is separate from member outcomes. A failure before member evaluation records zero evaluated members and the complete admitted/index denominator; it does not manufacture member outcomes. Once evaluation of a member begins, that member receives exactly one terminal member outcome even if the operation later fails.

The following equations are normative and map every member outcome in the corresponding v1 enum exactly once:

```text
describe_admitted =
  complete + abstained + invalid_input + model_unavailable + context_limit +
  output_invalid + timeout + cancelled + resource_limit + backend_failure +
  policy_mismatch + duplicate_input

embed_admitted =
  complete + abstained + invalid_input + model_unavailable + context_limit +
  output_invalid + timeout + cancelled + resource_limit + backend_failure +
  policy_mismatch + duplicate_input

index_build_admitted =
  included + invalid_input + embedding_unavailable + cancelled +
  resource_limit + backend_failure + policy_mismatch + duplicate_input

search_index_members =
  returned + below_threshold + filtered_by_policy + duplicate_member + invalid_member

group_index_members =
  grouped + unmatched + filtered_by_policy + duplicate_member + invalid_member
```

Duplicate inputs and members remain in the denominator and reference the canonical member. A Describe whole-operation outcome is `COMPLETE` if and only if `describe_admitted` balances and every admitted member has exactly one terminal member outcome. An Embed whole-operation outcome is `COMPLETE` if and only if `embed_admitted` balances and every admitted member has exactly one terminal member outcome. An Index-build whole-operation outcome is `COMPLETE` if and only if `index_build_admitted` balances and every admitted member has exactly one terminal member outcome.

**Search is `COMPLETE` if and only if one closed query record exists, the `search_index_members` denominator equation balances, and every index member has exactly one terminal search-member outcome.**

**Group is `COMPLETE` if and only if the `group_index_members` denominator equation balances and every index member has exactly one terminal group-member outcome, including `UNMATCHED`.**

A non-`COMPLETE` Search or Group retains the complete index denominator, evaluated-member count, and exactly one terminal member outcome for each member whose evaluation began; it cannot imply complete evaluation, complete coverage, or absence. A non-`COMPLETE` Describe, Embed, or Index-build retains the complete admitted denominator, evaluated-member count, and exactly one terminal member outcome for each member whose evaluation began; it cannot imply complete evaluation or completeness. No operation reports `COMPLETE` while omitting a denominator member.

## Required provenance and replay

Every result binds admission, record, correction-lineage, dependency, protocol, schema, terminal-enum, and exact index identities; canonical input/output bytes or immutable selectors and digests; executable and native-library digests; backend and exact `llama.cpp` identity when used; OS, architecture, accelerator, driver, threads, context, and resource limits; exact model source/revision/artifact, model-card, tokenizer, quantization, and chat-template digests; license, redistribution, SBOM, vulnerability-review, and dependency identities; prompt/system/template/grammar/sampling/seed digests; inference-affecting environment names and stable secret-reference digests; policies, metrics, thresholds, modes, coverage, denominators, and terminal outcomes.

Replay claims remain explicitly typed as byte replay, schema replay, description-equivalence replay under frozen tolerances, or rank/group replay under frozen overlap/order/membership tolerances. A fixed seed does not imply byte replay across model, runtime, hardware, driver, thread, platform, or backend changes. Cross-backend executions are distinct records.

## Sensitive data, privacy, and deletion

All raw and derived artifacts are potentially source-sensitive, including representations, descriptions, embeddings, indexes, queries, rankings, work-context packets, group rationales, logs, caches, temporary files, deletion records, and backups.

Before execution, frozen policy assigns owners and defines storage, encryption where applicable, access control, and retention for every artifact class. Deletion by admission ID invalidates and rebuilds affected products, cleans caches and temporary files, explicitly treats backups/snapshots, records tombstones preventing stale reappearance, and closes a receipt enumerating `DELETED`, `REBUILT`, `UNAVAILABLE`, and `EXTERNALLY_RETAINED` artifacts with owners and reasons. No secure-erasure claim is made without independent storage-layer evidence.

## Supply-chain and runtime controls

Before admission, indexing, or inference, an operator stages model/runtime artifacts from approved sources and verifies exact digests. The worker runs network-denied with automatic download disabled. No request, build, or retry downloads or updates an artifact.

Frozen policy records source, license/redistribution review, SBOM, vulnerability/update owners, signatures where available, quarantine before replacement, and revocation. Revocation invalidates every dependent generated product and requires rebuild or closed rejection. Missing, quarantined, revoked, or unverifiable artifacts fail only the optional semantic capability and cannot affect structural operations.

## Frozen evaluation plan and promotion gates

Engineering retrieval evaluation must cover TARGET one-item cases, bounded NEIGHBORHOOD cases, and closed CENSUS cases without treating one mode as another. Before execution, a digest-pinned plan freezes corpus composition and typed inclusion/exclusion criteria; sample sizes and train/calibration/test splits; leakage controls; independent relevance, work-context, candidate-site, review-target, and grouping labels; structural-only, lexical, and random/null baselines; exact `k`; Recall@k, nDCG, unsupported-claim rate, candidate-site and review-target precision/recall, grouping precision/recall, false-merge rate, missing/invalid/abstention/failure rates, reviewer efficiency, latency, memory, context, and index-size metrics; minimum effects and uncertainty treatment; replay tolerances; resource budgets; and automatic stops.

Thresholds are fixed before test execution. Promotion beyond an isolated rejected-or-completed experiment requires balanced terminal accounting for every applicable denominator; no authority or acceptance violation; engineering retrieval exceeding every required baseline by the frozen effect/uncertainty threshold; candidate change-site, review-target, work-context, and optional grouping quality meeting their thresholds; replay tolerances; disclosed cross-backend variation; complete provenance/admission/lineage/privacy/deletion/supply-chain schemas; optional-only degradation; and accepted ownership for privacy, retention, deletion, vulnerability, licensing, resources, evaluation, and acceptance.

Feature-inventory evaluation is an optional subtype and retains its independent semantic adjudication, unsupported-claim, false-merge, stakeholder-correction, and acceptance gates. It is not required for exact engineering retrieval use and cannot weaken the core quantitative gates.

Passing evaluation does not authorize shipment, registry changes, a public CLI command, an MCP surface, or integration into core binaries. Those require a separate accepted ADR and implementation authorization.

## Stop and rejection criteria

Stop and append an immutable `REJECTED_PILOT` record when a frozen threshold is crossed; an equation or terminal outcome fails to balance; mode or coverage is misstated; an index implies absence/completeness beyond admitted scope; generated output asserts authority, acceptance, correctness, authorization, safety, ownership, production use, completion, or feature identity; test data is used for tuning; privacy/deletion/lineage/provenance/license/supply-chain records are incomplete; stale/revoked dependencies remain active; unapproved retention, network access, automatic download, model-selected tools, traversal, or external retrieval occurs; or retrieval, candidate-site, review-target, grouping, replay, resource, failure-rate, or reviewer-efficiency gates fail.

Rejection preserves the immutable plan, records, denominators, outcomes, and reason. It grants no permission to weaken thresholds post hoc.

## Consequences

### Positive

- Exact one-item context is available without census while preserving honest coverage.
- Governing decisions, related code/tests/failures, analogues, assumptions, candidate sites, and review targets can be retrieved during active design and engineering.
- Typed corpora preserve source authority across cross-corpus similarity.
- Incremental exact-identity caching permits revisions to coexist without mutable records.
- Feature inventory remains available as a separately adjudicated downstream workflow.
- Local processing avoids hosted source disclosure, while backend replacement and structural authority remain unchanged.

### Negative

- Native runtime, licensing, supply-chain, SBOM, vulnerability, revocation, privacy, and deletion obligations remain substantial.
- Append-only lineage, dependency invalidation, revision policy, and exact denominator accounting increase protocol complexity.
- Partial indexes demand persistent coverage qualification and cannot answer global absence questions.
- Generated context can hallucinate, mis-rank, or distract and therefore requires external judgment.
- Replay can vary across hardware and backend configurations.

## Alternatives considered

### Require census before semantic use

Rejected. It prevents exact one-item assistance during normal design and engineering and incorrectly couples useful local context to repository-wide coverage.

### Treat all indexed material as one authority class

Rejected. Similarity cannot transform working or generated context into accepted decisions or revision-bound evidence.

### Add Yzma directly to `lsp-trace` or `lsp-trace-mcp`

Rejected. It couples optional model ABI, memory, artifact acquisition, and upgrade cadence to the structural evidence product before value is established.

### Run a standalone semantic MCP or design a public CLI now

Rejected. No public naming, multi-consumer lifecycle, privacy, authentication, access, compatibility, or support contract is approved. The closed offline protocol remains the maximum proposed boundary.

### Use embeddings or groups as feature identity or design correctness

Rejected. Similarity and grouping are navigation and nomination aids, not authority or acceptance evidence.

### Use hosted inference

Rejected because it changes source-disclosure, retention, availability, and custody assumptions.

### Keep only lexical and structural retrieval

Retained as required baselines and as the final outcome if semantic methods do not pass every frozen gate.

## Unresolved questions

The following implementation choices remain unresolved after acceptance and must be closed by the frozen prerequisites before pilot execution:

1. Which public or synthetic typed corpora should be proposed for the frozen evaluation plan?
2. Which model and quantization should be submitted for source, license, redistribution, vulnerability, and resource review?
3. Which accountable people should own privacy, deletion, supply chain, licensing, evaluation, and acceptance?
4. Which finite public names should a later authorized protocol or product use?

Typed admission, identity, lineage, terminal accounting, privacy/deletion, supply chain, and evaluation thresholds are not optional; frozen schemas and policies are execution prerequisites.

## Follow-up

Before pilot execution:

1. Draft and approve the digest-referenced evaluation plan with every required numeric threshold and acquisition-mode case.
2. Draft and approve typed admission, identity, lineage, terminal-state, provenance, privacy/deletion, and supply-chain schemas and policies.
3. Assign and record all required owners.
4. Verify that every immutable prerequisite is frozen and referenced by digest before enabling inference or indexing.

Until those prerequisite steps occur, there is no pilot execution authorization and no Yzma integration. This acceptance never grants shipment authorization, operation registration or renumbering, a public CLI or MCP surface, or integration into core binaries.
