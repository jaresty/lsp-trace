# ADR 0011 Go/gopls incoming CALLS vector inventory

**Profile:** `CALLS_GO_GOPLS_INCOMING_EXACT_V1`  
**Status:** independently authored expectation inventory; planned and unexecuted  
**Validator:** semantic-vector is optional and has not authored these expectations

This inventory freezes expected semantics before any optional semantic-vector use. The qualification owner, not a validator, determines direction, multiplicity, normalization, ordinals, terminal outcomes, refusal reasons, accounting, and canonical expectations.

## Positive vectors

| ID | Input world | Independently frozen expectation |
|---|---|---|
| V-P01 | Exact gopls binary/toolchain/env; clean exact repo/file/session; one target-bound prepare item; incoming `[]` | Separate successful prepare and incoming records; ordinary selected ordinal `0` and canonical prepared identity retained; zero occurrences; complete-empty terminal under the frozen provider contract. |
| V-P02 | Same as P01, incoming `null` | Distinct raw/bounded evidence from `[]`; successful-empty only if explicitly admitted by provider contract; never interpreted as missing. |
| V-P03 | Queried `B`, returned `from=A`, one range | Pre-normalization `B <- A`; post-normalization CALLS `A -> B`; one occurrence with response/range/request/expansion ordinals. |
| V-P04 | One returned member with two distinct fromRanges | Two occurrences (under per-range policy), same endpoints, distinct range ordinals, no pair collapse. |
| V-P05 | Duplicate equal ranges in one member | Duplicate occurrences retained with distinct range ordinals; any derived pair union references both. |
| V-P06 | Duplicate equal response members | Multiplicity retained with distinct response ordinals; canonical sorting preserves originals. |
| V-P07 | Parallel caller→callee members plus self-loop | All occurrences retained; later dedup/self-loop treatment is separately identified and cannot mutate ledger. |
| V-P08 | Multiple prepare items in raw order with an in-range explicit ordinal selecting a target-bound item | Explicit ordinal takes precedence only as a target-bound disambiguator; selected ordinal and canonical full-item identity are recorded and the item passes unchanged to incomingCalls; all unselected items and raw order remain retained, not evidence for edges. |
| V-P09 | Symbol target found through documentSymbol then successful prepare | documentSymbol retained only as target-selection custody; prepare establishes selected callable item; only incoming establishes CALLS. |
| V-P10 | Exact finite ceiling | Frozen exact-limit outcome and balanced accounting; no undeclared truncation. |
| V-P11 | Retained-record replay with all live callbacks forbidden | Byte-identical canonical result and terminal outcome; zero reacquisition. |
| V-P12 | Historical selector omitted | Exact historical `CALLS_ONLY` path and bytes; additive profile unreachable. |
| V-P13 | Separately qualified incoming and outgoing receipts composed | Bidirectional policy/composition receipt only; no new acquisition facts or third profile. |

## Negative prepare vectors

| ID | Mutation | Expected outcome |
|---|---|---|
| V-NP01 | Prepare payload missing | Typed evidence-missing refusal; zero incoming dispatch and occurrences. |
| V-NP02 | Prepare `null` | Typed no-prepared-target outcome; not missing and not callable. |
| V-NP03 | Prepare `[]` | Complete absent target; zero incoming dispatch. |
| V-NP04 | Multiple target-bound matches without an explicit ordinal | Ambiguous refusal before incoming dispatch and effects. |
| V-NP05 | No item satisfies the target binding | No-match refusal before incoming dispatch and effects. |
| V-NP06 | Explicit ordinal is negative/out of range or selects a nonmatching item | Out-of-range or target-binding refusal; no fallback to another item. |
| V-NP07 | Malformed top level or first/later item | Malformed refusal; no partial target selection. |
| V-NP08 | Partial/truncated capture | Capture-incomplete refusal; no prefix selection. |
| V-NP09 | Selected item field or opaque data substituted before incoming | Prepared-item identity mismatch; zero incoming dispatch or effects. |
| V-NP10 | Response reordered, even if the same values remain | Ordering-custody refusal; retained ordinal and canonical selected identity cannot be silently recomputed. |
| V-NP11 | Timeout/cancellation/server error/unsupported | Distinct typed terminal outcome; no hidden retry unless a separately frozen bounded retry policy exists. |

## Negative incoming vectors

