# ADR 0014 amendment proposal: private multi-seed v2 mechanics and preflight

- **Status:** PROPOSED
- **Date:** 2026-10-26
- **Authority:** `0`
- **Accepted:** `false`
- **Decision state:** non-authoritative until independent review and explicit design-time acceptance
- **Amends:** [ADR 0014](../adr/0014-private-representative-denominator-lineage.md)
- **Scope:** private multi-seed v2 mechanics, private adapter, and private preflight only

This proposal has no force merely because it exists. Its exact bytes and SHA-256 must be independently reviewed, receive an affirmative proposal verdict, and then receive explicit design-time acceptance before implementation is authorized. The `ACCEPT_MEASUREMENT` verdict below accepts bounded measurement evidence only; it is not review or acceptance of this proposal.

## Exact evidence binding

This proposal binds these exact inputs:

| Artifact | SHA-256 | Bytes |
|---|---:|---:|
| Repository HEAD before this proposal | `71aea02ffd1da4042860db697deafc7cb83e7a20` | — |
| `internal/censuscontinuation/programc_v2_multiseed_measurement_test.go` | `44e9c3c05acd72fa536d0bc047a9173625e40f29c632f699fded360ad987a1c2` | `29127` |
| `docs/qualification/adr0014-private-multiseed-v2-mechanics-measurement-v1/measurement.json` including final LF | `40cdd09eb7d2fc84b97f45ff5672ac73c86d7b19f46e635df636556619f4fa31` | `1073` |
| Raw JSON in `measurement.json`, excluding its one final LF | `b4dbadaa546c8ee6d9f10f77b5b15e04e0032fc1743063caf84602b1821d5964` | `1072` |
| `docs/qualification/adr0014-private-multiseed-v2-mechanics-measurement-v1/MEASUREMENT_EVIDENCE.md` | `c032c5633de83d3b681df82f0e94a036166c6263db022bf9d55615749b74a04f` | `3876` |

Independent measurement review returned `ACCEPT_MEASUREMENT` in isolated task `feed307e-259e-489`. That verdict is bounded by the evidence note's pilot-local custody and authority/completeness ceilings. It establishes neither portable retained-state custody nor production, semantic, public, denominator, or lineage authority.

## Additive decision and supersession boundary

If accepted, this amendment authorizes implementation and qualification of only:

1. pure immutable multi-seed v2 mechanics in `internal/programcrepresentative`;
2. a private repository-owned Native V5-to-v2 adapter in `internal/censuscontinuation`;
3. a private read-only frozen-corpus preflight for those mechanics.

It supersedes only ADR 0014 v1's requirement that an eligible constituent have exactly one invocation-seed membership group. The supersession applies only to additive v2 mechanics. Native V5 is unchanged. Every v1 schema, domain, canonical byte, identity, parser, golden, constructor, and public/private contract remains immutable. V1 mechanics remains available and continues to reject zero or multiple invocation-seed groups.

V2 is reconstructed fresh from strictly verified retained Native V5 and Program C custody. Converting, wrapping, promoting, or inferring v2 from v1 mechanics is prohibited.

The denominator remains exactly one eligible Program C run/community. Invocation seed is subordinate mechanics evidence inside that unit; it is not a denominator axis. Therefore multi-seed v2 changes no Program C run count, community count, or denominator count `D`.

## Ordered seed-group bijection

For each eligible retained Native V5 source, canonical `invocation.seeds` and canonical `seed_memberships` must satisfy an ordered cardinality- and value-preserving bijection:

```text
(seed_groups[i].seed_label,
 seed_groups[i].seed_at,
 seed_groups[i].resolved_uri,
 seed_groups[i].content_sha256,
 seed_groups[i].language_id) == invocation.seeds[i]
seed_groups[i].invocation_seed_ordinal == i
seed_group_key = (execution_bundle_id, seed_label, seed_at)
len(seed_groups) == len(invocation.seeds)
∀ membership m: ∃! seed_group g such that m is retained in g
∀ seed_group g: len(g.memberships) > 0
```

