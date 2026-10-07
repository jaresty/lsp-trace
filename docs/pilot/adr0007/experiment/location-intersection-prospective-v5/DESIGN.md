# ADR0007 Location Intersection — prospective v5 normative design

`MUST`, `MUST NOT`, `SHOULD`, and `MAY` are normative. `ALGORITHM.md`, `POLICY.json`, and `LIMITS.json` are co-normative; disagreement is a specification defect and execution MUST stop.

## 1. Identities and digests

Assumption: v5 must not collide with predecessors. Therefore the exact IDs are:

- request schema: `lsp-trace.adr0007.location-intersection.request.private.v5`;
- result schema: `lsp-trace.adr0007.location-intersection.result.private.v5`;
- policy schema/ID: `lsp-trace.adr0007.location-policy.private.v5` / `lsp-trace.adr0007.location-policy.v5`;
- limits schema/ID: `lsp-trace.adr0007.location-limits.private.v5` / `lsp-trace.adr0007.location-limits.v5`;
- canonical profile: `lsp-trace.adr0007.strict-json-c14n.v1`.

`policyDigest` is `sha256:` plus lowercase hex SHA-256 of the canonical `POLICY.json` object after replacing its `digest` value with exactly `sha256:` followed by 64 ASCII zeroes. The published `digest` MUST equal that value. `limitsDigest` is SHA-256 of the canonical bytes of `LIMITS.json` exactly as published. A request binds both digests. This zeroed-field rule avoids self-reference.

## 2. Strict JSON and canonical bytes

Input MUST be one UTF-8 JSON object followed by either EOF or one LF. BOM, invalid UTF-8, duplicate keys, unknown keys, comments, NaN/infinity, exponent notation, negative zero, fractional numbers, trailing non-whitespace, trailing whitespace other than the single optional LF, and unpaired surrogate escapes are invalid. Numbers are base-10 unsigned integers with no leading zero except `0`, bounded to `[0, 2^63-1]`. Strings decode JSON escapes, MUST be Unicode scalar sequences, and are NFC-normalized only where a field explicitly says so; noncanonical normalizable values are rejected, not rewritten.

Canonical serialization has no insignificant whitespace, emits object fields in the schema-declared order, preserves array order unless a rule explicitly sorts it, uses decimal integers, `true`/`false`/`null`, and emits strings as UTF-8 with only `"`, `\\`, `\b`, `\f`, `\n`, `\r`, `\t`, or lowercase `\u00xx` for U+0000–U+001F. Other scalars are emitted directly, including `/`; U+2028/U+2029 are not escaped. Canonical documents end with exactly one LF. Field order is normative arrays in this document, not lexical key order.

Request order: `schema,id,relation,selector,admissionDigest,policyDigest,limitsDigest,topK,members`. Selector order: `kind,path,frozenPaths,union`; absent forbidden fields are omitted. Union entry: `path,ranges`. Position: `line,character`; range: `start,end`. Member: `id,path,revision,fileDigest,objectDigest,ranges,available,policyAllowed,score`. Result: `schema,requestId,outcome,members,ranked,counters,detail`. Member row: `ordinal,memberId,outcome,witnesses`. Witness: `path,selectorRange,candidateRange,intersection,revision,fileDigest,objectDigest`. Ranked: `ordinal,memberId,score`. Counters: `input,eligible,ineligible,unavailableLocation,invalidLocation,duplicateMember,filteredByPolicy,ranked,witnesses,work,sourceBytes,outputBytes`.

## 3. Request, selector, and paths

A selector is exactly one form:

- `EXACT_FILE`: `path` present; `frozenPaths` and `union` absent.
- `PATH_PREFIX`: `path` and `frozenPaths` present; `union` absent.
- `RANGE_UNION`: `union` present; `path` and `frozenPaths` absent.

An empty `union`, empty `frozenPaths`, repeated frozen path, repeated union path, or empty ranges is invalid selector. Paths MUST satisfy the imported canonical-path predicate. Prefix matching is segment-aware: prefix `p` selects exactly `p` and strings beginning `p + "/"`; `src` does not select `src2/a`. `PATH_PREFIX.frozenPaths` is the exact expansion: it MUST be strictly increasing by UTF-8 bytes, every path MUST match the prefix, and it MUST equal the strictly sorted set of admitted binding paths matching the prefix. Missing or extra paths cause `INVALID_SELECTOR/FROZEN_EXPANSION`.

