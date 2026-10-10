# ADR0007 host-LLM bridge contract decision

- **Date:** 2026-10-08
- **Status:** additive design decision; execution disabled
- **Authority:** 0
- **Accepted:** false
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`
- **Applies to:** host orchestration over the ADR0007 bounded chain `Describe -> Search -> Group -> Location -> Source Text Search`
- **Depends on / stop gates:** ADR0011 C15 aggregate logical-buffer/resource-accounting acceptance, separate typed occurrence/input admission, Program C qualification, parallel-MCP runtime qualification, and separate execution authority

## Decision

Define a target-neutral host-LLM bridge contract for planning and disclosing a bounded ADR0007 inventory run without adding public schemas, CLI/MCP operations, code generation, runtime behavior, or executable changes.

The bridge preserves the existing separation:

```text
prepare bounded deterministic evidence
-> external host inference and orchestration planning
-> ingest only typed, admitted, immutable evidence/results
```

The lsp-trace implementation never invokes models, providers, or host harnesses in qualification paths or bridge execution paths. The external host owns model/provider/harness selection, credentials, disclosure permissions, prompting, transcript retention, cancellation before handoff, inference execution, and user interaction. lsp-trace prepares bounded evidence packets for possible external use and later ingests only typed immutable inference results that a separately authorized contract admits. lsp-trace owns deterministic operations, immutable evidence identity, admission gates, accounting, custody, and fail-closed ingestion rules that are separately authorized and qualified.

Execution is explicitly **DISABLED** until all required gates are met and a separate execution authority exists:

1. **ADR0011 C15 aggregate accounting gate:** currently `UNKNOWN`; concerns aggregate logical-buffer/resource-accounting acceptance. C15 does **not** by itself provide typed input-family or occurrence admission for this bridge.
2. **Typed occurrence/input admission gate:** a separate required contract must admit the exact input and inference-result families consumed by the bridge.
3. **Program C qualification gate:** a separate qualification receipt must admit the exact grouping/profile/policy consumed by the bridge.
4. **Parallel-MCP runtime qualification gate:** retained qualification must demonstrate safe, bounded behavior for realistic concurrent requests from one host against the same and different managed sessions. It must preserve C18 ownership, refusal-before-effect, exact-once terminal accounting, stale-generation rejection, and fresh-owner restart semantics. Parallel arrival or completion order is never semantic order; dependent requests require explicit predecessor identities. This gate qualifies existing behavior and does not pre-authorize a mailbox or scheduler.
5. **Execution-authority gate:** even after the four technical gates pass, a separate decision must authorize execution/public surface exposure for this bridge.

Until all of those gates pass, this document is prose design only. It does not authorize public CLI, MCP, API, schema registry, code generation, private runtime dispatch, local model execution, hosted model execution, production enablement, release, migration, or mutation of frozen artifacts.

## Normative cross-links and boundaries

This decision is additive to:

- [ADR0007 optional local semantic feature index](../../../adr/0007-optional-local-semantic-feature-index.md)
- [ADR0007 caller-provided inference amendment](../../../adr/0007-caller-provided-inference-amendment.md)
- [ADR0007 normative consolidation decision](2026-10-08-normative-consolidation.md)
- [ADR0011 versioned definition/reference grouping evidence](../../../adr/0011-versioned-definition-reference-grouping-evidence.md)
- [ADR0009 provenance-bounded semantic discovery](../../../adr/0009-provenance-bounded-semantic-discovery.md)

It does not mutate frozen campaign roots, predecessor manifests, schemas, qualification artifacts, runtime directories, or existing ADR text. Later accepted contracts may cite this design, but this design alone is not an implementation authority.

## Goals

- Let a host answer a user request such as “find features that do X” by planning over the qualified bounded chain.
- Make inventory/cache reuse explicit rather than magical.
- Preserve exact source, workspace, revision, freeze, profile, policy, and privacy compatibility checks.
- Plan only bounded incremental capture when compatible evidence is missing or stale.
- Sequence deterministic operations predictably.
- Return one consolidated result identity that binds reused and fresh evidence.
- Disclose omissions, limits, failures, freshness decisions, and match reasons.
- Preserve custody/provenance and authority ceilings.
- Fail closed on any unadmitted input, unqualified policy, incompatible cache, ambiguous identity, missing custody, or partial terminal accounting.

## Non-goals and non-authorizations

The bridge does not authorize:

- lsp-trace model invocation in qualification paths;
- automatic external inference from lsp-trace;
- hidden host model/provider selection by lsp-trace;
- public CLI/MCP/API/schema/codegen/runtime;
- registration of public request/result schemas;
- execution before ADR0011 C15 aggregate accounting acceptance, separate typed occurrence/input admission, Program C qualification, and separate execution authority all pass;
- repair, retry, or substitution of semantic attempts;
- feature identity, feature completeness, ownership, acceptance, product authority, runtime authority, or implementation correctness;
- conversion of Group candidates into accepted features;
- inference of structural facts from source text or model output;
- cache presence as freshness;
- workspace broadening, source disclosure, or cross-privacy-partition retrieval by default.

All bridge products remain:

```text
authority = 0
accepted = false
completeness = UNKNOWN
featureIdentity = UNRESOLVED
```

Representative status is separately typed. A representative can be `AVAILABLE`, `UNAVAILABLE`, `NO_CALLS_REPRESENTATIVE`, `UNQUALIFIED`, or `NOT_REQUESTED`; none of those statuses accepts feature identity or raises authority.

## Conceptual request schema

This is a strict conceptual schema for prose design. It is not a registered public schema.

| Field | Required | Meaning | Fail-closed condition |
| --- | --- | --- | --- |
| `bridgeContractVersion` | yes | Exact bridge prose-contract version, initially `ADR0007_HOST_LLM_BRIDGE_CONCEPT_V0`. | Unknown version. |
| `intentText` | yes | User-visible natural-language intent retained by the host. | Empty, hidden, or redacted so identity cannot be checked. |
| `hostInvocation` | yes | Host-owned model/provider/harness identity or `NO_MODEL_USED`. | Claimed lsp-trace-owned model inference. |
| `privacyPolicyId` | yes | Privacy/disclosure partition governing source, prompt, result, and cache access. | Missing or incompatible partition. |
| `workspaceSelector` | yes | Exact workspace URI, repository identity, and allowed source roots. | Ambiguous workspace or broader root than user allowed. |
| `revisionSelector` | yes | Exact commit/revision/session/freeze selector and cleanliness policy. | Dirty or moving revision unless explicitly admitted by typed policy. |
| `freezeSelectors` | yes | Exact ADR0007, ADR0011 C15 aggregate-accounting, typed occurrence/input admission, and Program C freeze/profile/policy identities required for each stage. | Missing, unknown, unqualified, or mismatched freeze/profile/policy. |
| `inventorySelector` | optional | Existing immutable inventory/cache candidate to assess. | Cache used without compatibility and freshness decision. |
| `freshnessPolicy` | yes | Exact policy for accepting, rejecting, or supplementing existing evidence. | Policy omitted or host invents freshness ad hoc. |
| `capturePlanBounds` | yes | Max stages, files, symbols, ranges, source bytes, objects, work, messages, wall time, and output bytes. | Any unbounded dimension. |
| `operationPlan` | yes | Ordered stage requests over Describe, Search, Group, Location, Source Text Search, externally owned handoff, and typed immutable inference-result ingestion. | Non-deterministic or model-selected runtime operations inside lsp-trace. |
| `handoffPolicy` | yes | Host-owned handoff/cancellation policy and prepared-packet disclosure bounds. | Missing host ownership or attempted lsp-trace inference. |
| `ingestRequirements` | yes | Typed immutable inference-result family, receipt, custody, and rejection requirements. | Untyped, mutable, host-prose-only, or identity-mismatched inference result. |
| `admissionRequirements` | yes | Required ADR0011 C15 aggregate accounting status, separate typed input/occurrence admission, typed inference-result admission, Program C status, parallel-MCP runtime qualification, and execution authority. | Any required gate is unknown, absent, unqualified, or unauthorized. |
| `disclosureRequirements` | yes | Required reused/fresh/omitted/failed/limit/match-reason disclosures. | Any required disclosure omitted. |
| `cancellationDeadline` | yes | Caller deadline and cancellation precedence. | Missing deadline/cancellation semantics. |
| `idempotencyKey` | yes | Host-supplied deterministic key for replay/idempotence over exact request bytes. | Missing or reused with different request bytes. |

### Conceptual request example

```yaml
bridgeContractVersion: ADR0007_HOST_LLM_BRIDGE_CONCEPT_V0
intentText: "Find candidate features related to retained source projection."
hostInvocation:
  owner: external-host
  modelProvider: host-owned
  lspTraceInvokedModel: false
