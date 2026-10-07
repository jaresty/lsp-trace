# ADR0007 Location Intersection — prospective v5 normative design

This root is a prospective private contract. `MUST`/`MUST NOT` are normative. Co-normative files are those listed in `SPEC_MANIFEST.json`. Any disagreement is a specification defect and evaluation MUST stop.

## 1. Closed identities and canonical JSON

Exact IDs are request `lsp-trace.adr0007.location-intersection.request.private.v5`, result `lsp-trace.adr0007.location-intersection.result.private.v5`, policy `lsp-trace.adr0007.location-policy.private.v5`, limits `lsp-trace.adr0007.location-limits.private.v5`, source binding `lsp-trace.adr0007.source-admission.private.v2`, and profile `lsp-trace.adr0007.strict-json-c14n.v1`. The five `*.schema.json` files are closed Draft 2020-12 schemas (`additionalProperties:false`; no nullable field).

Strict input is one UTF-8 JSON object followed by EOF or one LF. BOM, invalid UTF-8, duplicate keys, unknown keys, comments, non-integer numbers, exponent notation, leading zero, negative zero, values outside `[0,2^63-1]`, unpaired surrogate escape, trailing data, and whitespace other than the optional final LF are invalid. Duplicate-key detection occurs before object construction. Schema validation cannot relax lexical rejection.

Canonical output is compact UTF-8 JSON plus one LF. Object fields follow schema `required` order; arrays preserve semantic order. Integers are shortest decimal. Strings escape quote, backslash, and controls (short escapes for `\b\f\n\r\t`, otherwise lowercase `\u00xx`); `/`, U+2028, and U+2029 remain UTF-8. Unknown fields never serialize. Request field order is `schema,id,relation,selector,admissionDigest,policyDigest,limitsDigest,topK,members`; result order is `schema,requestId,outcome,members,ranked,counters,detail`. Nested orders are their schema `required` arrays.

Every request field is required and non-null. `requestId` is the decoded top-level `id` only if strict lexical parse produced one object and `id` was a string; otherwise it is `""`. Later schema failure does not erase a decoded string ID.

## 2. Digest envelope

`limitsDigest = sha256(canonical LIMITS.json bytes)`. Policy embeds that digest. Policy self-digest is SHA-256 over canonical `POLICY.json` after replacing only `digest` with `sha256:` plus 64 zeroes. The published digest MUST match. Manifest entries hash exact published bytes. For the manifest’s own entry only, `sha256` and `bytes` describe the canonical measurement image in which that entry’s `sha256` is 64 zeroes and `bytes` is 0; this is the sole self-reference exception.

## 3. Source admission import and mapping

The immutable imported contract is pinned in the manifest by repository commit, path, byte count, SHA-256, and Git blob. Its behavior is vendored normatively here: input is nonempty; limits all positive; paths are nonempty valid UTF-8 NFC clean relative slash paths with no backslash, colon, repeated slash, leading slash, `.`/`..` or dot segments; revision and bytes are nonempty; bytes are valid UTF-8; path is unique; per-source and cumulative byte maxima are inclusive; file/object digest, if supplied, equals SHA-256 bytes and both output digests are that value; sources sort by path bytes; admission digest is SHA-256 of source schema bytes followed, for every sorted source and each path/revision/fileDigest/objectDigest, by NUL then UTF-8 value.

| Imported state | v5 terminal outcome/detail | precedence |
|---|---|---:|
| binding absent | `SOURCE_ADMISSION_UNAVAILABLE/BINDING_UNAVAILABLE` | 3 |
| schema differs | `SOURCE_ADMISSION_MISMATCH/BINDING_SCHEMA` | 3 |
| `INVALID_REQUEST` | `SOURCE_ADMISSION_MISMATCH/BINDING_INVALID_REQUEST` | 3 |
| `INVALID_SOURCE` | `SOURCE_ADMISSION_MISMATCH/BINDING_INVALID_SOURCE` | 3 |
| `DUPLICATE_SOURCE` | `SOURCE_ADMISSION_MISMATCH/BINDING_DUPLICATE_SOURCE` | 3 |
| `RESOURCE_LIMIT`, detail `sources` | `RESOURCE_LIMIT/SOURCES` | 7 |
| `RESOURCE_LIMIT`, detail `bytes` | `RESOURCE_LIMIT/SOURCE_BYTES` | 7 |
| binding/request digest differs | `SOURCE_ADMISSION_MISMATCH/BINDING_DIGEST` | 3 |
| complete | continue | — |

