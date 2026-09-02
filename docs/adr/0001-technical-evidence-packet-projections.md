# ADR 0001: Add versioned all-seed inspection projections

- **Status:** Accepted
- **Date:** 2026-09-02
- **Decision owners:** LSP Trace maintainers
- **Scope:** Read-only inspection and projection of existing `lsp-trace.graph.v3` evidence
- **Related commands:** `slice`, `incoming`, `validate`, `verify`, `inspect`

## Context

`lsp-trace` produces bounded, attributable evidence from language-server Call Hierarchy, Type Hierarchy, and document-symbol observations. Its v3 graph retains requested seeds, same-label results, causal memberships, separate call and discovery namespaces, traversal boundaries, diagnostics, and semantic and exact-byte identities.

The existing command:

```bash
lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json
```

projects one seed without creating a replacement graph. A consumer inspecting every seed must still enumerate labels, preserve failures, invoke the command repeatedly, join projections with global evidence, and calculate basic accounting. Reimplementing these deterministic operations downstream risks:

- dropping failed seeds;
- attributing the union graph to every seed;
- reconstructing memberships incorrectly;
- duplicating shared records as independent observations;
- confusing global diagnostics with exact per-seed custody;
- bypassing selector validation.

`lsp-trace` must not absorb feature-inventory policy. Native evidence cannot decide feature identity, user purpose, runtime use, canonical workflow boundaries, product value, priority, lifecycle, or acceptance.

## Decision

Extend `inspect` with `--all-seeds` and define one versioned inspection family, `lsp-trace.inspect.v1`, for single-seed and all-seed projections.

Phase 1 does not introduce a separate evidence-packet schema, `--format evidence-packet`, or `--artifact-info`. Those conveniences remain deferred until independent consumer demand justifies their compatibility cost.

The implementation does not change graph production, graph schemas, traversal semantics, semantic commitments, publication receipts, or the authority of `validate` and `verify`.

## CLI contract

Support:

```bash
lsp-trace inspect INPUT --seed LABEL --json
lsp-trace inspect INPUT --all-seeds --json
```

`--seed` and `--all-seeds` are mutually exclusive. Exactly one is required.

Both modes emit the independently versioned `lsp-trace.inspect.v1` family with authority `NON_AUTHORITATIVE_DERIVED_VIEW`. Compatibility is additive rather than shape-identical: single-seed output preserves its historical top-level fields and uses `projection_kind: "SEED_INSPECTION"`; all-seed output uses `projection_kind: "ALL_SEEDS"` with `records`, `seeds`, and `accounting` sections.

`--all-seeds` returns every stored `invocation.seeds` entry in stored order, including failed seeds. The implementation must not derive the seed list from successful graph members. Each invocation seed must resolve to exactly one same-label `seeds` result; missing or duplicate results fail closed.

The existing `--json` flag remains accepted even though JSON is currently the only format. No `--format` flag is added until a second representation exists.

## Inspection model

### Artifact identity

The projection retains existing native identities:

```text
execution_bundle_id
semantic_commitment_digest
exact_serialized_bytes_digest
```

Equivalent artifact and selector inputs must project identical native evidence. Default inspection output does not add selector-relative paths or input-origin metadata that would break this equivalence.

For direct artifacts, the exact-byte digest is computed from supplied bytes and makes no custody claim. Selector input must pass native custody verification before projection, but selector-specific details remain outside the default evidence projection.

### Copied-once native records

The aggregate projection copies each already-admitted native record once:

```json
{
  "records": {
    "nodes": [],
    "call_relations": [],
    "dispatch_relationships": [],
    "sibling_candidates": [],
    "diagnostics": [],
    "terminals": [],
    "frontier": []
  }
}
```

Per-seed sections reference canonical records by existing native identity. They do not duplicate shared native records or assign new evidence identities.

Records retain canonical artifact order. Shared records may be referenced by multiple seeds without becoming independent observations.

### Per-seed evidence

Each entry contains:

```text
stored seed request and label
preparation status
failure phase and message
prepared target IDs
reached node IDs
reached relation IDs
exact same-label seed_memberships
native record references
derived diagnostic references
```

Failed seeds remain present with empty memberships and native-record references.

Derived diagnostic references select global diagnostics whose nonempty `node_id` occurs in the seed's reached node IDs. They retain authority:

```text
TOOL_DERIVED_NODE_CORRELATION
```

Correlation does not establish exact per-seed diagnostic custody or causation. Unattributed diagnostics remain global only.

### Native evidence semantics

The inspection does not author a second claim-ceiling vocabulary. It preserves and references existing native v3 semantics, including:

