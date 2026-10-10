# ADR 0011 Go/gopls outgoing CALLS exact-profile qualification plan

**Profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
**State:** qualification-plan freeze only; `PLANNED`, `UNEXECUTED`, `DEFAULT_OFF`
**Authority ceiling:** `authority=0`, `accepted=false`, `completeness=UNKNOWN`
**Production admission:** `REFUSED_MISSING_EVIDENCE`

This author file plans qualification only. It creates no implementation, runner, evidence, copy, semantic-vector execution, review, admission, registration, publication, migration, production activation, or GO. A later independent reviewer may issue only `ADR0011_CALLS_OUTGOING_PLAN_GO`, scoped to review of these four author files. That token is not issued here, cannot qualify the profile, and cannot authorize evidence production. No profile GO is defined or requested by this plan.

## Exact boundary

The sole acquisition profile is outgoing Go call hierarchy from an exact gopls process:

- language: Go;
- provider: exact captured gopls binary;
- preparation method: `textDocument/prepareCallHierarchy`;
- edge method: `callHierarchy/outgoingCalls`;
- normalized relation: server-reported `CALLS`, caller to callee;
- canonical identity domain: `lsp-trace.adr0011.calls.go.gopls.outgoing.exact.v1`.

Incoming qualification supplies no fact, evidence, custody, authority, review, or GO to this profile. Bidirectional acquisition and shared Stage 3–7 qualification are outside scope. Historical omitted-selector `CALLS_ONLY` remains unchanged and cannot select this profile.

## Frozen transactions

Qualification must retain separate ordered transactions; no combined receipt may replace either operation.

1. **Prepare request custody** records exact method, params, caller document URI and position, request/session/generation identity, request ordinal, limits, framing/accounting, and terminal outcome.
2. **Prepare result custody** records missing, null, empty, malformed, partial, error, cancellation, timeout, or the complete ordered item array. Request custody cannot prove result custody.
3. **Prepared caller selection** binds candidates to a caller target established before selection. A documentSymbol path binds canonical URI, name, kind, range, and selectionRange. A direct-position path binds canonical URI and requires the requested position to be contained by both range and selectionRange. An explicit ordinal succeeds only when in range and target-bound; without one, exactly one target-bound item selects. Zero matches refuse; multiple matches refuse as ambiguous. Retain the selected array ordinal and canonical full item, including optional detail, tags, and opaque data.
4. **Outgoing request custody** proves the request item is the selected prepared caller without substitution or lossy remarshal and records request and expansion ordinals.
5. **Outgoing result custody** retains exact result bytes or a bounded record preserving original method, params, result bytes, response order, limits, framing/accounting, capture completeness, and terminal outcome.

Re-encoding cannot claim original-wire identity. A one-item prepare result is ordinary ordinal `0`, not a special selection rule. Every failed prepare probe remains accounted; no failed probe may be erased.

## Optional documentSymbol custody

`textDocument/documentSymbol` is optional locator custody, never CALLS evidence. If used, retain its request and result separately, raw response ordering, selected symbol identity and ordinal, ranges, normalization, ambiguity/absence/malformed outcomes, and each subsequent prepare probe. It cannot replace prepareCallHierarchy, select an outgoing edge, or authorize dispatch.

## Outgoing semantics and normalization

For queried prepared caller `A`, outgoing response member ordinal `r`, returned `to=B`, and `fromRanges` ordinal `s`:

- pre-normalization method form is `queried_caller A -> returned_to B`;
- canonical normalized CALLS form is `A -> B`;
- each `fromRanges[s]` is a call-site range in the queried caller item/document, not a range in `to`;
- retain exact returned `to`, exact range, response/member ordinal, range ordinal, request ordinal, expansion ordinal, selected prepared-caller ordinal, source URI, and encoding;
- admit one occurrence per range; a non-null empty `fromRanges` contributes zero occurrences and remains a successful member with zero sites;
- missing or null `fromRanges` is malformed, not empty;
- duplicate equal ranges and repeated equal members remain distinct occurrences;
- parallel caller→callee edges and self-loops remain explicit occurrences;
- canonical sorting may derive another order but cannot erase or rewrite source ordinals.