Imported validation occurs before policy/selector/member evaluation. Cancellation/deadline polls outrank every mapping.

## 4. Selector and text decision tables

| kind | required | forbidden | expansion |
|---|---|---|---|
| `EXACT_FILE` | `path` | `frozenPaths,union` | bound source whole-file range |
| `PATH_PREFIX` | `path,frozenPaths` | `union` | exactly the strictly byte-sorted admitted paths equal to prefix or starting `prefix/` |
| `RANGE_UNION` | nonempty `union` | `path,frozenPaths` | entries sorted path then range tuple; exact duplicate ranges removed |

Empty expansion/union/ranges → `INVALID_SELECTOR/EMPTY_SELECTION`; noncanonical path → `INVALID_SELECTOR/PATH`; wrong presence → `INVALID_SELECTOR/SELECTOR_SHAPE`; prefix missing, extra, duplicate, unsorted, or non-segment member → `INVALID_SELECTOR/FROZEN_EXPANSION`. `src` matches `src` and `src/a`, never `src2/a`. Repeated union path is `SELECTOR_SHAPE` (it is not merged).

LF splits lines. CR immediately before LF is terminator and unaddressable; bare CR is one UTF-16 unit. Final LF creates an empty final line. Character is UTF-16 code units in line content. Line must exist; character may equal line length; out-of-file or mid-surrogate → `INVALID_RANGE/POSITION`. Range is half-open and requires start < end; empty/reversed selector/member range → `SELECTOR_RANGE`/`MEMBER_RANGE`. Whole file is `(0,0)` to final-line end. Touching ranges do not intersect.

For same-path selector `s` and candidate `c`: `INTERSECTS ⇔ max(start)<min(end)`; `CONTAINED_BY ⇔ s.start≤c.start ∧ c.end≤s.end`; `CONTAINS ⇔ c.start≤s.start ∧ s.end≤c.end`. Eligibility is existential over all same-path pairs.

## 5. Member/source decision table

Identity is nonempty NFC `id`; malformed identity is `INVALID_REQUEST/REQUEST_FIELD`. First ordinal owns an identity; later occurrences are `DUPLICATE_MEMBER` without source, range, policy, or score inspection.

| first-owner condition, first match wins | row outcome |
|---|---|
| `available=false` | `UNAVAILABLE_LOCATION` |
| path absent or revision/fileDigest/objectDigest differs from admitted source | `INVALID_LOCATION` |
| any member range invalid | `INVALID_LOCATION` |
| `policyAllowed=false` | `FILTERED_BY_POLICY` |
| some relation-true pair | `ELIGIBLE` |
| otherwise | `INELIGIBLE` |

Unavailable suppresses invalid/policy; invalid suppresses policy; duplicate suppresses all. Needed source population is selector-expanded paths union paths of first-owner `available=true` members that exist in binding. `sourceBytes` is sum of their unique admitted byte lengths, computed before pair evaluation. No backend exists: all bytes are immutable in the supplied binding. `BACKEND_FAILURE` is not a v5 outcome.

## 6. Witnesses, rows, counters, ranking

A witness exists for every relation-true pair of an eligible first owner. Its intersection is `[max starts,min ends)`. Sort by path bytes, selector tuple, candidate tuple, revision bytes, fileDigest, objectDigest; deduplicate exact seven-field equality. Global cap applies after concatenating member ordinal order and before materialization. Non-true pairs emit none.

Complete returns one member row per input ordinal. Failure returns no rows and all twelve counters zero. Complete satisfies:

`input = eligible + ineligible + unavailableLocation + invalidLocation + duplicateMember + filteredByPolicy`.

`witnesses` is emitted count. Ranked population is all eligible rows, sorted score descending, ordinal ascending, memberId UTF-8 ascending. `ranked=min(topK,eligible)`; `topK=0` emits none without changing witnesses.