```text
NON_AUTHORITATIVE_DERIVED_VIEW
TOOL_DERIVED_NODE_CORRELATION
source_graph_complete
evidence_semantics
evidence_kind
support_contribution
```

A call relation supports only its reported endpoints and direction. Dispatch and sibling records remain discovery nominations. Derived locators do not prove behavior. Traversal completeness remains relative to the configured server and bounds. Integrity and custody do not establish authenticity, source truth, runtime execution, or feature identity.

## Machine-readable accounting

The inspection includes deterministic counters whose names identify what is counted:

```text
requested_seed_count
successful_seed_count
failed_seed_count
successful_seed_with_membership_count
successful_seed_without_membership_count
global_node_record_count
global_call_relation_record_count
global_dispatch_relationship_record_count
global_sibling_candidate_record_count
global_diagnostic_record_count
global_terminal_record_count
global_frontier_record_count
seed_membership_record_count
seed_node_reference_count
seed_call_relation_reference_count
seed_discovery_nomination_reference_count
seed_correlated_diagnostic_reference_count
truncated
traversal_complete
source_graph_complete
```

Record counters equal copied global collection sizes. Reference counters count per-seed occurrences and may exceed global counts when records are shared.

A failed seed is distinct from a successful seed with empty membership. Counter definitions must preserve that distinction.

These fields describe retained evidence. They do not determine evidence sufficiency, feature coverage, confidence, priority, or acceptance.

## Validation and custody

Inspection reuses existing authoritative mechanisms rather than adding parallel validators.

For direct artifacts:

```text
read exact bytes
→ Draft 2020-12 structural validation
→ v3 semantic validation
→ projection
```

For selectors:

```text
strict selector decoding
→ selected-generation resolution
→ strict custody-receipt validation
→ exact-byte verification
→ Draft 2020-12 structural validation
→ v3 semantic validation
→ projection
```

Structural validation precedes semantic validation. Unknown seeds, malformed artifacts, invalid semantics, incomplete generations, malformed receipts, and digest mismatches fail closed with no JSON projection.

## Determinism and read-only behavior

- Seeds retain stored `invocation.seeds` order.
- Memberships, nodes, relations, boundaries, and diagnostics retain canonical artifact order.
- No map iteration determines serialized array order.
- Identical artifact bytes and options produce identical projection bytes.
- Equivalent artifact and selector inputs produce identical native evidence projections.
- Inspection starts no language server.
- Inspection reads no mutable workspace source.
- Inspection modifies no artifact, selector, generation, or receipt.
- Inspection creates no graph or custody receipt.
- Failure writes no JSON projection to stdout.

## Authority boundary

### Owned by `lsp-trace`

`lsp-trace` owns deterministic projection of:

- validated native graph records;
- exact stored seed state;
- exact same-label memberships;
- global boundaries and diagnostics;
- derived node correlation;
- mechanical record and reference counts;
- existing semantic and exact-byte identities;
- existing native evidence authority classifications.

### Explicitly external

`lsp-trace` does not own:

- Git revision authentication or repository-denominator accounting;
- downstream evidence-capsule construction;
- independent execution, environment, binary, transcript, or source authentication;
- independent semantic-recomputation verification;
- proposition admission or verdicts;
- technical-workflow boundary adjudication;
- downstream candidate-record or mapping-assessment schemas;
- canonical feature identity, naming, or correspondence;
- actor, objective, trigger, or terminal-outcome interpretation;
- duplicate, split, merge, supersession, or retirement decisions;
- production-use, value, priority, privacy, parity, or scope claims;
- feature-inventory coverage or acceptance policy;
- product/domain acceptance.

An inspection projection is not an independently authenticated execution specimen. Native selector custody and v3 validation cannot substitute for external source, execution, environment, binary, transcript, or semantic-recomputation verification.

A projection may be referenced by downstream candidate records, but it does not assign candidate identity, workflow boundary, proposition verdicts, mapping relations, or feature correspondence. Multiple candidates may reference one projection, and one candidate may reference multiple projections.

Recorded source revisions remain caller assertions unless authenticated externally. Projection generation neither establishes a pinned repository denominator nor proves that mutable workspace contents matched the source claim.

## Rejected alternatives

### Import the complete feature-inventory process

Rejected because it would couple a language-neutral tracing tool to one consumer's ontology, admission rules, and authority model.

### Add a separate evidence-packet format in Phase 1

Rejected because all-seed inspection already supplies the required packet. A `--format` selector creates compatibility cost without a second representation.

### Duplicate native records per seed