Every canonical membership appears exactly once in its tuple-keyed group. Unknown or foreign keys, duplicate seed keys, duplicate membership identities, missing memberships, duplicate memberships, empty groups, and cardinality or value mismatch fail closed.

All five canonical membership kinds are retained in each group's identity closure, in the existing fixed contract order:

```text
PREPARED_TARGET
REACHED_NODE
CALL_RELATION
SIBLING_CANDIDATE
DISPATCH_ASSOCIATION
```

Only the first three map, and they map group-locally:

```text
PREPARED_TARGET  -> seed_group.prepared_targets
REACHED_NODE     -> seed_group.reached_nodes
CALL_RELATION    -> seed_group.relation_ids
```

`SIBLING_CANDIDATE` and `DISPATCH_ASSOCIATION` remain identity-bound but do not affect CALLS-only mechanics. No implementation may flatten groups, select one group, drop a group or membership, merge groups, reorder groups, construct consensus, or emit one mechanics value per invocation seed.

For each eligible Program C run/community there is exactly one `MechanicsIdentityDTOv2`. It contains one ordered `SelectionContextIdentityDTOv2`, which contains the complete ordered `[]SeedGroupIdentityDTOv2`. There is not one mechanics value or one denominator entry per seed group.

## Canonical v2 schemas, domains, and field order

Canonical JSON retains ADR 0014 v1 rules: lower-snake-case field names in the declared order; UTF-8 strings; base-10 integers; arrays always present and never `null`; strict unknown-field, duplicate-key, trailing-byte, and canonical-order rejection; compact Go `encoding/json.Marshal` bytes with HTML escaping enabled and no trailing LF. Every v2 identity is:

```text
sha256(v2 domain UTF-8 bytes || 0x00 || canonical DTO with only that DTO's identity="")
```

The profile remains `CALLS_ONLY_FROZEN_CENSUS_V1`. New schema IDs and domains are:

```text
AdapterContractDTOv2.schema_version = lsp-trace.program-c-stage6-adapter-contract.private.v2
AdapterContractDTOv2 domain         = lsp-trace:program-c-stage6-adapter-contract:v2
SeedGroupIdentityDTOv2.schema_version = lsp-trace.program-c-stage6-seed-group.private.v2
SeedGroupIdentityDTOv2 domain         = lsp-trace:program-c-stage6-seed-group:v2
SelectionContextIdentityDTOv2.schema_version = lsp-trace.program-c-stage6-context.private.v2
SelectionContextIdentityDTOv2 domain         = lsp-trace:program-c-stage6-context:v2
MechanicsIdentityDTOv2.schema_version = lsp-trace.program-c-stage6-mechanics.private.v2
MechanicsIdentityDTOv2 domain         = lsp-trace:program-c-stage6-mechanics:v2
```

V1 `PositionDTO`, `RangeDTO`, `EvidenceIdentityDTO`, `CustodyIdentityDTO`, and `MembershipIdentityDTO` may be reused only with their exact v1 schema/domain semantics and byte-identical populated values. They are not rehashed under v2 domains. No other v1 identity is reusable in v2.

The exact v2 DTO field order is:

```text
AdapterContractDTOv2 {
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
  seed_group_order_rule string
  seed_group_key_rule string
  membership_partition_rule string
  group_occurrence_closure_rule string
  authority uint8
  accepted bool
  identity string
}

SeedGroupIdentityDTOv2 {
  schema_version string
  execution_bundle_id string
  invocation_seed_ordinal int
  seed_label string
  seed_at string
  resolved_uri string
  content_sha256 string
  language_id string
  memberships []MembershipIdentityDTO
  prepared_targets []string
  reached_nodes []string
  relation_ids []string
  occurrence_ids []string
  identity string
}

SelectionContextIdentityDTOv2 {
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
  seed_groups []SeedGroupIdentityDTOv2
  identity string
}

MechanicsIdentityDTOv2 {
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
  selection_context SelectionContextIdentityDTOv2
  authority uint8
  accepted bool
  identity string
}
```

