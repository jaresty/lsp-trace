# FUTURE/PROPOSED operation 35 semantic validation requirements v1

Status: **PROPOSED — REGISTRATION BLOCKER**

Operation 35 (`lsp_trace_v1_structural_context`) remains unregistered and unadvertised. JSON Schema validation is necessary but is not sufficient for this operation. Registration is prohibited until every success-result producer runs a mandatory runtime validator conforming to this document and tests prove that transport, registry, profile, and manifest cardinalities remain unchanged until that separate registration decision.

## Boundary

The draft schemas enforce closed object shapes, discriminators, enums, constants, scalar ranges, opaque identifier syntax, and COMPLETE/EMPTY-only success envelopes. JSON Schema 2020-12 does **not** establish the cross-field relations below. Schema acceptance must never be described as semantic closure.

`ValidateFutureStructuralSemanticsV1` is the current internal pure reference validator. It is deliberately not wired into MCP registration or transport. It accepts direct in-memory `map[string]any` values without assuming prior schema validation and fails closed on missing or unknown fields, nil or wrong-typed objects and entries, non-integer numeric representations, duplicate identifiers, out-of-range values, oversized collections, and checked-arithmetic overflow.

Validation precedence is fixed. Each caller-controlled object first receives its constant `len(map)` cardinality check before any unknown-key scan; each caller-controlled slice receives its `len(slice)` bound before element iteration; and every caller-controlled string, including IDs inside reference arrays, receives invalid-UTF-8 rejection and a stopping Unicode-code-point count before any regular expression or semantic parsing. Within that bounded processing, validation proceeds as follows: input closed shape, required fields, code-point/scalar bounds, conservative absolute-URI-subset syntax, target-form exclusivity, and input analysis shape; result closed shape, constants, policy closed shapes and bounds, then exact analysis-policy ID/kind coupling; analysis collection bounds, root identity, identifier uniqueness, endpoint/reference membership, analysis closed shape, and directional depth; accounting closed shape, scalar bounds, exact reason-map shapes, and checked equations; then the COMPLETE/EMPTY relation. Failures use fixed `semantic-v1` error categories and do not include caller-supplied values.

The input URI is limited to 4096 Unicode code points and uses one parser-free subset in both the draft schema and semantic helper: an RFC3986-style scheme `[A-Za-z][A-Za-z0-9+.-]*:` followed either by `//` for a hierarchical URI or by a non-`/` opaque payload character; no Unicode/ASCII whitespace, C0 control, DEL, or raw `%` is allowed, and every `%` must begin exactly two ASCII hexadecimal digits. This accepts bounded `file://`, HTTP(S), and custom opaque URIs while rejecting relative references, bare paths (including drive-letter paths), and malformed escapes. The draft deliberately omits `format: uri`: format implementations differ and would add acceptance behavior not represented by this subset. JSON Schema governs valid JSON Unicode strings; the standalone Go helper additionally rejects invalid UTF-8 in an original in-memory Go string before the shared expression runs, because JSON serialization replaces such invalid bytes before schema evaluation.

## Mandatory relations

For input `i` and result `r`, a conforming v1 runtime validator MUST reject unless:

1. `r.analysis.root_node_id == r.target_node_id`.
2. `r.state == EMPTY` iff the admitted `CALLS` edge count is zero; otherwise state is `COMPLETE`.
3. For `IMPACT`, `analysis.depth <= i.up_depth` when direction is `INCOMING`, and `analysis.depth <= i.down_depth` when direction is `OUTGOING`.
4. Every opaque node or edge reference in edges, reachable-node lists, and witness-edge lists belongs to the corresponding admitted node or edge set in the result.
5. Accounting equations hold:
   - `node_observed = node_admitted + node_rejected + node_omitted`
   - `occurrence_observed = occurrence_admitted + occurrence_rejected + occurrence_omitted`
   - `frontier_observed = frontier_expanded + frontier_unexpanded`
   - `request_attempted = request_succeeded + request_failed + request_cancelled`
   - `prepared_attempted = prepared_returned + prepared_empty + prepared_failed`
6. Each omitted node or occurrence and each unexpanded frontier item is assigned exactly one reason. Failed or cancelled requests are likewise partitioned by request omission reason. Every closed reason-count object uses exactly `DEPTH_BOUND`, `NODE_BOUND`, `REQUEST_BOUND`, `TIMEOUT`, `CANCELLATION`, `UNSUPPORTED_RESPONSE`, `MALFORMED_RESPONSE`, and `DEDUPLICATION`; its sum equals the corresponding omitted/unexpanded count (or failed-plus-cancelled request count). Counts are aggregate partitions; producers MUST NOT double-count one omission under multiple reasons.
7. A host creates each correlation ID from fresh randomness in `sc_[0-9a-f]{32}` format. It MUST NOT derive the ID from caller content, symbols, paths, URIs, or encoded forms. Success and domain-error construction for one request MUST reuse the same ID.

## Policy qualification

All draft traversal, resource, and analysis policy records have `policy_status: PROVISIONAL_NONCERTIFIED`. Their digest fields prove only syntactic digest shape and are not fixed certified kernel digests. A result cannot claim kernel qualification, publication eligibility, hydration eligibility, replayability, retention, or authority. Registration requires committed canonical policy documents plus an explicit certification/qualification decision, or an equally explicit continued noncertified qualification.

## Required tests before registration

Tests MUST preserve counterexamples that JSON Schema accepts but the v1 semantic validator rejects for every relation above. Mutation tests MUST cover Unix paths, Windows paths, file URIs, traversal strings, percent-encoded paths, malformed prefixes, and wrong-length correlation IDs. No operation-35 schema or semantic validator may be added to MCP registry, transport dispatch, capability profiles, or published manifests during this proposed phase.
