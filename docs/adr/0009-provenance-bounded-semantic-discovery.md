# ADR 0009: Add provenance-bounded semantic discovery and change planning

- **Status:** Proposed
- **Date:** 2026-09-20
- **Decision owners:** LSP Trace maintainers
- **Depends on:** ADR 0007 and ADR 0008
- **Implementation sequence:** Deferred until after ADR 0010 provenance-bounded feature attribution

## Context

ADR 0007 produces a local, offline, provisional feature catalog from immutable census, structural, projected-source, and Describe evidence. Its catalog entries remain `authority=0`, `accepted=false`, and `completeness=UNKNOWN`. ADR 0008 separates graph custody from deterministic source projection and prevents source text or semantic interpretation from creating structural facts.

Those foundations make evidence review possible, but they do not yet provide a general discovery interface for questions such as:

- “Where is feature X implemented?”
- “I want to add X; where should the change probably enter?”
- “Which existing behavior is analogous to this proposal?”
- “What tests, decisions, failures, or unresolved assumptions surround this area?”
- “What evidence conflicts with this provisional feature description?”
- “How should this work-in-progress item be organized relative to existing evidence?”

These are not only lookup questions. Some are change-planning questions asked before the desired feature exists. A useful system must retrieve evidence, expose uncertainty, and optionally synthesize a bounded answer without converting relevance into truth, feature identity, ownership, completeness, authorization, or implementation correctness.

The ADR 0007 catalog is one useful retrieval corpus, but catalog prose alone is insufficient. It is generated, provisional, and may be corrected. Search must preserve distinctions among exact source passages, symbols, server-reported structural relations, tests, accepted decisions, working context, catalog descriptions, failures, and corrections.

A local model can help interpret natural-language queries, select a finite retrieval intent, rerank candidates, and synthesize a response. It cannot silently broaden the admitted corpus, create structural facts, resolve disputed corrections, or promote a result above the authority of its evidence.

## Decision

Introduce an optional, local-only **provenance-bounded candidate-discovery pipeline** over immutable admitted evidence.

The pipeline will:

1. retain the raw natural-language query as an immutable query record;
2. use a pinned local model to propose one finite query intent and a closed query plan;
3. execute separately accounted lexical, structural, and local-semantic retrieval channels;
4. fuse channel results deterministically under a versioned policy;
5. return both inspectable candidate evidence and an optional bounded synthesized answer;
6. keep the candidate result and synthesized answer as separate records with separate citations and terminal outcomes;
7. preserve corrections, conflicts, currentness, replay identities, resource accounting, and all ADR 0007/0008 authority ceilings;
8. expose the capability through both CLI and MCP after evaluation and enablement gates pass.

Every retrieval result and synthesized answer remains:

```text
authority = 0
accepted = false
completeness = UNKNOWN
```

No view type, query intent, score, rank, threshold, channel agreement, fusion outcome, currentness rule, or synthesized statement can change those values.

## Goals

- Support evidence lookup and work-in-progress change planning from natural language.
- Return evidence and synthesis together without conflating their authority.
- Use a pinned local model for query interpretation, semantic ranking, and bounded synthesis.
- Preserve exact provenance, custody, correction history, and replay inputs.
- Make ranking contributions and shared channel inputs inspectable.
- Keep execution local, offline, bounded, opt-in, and fail-closed.
- Provide deterministic lexical and structural behavior and explicitly qualified semantic replay.
- Expose one coherent CLI and MCP contract.

## Non-goals

This decision does not authorize:

- accepted or canonical feature identity;
- automatic ownership or product-purpose assignment;
- proof that a candidate is the correct implementation location;
- repository or domain completeness claims;
- autonomous edits, commits, issue changes, or task assignment;
- automatic acceptance, merge, split, rename, or rejection of catalog entries;
- structural relations inferred from source text, embeddings, descriptions, or model output;
- hosted inference, hidden downloads, ambient network access, or silent model substitution;
- unbounded repository crawling or workspace fallback;
- silent replacement of semantic retrieval with lexical retrieval;
- resolution of conflicting corrections by recency alone.

