# ADR 0011 qualified CALLS profile causal test specification

- **Framework:** `QUALIFIED_CALLS_V1`
- **Concrete candidate:** `CALLS_SYNTHCALLS_EXACT_V1`
- **Status:** planned, unexecuted, default-off
- **Production admission:** permanently and unconditionally forbidden for this exact identity; no `ADR0011_QUALIFIED_CALLS_PROFILE_GO` can name, upgrade, or authorize it

Tests are persistent semantic contracts, not one-time demonstrations. A test-only builder may create structurally exact fixtures and receipts only in test custody. Test-only receipts are never production-admissible, cannot satisfy manifest roles, cannot cross a production parser/admission boundary, and cannot issue review or GO.

## Exact positive chain

### Q01 — complete exact qualification chain

Construct one fully content-addressed test world with exact framework/profile contracts; provider/method, transaction, session, revision and capability custody; admitted occurrences and multiplicity; relation-family policy; Stage 3 input/composition; independently qualified Stage 4 policy; Stage 5 composition/readiness; Stage 6 representative; Stage 7A source context; Stage 7B checkpoint/replay; Stage 7C coupling/instability; accounting, resources, privacy, canonical identity, historical compatibility, and independent-review carriers. Assert exact predecessor closure, role order, schema/identity/digest/length equality, balanced accounting, unchanged ceiling, and deterministic canonical bytes.

The test world must remain marked `TEST_ONLY` and production admission must still refuse it.

### Q02 — production-shaped authority fixture

Exercise the same parser and verifier with production-shaped carriers whose authority hook is deliberately absent. Assert `PRODUCTION_AUTHORITY_MISSING`, no admission, no fallback to Q01, no evidence creation, and zero publication effects.

## Method/provider and occurrence semantics

### Q03 — provider/method custody

Vary method, provider slug/version, adapter, capability, session, generation, revision/custody, position encoding, request bytes, response bytes, implementation, and terminal outcome one at a time. Each mutation rejects before occurrence admission. Text-only, parser-derived, ambient-workspace, and unregistered-provider inputs contribute zero CALLS.

### Q04 — occurrence direction and multiplicity

Use two equal caller/callee pairs at distinct call sites and ordinals, plus repeated identical returned locations with distinct provider ordinals. Assert one admitted occurrence per returned member, caller→callee direction, exact ranges and relation-local ordinals, no pair collapse, canonical permutation stability, and balanced denominators.

### Q05 — malformed, partial, empty, and bounded outcomes

Exercise complete, complete-empty, unsupported, malformed first/later member, partial prefix, timeout, cancellation, provider failure, revision/policy mismatch, exact limit, and limit-plus-one. Assert closed outcomes, no truncation into success, no invented unstarted member outcomes, and no admission from incomplete or malformed results.

## Relation family and Stage 3

### Q06 — family isolation

Attempt to insert `REFERENCES_SYMBOL`, `RESOLVES_TO_DEFINITION`, unknown kinds, source-text locators, historical omitted-selector inputs, and cross-family duplicate endpoints. Assert zero additive CALLS admission and no relabeling. Explicit qualified CALLS selection is required.

### Q07 — Stage 3 exact input

Mutate occurrence ledger, denominator, ordinal, direction, custody, exclusion, duplicate handling, multiplicity treatment, canonical order, resource/privacy policy, or contribution record independently. Assert input/composition rejection, zero grouping execution, and no historical artifact mutation.

## Independent Stage 4 qualification

### Q08 — independence from provider qualification

Supply valid provider/method and occurrence evidence but omit Stage 4 qualification. Assert `GROUPING_POLICY_UNQUALIFIED` and zero Stage 5 execution. Conversely, supply a qualified grouping-policy fixture with absent provider/method evidence and assert refusal before Stage 3. Neither receipt substitutes for the other.

### Q09 — preregistration and held-out integrity

