# CALLS_LSP_INCOMING_EXACT_V1 generic incoming semantic freeze

Status: `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`  
Authority: `0`; accepted: `false`; provider GO: `false`; production GO: `false`; incoming provider GO: `false`.

This package freezes only provider-neutral generic incoming LSP call-hierarchy semantics for `CALLS_LSP_INCOMING_EXACT_V1`. It does not use Go/gopls behavior as semantic authority and does not qualify `CALLS_GO_GOPLS_INCOMING_EXACT_V1`.

## Decision table

| Source shape | Generic decision | Relationship/endpoint evidence | Occurrence evidence | Notes |
|---|---|---|---|---|
| Complete top-level `null` | Successful empty source form if terminal policy admits null | none | none | LSP response type is `CallHierarchyIncomingCall[] | null`; null is distinct from `[]`. |
| Complete top-level `[]` | Successful empty array source form | none | none | No members; no relationships; no occurrences. |
| Top-level non-array/non-null | malformed transaction | none | none | Atomic refusal. |
| Member missing/null/malformed `from` | malformed transaction | none | none | `from` is required and must be a valid `CallHierarchyItem`. |
| Returned `from.name`, `from.kind`, `from.uri`, `from.range`, or `from.selectionRange` missing/malformed | malformed transaction | none | none | Required endpoint identity/display fields are retained only when valid. |
| Returned `from.range` / `from.selectionRange` valid | retained caller endpoint identity/display metadata | retained on the member/relationship row | none | These spans describe/enclose/reveal the caller endpoint. They are not CALLS occurrence rows. |
| Returned optional `from.tags`, `from.detail`, `from.data` present and valid | retained endpoint metadata | retained on the member/relationship row | none | Optional metadata does not create occurrence evidence. |
| Member missing `fromRanges` | malformed transaction | none | none | LSP defines non-optional `fromRanges: Range[]`; accepted amendment already pins missing/null as malformed. Preserve absence in raw custody only; do not admit semantic member. |
| Member `fromRanges: null` | malformed transaction | none | none | `Range[]` is non-null; atomic refusal. |
| Member `fromRanges: []` | valid zero-site member | retained `from -> queried callee` relationship/member row with full returned caller endpoint identity/display fields | zero call-site occurrences/ranges | Explicit present empty array is not missing, not null, and not a failure. It provides no occurrence range evidence. |
| Member `fromRanges: [range...]` | valid when every range is well formed | retained `from -> queried callee` relationship/member row with full returned caller endpoint identity/display fields | one occurrence per range | Preserve member ordinal, range ordinal, order, duplicate multiplicity, endpoint item, and exact occurrence range. |
| Duplicate ranges in one member | valid if each range is valid | one relationship/member row | one occurrence per supplied array element | No de-duplication or synthesis. |
| Valid prefix then later malformed member/endpoint/range | malformed transaction | none | none | No prefix salvage; terminal is atomic for semantic admission. |
| D/R occurrence, source text, documentSymbol, outgoing reverse, provider heuristic, `from.range`, or `from.selectionRange` | not CALLS occurrence evidence | none unless independently supplied by valid incoming member | none | Never retype endpoint display spans or Definition/Reference/locator/source evidence as CALLS. |

## Normalization order

1. Establish queried callee `B` under an independent target-selection/preparation contract.
2. Require the incoming response result to be complete top-level `null` or array.
3. For each array member in response order, require valid `from` as a `CallHierarchyItem`: `name`, `kind`, `uri`, `range`, and `selectionRange`, with `selectionRange` contained by `range`; retain optional `tags`, `detail`, and `data` when supplied.
4. Require present non-null array `fromRanges` before admitting any semantic member from the transaction.
5. Validate every supplied `fromRanges` `Range` shape and position order under prior strict range contracts.
6. Only after the whole response validates, emit relationship/member rows for every member. Each relationship row carries the returned `from_endpoint` identity/display metadata separately from occurrences.
7. Emit occurrence rows only for each supplied `fromRanges[s]`; present-empty emits no occurrence row.
8. Preserve supplied order and duplicate multiplicity; do not sort, collapse, infer, or repair.

## Endpoint identity versus occurrence evidence

The returned `from` endpoint is the caller endpoint identity and display evidence. Its `range` and `selectionRange` are retained because LSP defines them as the symbol enclosing range and selected/revealed range. They are not call-site spans and never become `CALLS` occurrences. The only call-site occurrence evidence in an incoming member is `fromRanges[]`.

## Revision after independent NO-GO

The independent semantic reviewer issued `CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_NO_GO` because the previous package under-specified returned `from` endpoint identity and had weak endpoint mutation tests. This revision preserves the accepted findings and adds the normative `CallHierarchyItem` field contract, endpoint identity/display retention, fixtures/oracles for missing/null/malformed `from`, valid mutations of `from.uri`, `from.kind`, `from.range`, and `from.selectionRange`, and tamper tests that detect erasing each valid endpoint mutation.

## Preserved findings

- Missing/null `fromRanges` is malformed for semantic admission.
- Present-empty `fromRanges: []` retains an otherwise-valid incoming member/relationship and emits zero occurrence rows.
- Top-level `null` and `[]` remain distinct successful-empty source forms when admitted by terminal policy.
- D/R occurrences are not retyped as CALLS.
- Go/gopls behavior is not generic semantic authority.

## Ceiling

This file establishes no provider behavior, runtime behavior, source completeness, public schema, registry entry, implementation write-set, outgoing semantics, composition behavior, custody change, or production admission. The only permitted producer verdict is `GENERIC_INCOMING_SEMANTICS_AUTHORED_NOT_GO`.
