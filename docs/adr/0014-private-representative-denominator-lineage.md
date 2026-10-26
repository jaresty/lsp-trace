# ADR 0014: Retain a complete representative nomination denominator and community lineage

- **Status:** Accepted additive private implementation and qualification contract
- **Date:** 2026-10-25
- **Accepted:** 2026-10-25
- **Decision authority:** @jaresty (GitHub handle)
- **Decision owners:** LSP Trace maintainers
- **Accepted proposal SHA-256:** `85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda`
- **Independent review:** `ACCEPT` after all blocking findings were corrected; task `5957dec3-f1c9-419`
- **Accepted receipt-discovery amendment SHA-256:** `df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57`
- **Receipt-amendment independent review:** `ACCEPT`; task `ca520100-8448-475`
- **Accepted prequalification-mechanics amendment SHA-256:** `b0af4add81ab71ca34d91873d8faa728c58166f48ff9b5d46c7ae9c4eb385f47`
- **Mechanics-amendment independent review:** `ACCEPT`; task `bfcf46b9-abe4-431`
- **Accepted qualification-subject amendment SHA-256:** `235facbbba34f479df4771be845f34b3a39c83988a339dca69eec4f2b6bad9a6`
- **Qualification-contract independent review:** `ACCEPT`; task `00147418-3d69-4bf`
- **Accepted multi-seed v2 mechanics/preflight amendment SHA-256:** `839c71b4d74c0e3fa2fb6c6b49be85725b615754586bf40ab116f9a709f7d66e`
- **Multi-seed amendment independent review:** `ACCEPT`; tasks `15f9adca-d59e-436`, `800da54d-a2a2-4a1`
- **Multi-seed amendment accepted:** 2026-10-26 by @jaresty for private automated implementation and qualification only
- **Depends on:** [ADR 0011](0011-versioned-definition-reference-grouping-evidence.md), [ADR 0012](0012-private-census-to-candidate-orchestration-bridge.md), and [ADR 0013](0013-resource-blocked-program-c-instability.md)
- **Extends without modifying:** historical Program C receipts, community-instability v1/v2/v3, Stage 6 representative mechanics, and the Describe-preparation gap-ledger v1

## Context

The qualified private workflow now advances through all 395 frozen Program C constituents and reaches `WAITING_EVIDENCE`. The retained gap ledger truthfully reports that representative nomination and candidate preparation are `NOT_COMPUTED` and that these evidence requirements are `NOT_RETAINED`:

- representative nomination denominator;
- community-to-representative lineage;
- complete retained source-byte closure;
- `FULL_DEFINITION` display bindings;
- caller-occurrence V3 custody.

Existing Stage 6 mechanics can deterministically select a representative or a typed non-selection for each admitted community and selection context. They do not resolve the frozen Program C head, reconstruct exact communities from retained source custody, define the complete expected nomination population, or retain lineage into the inventory successor.

Program C transition receipts retain result bytes, digests, lengths, ordinals, and predecessor bindings. Complete instability artifacts bind community artifact hashes but do not themselves retain all community bytes, CALLS occurrences, or Stage 6 selection contexts. Direct caller reconstruction, checkout reads, language-server reacquisition, historical fixture reuse, or community omission would violate the accepted custody boundary.

## Decision required

Add a private, immutable bridge that derives and retains two coupled artifacts from verified frozen Program C custody:

1. a complete representative nomination denominator;
2. a total community-to-representative lineage bound to that denominator.

No implementation begins merely because this proposal exists. Implementation requires independent review, explicit acceptance by the decision authority, and a frozen proposal digest.

## Scope and authority

This contract authorizes only private implementation and qualification of denominator and lineage custody. It does not authorize:

- Describe packet construction;
- candidate preparation;
- feature identity, purpose, ownership, value, or completeness claims;
- stakeholder correction or semantic acceptance;
- public CLI or MCP registration;
- production admission, release, or deployment.

