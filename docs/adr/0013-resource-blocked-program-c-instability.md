# ADR 0013: Represent resource-blocked Program C instability without changing historical bytes

- **Status:** Proposed; no implementation authority
- **Date:** 2026-10-25
- **Decision authority:** @jaresty (GitHub handle)
- **Decision owners:** LSP Trace maintainers
- **Depends on:** [ADR 0011](0011-versioned-definition-reference-grouping-evidence.md) and [ADR 0012](0012-private-census-to-candidate-orchestration-bridge.md)
- **Preserves:** community-instability v1 and v2 schemas, canonical bytes, policy identity, matching algorithm, and resource limits

## Context

ADR 0012's private repeated workflow now verifies frozen Census custody once per top-level invocation while retaining close/reopen custody for every Program C transition. In the exact frozen 395-constituent qualification corpus, the workflow advances 46 transitions and then retains a truthful failure:

```text
program_c_outcome=COMPLETE
failure_stage=INSTABILITY_BUILD
failure=matching policy work limit
```

A diagnostic replay from the exact frozen receipt and availability ledger established the first rejected matching dimensions:

```text
left_community_count=17
right_community_count=17
matching_state_count=131072
matching_work=2228224
matching_work_limit=1000000
```

This is the existing `n * 2^m` policy guard, not the `m >= 63` width guard and not defective accounting. Six Program C runs completed and their community and boundary artifacts were constructed in memory, but pairwise exact matching could not complete under the frozen policy. Community-instability v2 can represent pairs blocked by an incident `FAILED` or `INCOMPLETE` run; it does not define a pair whose incident runs are complete but whose comparison is resource-blocked.

Increasing the limit, retrying unchanged work, treating the failure as successful or empty, skipping the constituent, or changing v2 interpretation would violate accepted constraints. Retaining only a generic failed transition is truthful but cannot advance the corpus or produce a canonical instability artifact.

## Decision required

Define an additive private community-instability v3 contract for complete runs whose pairwise comparison is blocked by the frozen matching resource policy.

No implementation begins merely because this proposal exists. Implementation requires independent review, explicit acceptance by the decision authority, and a frozen proposal digest.

## Proposed contract

### Versions and immutability

1. Community-instability v1 and v2 schemas, canonical bytes, policy bytes, digests, validators, fixtures, and production outputs remain unchanged.
2. V3 is additive. V1 or v2 bytes cannot be reinterpreted as v3.
3. The matching algorithm, tie-break, pair order, and `instabilityMaxMatchingWork = 1000000` remain unchanged.
4. Existing successful nonempty and EMPTY production behavior remains byte-identical.

### Resource-blocked pair

V3 adds one pair outcome:

```text
BLOCKED_MATCHING_RESOURCE
```

It is valid only when:

- both incident runs are valid `COMPLETE` runs;
- canonical community ordering and pair ordering have been established;
- after existing canonical side selection, `n` is the smaller community count and `m` is the larger community count;
- the guards are evaluated in this exact order:

```text
if m >= 63:
  guard_kind = MATCHING_STATE_WIDTH
  matching_state_count = unavailable
  matching_work = unavailable
else:
  matching_state_count = 2^m
  matching_work = n * matching_state_count
  if matching_work > matching_work_limit:
    guard_kind = MATCHING_WORK_LIMIT
```

- `2^m` and `n * 2^m` are evaluated using checked non-negative integer arithmetic; overflow or inability to represent an exact required value fails closed and cannot wrap, saturate, or select a different outcome;
- `BLOCKED_MATCHING_RESOURCE` is valid only for `MATCHING_STATE_WIDTH` or `MATCHING_WORK_LIMIT`;
- the artifact records `guard_kind`, `n`, `m`, the exact state count and work value when the selected guard permits their computation, and the frozen limit;
- the guard is evaluated before dynamic-programming allocation or matching search;
- no matching, node reassignment, unmatched-community, variation-of-information, or pairwise-Jaccard metric is invented for the blocked pair.

The pair reason is structural resource accounting only. It establishes no semantic claim and no evidence of similarity, difference, stability, or instability.

### Artifact outcome and accounting

If any pair is `BLOCKED_MATCHING_RESOURCE`:

```text
artifact.outcome=INCOMPLETE
```

V3 records these mutually exclusive pair counts:

- `completed_pair_count`;
- `blocked_incomplete_run_pair_count`;
- `blocked_failed_run_pair_count`;
- `blocked_matching_resource_pair_count`.

Classification precedence is exact: a pair with either incident run `FAILED` is `BLOCKED_FAILED_RUN`; otherwise a pair with either incident run `INCOMPLETE` is `BLOCKED_INCOMPLETE_RUN`; otherwise both runs are `COMPLETE` and the pair is either completed or `BLOCKED_MATCHING_RESOURCE`.

The following equations are mandatory:

```text
declared_run_count =
  completed_run_count +
  failed_run_count +
  incomplete_run_count

expected_pair_count =
  declared_run_count * (declared_run_count - 1) / 2

expected_pair_count =
  completed_pair_count +
  blocked_incomplete_run_pair_count +
  blocked_failed_run_pair_count +
  blocked_matching_resource_pair_count

len(pairwise_comparisons) = expected_pair_count
```

Every expected canonical run pair has exactly one pair record and contributes to exactly one count. Unknown outcomes, duplicate or missing pairs, arithmetic overflow, or any failed equation invalidate the artifact. `STABLE` and `UNSTABLE` are valid only when every expected pair completed; otherwise the artifact outcome is `INCOMPLETE`.