Mutate calibration/held-out partition, labels, corpus revision, expected groupings, policy, threshold, metric, seed, algorithm/profile, numeric contract, limit, reviewer class/count, or execution environment after preregistration. Assert `QUALIFICATION_SUBSTITUTED` or `QUALIFICATION_INCOMPLETE`. Missing mandatory corpus, adversarial fixture, metric, denominator, deterministic replay/permutation result, or independent acceptance yields `INCOMPLETE`.

### Q10 — false merges, fragmentation, and hubs

Exercise shared helpers, generated registries, builtins, high-degree hubs, duplicate returns, aliases, disconnected nodes, and expected boundaries. Assert preregistered thresholds and exact accounting; a failure cannot be converted to qualified by report prose or resealing.

## Stages 5–7

### Q11 — Stage 5 composition/readiness

Independently substitute Stage 3 input, Stage 4 policy/receipt, implementation, environment, algorithm, numeric policy, seed, limits, partition, or terminal accounting. Assert changed identity or rejection, never silent reuse. Equal partition bytes under unequal predecessors have unequal result identities.

### Q12 — Stage 6 representative

Test unique representative, deterministic tie, no eligible representative, omitted member, and invalid CALLS evidence. Assert only eligible admitted CALLS influences selection. D/R degree, lexical prominence, source order, file size, or test model output cannot alter it.

### Q13 — Stage 7A source context

Independently vary occurrence, representative, URI, revision, session/generation, document version, digest/length/bytes, encoding, every range class, display-resolution receipt, privacy policy, and limits. Test unavailable, ambiguous, withheld, stale, mismatched, and limit-omitted outcomes. Assert no ambient read/reacquisition, no identifier fragment presented as full context, exact terminal accounting, and no relation repair.

### Q14 — Stage 7B checkpoint/replay

Reconstruct from retained carriers only with workspace, network, LSP, clock, and ambient filesystem callbacks set to panic. Assert byte-identical output, exact-stage resume, idempotent reuse, concurrency arbitration, and no re-execution of completed stages. Missing, reordered, duplicated, spliced, stale, or substituted checkpoints/ref chains produce one typed refusal and no partial output.

### Q15 — Stage 7C coupling and instability

Test `Ca`, `Ce`, `I`, zero denominator, self loops, parallel-neighbor deduplication, node universe, direction, omitted/external evidence, incomplete acquisition, exact seed/run inventory, label-independent matching, unmatched communities, threshold boundaries, and failed/incomplete runs. Mutate every predecessor and policy independently. Assert publication blocking and no fallback to historical metrics.

## Accounting, resources, privacy, and canonical identity

### Q16 — accounting equations

At every stage assert applicable equations including `terminal=successful+failed+omitted`, `evaluated=terminal`, `expected=terminal+missing`, and occurrence/admission/composition denominators. Exercise checked overflow and every failure point before evaluation, during each member, after computation, and before commit. No failure exposes a completion checkpoint or downstream effect.

### Q17 — finite ceilings

For requests, messages, documents, candidates, occurrences, ranges, source bytes, canonical bytes, output bytes, aggregate bytes, time, logical work, and enforceable allocation bounds, assert exact-limit success and plus-one typed refusal/omission. Limit failure cannot emit partial production-visible artifacts.

### Q18 — privacy non-disclosure

For every withheld field/body, scan canonical outputs, errors, diagnostics, logs exposed by the contract, checkpoints, replay, coupling, instability, and review packets. Assert withheld bytes and reversible encodings are absent. Privacy-policy substitution rejects.

### Q19 — strict canonical identity

Reject duplicate/unknown keys, trailing values, invalid UTF-8, CRLF or missing/extra LF where canonical LF is required, noncanonical ordering/encoding, invalid base64, range/ordinal errors, integer overflow, and oversized nested bytes. Mutate role, schema, object identity, digest, decoded length, bytes, predecessor order, or domain separator independently; assert rejection. Resealed semantic forgery is never equivalent.

## Compatibility and admission gates

### Q20 — historical omitted-selector bytes

