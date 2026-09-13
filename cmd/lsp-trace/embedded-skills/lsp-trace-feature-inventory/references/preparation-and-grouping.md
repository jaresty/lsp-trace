# Preparation and reversible grouping

## Retain the evidence packet

Retain one transport-neutral set containing:

- asserted repository paths and revisions;
- included/excluded source scope and caller-owned census inputs;
- workspace, profile/server, language, and position-encoding configuration;
- seed labels, exact positions, and investigation reasons;
- traversal and resource bounds;
- selector, artifact, capture-set, inspection, filter, community, and instability bytes/receipts as applicable;
- stdout, stderr, exit status, diagnostics, failed and successful-empty seeds;
- verification results when run;
- unresolved authority and completeness limits.

Caller-supplied revisions are assertions, not source authentication. Preserve every round rather than replacing prior evidence.

## Reconcile mechanical state

Require:

```text
requested_seed_count = successful_seed_count + failed_seed_count
successful_seed_count = successful_seed_with_membership_count + successful_seed_without_membership_count
```

Global record counts equal their arrays; per-seed reference counts equal the corresponding collection lengths. Occurrence counts may exceed deduplicated records. Use each seed's own result and typed references. Never borrow another seed's closure or the union graph.

Treat `FAILED`, `SUCCESSFUL_EMPTY`, bounded zero, `UNKNOWN`, and untouched ambiguity as distinct explicit states. An unmatched census item is a lead, not a missing feature. A matching name is not identity.

## External candidate record

Keep candidates outside native evidence artifacts:

```yaml
candidate_id: stable-external-id
revision: 1
status: PROVISIONAL | SPLIT | MERGED | EXCLUDED | UNRESOLVED | ACCEPTED_BOUNDARY
working_label: non-canonical label
seed_labels: []
evidence:
  nodes: []
  calls: []
  dispatch_relationships: []
  sibling_candidates: []
  communities: []
  instability_observations: []
  diagnostic_correlations: []
missing_or_failed_evidence: []
competing_interpretations: []
contrary_evidence: []
rejection_condition:
authority_ceiling:
predecessors: []
inventory_state_delta:
revisit_condition:
```

Reference exact native IDs without copying and rewriting native records. Keep namespaces separate. The working label is non-canonical. Every non-`UNRESOLVED` disposition needs a contrary observation or falsifier. Splits and merges preserve predecessor IDs and finite deltas.

## Use Leiden communities as review nominations

**Headline: a community is a structurally notable set worth examining, never a feature.** Community membership does not establish semantic identity, purpose, value, production use, confidence, coverage, or acceptance.

Before grouping, perform an independent entry-point census rather than deriving the census from the captured graph. The documented CLI `census` command may supply that accountable enumeration and batched structural acquisition, but its authority remains `0`, source-graph completeness remains `UNKNOWN`, and its capture set has neither native aggregate custody nor inferred cross-capture `CALLS` nor direct Leiden admission. Acquire source-bearing evidence first; if required source bytes are unavailable, preserve that gap instead of treating structure as semantics. Keep the native directed graph intact and retain per-seed attribution rather than symmetrizing edges, flattening direction, or assigning the union graph to each seed.

Run Leiden at a fixed declared resolution with a deterministic seed. Treat the resulting partition as one bounded structural observation, then run A-08 instability campaigns using deterministic alternate seeds and declared perturbations. Record stable and unstable memberships without converting stability into feature confidence.

For every semantically relevant community member, prepare a source-complete evidence packet containing its exact source bytes and native typed evidence references. Split packets that are too large for bounded review or whose memberships remain unstable; do not hide unstable or omitted members behind a community label. Review technical behavior first. Only after that review ask task, feature, infrastructure, overlap, and split/merge questions.

Report explicit denominators for unmatched entry points, members lacking source bytes, failed acquisitions, successful-empty results, and candidates not assigned to a community. Recapture only bytes that are missing, mismatched, or unavailable; reuse matching retained bytes rather than reacquiring an entire packet.

## Form grouping hypotheses

A reversible grouping hypothesis records:

- a provisional bounded claim;
- exact supporting record IDs and namespaces;
- missing/failed evidence;
- competing interpretations;
- contrary evidence or a rejecting observation;
- the withdrawal/split action;
- its authority ceiling and revisit condition.

Shared callers, overlap, labels, mechanical counts, communities, centrality, or low instability do not merge candidates. Community membership is a review arrangement. Instability is sensitivity of a structural result to declared perturbation. Neither contributes semantic support or confidence.

Capture-set co-membership says only that exact constituents were composed. It adds no cross-capture CALLS and no native single-capture custody.

## Coverage-gap ledger

Review failed seeds, successful seeds without membership, accounting mismatches, census symbols absent from candidates, frontier/terminal nodes, active bounds, unsupported capabilities, uninvestigated nominations, isolated evidence regions, and dynamic/generated/reflective/configuration/template/framework relationships that Call Hierarchy may omit.

Classify each gap as `INVESTIGATE`, `ACCEPT_BOUNDARY`, `EXTERNAL_ADJUDICATION`, or `UNRESOLVED`. Mechanical preparation stops before feature acceptance.