Every artifact remains:

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
reviews=NOT_REQUESTED
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
source_graph_complete=UNKNOWN
absence_inference=PROHIBITED
```

## Exact predecessor custody

Derivation begins only from:

- the verified terminal Program C head;
- its exact ordered transition plan;
- every immutable Program C receipt in committed-prefix order;
- the exact frozen Census receipt and availability ledger;
- exact retained Native V5 constituent bytes resolved through the already verified publication resolver;
- the accepted Program C bindings, source pin, policy identities, limits, seeds, and replay count.

The bridge may deterministically recompute Program C from exact retained Native V5 bytes solely to materialize a new immutable carrier. This is not checkout fallback or language-server reacquisition. Each recomputed result must match the retained receipt and instability evidence before it can contribute positively.

### Complete receipt inventory

The terminal Program C head names only its latest receipt. Before denominator derivation, a repository-owned read-only verifier must establish every receipt in plan-ordinal order. It must either:

1. deterministically reconstruct every claim, head, and receipt selector from the initial head, exact plan entry, bindings, recomputed result bytes, and predecessor selector; or
2. verify a separately retained content-addressed complete receipt manifest linked from verified terminal custody.

For historical v2 instability result bytes, the receipt schema is not encoded in the plan, bindings, result bytes, or predecessor selector. The verifier therefore performs bounded repository-owned schema probing at each ordinal:

1. derive the complete canonical v3 receipt and selector;
2. derive the complete canonical v4 receipt and selector;
3. load both derived selectors without listing the store;
4. admit exactly one candidate only when its retained bytes exactly equal the derived canonical bytes;
5. reject zero matches as missing and two matches as ambiguous/duplicate.

For result bytes that require v4, only the canonical v4 candidate is admissible; a v3 representation is invalid rather than an alternative. No other schema candidate may be probed. This rule is deterministic and accepts no caller-supplied schema or selector.

The verifier requires exact ordinal coverage, canonical receipt bytes, exact result bytes/digest/length, claim-predecessor equality, and a reconstructed final head byte-identical to the independently verified terminal head. Caller-supplied receipt selectors are prohibited. A missing, foreign, duplicate, unreachable, or unverifiable receipt in the reconstructed chain contributes to `I`; it cannot be omitted. Unrelated or CAS-orphaned objects outside the reconstructed selector set neither prove nor disprove chain completeness and need not be enumerated.

The bridge must not:

- read ambient checkout source;
- invoke or reacquire an LSP session;
- accept caller-supplied ordinals, selectors, processed sets, limits, seeds, retry policy, skip policy, stage state, or outcomes;
- substitute historical Stage 6 fixtures, candidates, groups, Describe packets, or semantic results;
- reinterpret a resource-incomplete constituent as complete or empty.

## Denominator unit

The nomination unit is one exact **Program C run/community selection context**:

```text
constituent ordinal
× canonical run identity
× canonical community ordinal and membership
× exact Stage 6 selection context
```

Every eligible run/community contributes exactly one denominator entry. Multiple seeds and replays remain distinct. Byte-identical community memberships from different run identities remain distinct entries and cannot be collapsed.

A denominator entry binds:

- workflow identity and terminal Program C head;
- transition plan and receipt selector;
- constituent ordinal and immutable constituent selector;
- Program C outcome and receipt disposition;
- Native V5 selector, digest, and byte length;
- instability schema, selector, digest, and byte length;
- run ID, seed, replay ordinal, and run status;
- community artifact digest and exact canonical bytes;
- community ordinal and canonical ordered members;
- exact admitted CALLS occurrence identities and relation identities;
- source predecessor and custody identities;
- Stage 6 selection-context identity, execution-bundle identity, prepared targets, reached nodes, and exact relation-occurrence closure;
- Stage 6 policy, implementation, qualification, limits, and claim-ceiling identities.

## Deterministic Native V5 to Stage 6 adapter

For each eligible run/community, a repository-owned adapter creates one singleton `RepresentativeComposition`, one singleton `RepresentativeGrouping`, and exactly one `RepresentativeSelectionContext`:

- each `(edge, CallSites[ordinal])` pair becomes exactly one `RepresentativeEvidence` record; a zero-call-site edge is rejected;
- evidence identity is a canonical digest over constituent, execution bundle, relation ID, occurrence ordinal, endpoints, and exact call-site range;
- relation identity is the Native V5 `relation_id`; `From`/`To` are `caller_node_id`/`callee_node_id`; ordinal is canonical edge/call-site order; call site is copied exactly;
- predecessor and custody bind the Native V5 selector, complete receipt selector, execution bundle, and source publication; disposition is `INCLUDED` only for admitted `CALL_RELATION` evidence under `CALLS_ONLY_FROZEN_CENSUS_V1`;
- the recomputed community artifact supplies a singleton partition containing its exact sorted members;
- Native V5 seed memberships for the exact execution bundle, seed label, and seed position map `PREPARED_TARGET` endpoint IDs to `PreparedTargets`, `REACHED_NODE` endpoint IDs to `ReachedNodes`, and `CALL_RELATION` endpoint IDs to `RelationIDs`;
- `OccurrenceIDs` is the complete closure of every admitted call-site occurrence for those relation IDs;
- composition/grouping input, policy, qualification, limits, accounting, digest, and byte-length fields are derived from canonical predecessor and adapter bytes, never caller text;
- `ModelJudgment` and `ModelAdvice` are empty and cannot be caller-authored;
- context identity is a canonical digest over the exact constituent, run, community, seed-membership closure, and adapter contract.

Unknown membership kinds, duplicate membership/evidence/context identities, missing relations, partial occurrence closure, noncanonical ordering, mixed execution bundles, or mismatched seed labels/positions reject derivation. Exactly one derived context must exist for each denominator entry. Zero or multiple candidate contexts fail closed; arbitrary seed selection, context collapse, and context splitting are prohibited absent a separately accepted contract.

### Prequalification Stage 6 mechanics amendment

Before qualification-envelope publication, implementation may derive only an immutable prequalification mechanics carrier. It must not construct a final `RepresentativeComposition`, `RepresentativeGrouping`, qualification token, representative result, denominator artifact, or lineage artifact. Final sealing remains blocked on the accepted qualification sequence.

The mechanics carrier schema is `lsp-trace.program-c-stage6-mechanics.private.v1` under profile `CALLS_ONLY_FROZEN_CENSUS_V1`. It contains defensive copies of exact evidence, one singleton community partition, one derived selection context, predecessor bindings, and adapter-contract identity. It has authority `0`, acceptance `false`, and no semantic claims.

A narrow repository-owned Native V5 representative view belongs in `internal/graph`. The concrete type is opaque and zero-invalid: storage fields are unexported, construction requires strict Native V5 validation, and accessors defensively copy canonical edges, call sites, invocation seeds, and exact seed memberships. `internal/programcrepresentative` owns construction and validation of immutable singleton mechanics. `internal/censuscontinuation` alone adapts its private verified receipt-inventory entries; it exports neither that inventory nor a custody token.

Every outcome entering adaptation is deep-cloned with `programc.CloneOutcome`; copying only the outcome slice is insufficient. Standalone mechanics bytes confer no custody or producer authentication. Final qualification must reopen and verify the private receipt inventory, recompute mechanics, and require canonical byte identity; it cannot trust caller-supplied mechanics or predecessor fields.

Freeze these domain separators:

```text
lsp-trace:program-c-stage6-adapter-contract:v1
lsp-trace:program-c-stage6-evidence:v1
lsp-trace:program-c-stage6-context:v1
lsp-trace:program-c-stage6-custody:v1
lsp-trace:program-c-stage6-mechanics:v1
```

Every identity uses:

```text
sha256(domain UTF-8 bytes || 0x00 || canonical JSON DTO bytes)
```

Canonical JSON uses lower-snake-case field names in the declared order, UTF-8 strings, base-10 JSON integers, booleans, and arrays that are always present (`[]`, never `null`). Unknown fields are rejected. Bytes are exactly Go `encoding/json.Marshal` output: compact JSON, HTML escaping enabled, no trailing newline, and no Unicode normalization. Digest fields use lowercase `sha256:<64 hex>`.

Only the root DTO's `identity` is blank while computing that DTO's hash. Nested evidence and context identities are already populated and independently verified before embedding. Fixed contract arrays retain the stated order; memberships sort strictly by `membership_id`; evidence retains canonical edge order then call-site ordinal; community members, prepared targets, reached nodes, relation IDs, and occurrence IDs sort strict lexicographic unique order.

The exact schema constants are:

```text
AdapterContractDTO.schema_version  = "lsp-trace.program-c-stage6-adapter-contract.private.v1"
EvidenceIdentityDTO.schema_version = "lsp-trace.program-c-stage6-evidence.private.v1"
CustodyIdentityDTO.schema_version  = "lsp-trace.program-c-stage6-custody.private.v1"
SelectionContextIdentityDTO.schema_version = "lsp-trace.program-c-stage6-context.private.v1"
MechanicsIdentityDTO.schema_version = "lsp-trace.program-c-stage6-mechanics.private.v1"
profile = "CALLS_ONLY_FROZEN_CENSUS_V1"
native_schema_version = "lsp-trace.graph.v5"
program_c_profile_id = "calls-v1"
program_c_profile_sha256 = "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9"
program_c_algorithm = "gonum.org/v1/gonum/graph/community.Leiden"
```

Program C algorithm version is not a free constant: it is copied from and bound to the verified transition bindings' nonempty `algorithm_version`, and every outcome's algorithm must equal `program_c_algorithm`. `native_v5_sha256` and `native_v5_byte_length` cover the exact retained constituent bytes returned by verified `ResolveConstituent`, before decoding or projection.

Canonical mechanics wire bytes are exactly the populated `MechanicsIdentityDTO` marshaled under these rules. There are no additional unhashed fields or trailing bytes.

The exact preimages are:

```text
PositionDTO {
  line uint32
  character uint32
}

RangeDTO {
  start PositionDTO
  end PositionDTO
}

AdapterContractDTO {
  schema_version string
  profile string
  domains []string
  native_schema_version string
  program_c_profile_id string
  program_c_profile_sha256 string
  program_c_algorithm string
  program_c_algorithm_version string
  run_seeds []uint64
  replays_per_seed uint8
  run_id_grammar string
  occurrence_ordinal_rule string
  membership_kinds []string
  mapped_membership_kinds []string
  community_order_rule string
  authority uint8
  accepted bool
  identity string
}

EvidenceIdentityDTO {
  schema_version string
  constituent_selector string
  execution_bundle_id string
  relation_id string
  call_site_ordinal int
  from_node_id string
  to_node_id string
  call_site RangeDTO
  receipt_selector string
  identity string
}

CustodyIdentityDTO {
  schema_version string
  workflow_key string
  terminal_program_c_head_selector string
  receipt_selector string
  receipt_ordinal int
  constituent_ordinal int
  publication_selector string
  constituent_selector string
  native_v5_sha256 string
  native_v5_byte_length uint64
  execution_bundle_id string
  identity string
}

MembershipIdentityDTO {
  membership_id string
  execution_bundle_id string
  seed_label string
  seed_at string
  evidence_kind string
  endpoint_id string
}

SelectionContextIdentityDTO {
  schema_version string
  adapter_contract_sha256 string
  custody_identity string
  constituent_selector string
  execution_bundle_id string
  run_id string
  seed uint64
  replay_ordinal uint8
  community_ordinal int
  community_artifact_sha256 string
  community_members []string
  memberships []MembershipIdentityDTO
  prepared_targets []string
  reached_nodes []string
  relation_ids []string
  occurrence_ids []string
  identity string
}

