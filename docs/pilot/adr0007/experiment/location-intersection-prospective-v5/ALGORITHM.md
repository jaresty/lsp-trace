# Prospective v5 algorithm and hand calculations

This document is normative. Symbols and rules come from `DESIGN.md`; numbers below are newly constructed examples, not predecessor expected results.

## Deterministic evaluation

1. Poll cancellation then deadline. Strictly parse and validate JSON, charging `J`; retain no result state on failure.
2. Require exact request schema, then check `members ≤ maxMembers` and `topK ≤ maxTopK`.
3. Obtain the immutable `sourceadmissionv2` binding. Poll around acquisition. Apply its exact validation/digest algorithm and source limits. Require request admission digest.
4. Verify policy and limits digests.
5. Validate selector shape and canonical paths; expand exact file/prefix against the binding or normalize range union. Validate selector ranges against source text. Enforce selector counts.
6. Determine the needed unique source-path population and `sourceBytes`; enforce its limit. Any backend read failure is terminal after the mandatory override poll.
7. Walk members by ordinal. A repeated ID is immediately duplicate. For a first owner apply unavailable, binding/range invalid, policy, then relation classification. Poll and precharge before every member and pair.
8. Sort/deduplicate prospective witnesses, enforce global cap, then materialize. Build all member rows.
9. Sort eligible rows by score descending, ordinal ascending, member ID bytes ascending; take `topK` (`0` takes none).
10. Assert the six-way sum. Canonically serialize once with outputBytes zero, charge each serialization byte, set `outputBytes` to that length, enforce maxOutputBytes, poll, and return. Any terminal path returns empty arrays and zero counters.

## Worked geometry corpus: future cases 01–24

Use source A = `"a😀b\r\nxy\n"`. Its lines are line 0 `a😀b` of UTF-16 length 4, line 1 `xy` length 2, line 2 empty length 0. Whole-file range is `(0,0)..(2,0)`. Let `S0=(0,0)..(0,4)`, `S1=(1,0)..(1,2)`, and `C=(0,1)..(0,3)` unless stated.

1. EXACT_FILE + INTERSECTS with `C`: true; overlap `C`; one eligible row.
2. EXACT_FILE with candidate `(2,0)..(2,0)`: invalid empty range, not ineligible.
3. RANGE_UNION `S0`, CONTAINED_BY, `C`: true because selector contains candidate.
4. RANGE_UNION `C`, CONTAINS, candidate `S0`: true because candidate contains selector.
5. `S0=(0,0)..(0,2)`, candidate `(0,1)..(0,3)`, INTERSECTS: overlap `(0,1)..(0,2)`.
6. Selector ending `(1,0)`, candidate starting `(1,0)`: false; half-open touching.
7. Two selector ranges, only the second relates: eligible by existential quantification.
8. Two candidate ranges, only one relates: eligible; only true pairs witness.
9. Prefix `src` over frozen `src/a.go,src/lib/b.go`: both selected; `src2/c.go` excluded.
10. Frozen expansion missing admitted `src/lib/b.go`: terminal `INVALID_SELECTOR/FROZEN_EXPANSION`.
11. Frozen expansion containing `src2/c.go`: same terminal outcome.
12. Union paths supplied `z.go,a.go`: normalized to `a.go,z.go`; ranges sort by tuple.
13. Two identical true pairs: dedup to one witness.
14. First owner has `available=false`: unavailable, no source/range/policy evaluation.
15. Second occurrence of same ID is duplicate even if unavailable; first retains its own classification.
16. Available first owner whose revision differs from binding: invalid location.
17. Valid first owner with `policyAllowed=false`: filtered, even if relation would be true.
18. Six first-owner rows, one of each outcome: counters each 1 and input 6; invariant `6=1+1+1+1+1+1`.
19. Scores `(7,9,9)` at ordinals `(0,1,2)`: ranked order 1,2,0; ordinal resolves score tie.
20. `topK=0` with two eligible rows: eligible=2, ranked=0, witnesses remain emitted.
21. Two available members reference the same admitted 10-byte source: `sourceBytes=10`, not 20.
22. One unavailable and one duplicate reference a 10-byte source otherwise unused: both add zero; `sourceBytes=0`.
23. Backend read fails and cancellation is observed at the immediate override poll: `CANCELLED/CANCEL_SIGNAL`, not backend failure.
24. Cancellation and expired deadline both true at one poll: `CANCELLED/CANCEL_SIGNAL`.

## Position boundaries

- In `a😀b`, valid positions are 0,1,3,4; position 2 is `INVALID_RANGE/POSITION` because it bisects the emoji surrogate pair.
- CR in CRLF is not addressable: line 0 length is 4, not 5. In `a\rb` without LF, the bare CR is one UTF-16 unit and line length is 3.
- `(0,4)` is valid line-end; `(0,5)` is out of file. `(2,0)` is valid EOF in the example; line 3 is invalid.
- `[p,p)` is always invalid, including EOF. `[p,q)` and `[q,r)` do not intersect.
- A one-LF source has lines 0 and 1 and whole range `(0,0)..(1,0)`.

## Exact limit boundaries

For every limit `L`, count `L` is admitted and `L+1` returns its typed `RESOURCE_LIMIT` detail before the over-limit operation. Examples: 10,000 members pass; 10,001 gives `MEMBERS`. 1,000 frozen paths pass; 1,001 gives `FROZEN_PATHS`. 10,000 unique witnesses pass; the 10,001st prospective witness gives `WITNESSES` before materialization. 8,388,608 unique needed source bytes pass; 8,388,609 gives `SOURCE_BYTES`. A zeroed canonical result of 8,388,608 bytes passes; 8,388,609 gives `OUTPUT_BYTES`.

Work example: with `J=20,P=2,R=2,M=1,S=10,Q=1,X=1,C=0,B=100`,

`W=50+3·20+7·2+11·2+13·1+1·10+19·1+23·1+29·0+31·100 = 3311`.

If `maxWork=3311`, it completes; at `maxWork=3310`, the final serialization-byte precharge that would cross the maximum is refused and the terminal result is `RESOURCE_LIMIT/WORK` with every counter zero. If any coefficient multiplication or sum exceeds `2^64-1`, it is the same refusal before the operation.

Output-byte example: suppose canonical serialization with `counters.outputBytes` set to `0` is exactly 412 bytes including LF. Report `outputBytes=412`, even if replacing `0` by `412` makes the transmitted document 414 bytes. Compare 412, not 414, to maxOutputBytes.

## Digest example procedure

For policy digest calculation, replace only the digest value by `sha256:` plus 64 zeroes, serialize using the declared field order and final LF, SHA-256 those bytes, and publish lowercase hex. For limits, hash the published canonical file bytes directly. For source admission, sort sources by path and hash schema plus NUL-prefixed path/revision/fileDigest/objectDigest values; source bytes themselves are represented by their bound digests and are not appended to the admission-digest stream.
