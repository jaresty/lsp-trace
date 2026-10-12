# ADR 0011 Go/gopls incoming CALLS causal-test specification

- **Exact profile:** `CALLS_GO_GOPLS_INCOMING_EXACT_V1`
- **State:** planned, persistent semantic contracts, unexecuted
- **Plan gate:** `ADR0011_CALLS_INCOMING_PLAN_GO` (not issued)
- **Evidence gate:** `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` (not issued)
- **Current production admission:** `REFUSED_MISSING_EVIDENCE`

These are persistent semantic tests, not demonstrations. Passing tests cannot create evidence, authority, admission, review, or GO.

## Transaction and custody tests

### I01 — separate prepare and incoming transactions
Retain a separate `textDocument/prepareCallHierarchy` request/result followed by separately identified `callHierarchy/incomingCalls` request/result records. Assert method, params, response, request ordinal, response order, terminal outcome, limits, and capture completeness. Collapsing both methods into one receipt, deriving prepare from incoming, or omitting either transaction refuses before occurrence admission.

### I02 — selected prepared-item identity
Return one or more prepared items and retain the complete array in exact raw response order. Bind candidates to the preexisting target: a documentSymbol path requires exact canonical URI, name, kind, range, and selectionRange; a direct-position path requires exact canonical URI and the requested position contained by both range and selectionRange. An explicit ordinal has precedence only as a disambiguator: it must be in range and select a target-bound item. Without an explicit ordinal, exactly one target-bound match selects; zero matches refuses as no match and multiple matches refuse as ambiguous. Record the selected ordinal and canonical full-item identity, including opaque `data`, and assert that item is byte/semantic-identical to incomingCalls params. A single item is the ordinary ordinal `0` case. Mutate each field, binding field, and ordinal independently; expect typed no-match, out-of-range, substitution, or ambiguity refusal before incoming dispatch, with zero effects.

### I03 — initialization and capability custody
Vary initialize request/result, client capability, server `callHierarchyProvider`, position encoding, initialization options, and terminal state one at a time. Missing, unsupported, stale, foreign, or substituted capability custody refuses before prepare/incoming dispatch.

### I04 — gopls executable custody
Vary gopls bytes, SHA-256, length, reported version, launch argv, working directory, and relevant environment independently. A version string without original binary custody, equal version with unequal bytes, or equal bytes under substituted argv/environment fails exact-profile admission.

### I05 — Go environment custody
Vary Go executable/toolchain, `go version`, relevant `go env`, build flags/tags, module/workspace files, and dependency selection independently. Expect changed identity or typed refusal, never silent reuse.

### I06 — repository, file, and session custody
Vary commit, dirty state, workspace root, target file bytes/digest/length/URI, language ID, encoding, session ID, and generation independently. Stale, dirty-without-custody, foreign-root, foreign-file, or substituted-session evidence admits zero occurrences.

### I07 — exact bytes versus bounded records
Exercise exact JSON-RPC frames and the allowed bounded-record form. Assert bounded records preserve original method/params/result bytes, ordering, framing/accounting, limits, and outcome. Re-marshaled semantic equality cannot claim original-wire equality.

## Target selection firewall

### I08 — documentSymbol is locator custody only
For symbol selection, retain the separate `textDocument/documentSymbol` transaction, selected symbol, selectionRange, and every bounded prepare probe. Assert documentSymbol produces no CALLS occurrence and cannot replace prepareCallHierarchy. A documentSymbol-only fixture must fail with zero edge effects.

### I09 — direct position and symbol paths remain distinct
Prove direct position selection issues prepare against the supplied position, while symbol selection uses documentSymbol only to choose a candidate prepare position. Cross-laundering either path, hiding failed probes, or changing selected coordinates refuses.

## Direction, occurrence, and multiplicity

### I10 — incoming pre/post orientation
Given queried callee `B` and returned `from=A`, assert pre-normalization method orientation records `B <- A` and post-normalization CALLS orientation records `A -> B`. Reverse either form and expect refusal. No text, documentSymbol, or graph heuristic may repair direction.

### I11 — one response member and fromRanges custody
Use two response members with repeated endpoints and multiple `fromRanges`. Assert response-member ordinal, range ordinal, exact range, request ordinal, expansion ordinal, and selected prepared-item ordinal. Every admitted occurrence traces to one exact member/range under the frozen empty-range rule.

### I12 — duplicate ranges and repeated members
Return duplicate equal ranges in one member and duplicate equal members at distinct ordinals. Assert multiplicity and all original ordinals survive; canonical sorting cannot collapse them. A pair-union graph may be separately derived but cannot replace the occurrence ledger.

### I13 — parallel edges and self-loops
Return parallel caller→callee evidence and a self-loop. Assert both remain explicit occurrences. Apply any later deduplication/self-loop policy only as a separately identified composition transform with source occurrence references and balanced counts.

### I14 — response ordering
Permute response members and range arrays. Assert original ordering fields change while any canonical derived identity follows the frozen ordering policy. A producer that silently sorts before recording original ordinals fails.

## Terminal/refusal semantics

### I15 — prepare null, empty, selection, malformed, and partial
Exercise missing payload, `null`, `[]`, one item, multiple items, unique target match, zero target matches, multiple target matches, valid explicit ordinal, out-of-range ordinal, ordinal selecting a nonmatching item, reordered response, substituted item, malformed top level, malformed first/later item, partial capture, server error, timeout, cancellation, unsupported, and ceiling refusal. A complete array with one or more items may select only under I02's deterministic target-bound rule; one item is ordinary ordinal `0`, not the only valid cardinality. Empty/null, no match, ambiguity, out-of-range ordinal, reorder, substitution, malformed, or partial capture refuses before incoming dispatch and every effect. No partial prefix succeeds.

