# Prospective v5 deterministic algorithm

This pseudocode is co-normative. `inc(k,n)` performs checked multiplication/addition for the coefficient of k; overflow or prospective work above maxWork returns `RESOURCE_LIMIT/WORK`. Any `fail` returns the total failure shape with empty arrays and zero counters. Diagnostic populations are never exposed on failure.

```text
evaluate(raw, binding, cancel, deadline):
  poll()                                      // cancel, then deadline
  if len(raw)>maxRequestBytes: fail INVALID_REQUEST/REQUEST_FIELD
  J=len(raw); inc(J,J)
  lex=parse_strict(raw)                       // duplicate/unknown/trailing rules
  if lex.error: fail INVALID_REQUEST/lex.detail
  requestId = lex.id if lex.id_is_string else ""
  poll()
  validate request.schema.json; on error fail INVALID_REQUEST/{SCHEMA_ID|REQUEST_FIELD|UNKNOWN_FIELD}
  if len(members)>maxMembers: fail RESOURCE_LIMIT/MEMBERS
  if topK>maxTopK: fail RESOURCE_LIMIT/TOP_K

  poll(); map_admission(binding); poll()       // table in DESIGN
  for admitted source in input order: inc(P,1); validate path
  inc(C, choose2(len(binding.sources)))        // admitted-source canonical sort population
  validate admission digest
  if policyDigest!=POLICY.digest: fail POLICY_MISMATCH/POLICY_DIGEST
  if limitsDigest!=POLICY.limitsDigest: fail POLICY_MISMATCH/LIMITS_DIGEST

  validate selector shape
  if selector has path: inc(P,1); validate
  for each union path in request order: inc(P,1); validate
  for each frozen path in request order: inc(P,1); validate
  enforce selector path/frozen/range limits at table checkpoints
  for each selector range in request order: inc(R,1); validate against source
  inc(C, choose2(unionEntryCount))
  for each union entry: inc(C, choose2(rangeCount(entry)))
  if prefix: inc(C,choose2(frozenPathCount)); compare exact frozen expansion
  build canonical selector ranges

  needed = selector paths
  for member ordinal i:
    if id first-owner and available and bound path exists: add path to needed
  S=sum byte lengths of unique needed paths
  inc(S,S); if S>maxTotalSourceBytes: fail RESOURCE_LIMIT/SOURCE_BYTES

  for member ordinal i:
    poll(); inc(M,1)
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

`choose2(n)=n(n−1)/2` using checked unsigned arithmetic. A path/range attempt increments before validation, so an offending item is counted diagnostically. Any earlier failure prevents later populations. Cancellation at any poll beats deadline and all provisional failures/results. There is no backend call.

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
12. Repeated union path → `INVALID_SELECTOR/SELECTOR_SHAPE`.
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

Let a synthetic valid request have raw length `J=200`, path attempts `P=3`, ranges `R=2`, members `M=1`, needed bytes `S=11`, pair predicates `Q=1`, witnesses `X=1`; sortable populations are admitted sources 1, union entries 1, one range 1, witnesses 1, eligible rows 1, so `C=0`. Suppose the canonical measurement image has `B=999` bytes. Then:

`W=50+3·200+7·3+11·2+13·1+1·11+19·1+23·1+29·0+31·999=31728`.

The image contains `work:0` and `outputBytes:0`; the final result contains `work:31728` (five digits) and `outputBytes:999` (three digits). The transmitted final result is therefore 6 bytes longer than the image (four added work digits plus two added outputBytes digits), but reported outputBytes remains exactly 999 and W remains 31728. This demonstrates absence of digit-width recursion.

## Exact failures and boundaries

- With the same populations and `maxWork=31728`, success is admitted; `maxWork=31727` fails `RESOURCE_LIMIT/WORK` and returns all counters zero.
- If image B is exactly 8,388,608, output passes; B=8,388,609 fails `OUTPUT_BYTES`. Failure serialization is exempt and does not recurse.
- 10,000 members pass; 10,001 fails `MEMBERS`. 1,000 frozen paths pass; 1,001 fails `FROZEN_PATHS`. 10,000 unique witnesses pass; 10,001 fails `WITNESSES` before X materialization.
- In `a😀b`, positions 0,1,3,4 are valid; 2 is mid-surrogate. CR in CRLF is not addressable. `(2,0)` is EOF; line 3 is invalid.
- Malformed JSON without a safely decoded string id returns requestId `""`; a decoded id `"r7"` followed by an unknown field returns requestId `"r7"`.