## Query contract

### Query record

Each query produces an immutable record containing:

- exact retained query text and canonical UTF-8 bytes;
- query digest and byte length;
- caller-selected corpus/index selector;
- requested or confirmed intent;
- ranking profile and policy identities;
- exact revision/currentness policy;
- privacy and retention policy;
- resource limits;
- model, runtime, tokenizer, adapter, and sandbox identities where used;
- terminal status and source-safe diagnostics.

Queries use one explicit retention class:

- `EPHEMERAL` — retained only long enough to produce the response; no exact replay is promised after completion;
- `SESSION` — retained in the local immutable publication store while an investigation or feature-planning session is active;
- `DURABLE_INVESTIGATION` — explicitly promoted because the investigation materially influenced a design, correction, implementation, or review decision.

`SESSION` is the default. Promotion to `DURABLE_INVESTIGATION` is deliberate and records the promoting decision, actor, reason, related issue/ADR/task/review, and predecessor query identities. Retention class changes append records; they never mutate the original query.

Exact replay is available only while all required query, evidence, index, policy, and runtime records remain retained. Ephemeral completion must state that later exact replay is unavailable.

### Finite intents

Release one defines a closed, versioned intent enum:

- `LOCATE_EVIDENCE` — retrieve evidence relevant to a described behavior or concept;
- `FIND_CHANGE_SITES` — nominate source, test, decision, and structural locations relevant to adding or changing behavior;
- `FIND_ANALOGUES` — retrieve similar existing behavior or implementation patterns;
- `FIND_CONTRARY_EVIDENCE` — retrieve evidence that challenges a query or provisional description;
- `REVIEW_PROVISIONAL_FEATURE` — gather evidence around one ADR 0007 catalog entry;
- `FIND_UNRESOLVED_ASSUMPTIONS` — retrieve working-context assumptions, failures, and open questions;
- `TRACE_EVIDENCE_CONTEXT` — retrieve bounded governing, dependent, consumer, test, and provenance context.

The local model may propose an intent. The proposal is authority-zero and is retained with its rationale. The caller either supplies an intent or explicitly permits model selection under a pinned intent-selection policy. Unknown intents fail closed.

Intent names describe retrieval mechanics. They do not assert that a feature exists, that a candidate implements it, or that a location is correct.

## Evidence corpus and retrieval views

The system searches immutable, provenance-typed views rather than one blended “feature document.” Eligible view types describe origin, not truth:

- `SOURCE_PASSAGE_VIEW`;
- `SYMBOL_VIEW`;
- `SERVER_REPORTED_RELATION_VIEW`;
- `TEST_VIEW`;
- `ACCEPTED_DECISION_PASSAGE_VIEW`;
- `WORKING_CONTEXT_VIEW`;
- `PROVISIONAL_CATALOG_RECORD_VIEW`;
- `FAILURE_OR_DIAGNOSTIC_VIEW`;
- `CORRECTION_RECORD_VIEW`;
- `UNRESOLVED_ASSUMPTION_VIEW`.

Each view binds:

- immutable source and projection identities;
- exact revision, generation, or retained selector;
- logical source URI and exact range where applicable;
- digest, byte length, encoding, and canonicalization policy;
- corpus, source authority, source acceptance, and currentness metadata;
- derivation and correction identities;
- privacy partition and retention policy;
- `authority=0` and `accepted=false` for the derived view.

One source item may yield multiple views. Final ranking deduplicates by immutable evidence identity before aggregating to a catalog or planning candidate, so representation count cannot manufacture relevance.

Catalog descriptions may contribute low-authority ranking text and filtering metadata. They never replace exact evidence and never transfer authority to another channel.

## Retrieval channels

### Lexical channel

The lexical channel performs deterministic sparse retrieval over admitted canonical text, identifiers, aliases, paths, and metadata under a pinned tokenizer and scoring policy.

It must expose:

- candidate identity;
- raw and normalized score;
- matched terms or fields;
- tokenizer and policy identity;
- eligibility and exclusion accounting;
- deterministic tie-breaking.

### Structural channel

The structural channel performs bounded retrieval over admitted server-reported relationships and provenance links.

It may use direction, relation type, bounded depth, and exact path cost. It cannot create or repair graph relations from text, embeddings, catalog membership, co-location, or model interpretation.

### Local-semantic channel

The semantic channel uses an explicitly pinned local model and runtime. Release one may use embeddings, constrained pairwise relevance scoring, or a local reranker, but the exact mechanism is part of the ranking-profile identity.

The semantic worker:

- receives only admitted bounded records;
- runs under externally enforced network denial;
- cannot read the workspace, session, object store, or undisclosed neighboring records;
- cannot download or select a different model;
- emits strict typed outputs;
- records terminal outcomes for every attempted candidate;
- cannot rewrite evidence, intent, or channel scores.

Semantic ranking is explicit opt-in until the enablement gates in this ADR pass. Unavailable semantic resources produce a typed terminal failure. They do not silently fall back to another profile.

#### Constrained local relevance decisions

One eligible semantic mechanism is a textless, fixed-choice local decision scorer. After deterministic channels construct a bounded candidate pool, a pinned local model may score each query/candidate pair against a closed choice set such as `FIT`, `NOT_FIT`, and `ABSTAIN` by evaluating complete fixed branch-token sequences and retaining their raw log-likelihoods. This is a ranking mechanism, not a truth, feature-identity, acceptance, or confidence claim.

Release-one use is limited to reranking. Deterministic retrieval preserves the candidate set and inspection access; a low semantic score cannot silently remove a candidate. Any later hard-filter policy requires a separately versioned evaluation demonstrating bounded false-negative behavior for each enabled intent and corpus.

Each decision record binds:

- exact query, candidate, catalog/index, and evidence-packet identities;
- the closed choice set, ordering, complete token sequences, and tokenizer identity;
- raw branch log-likelihoods before normalization and the normalization policy;
- model, runtime, quantization, adapter, grammar, sandbox, and inference identities;
- deterministic pre-semantic channel scores/ranks and resulting rerank position;
- one terminal outcome per attempted candidate, including explicit abstention;
- `authority=0`, `accepted=false`, and `completeness=UNKNOWN`.

The runtime architecture keeps retrieval, inference, validation, and ranking under separate custody:

```text
Discovery host
  ├─ immutable index/query admission
  ├─ deterministic lexical and structural retrieval
  ├─ bounded candidate-packet builder
  ├─ pinned network-denied Decision Worker
  │    └─ local runtime + model + tokenizer + branch scorer
  ├─ strict decision-record validator and immutable publisher
  └─ deterministic reranker/fusion and result renderer
```

The host constructs one bounded packet per query/candidate pair and owns candidate identity, choice identity, evidence selectors, limits, ordering, publication, and terminal accounting. The Decision Worker receives only admitted packet bytes and host-selected pins. It cannot read the index, corpus, workspace, session, object store, neighboring candidates, publication root, or network, and it cannot select its own model, tokenizer, choices, thresholds, or output paths.

The worker may evaluate declared choice branches through one forward state with bounded parallel branch scoring when the pinned runtime supports it, or score each complete branch deterministically under an equivalent versioned adapter. Batching cannot alter candidate independence, choice order, accounting, or identity. One candidate's text, logits, cache state, or outcome cannot become another candidate's input unless a separately versioned policy explicitly admits and records that dependency.

Only strictly parsed decision records reach the deterministic reranker. Invalid, incomplete, non-finite, missing-branch, extra-branch, or identity-mismatched output receives a typed terminal outcome and contributes no semantic score. The reranker never reads raw worker prose or stderr and never asks the worker to retrieve, fuse, filter, or synthesize evidence.

Single-token top-logprob output is insufficient when choices tokenize differently or eligible branches are omitted from a bounded top-k response. Renormalized branch scores are model- and policy-relative ranking evidence, not calibrated probabilities. Lexical, structural, embedding, and constrained-decision scores remain separately inspectable and are never collapsed into an unexplained scalar.

