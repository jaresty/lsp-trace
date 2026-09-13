# ADR 0007: Pilot an optional local semantic index for feature inventory

- **Status:** Proposed
- **Date:** 2026-09-13

## Context

Revision-bound structural evidence can enumerate source symbols, preserve server-reported `CALLS`, and nominate structurally notable candidate sets. It cannot efficiently answer semantic retrieval questions such as “which packets appear related to enrollment reporting?” Structural communities, lexical overlap, generated descriptions, embeddings, and model confidence also cannot establish feature identity, ownership, production use, or stakeholder acceptance.

Yzma is a possible inference backend because it exposes local in-process `llama.cpp` inference, including constrained-generation and embedding APIs. Its installer verifies downloaded native-library files against release-published SHA-256 digests, and tagged releases identify compatible `llama.cpp` versions. Model verification, exact model and runtime artifact pinning, network prohibition, and process isolation are requirements imposed by this pilot wrapper, not guarantees supplied by Yzma. Yzma is not itself a feature-inventory, correction-lineage, custody, or acceptance system.

This decision is independent of canonical structural MCP operations 33–35. This ADR does not register or renumber operations 33–35, must not delay or alter them, and must not become part of `lsp-trace-mcp`. Yzma is not integrated into `lsp-trace`, its CLI, or `lsp-trace-mcp`. This Proposed ADR does not authorize implementation or shipment.

## Decision

Propose a network-denied offline worker/process, behind a closed versioned JSON protocol, for a bounded semantic-index pilot. The worker may use Yzma as one replaceable inference backend. It is not an approved daemon, shipped service, CLI surface, or MCP service.

Do not add Yzma to the `lsp-trace` module, `lsp-trace` CLI, `lsp-trace-mcp`, reusable feature-inventory workflow, release archives, installers, or restart paths. Do not require Go, Yzma, or Bar to consume the workflow.

Process separation is an operational containment choice for dependency, resource, upgrade, lifecycle, and failure isolation; it is not evidence of semantic independence. Item-independent generation and independent adjudication are distinct protocol relationships tested separately. A standalone MCP remains out of scope unless a later ADR demonstrates a multi-consumer need, lifecycle ownership, privacy review, authentication and access policy, and compatibility burden that an offline protocol cannot satisfy.

The pilot cannot execute until the evaluation plan, admission schema, lineage schema, terminal-state enums, privacy policy, supply-chain policy, and ownership assignments required below are frozen and referenced by immutable digest from this ADR or an approved successor.

## Scope

The proposed pilot has four conceptual operations. Their public names and schemas are deferred.

1. **Describe** — generate a bounded item-independent description for one admitted packet, or close it with one terminal outcome.
2. **Embed** — embed one admitted representation under one frozen policy, or close it with one terminal outcome.
3. **Search** — account for one query and rank admitted packet identities within one exact index identity.
4. **Nominate groups** — emit reversible candidate groups, explicit unmatched members, and rationale references from one exact index and policy.

An item-independent description receives exactly one admitted packet representation plus frozen system instructions. It receives no corpus statistics, neighboring packets, search results, candidate groups, stakeholder labels, prior descriptions, or persistent conversational or model state. Corpus-level context begins only after the immutable per-item description record is closed. Item independence does not by itself satisfy independent semantic adjudication, which requires a separately defined review relationship and authority boundary rather than a particular person, process, or tool.

Search and grouping may use corpus-level indexes only after all included item records are closed. They must not rewrite item evidence, descriptions, or prior judgments.

## Authority and acceptance ceiling

Every description, representation, embedding, index, search result, and nomination record has:

```json
{
  "authority": 0,
  "accepted": false
}
```

These records may nominate, retrieve, compare, or challenge candidates. They may not:

- establish, merge, or accept feature identities;
- change stakeholder acceptance state;
- infer ownership, service boundaries, runtime behavior, production use, or architecture;
- manufacture `CALLS` or alter structural evidence;
- suppress unmatched, abstained, invalid, failed, cancelled, duplicate, or truncated outcomes;
- overwrite evidence, descriptions, corrections, or lineage.