MechanicsIdentityDTO {
  schema_version string
  profile string
  adapter_contract_sha256 string
  custody_identity string
  receipt_selector string
  run_id string
  seed uint64
  replay_ordinal uint8
  community_ordinal int
  community_artifact_sha256 string
  evidence []EvidenceIdentityDTO
  community_members []string
  selection_context SelectionContextIdentityDTO
  authority uint8
  accepted bool
  identity string
}
```

`domains` is the five-domain list above in that exact order. `run_seeds=[1,2]`, `replays_per_seed=3`, `run_id_grammar="seed-%020d-replay-%d"`, `occurrence_ordinal_rule="ZERO_BASED_WITHIN_CANONICAL_EDGE_CALL_SITES"`, `membership_kinds` is the five-kind list below, `mapped_membership_kinds` is its first three entries, and `community_order_rule="PROGRAM_C_VALID_CANONICAL_COMMUNITIES"`. The adapter-contract identity uses the adapter-contract domain; evidence, context, custody, and mechanics identities use their correspondingly named domains. Every `int` ordinal must be nonnegative and within the length of its owning canonical array; `receipt_ordinal` must be positive. Composition identity, grouping identity, production policy bytes, limits bytes, qualification IDs, and final family/version constants are deliberately deferred to the qualification subject.

For each canonical Native V5 edge, call-site ordinal is zero-based within that edge's canonical `CallSites` array. One `(edge, CallSites[ordinal])` pair produces one occurrence. A zero-call-site edge, noncanonical edge order, noncanonical call-site order, or duplicate call site fails closed.

All canonical Native V5 seed-membership kinds are validated, endpoint-checked, and retained in the context-identity closure in this exact order:

```text
PREPARED_TARGET
REACHED_NODE
CALL_RELATION
SIBLING_CANDIDATE
DISPATCH_ASSOCIATION
```

Only the first three map into `PreparedTargets`, `ReachedNodes`, and `RelationIDs`. `SIBLING_CANDIDATE` and `DISPATCH_ASSOCIATION` are bound but do not participate in CALLS-only selection and cannot support absence or semantic inference. Any other membership kind fails closed. A new constructor must not reuse a decoder that rejects canonical dispatch membership.

Membership groups must be a bijection with canonical `invocation.seeds`, keyed by `(execution_bundle_id, seed_label, seed_at)`. This unchanged denominator contract admits exactly one group per eligible constituent. Zero or multiple groups classify that constituent and the denominator `INCOMPLETE`; they are never empty or silently excluded. Frozen-corpus qualification must prove exactly one group for every eligible `COMPLETE` constituent.

The receipt inventory supplies exactly six outcomes in preserved order:

```text
seed 1 replay 1
seed 1 replay 2
seed 1 replay 3
seed 2 replay 1
seed 2 replay 2
seed 2 replay 3
```

No sorting or deduplication is permitted. Seeds must match that sequence. Replay ordinal is derived from position within each seed, and run ID is exactly `seed-%020d-replay-%d`.

For every eligible `COMPLETE` outcome, communities and members must satisfy Program C `validCanonicalCommunities` ordering. The adapter rebuilds each canonical community artifact with `BuildCommunityArtifact`. After strict v2/v3 result validation, it reconstructs all 15 expected pair identities. Each run occurs in five pairs; every incident left/right community-artifact digest for that run must be present, agree with the other four, and equal SHA-256 of the rebuilt artifact bytes. Missing, conflicting, foreign, or unmatched pair/digest evidence fails closed. The verified digest is bound into context and mechanics identities. No exported community-artifact decoder is required.

## Qualification-subject wire amendment

Before any review receipt or final qualification envelope, add one pure canonical value owned by `internal/programcrepresentative`:

```text
ProgramCRepresentativeQualificationSubject
```

This slice is storage-agnostic. It imports no Census coordinator, inventory-successor, publication, immutable-store, head-store, or CAS package. It cannot name a review receipt, qualification envelope, denominator, lineage, successor, or downstream head.

### Root wire and identity

The exact schema is `lsp-trace.program-c-representative-qualification-subject.private.v1`, profile is `CALLS_ONLY_FROZEN_CENSUS_V1`, and identity domain is `lsp-trace:program-c-representative-qualification-subject:v1`.

Identity is:

```text
sha256(domain UTF-8 bytes || 0x00 || Go encoding/json.Marshal(root with only root identity=""))
```

Canonical JSON follows the mechanics amendment's exact compact Go `encoding/json.Marshal` semantics. All arrays are present and never `null`; unknown fields, trailing bytes, duplicate keys, noncanonical order, and self-consistent substitution against independently supplied expected context fail closed. The canonical subject bytes are exactly the populated root DTO with no trailing newline.

Field order and nested DTOs are:

```text
QualificationSubjectDTO {
  schema_version string
  profile string
  accepted_adr_sha256 string
  accepted_receipt_discovery_amendment_sha256 string
  accepted_prequalification_mechanics_amendment_sha256 string
  accepted_qualification_subject_amendment_sha256 string
  adapter_contract AdapterContractDTO
  mechanics_schema_version string
  mechanics_profile string
  stage6_implementation Stage6ImplementationDTO
  stage6_policy ArtifactBindingDTO
  stage6_limits ArtifactBindingDTO
  stage6_finalization ArtifactBindingDTO
  corpus_identity CorpusIdentityDTO
  workflow_identity QualificationWorkflowIdentityDTO
  source_pin_sha256 string
  receipt_inventory []ReceiptInventoryEntryDTO
  terminal_program_c_head TerminalProgramCHeadDTO
  claim_ceiling QualificationClaimCeilingDTO
  authority uint8
  accepted bool
  identity string
}

Stage6ImplementationDTO {
  repository_commit string
  repository_tree string
  files []ImplementationFileDTO
  identity string
}

ImplementationFileDTO {
  path string
  git_blob_sha1 string
  sha256 string
  byte_length uint64
}

ArtifactBindingDTO {
  schema_version string
  identity string
  sha256 string
  byte_length uint64
}

For every `ArtifactBindingDTO`, `sha256` is also the immutable content-addressed selector:

```text
selector = sha256 = SHA-256(exact canonical populated artifact bytes)
byte_length = len(exact canonical populated artifact bytes)
```

The domain-separated artifact `identity` is distinct from that selector. Resolution loads by `sha256`, requires exact length and byte digest, strictly parses the declared schema, reconstructs the domain identity, and requires equality.

CorpusIdentityDTO {
  frozen_corpus_sha256 string
  census_receipt_sha256 string
  availability_ledger_sha256 string
  program_c_plan_sha256 string
  program_c_bindings_sha256 string
}

QualificationWorkflowIdentityDTO {
  profile string
  contract string
  census_receipt_sha256 string
  availability_ledger_sha256 string
  source_pin_sha256 string
  program_c_plan_sha256 string
  program_c_bindings_sha256 string
  privacy_policy_sha256 string
  retention_policy_sha256 string
  resource_policy_sha256 string
  replay_policy_sha256 string
  qualification_policy_sha256 string
  provenance string
}

ReceiptInventoryEntryDTO {
  plan_ordinal int
  receipt_ordinal int
  receipt_schema_version string
  receipt_selector string
  receipt_sha256 string
  receipt_byte_length uint64
}

TerminalProgramCHeadDTO {
  schema_version string
  workflow_key string
  head_key string
  generation uint64
  selector string
  head_bytes_sha256 string
  byte_length uint64
  committed_prefix int
  constituent_count int
}

