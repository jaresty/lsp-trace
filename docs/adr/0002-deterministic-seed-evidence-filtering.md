# ADR 0002: Add deterministic pairwise seed-evidence comparison

- **Status:** Accepted
- **Date:** 2026-09-02
- **Decision owners:** LSP Trace maintainers
- **Scope:** Read-only pairwise set projection over admitted `lsp-trace.inspect.v1` evidence
- **Related commands:** `inspect`, proposed `filter`, `schema get`, `validate`

## Context

`lsp-trace.inspect.v1` `ALL_SEEDS` output preserves stored seed state, per-seed memberships and references, copied-once native records, global diagnostics and boundaries, and mechanical accounting. A reviewer investigating overlapping technical workflows or a possible split needs one basic operation before broader search or grouping:

- compare two exact seeds;
- identify exact references shared by both;
- identify exact references exclusive to either seed;
- retain failure, empty-evidence, and completeness boundaries;
- keep evidence namespaces and authority distinct.

This is a deterministic set projection. It does not decide whether the seeds represent one feature, several features, shared infrastructure, or incidental reuse.

The original draft proposed pairwise comparison, shared-by-threshold lookup, reverse evidence lookup, seed-state filtering, three input families, and several mode-specific output shapes. Probe review found that scope too broad for a first compatibility surface. It also found unresolved identity, duplicate, namespace, custody, ordering, and accounting semantics.

BM25, embeddings, lexical search, automatic clustering, ranking, and merge/split recommendations are different classes of operation and remain outside this decision.

## Decision

Add one domain-neutral command:

```bash
lsp-trace filter INSPECTION \
  --compare-seeds LEFT_LABEL \
  --compare-seeds RIGHT_LABEL \
  --json
```

V1 performs pairwise comparison only.

The command:

- accepts one `lsp-trace.inspect.v1` `ALL_SEEDS` document;
- requires exactly two repeated `--compare-seeds` values;
- partitions explicit typed references into `shared`, `left_only`, and `right_only`;
- emits references only, not copied native records;
- preserves selected seed state and global completeness boundaries;
- emits independently versioned `lsp-trace.filter.v1` JSON;
- remains read-only and starts no language server;
- makes no feature, workflow, merge, or split determination.

Shared-by-threshold lookup, reverse lookup, seed-state filtering, graph/selector input, generic expressions, and text retrieval are deferred.

## Input contract

V1 accepts exactly one input family:

```text
inspection_schema_version = lsp-trace.inspect.v1
projection_kind = ALL_SEEDS
authority = NON_AUTHORITATIVE_DERIVED_VIEW
```

Single-seed inspection projections, graph artifacts, selectors, and unknown JSON documents fail closed.

Input is path-only in v1. `-` stdin support is deferred.

### Input admission

Admission order is:

```text
CLI parse and mode validation
→ bounded header decoding
→ inspection-family discrimination
→ Draft 2020-12 inspection-schema validation
→ reusable all-seed semantic/accounting/reference validation
→ pairwise projection
→ filter semantic/accounting/reference validation
→ Draft 2020-12 filter-schema validation
→ JSON emission
```

Mode validation occurs before reading the input.

Family discrimination requires the explicit constants above. Absence of `inspection_schema_version`, a graph `schema_version`, a single-seed projection kind, or conflicting family markers fails with:

```text
filter: input must be lsp-trace.inspect.v1 ALL_SEEDS
```

### Inspection custody ceiling

An inspection document contains artifact identities but no selector receipt or input-origin proof. Validating it establishes only that the inspection projection is structurally and semantically self-consistent under its contract.

Filtering an inspection document does not:

- recover or transfer selector custody;
- prove the inspection was produced from a selector;
- authenticate its producer;
- authenticate source, server, execution, environment, or hidden inputs;
- turn computed digests into signatures or custody receipts.

## CLI contract

### Usage

```text
usage: lsp-trace filter INSPECTION --compare-seeds LABEL --compare-seeds LABEL [--json]
```

The top-level usage text advertises `filter`.