`EXACT_FILE` expands to the whole-file range of its bound source. `PATH_PREFIX` expands each frozen path to that source's whole-file range in frozen order. `RANGE_UNION` retains request union order for validation, then canonicalizes entries by path UTF-8 ascending and ranges by `(start.line,start.character,end.line,end.character)` ascending; exact duplicate ranges are removed.

## 4. Source admission and member binding

The source binding imports `sourceadmissionv2` exactly: schema `lsp-trace.adr0007.source-admission.private.v2`; nonempty valid UTF-8 bytes; canonical NFC relative slash path; nonempty revision; unique path; per-source and total byte limits; file/object digest equal SHA-256 of bytes (filled by admission if absent); sources sorted by path; admission digest SHA-256 over schema then, for each sorted source and each of path/revision/fileDigest/objectDigest, one NUL byte followed by UTF-8 value bytes.

An absent binding produces `SOURCE_ADMISSION_UNAVAILABLE/BINDING_UNAVAILABLE`. Wrong binding schema or request admission digest produces `SOURCE_ADMISSION_MISMATCH/BINDING_DIGEST`. A member source is bound only when its path exists and its revision, fileDigest, and objectDigest exactly equal the admitted source. `available=false` is classified unavailable without backend access. `available=true` with absent/mismatched source binding is invalid location.

Member identity is the NFC `id` string alone. Empty or non-NFC IDs are invalid request. The first ordinal for an ID is the identity owner; every later ordinal is `DUPLICATE_MEMBER`, irrespective of availability, policy, binding, ranges, or score. The first owner is then classified in this order: `available=false` → `UNAVAILABLE_LOCATION`; source/binding/range invalid → `INVALID_LOCATION`; `policyAllowed=false` → `FILTERED_BY_POLICY`; relation true → `ELIGIBLE`; otherwise `INELIGIBLE`. Thus unavailable precedes invalid, invalid precedes policy, and duplicates precede all three for subsequent occurrences.

## 5. Text and range model

Source bytes MUST be valid UTF-8. Logical lines are split on LF. A CR immediately before LF belongs to the line terminator and is not addressable; a bare CR is an ordinary U+000D scalar occupying one UTF-16 code unit. The final LF creates a following empty line. Without final LF, the final line ends at EOF. Position character counts UTF-16 code units in line content only. A position may equal line length. It MUST NOT exceed it or bisect a surrogate pair. Line equal to line count is invalid; the only EOF is the end position of the last logical line.

A range is half-open `[start,end)` in lexicographic `(line,character)` order. `start < end` is required; empty or reversed ranges are invalid. Every endpoint must be valid. Whole-file range is `(0,0)` to the EOF position. An empty file is forbidden by source admission, but a one-LF file has whole range `(0,0)..(1,0)`. Touching ranges do not intersect. Out-of-file and mid-surrogate endpoints are invalid.

## 6. Relations

For selector range `s` and candidate/member range `c` on the same path:

- `INTERSECTS(s,c) ⇔ max(s.start,c.start) < min(s.end,c.end)`;
- `CONTAINED_BY(s,c) ⇔ s.start ≤ c.start ∧ c.end ≤ s.end` (candidate is contained by selector);
- `CONTAINS(s,c) ⇔ c.start ≤ s.start ∧ s.end ≤ c.end` (candidate contains selector).

A member is eligible iff there exists at least one selector range and at least one candidate range on the same path satisfying the requested relation. Quantification is existential over pairs; no all-ranges condition exists.

## 7. Witnesses

Only eligible first-owner rows receive witnesses. One witness is generated for every relation-true pair. `intersection` is always `[max starts,min ends)`; for containment it remains the geometric overlap, not the larger range. Witnesses sort by path UTF-8, selector range tuple, candidate range tuple, revision UTF-8, fileDigest, objectDigest. Exact seven-field duplicates are removed after sorting. The global `maxWitnesses` cap applies to the concatenation in member ordinal order. The evaluator MUST precharge the complete prospective unique count; if it exceeds the cap, return terminal `RESOURCE_LIMIT/WITNESSES` with no rows. Non-intersecting and otherwise relation-false pairs produce no witness.

## 8. Rows, counters, and ranking

