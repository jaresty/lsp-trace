# ADR 0011 Go/gopls outgoing CALLS vector inventory and future adapter plan

**Profile:** `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`
**Status:** independently authored expectations; `PLANNED`, `UNEXECUTED`
**semantic-vector:** binding future plan only; no dependency, adapter, import, execution, output, or receipt exists

These outgoing expectations are authored independently of semantic-vector and must receive independent byte-level review before handwritten checks or any future adapter execution. Neither a tool nor observed gopls output may generate or revise the oracle.

## Positive outgoing vectors

| ID | World | Frozen expectation |
|---|---|---|
| O-P01 | Exact custody; selected prepared caller `A`; outgoing `[]` | Separate prepare request/result and outgoing request/result; selected caller ordinal retained; complete empty; zero occurrences. |
| O-P02 | Caller `A`, returned `to=B`, one caller-side range | Canonical `A -> B`; one occurrence; range is in queried caller document; all ordinals retained. |
| O-P03 | One member with two distinct ranges | Two occurrences with equal endpoints and distinct range ordinals. |
| O-P04 | Duplicate equal ranges | Duplicate occurrences survive with distinct range ordinals. |
| O-P05 | Duplicate equal response members | Multiplicity and response ordinals survive. |
| O-P06 | Parallel `A -> B` plus self-loop `A -> A` | All occurrences retained; no profile-level dedup or self-loop removal. |
| O-P07 | Multiple prepare items; explicit valid target-bound ordinal | Exact selected prepared caller passes unchanged to outgoingCalls; all raw prepare order retained. |
| O-P08 | Optional documentSymbol locates caller then prepare succeeds | documentSymbol remains locator custody; prepare selects caller; outgoing result alone supplies edge occurrences. |
| O-P09 | `fromRanges: []` | Zero-site member retained; zero occurrences; no inferred site. |
| O-P10 | Exact independent ceiling | Frozen exact-limit result and closed accounting. |
| O-P11 | Offline replay | Same canonical output/terminal outcome; no reacquisition. |
| O-P12 | Shared immutable gopls/toolchain/repo artifact | Reference accepted only after independent identity, relevance, freshness, and outgoing-binding verification; zero inherited authority. |

## Negative transaction and selection vectors

- O-N01 missing prepare request or result: evidence-missing refusal; no outgoing dispatch.
- O-N02 prepare null/empty: no selected caller; no outgoing dispatch.
- O-N03 no target-bound match: typed no-match refusal.
- O-N04 multiple target-bound matches without explicit ordinal: ambiguity refusal.
- O-N05 ordinal out of range or selecting a nonmatching item: typed refusal; no fallback.
- O-N06 malformed/partial/reordered prepare result: atomic refusal; no prefix selection.
- O-N07 selected caller field or opaque data changed before outgoing request: substitution refusal.
- O-N08 documentSymbol-only callable-shaped result: zero CALLS.
- O-N09 hidden failed prepare probe: accounting/custody refusal.

## Negative outgoing semantic vectors

- O-N10 missing outgoing payload: evidence-missing, never empty success.
- O-N11 null outgoing payload: distinct from `[]`; succeeds empty only under a future independently qualified exact-gopls rule.
- O-N12 malformed top level or first/later member: whole transaction refused.
- O-N13 missing/null/scalar/object `fromRanges`: malformed, not empty.
- O-N14 partial valid prefix then malformed/truncated bytes: zero qualified occurrences.
- O-N15 interpret caller-side `fromRanges` in returned callee document: semantic refusal.
- O-N16 reverse `A -> B` to `B -> A`: normalization refusal.
- O-N17 collapse duplicate range/member: multiplicity refusal.
- O-N18 silently drop self-loop or merge parallel edges: ledger mismatch.
- O-N19 silently sort before retaining ordinals: ordering-custody refusal.
- O-N20 mutate returned `to` identity: changed identity or refusal, never repair.

## Substitution, foreign, and stale vectors

Change exactly one field while resealing claimant-controlled descendants: profile/domain, provider, gopls bytes/version/launch environment, Go toolchain/environment, initialize capability, repository/dirty state, workspace, target file, encoding, session/generation, documentSymbol record, prepare request/result/caller ordinal, outgoing request/result, returned callee, range, every ordinal, limits, privacy policy, admission envelope, predecessor role, freshness, or immutable shared-artifact identity. Expected result is changed identity or typed refusal, never silent equivalence.

Incoming evidence, review, or GO; bidirectional receipts; synthetic carriers; copied/resealed records; digest-only claims; self-review; runtime flags/config/build tags; and plan-GO-shaped tokens leave every absent outgoing role absent and cause zero production-visible effects.

## Ceiling and accounting vectors

For initialization, optional documentSymbol, prepare probes, outgoing expansions, messages, per-message bytes, response bytes, aggregate evidence bytes, requests, nodes, depth, members, ranges, occurrences, time, logical work, and enforceable allocation, freeze:

- exact limit expected result;
- plus-one typed refusal before production effects;
- overflow/underflow/impossible accounting refusal;
- unstarted work recorded as budget-blocked without fabricated response.

Verify declared = begun + unstarted; begun terminal classes balance; members = with-sites + zero-site + rejected; ranges = admitted + rejected; and every occurrence references one retained member/range.

## Binding future semantic-vector adapter plan

No semantic-vector dependency, adapter, invocation, execution, or output is present. If separately authorized later, the required order is immutable:

```text
freeze independently acquired evidence bytes
→ independently author expected outgoing vectors
→ independently review exact vector bytes and oracle provenance
→ execute handwritten checks against the frozen evidence and expectations
→ execute semantic-vector through one pinned outgoing adapter
→ compare exact outputs, failures, effects, accounting, and classifications
→ produce one bounded validation receipt
```

Any disagreement among frozen expectations, handwritten checks, adapter execution, semantic-vector output, classification, accounting, or effects **blocks** validation and qualification. It cannot be resolved by majority vote, tool preference, rewriting expected vectors from observations, or accepting a best-effort subset.

A future pinned semantic-vector adapter may establish only test-mechanical facts within its bounded run:

- canonical test worlds encoded from already frozen evidence and expected vectors;
- controlled one-variable mutations;
- baseline and mutations executed through the same pinned adapter and invocation;
- exact output, terminal failure, and production-visible-effect comparison;
- classification contrast between baseline and controlled mutations;
- oracle provenance linking each expected byte to the independently authored and independently reviewed vector inventory;
- enforcement and accounting of the adapter's own finite message/byte/world/mutation/time/work/allocation ceiling;
- a bounded validation receipt naming exact inputs, adapter/tool identities, invocation, limits, outputs, mismatches, and terminal status.

semantic-vector cannot establish:

- correctness, authenticity, completeness, freshness, or provider authority of gopls evidence;
- correctness of prepared-caller selection, returned-callee identity, `fromRanges` interpretation, caller→callee normalization, multiplicity, ordinals, or accounting policy;
- qualification policy, role completeness, production admissibility, review independence, authority, plan GO, profile GO, or production activation;
- repair of missing, malformed, partial, reordered, substituted, stale, foreign, copied, or synthetic evidence.

The same-adapter comparison is a controlled contrast, not proof that the adapter or oracle is correct. The bounded receipt is diagnostic validation evidence only and cannot satisfy a gopls custody or qualification-policy role by itself.

## Disposition

All vectors and adapter steps are planned and unexecuted. This file supports only later independent review of the four-file plan under `ADR0011_CALLS_OUTGOING_PLAN_GO`. It supplies no evidence, dependency, implementation, review, qualification, production admission, or profile GO.