`filter -h` and `filter --help` print command usage and flag descriptions to stdout and exit `0` with empty stderr. `--json` is optional and has no behavioral effect in v1; it is accepted for consistency with `inspect` and possible future format negotiation.

### Pair operands

Exactly two `--compare-seeds` occurrences are required. Labels must:

- be nonempty;
- be distinct;
- each match exactly one stored seed;
- appear uniquely in the admitted inspection input.

Operand order is meaningful only for naming `left_only` and `right_only`. Reversing operands swaps those two partitions and leaves `shared` unchanged.

### Errors

Syntax and mode errors print usage. Semantic errors use an actionable `filter:` prefix. All failures exit `1` with empty stdout.

Required error distinctions include:

```text
filter: exactly two --compare-seeds values are required
filter: compared seed labels must be distinct
filter: seed label "LABEL" not found
filter: duplicate seed label "LABEL" in inspection input
filter: input must be lsp-trace.inspect.v1 ALL_SEEDS
filter: invalid inspection accounting: ...
filter: unresolved TYPE reference "ID" for seed "LABEL"
filter: duplicate TYPE reference "ID" for seed "LABEL"
```

Additional positional arguments fail with usage.

## Reusable inspection admission

Current inspection validation is insufficient for arbitrary inspection documents because structural validation alone does not recompute accounting or resolve references. Before `filter` ships, `lsp-trace` must expose a reusable all-seed inspection validator outside the command-local projection constructor.

It must verify:

- exact `ALL_SEEDS` kind and authority constants;
- stored seed labels are unique and ordered;
- every seed preparation status is valid;
- failed seeds have empty filterable reference collections;
- requested, successful, failed, with-membership, and without-membership counts reconcile;
- every global record identity is unique in its typed namespace;
- every explicit reference resolves exactly once in its namespace;
- diagnostic indexes are in range;
- duplicate per-seed references are rejected;
- record and reference counters recompute from arrays;
- no graph `schema_version` is present.

A label is the v1 seed selector because admitted inspection validation guarantees uniqueness. No stronger seed identity is invented by this ADR. Stored invocation order remains the stable ordering coordinate.

## Typed reference namespaces

Set operations use domain-separated keys:

```text
ReferenceKey = (namespace, value)
```

The closed v1 namespace vocabulary is:

```text
NODE
CALL_RELATION
DISPATCH_RELATIONSHIP
SIBLING_CANDIDATE
DIAGNOSTIC_CORRELATION
```

Equal raw strings in different namespaces never compare equal.

### Reference sources

References are derived only from explicit admitted per-seed fields:

| Namespace | Explicit source |
|---|---|
| `NODE` | `native_node_ids` |
| `CALL_RELATION` | `native_call_relation_ids` |
| `DISPATCH_RELATIONSHIP` | same-label `seed_memberships` whose evidence kind is `DISPATCH_ASSOCIATION` |
| `SIBLING_CANDIDATE` | same-label `seed_memberships` whose evidence kind is `SIBLING_CANDIDATE` |
| `DIAGNOSTIC_CORRELATION` | `correlated_diagnostic_indexes` |

Dispatch and sibling references remain distinct even if their raw endpoint IDs collide.

V1 does not compare or partition `seed_memberships` as records. `inspect.v1` does not define a cross-seed canonical membership identity, and endpoint equality is not sufficient to invent one.

### Explicit-reference-only rule

A reference is never inferred from another namespace or from embedded record fields.

For example, a seed that explicitly references a call relation but does not list one of its endpoint nodes in `native_node_ids` does not thereby reference that node for filter purposes.

Diagnostic correlations remain zero-based references into global diagnostics. Intersecting, differencing, or counting them establishes neither seed custody nor causation. Multiple seed correlations do not turn one diagnostic into independent observations or a shared semantic outcome.

## Duplicate and resolution rules

Admission rejects:

- duplicate labels;
- duplicate global identities within one namespace;
- duplicate references within one seed and namespace;
- references that resolve to zero or multiple global records;
- diagnostic indexes outside the global diagnostic array.