`AdapterContractDTOv2.domains` is this exact order:

```text
lsp-trace:program-c-stage6-adapter-contract:v2
lsp-trace:program-c-stage6-evidence:v1
lsp-trace:program-c-stage6-seed-group:v2
lsp-trace:program-c-stage6-context:v2
lsp-trace:program-c-stage6-custody:v1
lsp-trace:program-c-stage6-mechanics:v2
```

The new fixed rules are:

```text
seed_group_order_rule=NATIVE_V5_INVOCATION_SEEDS_ARRAY_ORDER
seed_group_key_rule=EXECUTION_BUNDLE_ID_SEED_LABEL_SEED_AT
membership_partition_rule=ORDERED_CARDINALITY_VALUE_PRESERVING_BIJECTION
group_occurrence_closure_rule=EXACT_ALL_CALLS_OCCURRENCES_FOR_GROUP_RELATION_IDS
```

`SeedGroupIdentityDTOv2` preserves the zero-based canonical invocation-seed array ordinal and all five Native V5 seed fields exactly and in Native V5 declaration order: `label`, `at`, `resolved_uri`, `content_sha256`, `language_id`. Optional source values remain exact strings in the v2 DTO; canonical v2 fields are not omitted. Seed ordinal, label, position, resolved URI, content digest, or language substitution fails closed.

Memberships remain strict ascending `membership_id` order within each group. Seed groups retain exact Native V5 invocation-seed array order. Prepared targets, reached nodes, relation IDs, and occurrence IDs are strict lexicographic unique arrays within their owning group. Communities, community members, root evidence, and run order retain v1 rules.

For each group, every retained `CALL_RELATION` must resolve to a canonical Native V5 CALLS edge in the same execution bundle. The group's `occurrence_ids` is the exact complete closure of all canonical call-site occurrences for precisely that group's `relation_ids`. Missing, foreign, duplicate, extra, or partial occurrence closure fails closed. The same occurrence may be identity-bound by more than one group only when its relation is independently and canonically retained by each group; no cross-group deduplication changes group-local closure.

A seed-group identity binds its canonical invocation-seed ordinal, all five invocation-seed fields, complete membership list, all four group-local arrays, and execution bundle. Context identity binds the v2 adapter contract, v1 custody identity, constituent, execution bundle, and complete ordered seed-group values, not only their identities. Mechanics identity binds the complete context. Mixed v1/v2 adapter, seed-group, context, or mechanics schemas fail closed.

Constructors, parsers, DTO accessors, and canonical-byte accessors deep-copy every mutable value: root evidence, root community members, the complete selection context, seed groups, each group's memberships, all four group-local arrays, and canonical bytes. Every declared array is present and non-`nil`, including empty group-local arrays, so canonical JSON uses `[]`, never `null`. Mutation of constructor inputs or any returned nested DTO/array/byte value cannot alter retained state, identity, validity, or subsequent canonical bytes.

Only `authority=0` and `accepted=false` are wire fields, at the exact DTO positions shown above. `semantic_authority=NONE` and `public_authority=NONE` are non-wire scope constraints: they must not be serialized into any v2 DTO and are enforced by this amendment's explicit refusal to authorize downstream semantic or public surfaces. Canonical golden tests require those keys to be absent.

Standalone mechanics bytes remain unauthenticated prequalification values. Final use must reopen verified receipt inventory and Native V5 custody, reconstruct v2 mechanics, and require canonical byte identity.

## Frozen corpus accounting

The accepted measurement evidence freezes:

```text
constituents P=395
COMPLETE C=268
EMPTY E=2
resource-incomplete R=125
eligible runs=1608
eligible run/community results D=15537
seed-group results G=530232

max seed groups per eligible source=63
max memberships per eligible source=1312
max memberships per seed group=154
max group occurrence IDs per seed group=212
max sum group occurrences per eligible source=1120
max measured structural string bytes=235
max communities per run=15
max total partition members per run=214
max candidates per community=67
max composition occurrences per run=1120
max outer run/community work=7186
max run-source work=16021
```

