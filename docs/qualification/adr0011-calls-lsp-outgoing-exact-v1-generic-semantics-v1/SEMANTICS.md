# CALLS_LSP_OUTGOING_EXACT_V1 generic outgoing semantic freeze

Status: `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`  
Authority: `0`; accepted: `false`; provider GO: `false`; production GO: `false`; outgoing provider GO: `false`.

This package freezes only provider-neutral generic outgoing LSP call-hierarchy semantics for `CALLS_LSP_OUTGOING_EXACT_V1`. It does not use Go/gopls behavior as semantic authority and does not qualify `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`.

## Decision table

| Source shape | Generic decision | Relationship/endpoint evidence | Occurrence evidence | Notes |
|---|---|---|---|---|
| Complete top-level `null` | Successful empty source form if terminal policy admits null | none | none | LSP response type is `CallHierarchyOutgoingCall[] \| null`; null is distinct from `[]`. |
| Complete top-level `[]` | Successful empty array source form | none | none | No members; no relationships; no occurrences. |
| Top-level non-array/non-null | malformed transaction | none | none | Atomic refusal. |
| Member missing/null/malformed `to` | malformed transaction | none | none | `to` is required and must be a valid `CallHierarchyItem`. |
| Returned `to.name`, `to.kind`, `to.uri`, `to.range`, or `to.selectionRange` missing/malformed | malformed transaction | none | none | Required endpoint identity/display fields are retained only when valid. |
| Returned `to.range` / `to.selectionRange` valid | retained callee endpoint identity/display metadata | retained on the member/relationship row | none | These spans describe/enclose/reveal the callee endpoint. They are not CALLS occurrence rows. |
| Returned optional `to.tags`, `to.detail`, `to.data` present and valid | retained endpoint metadata | retained on the member/relationship row | none | Optional metadata does not create occurrence evidence. |
| Member missing `fromRanges` | malformed transaction | none | none | LSP defines non-optional `fromRanges: Range[]`; preserve absence in raw custody only; do not admit semantic member. |
| Member `fromRanges: null` | malformed transaction | none | none | `Range[]` is non-null; atomic refusal. |
| Member `fromRanges: []` | valid zero-site member | retained `queried caller -> to` relationship/member row with full returned callee endpoint identity/display fields | zero call-site occurrences/ranges | Explicit present empty array is not missing, not null, and not a failure. It provides no occurrence range evidence. |
| Member `fromRanges: [range...]` | valid when every range is well formed | retained `queried caller -> to` relationship/member row with full returned callee endpoint identity/display fields | one occurrence per range | Preserve member ordinal, range ordinal, order, duplicate multiplicity, endpoint item, and exact occurrence range. |
| Duplicate ranges in one member | valid if each range is valid | one relationship/member row | one occurrence per supplied array element | No de-duplication or synthesis. |
| Valid prefix then later malformed member/endpoint/range | malformed transaction | none | none | No prefix salvage; terminal is atomic for semantic admission. |
| D/R occurrence, source text, documentSymbol, incoming reverse, provider heuristic, `to.range`, or `to.selectionRange` | not CALLS occurrence evidence | none unless independently supplied by valid outgoing member | none | Never retype endpoint display spans or Definition/Reference/locator/source evidence as CALLS. |

## Normalization order

1. Establish queried caller `A` under an independent target-selection/preparation contract.
2. Require `textDocument/prepareCallHierarchy` to be an ordered, target-bound transaction; retain the complete ordered prepare result and selected zero-based item ordinal.
3. Pass the selected prepared item byte-identically, or through an independently qualified exact semantic-preservation rule, to `callHierarchy/outgoingCalls` as the `item` parameter.
4. Require the outgoing response result to be complete top-level `null` or array.
5. For each array member in response order, require valid `to` as a `CallHierarchyItem`: `name`, `kind`, `uri`, `range`, and `selectionRange`, with `selectionRange` contained by `range`; retain optional `tags`, `detail`, and `data` when supplied.
6. Require present non-null array `fromRanges` before admitting any semantic member from the transaction.
7. Validate every supplied `fromRanges` `Range` shape and position order under prior strict range contracts.
8. Only after the whole response validates, emit relationship/member rows for every member. Each relationship row carries the queried caller identity, returned `to_endpoint` identity/display metadata, response member ordinal, request/expansion/prepared-item ordinals, and range count.
9. Emit occurrence rows only for each supplied `fromRanges[s]`; present-empty emits no occurrence row.
10. Preserve supplied order and duplicate multiplicity; do not sort, collapse, infer, or repair.

## Endpoint identity versus occurrence evidence

The returned `to` endpoint is the callee endpoint identity and display evidence. Its `range` and `selectionRange` are retained because LSP defines them as the symbol enclosing range and selected/revealed range. They are not call-site spans and never become `CALLS` occurrences. The only call-site occurrence evidence in an outgoing member is `fromRanges[]`, whose ranges are relative to the queried caller item.

## Multi-item prepare selection and target binding

A generic outgoing transaction must bind a caller target before the outgoing request. If `textDocument/prepareCallHierarchy` returns multiple items, the selected item ordinal and selected item bytes are part of the transaction identity. Missing, null, empty, no-match, ambiguous, malformed, partial, stale, substituted, foreign, or over-limit preparation selects nothing and cannot produce outgoing semantics. Re-encoding alone never proves byte identity; an exact semantic-preservation rule must be independently qualified if byte identity is not retained.

## Preserved findings from accepted CALLS/D/R contracts

- Generic incoming and outgoing profiles are independent; neither direction inherits semantic authority from the other.
- Missing/null `fromRanges` is malformed for semantic admission.
- Present-empty `fromRanges: []` retains an otherwise-valid outgoing member/relationship and emits zero occurrence rows because normative `Range[]` does not require non-empty arrays.
- Top-level `null` and `[]` remain distinct successful-empty source forms when admitted by terminal policy.
- D/R occurrences are not retyped as CALLS.
- Go/gopls behavior and provider NO_GO packages are not generic semantic authority.

## Ceiling

This file establishes no provider behavior, runtime behavior, source completeness, public schema, registry entry, implementation write-set, incoming semantics, composition behavior, custody change, or production admission. The only permitted producer verdict is `GENERIC_OUTGOING_SEMANTICS_AUTHORED_NOT_GO`.