privacyPolicyId: local-private-default
workspaceSelector:
  uri: file:///repo
  allowedRoots: [file:///repo]
revisionSelector:
  commit: abc123
  cleanliness: CLEAN_REQUIRED
freezeSelectors:
  adr0007Chain: Describe-Search-Group-Location-SourceTextSearch-qualified-identities
  adr0011C15AggregateAccounting: ACCEPTED_REQUIRED_CURRENTLY_UNKNOWN
  typedOccurrenceInputAdmission: REQUIRED_SEPARATE_CONTRACT
  typedInferenceResultAdmission: REQUIRED_SEPARATE_CONTRACT
  programC: QUALIFIED_REQUIRED
  executionAuthority: REQUIRED_SEPARATE_DECISION
inventorySelector: optional-existing-inventory-digest
freshnessPolicy: EXACT_REVISION_OR_STALE
capturePlanBounds:
  maxStages: 5
  maxSourceBytes: 2097152
  maxObjects: 1000
  maxWork: 10000
operationPlan:
  - INVENTORY_CACHE_ASSESSMENT
  - DESCRIBE_REUSE_OR_CAPTURE
  - SEARCH_REUSE_OR_CAPTURE
  - GROUP_REUSE_OR_CAPTURE
  - LOCATION_REUSE_OR_CAPTURE
  - SOURCE_TEXT_SEARCH_REUSE_OR_CAPTURE
  - HANDOFF_PREPARE_ONLY
  - EXTERNAL_HOST_INFERENCE_NOT_LSP_TRACE
  - INFERENCE_RESULT_INGEST_REQUESTED
admissionRequirements:
  failIfAdr0011C15AggregateAccountingNotAccepted: true
  failIfTypedOccurrenceInputAdmissionMissing: true
  failIfTypedInferenceResultAdmissionMissing: true
  failIfProgramCNotQualified: true
  failIfExecutionAuthorityMissing: true
handoffPolicy:
  owner: external-host
  lspTraceInvokesModel: false
  hostEvaluatesCancelBeforeHandoff: true
ingestRequirements:
  requireImmutableInferenceResult: true
  rejectHostProseWithoutTypedReceipt: true
disclosureRequirements:
  includeReusedEvidence: true
  includeFreshEvidence: true
  includeOmissions: true
  includeLimitsAndFailures: true
  includeMatchReasons: true
cancellationDeadline:
  deadlineMs: 30000
  cancelPrecedence: CANCEL_BEFORE_NEW_STAGE
idempotencyKey: sha256-of-canonical-request
```

## Conceptual result schema

This is a strict conceptual schema for prose design. It is not a registered public schema.

| Field | Meaning |
| --- | --- |
| `bridgeResultVersion` | Exact conceptual result version. |
| `requestDigest` | Digest of canonical request bytes. |
| `terminalStatus` | One of `DISABLED_STOP_GATE`, `COMPLETE`, `DEGRADED_COMPLETE`, `FAILED_CLOSED`, `CANCELLED`, `DEADLINE_EXCEEDED`, `LIMIT_EXCEEDED`, `REPLAY_MATCH`, `REPLAY_MISMATCH`. Current status must be `DISABLED_STOP_GATE`. |
| `stopGateStatus` | Separate statuses and exact evidence selectors for ADR0011 C15 aggregate accounting, typed occurrence/input admission, typed inference-result admission, Program C qualification, parallel-MCP runtime qualification, and execution authority. |
| `consolidatedResultId` | Digest over request, reused evidence, fresh evidence, policies, operation receipts, disclosures, terminal accounting, and result projection. |
| `inventoryDecision` | Reuse/reject/supplement decision for every candidate cache/inventory. |
| `freshnessDecision` | Exact compatibility/freshness outcome for source, workspace, revision, freeze, profile, policy, and privacy partition. |
| `operationReceipts` | Ordered deterministic receipts for each planned stage, including skipped stages, handoff-prepared/handed-off records, and ingest-requested/admitted/rejected records. |
| `evidenceDisclosure` | Reused evidence, fresh evidence, omitted evidence, rejected cache entries, and unavailable evidence. |
| `candidateArtifacts` | Authority-zero candidate records only; accepted false; unresolved feature identity; candidate publication, if any, refers to one immutable qualified candidate-group generation. |
| `candidatePublication` | Optional private-only publication status using existing candidate-generation selectors/receipts; no migration, no public schema, no accepted inventory. |
| `representativeStatus` | Separately typed representative status independent of feature identity and never inferred from publication. |
| `matchReasons` | Per-candidate reason records with source stage, evidence selector, channel, and limits. |
| `custodyProvenance` | Custody chain, publication/default privacy status, source selectors, admission receipts, producer identities, external host handoff custody, and immutable inference-result custody transitions. |
| `limitsAccounting` | Requested limits, consumed limits, plus-one failures, truncations, and omitted counts. |
| `failures` | Stage/code/status records with source-safe diagnostics. |
| `replay` | Idempotency/replay classification and exact inputs required for replay. |
| `securityPrivacy` | Disclosure partition, archive boundary, sanitized/unsanitized result class, and retention class. |

### Conceptual disabled result example

```yaml
bridgeResultVersion: ADR0007_HOST_LLM_BRIDGE_RESULT_CONCEPT_V0
requestDigest: sha256:...
terminalStatus: DISABLED_STOP_GATE
stopGateStatus:
  adr0011C15AggregateAccounting: REQUIRED_UNKNOWN
  typedOccurrenceInputAdmission: REQUIRED_NOT_PRESENT
  typedInferenceResultAdmission: REQUIRED_NOT_PRESENT
  programC: REQUIRED_NOT_QUALIFIED
  executionAuthority: REQUIRED_NOT_GRANTED
consolidatedResultId: null
inventoryDecision: []
freshnessDecision: []
operationReceipts: []
evidenceDisclosure:
  reused: []
  fresh: []
  omitted:
    - reason: EXECUTION_DISABLED_PENDING_STOP_GATES
candidateArtifacts: []
representativeStatus: NOT_REQUESTED
limitsAccounting:
  consumedWork: 0
failures:
  - stage: BRIDGE_ADMISSION
    code: EXECUTION_DISABLED
    safeMessage: "ADR0011 C15 aggregate accounting acceptance, separate typed occurrence/input admission, typed inference-result admission, Program C qualification, and separate execution authority are required before bridge execution."
replay:
  idempotent: true
  replayClass: DISABLED_NO_EFFECT
securityPrivacy:
  archiveClass: DESIGN_ONLY_NO_SOURCE_CAPTURE
```

## State machine

| State | Entry condition | Permitted transition | Forbidden transition |
| --- | --- | --- | --- |
| `DESIGN_ONLY_DISABLED` | This document exists; all required gates and execution authority are not satisfied. | `STOP_GATES_SATISFIED` only after separate accepted ADR0011 C15 aggregate accounting, typed occurrence/input admission, typed inference-result admission, Program C qualification, parallel-MCP runtime qualification, and execution-authority receipts. | Any execution, schema registration, public surface, runtime dispatch, or inference handoff. |
| `STOP_GATES_SATISFIED` | Exact gate receipts are independently accepted; C15 is not treated as typed input-family admission. | `REQUEST_ADMITTED` if a future authorized implementation admits exact request bytes. | Host-only claim that gates are satisfied without lsp-trace verification. |
| `REQUEST_ADMITTED` | Conceptual request has exact identity, limits, privacy, workspace, revision, and freeze selectors. | `INVENTORY_ASSESSED`. | Operation execution before inventory/cache assessment. |
| `INVENTORY_ASSESSED` | Every candidate cache/inventory has compatibility and freshness decision. | `CAPTURE_PLANNED`. | Cache reuse without disclosure. |
| `CAPTURE_PLANNED` | Missing/stale evidence has bounded incremental plan. | `OPERATIONS_SEQUENCED`. | Unbounded capture or host-selected hidden operations. |
| `OPERATIONS_SEQUENCED` | Deterministic stage order and dependencies are frozen. | `PREPARE_EXECUTED` in a future authorized implementation. | Reordering by model output after admission. |
| `PREPARE_EXECUTED` | lsp-trace has prepared bounded deterministic evidence packets and receipts only. | `HANDOFF_PREPARED` or `FAILED_CLOSED`. | lsp-trace invoking any model/provider/harness. |
| `HANDOFF_PREPARED` | Prepared packet identity, bounds, privacy policy, and host handoff target are recorded. | `HANDED_OFF`, `HANDOFF_CANCELLED_BY_HOST`, or `FAILED_CLOSED`. | Automatic lsp-trace inference. |
| `HANDED_OFF` | External host accepts custody of the prepared packet. | `INGEST_REQUESTED` when the host returns a typed immutable inference result. | Treating handoff as inference success. |
| `INGEST_REQUESTED` | Host submits a typed immutable inference-result receipt for admission. | `INGEST_ADMITTED`, `INGEST_REJECTED`, or `FAILED_CLOSED`. | Ingesting free-form prose or mutable results. |
| `INGEST_ADMITTED` | lsp-trace admits the immutable inference-result under a separately qualified typed contract. | `OPERATIONS_EXECUTED`. | Authority elevation from inference admission. |
| `INGEST_REJECTED` | Inference result is untyped, mutable, identity-mismatched, over-limit, wrong-custody, or otherwise inadmissible. | `FAILED_CLOSED` with safe diagnostics. | Repairing or normalizing the semantic result silently. |
| `OPERATIONS_EXECUTED` | Every deterministic stage, handoff, and ingest step has terminal accounting. | `RESULT_CONSOLIDATED`. | Silent partial success. |
| `RESULT_CONSOLIDATED` | Consolidated result identity binds all reused/fresh/skipped/failed evidence. | `RESULT_DISCLOSED` or `REPLAYED`. | Candidate acceptance or feature identity elevation. |
| `RESULT_DISCLOSED` | Required disclosures are present. | `ARCHIVED_PRIVATE_ONLY` or caller review outside lsp-trace. | Public publication by default. |
| `FAILED_CLOSED` | Any required check fails. | Replay/report safe failure. | Retry, fallback, cache substitution, or semantic repair. |
| `CANCELLED` / `DEADLINE_EXCEEDED` | Cancellation or deadline observed at a defined precedence point. | Safe terminal accounting. | Continue into a new stage. |

Current allowed state is only `DESIGN_ONLY_DISABLED`.

## Inventory/cache assessment

A host may propose reuse of existing inventories or cache entries, but lsp-trace must independently classify each candidate as one of:

| Decision | Meaning |
| --- | --- |
| `REUSE_COMPATIBLE_FRESH` | Exact source/workspace/revision/freeze/profile/policy/privacy selectors match and freshness policy accepts reuse. |
| `REUSE_COMPATIBLE_STALE_DISCLOSED` | Reuse is allowed only as historical evidence, with explicit stale disclosure and no currentness claim. |
| `SUPPLEMENT_REQUIRED` | Some compatible evidence exists, but missing/stale portions require bounded incremental capture. |
| `REJECT_INCOMPATIBLE` | Selector, custody, policy, privacy, profile, or freeze mismatch. |
| `REJECT_UNQUALIFIED` | Required ADR0011 C15 aggregate accounting, typed admission, Program C, or execution-authority gate is not satisfied. |
| `REJECT_UNKNOWN_CUSTODY` | Custody or provenance is missing or unverifiable. |

Cache compatibility is exact by default. A future policy may define coarser compatibility only if separately versioned and qualified. Cache freshness is never inferred from existence, newest timestamp, source path, or host confidence.

## Source/workspace/revision/freeze compatibility

The bridge must compare:

- logical source URI and allowed roots;
- workspace identity and session generation, when applicable;
- commit/revision and cleanliness/freeze policy;
- source digest, byte length, encoding, and range selectors;
- ADR0007 stage freeze/profile/policy identities;
- ADR0011 C15 aggregate logical-buffer/resource-accounting identity and status;
- separate typed occurrence/input admission family/version and occurrence/source identities;
- separate typed immutable inference-result admission family/version and receipt identity;
- Program C algorithm/profile/policy/seed/numeric contract identities;
- privacy partition and disclosure class;
- retained publication selector and custody chain.

Any unknown, ambiguous, broader, or mismatched selector fails closed or becomes a disclosed omission. It cannot be silently narrowed or broadened.

## Bounded incremental capture planning

Incremental capture is permitted only in a future authorized implementation and only for missing/stale evidence that passes typed admission and bounds. A capture plan must name:

1. exact stage;
2. predecessor evidence identity;
3. reason for fresh capture;
4. source/workspace/revision/freeze selector;
5. maximum work/messages/bytes/objects/ranges/time;
6. expected terminal outcomes;
7. admission family and profile, distinguishing C15 aggregate accounting from typed occurrence/input admission;
8. typed inference-result ingest family, if external host inference may be returned;
9. privacy partition;
10. cancellation point and whether it is lsp-trace-preparation or external-host-handoff cancellation;
11. how fresh evidence will be distinguished from reused evidence.

A model may suggest what to inspect, but deterministic lsp-trace admission owns the actual bounded plan. Suggestions that cannot be admitted become host commentary outside the bridge result.

## Deterministic operation sequencing

The bridge order is fixed unless a future versioned policy says otherwise:

```text
1. BRIDGE_ADMISSION
2. STOP_GATE_CHECK
3. INVENTORY_CACHE_ASSESSMENT
4. FRESHNESS_COMPATIBILITY_CHECK
5. INCREMENTAL_CAPTURE_PLAN
6. DESCRIBE_STAGE
7. SEARCH_STAGE
8. GROUP_STAGE
9. LOCATION_STAGE
10. SOURCE_TEXT_SEARCH_STAGE
11. HANDOFF_PREPARED
12. HANDED_OFF_BY_EXTERNAL_HOST_OR_CANCELLED
13. INFERENCE_RESULT_INGEST_REQUESTED
14. INFERENCE_RESULT_ADMISSION_OR_REJECTION
15. RESULT_CONSOLIDATION
16. DISCLOSURE_RENDERING
17. PRIVATE_ARCHIVE_OR_REPLAY_RECORD
```

Each stage either executes, reuses compatible evidence, is skipped by policy, hands off to the external host, admits a typed immutable inference result, rejects an inference result, or fails with one terminal disposition. A later stage may not infer success or absence from an earlier omitted or failed stage. The external host, not lsp-trace, evaluates and records cancellation before external handoff; lsp-trace records only the host-supplied cancellation/handoff receipt and its own deterministic preparation/ingest terminal state.

## Parallel MCP runtime qualification

Before automatic host-LLM execution may leave `DESIGN_ONLY_DISABLED`, retained qualification must exercise realistic parallel MCP requests. At minimum it covers read/read, read/acquisition, acquisition/acquisition, retention/readback, acquisition/STOP, STOP/restart, duplicate request identities, stale generations, and cancellation at each qualified effect boundary.

Operations are classified by their actual effects and dependencies. Independent immutable reads or computations may proceed concurrently. Shared mutation and lifecycle operations preserve existing owner and barrier semantics. Transport arrival, handler scheduling, and response completion order do not establish semantic order. A dependent request supplies the exact session generation, owner token, artifact/checkpoint identity, selector version, or other predecessor required by its operation; absent or stale predecessors fail with typed outcomes rather than inferred FIFO intent.

Qualification requires no deadlock, no partial irreversible effects, exact-once terminal response/effect accounting, bounded latency and resource consumption, source-safe attributable diagnostics, typed recoverable refusal where permitted, stale-authority rejection, and historical C18 behavior unchanged. It records refusal rates and contention outcomes for each matrix cell. A bounded mailbox or scheduler remains deferred and may be authorized only by a later decision if retained evidence demonstrates harmful contention, starvation, ambiguous recovery, or unacceptable refusal rates; this contract does not prescribe one.

## Consolidated result identity

`consolidatedResultId` is conceptually computed from canonical bytes for:

- request identity and idempotency key;
- stop-gate receipts, separately including ADR0011 C15 aggregate accounting, typed occurrence/input admission, typed inference-result admission, Program C, and execution authority;
- inventory/cache decisions;
- all reused evidence selectors;
- all fresh evidence selectors;
- all skipped/omitted/failed stage records;
- stage policies, freezes, profiles, limits, accounting, handoff receipts, and ingest receipts;
- candidate artifact identities;
- candidate-generation publication selector, receipt, predecessor selector, and `PublishVerifiedGeneration` receipt when present;
- representative status records;
- disclosure projection policy.

Equal visible candidates produced from different evidence, freshness decisions, policies, privacy partitions, or failures must have different consolidated identities.

## Disclosure requirements

Every result must disclose:

- reused evidence and why it was compatible;
- fresh evidence and why it was captured;
- omitted evidence and whether omission was due to limits, privacy, incompatibility, unavailability, cancellation, deadline, or disabled execution;
- rejected cache entries and exact rejection reasons;
- stage failures with source-safe diagnostics;
- reached and unreached limits;
- match reasons per candidate, including stage and evidence selector;
- representative status separately from group membership;
- currentness/freshness status;
- custody and provenance chain;
- host-owned inference role and lsp-trace prepare/ingest-only role;
- handoff state, host cancellation-before-handoff state, and immutable inference-result ingest disposition.

Disclosure must not expose raw source, prompts, private paths, provider secrets, stderr, or full transcripts unless the privacy policy explicitly permits it.

## Private-only publication defaults

Bridge candidate artifacts default to private-only publication. They remain:

```text
authority = 0
accepted = false
featureIdentity = UNRESOLVED
completeness = UNKNOWN
```

Candidate artifacts are not accepted features. Group membership is not feature identity. Representative status is separately typed and may be unavailable. Private publication stores may retain exact immutable bytes for replay, but public Git documents should contain only sanitized decisions or summaries explicitly approved for archive.

## Candidate-generation publication synchronization

The bridge incorporates the candidate-generation publication decision in the caller-provided inference amendment without reopening its qualified semantics:

- the publishable unit is one immutable qualified candidate-group generation, not an accepted feature inventory and not a bundle that silently combines independently qualified products;
- existing `PutCandidateGroup`, `GetCandidateGroup`, selectors, receipts, and verified-generation publication contracts are preserved;
- any future bridge adapter may only use the existing `PublishVerifiedGeneration` path and must not revive, cherry-pick, migrate, normalize, or reinterpret historical publication machinery;
- only an explicit repository-local owner operation may advance a current-generation selector, and only after strict receipt verification;
- qualification success, caller input, filesystem recency, cache presence, host confidence, or ambient “latest” selection cannot advance a selector;
- the private publication package owns the additive schema and canonical decoding; this bridge document registers no public schema;
- the publication receipt binds the exact candidate artifact and byte digest, grouping-policy or qualification identity, source revision, generation identity, and predecessor selector;
- selector transition is ordered after durable artifact and receipt commitment, preserves cancellation boundaries and truthful committed-success semantics, detects tampering and immutable collisions, supports restart-safe retrieval and idempotent repetition, and fails closed under noncanonical input;
- unresolved Group candidates may be published only with explicit unresolved status while preserving `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`;
- representative status remains separately typed and is never inferred, upgraded, or accepted by publication;
- the initial adapter remains repository-private and defines no public schema, CLI/MCP advertisement, production dispatch, release, push authority, or migration.

This synchronization is a policy boundary only. It does not authorize implementation, modify qualified semantics, change existing Put/Get/selectors/receipts, migrate historical artifacts, or create execution authority for this bridge.

## Custody and provenance

Custody records must preserve:

- producer identity for each deterministic lsp-trace stage;
- host identity for external inference and orchestration suggestions;
- `HANDOFF_PREPARED` receipt identity for lsp-trace prepared packet custody;
- `HANDED_OFF` or host-cancelled handoff receipt identity for external host custody;
- `INGEST_REQUESTED`, `INGEST_ADMITTED`, or `INGEST_REJECTED` receipt identity for immutable inference-result custody;
- candidate-generation `Put`/`Get`/selector/receipt identity, predecessor selector, and `PublishVerifiedGeneration` receipt identity when publication is in scope;
- explicit repository-local owner receipt for any selector advancement;
- admission receipt identities;
- publication/default privacy status;
- source selector and revision provenance;
- policy/freeze/profile provenance;
- correction/supersession provenance;
- exact terminal disposition for every admitted item.

External host inference can explain, rank, or suggest within the host boundary. It cannot become lsp-trace custody for deterministic evidence, cannot admit occurrences, cannot qualify Program C, and cannot raise candidate authority. lsp-trace custody may resume only for a typed immutable inference result submitted through `INGEST_REQUESTED` and admitted through a separately qualified ingest contract; rejected inference results remain external-host artifacts with safe rejection diagnostics.

## Fail-closed behavior

The bridge fails closed for:

- ADR0011 C15 aggregate logical-buffer/resource-accounting acceptance is unknown or not accepted;
- typed occurrence/input admission contract is absent or not qualified;
- typed immutable inference-result admission contract is absent or not qualified;
- Program C not qualified;
- parallel-MCP runtime qualification absent, incomplete, stale, or mismatched;
- separate execution authority not granted;
- unknown contract version;
- unbounded limits;
- missing privacy policy;
- incompatible workspace/revision/freeze/profile/policy;
- dirty or moving workspace when clean/frozen revision is required;
- cache reuse without compatibility/freshness decision;
- missing custody/provenance;
- model/provider/harness claimed as lsp-trace-owned inference;
- missing, mutable, untyped, over-limit, or identity-mismatched inference-result ingest receipt;
- partial terminal accounting;
- cancellation/deadline before a new stage;
- result identity mismatch;
- replay mismatch;
- attempted public publication by default;
- candidate-generation selector advancement without explicit repository-local owner action and strict receipt verification;
- candidate publication through anything other than an existing `PublishVerifiedGeneration` path;
- migration, normalization, or reinterpretation of historical candidate artifacts.

Fail-closed results are still useful artifacts if they disclose safe diagnostics and consumed limits. They do not authorize retry with broader evidence, alternate model, alternate policy, or hidden fallback.

## Replay and idempotence

A request with the same canonical bytes and idempotency key must either return the same disabled/no-effect result, the same replay result over retained immutable inputs, or a typed replay failure. Replay may not use ambient checkout state, current filesystem contents, current host memory, current model availability, current cache contents, or network access unless those identities were part of the admitted immutable inputs.

Idempotence covers terminal accounting and publication effects. It does not imply concurrency safety, semantic ordering, or predecessor satisfaction beyond the separately qualified publication/admission and parallel-MCP runtime contracts.

## Limits and accounting

Limits must account for:

- source bytes admitted and returned;
- candidate objects;
- source ranges;
- stages;
- operation messages/requests;
- work units;
- wall-clock deadline;
- result bytes;
- omitted/truncated counts;
- cancellation/deadline precedence;
- plus-one limit failures where applicable.

A limit hit is a result fact, not a reason to infer absence. If a limit prevents complete inspection, the result must say so.

## Cancellation and deadlines

Cancellation and deadlines are observed at declared boundaries:

1. before admitting a new deterministic lsp-trace stage;
2. before preparing a handoff packet;
3. before external host inference handoff, evaluated and recorded by the external host orchestration rather than by automatic lsp-trace inference;
4. before typed inference-result ingest;
5. before publication;
6. before archive/export.

If cancellation or deadline fires before or during lsp-trace preparation/ingest, lsp-trace records completed stage receipts, marks not-started stages as omitted due to cancellation/deadline, and publishes no partial candidate as complete. If cancellation fires before external handoff, the external host records `HANDOFF_CANCELLED_BY_HOST` or equivalent and no lsp-trace inference occurs. Cancellation cannot be converted into success by the host.

## Security, privacy, and archive boundaries

- The bridge does not transmit source to an external provider by itself; only the external host can perform handoff under its own disclosure authority.
- The host owns any source disclosure decision to its model/provider/harness.
- lsp-trace results default to private local immutable publication, not Git.
- Candidate-generation publication remains repository-private and private-package-owned; it grants no public schema or public surface.
- Public docs may contain sanitized design decisions only.
- Raw prompts, transcripts, source bodies, provider responses, private paths, credentials, stderr, and cache internals are excluded from public archive by default.
- Privacy partitions prevent cross-partition retrieval, scoring, cache reuse, or disclosure.
- Deletion/tombstone workflows are separate and do not mutate historical public docs.
- Supply-chain and sandbox claims require their own qualification; this bridge does not establish them.

## Interaction with the host

The host may:

- interpret a user goal;
- ask follow-up questions;
- propose an operation plan;
- receive a prepared handoff packet and record `HANDOFF_PREPARED`, `HANDED_OFF`, or host-cancelled status;
- select and execute its own model/provider/harness;
- hold transcripts and prompts under its own policy;
- display lsp-trace evidence and limitations to the user.

The host may not, within this bridge:

- cause lsp-trace to invoke a model in qualification or bridge execution paths;
- bypass typed admission or treat ADR0011 C15 aggregate accounting as typed input-family admission;
- substitute model judgment for Program C qualification;
- hide reused/fresh evidence distinction;
- hide omissions, failures, or limits;
- accept features;
- advance candidate-generation selectors except through explicit repository-local owner action after strict receipt verification;
- treat publication as migration, normalization, representative acceptance, or feature identity;
- claim completeness, ownership, authority, or correctness.

## Open questions left explicit

1. What exact ADR0011 C15 aggregate logical-buffer/resource-accounting artifact names will the bridge consume once C15 is no longer `UNKNOWN`?
2. What separate typed occurrence/input admission contract admits each input family needed by the bridge?
3. What separate typed immutable inference-result admission contract admits host-returned inference results?
4. What exact Program C qualification receipt, policy, seed, and numeric contract will qualify the Group stage for this bridge?
5. Which separate execution authority, if any, may enable bridge execution after the technical gates pass?
6. Which freshness policy variants are acceptable for common workflows?
7. Which private-only publication store and retention class should hold disabled/no-effect request records?
8. What sanitized public receipt format, if any, should summarize a completed host-LLM bridge run?
9. Which host skill/prompt/resource should teach the workflow without becoming authority?
10. Which explicit repository-local owner operation, if any, may later advance bridge-related candidate-generation selectors after strict receipt verification?

## Verdict

`ADR0007_HOST_LLM_BRIDGE_DESIGN_READY` for documentation-only review.

`EXECUTION_DISABLED_PENDING_ADR0011_C15_AGGREGATE_ACCOUNTING_TYPED_ADMISSION_PROGRAM_C_AND_EXECUTION_AUTHORITY` for any runtime, public surface, schema registration, code generation, handoff execution, ingest execution, or model execution.