| ID | Mutation | Expected outcome |
|---|---|---|
| V-NI01 | Incoming payload missing | Evidence-missing refusal; never empty success. |
| V-NI02 | Malformed top level | Malformed refusal; zero qualified occurrences. |
| V-NI03 | Malformed later member after valid prefix | Whole transaction unqualified; valid prefix admits zero qualified occurrences. |
| V-NI04 | `fromRanges` missing or null | Malformed member/refusal; not equivalent to empty array. |
| V-NI05 | `fromRanges` empty | Frozen zero-site policy applies; never invent a site. |
| V-NI06 | Invalid URI, range, kind, or selectionRange | Member/transaction refusal according to frozen atomicity; no repaired edge. |
| V-NI07 | Direction reversed after decode | Normalization refusal; validator/source text cannot repair. |
| V-NI08 | Duplicate range/member collapsed | Multiplicity mismatch/refusal. |
| V-NI09 | Self-loop silently dropped or parallel edges silently merged | Occurrence-ledger mismatch/refusal. |
| V-NI10 | Timeout/cancellation/error after partial bytes | Partial terminal refusal; no prefix admission. |

## Custody and substitution vectors

Each vector changes exactly one field while preserving/resealing claimant-controlled descendants. Expected result is changed identity or typed refusal, never silent equivalence:

- V-C01 profile ID;
- V-C02 canonical domain;
- V-C03 gopls bytes/digest/length;
- V-C04 reported gopls version;
- V-C05 launch argv/workdir/env;
- V-C06 Go toolchain/version;
- V-C07 relevant `go env`, flags/tags, module/workspace bytes;
- V-C08 initialize request/result or capability;
- V-C09 repository commit or dirty state;
- V-C10 workspace root;
- V-C11 target file bytes/digest/length/URI/language;
- V-C12 position encoding;
- V-C13 session ID or generation;
- V-C14 prepare request/result identity;
- V-C15 selected prepared item/index;
- V-C16 incoming request/result identity;
- V-C17 response/member/range/request/expansion ordinal;
- V-C18 admission envelope or predecessor role;
- V-C19 numeric ceiling or privacy policy;
- V-C20 stale validity or foreign workspace/session evidence.

## Ceiling and accounting vectors

For each independent bound—messages, per-request bytes, response bytes, aggregate evidence bytes, requests, nodes, depth, occurrences, ranges, time, logical work, and enforceable allocation—freeze:

- V-Lxx-A: exact limit expected success or explicitly planned terminal outcome;
- V-Lxx-B: limit plus one expected typed refusal before production-visible effect;
- V-Lxx-C: overflow/negative/impossible accounting expected parser refusal;
- V-Lxx-D: unattempted operation expected explicit budget-blocked record with no fabricated response.

No bound may borrow headroom from another. No limit failure may be represented as complete success.

## Historical compatibility vectors

- V-H01 omitted selector preserves exact `CALLS_ONLY` request/result behavior and bytes.
- V-H02 historical artifacts gain no profile, receipt, wrapper, field, or new identity.
- V-H03 additive failure cannot fall back while labeled qualified.
- V-H04 explicit incoming profile selection cannot alter historical defaults/errors/exits.
- V-H05 outgoing or bidirectional selector cannot alias this incoming profile.

## Synthetic firewall vectors

For fabricated fake-server data, hand-authored records, test fixtures, replay-only carriers, semantic-vector output, copied/resealed test evidence, matching digest without payload, self-signed review, privileged in-process caller, runtime flag, environment variable, configuration, and build tag, expect:

- test evidence remains non-production;
- missing roles remain missing;
- no production admission or GO;
- zero writes, publications, registrations, admissions, dispatches, checkpoints, receipts, or authority changes;
- no fallback, laundering, wrapping, aliasing, conversion, inheritance, successor reuse, self-refresh, or evidence minting.

## documentSymbol vectors

- V-D01 unique symbol: selected range/selectionRange retained; successful prepare still mandatory.
- V-D02 absent/ambiguous/malformed/truncated symbols: typed target-selection refusal; zero CALLS.
- V-D03 documentSymbol-only response shaped like a callable: still zero CALLS.
- V-D04 changed documentSymbol ordering/selected ordinal: changed target-selection identity.
- V-D05 successful symbol lookup followed by prepare mismatch: refusal; documentSymbol cannot override prepare.

## semantic-vector acceptance boundary

semantic-vector may later execute these frozen vectors and compare observed to expected values. It may not:

- choose or infer caller/callee direction;
- decide whether a returned member/range is one or many occurrences;
- choose duplicate, parallel-edge, self-loop, or empty-range policy;
- select normalization stages or canonical order;
- convert malformed/partial/stale/foreign/substituted input into an expected success;
- generate expected outcomes, refusal reasons, ceilings, accounting, identities, evidence, review, or GO.

A validator mismatch is diagnostic. The inventory changes only through separately reviewed human/domain custody, never because a validator generated a different answer.

## Current disposition

All vectors are planned and unexecuted. This inventory supports only review of the four-file plan freeze under `ADR0011_CALLS_INCOMING_PLAN_GO`. It supplies no evidence for `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`; current handoff remains **NO GO**.