Stakeholder acceptance remains a separate explicit external action. Neither successful evaluation nor approval of this ADR grants implementation or shipment authority.

## Admission and custody boundary

An operation accepts only an immutable admission record. The record exists for accepted and rejected admission attempts and contains:

- immutable admission ID and protocol/index namespace;
- SHA-256 of the exact submitted bytes and exact byte length;
- schema identity, media type, and canonicalization policy, including whether the digest covers original or canonical bytes;
- caller-supplied identity and an explicit classification of the caller’s authority to assert it;
- admission method, performed verification, and closed verification disposition;
- rejection reason when admission fails.

Digest and length matching establish only byte integrity under the declared canonicalization. They do not establish producer authentication, source truth, execution, completeness, caller identity, custody beyond the recorded admission method, or feature identity. Every derived output references the immutable admission ID; repeating a packet digest is insufficient.

The worker must not traverse a repository, resolve arbitrary paths, fetch external context, invoke model-selected tools, or retrieve outside the admitted index.

## Sensitive-data and deletion policy

All raw and derived artifacts are potentially source-sensitive, including packet representations, descriptions, embeddings, indexes, queries, scores, rankings, grouping rationales, logs, caches, temporary files, deletion records, and backups.

Before execution, a frozen pilot policy must assign owners and define storage location, encryption at rest and in transit where applicable, access control, and retention periods for every artifact class. It must define deletion by admission ID and require:

- invalidation and rebuild of affected indexes, search products, and groups;
- cleanup of caches and temporary files;
- explicit treatment of backups and snapshots;
- tombstones that prevent a deleted admission from reappearing through stale indexes;
- a closed deletion receipt enumerating `DELETED`, `REBUILT`, `UNAVAILABLE`, and `EXTERNALLY_RETAINED` artifacts.

The receipt must identify owners and reasons for anything unavailable or externally retained. The pilot makes no secure-erasure claim unless the storage layer independently demonstrates it.

## Normative correction lineage

Every description, representation, embedding, index, search result, and nomination record must carry:

- immutable record ID and revision;
- predecessor and `supersedes` references;
- correction-event ID, actor identity, actor authority classification, and reason;
- a finite versioned `inventory_state_delta` enum;
- affected admission, packet, record, and index identities;
- downstream invalidation and rebuild dispositions for descriptions, embeddings, indexes, rankings, search results, and groups.

The frozen lineage schema must define the finite `inventory_state_delta` values before execution. Every correction appends a new record. Prior records remain replayable, visibly superseded, and excluded from current indexes according to the recorded invalidation disposition. No correction may mutate or erase predecessor history.

## Closed terminal states and denominator accounting

The protocol must version these exact uppercase, mutually exclusive per-operation enums; adding or removing a state requires a protocol version change.

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

For each operation, whole-operation outcome is separate from member outcomes. A whole-operation failure before member evaluation records zero evaluated members and the complete admitted denominator; it may not manufacture member outcomes. Once member evaluation starts, every denominator member receives exactly one member outcome even when the whole operation later closes as `CANCELLED`, `TIMEOUT`, `RESOURCE_LIMIT`, or `BACKEND_FAILURE`.

The following equations are normative:

```text
describe_or_embed_admitted =
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

Duplicate inputs and members remain in the denominator and reference the retained canonical member. A Describe or Embed whole-operation outcome is `COMPLETE` if and only if `describe_or_embed_admitted` balances and every admitted member has one terminal member outcome. An Index-build whole-operation outcome is `COMPLETE` if and only if `index_build_admitted` balances and every admitted member has one terminal member outcome. Any non-`COMPLETE` Describe, Embed, or Index-build whole-operation outcome still carries the complete admitted denominator, the evaluated-member count, and exactly one terminal outcome for each member whose evaluation began; it cannot imply completeness. Search `COMPLETE` additionally requires one closed query record and complete result-member accounting. Group `COMPLETE` requires complete group-member accounting, including every unmatched member. No operation may report `COMPLETE` while omitting a denominator member.

## Required provenance and replay levels

Every result binds:

- admission IDs, record IDs, correction-lineage IDs, and exact protocol/index namespace;
- protocol, operation-enum, lineage, and output-schema versions;
- canonical input and output bytes or immutable selectors, their digests, byte lengths, schema identities, media types, and canonicalization policies;
- worker executable digest and every loaded native-library digest;
- backend, Yzma when used, and exact `llama.cpp` identities and revisions;
- OS, architecture, accelerator, driver, hardware/backend, threads, context window, and resource limits;
- exact model source URI and revision, model artifact digest, model-card digest, quantization, tokenizer digest, and chat-template digest;
- complete license text/source digest, redistribution classification, and recorded license-review decision;
- dependency/SBOM identity and vulnerability-review disposition;
- prompt, frozen system-instruction, template, grammar, sampling-policy, and seed digests;
- inference-affecting environment-variable names and values, with secrets represented by stable secret-reference digests rather than disclosed plaintext;
- index identity, embedding policy, distance metric, search policy, grouping policy, and all policy digests;
- whole-operation and member terminal outcomes with balanced denominator accounting.

Replay claims use one of four explicit levels:

1. **Byte replay** — canonical output bytes and digest are identical.
2. **Schema replay** — output validates against the same schema and has equivalent canonical semantic fields, while bytes may differ.
3. **Description replay** — a frozen evaluation rule records description equivalence within predeclared tolerances; it is not byte replay.
4. **Rank/group replay** — rankings and groups satisfy predeclared overlap, order-distance, and membership tolerances.

A fixed seed does not imply byte replay across model versions, hardware, drivers, thread settings, platforms, or backends. Cross-backend executions are always distinct evidence records.

## Supply-chain and runtime controls

Before admission, indexing, or inference, an operator must stage model and runtime artifacts from approved sources and verify exact digests. The worker runs with network access denied and with runtime/model automatic download disabled. No inference request, admission step, index build, or retry may download or update an artifact.

The frozen supply-chain policy must record source and license/redistribution review, SBOM, vulnerability and update owners, signature verification where signatures are available, quarantine and review before replacement, and an explicit revocation procedure. Revocation invalidates every dependent admission-independent description, embedding, index, search result, and group through correction lineage and requires rebuild or closed rejection. Missing, quarantined, revoked, or unverifiable artifacts must fail only the optional semantic capability and cannot affect structural operations.

## Frozen evaluation plan prerequisite

The pilot cannot execute until an approved evaluation plan is frozen and referenced by exact digest. The plan must specify:

- corpus size, minimum sample size, and inclusion/exclusion criteria;
- independently reviewed relevance and grouping labels;
- train, calibration, and test splits plus leakage controls;
- prohibition on policy, prompt, model, threshold, or representation tuning against the test corpus;
- structural-only, lexical, and random/null baselines;
- exact `k` values and definitions for Recall@k, nDCG, grouping precision/recall, unsupported-claim rate, false-merge rate, missing/invalid/abstention rate, and reviewer efficiency;
- minimum effect sizes and uncertainty/confidence treatment for every promotion comparison;
- maximum unsupported-claim and false-merge rates;
- byte, schema, description, rank, and group replay tolerances;
- latency, peak-memory, index-size, context, and total-resource budgets;
- maximum missing, invalid, abstention, cancellation, and backend-failure rates;
- reviewer-efficiency collection method and minimum improvement;
- automatic stop thresholds and the minimum sample required before promotion or rejection.

All numeric thresholds must be fixed before test execution. Failure retains an immutable closed `REJECTED_PILOT` record with the plan digest, completed denominators, results, and stop reason. It must not trigger tuning on the test corpus.

## Promotion gates

Promotion beyond an isolated rejected-or-completed experiment requires every predeclared threshold in the frozen evaluation plan and all of the following:

- every admitted item and applicable index member is terminally accounted for;
- outputs never assert accepted feature identity or exceed `authority=0`;
- semantic search exceeds every required baseline by the minimum effect and uncertainty threshold;
- candidate grouping meets reviewer-efficiency and false-merge thresholds;
- every replay level claimed meets its exact tolerance;
- cross-backend variation is measured and disclosed as distinct records;
- provenance, admission, lineage, privacy, deletion, supply-chain, and terminal schemas are complete;
- missing or revoked artifacts degrade only the optional semantic capability;
- all privacy, retention, deletion, vulnerability, license, resource, and acceptance owners have accepted responsibility.

Passing the pilot does not authorize implementation, shipment, registry changes, a CLI command, or an MCP surface. Any such change requires a separate accepted ADR and its own implementation authorization.

## Stop and rejection criteria

Stop and close the pilot as `REJECTED_PILOT` when any frozen automatic stop threshold is crossed, or when:

- a denominator member disappears or `COMPLETE` fails to balance;
- generated output asserts feature identity, authority, acceptance, ownership, production use, or architecture;
- the held-out test corpus is used for tuning;
- privacy, deletion, lineage, provenance, license, or supply-chain records are incomplete;
- a revoked or mismatched artifact remains represented in an active index;
- unapproved raw-source retention, network access, automatic download, tools, or external retrieval occurs;
- context, cancellation, model, backend, or resource failures exceed the frozen rate;
- semantic retrieval or grouping misses any required baseline, effect, uncertainty, unsupported-claim, false-merge, replay, resource, or reviewer-efficiency threshold.

Rejection preserves the immutable plan, records, denominators, and reason. It grants no permission to weaken thresholds post hoc.

## Consequences

### Positive

- A bounded path to evaluate local semantic retrieval without hosted source disclosure.
- Item-independent descriptions may improve search snippets and reviewer navigation.
- Embeddings may nominate relationships missed by structural or lexical methods.
- The backend remains replaceable, and structural authority and stakeholder acceptance remain unchanged.

### Negative

- Native runtime, model licensing, supply-chain, SBOM, vulnerability, and revocation obligations.
- Sensitive derived-data storage, encryption, retention, deletion, backup, and tombstone obligations.
- Append-only lineage and exact denominator accounting increase protocol complexity.
- Generated descriptions can hallucinate and distort downstream retrieval.
- Replay may vary across hardware and backend configurations.
- A future MCP surface would add lifecycle, privacy, authentication, compatibility, and support commitments.

## Alternatives considered

### Add Yzma directly to `lsp-trace` or `lsp-trace-mcp`

Rejected. It would couple optional model ABI, memory, artifact acquisition, and upgrade cadence to the structural evidence product before semantic value is established.

### Run a standalone semantic MCP now

Rejected. No multi-consumer, lifecycle, privacy, authentication, access, or compatibility need has been demonstrated. A network-denied offline worker is the maximum proposed containment boundary.

### Use descriptions without structural summaries

Retained only as an evaluation arm. Description generation may erase discriminating structural facts.

### Use embeddings as feature identity

Rejected. Vector proximity is a nomination and ranking metric, not identity or acceptance evidence.

### Use a hosted inference API

Rejected because it changes source-disclosure, retention, availability, and custody assumptions.

### Keep only lexical and structural search

Retained as required baselines and as the final outcome if semantic methods do not meet every frozen gate.

## Unresolved questions

Only implementation-neutral choices may remain unresolved while this ADR is Proposed:

1. Which public or synthetic corpus should be proposed for the frozen evaluation plan?
2. Which model and quantization should be submitted for source, license, redistribution, vulnerability, and resource review?
3. Which accountable people should own privacy, deletion, supply chain, licensing, evaluation, and acceptance?

Correction lineage, terminal accounting, privacy/deletion policy, supply-chain controls, and evaluation thresholds are not unresolved; their frozen schemas and policies are execution prerequisites.

## Follow-up

Before any pilot execution or implementation authorization:

1. Draft and approve the digest-referenced evaluation plan with every required numeric threshold.
2. Draft and approve the admission, lineage, terminal-state, provenance, privacy/deletion, and supply-chain schemas and policies.
3. Assign and record all required owners.
4. Review this Proposed ADR and its immutable prerequisites, then explicitly accept, revise, or reject it.

Until those steps occur, there is no implementation authorization, no shipment authorization, no operation registration or renumbering, and no Yzma integration.