`D` remains the v1 eligible run/community count and is unchanged by v2. `G` is checked:

```text
G = sum(seed_group_count(source) for every eligible run/community result using that source)
  = 530232
```

Every `EMPTY` and resource-incomplete constituent contributes zero eligible v2 contexts and zero to `D` and `G`. It remains explicitly classified in complete terminal accounting.

## Frozen preflight limits and checked work

```text
MaxSeedGroups=64                         // per eligible source
MaxMemberships=2048                      // per eligible source
MaxMembershipsPerGroup=256               // per seed group; observed max 154
MaxGroupOccurrences=256                  // per seed group
MaxSumGroupOccurrences=2048              // per eligible source
MaxStringBytes=1048576                   // independent defensive cap for every admitted v2/preflight string
MaxCommunities=10000                     // per run; retained v1 value
MaxMembers=10000                         // total partition members per run; retained v1 value
MaxCandidates=10000                      // per community; retained v1 value
MaxOccurrences=100000                    // composition occurrences per run; retained v1 value
MaxOuterWork=10000                       // per eligible run/community result
MaxRunSourceWork=20000                   // per eligible run source
```

All counters and intermediate additions/multiplications use checked unsigned arithmetic. Each individual comparison is inclusive: equality with that limit passes that comparison and `limit+1` fails it. Passing one comparison does not override another limit; an input equal to `MaxCandidates`, `MaxMembers`, `MaxCommunities`, or `MaxOccurrences` may correctly fail a work cap. End-to-end admission requires all limits simultaneously. Define for one eligible run source:

```text
O = composition occurrence count for the run
N = total partition member count for the run
S = seed-group count for the source
M = seed-membership count for the source
Q = sum of group-local occurrence counts for the source
C = candidate/member count for the current community
```

The exact work equations and scopes are:

```text
OuterWork(run, community, source)
  = checked_add(O, N, S, M, Q, checked_mul(S, C))
  <= MaxOuterWork

RunSourceWork(run, source)
  = checked_add(O, N, S, M, Q, checked_mul(S, N))
  <= MaxRunSourceWork
```

`OuterWork` is checked once for each eligible Program C run/community result. `RunSourceWork` is checked once for each eligible run source. `S`, `M`, and `Q` are source-local; `O` and `N` are run-local; `C` is community-local. No counter is amortized across constituents, runs, communities, or groups.

The measured value `235` is scoped exactly to strings in canonical Native V5 invocation seeds, seed memberships, CALLS edges, derived seed groups, Program C occurrences, and Program C communities. It does not measure workflow/head/receipt/publication/constituent selectors, run IDs, algorithm/contract constants, custody fields, or every future canonical output string. Therefore `MaxStringBytes=1048576` is an independent defensive cap applied to every string admitted by v2 mechanics construction, custody reconstruction, adapter/preflight input, and canonical v2 output; it is not inferred from `235`.

`MaxCanonicalBytes` remains deferred to the post-qualification contract. It is not frozen, inferred, or checked by this mechanics preflight. No canonical-byte estimate may substitute for that later reviewed limit.

## Fail-closed preflight and exposure barrier

The private adapter/preflight rejects exact-bijection failure; unknown, foreign, duplicate, or missing seeds/memberships; empty groups; membership-kind or endpoint mismatch; group-local or global closure mismatch; noncanonical ordering; arithmetic overflow; any limit violation; mixed schema; partial inventory; partial mechanics; and partial exposure.

The frozen-corpus preflight is single-pass and read-only. It loads the terminal Program C head exactly once, reconstructs and verifies the complete receipt inventory, classifies every constituent, reconstructs all eligible v2 mechanics, checks all accounting and limits, and only then crosses one terminal equality barrier. Before that barrier it returns no mechanics, context, per-constituent result, partial accounting, or positive eligibility signal. On any failure it returns no v2 value or accounting. It performs zero immutable-object writes and zero head CAS operations.