No pair union, deduplication, self-loop filtering, endpoint inference, source-text inference, incoming direction flip, or documentSymbol inference may replace the occurrence ledger.

## Custody and independent shared-artifact reuse

The later evidence plan must independently establish initialize request/result and capabilities, negotiated position encoding, exact gopls binary/version/argv/workdir/environment, Go toolchain and relevant `go env`, build flags/tags, module/workspace files, repository commit and dirty state, root, target bytes/URI/language, managed session/generation, request limits, and admission-envelope identity.

A gopls/toolchain/environment/repository/workspace artifact used by another profile may be referenced later only when this outgoing profile independently verifies its immutable schema identity, locator, byte length, SHA-256, relevance, freshness, and exact-profile binding. Equal bytes permit a reference, not authority inheritance. Incoming manifests, receipts, reviews, role names, or GO tokens cannot satisfy outgoing roles.

## Refusal matrix

Persist typed refusal, with zero qualified occurrences and zero production-visible effects, for:

- prepare request/result missing, null, empty, no match, ambiguous, ordinal out of range, target mismatch, malformed, partial, reordered, substituted, stale, foreign, error, cancellation, timeout, unsupported, or ceiling-blocked;
- outgoing request/result missing, null/empty when not explicitly qualified, malformed top level/member/range, partial prefix, reordered, substituted, stale, foreign workspace/session/generation, error, cancellation, timeout, unsupported, or ceiling-blocked;
- changed caller item/data, returned callee identity, URI/range/selectionRange/kind/name, `fromRanges`, ordinal, capability, encoding, provider, toolchain, environment, repository, file, or admission envelope;
- copied, resealed, cross-profile, synthetic, digest-only, self-reviewed, or plan-GO-shaped evidence.

No fallback, repair, ambient reread, hidden reacquisition, best-effort prefix, incoming conversion, or family alias yields qualified success.

## Outgoing accounting and ceilings

Freeze finite independent numeric ceilings before execution for initialization, documentSymbol, prepare probes, outgoing expansions, wire messages, per-message bytes, response bytes, aggregate evidence bytes, requests, nodes, depth, response members, ranges, admitted occurrences, time, logical work, and enforceable allocation.

Required closed accounting includes:

- declared operations = begun + unstarted;
- begun = successful + failed + cancelled + timed_out + unsupported + ceiling_refused;
- returned members = members_with_sites + zero_site_members + rejected_members;
- returned ranges = admitted_occurrences + rejected_ranges;
- every admitted occurrence references exactly one retained member/range location;
- checked overflow, underflow, duplicate ordinal, impossible total, exact-limit mismatch, or plus-one input refuses before every production-visible effect.

No ceiling borrows from another and no truncation is complete success.

## Binding future semantic-vector boundary

No dependency, adapter, invocation, execution, or output is added here. If separately authorized after evidence exists, the mandatory order is: freeze evidence bytes; independently author outgoing expected vectors; independently review the exact vector bytes and oracle provenance; run handwritten checks; run semantic-vector through one pinned same-adapter path; compare exact outputs, failures, effects, accounting, and classification contrasts; then produce one bounded validation receipt. Any disagreement blocks validation and qualification without tool-preference repair.

semantic-vector may establish canonical test worlds, controlled one-variable mutations, same-adapter baseline/mutation execution, exact output/failure/effect comparison, classification contrast, oracle provenance, its own finite ceiling/accounting, and a bounded validation receipt. It cannot establish gopls evidence correctness/authenticity/completeness, prepared-caller selection, returned-callee identity, `fromRanges` meaning, caller→callee normalization, multiplicity/ordinal policy, qualification policy, authority, admission, review, or GO.

## Planned sequence and hard boundary

1. Freeze these four independently authored plan artifacts.
2. Obtain independent review of plan coherence only.
3. A reviewer may issue `ADR0011_CALLS_OUTGOING_PLAN_GO` or return REVISE/REJECT.
4. Any evidence production, implementation, semantic-vector adapter or execution, profile qualification, production activation, bidirectional work, or shared Stage 3–7 work requires a new separately authorized scope not supplied here.

Current handoff is **NO GO**. Every evidence role is `MISSING` or `PLANNED`; none is `PRESENT`; production remains `REFUSED_MISSING_EVIDENCE`.