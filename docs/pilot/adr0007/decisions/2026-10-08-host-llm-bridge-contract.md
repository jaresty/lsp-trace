# ADR0007 host-LLM bridge contract decision

- **Date:** 2026-10-08
- **Status:** additive design decision; execution disabled
- **Authority:** 0
- **Accepted:** false
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`
- **Applies to:** host orchestration over the ADR0007 bounded chain `Describe -> Search -> Group -> Location -> Source Text Search`
- **Depends on / stop gates:** ADR0011 C15 typed admission and Program C qualification

## Decision

Define a target-neutral host-LLM bridge contract for planning and disclosing a bounded ADR0007 inventory run without adding public schemas, CLI/MCP operations, code generation, runtime behavior, or executable changes.

The bridge preserves the existing separation:

```text
prepare bounded deterministic evidence
-> external host inference and orchestration planning
-> ingest only typed, admitted, immutable evidence/results
```

The lsp-trace implementation never invokes models in qualification paths. The external host owns model/provider/harness selection, credentials, disclosure permissions, prompting, transcript retention, and user interaction. lsp-trace owns only the deterministic operations, immutable evidence identity, admission gates, accounting, custody, and fail-closed ingestion rules that are separately authorized and qualified.

Execution is explicitly **DISABLED** until both stop gates are met:

1. ADR0011 C15 typed admission admits the exact input families needed by the bridge.
2. Program C qualification admits the exact grouping/profile/policy consumed by the bridge.

Until those gates pass, this document is prose design only. It does not authorize public CLI, MCP, API, schema registry, code generation, private runtime dispatch, local model execution, hosted model execution, production enablement, release, migration, or mutation of frozen artifacts.

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
- execution before ADR0011 C15 and Program C gates pass;
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
| `freezeSelectors` | yes | Exact ADR0007/ADR0011/Program C freeze/profile/policy identities required for each stage. | Missing, unknown, unqualified, or mismatched freeze/profile/policy. |
| `inventorySelector` | optional | Existing immutable inventory/cache candidate to assess. | Cache used without compatibility and freshness decision. |
| `freshnessPolicy` | yes | Exact policy for accepting, rejecting, or supplementing existing evidence. | Policy omitted or host invents freshness ad hoc. |
| `capturePlanBounds` | yes | Max stages, files, symbols, ranges, source bytes, objects, work, messages, wall time, and output bytes. | Any unbounded dimension. |
| `operationPlan` | yes | Ordered stage requests over Describe, Search, Group, Location, and Source Text Search. | Non-deterministic or model-selected runtime operations. |
| `admissionRequirements` | yes | Required typed admission families and Program C status. | ADR0011 C15 or Program C not qualified. |
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
  adr0011Admission: C15_REQUIRED
  programC: QUALIFIED_REQUIRED
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
admissionRequirements:
  failIfAdr0011C15NotQualified: true
  failIfProgramCNotQualified: true
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
| `stopGateStatus` | ADR0011 C15 and Program C statuses and exact evidence selectors. |
| `consolidatedResultId` | Digest over request, reused evidence, fresh evidence, policies, operation receipts, disclosures, terminal accounting, and result projection. |
| `inventoryDecision` | Reuse/reject/supplement decision for every candidate cache/inventory. |
| `freshnessDecision` | Exact compatibility/freshness outcome for source, workspace, revision, freeze, profile, policy, and privacy partition. |
| `operationReceipts` | Ordered deterministic receipts for each planned stage, including skipped stages. |
| `evidenceDisclosure` | Reused evidence, fresh evidence, omitted evidence, rejected cache entries, and unavailable evidence. |
| `candidateArtifacts` | Authority-zero candidate records only; accepted false; unresolved feature identity. |
| `representativeStatus` | Separately typed representative status independent of feature identity. |
| `matchReasons` | Per-candidate reason records with source stage, evidence selector, channel, and limits. |
| `custodyProvenance` | Custody chain, publication/default privacy status, source selectors, admission receipts, and producer identities. |
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
  adr0011C15: REQUIRED_NOT_ADMITTED
  programC: REQUIRED_NOT_QUALIFIED
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
    safeMessage: "ADR0011 C15 typed admission and Program C qualification are required before bridge execution."
replay:
  idempotent: true
  replayClass: DISABLED_NO_EFFECT
securityPrivacy:
  archiveClass: DESIGN_ONLY_NO_SOURCE_CAPTURE
```

## State machine

| State | Entry condition | Permitted transition | Forbidden transition |
| --- | --- | --- | --- |
| `DESIGN_ONLY_DISABLED` | This document exists; stop gates not both satisfied. | `STOP_GATES_SATISFIED` after separate accepted ADR0011 C15 and Program C evidence. | Any execution, schema registration, public surface, or runtime dispatch. |
| `STOP_GATES_SATISFIED` | Exact gate receipts are independently accepted. | `REQUEST_ADMITTED` if a future authorized implementation admits exact request bytes. | Host-only claim that gates are satisfied without lsp-trace verification. |
| `REQUEST_ADMITTED` | Conceptual request has exact identity, limits, privacy, workspace, revision, and freeze selectors. | `INVENTORY_ASSESSED`. | Operation execution before inventory/cache assessment. |
| `INVENTORY_ASSESSED` | Every candidate cache/inventory has compatibility and freshness decision. | `CAPTURE_PLANNED`. | Cache reuse without disclosure. |
| `CAPTURE_PLANNED` | Missing/stale evidence has bounded incremental plan. | `OPERATIONS_SEQUENCED`. | Unbounded capture or host-selected hidden operations. |
| `OPERATIONS_SEQUENCED` | Deterministic stage order and dependencies are frozen. | `OPERATIONS_EXECUTED` in a future authorized implementation. | Reordering by model output after admission. |
| `OPERATIONS_EXECUTED` | Every stage has terminal accounting. | `RESULT_CONSOLIDATED`. | Silent partial success. |
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
| `REJECT_UNQUALIFIED` | Required ADR0011/Program C gate is not qualified. |
| `REJECT_UNKNOWN_CUSTODY` | Custody or provenance is missing or unverifiable. |