## Versioning and implementation sequence

1. Native V5 remains unchanged.
2. V1 mechanics and all v1 goldens/contracts remain unchanged; v1 continues rejecting multi-seed input.
3. Implement pure v2 mechanics and canonical parse/validate/replay first in `internal/programcrepresentative`.
4. Only after pure mechanics tests pass, implement the private v2 adapter and terminal preflight in `internal/censuscontinuation`.
5. Independently review the complete implementation and pin its exact repository/file revision before any downstream v2 contract is proposed.
6. Only afterward may a separate amendment propose exact `RepresentativeGroupingV2`, `ResultV2`, `QualificationV2`, subject, policy, finalization, review, and envelope v2 contracts.

This proposal deliberately does not freeze those downstream DTOs. It does not authorize denominator publication, lineage, successor migration/publication, candidate preparation, public or private CLI/MCP registration, production admission, release, or deployment.

## Required tests and acceptance criteria

An implementation is eligible for independent review only when all of these exact tests pass:

1. **Five-seed mechanics:** five canonical invocation seeds produce one mechanics value with five ordered nonempty groups; every membership appears exactly once; every full invocation-seed value is preserved; all five kinds are retained; first-three mappings and occurrence closure are group-local.
2. **Observed-maximum mechanics:** 63 canonical seeds pass and preserve order/cardinality/value; 64 passes only when every other limit remains satisfied; 65 fails `MaxSeedGroups`.
3. **Denominator conservation:** multi-seed v2 leaves `D=15537` unchanged.
4. **Seed-group accounting:** exact frozen-corpus recomputation gives `G=530232`; omit, duplicate, reorder, flatten, select, drop, merge, or consensus mutations fail.
5. **V1 immutability:** all v1 canonical goldens remain byte-identical and v1 still rejects the same multi-seed fixture accepted by v2.
6. **Identity/schema:** every v2 schema/domain and declared field order has an exact canonical golden; evidence/custody reuse is byte-identical; v1/v2 substitution and mixed schemas fail.
7. **Closure, seed fields, and immutability:** missing/extra/foreign/duplicate group relations or occurrences, membership endpoint mismatch, and mutation of any of the five invocation-seed fields fail. Mutation tests cover constructor inputs and every nested accessor result: root evidence/community members, context, seed groups, memberships, four group arrays, and canonical bytes; empty arrays remain non-`nil` and encode as `[]`. Golden tests reject serialized `semantic_authority` or `public_authority` keys.
8. **Limits:** isolated comparator tests prove equality and `+1` behavior for every individual limit; jointly feasible end-to-end fixtures prove simultaneous admission. Tests also prove that equality with `MaxCandidates`, `MaxMembers`, `MaxCommunities`, or `MaxOccurrences` may still fail a work cap. Checked addition and multiplication overflow fail; `MaxMembershipsPerGroup=256` is explicitly exercised independently of the observed `154`.
9. **Work accounting:** exact equation terms and per-run/community versus per-run-source scopes are asserted; observed maxima `7186` and `16021` pass.
10. **Excluded classifications:** every `EMPTY` and resource-incomplete constituent creates zero eligible v2 contexts and contributes zero to `D` and `G` while remaining classified.
11. **Terminal barrier:** success performs exactly one head load, zero object writes, zero CAS operations, and exposes results only after complete terminal equality; every injected failure exposes zero partial result/accounting.
12. **Regression gates:** `gofmt` is clean, default `go test ./internal/censuscontinuation` passes, affected pure-mechanics tests pass, evidence hashes match, and `git diff --check` passes.

Passing these tests does not accept this proposal or implementation. Independent implementation review and an exact revision pin are still required.

## Explicit non-authorization

This is prequalification only. It authorizes no `RepresentativeGroupingV2`, representative `ResultV2`, `QualificationV2`, qualification subject/review/envelope v2, denominator or lineage object/head, successor transition, candidate preparation, semantic claim, public authority, CLI, MCP, production code path, production admission, release, or deployment.