Measurement image `I` is the complete canonical result with both `counters.outputBytes=0` and `counters.work=0`. `B=|I|` including LF. Report `outputBytes=B`. Work uses B, never final serialized width. Thus both counters are acyclic.

## 7. Total terminalization

Poll before parse; after lexical parse; before/after admission; before each member, pair, witness, named sort population, measurement serialization, and return. At each poll cancellation precedes expired deadline. No partial state survives.

Terminal precedence: (1) `CANCELLED/CANCEL_SIGNAL`; (2) `TIMEOUT/DEADLINE`; (3) lexical/schema `INVALID_REQUEST` details; (4) source-unavailable/mismatch mapping; (5) policy then limits digest `POLICY_MISMATCH`; (6) selector/range errors; (7) static/dynamic `RESOURCE_LIMIT`; (8) `COMPLETE/NONE`. Within a class, first request order wins; selector canonical populations use canonical order.

Every terminal result is canonical result schema, fallback requestId rule, terminal outcome/detail, empty members/ranked, and zero counters. Failure serialization and polling are exempt from work/output limits; it MUST fit `maxRequestBytes + 4096`, a construction guaranteed by fixed fields plus bounded requestId. If construction unexpectedly cannot fit or serialize, evaluator returns the same minimal shape with `requestId=""`; if that cannot serialize, the implementation is nonconforming and returns no v5 result. This rule prevents recursive failure. A complete measurement with `B>maxOutputBytes` terminalizes once as `RESOURCE_LIMIT/OUTPUT_BYTES`; the failure result is exempt.

## 8. Exhaustive limits

All maxima are inclusive; plus one fails before the exceeding operation.

| limit/population | checkpoint/order | detail | work charged before check? |
|---|---|---|---|
| request bytes | before parse | `INVALID_REQUEST/REQUEST_FIELD` | no |
| members | after schema | `MEMBERS` | request bytes only |
| topK | after members | `TOP_K` | request bytes only |
| imported sources | admission | `SOURCES` | request bytes only |
| source bytes/file,total | admission | `SOURCE_BYTES` | admitted prior bytes only |
| selector distinct paths | after selector shape | `SELECTOR_PATHS` | P/R already validated through offender |
| ranges per path | after path count | `RANGES_PER_PATH` | through offender |
| total ranges | after per-path | `TOTAL_RANGES` | through offender |
| frozen paths | before expansion compare | `FROZEN_PATHS` | P through offender |
| needed source bytes | after needed population | `SOURCE_BYTES` | S through offender |
| unique witnesses | after prospective dedup | `WITNESSES` | Q/C, no X |
| work | before every charged unit | `WORK` | refusal reports zero |
| measurement bytes B | after I | `OUTPUT_BYTES` | all success work calculated |

## 9. Work populations

`W=50+3J+7P+11R+13M+1S+19Q+23X+29C+31B`, checked unsigned 64-bit, then required `W≤maxWork`. Coefficients are positive schema constants.

- `J`: exact raw request byte length including optional LF; renamed semantic is request-byte population.
- `P`: one per attempted canonical-path validation: selector path, each union path, each frozen path, each admitted source path, and each first-owner available member path.
- `R`: one per attempted endpoint-pair range validation, selector then first-owner available member order.
- `M`: one per input member whose classification begins, including duplicate.
- `S`: one per byte in the unique needed source population.
- `Q`: one per same-path selector/candidate pair whose relation predicate is evaluated.
- `X`: unique witnesses actually materialized.
- `C`: `Σ n(n−1)/2` over these named sortable populations, regardless of implementation: admitted sources; prefix frozen paths; normalized union entries; ranges within each union entry; prospective witnesses within each member; eligible ranking rows. Populations of 0/1 contribute 0. Dedup occurs after the associated population charge.
- `B`: measurement-image bytes.

Every prospective increment is checked for overflow and precharged conceptually before its operation. If adding the next unit would overflow or exceed maxWork, terminalize `WORK`; equality passes. On non-work failures, pseudocode-defined increments may be computed diagnostically but failure counters remain zero.

## 10. Authority ceiling

Only the immutable syntax and evaluation rules inside this prospective root are claimed. No claim is made about current runtime behavior, dispatch, compatibility, implementation readiness, qualification, or acceptance. `SPEC_REVIEW_PENDING.md` controls the stop boundary.