Pin pre-change historical `CALLS_ONLY` request/result, selector, schema, snapshot, packet, checkpoint, replay, representative, coupling/instability, CLI/MCP, error, default, and exit-behavior bytes. With the additive profile code present and absent, omission must select the same historical path and produce byte-identical outputs. Assert no additive parser, profile, receipt, field, wrapper, or identity appears.

### Q21 — explicit-selection isolation

Assert only exact explicit `QUALIFIED_CALLS_V1` plus `CALLS_SYNTHCALLS_EXACT_V1` can reach the additive verifier. Unknown, absent, empty, case-varied, stale, and legacy selectors refuse or take the exact historical behavior specified by their existing contract; none silently selects the proposed profile.

### Q22 — no retrofit or migration

Attempt to mark historical artifacts qualified, add profile fields, wrap/reissue historical bytes, transform old checkpoints, or replace historical replay. Assert refusal and byte-identical originals. An additive compatibility projection, if present as a fixture, remains a separate identity and grants no qualification.

### Q23 — test-only receipt barrier

For every required manifest role, substitute a structurally valid `TEST_ONLY` receipt. Assert production parser/admission returns `TEST_EVIDENCE_NOT_ADMISSIBLE`, zero downstream effects, and does not copy, upgrade, resign, publish, or register it. Test configuration, build tags, environment variables, runtime flags, or privileged in-process callers cannot override the barrier.

### Q24 — producer cannot self-mint

Remove each required role in turn and invoke every production entry point with producer-generated bytes, self-signed review, matching hashes without payloads, prose assertions, cached results, and runtime override flags. Assert the role remains missing, admission refuses, no artifact is created to fill it, and no GO token is emitted.

### Q25 — independent review and GO

For `CALLS_SYNTHCALLS_EXACT_V1`, before and after review, even a structurally complete test chain remains permanently production-forbidden. Any review or GO-shaped fixture naming, aliasing, wrapping, converting, inheriting from, succeeding, or transitively relying on the synthetic identity must return `SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN` before production effects. A production GO may exist only in a separate test vector for a distinct production profile ID, schema identity, canonical domain, independently selected provider/method authority, manifest, predecessor closure, evidence set, and review. That GO is jointly scoped to the exact production profile ID and canonical domain and implies no family, alias, lineage, predecessor, successor, or equal-byte authority. This specification itself never emits GO.

## Substitution, staleness, absence, and refusal matrix

### Q26 — exhaustive one-field substitutions

For every role and every semantic field, change one field while preserving all others; test expectation-side, retained-object-side, and cross-reference substitutions. Required result is changed identity or typed refusal with zero new admission. Consistently rehashing claimant-controlled descendants cannot overcome an unchanged authoritative predecessor.

### Q27 — staleness

Expire or supersede provider authority, profile, revision/session custody, implementation, policy, corpus, reviewer authority, privacy policy, readiness, checkpoint, source context, or validity epoch one at a time. Assert `QUALIFICATION_STALE` (or narrower typed outcome), no fallback, no self-refresh, no reacquisition, and no GO.

### Q28 — absence

Delete payload while retaining digest, delete digest/length while retaining payload, remove each role, remove one predecessor link, and make source bytes unavailable. Assert `EVIDENCE_MISSING`/`REPLAY_UNAVAILABLE`, no reconstruction from ambient state, and no interpretation as empty or complete.

### Q29 — explicit refusal persistence

Persist every refusal as a canonical, replayable diagnostic result with no production-admissible success receipt. Replay reproduces refusal bytes without retrying methods, stages, source acquisition, grouping, or review. Refusal cannot become success solely because dependencies later appear.

### Q30 — default-off and no public surface

Mechanically inspect any separately authorized implementation diff and registration graph. Assert no default, public CLI/MCP selector, capability advertisement, schema registry, catalog, help/completion, production dispatch, or migration changes. The current documentation-only authoring phase changes exactly the four named source files and produces no evidence, copies, commit, implementation, or registration.

### Q31 — permanent synthetic-identity bridge refusal vectors