No first-occurrence or last-occurrence repair is performed. Malformed input fails closed rather than being silently deduplicated.

Every valid reference receives a canonical global ordinal from the admitted global collection for its namespace. That ordinal controls emitted partition order.

## Pairwise set semantics

For namespace `N` and selected seeds `L` and `R`, let `Refs(N,S)` be the admitted mathematical set of explicit typed references for seed `S`.

Define the pairwise universe:

```text
U(N,L,R) = Refs(N,L) ∪ Refs(N,R)
```

Define partitions:

```text
shared(N)     = Refs(N,L) ∩ Refs(N,R)
left_only(N)  = Refs(N,L) − Refs(N,R)
right_only(N) = Refs(N,R) − Refs(N,L)
```

The partitions are pairwise disjoint:

```text
shared(N) ∩ left_only(N) = ∅
shared(N) ∩ right_only(N) = ∅
left_only(N) ∩ right_only(N) = ∅
```

They exhaust only the pairwise universe:

```text
shared(N) ⊎ left_only(N) ⊎ right_only(N) = U(N,L,R)
```

Global records referenced by neither selected seed are outside the pairwise universe and are not emitted.

References in every partition follow their canonical global ordinal. The two selected seed summaries follow CLI operand order. No map iteration determines serialized order.

## Failed and empty seed semantics

Admission requires:

```text
failed(seed) ⇒ every filterable reference collection is empty
```

The projection distinguishes:

```text
FAILED
SUCCESSFUL_EMPTY
SUCCESSFUL_WITH_EVIDENCE
```

where:

```text
SUCCESSFUL_EMPTY(seed)
  ⇔ preparation_status = SUCCEEDED
  ∧ every filterable reference set is empty

SUCCESSFUL_WITH_EVIDENCE(seed)
  ⇔ preparation_status = SUCCEEDED
  ∧ at least one filterable reference set is nonempty
```

Existing `with-membership` and `without-membership` accounting remains retained input context but does not define pairwise emptiness, because a seed may have membership records while some reference namespaces are empty.

Comparing a failed or successful-empty seed is valid. Its empty partitions must not be interpreted as evidence that the other seed represents a distinct feature.

## Output contract

Define an independent Draft 2020-12 family:

```text
lsp-trace.filter.v1
```

The output shape is fixed for the only v1 mode:

```json
{
  "filter_schema_version": "lsp-trace.filter.v1",
  "projection_kind": "SEED_EVIDENCE_COMPARISON",
  "authority": "TOOL_DERIVED_SET_PROJECTION",
  "support_contribution": 0,
  "native_semantics_policy": "PRESERVE_WITHOUT_AUTHORITY_UPGRADE",
  "input_identity": {
    "inspection_exact_bytes_digest": "sha256:...",
    "artifact_semantic_commitment_digest": "sha256:...",
    "artifact_exact_serialized_bytes_digest": "sha256:...",
    "execution_bundle_id": "sha256:..."
  },
  "operands": {
    "left_seed_label": "LEFT_LABEL",
    "right_seed_label": "RIGHT_LABEL"
  },
  "seeds": [
    {
      "label": "LEFT_LABEL",
      "state": "SUCCESSFUL_WITH_EVIDENCE",
      "failure": null
    },
    {
      "label": "RIGHT_LABEL",
      "state": "SUCCESSFUL_WITH_EVIDENCE",
      "failure": null
    }
  ],
  "partitions": {
    "nodes": {"shared": [], "left_only": [], "right_only": []},
    "call_relations": {"shared": [], "left_only": [], "right_only": []},
    "dispatch_relationships": {"shared": [], "left_only": [], "right_only": []},
    "sibling_candidates": {"shared": [], "left_only": [], "right_only": []},
    "diagnostic_correlations": {"shared": [], "left_only": [], "right_only": []}
  },
  "global_boundary": {
    "truncated": false,
    "traversal_complete": true,
    "source_graph_complete": "UNKNOWN"
  },
  "accounting": {},
  "claim_ceiling": {
    "supports": [
      "EXACT_REFERENCE_INTERSECTION",
      "EXACT_REFERENCE_DIFFERENCE"
    ],
    "does_not_support": [
      "SHARED_FEATURE_PURPOSE",
      "DISTINCT_FEATURE_PURPOSE",
      "FEATURE_IDENTITY",
      "WORKFLOW_IDENTITY",
      "MERGE_OR_SPLIT_DISPOSITION",
      "INDEPENDENT_OBSERVATION",
      "EVIDENTIARY_SUPPORT",
      "CONFIDENCE",
      "COVERAGE",
      "RUNTIME_BEHAVIOR",
      "ACCEPTANCE"
    ]
  }
}
```