## Deterministic fusion

Release one uses deterministic weighted reciprocal-rank fusion over eligible channel rankings.

The frozen fusion policy specifies:

- eligible channels by intent;
- candidate-pool construction;
- per-channel rank normalization;
- weights;
- duplicate-evidence collapse;
- correction/currentness filters;
- tie-breaking;
- top-k and truncation limits;
- missing-channel behavior.

Every fused result exposes:

- each contributing channel and rank;
- normalized contribution;
- shared input identities;
- known correlation, including shared source text, identifiers, descriptions, or correction records;
- fusion-policy digest;
- exclusion and truncation reasons.

Channel agreement is not represented as independent corroboration when channels share inputs. A fused score is a review priority, not confidence in truth.

## Candidate evidence and synthesized answers

Each successful query may produce two separately identified products.

### Candidate result

The candidate result contains ranked evidence references, channel contributions, coverage/accounting, limitations, and exact inspection selectors. It contains no uncited semantic conclusion.

### Synthesized answer

The optional synthesized answer is generated from only the admitted candidate packet. It:

- has its own immutable request, invocation, response, and terminal identities;
- cites every substantive claim to candidate evidence;
- separates observed evidence, model interpretation, uncertainty, and suggested next inspection;
- cannot add evidence or relations;
- cannot claim that a nominated change site is correct;
- remains `authority=0`, `accepted=false`, and `completeness=UNKNOWN`.

For change-planning queries, synthesis may describe likely seams, related tests, governing decisions, affected consumers, contrary evidence, and unresolved questions. It may not authorize or execute the change.

## Corrections and currentness

Corrections are append-only immutable records. They may:

- `SUPERSEDE`;
- `AMEND` a declared subset of fields or claims;
- `RETRACT` current eligibility while preserving historical inspection;
- `DISPUTE` without selecting a winner.

Every correction records valid time, transaction time, exact scope, reason, actor authority, and predecessor identities.

Release one defines `LATEST_NON_RETRACTED_WITH_CONFLICTS` as the default currentness policy:

- use the latest applicable non-retracted state when no unresolved conflict exists;
- preserve and return all applicable unresolved disputes;
- never resolve conflict by timestamp alone;
- retain historical index and query replay against exact prior selectors.

A correction creates new identities for affected views, indexes, caches, fusion results, and synthesized answers. Historical artifacts remain immutable.

## Replay and determinism

Lexical, structural, fusion, correction projection, and result rendering require byte-identical replay under exact inputs.

Semantic operations declare one replay class:

- `BYTE_EXACT` — identical canonical output bytes are required; or
- `RANK_EQUIVALENT` — a frozen equivalence rule and tolerance are required.

Hardware backend, runtime, model, tokenizer, quantization, preprocessing, grammar, and seed identities are part of the semantic invocation. A different backend or unavailable artifact creates a distinct invocation or typed replay failure; it is never silently substituted.

No replay may use ambient checkout, current session, network, process, model, cache, or filesystem fallback.

## Privacy and security

Queries, source text, catalog prose, embeddings, scores, snippets, caches, corrections, and synthesized answers are potentially sensitive.

The implementation must:

- remain local and offline;
- externally deny worker network access;
- use host-controlled canonical paths and verified digests;
- partition indexes and caches by privacy-policy identity;
- prohibit cross-partition retrieval and score leakage;
- bound query, corpus, candidate, source-byte, work, memory, output, and time limits;
- retain source-safe typed diagnostics without raw paths, source bodies, prompts, provider strings, or stderr;
- keep raw query and interaction records out of Git unless a separately sanitized investigation receipt is explicitly approved;
- fail closed on unsupported sandboxing;
- publish immutable records atomically with no replacement;
- support tombstones and rebuild receipts for approved deletion workflows.

Retained queries are governed by the same privacy partition and deletion policy as their admitted corpus.