For every vector below, begin with otherwise complete evidence, valid receipts, independent-review-shaped bytes, valid signatures, enabled flags/configuration/build tags, privileged callers, and a GO-shaped token. Change only the named bridge. Every vector must return typed `SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN` before any production effect and must assert exactly zero production writes, publications, registrations, admissions, dispatches, checkpoints, receipts, and authority changes:

1. alias the synthetic profile ID under a production-facing name;
2. rename the synthetic profile or its identity-bearing object;
3. convert synthetic evidence or receipts into production-shaped carriers;
4. wrap or embed the synthetic profile, evidence, receipts, or canonical preimage;
5. inherit from or extend the synthetic profile;
6. mark a production candidate as successor and reuse synthetic facts, lineage, or bytes;
7. reuse the synthetic schema ID;
8. reuse the synthetic canonical identity domain or canonical preimage;
9. reuse any synthetic receipt;
10. claim lineage or predecessor authority from the synthetic identity;
11. copy, rehash, resign, or reseal synthetic evidence into a production closure;
12. substitute a production profile ID around synthetic bytes, objects, or predecessors; and
13. name or transitively rely on the synthetic identity in a production GO.

For each vector, also assert no fallback, laundering, repair, self-refresh, alternate typed success, partial effect, or successor authority. Independently repeat the vectors through every production parser, verifier, admission, dispatch, publication, checkpoint, receipt, and registration entry point. A distinct production profile control is admissible for evaluation only when its profile ID, schema identity, canonical domain, provider/method authority, manifest, predecessor closure, evidence, receipts, review, and GO are all independently produced; it inherits exactly zero authority from the synthetic profile. GO scope is exact profile ID plus exact canonical domain and has no family, alias, lineage, predecessor, successor, or equal-byte implication.

## Test-only semantic-vector validation boundary

A later qualification effort may evaluate `/Users/schwa/dev/semantic-vector` as a test-only runner for independently authored vectors. This specification does not add or execute that tool. Before use, positive and negative vectors, expected canonical bytes, expected typed outcomes, accounting, and refusal reasons must be authored and frozen by domain/qualification owners without semantic-vector assistance.

The proposed inventory is: positive exact custody/multiplicity, complete test-only chain, permutation/replay, and exact-limit vectors; negative provider/method/custody, occurrence, family, Stage 3–7, canonical, privacy/resource, compatibility, staleness, absence, refusal, test-only promotion, self-minting, self-review, and GO vectors. semantic-vector may compare observed results to these frozen expectations and report mismatches. It must never synthesize expectations, expected groupings, thresholds, authority, evidence, review findings, or GO. Its receipts and output are test-only and cannot satisfy a production manifest role.

## Required execution sequence

```text
freeze historical byte baseline
→ land persistent RED tests
→ independent RED review
→ produce test-only contracts and bounded test evidence
→ run focused semantic tests
→ run replay/concurrency/race tests
→ run historical compatibility gates
→ freeze content-addressed TEST_ONLY manifest
→ independent test-adequacy review
→ permanent SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN
→ no GO possible for CALLS_SYNTHCALLS_EXACT_V1
```

A passing test suite is evidence for named tests only. It is not production admission, producer authentication, source completeness, acceptance, or GO.

## Reviewer handoff

Current verdict is **NO GO**. A reviewer receives only the frozen vector inventory and exact test-only evidence manifest, verifies that expectations were independently authored, and verifies semantic-vector (if adopted) acted only as a test runner. All exactly 42 roles remain `MISSING` or `PLANNED`, and `production_admission` remains `REFUSED_MISSING_EVIDENCE`. The reviewer must return `REVISE` or `REJECT`; no review artifact, evidence completion, receipt, signature, configuration, build tag, or future action may issue GO for, promote, or authorize `CALLS_SYNTHCALLS_EXACT_V1`. Any production GO is outside this handoff and requires a distinct production profile ID and canonical domain with zero inherited authority and wholly independent evidence, receipts, predecessor closure, review, and GO.