QualificationClaimCeilingDTO {
  authority uint8
  accepted bool
  feature_identity string
  completeness string
  effects string
  reviews string
  implementation_decision string
  production_admission string
  source_graph_complete string
  absence_inference string
}
```

All SHA-256 wire values use `sha256:<64 lowercase hex>`, including accepted proposal digests.

### Accepted decision bindings

The subject binds:

```text
accepted_adr_sha256 = sha256:85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda
accepted_receipt_discovery_amendment_sha256 = sha256:df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57
accepted_prequalification_mechanics_amendment_sha256 = sha256:b0af4add81ab71ca34d91873d8faa728c58166f48ff9b5d46c7ae9c4eb385f47
accepted_qualification_subject_amendment_sha256 = the exact reviewed proposal SHA-256 recorded by this ADR's accepted qualification-subject amendment metadata
mechanics_schema_version = lsp-trace.program-c-stage6-mechanics.private.v1
mechanics_profile = CALLS_ONLY_FROZEN_CENSUS_V1
```

`adapter_contract` is the complete canonical `AdapterContractDTO` reconstructed from verified Program C bindings, not merely a digest.

### Stage 6 implementation manifest

The selected reviewed Stage 6 seam manifest is pinned to repository commit `b33cf8cdbda56eefd54c049832129eb67a056727` and tree `f87ac81bfd429d196044b0283574d3de370dc1c3`. The commit/tree seal only the repository source snapshot; the explicit file list identifies reviewed seams and is not a behavioral or compiler closure.

Files are strict lexicographic path order:

```text
internal/censuscontinuation/programc_stage6_adapter_private.go
internal/censuscontinuation/programc_transition.go
internal/graph/native_v5_representative.go
internal/programc/core.go
internal/programcrepresentative/mechanics.go
internal/programcrepresentative/representative.go
internal/programcrepresentative/stage6_result.go
```

This is a selected reviewed seam manifest, not a behavioral, transitive-dependency, compiler, module-resolution, platform, or runtime closure. The pinned repository commit and tree seal only the repository source snapshot. Each entry's Git blob SHA-1, SHA-256, and exact byte length are calculated over the exact raw Git blob content at that commit. Implementation identity uses domain `lsp-trace:program-c-representative-stage6-implementation:v1` and the same domain-zero-canonical-JSON rule with only `Stage6ImplementationDTO.identity` blank.

### Fixed policy and limits

The canonical policy schema is `lsp-trace.program-c-representative-stage6-policy.private.v1`, identity domain is `lsp-trace:program-c-representative-stage6-policy:v1`, and canonical fields are:

```text
Stage6PolicyDTO {
  schema_version string
  profile string
  admitted_kind string
  admitted_disposition string
  self_loop_disposition string
  context_node_scope string
  relation_occurrence_closure string
  graph_direction string
  strongly_connected_components string
  prepared_root_distance string
  candidate_scope string
  prepared_root_scope string
  selected_scc_members_scope string
  predecessor_scope string
  no_calls_outcome string
  selection_failure_outcome string
  selected_outcome string
  selection_order []string
  predecessor_order []string
  singleton_result_required bool
  authority uint8
  accepted bool
  identity string
}
```

Fixed values are:

```text
schema_version=lsp-trace.program-c-representative-stage6-policy.private.v1
profile=CALLS_ONLY_FROZEN_CENSUS_V1
admitted_kind=CALLS
admitted_disposition=INCLUDED
self_loop_disposition=RETAINED_INCLUDED_DERIVED_INELIGIBLE
context_node_scope=UNION_PREPARED_TARGET_AND_REACHED_NODE
relation_occurrence_closure=EXACT_ALL_CALLS_OCCURRENCES_PER_CONTEXT_RELATION
graph_direction=CALLER_TO_CALLEE
strongly_connected_components=DETERMINISTIC_TARJAN_CANONICAL_NODE_ORDER
prepared_root_distance=SHORTEST_DIRECTED_CONDENSATION_EDGE_COUNT
candidate_scope=REACHED_COMMUNITY_MEMBER_REACHABLE_FROM_PREPARED_ROOT_SCC
prepared_root_scope=ALL_SCCS_CONTAINING_PREPARED_TARGETS
selected_scc_members_scope=ALL_ACTIVE_CONTEXT_NODES_IN_SELECTED_SCC
predecessor_scope=INCLUDED_NON_SELF_CALLS_DIRECTLY_INCOMING_TO_SELECTED_NODE
no_calls_outcome=NO_CALLS_REPRESENTATIVE
selection_failure_outcome=CALLS_EVIDENCE_INELIGIBLE
selected_outcome=SELECTED
selection_order=[MIN_DISTANCE,LEXICOGRAPHIC_NODE_ID]
predecessor_order=[RELATION_ID,OCCURRENCE_ID]
singleton_result_required=true
authority=0
accepted=false
```

The policy identity is:

```text
sha256("lsp-trace:program-c-representative-stage6-policy:v1" || 0x00 || canonical Stage6PolicyDTO with only Stage6PolicyDTO.identity="")
```

The canonical limits schema is `lsp-trace.program-c-representative-stage6-limits.private.v1`, identity domain is `lsp-trace:program-c-representative-stage6-limits:v1`, and exact DTO field order is:

```text
Stage6LimitsDTO {
  schema_version string
  profile string
  max_communities int
  max_members int
  max_occurrences int
  max_candidates int
  max_canonical_bytes int
  max_string_bytes int
  max_work int
  authority uint8
  accepted bool
  identity string
}
```

Fixed values are:

```text
schema_version=lsp-trace.program-c-representative-stage6-limits.private.v1
profile=CALLS_ONLY_FROZEN_CENSUS_V1
max_communities=10000
max_members=10000
max_occurrences=100000
max_candidates=10000
max_canonical_bytes=67108864
max_string_bytes=1048576
max_work=110000
authority=0
accepted=false
```

The limits identity is:

```text
sha256("lsp-trace:program-c-representative-stage6-limits:v1" || 0x00 || canonical Stage6LimitsDTO with only Stage6LimitsDTO.identity="")
```

`max_candidates` applies per community; `max_members` applies to total partition membership. `max_work=110000` equals `max_occurrences + max_members` under the existing Stage 6 work equation.

These are corpus-specific qualification bounds, not a claim that every upstream-admissible Native V5 input is admitted. Before any qualification subject may be issued, a complete frozen-corpus preflight must compute the maximum observed communities, total members, members per community, occurrences, string bytes, and work for every eligible singleton. It does not construct a `RepresentativeResult` or require a qualification token. Every observed value must be within its corresponding non-result bound. ADR 0013 resource-excluded constituents are explicitly classified and contribute zero eligible singletons; they are not measured as successful Stage 6 inputs. Any missing, otherwise incomplete, or unmeasured eligible constituent fails the preflight. If any measured bound is exceeded, subject issuance is refused and the limits require a separately reviewed amendment.

`max_canonical_bytes` is a fixed post-qualification fail-closed cap, not a pre-subject admission proof. It is checked only when the production-qualified result is constructed. Exceeding any limit fails closed and never authorizes truncation.

Policy and limits are each canonical JSON artifacts. Their `ArtifactBindingDTO` binds schema version, populated identity, SHA-256 of complete canonical bytes, and exact byte length.

### Finalization artifact

The deterministic mapping below is itself a canonical artifact bound by the subject as `stage6_finalization`. Its schema is `lsp-trace.program-c-representative-stage6-finalization.private.v1`, profile is `CALLS_ONLY_FROZEN_CENSUS_V1`, identity domain is `lsp-trace:program-c-representative-stage6-finalization:v1`, and exact field order is:

```text
Stage6FinalizationDTO {
  schema_version string
  profile string
  composition_family string
  composition_version string
  grouping_family string
  grouping_version string
  grouping_algorithm string
  grouping_profile string
  grouping_numeric_contract string
  qualification_family string
  qualification_trust string
  composition_mappings []string
  evidence_mappings []string
  grouping_mappings []string
  context_mappings []string
  qualification_mappings []string
  finalization_order []string
  authority uint8
  accepted bool
  identity string
}
```

The fixed arrays are:

```text
composition_mappings=[INPUT_FROM_CUSTODY,POLICY_FROM_VERIFIED_POLICY,COMPOSITION_ID_FROM_MECHANICS,QUALIFICATION_BINDING_FROM_VERIFIED_ENVELOPE,OCCURRENCES_FROM_EXACT_EVIDENCE_MAPPING,ACCOUNTING_ALL_INCLUDED_NO_TRUNCATION,MODEL_FIELDS_EMPTY]
evidence_mappings=[IDENTITY_EXACT,RELATION_ID_EXACT,KIND_CALLS,FROM_NODE_ID_TO_FROM,TO_NODE_ID_TO_TO,DISPOSITION_INCLUDED,CONSTITUENT_SELECTOR_TO_SOURCE_PREDECESSOR,RECOMPUTED_CUSTODY_IDENTITY_TO_CUSTODY,CANONICAL_ARRAY_INDEX_TO_ORDINAL,CALL_SITE_EXACT]
grouping_mappings=[LIMITS_FROM_VERIFIED_LIMITS,COMPOSITION_DIGEST_AND_LENGTH_EXACT,SEED_FROM_MECHANICS,SINGLETON_PARTITION_FROM_COMMUNITY,SINGLETON_CONTEXT_FROM_EXACT_CONTEXT_MAPPING]
context_mappings=[IDENTITY_EXACT,CONSTITUENT_SELECTOR_TO_CONSTITUENT_IDENTITY,EXECUTION_BUNDLE_EXACT,SINGLE_MEMBERSHIP_GROUP_SEED_LABEL_AND_AT,RECOMPUTED_CUSTODY_ORDINAL,COMMUNITY_AND_CLOSURE_ARRAYS_EXACT]
qualification_mappings=[COMPOSITION_DIGEST_EXACT,GROUPING_DIGEST_TO_EVIDENCE_DIGEST,EPOCH_TO_VALID_FROM_AND_VALID_THROUGH,VERIFIER_RECONSTRUCTS_ALL_BINDINGS]
finalization_order=[POPULATE_AND_HASH_COMPOSITION,POPULATE_AND_HASH_GROUPING,DERIVE_QUALIFICATION]
```

`authority=0` and `accepted=false`. Identity is SHA-256 of its domain, zero byte, and canonical DTO with only its own identity blank. Its subject `ArtifactBindingDTO` binds populated identity, complete-byte SHA-256, and exact length.

### Corpus, workflow, and source pin

`CorpusIdentityDTO` is reconstructed from strictly verified Program C custody. The twelve predecessor workflow-identity fields are copied field-for-field from the strictly verified predecessor successor head into `QualificationWorkflowIdentityDTO`; the subject adds fixed `provenance=PREDECESSOR_RETAINED_CALLER_ASSERTED`. Current custody independently cross-binds only Census receipt, availability ledger, Program C plan, Program C bindings, and resource policy. The source-pin, privacy, retention, replay, and qualification-policy digests remain predecessor-retained caller assertions unless separately verified artifacts are supplied in a later contract. The subject binds them without promoting their provenance or claiming their content was verified.

`source_pin_sha256` equals the predecessor-retained `source_pin_sha256`. It does not claim a source-pin schema, selector, canonical bytes, byte length, complete source closure, or independent verification.

### Deterministic Stage 6 finalization

After the final qualification envelope is independently verified, each singleton mechanics value maps to the existing Stage 6 API with these exact constants:

```text
RepresentativeComposition.Family = PROGRAM_C_REPRESENTATIVE_COMPOSITION
RepresentativeComposition.Version = private.v1
RepresentativeGrouping.Family = PROGRAM_C_REPRESENTATIVE_GROUPING
RepresentativeGrouping.Version = private.v1
RepresentativeGrouping.Algorithm = gonum.org/v1/gonum/graph/community.Leiden
RepresentativeGrouping.Profile = CALLS_ONLY_FROZEN_CENSUS_V1
RepresentativeGrouping.NumericContract = EXACT_INTEGER_DIRECTED_GRAPH_V1
RepresentativeQualification.Family = PROGRAM_C_REPRESENTATIVE_QUALIFICATION
RepresentativeQualification.Trust = PRIVATE_INDEPENDENT_REVIEW_ENVELOPE_V1
```

Composition mapping is:

```text
InputID = mechanics custody_identity
InputDigest = custody native_v5_sha256
InputLength = checked int conversion of custody native_v5_byte_length
PolicyID = populated Stage6PolicyDTO.identity
PolicyDigest = sha256(canonical populated Stage6PolicyDTO bytes)
PolicyLength = len(canonical populated Stage6PolicyDTO bytes)
CompositionID = mechanics identity
QualificationID = verified final qualification envelope identity
QualificationDigest = sha256(exact canonical populated envelope bytes)
QualificationLength = checked int conversion of len(exact canonical populated envelope bytes)
Occurrences = mechanics evidence mapped by the exact evidence mapping below
Accounting = exact input/terminal/included occurrence counts with all other precomputed counters zero and Truncated=false
ModelJudgment = ""
ModelAdvice = ""
```

For mechanics evidence in canonical array order at index `i`, the exact `RepresentativeEvidence` mapping is:

```text
Identity = EvidenceIdentityDTO.Identity
RelationID = EvidenceIdentityDTO.RelationID
Kind = CALLS
From = EvidenceIdentityDTO.FromNodeID
To = EvidenceIdentityDTO.ToNodeID
Disposition = INCLUDED
SourcePredecessor = EvidenceIdentityDTO.ConstituentSelector
Custody = recomputed CustodyIdentityDTO.Identity
Ordinal = i
CallSite = exact EvidenceIdentityDTO.CallSite converted without coordinate changes
```

The flattened `RepresentativeEvidence.Ordinal` is distinct from the per-edge `EvidenceIdentityDTO.CallSiteOrdinal` used in evidence identity. Before mapping, recompute the complete `CustodyIdentityDTO` from independently verified workflow, terminal head, receipt, plan ordinal, publication/constituent selectors, exact retained Native V5 bytes, and execution bundle. Its populated identity must equal `MechanicsIdentityDTO.custody_identity`; that identity cannot be accepted from mechanics bytes alone.

The composition unsigned bytes are exactly `append(json.Marshal(composition with Digest=""), '\n')`. The composition digest is SHA-256 of those LF-terminated unsigned bytes using the existing Stage 6 digest spelling. After populating `Digest`, canonical populated composition bytes are exactly `append(json.Marshal(populated composition), '\n')`; `CompositionLength` uses this LF-terminated populated length.

Grouping mapping is:

```text
LimitsDigest = sha256(canonical populated Stage6LimitsDTO bytes)
LimitsLength = checked int conversion of len(canonical populated Stage6LimitsDTO bytes)
CompositionDigest = populated composition digest
CompositionLength = checked int conversion of LF-terminated canonical populated RepresentativeComposition bytes length
Seed = mechanics seed
Partition = [mechanics community_members]
SelectionContexts = [mechanics selection_context mapped by the exact context mapping below]
```

The exact `RepresentativeSelectionContext` mapping is:

```text
Identity = MechanicsIdentityDTO.SelectionContext.Identity
ConstituentIdentity = recomputed CustodyIdentityDTO.ConstituentSelector
ExecutionBundleID = MechanicsIdentityDTO.SelectionContext.ExecutionBundleID
SeedLabel = the single verified membership group's SeedLabel
SeedAt = the single verified membership group's SeedAt
ConstituentOrdinal = recomputed CustodyIdentityDTO.ConstituentOrdinal
CommunityMembers = exact MechanicsIdentityDTO.SelectionContext.CommunityMembers
PreparedTargets = exact MechanicsIdentityDTO.SelectionContext.PreparedTargets
ReachedNodes = exact MechanicsIdentityDTO.SelectionContext.ReachedNodes
RelationIDs = exact MechanicsIdentityDTO.SelectionContext.RelationIDs
OccurrenceIDs = exact MechanicsIdentityDTO.SelectionContext.OccurrenceIDs
```

All arrays remain canonical defensive copies. Every `uint64` custody or artifact length converted to a Stage 6 `int` must equal the resulting nonnegative `int` when converted back to `uint64`; overflow, truncation, or mismatch fails closed. Every membership must share the single derived seed label/location; zero or multiple groups fail closed.

The grouping unsigned bytes are exactly `append(json.Marshal(grouping with Digest=""), '\n')`. The grouping digest is SHA-256 of those LF-terminated unsigned bytes. Canonical populated grouping bytes are exactly `append(json.Marshal(populated grouping), '\n')`.

### Independent-review receipt wire

The exact review schema is `lsp-trace.program-c-representative-qualification-review.private.v1`, profile is `CALLS_ONLY_FROZEN_CENSUS_V1`, identity domain is `lsp-trace:program-c-representative-qualification-review:v1`, and field order is:

```text
ReviewActorIdentityDTO {
  identity_kind string
  harness_actor_id string
  attribution_evidence ArtifactBindingDTO
  identity string
}