`COMPLETE` returns exactly one row for every input member in ordinal order. Every failure returns zero member and ranked rows and all counters zero. For complete results:

`input = members.length`; each row contributes to exactly one of six counters; therefore

`input = eligible + ineligible + unavailableLocation + invalidLocation + duplicateMember + filteredByPolicy`.

`witnesses` is the sum of emitted witness counts. Eligible rows form the ranked eligible set. Sort descending score, then ascending ordinal, then member ID by UTF-8 bytes. `ranked = min(topK, eligible)` and ranked rows are the first `ranked`; `topK=0` returns no ranked rows but does not change eligibility or witnesses.

`sourceBytes` is the sum of byte lengths of unique admitted source paths actually needed after selector expansion plus paths referenced by first-owner available members, deduplicated by path. It is computed after binding/selector validation and before pair evaluation; unavailable rows and duplicates add no path. If a referenced available member path is absent, no bytes are added for that path.

`outputBytes` is the byte length of the complete canonical result (including final LF) with only `counters.outputBytes` replaced by integer `0`; the reported value is that measurement. `maxOutputBytes` is tested against that value, preventing self-reference.

## 9. Outcome precedence and detail enum

At each polling point, cancellation is checked before deadline; if both are true, `CANCELLED/CANCEL_SIGNAL` wins. Otherwise terminal precedence is:

1. `CANCELLED/CANCEL_SIGNAL` or `TIMEOUT/DEADLINE` at a poll;
2. strict parse/schema failure: `INVALID_REQUEST/{JSON_SYNTAX,JSON_ENCODING,UNKNOWN_FIELD,DUPLICATE_FIELD,TRAILING_DATA,SCHEMA_ID,REQUEST_FIELD}`;
3. source binding unavailable/mismatch as above;
4. policy/limits digest mismatch: `POLICY_MISMATCH/{POLICY_DIGEST,LIMITS_DIGEST}`;
5. selector then range errors: `INVALID_SELECTOR/{SELECTOR_SHAPE,PATH,FROZEN_EXPANSION,EMPTY_SELECTION}` or `INVALID_RANGE/{SELECTOR_RANGE,MEMBER_RANGE,POSITION}`;
6. static/dynamic limit: `RESOURCE_LIMIT/{MEMBERS,SELECTOR_PATHS,RANGES,FROZEN_PATHS,SOURCES,SOURCE_BYTES,WITNESSES,TOP_K,WORK,OUTPUT_BYTES}`;
7. backend failure: `BACKEND_FAILURE/{SOURCE_READ,BACKEND_INTERNAL}`;
8. `COMPLETE/NONE`.

The detail field is exactly one enum token above; no free text. The first error in request order wins within a class, except canonicalized selector validation uses canonical path/range order. Backend failure never becomes a member outcome and never yields partial results.

## 10. Limits, work, cancellation, and atomic failure

All maxima are inclusive: value equal to maximum is accepted; maximum plus one is refused. Checkpoints: member/topK before binding; source limits during imported admission; selector path/frozen/range counts before expansion; sourceBytes after needed-path population; witness count after dedup count and before materialization; work before every charged operation; output bytes after canonical zeroed measurement and before return.

Checked work uses unsigned 64-bit arithmetic and coefficients in `LIMITS.json`:

`W = 50 + 3J + 7P + 11R + 13M + 1S + 19Q + 23X + 29C + 31B`.

`J` is strict JSON nodes validated (each object, array, key, and scalar); `P` canonical-path validations; `R` range validations; `M` member classifications including duplicates; `S` admitted source bytes read for the unique `sourceBytes` population; `Q` selector/candidate pair predicates evaluated; `X` unique witnesses materialized; `C` actual comparator calls across every required sort; `B` bytes in the zeroed-output canonical serialization. Each operation MUST precharge its coefficient before execution. If checked addition/multiplication overflows, or the postcharge would exceed maxWork, refuse before the operation with `RESOURCE_LIMIT/WORK`; equality is allowed. A refusal result has zero counters, including work.

Poll points are: before parsing; after parsing; before and after source admission/binding; before each source read; before each member; before each pair; before each witness materialization; before each sort comparison; before and after serialization; before return. A poll terminal result discards all provisional rows/counters. A backend error is recorded provisionally, then an immediate poll occurs; cancellation, then deadline, overrides it. No partial result is ever returned.