Every array is present, including empty arrays. `additionalProperties: false` applies throughout the filter schema except where copied failure details require a separately closed native-compatible definition.

The output contains references only. Consumers join them to the admitted inspection input. It contains no copied node, call-relation, dispatch, sibling, membership, or diagnostic records.

### Input identity ceiling

`inspection_exact_bytes_digest` identifies the exact inspected projection bytes. Artifact identities are copied from the admitted inspection projection and retain their existing meanings. Their presence does not assert selector custody or producer authenticity.

`execution_bundle_id` is optional because `lsp-trace.inspect.v1` permits it to be absent. When the admitted inspection contains a valid digest, the filter copies it byte-for-byte; when absent, the filter omits it. The filter never invents, normalizes, or substitutes an execution-bundle identity. A present malformed value fails closed during filter output validation.

`input_identity` has a closed schema. No input-origin or custody-verification boolean is emitted because v1 accepts inspection documents only and cannot reconstruct their prior custody path.

## Accounting

Common input-context counts are copied only when independently recomputed during admission:

```text
requested_seed_count
successful_seed_count
failed_seed_count
successful_seed_with_membership_count
successful_seed_without_membership_count
```

The comparison emits, for each namespace `N`:

```text
left_reference_count(N)       = |Refs(N,L)|
right_reference_count(N)      = |Refs(N,R)|
shared_reference_count(N)     = |shared(N)|
left_only_reference_count(N)  = |left_only(N)|
right_only_reference_count(N) = |right_only(N)|
pair_universe_count(N)        = |U(N,L,R)|
```

Required equations are:

```text
shared_reference_count(N) + left_only_reference_count(N)
  = left_reference_count(N)

shared_reference_count(N) + right_only_reference_count(N)
  = right_reference_count(N)

shared_reference_count(N)
  + left_only_reference_count(N)
  + right_only_reference_count(N)
  = pair_universe_count(N)
```

The output does not use a generic `matched_record_count` or `selected_seed_count`; those names have ambiguous meanings across modes.

Counts are mechanical reconciliation only. They do not express evidence weight, semantic importance, feature coverage, confidence, or acceptance.

## Authority composition

`authority: TOOL_DERIVED_SET_PROJECTION` and `claim_ceiling` apply only to newly derived partitions, ordering, state classification, and counts.

They do not replace, summarize, upgrade, or downgrade native evidence authority. Referenced native records retain their existing v3 evidence class, evidence role, support contribution, provenance, execution-bundle identity, and semantic ceiling.

A listed seed reference means only that the admitted per-seed projection explicitly contained that reference under current `inspect.v1` attribution rules. It does not mean the seed independently observed, caused, executed, semantically depended on, or shared the purpose represented by the record. References from one execution bundle remain non-independent.

Shared evidence is a review signal, not proof of shared feature identity. Exclusive evidence is not proof of distinct feature identity. Absence of overlap may reflect failed seeds, configured bounds, language-server limitations, dynamic behavior, or semantic relationships unavailable to Call Hierarchy.

## Determinism and read-only behavior

Identical inspection bytes and ordered filter operands produce identical output bytes.

Reversing operands changes only:

- `left_seed_label` and `right_seed_label`;
- selected seed order;
- `left_only` and `right_only` arrays and their corresponding counts.