### Program C transition behavior

A canonical v3 artifact with `outcome=INCOMPLETE` is immutable retained evidence. It never establishes successful instability completion.

**Acceptance gate:** exactly one rule below must be retained as normative text before this ADR's status may become `Accepted`. Acceptance is invalid if both rules remain, neither rule remains, the selection appears only in an acceptance-record comment, or the selected rule leaves receipt outcome, prefix movement, durable-head state, replay, or CAS behavior unspecified.

**Rule A — retain without prefix advancement**

- retain the canonical v3 artifact in the immutable receipt;
- record a distinct terminal receipt disposition `INCOMPLETE_MATCHING_RESOURCE`;
- do not classify the receipt as generic `FAILED` or successful instability completion;
- do not advance `committed_prefix`;
- the durably linked head remains in the versioned retained terminal state `RETAINED_INCOMPLETE_MATCHING_RESOURCE`, binds the immutable receipt, and preserves the unchanged `committed_prefix`;
- return the retained terminal workflow outcome without retry, skip, replacement, or predecessor rewrite;
- close and reopen storage before any later invocation, which must return the same retained disposition from the verified head and receipt;
- preserve existing CAS mappings exactly: not committed is `CONFLICT`; commit uncertainty is `INDETERMINATE_EXECUTION`; neither condition permits retry or advancement.

**Rule B — advance structural prefix as incomplete**

- retain the canonical v3 artifact in the immutable receipt;
- record a distinct terminal receipt disposition `COMMITTED_INCOMPLETE_MATCHING_RESOURCE`;
- perform at most the existing single logical Program C transition and advance `committed_prefix` exactly once only after the receipt is durably linked and verified;
- permanently mark the constituent ineligible for candidate nomination, Describe preparation, successful-instability denominators, and any inference of absence, stability, or successful instability completion;
- close and reopen storage before processing the next constituent;
- after close/reopen, verification of the committed head and receipt returns the same committed incomplete disposition without recomputing or republishing the constituent or advancing it again; the next invocation selects only `plan[committed_prefix]`;
- preserve existing CAS mappings exactly: not committed is `CONFLICT`; commit uncertainty is `INDETERMINATE_EXECUTION`; neither condition permits retry, duplicate advancement, or predecessor rewrite.

The implementation must not choose between these rules.

### Authority ceiling

Every v3 artifact and downstream projection remains governed by:

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

A resource-blocked artifact cannot establish feature identity, grouping identity, purpose, value, completeness, production use, acceptance, or authority. It supplies only canonical evidence that a declared structural comparison did not complete under the declared frozen resource policy.

### Custody and orchestration

- V3 binds the exact graph, projection, implementation, parameters, seeds, run inventory, community artifacts, boundary artifacts, policy, and resource-policy identities already required by v2.
- Resource dimensions are derived internally; callers cannot supply counts, limits, pair selectors, processed sets, retry policy, skip policy, or outcomes.
- The private repeated workflow continues one durable transition at a time and closes/reopens storage between transitions.
- Unknown CAS outcomes remain `INDETERMINATE_EXECUTION`.
- Retained failed predecessors are never rewritten or silently upgraded.
- A new top-level invocation re-verifies exact frozen inputs.

### Public and semantic exclusions

This decision does not authorize:

- public CLI or MCP registration;
- Describe packet construction;
- representative nomination or candidate preparation;
- reuse of historical candidates, groups, or Describe results;
- semantic grouping, correction, or acceptance;
- production admission, release, or deployment;
- increasing or bypassing matching limits.

## Required qualification

Before private implementation can be accepted:

1. Freeze the accepted ADR bytes and digest.
2. Add RED tests for both policy guards and their exact canonical accounting.
3. Prove successful v1 and v2 fixture bytes remain unchanged.
4. Prove successful nonempty production v2 bytes remain unchanged.
5. Prove EMPTY v2 bytes and semantics remain unchanged.
6. Prove blocked v3 pairs contain no fabricated matching or boundary metrics.
7. Prove denominator balance, canonical pair ordering, input permutation behavior, and deterministic replay.
8. Prove malformed, substituted, stale, mixed-version, and limit-mismatched artifacts fail closed.
9. Prove cancellation, conflict, and uncertain CAS behavior remain unchanged.
10. Prove standalone invocation and separate top-level repeated invocations independently reverify authoritative inputs.
11. Rerun the exact frozen corpus from fresh state and retain the first authoritative terminal outcome.
12. Obtain independent review distinct from the implementation writer.

## Alternatives rejected

### Increase the matching-work limit

Rejected. It changes the frozen resource policy and converts a bounded refusal into unqualified work.

### Retry the unchanged constituent

Rejected. The input and deterministic policy are unchanged, so retry supplies no new evidence.

### Mark one or more complete runs incomplete

Rejected. The runs completed; rewriting their status would falsify run accounting.

### Emit a v2 blocked pair

Rejected. V2 defines blocked pairs through incident run status and has no resource-blocked-comparison outcome. Reinterpretation would change historical semantics.

### Skip the constituent and advance

Rejected. It would create an unaccounted hole in the committed prefix and permit absence inference from omitted work.

### Treat resource blocking as successful empty instability

Rejected. The admitted graph and communities are nonempty, and no pairwise determination completed.

## Acceptance record

None. This proposal remains non-authoritative until independently reviewed, corrected if required, frozen by digest, and explicitly accepted by @jaresty. The acceptance record must identify the single retained rule by name and freeze the resulting ADR digest. An acceptance record is invalid while both alternatives remain normative.