Cache compatibility is exact by default. A future policy may define coarser compatibility only if separately versioned and qualified. Cache freshness is never inferred from existence, newest timestamp, source path, or host confidence.

## Source/workspace/revision/freeze compatibility

The bridge must compare:

- logical source URI and allowed roots;
- workspace identity and session generation, when applicable;
- commit/revision and cleanliness/freeze policy;
- source digest, byte length, encoding, and range selectors;
- ADR0007 stage freeze/profile/policy identities;
- ADR0011 admission family/version and occurrence/source identities;
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
7. admission family and profile;
8. privacy partition;
9. cancellation point;
10. how fresh evidence will be distinguished from reused evidence.

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
11. RESULT_CONSOLIDATION
12. DISCLOSURE_RENDERING
13. PRIVATE_ARCHIVE_OR_REPLAY_RECORD
```

Each stage either executes, reuses compatible evidence, is skipped by policy, or fails with one terminal disposition. A later stage may not infer success or absence from an earlier omitted or failed stage.

## Consolidated result identity

`consolidatedResultId` is conceptually computed from canonical bytes for:

- request identity and idempotency key;
- stop-gate receipts;
- inventory/cache decisions;
- all reused evidence selectors;
- all fresh evidence selectors;
- all skipped/omitted/failed stage records;
- stage policies, freezes, profiles, limits, and accounting;
- candidate artifact identities;
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
- host-owned inference role and lsp-trace non-inference role.

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

## Custody and provenance

Custody records must preserve:

- producer identity for each deterministic lsp-trace stage;
- host identity for external inference and orchestration suggestions;
- admission receipt identities;
- publication/default privacy status;
- source selector and revision provenance;
- policy/freeze/profile provenance;
- correction/supersession provenance;
- exact terminal disposition for every admitted item.

External host inference can explain, rank, or suggest within the host boundary. It cannot become lsp-trace custody for deterministic evidence, cannot admit occurrences, cannot qualify Program C, and cannot raise candidate authority.

## Fail-closed behavior

The bridge fails closed for:

- ADR0011 C15 not admitted;
- Program C not qualified;
- unknown contract version;
- unbounded limits;
- missing privacy policy;
- incompatible workspace/revision/freeze/profile/policy;
- dirty or moving workspace when clean/frozen revision is required;
- cache reuse without compatibility/freshness decision;
- missing custody/provenance;
- model/provider/harness claimed as lsp-trace-owned inference;
- partial terminal accounting;
- cancellation/deadline before a new stage;
- result identity mismatch;
- replay mismatch;
- attempted public publication by default.

Fail-closed results are still useful artifacts if they disclose safe diagnostics and consumed limits. They do not authorize retry with broader evidence, alternate model, alternate policy, or hidden fallback.

## Replay and idempotence

A request with the same canonical bytes and idempotency key must either return the same disabled/no-effect result, the same replay result over retained immutable inputs, or a typed replay failure. Replay may not use ambient checkout state, current filesystem contents, current host memory, current model availability, current cache contents, or network access unless those identities were part of the admitted immutable inputs.

Idempotence covers terminal accounting and publication effects. It does not imply concurrency safety beyond the separately qualified publication/admission contract.

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

1. before admitting a new stage;
2. before external host inference handoff;
3. before publication;
4. before archive/export.

If cancellation or deadline fires, the bridge records completed stage receipts, marks not-started stages as omitted due to cancellation/deadline, and publishes no partial candidate as complete. Cancellation cannot be converted into success by the host.

## Security, privacy, and archive boundaries

- The bridge does not transmit source to an external provider by itself.
- The host owns any source disclosure decision to its model/provider/harness.
- lsp-trace results default to private local immutable publication, not Git.
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
- select its own model/provider/harness;
- hold transcripts and prompts under its own policy;
- display lsp-trace evidence and limitations to the user.

The host may not, within this bridge:

- cause lsp-trace to invoke a model in qualification paths;
- bypass typed admission;
- substitute model judgment for Program C qualification;
- hide reused/fresh evidence distinction;
- hide omissions, failures, or limits;
- accept features;
- claim completeness, ownership, authority, or correctness.

## Open questions left explicit

1. What exact ADR0011 C15 artifact names and typed admission families will the bridge consume?
2. What exact Program C qualification receipt, policy, seed, and numeric contract will qualify the Group stage for this bridge?
3. Which freshness policy variants are acceptable for common workflows?
4. Which private-only publication store and retention class should hold disabled/no-effect request records?
5. What sanitized public receipt format, if any, should summarize a completed host-LLM bridge run?
6. Which host skill/prompt/resource should teach the workflow without becoming authority?

## Verdict

`ADR0007_HOST_LLM_BRIDGE_DESIGN_READY` for documentation-only review.

`EXECUTION_DISABLED_PENDING_ADR0011_C15_AND_PROGRAM_C` for any runtime, public surface, schema registration, code generation, or model execution.