Raw query records, candidate lists, prompts, model invocations, and local publication objects are not committed to Git by default. They remain in the host-owned local immutable publication store.

A durable investigation may produce a separately reviewed, sanitized receipt suitable for version control. That receipt contains only the investigation intent, immutable evidence selectors and digests, the decision or correction it informed, important rejected alternatives, remaining uncertainty, and links to related ADRs, issues, tasks, reviews, or commits. It must exclude raw source bodies, private paths, secrets, worker stderr, hidden prompts, and the complete interaction transcript.

Committing such a receipt requires an explicit human or repository-policy decision. Promoting a query to durable local retention does not itself authorize a Git commit.

## Interfaces

### CLI

Add one coherent discovery command after qualification:

```text
lsp-trace discover --index SELECTOR --query TEXT --intent INTENT \
  --ranking-profile PROFILE [--synthesize] [--machine]
```

A host-controlled configuration selects pinned local semantic resources. Request flags cannot provide executable, model, library, sandbox, grammar, cache, or publication paths.

CLI output returns one canonical query result or one typed failure. Human output is a bounded projection of the same result.

### MCP

Add one canonical operation after qualification:

```text
lsp_trace_v1_discover
```

The request carries immutable selectors, query text, intent policy, ranking profile, bounds, and synthesis choice. It carries no host paths or executable/model pins.

The result schema preserves candidate evidence and synthesized answer as separate records. It exposes exact accounting, authority ceilings, currentness policy, channel contributions, correlation disclosure, and replay identities.

CLI and MCP share one transport-neutral kernel and canonical schemas.

## Failure semantics

Failures use closed stage and code enums. Stages include:

- `QUERY_ADMISSION`;
- `INTENT_SELECTION`;
- `INDEX_LOAD`;
- `LEXICAL_RETRIEVAL`;
- `STRUCTURAL_RETRIEVAL`;
- `SEMANTIC_RETRIEVAL`;
- `FUSION`;
- `SYNTHESIS`;
- `PUBLICATION`;
- `REPLAY`.

Every admitted channel member receives one terminal disposition. Partial channel execution cannot imply absence. If the requested profile requires a failed channel, the query fails or returns an explicitly degraded result only when the frozen profile permits degradation. No hidden retry, alternate model, broadened corpus, repaired query, raised limit, or fallback profile is allowed.

## Evaluation and enablement gates

Semantic or hybrid ranking remains explicit opt-in until a frozen evaluation passes.

Evaluation is performed per query intent, evidence type, corpus, acquisition mode, correction state, and ambiguity class. It includes:

- lexical-only, structural-only, catalog-only, semantic-only, hybrid, and null/random baselines;
- known-item lookup and genuine change-planning queries reported separately;
- blinded human relevance judgments made without fused scores or generated answers;
- adversarial negatives with lexical overlap but distinct behavior;
- adversarial negatives with structural proximity but no shared feature identity;
- correction sensitivity and superseded-result leakage;
- provenance correctness and citation sufficiency;
- Recall@k, nDCG@k, candidate-site precision/recall, and contrary-evidence recall;
- unsupported synthesis-claim rate;
- replay, latency, memory, index-size, privacy, and terminal-accounting checks;
- constrained-decision calibration, choice-order sensitivity, tokenizer/branch-token sensitivity, abstention behavior, and false-negative rates before any hard-filter policy.

Before hybrid becomes a default profile, it must:

1. outperform lexical-only and structural-only baselines by frozen minimum effects on each enabled intent;
2. introduce no authority, acceptance, currentness, or provenance violations;
3. keep unsupported synthesis claims below a frozen threshold;
4. pass correction, conflict, replay, privacy, and network-denial tests;
5. pass independent maintainer review of the evaluation corpus and labels;
6. retain an explicit caller-selectable lexical/structural profile.

Passing evaluation authorizes a default retrieval profile. It never authorizes semantic acceptance or autonomous action.

## Implementation phases

### Phase 0 — Freeze contracts

