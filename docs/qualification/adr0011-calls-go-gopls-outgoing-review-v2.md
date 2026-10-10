# ADR 0011 Go/gopls outgoing CALLS qualification-plan review v2

**Reviewed profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
**Review scope:** the four frozen outgoing author files only
**Decision:** `ADR0011_CALLS_OUTGOING_PLAN_GO`
**Profile/evidence/production disposition:** blocked; `REFUSED_MISSING_EVIDENCE`

## Independent review

The corrected four-file plan is coherent and exact enough to proceed to a separately authorized future phase. This decision approves only the qualification plan. It does not produce evidence, authorize implementation or semantic-vector execution, satisfy a manifest role, qualify the profile, activate production, compose directions, authorize shared Stage 3–7 work, or issue profile GO.

### Successful top-level null

The prior review blocker is resolved consistently across the profile plan, causal tests, evidence manifest, and vector inventory:

- a terminally successful, complete outgoing response whose present result token is exactly JSON `null` is `SUCCESSFUL_EMPTY`;
- its exact null token/bytes, response ordinal, and source identity remain retained and distinct from `[]`;
- it closes one begun expansion as successful and retains actual request/message/byte/time/work use;
- it produces zero members, ranges, occurrences, rejected members/ranges, and production-visible effects;
- its canonical CALLS occurrence representation is the same empty set as successful `[]`, without aliasing source custody;
- missing payload is evidence-missing, transport/protocol error is failed, and malformed or partial/truncated input is atomic refusal with no prefix salvage;
- member-level missing/null `fromRanges` remains malformed and is not confused with successful top-level null.

O17/O18, O-P01/O-P01N/O-P11, O-N10 through O-N14, and the manifest policy/accounting fields provide explicit positive, contrast, replay, canonicalization, and failure vectors for this distinction.

### Remaining plan requirements

The outgoing plan remains internally aligned on:

- separate prepare request/result and outgoing request/result custody;
- deterministic target-bound prepared-caller selection and unchanged dispatch identity;
- caller-to-callee normalization and caller-side `fromRanges` semantics;
- multiplicity, duplicate, parallel-edge, self-loop, and source-ordinal preservation;
- atomic refusal for malformed, partial, substituted, reordered, stale, foreign, copied, resealed, digest-only, cross-profile, or synthetic inputs;
- finite independent ceilings, exact-limit/plus-one behavior, checked arithmetic, closed accounting, and zero effects before refusal;
- offline replay without filesystem, process, workspace, clock, LSP callback, retry, hidden reacquisition, or custody aliasing;
- exact historical omitted-selector `CALLS_ONLY` compatibility and isolation from incoming, bidirectional, alias, case-varied, missing, or empty selectors;
- shared-artifact references requiring independent immutable identity, relevance, freshness, and exact outgoing-profile binding, with no inherited authority;
- semantic-vector’s immutable future order, independently authored and reviewed oracle, same-adapter controlled contrast, exact comparison, bounded receipt, disagreement block, and inability to establish gopls evidence truth, profile policy, authority, admission, review, or GO.

### Manifest honesty

The manifest remains a planned, unexecuted description: `authority: 0`, `accepted: false`, `completeness: UNKNOWN`, `present_role_count: 0`, and every required role is only `MISSING` or `PLANNED`. The corrected successful-null rule changes semantic classification only; it does not raise evidence, completeness, authority, review, admission, or GO.

## Decision boundary

`ADR0011_CALLS_OUTGOING_PLAN_GO`

Stop after this plan-review decision. The exact profile remains unqualified and production remains `REFUSED_MISSING_EVIDENCE`. Any evidence production, implementation, test execution, semantic-vector adapter or execution, profile qualification, production activation, bidirectional composition, or shared Stage 3–7 work requires new explicit authorization.