### I16 — incoming null and empty
Exercise `null` and `[]` separately. Both may yield successful-empty only if the frozen gopls provider contract admits both; evidence and identities remain distinguishable. Missing payload is never empty.

### I17 — incoming malformed and partial
Exercise malformed top level, malformed first/later member, missing/null/scalar/object `fromRanges`, truncated capture, invalid URI/range/kind, and trailing JSON. Assert zero qualified occurrences for the transaction, typed refusal, and no prefix admission.

### I18 — stale, foreign, and substituted inputs
Substitute profile, canonical domain, provider, binary, toolchain, environment, workspace, commit, dirty state, file, capability, session, generation, encoding, prepare item, request, response, ordinal, or admission envelope one at a time. Expect changed identity or typed refusal with zero downstream effects.

### I19 — admission-envelope exact identity
Bind the envelope to all required predecessor roles, methods, profile/domain, provider, environment, target, limits, outcome, and occurrence ledger. Duplicate/unknown roles, reordered role list where order is semantic, missing payload behind a digest, cross-profile evidence, or consistently rehashed claimant descendants cannot pass.

## Ceilings, accounting, replay, and privacy

### I20 — independent finite ceilings
For messages, per-request bytes, response bytes, total evidence bytes, requests, nodes, depth, occurrences, ranges, time, logical work, and enforceable allocation, assert exact-limit behavior and plus-one typed refusal. Plus-one has zero production-visible effects and balanced attempted/unattempted accounting.

### I21 — closed accounting
Assert declared = begun + unstarted, begun = successful + failed + cancelled + timed_out + unsupported + ceiling_refused, returned members/ranges balance admitted + rejected, and every occurrence references one retained response location. Checked overflow refuses.

### I22 — replay without reacquisition
Replay from retained records with workspace, filesystem, process launch, clock, and LSP callbacks set to panic. Assert exact terminal result and canonical bytes. Missing, reordered, stale, or substituted records yield one refusal and no retry, ambient read, or hidden reacquisition.

### I23 — privacy/non-disclosure
Withhold selected environment/source/diagnostic values under a frozen policy. Scan outputs, errors, logs covered by contract, manifests, checkpoints, and replay. Assert bytes and reversible encodings are absent while custody/accounting remains explicit.

## Compatibility and profile firewalls

### I24 — historical `CALLS_ONLY` compatibility
Pin historical omitted-selector request/result, schema, canonical bytes, snapshots, checkpoints, replay, CLI/MCP behavior, defaults, diagnostics, and exit behavior. With this planned profile present or absent, omission must follow the same path and produce byte-identical results. No profile field, wrapper, receipt, or additive parser appears.

### I25 — explicit exact selection
Only exact selection of `CALLS_GO_GOPLS_INCOMING_EXACT_V1` may reach its verifier. Missing, empty, case-varied, alias, stale, family-only, outgoing, and bidirectional selectors cannot select it implicitly.

### I26 — outgoing independence
Supply otherwise valid outgoingCalls evidence. Assert it cannot satisfy any incoming role, cannot be direction-flipped into incoming, and requires its own profile and GO. Incoming qualification implies nothing about outgoing.

### I27 — bidirectional composition only
Compose separately qualified incoming and outgoing receipts under an explicit policy. Assert the composition adds no acquisition fact and cannot mint a third acquisition profile. Missing either predecessor refuses composition; it does not trigger reacquisition.

### I28 — synthetic firewall
Run fabricated, fake-server, fixture-built, hand-authored, replay-only, test-only, and semantic-vector-produced carriers through every production parser/admission boundary. Expect `TEST_EVIDENCE_NOT_ADMISSIBLE` (or narrower frozen refusal), zero production writes/publications/registrations/admissions/dispatches/checkpoints/receipts/authority changes, and no copying, resealing, signing, or self-minting repair.

### I29 — producer cannot self-review or issue GO
Remove each role in turn and present matching hashes without payloads, prose claims, self-signed review, privileged caller, flags/config/build tags, plan GO, or evidence GO-shaped token. The role remains missing. `ADR0011_CALLS_INCOMING_PLAN_GO` authorizes only the frozen plan; it cannot satisfy evidence roles. Only an independent exact-profile review may issue `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`.

## semantic-vector confinement

### I30 — independently authored expectations
Verify the vector inventory predates any semantic-vector input/output and identifies its human/domain authoring custody. semantic-vector may compare observed results to frozen expectations only. Mutate direction, multiplicity, normalization, ordinals, or outcomes in validator-generated output; assert the frozen inventory remains authoritative and the validator cannot repair or redefine it.

### I31 — optional validator has zero authority
Run all vectors without semantic-vector and, optionally, through a pinned validator. Assert validator absence does not alter domain expectations; validator success cannot satisfy a manifest role, create evidence, review, admission, or GO.

## Required sequence and handoff

```text
freeze four-file plan
→ independent plan review
→ ADR0011_CALLS_INCOMING_PLAN_GO or REVISE/REJECT
→ separately authorize evidence production
→ land persistent RED tests
→ independently review RED causality
→ produce bounded exact-profile evidence
→ rerun semantic/compatibility/firewall/replay/ceiling tests
→ complete content-addressed predecessor closure
→ independent exact-profile evidence review
→ ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO or REVISE/REJECT
```

Current result is **NO GO**: no test has run and no evidence role is PRESENT.