It does not change `shared`, the global boundary, input identity, or common input-context accounting.

The command:

- starts no language server;
- reads no mutable workspace source;
- modifies no inspection input;
- emits no graph, selector, generation, custody receipt, or replacement inspection projection;
- writes JSON only to stdout after all validation succeeds.

## Schema retrieval and validation

A stable public `lsp-trace.filter.v1` contract requires family-aware schema APIs.

Add:

```bash
lsp-trace schema get --family graph --version v3
lsp-trace schema get --family inspect --version v1
lsp-trace schema get --family filter --version v1

lsp-trace validate --family graph --version v3 PATH|-
lsp-trace validate --family inspect --version v1 PATH|-
lsp-trace validate --family filter --version v1 PATH|-
```

Existing graph syntax remains a compatibility alias:

```bash
lsp-trace schema get --schema v1|v2|v3
lsp-trace validate [--schema v1|v2|v3] PATH|-
```

Bare `v1` is never overloaded across families.

Filter and inspection validation include their family-specific semantic/accounting/reference validators after structural validation. Structural validation precedes deeper semantic validation for every family.

## Compatibility

This decision does not change:

- `lsp-trace.graph.v1`, v2, or v3;
- graph production or traversal;
- publication receipts;
- existing `inspect`, `verify`, or graph-validation behavior;
- `lsp-trace.inspect.v1` wire shape or authority.

It adds:

- family-aware schema retrieval and validation;
- reusable semantic admission for existing all-seed inspection documents;
- one `lsp-trace.filter.v1` projection kind.

`lsp-trace.filter.v1` is a stable compatibility surface. Removing fields, changing requiredness, altering namespace keys, changing partition definitions, changing ordering, or changing claim ceilings requires a new filter major version.

The v1 schema is closed. Additive optional fields require updating the committed schema and deterministic omission/presence rules; consumers must accept schema-valid optional additions. Additions cannot increase authority or alter existing set membership.

## Explicitly external

LSP Trace does not own:

- feature or workflow identity or naming;
- merge, split, duplicate, supersession, or retirement decisions;
- interpretation of overlap as shared purpose;
- interpretation of exclusivity as distinct purpose;
- user, operator, or business outcomes;
- production use or runtime reachability;
- evidence sufficiency, weighting, confidence, or ranking;
- inventory construction or coverage;
- product or domain acceptance.

## Deferred operations

The following are not promised by `lsp-trace.filter.v1`:

- `--shared-by-min`;
- reverse lookup by evidence ID;
- seed-state filtering as an independent mode;
- direct graph or selector input;
- stdin input;
- copied native records;
- membership-record partitions;
- generic field expressions;
- path-prefix or diagnostic-message filtering;
- BM25, embeddings, lexical similarity, clustering, or ranking;
- feature candidate generation;
- merge/split recommendations.

Each requires independent demand and a later decision.

## Rejected alternatives

### Put filters inside `inspect`

Rejected because `inspect` is a stable admission/projection contract. A separate command prevents selection modes from multiplying its compatibility surface.

### Accept graph, selector, and inspection inputs in v1

Rejected because three admission paths create custody, equivalence, and testing complexity before pairwise demand validates the command.

### Copy native records into pairwise output

Rejected because references are sufficient for exact comparison, avoid duplication, and preserve one admitted source of native record truth.

### Compare full membership records

Rejected because `inspect.v1` defines no cross-seed canonical membership identity. Endpoint similarity cannot substitute for identity.

### Add several filter modes immediately

Rejected because each adds selectors, output shapes, accounting, errors, and compatibility costs before demonstrated need.

### Add lexical or automatic grouping

Rejected because those operations introduce interpretation and may be mistaken for feature identity or adjudication.

## Implementation sequence

### Phase 1: Reusable inspection admission

1. Extract all-seed inspection types from command-local code without changing `inspect.v1` output.
2. Add structural decoding and reusable semantic/accounting/reference validation.
3. Reject duplicate labels, global identities, and per-seed references.
4. Validate typed dispatch and sibling references through same-label memberships.
5. Add mutation tests for every admission invariant.