Rejected because shared records would inflate output and could be mistaken for independent observations.

### Generate, group, or rank feature candidates

Rejected because those operations require semantic and product judgments absent from native evidence. Graph overlap alone cannot distinguish one feature, several features, shared infrastructure, or incidental reuse.

### Add proposition verdicts

Rejected because verdicts depend on consumer-defined propositions and admission policy.

### Extend `verify` with membership expectations

Rejected because `verify` establishes integrity and custody. Caller-specific membership expectations are policy.

### Add `--artifact-info` in Phase 1

Deferred because it introduces path sensitivity, publication-layout exposure, richer custody-loader state, and another command mode without demonstrated need beyond existing identities.

## Compatibility

No changes are made to:

- `lsp-trace.graph.v1`;
- `lsp-trace.graph.v2`;
- `lsp-trace.graph.v3`;
- v3 semantic commitments;
- publication receipts;
- traversal output;
- existing validation or verification contracts.

Existing syntax remains valid:

```bash
lsp-trace inspect INPUT --seed LABEL --json
```

Single-seed compatibility is additive: `inspection_schema_version` is added while historical top-level fields remain in place. The aggregate envelope is selected only by `--all-seeds`; it does not replace the single-seed shape. Tests guard both forms.

The inspection contract is versioned independently as `lsp-trace.inspect.v1`. It is not an `lsp-trace.graph.v3` document and does not carry a graph `schema_version`. Historical v3 artifacts accepted by current structural and semantic validation remain inspectable; omitted historical fields remain absent rather than being fabricated.

## Implementation record

The accepted implementation:

1. preserves single-seed top-level compatibility additively;
2. provides mutually exclusive `--seed` and `--all-seeds` modes;
3. iterates stored invocation seeds and requires exactly one same-label result;
4. preserves failed seeds with empty memberships and references;
5. copies admitted native records once and emits normalized per-seed reference collections;
6. keeps global diagnostics separate from derived node correlation;
7. recomputes and validates every mechanical accounting count before emission;
8. validates both projection kinds against an embedded independent Draft 2020-12 schema before writing JSON;
9. documents the native workflow and its external adjudication boundary in the embedded skill.

### Deferred

Do not add `--format evidence-packet`, a second packet schema, or `--artifact-info` without a later decision supported by independent demand.

## Embedded-skill feature-inventory guide

The embedded skill explains how to use native evidence as technical input to a feature inventory without claiming that `lsp-trace` constructs or accepts feature identity.

The guide teaches:

1. **Declare source scope externally.** Pin and record repositories and revisions outside `lsp-trace`. Treat recorded revisions as caller claims until externally authenticated.
2. **Choose labeled technical roots.** Use `slice --from-file` for a bounded file census, repeated `--at` or `--seed-file` for curated starts, and `incoming` when outgoing discovery is unnecessary.
3. **Capture bounded evidence.** Set explicit depth, node, request, and global time limits. Publish v3 output and preserve stderr and status.
4. **Retain incomplete evidence honestly.** Traversal status is command-specific: status `2` carries structured incomplete evidence; interrupted incoming status `130` may still emit or publish evidence; publication status `1` may retain marshaled graph stdout. None should be relabeled as completed traversal.
5. **Admit evidence.** Selector inspection performs its own custody gate; standalone `verify` is an explicit selector audit. Direct artifact digests make no custody claim.
6. **Inspect all stored seeds.** Run `lsp-trace inspect SELECTOR --all-seeds --json` and retain failures, exact memberships, copied-once native records, normalized references, accounting, boundaries, and diagnostics.
7. **Interpret native evidence narrowly.** Do not borrow another seed's closure, treat transitive paths as direct support, treat discovery nominations as call evidence, or convert derived diagnostic correlation into causation.
8. **Form falsifiable reversible technical hypotheses.** Retain exact supporting IDs, missing evidence, competing interpretations, contrary evidence or a rejection observation, and the withdrawal or split action.
9. **Stop before feature adjudication.** External systems and human reviewers own proposition verdicts, candidate and mapping schemas, feature identity, naming, value, priority, lifecycle, and acceptance.

The guide includes:

```bash
lsp-trace slice ... --output evidence.selector.json
lsp-trace verify evidence.selector.json
lsp-trace inspect evidence.selector.json \
  --all-seeds \
  --json > evidence-inspection.json
```

It retains these warnings:

> Inspection is not authentication, normalization is not evidence admission, and record presence is not proposition validity.

> Mechanical counts are not evidentiary weight, feature coverage, candidate disposition, or inventory acceptance.

