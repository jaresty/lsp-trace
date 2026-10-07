# Prospective v5 deterministic algorithm

This pseudocode is co-normative. `inc(k,n)` performs checked multiplication/addition for the coefficient of k; overflow or prospective work above maxWork returns `RESOURCE_LIMIT/WORK`. Any `fail` returns the total failure shape with empty arrays and zero counters. Diagnostic populations are never exposed on failure.

```text
evaluate(raw, binding, cancel, deadline):
  poll()                                      // cancel, then deadline
  if len(raw)>maxRequestBytes: fail INVALID_REQUEST/REQUEST_FIELD
  J=len(raw); inc(J,J)
  scan=reference_scan(raw)                    // collect candidates; class, byte, traversal tie-break
  if scan.candidates: fail INVALID_REQUEST/select_one(scan.candidates)
  requestId = scan.id if scan.id_is_string else ""
  poll()
  walk request.schema.json in field/index order; collect SCHEMA_ID/REQUEST_FIELD candidates
  if candidates: fail INVALID_REQUEST/select_one(candidates)
  if len(members)>maxMembers: fail RESOURCE_LIMIT/MEMBERS
  if topK>maxTopK: fail RESOURCE_LIMIT/TOP_K
  for member ordinal i:                       // identity checkpoint before envelope
    poll(); inc(M,1)
    if id=="" or NFC(id)!=id: fail INVALID_REQUEST/REQUEST_FIELD

  poll(); project envelope using only the five structural checks in DESIGN
  if projection fails: fail SOURCE_ADMISSION_MISMATCH/BINDING_SCHEMA
  for envelope source/input tuple in array order:
    require all five tuple keys and string values; otherwise fail BINDING_INVALID_SOURCE
    validate canonical padded base64; decode; require reencode equality; otherwise fail BINDING_INVALID_SOURCE
    inc(P,1); validate path/revision/digest syntax and decoded bytes; otherwise fail BINDING_INVALID_SOURCE
  if len(decoded sources)>maxSources: fail RESOURCE_LIMIT/SOURCES
  for decoded source in input order:
    inc(S,decodedLength)
    if decodedLength>maxSourceBytes: fail RESOURCE_LIMIT/SOURCE_BYTES
  cumulativeDecodedLength=0
  for decoded source in input order:
    cumulativeDecodedLength=checked_add(cumulativeDecodedLength,decodedLength)
    if cumulativeDecodedLength>maxTotalSourceBytes: fail RESOURCE_LIMIT/SOURCE_BYTES
  require complete-source path order strictly ascending; otherwise fail BINDING_INVALID_SOURCE
  rerun imported admission and map exact typed outcome/detail
  inc(C, choose2(len(complete binding sources)))
  recompute source and admission digests; compare envelope and request
  poll()
  if policyDigest!=POLICY.digest: fail POLICY_MISMATCH/POLICY_DIGEST
  if limitsDigest!=POLICY.limitsDigest: fail POLICY_MISMATCH/LIMITS_DIGEST

  // request schema already established selector shape; no semantic shape diagnostic exists
  if EXACT: inc(P,1); validate path; require bound; selectorPaths=1; generatedRanges=1
  if RANGE:
    for union entry raw order: inc(P,1); validate path; require bound
    check raw union count as selectorPaths
    for each raw range: inc(R,1); validate; check raw per-entry then raw total counts
    inc(C,choose2(raw union count)); inc(C,choose2(raw ranges per entry)); then sort, merge repeated paths, and dedup exact ranges
  if PREFIX:
    inc(P,1); validate prefix
    for frozen raw order: inc(P,1); validate
    check raw frozen count; expand binding matches; check expanded selectorPaths
    check generated one range/path and generated total; inc(C,choose2(raw frozen count))
    compare exact frozen expansion; zero matches with nonempty frozen list fail FROZEN_EXPANSION
  build canonical selector ranges

  needed = selector paths union existing paths of first-owner available members
  sourceBytes=sum unique needed decoded lengths; if sourceBytes>maxTotalSourceBytes: fail RESOURCE_LIMIT/SOURCE_BYTES

  for member ordinal i:
    poll()                                      // M was charged at identity checkpoint
    if id seen: row DUPLICATE_MEMBER; continue
    mark seen
    if !available: row UNAVAILABLE_LOCATION; continue
    inc(P,1); validate member path
    if source binding tuple mismatches: row INVALID_LOCATION; continue
    for member range in request order: inc(R,1); if invalid mark INVALID_LOCATION
    if invalid: row INVALID_LOCATION; continue
    if !policyAllowed: row FILTERED_BY_POLICY; continue
    prospective=[]
    for same-path selector range s in canonical order:
      for candidate range c in request order:
        poll(); inc(Q,1)
        if relation(s,c): append witness tuple to prospective
    inc(C,choose2(len(prospective)))
    sort and exact-dedup prospective
    if nonempty: row ELIGIBLE with prospective else row INELIGIBLE

  allWitnesses=concatenate eligible witness lists by ordinal
  if len(allWitnesses)>maxWitnesses: fail RESOURCE_LIMIT/WITNESSES
  for each allWitnesses: poll(); inc(X,1); materialize
  eligibleRows=all eligible rows
  inc(C,choose2(len(eligibleRows)))
  rank by score desc, ordinal asc, memberId bytes asc; take topK
  assert six-way sum

  image=canonical complete result with work=0,outputBytes=0
  for each image byte: poll before measurement unit
  B=len(image); inc(B,B)
  W=current checked work total
  if W>maxWork: fail RESOURCE_LIMIT/WORK
  if B>maxOutputBytes: fail RESOURCE_LIMIT/OUTPUT_BYTES
  set counters.work=W,counters.outputBytes=B
  poll(); return canonical complete result
```

