# ADR 0011 Go/gopls outgoing CALLS plan — independent review

**Reviewed source commit:** `e2d7419612f3382172036d1fff86c10f2ddb44a3`  
**Scope:** the four outgoing author files only, compared with the accepted framework/incoming separation and source-visible outgoing acquisition behavior.  
**Disposition:** `ADR0011_CALLS_OUTGOING_PLAN_BLOCKED`

## Independent findings

The proposed identity `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` is direction-specific and correctly keeps qualification-plan review separate from evidence production, implementation, semantic-vector execution, profile qualification, production activation, and profile GO. The plan does not require evidence roles to be `PRESENT`: the manifest has 52 roles, all `MISSING` or `PLANNED` (40/12), `present_role_count: 0`, `authority: 0`, `accepted: false`, `completeness: UNKNOWN`, and `production_admission: REFUSED_MISSING_EVIDENCE`.

The planned request/result and normalization model otherwise matches source-visible outgoing behavior:

- independent target-bound prepared-caller selection precedes dispatch;
- the exact method is `callHierarchy/outgoingCalls`, with the selected prepared item in `CallHierarchyOutgoingCallsParams.Item`;
- each returned member supplies callee identity through `to`;
- outgoing normalization is queried caller → returned callee;
- `fromRanges` are caller-side call-site ranges in the queried caller document;
- occurrence policy preserves one occurrence per range, original member/range ordinals, duplicate equal ranges, repeated equal members, parallel caller→callee occurrences, and self-loops; canonical order cannot erase source order;
- malformed or partial rows/results, substitutions, reordered custody, stale inputs, and foreign workspace/session/generation inputs fail closed rather than yielding a best-effort prefix;
- finite independent ceilings and closed outgoing accounting are explicitly planned, including exact-limit, plus-one, overflow, underflow, impossible-total, and duplicate-ordinal refusal;
- incoming evidence, review, roles, receipts, authority, and GO do not transfer; immutable shared artifacts may only be referenced after independent identity, relevance, freshness, and exact outgoing-profile binding checks;
- no bidirectional acquisition profile or shared Stage 3–7 qualification is defined.

The future semantic-vector sequence is correctly ordered as frozen evidence → independently authored expectations → independent byte/oracle review → handwritten checks → one pinned same-adapter execution → exact comparison → bounded receipt. Its allowed claims and finite adapter limits are bounded, disagreement blocks, and these files add no dependency, adapter, invocation, execution, output, or receipt.

## Blocking contradiction: successful top-level `null`

The plan does not freeze the actual source-visible outgoing result contract for successful protocol `null`:

- `docs/acquisition-coordinator.md` states that the wire client accepts successful protocol `null` as empty and distinguishes this from malformed member `fromRanges`.
- `internal/acquisition/coordinator.go` converts a successful null outgoing result to a successful empty call set.
- The outgoing plan instead says null is refused unless it is “explicitly qualified,” O17 says it is complete-empty only if independently qualified later, and O-N11 defers the exact-gopls rule to the future.

This is not a request for evidence to be `PRESENT`. It is an unresolved planned semantic rule for the exact profile being reviewed. The four-file freeze must select and consistently test the source-compatible behavior—successful top-level `null` as successful-empty—or explicitly establish a reviewed incompatibility/version boundary. Leaving the decision to later evidence qualification makes the causal/vector oracle conditional and prevents exact plan approval.

No profile GO, evidence-production authorization, implementation authorization, or production admission is issued.

ADR0011_CALLS_OUTGOING_PLAN_BLOCKED