> `lsp-trace` produces bounded, attributable technical evidence; downstream systems and human reviewers decide whether and how that evidence corresponds to features.

The skill does not include downstream authentication schemas, consumer-specific admission rules, proposition-vocabulary definitions, workflow-adjudication rules, candidate or mapping schemas, feature-record lineage, or accepted-inventory procedures.

## Test strategy

Development is assertion-first.

### Input and custody

- Direct artifact and selector inputs are accepted.
- Equivalent artifact and selector inputs project identical native evidence.
- Malformed selectors, incomplete generations, malformed receipts, metadata mismatches, byte mismatches, structural failures, and semantic failures are rejected.
- Structural validation precedes semantic validation.

### Modes and compatibility

- Existing single-seed behavior remains compatible according to the selected migration contract.
- `--seed` and `--all-seeds` are mutually exclusive.
- Exactly one mode is required.
- Both modes identify `lsp-trace.inspect.v1` when the versioned envelope is selected.
- Invalid combinations fail with usage errors and empty stdout.

### Seeds and membership

- Every stored invocation seed appears exactly once in all-seed output.
- Successful seeds retain exact memberships.
- Failed seeds remain with empty memberships and references.
- Missing or duplicate same-label results fail closed.
- Relations are never borrowed from another seed.
- Shared native records may be referenced by multiple seeds.

### Copied records, normalized references, and diagnostics

- Every admitted native record occurs once in copied global collections.
- Per-seed sections reference rather than duplicate records.
- Global diagnostics remain global.
- Only nonempty matching node IDs enter derived correlation.
- Correlation authority remains `TOOL_DERIVED_NODE_CORRELATION`.
- Unattributed diagnostics remain global only.

### Accounting

- Record counters equal copied collection lengths.
- Reference counters equal per-seed reference totals.
- Successful-empty and failed seeds remain distinct.
- Counts reconcile with native artifact evidence.
- No field claims feature coverage or evidence sufficiency.

### Determinism, read-only behavior, and authority

- Repeated invocations produce identical bytes.
- Artifacts, selectors, and generations remain unchanged.
- No language server starts and no graph or receipt is written.
- Projection authority remains `NON_AUTHORITATIVE_DERIVED_VIEW`.
- Inspection output contains no graph `schema_version`.
- Direct artifacts make no selector-custody claim.
- Output contains no feature identity, confidence, priority, acceptance, or proposition-verdict fields.

## Release guards

Add assertions that:

- help advertises `--all-seeds`;
- help does not advertise deferred `--format evidence-packet` or `--artifact-info`;
- `--seed` and `--all-seeds` are mutually exclusive;
- the chosen single-seed compatibility contract remains guarded;
- versioned output identifies `lsp-trace.inspect.v1`;
- copied records and normalized per-seed references reconcile with precise counters;
- README distinguishes selector custody from direct-artifact digest computation;
- semantics distinguish global diagnostics from derived correlation;
- no graph schema references the inspection projection;
- release binaries emit deterministic single-seed and all-seed fixture projections;
- the embedded skill contains the native three-command workflow;
- the skill preserves external source-scope, authentication, and feature-identity boundaries.

Release checks must not start or install a language server.

## Consequences

### Positive

- Consumers receive one deterministic all-seed projection without custom joins.
- Failed seeds cannot silently disappear.
- Cross-seed contamination becomes less likely.
- Shared native records remain deduplicated.
- Global and derived diagnostics remain visibly distinct.
- The embedded skill becomes shorter and executable.
- `lsp-trace` remains domain-neutral.
- Downstream consumers receive cleaner native input without weakening their external authority layers.

### Negative

- `lsp-trace.inspect.v1` becomes a compatibility surface.
- Future field changes require inspection-schema versioning discipline.
- Normalized references require consumers to join IDs to records.
- Maintainers must prevent convenience fields from drifting into consumer policy.

### Neutral

- The projection does not improve language-server completeness.
- It does not authenticate source revisions or execution.
- It does not resolve downstream semantic-recomputation or authentication blockers.
- It does not determine workflow or feature boundaries.
- Consumers still need proposition, mapping, and adjudication models.

## Criteria for later extensions

A later convenience belongs in `lsp-trace` only when all are true:

1. It is deterministically derivable from validated native evidence.
2. It requires no product or consumer semantics.
3. Its authority does not increase the native claim ceiling.
4. More than one consumer would otherwise implement the same mechanical operation.
5. It remains read-only or preserves existing publication semantics.
6. It does not turn `verify` into a policy validator.

Operations that fail any criterion remain downstream.