`choose2(n)=n(n−1)/2` using checked unsigned arithmetic. For RANGE_UNION specifically, let `U` be the supplied raw union-entry sequence and let `ranges(e)` be the supplied raw range sequence of entry `e`. Before any sorting, repeated-path merging, or exact-range deduplication, charge exactly `choose2(len(U)) + Σ(e in U) choose2(len(ranges(e)))`; no normalized RANGE_UNION population contributes any C charge. Worked repeated-path example: raw entries `[(a,2 ranges),(a,1 range),(b,3 ranges)]` charge `choose2(3)+choose2(2)+choose2(1)+choose2(3)=3+1+0+3=7`, even if later repeated-path merging and exact-range deduplication reduce the normalized entries or ranges.

A path/range attempt increments before validation, so an offending item is counted diagnostically. Any earlier failure prevents later populations. Cancellation at any poll beats deadline and all provisional failures/results. There is no backend call.

## Decision examples 01–24

Fresh source `A` bytes are `a😀b\r\nxy\n` (11 UTF-8 bytes): lines have UTF-16 lengths 4,2,0 and whole range `(0,0)..(2,0)`.

1. EXACT_FILE, candidate `(0,1)..(0,3)`, INTERSECTS → eligible, overlap candidate.
2. Candidate `(2,0)..(2,0)` → complete row `INVALID_LOCATION` (empty member range).
3. Selector `(0,0)..(0,4)`, candidate `(0,1)..(0,3)`, CONTAINED_BY → true.
4. Selector `(0,1)..(0,3)`, candidate `(0,0)..(0,4)`, CONTAINS → true.
5. `(0,0)..(0,2)` and `(0,1)..(0,3)` INTERSECTS → `(0,1)..(0,2)`.
6. End equals start across ranges → false, no witness.
7. Two selector ranges, second true → eligible (existential).
8. Two candidate ranges, first true → eligible; one witness.
9. Prefix `src` matches `src` and `src/a`, not `src2/a`.
10. Frozen expansion missing one admitted matching path → `INVALID_SELECTOR/FROZEN_EXPANSION`.
11. Frozen expansion with extra nonmatching path → same failure.
12. Repeated union path is schema-valid: raw entries count for limits, then ranges merge under that path and exact duplicates dedup.
13. Duplicate true pair after exact range dedup → one witness.
14. First owner unavailable with malformed range → unavailable; range not inspected.
15. Later same ID unavailable → duplicate; duplicate takes precedence.
16. Available member revision mismatch → invalid location.
17. Valid relation but policy false → filtered by policy.
18. One row per six outcomes → input 6 and six-way sum `6=1+1+1+1+1+1`.
19. scores 9@ord2, 9@ord1, 7@ord0 → rank ord1,ord2,ord0.
20. topK 0 with two eligible → eligible 2, ranked 0, witnesses retained.
21. Two needed references to same 11-byte A → sourceBytes 11.
22. Unavailable and duplicate only, no selector reference to A → they add no needed bytes.
23. Imported duplicate source → `SOURCE_ADMISSION_MISMATCH/BINDING_DUPLICATE_SOURCE`.
24. Cancellation and deadline true at poll → `CANCELLED/CANCEL_SIGNAL`.

## Exact success with digit-width accounting

Let a synthetic valid request have raw length `J=200`, path attempts `P=3`, ranges `R=2`, identity-checked members `M=1`, decoded envelope bytes `S=11`, pair predicates `Q=1`, witnesses `X=1`; sortable populations are admitted sources 1, union entries 1, one range 1, witnesses 1, eligible rows 1, so `C=0`. Suppose the canonical measurement image has `B=999` bytes. Then:

`W=50+3·200+7·3+11·2+13·1+1·11+19·1+23·1+29·0+31·999=31728`.

The image contains `work:0` and `outputBytes:0`; the final result contains `work:31728` (five digits) and `outputBytes:999` (three digits). The transmitted final result is therefore 6 bytes longer than the image (four added work digits plus two added outputBytes digits), but reported outputBytes remains exactly 999 and W remains 31728. This demonstrates absence of digit-width recursion.

## Exact failures and boundaries

- With the same populations and `maxWork=31728`, success is admitted; `maxWork=31727` fails `RESOURCE_LIMIT/WORK` and returns all counters zero.
- If image B is exactly 8,388,608, output passes; B=8,388,609 fails `OUTPUT_BYTES`. Failure serialization is exempt and does not recurse.
- 10,000 members pass; 10,001 fails `MEMBERS`. 1,000 frozen paths pass; 1,001 fails `FROZEN_PATHS`. 10,000 unique witnesses pass; 10,001 fails `WITNESSES` before X materialization.
- In `a😀b`, positions 0,1,3,4 are valid; 2 is mid-surrogate. CR in CRLF is not addressable. `(2,0)` is EOF; line 3 is invalid.
- Malformed JSON without a safely decoded string id returns requestId `""`; a decoded id `"r7"` followed by an unknown field returns requestId `"r7"`.

## Worked invalid-envelope, selector, diagnostic, and identity examples

1. Complete envelope bytes `YQ==` decode to `a` and re-encode identically; decoded length and S charge are 1. `YQ`, `YQ=`, `YQ===`, `YQ==\n`, `YQ-_`, and `YR==` pass or fail projection solely as strings, then map `SOURCE_ADMISSION_MISMATCH/BINDING_INVALID_SOURCE` during base64 validation (`YR==` has nonzero pad bits and re-encodes as `YQ==`).
2. Missing top-level `binding`, extra top-level `foo`, non-object `binding`, non-array `sources`, or failure branch without `input` fails projection as `BINDING_SCHEMA`. A source tuple with numeric path, empty revision, `fileDigest:"bad"`, omitted bytes, or numeric bytes passes projection (the tuple object has only allowed keys) and then maps `BINDING_INVALID_SOURCE`. Complete sources `[b,a]` with valid tuples map `BINDING_INVALID_SOURCE` at deferred source-order validation. If the same valid decoded tuples are also over `maxSources`, any one decoded source is over `maxSourceBytes`, or their input-order cumulative decoded length is over `maxTotalSourceBytes`, the corresponding earlier check returns `RESOURCE_LIMIT/SOURCES` or `RESOURCE_LIMIT/SOURCE_BYTES`; the later unsorted-source defect does not win.
3. A typed `DUPLICATE_SOURCE/DUPLICATE_PATH` with input paths `[a,a]` reruns to the same result and maps `BINDING_DUPLICATE_SOURCE`; input `[a,b]` does not reproduce it and maps `BINDING_INVALID_REQUEST`.
4. EXACT_FILE `missing.go` with a valid complete binding lacking that path gives `INVALID_SELECTOR/PATH`; the same envelope with missing bytes fails earlier as `BINDING_INVALID_SOURCE`.
5. PATH_PREFIX `src` with no matching bound path and required nonempty frozen list gives `INVALID_SELECTOR/FROZEN_EXPANSION` because every listed path is extra. Empty frozen list, empty union/ranges, wrong kind fields, missing required fields, or extras are schema-invalid `INVALID_REQUEST/REQUEST_FIELD`; none reaches selector semantics.
6. RANGE_UNION with 11 paths each containing 10,000 identical raw ranges passes each per-path limit but totals 110,000 and fails `TOTAL_RANGES`; dedup to 11 never rescues it. PREFIX with 1,001 frozen entries fails `FROZEN_PATHS` before expansion; with 1,000 frozen entries but 1,001 binding matches it next fails `SELECTOR_PATHS`.
7. Input containing both an early unknown field and a later duplicate field returns `DUPLICATE_FIELD` because class priority precedes byte offset. Two duplicate keys return the one whose second occurrence starts at the earliest byte. Two member schema errors at the same structural class use lower member ordinal then field order.
8. Member 0 id `e\u0301` is valid Unicode but not NFC (`é` is NFC): after one M charge it returns `INVALID_REQUEST/REQUEST_FIELD` before an absent binding can return `BINDING_UNAVAILABLE`. A later duplicate is never considered.
9. Result pair `COMPLETE/NONE` validates; `COMPLETE/WORK`, `CANCELLED/DEADLINE`, and `RESOURCE_LIMIT/BINDING_DIGEST` fail `result.schema.json`. Every policy detail appears in exactly one allowed outcome branch.