### Phase 2: Pairwise projection

1. Add `filter` parsing and fail-fast mode validation.
2. Build separate typed reference indexes.
3. Implement pairwise set partitions in global native order.
4. Preserve selected seed states and global boundaries.
5. Recompute and validate namespace accounting.
6. Validate output structurally before emission.

### Phase 3: Schema APIs and operations

1. Add family-aware schema retrieval and validation while preserving graph aliases.
2. Add exact usage/help and error guards.
3. Update README, semantics, release checks, and embedded skill.
4. Run a real `ALL_SEEDS` projection smoke test without starting a server.

## Test strategy

Persistent tests must cover:

- CLI mode validation precedes input reads;
- help exits `0` with stdout usage and empty stderr;
- exactly two repeated, distinct, known labels;
- inspection-only input-family discrimination;
- single-seed, graph, selector, and ambiguous inputs fail closed;
- inspection validation does not assert selector custody;
- unique seed labels and unique global typed keys;
- duplicate per-seed references fail closed;
- every reference resolves exactly once in its typed namespace;
- equal raw IDs in different namespaces never cross-match;
- no reference is inferred through endpoints or another namespace;
- diagnostic indexes remain in range and correlation-only;
- failed seeds have empty filterable references;
- successful-empty and failed states remain distinct;
- membership records are not partitioned;
- every namespace partition is disjoint and exhaustive over its pairwise universe;
- global native ordinal controls reference order;
- operand reversal swaps only left/right results;
- every accounting equation recomputes from emitted arrays;
- family-aware schema retrieval returns exact committed bytes;
- structural validation precedes semantic validation;
- identical input bytes and operands produce identical output bytes;
- input bytes remain unchanged;
- no server starts and no graph or receipt is published;
- envelope authority cannot replace or strengthen native evidence authority;
- claim ceilings reject feature/workflow identity and merge/split disposition;
- output contains no feature candidate, confidence, ranking, coverage, or acceptance fields.

Counterfactual tests must independently perturb:

- one partition member;
- one namespace tag;
- one duplicate reference;
- one global ordinal;
- one accounting field;
- one authority constant;
- one required claim-ceiling exclusion;
- one schema-family/version constant.

## Consequences

### Positive

- The first release directly answers exact pairwise overlap questions.
- The compatibility surface has one input family, mode, and output shape.
- Set identity, ordering, duplicates, and accounting are mechanically defined.
- Failed and empty evidence remain visible.
- Native evidence authority remains unchanged.
- Feature adjudication remains external.

### Negative

- Callers must first produce and retain an `ALL_SEEDS` inspection document.
- Reusable inspection semantic validation is prerequisite work.
- Family-aware schema APIs broaden the existing schema command.
- Consumers must join references back to the admitted inspection input.
- Broader filtering requires later decisions and versions.

### Neutral

- Pairwise filtering does not improve language-server completeness.
- It does not authenticate source, producer, execution, or environment.
- It does not identify semantic similarity without exact shared references.
- It does not determine whether workflows should merge or split.

## Acceptance gates

The implementation proceeded after maintainers affirmed:

1. pairwise-only v1;
2. inspection-only input;
3. references-only output;
4. no membership-record partitions;
5. stable `lsp-trace.filter.v1` plus family-aware schema APIs;
6. the typed-reference and claim-ceiling vocabularies above.

## Resumption snapshot

Current accepted state:

- ADR narrowed from four modes to one pairwise comparison;
- input narrowed from three families to `lsp-trace.inspect.v1` `ALL_SEEDS` only;
- output narrowed to typed references only;
- pairwise universe, identity, duplicate, ordering, failed/empty, accounting, custody, and authority rules are explicit;
- stable schema retrieval and validation are family-aware;
- all broader filtering, lexical retrieval, and feature adjudication are deferred;
- implementation and durable semantic-validator guards are present;
- the six acceptance gates above define the accepted compatibility boundary.