- freeze view, query, intent, channel, correction, index, result, synthesis, failure, and replay schemas;
- freeze privacy, retention, deletion, and supply-chain policies;
- freeze evaluation corpus and enablement thresholds.

### Phase 1 — Deterministic lexical and structural kernel

- build immutable typed views and index identity;
- implement lexical and structural channels;
- implement accounting, inspection selectors, corrections, currentness, and replay;
- expose internal evaluation APIs only.

### Phase 2 — Pinned local semantic ranking

- qualify one pinned local model/runtime;
- implement bounded semantic candidate scoring;
- retain invocation and terminal records;
- keep semantic ranking explicit opt-in.

### Phase 3 — Fusion and synthesis

- implement deterministic fusion and correlation disclosure;
- implement evidence-only synthesis with claim citations;
- validate change-planning fixtures and adversarial negatives.

### Phase 4 — CLI and MCP

- expose `lsp-trace discover` and `lsp_trace_v1_discover` over one kernel;
- publish immutable query results and replay selectors;
- retain strict host/request separation.

### Phase 5 — Default-profile decision

- run the frozen evaluation;
- independently review results;
- adopt, revise, or reject hybrid defaulting in a separate decision amendment.

Each phase requires assertion-specific RED evidence before production changes and cannot silently authorize the next phase.

## Consequences

### Positive

- Natural-language discovery can support both lookup and change planning.
- Candidate evidence remains inspectable independently of synthesized prose.
- Search can use catalog descriptions without making them authoritative.
- Corrections and conflicts remain replayable rather than being overwritten.
- Local-model utility is available without hosted source disclosure.
- Lexical, structural, and semantic contributions remain separately auditable.

### Costs and risks

- Multiple immutable views and correction-derived indexes increase storage and rebuild work.
- Fused ranking requires careful normalization and correlation disclosure.
- Retained queries expand the privacy and deletion surface.
- Semantic replay may be backend-sensitive.
- Users may still over-trust fluent synthesis despite explicit authority ceilings.
- Change-site nominations require evaluation datasets that are harder to label than known-item lookup.

These costs are accepted only under bounded local execution, immutable provenance, and explicit enablement gates.

## Alternatives considered

### Search only ADR 0007 catalog prose

Rejected. It would be simple but would over-weight generated provisional descriptions and hide exact source, structural, decision, test, and correction evidence.

### Lexical and structural retrieval only

Retained as a baseline and caller-selectable profile, but not selected as the complete design because the intended natural-language and change-planning use cases benefit from a pinned local semantic channel.

### Semantic-first single index

Rejected. It obscures evidence types, weakens replay, and makes generated similarity appear more authoritative than it is.

### Separate channel results without fusion

Not selected as the default result shape. Separate channel outputs remain present, but deterministic fusion provides one review order while exposing every contribution and correlation.

### Hosted semantic service

Rejected. It violates local/offline source handling, host pinning, network denial, and replay requirements.

### Autonomous planning and editing agent

Rejected. Discovery and synthesis may nominate seams and next inspections, but execution and acceptance remain separate decisions.

## Relationship to ADR 0007 and ADR 0008

ADR 0007 remains the owner of local Describe, provisional catalog construction, semantic-worker containment, and feature-catalog authority boundaries. ADR 0009 consumes immutable ADR 0007 catalog records as one evidence type; it does not redefine or accept them.

ADR 0008 remains the owner of graph/source custody separation and deterministic source projection. ADR 0009 consumes only admitted projected records and server-reported structural evidence. It cannot hydrate, repair, broaden, or reinterpret graph evidence.

ADR 0010 adds deterministic post-inventory attribution from exact structural or retained-source anchors to provisional feature candidate IDs. ADR 0009 is implemented after ADR 0010 and may consume its immutable attribution artifacts as typed evidence; semantic retrieval cannot upgrade, repair, or silently replace exact attribution mappings.

This ADR adds candidate retrieval, deterministic fusion, bounded synthesis, query retention, correction-aware indexing, and change-planning interfaces. It does not weaken its predecessors or ADR 0010.