ReviewAssignmentRecordDTO {
  schema_version string
  actor_kind string
  harness_actor_id string
  parent_actor_id string
  subject ArtifactBindingDTO
  scope []string
}

ReviewTerminalRecordDTO {
  schema_version string
  actor_kind string
  harness_actor_id string
  parent_actor_id string
  subject ArtifactBindingDTO
  scope []string
  verdict string
}

ReviewTranscriptSnapshotDTO {
  schema_version string
  actor_kind string
  harness_actor_id string
  parent_actor_id string
  raw_transcript_selector string
  raw_transcript_sha256 string
  raw_transcript_byte_length uint64
  assignment_subject ArtifactBindingDTO
  assignment_scope []string
  terminal_subject ArtifactBindingDTO
  terminal_scope []string
  terminal_verdict string
  authority uint8
  accepted bool
  identity string
}

ReviewTaskSeparationDTO {
  schema_version string
  writer_session_id string
  reviewer_task_id string
  reviewer_parent_session_id string
  reviewer_isolated bool
  subject ArtifactBindingDTO
  review_scope []string
  authority uint8
  accepted bool
  identity string
}

ProgramCRepresentativeQualificationReviewDTO {
  schema_version string
  profile string
  subject ArtifactBindingDTO
  review_scope []string
  verdict string
  writer ReviewActorIdentityDTO
  reviewer ReviewActorIdentityDTO
  separation_method string
  separation_evidence ArtifactBindingDTO
  authority uint8
  accepted bool
  identity string
}
```

Assignment-record schema is `lsp-trace.program-c-representative-review-assignment.private.v1`; terminal-record schema is `lsp-trace.program-c-representative-review-terminal.private.v1`. Transcript snapshot schema is `lsp-trace.program-c-representative-review-transcript-snapshot.private.v1`, domain `lsp-trace:program-c-representative-review-transcript-snapshot:v1`. Task-separation schema is `lsp-trace.program-c-representative-review-task-separation.private.v1`, domain `lsp-trace:program-c-representative-review-task-separation:v1`. Actor identity domain is `lsp-trace:program-c-representative-review-actor:v1`. Each identity uses its domain, zero byte, and canonical DTO with only that DTO's identity blank.

The writer actor and snapshot use `identity_kind/actor_kind=PI_SESSION`, exact writer session ID, and `parent_actor_id=""`. The reviewer actor and snapshot use `identity_kind/actor_kind=PI_ISOLATED_TASK`, exact reviewer task ID, and `parent_actor_id` equal to the writer session ID that launched the isolated review. `harness_actor_id` is derived only from corresponding harness-issued session/task metadata. Actor identity hashes the populated kind, harness ID, and attribution-evidence binding.

The retained review-evidence JSONL is an exact two-record projection, not the full harness transcript:

```text
json.Marshal(ReviewAssignmentRecordDTO) || LF ||
json.Marshal(ReviewTerminalRecordDTO) || LF
```

There are exactly two nonempty UTF-8 lines and a final LF. Blank lines, additional lines, unknown fields, trailing bytes, duplicate keys, noncanonical JSON, missing records, or duplicate assignment/terminal records fail closed. Line 1 must strictly decode as the assignment schema; line 2 as the terminal schema. Both records repeat the snapshot's exact actor kind, harness actor ID, parent actor ID, subject, and fixed scope. Writer terminal verdict is exactly `SUBJECT_PRODUCED`; reviewer terminal verdict is exactly `ACCEPT`.

Each transcript snapshot requires `raw_transcript_selector=raw_transcript_sha256=SHA-256(these exact two LF-terminated JSONL records)` and exact byte length. Snapshot assignment/terminal fields are copied from those decoded records and must agree. Writer and reviewer snapshot `authority=0` and `accepted=false`.

The canonical task-separation record is emitted from harness task metadata. It binds the writer session, reviewer task, reviewer parent session, `reviewer_isolated=true`, exact subject, and scope. Writer session ID and reviewer task ID must differ; the reviewer task must not equal or reuse the writer task/session, and its isolated execution identity must match the reviewer transcript snapshot. `attribution_evidence` binds the corresponding canonical transcript snapshot. `separation_evidence` binds the canonical task-separation record.

Every `ReviewTaskSeparationDTO` also has `authority=0` and `accepted=false`. `review_scope` is this exact fixed-order array:

```text
[SUBJECT_CANONICAL_BYTES,SUBJECT_EXPECTED_CONTEXT_RECONSTRUCTION,IMPLEMENTATION_SEAM_MANIFEST,STAGE6_POLICY,STAGE6_LIMITS,STAGE6_FINALIZATION,CORPUS_AND_WORKFLOW_BINDINGS,COMPLETE_RECEIPT_INVENTORY,TERMINAL_PROGRAM_C_HEAD,CLAIM_CEILINGS,ACYCLICITY]
```

`verdict=ACCEPT`, `separation_method=DISTINCT_HARNESS_ISOLATED_TASK`, `authority=0`, and `accepted=false`. Writer and reviewer actor identities and attribution-evidence identities must all differ. The verifier resolves every evidence binding, validates exact transcript bytes, canonical snapshots/task records, and harness-issued IDs, proves task separation, confirms exact subject/schema/scope/version equality, and rejects unverifiable, self-reviewed, shared-task, non-affirmative, or stale evidence. `stale` means any subject binding, fixed scope array, review schema/profile, transcript-snapshot schema, task-separation schema, or verdict differs from the values required by the envelope being verified.

Review identity is SHA-256 of the review domain, zero byte, and canonical DTO with only review identity blank. Canonical review digest is SHA-256 of complete populated bytes; length is exact. The review receipt names no envelope, composition, grouping, denominator, lineage, successor, or head.

### Final qualification envelope wire

The exact future envelope schema is `lsp-trace.program-c-representative-qualification.private.v1`, profile is `CALLS_ONLY_FROZEN_CENSUS_V1`, identity domain is `lsp-trace:program-c-representative-qualification:v1`, and field order is:

```text
ProgramCRepresentativeQualificationDTO {
  schema_version string
  profile string
  subject ArtifactBindingDTO
  independent_review_receipt ArtifactBindingDTO
  stage6_policy ArtifactBindingDTO
  stage6_limits ArtifactBindingDTO
  stage6_finalization ArtifactBindingDTO
  qualification_epoch uint64
  authority uint8
  accepted bool
  identity string
}
```

`qualification_epoch` equals the independently verified subject's `terminal_program_c_head.generation` and must be positive. Envelope verification rejects any unequal, older, newer, caller-supplied, or otherwise stale epoch. `authority=0` and `accepted=false`. Identity is SHA-256 of the envelope domain, zero byte, and canonical DTO with only envelope identity blank. Canonical envelope digest is SHA-256 of complete populated envelope bytes; length is exact. Subject, review, policy, limits, and finalization bindings must equal independently verified canonical artifacts. The envelope contains no composition, grouping, representative result, denominator, lineage, successor, or head reference.

The final qualification envelope contains its deterministic terminal-head generation epoch, independently verified subject and review bindings, and exact policy, limits, and finalization bindings. For each singleton, derive:

```text
RepresentativeQualification.CompositionDigest = populated composition digest
RepresentativeQualification.EvidenceDigest = populated grouping digest
RepresentativeQualification.ValidFrom = envelope qualification_epoch
RepresentativeQualification.ValidThrough = envelope qualification_epoch
```

The verifier predicate returns true only when it reopens and verifies the exact final envelope, subject, review receipt, policy, limits, finalization, corpus/predecessor bindings, and epoch, reconstructs this exact qualification value, and observes byte/value equality. A caller-supplied boolean, token, digest, epoch, or envelope field cannot satisfy the predicate.

This mapping is acyclic: the envelope binds the subject/review/policy/limits/finalization and deterministic epoch but no per-singleton composition or grouping; composition binds the already published envelope; grouping binds composition; the derived qualification binds composition and grouping.

### Complete receipt inventory and terminal head

The complete verified inventory is represented inline, not by a new aggregate identity. Entries are strict `plan_ordinal` order from zero, with positive receipt ordinals and schema v3 or v4 only. For each entry:

```text
receipt_selector = receipt_sha256 = sha256(canonical receipt bytes)
receipt_byte_length = len(canonical receipt bytes)
```

The array length equals both terminal `committed_prefix` and `constituent_count`. Every plan ordinal appears exactly once.

`TerminalProgramCHeadDTO` binds exact canonical terminal-head bytes loaded from custody. `selector` equals SHA-256 of the canonical head encoded with its selector field empty, matching `transitionSelector`. `head_bytes_sha256` equals SHA-256 of the exact populated terminal-head bytes, and `byte_length` is their exact length. These digests are distinct bindings and are not required to equal. Generation, committed prefix, and count equal the parsed verified head. It must be the same head reconstructed by complete receipt-inventory verification.

### Claim ceiling

The claim ceiling is exact:

```text
authority=0
accepted=false
feature_identity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
reviews=NOT_REQUESTED
implementation_decision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
source_graph_complete=UNKNOWN
absence_inference=PROHIBITED
```

The subject root also requires `authority=0` and `accepted=false`. In this ceiling, `reviews=NOT_REQUESTED` means downstream feature/stakeholder semantic review has not been requested; it does not describe the mechanical qualification review required here. `implementation_decision=UNISSUED` means no feature/product implementation decision has been issued; it does not negate the accepted private bridge implementation contract.

### Acyclicity and trust

The pure subject parser may establish canonical syntax but not trusted qualification. Trusted validation reconstructs the entire subject from independently verified expected inputs and requires byte identity. The subject contains no review, envelope, denominator, lineage, successor, storage, head-publication, or CAS fields.

## Constituent classification

Every Program C plan constituent appears exactly once in constituent accounting.

### `COMPLETE`

A complete constituent is eligible only when:

1. exact retained Native V5 bytes resolve through verified publication custody;
2. the six declared runs recompute under the accepted seeds and replay inventory;
3. every recomputed community artifact hash agrees with the hashes bound by retained instability bytes;
4. run identities, community order, members, occurrences, and selection contexts validate canonically.

Every admitted run/community becomes one denominator entry and one singleton Stage 6 input.

### `EMPTY`

An empty constituent is terminally evaluated with zero denominator entries. It remains explicit in constituent accounting. It does not nominate a representative and does not establish feature absence.

### `COMMITTED_INCOMPLETE_MATCHING_RESOURCE`

A resource-incomplete constituent remains explicit in the declared constituent denominator but is ineligible for nomination, Describe preparation, successful-instability denominators, and absence or stability inference.

It contributes zero nomination entries and a typed exclusion record. Its complete runs and community hashes cannot be promoted into Stage 6 through this bridge.

### Any other state

Missing, malformed, stale, mixed-version, failed, indeterminate, conflicting, substituted, unreceipted, or unknown predecessor state makes the denominator artifact `INCOMPLETE`. It cannot be treated as empty or silently omitted.

## Denominator artifact

Add a canonical, content-addressed private artifact:

```text
ProgramCRepresentativeDenominator
```

It contains:

- the complete ordered constituent classification inventory;
- the complete ordered nomination-entry inventory;
- exact predecessor selectors, digests, lengths, and identities;
- balanced accounting;
- authority ceilings;
- an immutable selector derived from canonical bytes.

Canonical order is:

```text
constituent ordinal
→ run ID
→ community canonical member bytes
→ community ordinal
```

Unknown fields, duplicate identities, duplicate ordinals, missing records, noncanonical order, identity substitution, trailing bytes, arithmetic overflow, or failed accounting reject the artifact.

## Denominator accounting

Let:

- `P` = declared Program C constituents;
- `C` = complete constituents;
- `E` = empty constituents;
- `R` = committed-incomplete matching-resource constituents;
- `I` = invalid, missing, or otherwise incomplete constituents;
- `D` = eligible run/community denominator entries.

Mandatory equations:

```text
P = C + E + R + I
classified_constituent_records = P

evaluated_constituents = C + E
excluded_constituents = R
unresolved_constituents = I

D = sum(exact admitted community count across every eligible COMPLETE run)
denominator_entry_count = D
```

Let `K = accepted_seed_count × replay_count_per_seed`; under the frozen bindings, `K = 2 × 3 = 6`. Additional mandatory conservation equations are:

```text
terminal_run_records = K × (C + E + R)
eligible_complete_run_records = K × C
empty_run_records = K × E
resource_excluded_run_records = K × R

D = sum(canonical community count over every eligible complete run)
selection_context_count = D
```

Every expected run identity occurs exactly once in its applicable class. All multiplication and summation use checked arithmetic.

A complete denominator artifact requires `I = 0`. `R` may be nonzero because ADR 0013 permits structural-prefix advancement, but every such record remains explicitly excluded and contributes zero entries.

`D = 0` is a complete zero-entry denominator only when all `P` constituents are terminally classified, `I = 0`, and every constituent is `EMPTY` or explicitly resource-excluded. This does not establish feature absence.

## Lineage artifact

Add a second canonical, content-addressed private artifact:

```text
ProgramCRepresentativeLineage
```

It binds one terminal Stage 6 record to every denominator entry.

Each denominator entry is evaluated as one singleton Stage 6 computation: one partition community, one selection context, one `RepresentativeResult`, and one `RepresentativeRecord`. The aggregate result outcome and its sole record outcome must agree and be exactly one of:

```text
SELECTED
NO_CALLS_REPRESENTATIVE
CALLS_EVIDENCE_INELIGIBLE
```

`EMPTY_COMMUNITY` is valid only for a separately verified zero-entry aggregate and creates no lineage record. `MIXED_COMMUNITY_OUTCOMES` is prohibited in the singleton lineage path.

The lineage record binds:

```text
denominator entry
→ exact community artifact and members
→ exact selection context
→ exact admitted CALLS evidence closure
→ Stage 6 result digest and canonical bytes
→ representative record ordinal (zero) and exact canonical bytes
```

For `SELECTED`, it additionally binds the selected node, distance, SCC members, and incoming predecessor witnesses. A non-selection remains an explicit terminal record; it is not omitted and does not establish semantic absence.

## Lineage accounting

Let:

- `D` = denominator entries;
- `S` = `SELECTED` records;
- `N` = `NO_CALLS_REPRESENTATIVE` records;
- `X` = `CALLS_EVIDENCE_INELIGIBLE` records;
- `U` = missing, invalid, or unqualified records.

Mandatory equations:

```text
D = S + N + X + U
stage6_singleton_result_count = D
lineage_record_count = S + N + X
```

A complete lineage artifact requires `U = 0` and:

```text
lineage_record_count = S + N + X = D
```

Every denominator identity appears exactly once. Every lineage identity references exactly one denominator identity. Foreign, duplicate, missing, reordered, or substituted records reject the artifact.

## Immutable storage and heads

Use separate private namespaces for immutable denominator/lineage/qualification objects, a denominator head, and a lineage head.

Expose exactly three separately invokable one-CAS operations:

1. retain and read back the denominator object, then CAS the denominator head against the exact loaded denominator revision;
2. after close/reopen, verify the denominator head, retain and read back the lineage object, then CAS the lineage head against the exact loaded lineage revision and verified denominator selector;
3. after close/reopen, verify both evidence heads and CAS a v2 inventory-successor head against the exact verified v1 successor revision.

No operation automatically starts the next. The v2 successor transition marks only these two requirements `RETAINED`, preserves the remaining three as `NOT_RETAINED`, keeps candidate preparation `NOT_COMPUTED`, and remains `WAITING_EVIDENCE`.

CAS outcomes remain:

```text
not committed      → CONFLICT
commit uncertain   → INDETERMINATE_EXECUTION
```

Neither outcome permits retry, duplicate publication, prefix movement, predecessor rewrite, or silent continuation.

A reopened committed head returns byte-identical artifacts without recomputation or republication. Process restart re-verifies frozen predecessor custody. Immutable objects orphaned by a failed CAS may be reused only after exact canonical-byte, selector, digest, length, predecessor, and qualification verification.

## Stage 6 reuse boundary

The bridge may reuse deterministic mechanics from `internal/programcrepresentative`, including:

- exact partition and selection-context validation;
- relation-occurrence closure validation;
- deterministic CALLS-only selection;
- selected and typed non-selection records;
- canonicalization, replay, limits, and balanced accounting.

Historical fixtures, test-only qualification tokens, historical representative results, and synthetic causal worlds provide design and regression evidence only. They cannot qualify or seed the frozen-corpus artifacts.

Qualification publication is explicitly acyclic:

1. publish a canonical `ProgramCRepresentativeQualificationSubject` binding the accepted ADR digest, adapter contract, exact Stage 6 implementation seam identity, policy, limits, finalization, corpus identity, source pin, complete receipt inventory, Program C terminal head, and claim ceilings;
2. obtain and publish the exact canonical independent-review receipt binding the subject, fixed scope, affirmative verdict, transcript-backed writer/reviewer identities, and independently verified task separation;
3. publish a final `ProgramCRepresentativeQualification` envelope binding the exact subject, review receipt, policy, limits, finalization, and deterministic terminal-head generation epoch;
4. publish denominator, lineage, and successor evidence in that order.

Neither the subject nor review receipt may reference the final envelope or any denominator, lineage, or successor head. Each denominator entry binds the final qualification-envelope selector and byte length without creating a dependency on a not-yet-published denominator head.

Before Stage 6, a repository-owned verifier resolves and verifies the subject, review receipt, and final envelope; checks the receipt's exact scope, affirmative verdict, and reviewer/writer separation; then jointly verifies the committed denominator head and requires corpus, predecessor, adapter, policy, limit, finalization, and qualification identities to agree. Each lineage record binds both the final qualification-envelope selector/length and denominator selector/length.

The verifier rejects test helper tokens, test trust anchors, stale validity, foreign corpus or predecessor identities, non-affirmative or out-of-scope review, non-independent review, and caller-supplied boolean qualification. Only after this repository-owned verification may it provide the narrow qualification predicate required by the existing Stage 6 mechanics.

## Gap-ledger integration

After both heads are independently verified, one inventory-successor transition migrates a strictly verified `lsp-trace.inventory-successor-head.private.v1` generation-1 head to `lsp-trace.inventory-successor-head.private.v2`. The v2 head binds the exact prior successor selector plus denominator selector/byte length and lineage selector/byte length.

From that v2 head, the repository derives `lsp-trace.describe-preparation-gap-ledger.private.v2`, in which only:

```text
REPRESENTATIVE_NOMINATION_DENOMINATOR = RETAINED
COMMUNITY_TO_REPRESENTATIVE_LINEAGE   = RETAINED
```

Each retained requirement binds its exact selector and byte length. Representative nomination changes from:

```text
NOT_COMPUTED, count=null
```

to:

```text
COMPUTED, count=D
```

including `D=0`, only after the lineage artifact balances. Candidate preparation remains:

```text
NOT_COMPUTED, count=null
```

The workflow remains `WAITING_EVIDENCE` until retained source closure, `FULL_DEFINITION` display bindings, and caller-occurrence V3 custody are independently qualified.

## Corrections and history

Published artifacts and heads are immutable. A correction creates a new generation referencing the prior head and records a finite delta. It cannot rewrite Program C receipts, predecessor artifacts, denominator entries, lineage records, or prior gap ledgers.

## Required qualification

Before private implementation can be accepted:

1. freeze the accepted ADR bytes and digest;
2. prove complete ordinal receipt discovery, deterministic bounded v3/v4 probing, exactly-one candidate admission, receipt verification, and reconstructed-final-head equality without caller selectors;
3. prove zero-match and dual-match receipt candidates fail closed while unrelated shared-store objects do not affect reconstructed-chain completeness;
4. prove exact reconstruction from retained Native V5 bytes without checkout or LSP fallback;
5. prove the field-level Native V5 adapter, exact relation-occurrence closure, and singleton context invariant;
6. prove every complete-run community artifact hash matches retained instability evidence;
7. prove all six run identities are conserved and multiple seeds/replays remain distinct denominator entries;
8. prove `EMPTY` contributes zero entries but remains classified;
9. prove resource-incomplete contributes zero entries and remains explicitly excluded;
10. prove omission, duplication, permutation, substitution, stale identity, malformed bytes, and mixed versions fail closed;
11. prove denominator and lineage equations, overflow handling, and limit boundaries;
12. prove every denominator identity has exactly one singleton Stage 6 result and terminal lineage record;
13. prove selected-node, witness, community, context, occurrence, receipt, qualification, and head substitution rejection;
14. prove all three one-CAS operations, close/reopen replay, conflict, cancellation, and uncertain-CAS behavior;
15. prove deterministic v1→v2 successor migration and that only the two authorized gaps become retained;
16. prove all authority ceilings and the remaining three gaps stay unchanged;
17. publish and verify the acyclic qualification sequence—subject, independent-review receipt, then final envelope—before denominator derivation;
18. prove the subject and receipt do not reference the envelope or downstream evidence heads, and prove reviewer/writer separation;
19. run a bounded frozen-corpus qualification from freshly verified custody.

## Alternatives rejected

### Count communities without lineage

Rejected. A balanced count without attributable records permits selective omission or substitution.

### Retain lineage without a frozen denominator

Rejected. Individually plausible records cannot establish complete coverage.

### Collapse identical communities across seeds or replays

Rejected. Historical run identities are distinct evidence contexts and Stage 6 requires one context per community.

### Admit resource-incomplete communities

Rejected. ADR 0013 permanently excludes those constituents from nomination and Describe preparation.

### Reuse historical Stage 6 fixtures or representatives

Rejected. Code reuse does not transfer custody, corpus qualification, or authority.

### Read checkout source or reacquire LSP evidence

Rejected. The bridge must derive only from verified retained predecessor custody.

### Mark all five evidence gaps retained together

Rejected. Each gap has independent provenance and qualification requirements.

## Acceptance record

The proposal at SHA-256 `85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda` received independent verdict `ACCEPT` after three review rounds. On 2026-10-25, @jaresty explicitly selected **Accept ADR 0014**, authorizing private implementation and qualification only under this contract.

This acceptance does not authorize public CLI/MCP exposure, semantic acceptance, candidate preparation before all required evidence is retained, production admission, release, or deployment.

### Accepted receipt-discovery amendment

After acceptance, implementation planning exposed that historical v2 result bytes do not identify whether their immutable transition receipt used schema v3 or v4. The decision authority selected bounded repository-owned v3/v4 probing as the correction. The exact amendment bytes at SHA-256 `df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57` received independent verdict `ACCEPT` and were explicitly accepted by @jaresty on 2026-10-25 for private implementation and qualification only.

### Accepted prequalification mechanics amendment

Implementation planning after complete receipt-inventory custody exposed unresolved ownership, domain-separation, membership, community-binding, and occurrence-ordinal choices for the Native V5 to Stage 6 adapter. The exact amendment bytes at SHA-256 `b0af4add81ab71ca34d91873d8faa728c58166f48ff9b5d46c7ae9c4eb385f47` received independent verdict `ACCEPT` and were explicitly accepted by @jaresty on 2026-10-25 for private implementation and qualification only.

### Accepted qualification-subject wire amendment

After prequalification mechanics were implemented, planning exposed that the subject schema, identity domain, reviewed Stage 6 seam manifest, production Stage 6 policy and limits, finalization, review-evidence wire, envelope, terminal-head byte binding, and complete receipt-inventory representation were not yet frozen. The exact amendment bytes at SHA-256 `235facbbba34f479df4771be845f34b3a39c83988a339dca69eec4f2b6bad9a6` received independent verdict `ACCEPT` and were explicitly accepted by @jaresty on 2026-10-25 for private implementation and qualification only.
