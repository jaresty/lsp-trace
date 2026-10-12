# ADR 0011 Go/gopls incoming CALLS qualification-plan review v2

- **Review type:** independent corrected-source exact-profile plan-freeze rereview
- **Repository HEAD:** `c9f8e5a6ab13608aec7a12c1b7128e0343485052`
- **Scope:** four untracked author-owned plan files, the accepted qualified-CALLS framework, and the preserved original independent review
- **Plan decision:** `ADR0011_CALLS_INCOMING_PLAN_GO`
- **Profile qualification:** not performed; no profile GO issued
- **Authority ceiling:** `authority=0`, `accepted=false`, `completeness=UNKNOWN`
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

## Supersession and review boundary

This v2 rereview supersedes only the `ADR0011_CALLS_INCOMING_PLAN_BLOCKED` decision in `docs/qualification/adr0011-calls-go-gopls-incoming-review.md`. It does not overwrite, amend, reinterpret as evidence, or replace that reviewer artifact. The original remains byte-identical at SHA-256 `2ee5a029250649120a3f650680c9a70a3200e8e0260561a58fce14761416cca5` and remains the historical record of the contradiction it identified.

This review decides only whether the corrected four-file qualification plan is coherent enough for the authoring gate `ADR0011_CALLS_INCOMING_PLAN_GO`. It does not require any evidence role to be `PRESENT`; qualify or admit `CALLS_GO_GOPLS_INCOMING_EXACT_V1`; authorize implementation, evidence acquisition, registration, activation, publication, migration, or production effects; or issue `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`.

The accepted framework source is `docs/qualification/adr0011-qualified-calls-profile-review-v2.md`, SHA-256 `0a986b8915a7334f91eaab8038979c118d8685e57c538163d358df2f7c44b46c`. It accepts the additive qualified-CALLS framework while requiring each concrete production profile to have an independent exact identity, custody, evidence chain, review, and profile-scoped GO.

## Exact source pins

### Corrected author sources

| Source | Lines | Bytes | SHA-256 |
|---|---:|---:|---|
| `docs/qualification/adr0011-calls-go-gopls-incoming-profile-plan.md` | 108 | 11,351 | `e9b3d940a50d03d7863197966742bd92953b724a43cfac70b6bf69b866ddbcde` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-evidence-manifest.json` | 117 | 9,755 | `200109580b70d3a10881c80e90e8457707afccea0f634945371f9ce68ce6c276` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-causal-tests.md` | 134 | 12,863 | `45c468fb5784912d654b3d8ae90808e33e4aab06beacc403c5915caac1322407` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-vector-inventory.md` | 135 | 9,980 | `7435a12a63df07fbd2b8996d3d997f165ada820fce690c46fd254923338e9e40` |

Corrected author-source total: **494 lines**, **43,949 bytes**.

### Preserved reviewer source

| Source | Lines | Bytes | Status | SHA-256 |
|---|---:|---:|---|---|
| `docs/qualification/adr0011-calls-go-gopls-incoming-review.md` | 118 | 11,698 | unchanged historical independent review; only its blocked decision is superseded here | `2ee5a029250649120a3f650680c9a70a3200e8e0260561a58fce14761416cca5` |

## Methods

1. Verified the exact worktree HEAD and computed SHA-256, line, and byte pins without modifying the reviewed sources.
2. Read the corrected plan, manifest, causal tests, and vector inventory against the original review's sole blocking contradiction.
3. First checked contradiction closure across I02, I15, and V-P08, including prepare cardinality, target binding, deterministic selection, exact ordinal and item identity, and pre-incoming refusal timing.
4. Reran the complete plan-stage review for profile/direction isolation, method separation, custody, ordering, multiplicity, orientation, terminal/refusal semantics, admission envelope, ceilings, `documentSymbol`, manifest honesty, causal/vector adequacy, compatibility, synthetic firewall, replay, and semantic-vector confinement.
5. Kept plan acceptance separate from evidence production and exact-profile qualification. No `PRESENT` role was required or inferred.

## Original contradiction closure

The sole blocker from the original review is closed consistently.

- **I02** now permits a complete prepare result containing one or more items, binds every candidate to a preexisting target, gives an explicit ordinal precedence only when it is in range and selects a target-bound item, otherwise requires exactly one target-bound match, and records both the selected ordinal and canonical full-item identity.
- **I15** now applies I02's same deterministic rule to a complete array containing one or more items. It no longer limits successful selection to a one-item response. It expressly refuses empty/null, no-match, ambiguity, out-of-range ordinal, reorder, substitution, malformed, and partial cases before incoming dispatch and every effect.
- **V-P08** positively freezes the multi-item case: raw order is retained, an in-range explicit ordinal selects a target-bound item, exact ordinal and canonical full-item identity are recorded, and that exact item passes unchanged to `incomingCalls`.
- **V-NP04 through V-NP10** provide the matching ambiguity, no-match, invalid/nonmatching ordinal, malformed, partial, substitution, and reorder refusal corpus.
- The plan and manifest encode the same precedence and refusal semantics. A one-item result is the ordinary ordinal `0` case, not a separate or exclusive cardinality rule.

The corrected contract therefore has at least one ordered prepare item, target-bound deterministic selection, exact ordinal-plus-item identity, and precise no-match, ambiguity, reorder, and substitution refusals before incoming dispatch and all production-visible effects. No fallback to a different item is permitted.

## Full plan review findings

### 1. Exact profile and direction isolation are coherent

All four corrected files bind only `CALLS_GO_GOPLS_INCOMING_EXACT_V1` in canonical domain `lsp-trace.adr0011.calls.go.gopls.incoming.exact.v1`, for Go, exact gopls custody, and incoming acquisition. Outgoing requires its own identity, custody, vectors, evidence, review, and GO. Bidirectional output is only a policy/composition receipt over separately qualified directions; no third bidirectional acquisition profile is defined. Neither outgoing nor bidirectional material can alias, satisfy, or direction-flip into this incoming profile.

### 2. Prepare and incoming method custody remains separate

`textDocument/prepareCallHierarchy` and `callHierarchy/incomingCalls` are separate ordered transactions and separate manifest roles. The plan retains exact method, params/result bytes or bounded original-preserving records, request and expansion ordinals, response order, limits, completeness, and terminal outcome. Prepare cannot be derived from incoming, and both methods cannot be collapsed into one receipt. The selected prepare item must be exactly the item supplied to incoming, without substitution or lossy remarshal.

### 3. Provider, environment, workspace, repository, file, and session custody are adequate

Planned custody independently binds initialization request/result, client and server call-hierarchy capability, negotiated position encoding, initialization options and terminal state; gopls executable bytes, digest, length, version, argv, working directory, and behavior-relevant environment; Go toolchain/version, relevant `go env`, flags/tags, module/workspace files, and dependency selection; repository commit and dirty-state proof; workspace root; target file bytes/digest/length/URI/language; session ID and exact generation. One-field substitutions require changed identity or typed refusal, never silent equivalence. Same-process provenance and version strings alone are correctly rejected as authority.

### 4. Ordering, multiplicity, and orientation are frozen adequately

For incoming response member ordinal `r` and `fromRanges` ordinal `s`, the method orientation is retained as queried callee `B <- A`, while normalized CALLS orientation is caller `A -> B`. Response, range, request, expansion, and selected-item ordinals remain attached to occurrences. Duplicate equal ranges, repeated equal members, parallel edges, and self-loops remain distinct occurrences. Canonical sorting may be derived but cannot erase original order; pair union, deduplication, or self-loop filtering must be separately identified and source-linked.

The empty non-null `fromRanges` policy is explicitly deferred for freezing before evidence production, while missing or null `fromRanges` is already malformed. That named unresolved policy cannot produce admission and is acceptable at this plan-freeze gate.

### 5. Refusal and terminal semantics are precise

The corrected set distinguishes missing, null, empty, no target-bound match, ambiguous target, invalid or nonmatching explicit ordinal, reordered, substituted, malformed, partial, stale, foreign, server error, unsupported, timeout, cancellation, capability mismatch, environment mismatch, envelope mismatch, and ceiling refusal. Prepare failure yields zero incoming dispatch and zero production-visible effects. Incoming malformed or partial transactions admit no prefix. No repair, hidden retry, ambient reread, reacquisition, alternate method, fallback, self-refresh, or best-effort prefix can yield qualified success.

Incoming `null` and `[]` remain distinguishable and may be successful-empty only if the later frozen exact provider contract admits both. Missing is never reclassified as empty.

### 6. Admission envelope, accounting, and finite ceilings are adequate

The planned envelope binds profile and canonical domain, provider, both methods, session/generation, workspace revision, target, limits, predecessor roles, terminal outcome, and occurrence ledger. Duplicate/unknown/missing/substituted roles and cross-profile descendants are refusal cases.

Finite independent ceilings cover initialization, prepare, incoming expansion, messages, per-request and response bytes, aggregate evidence bytes, requests, nodes, depth, occurrences, ranges, time, logical work, and enforceable allocation. Exact-limit and plus-one vectors require typed no-effect behavior, checked overflow, and balanced attempted/unattempted and captured/uncaptured accounting. No bound may borrow from another.

### 7. `documentSymbol` remains outside CALLS authority

`textDocument/documentSymbol` is separately retained target-selection custody only. Its request/result, ordering, selected symbol, ranges, normalization, ambiguity, absence, malformed outcomes, and every attempted prepare probe are accounted independently. It is not CALLS evidence, cannot replace prepare, cannot authorize an edge, and cannot repair direction or prepared-item identity. Direct-position and symbol-assisted paths remain distinct.

### 8. Manifest is honest and production admission remains refused

The manifest contains **45 required roles**: **35 `MISSING`**, **10 `PLANNED`**, and **0 `PRESENT`**. It records `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `production_admission=REFUSED_MISSING_EVIDENCE`. Its invariant expressly forbids `PRESENT` in this plan-freeze manifest.

This is correct plan-stage accounting. Missing evidence is not a defect in this authoring decision; it is the reason exact-profile qualification and production admission remain unavailable. No prose, matching hash without payload, self-review, privilege, configuration, plan GO, or GO-shaped token can fill a missing role.

### 9. Causal and vector coverage is adequate

The causal specification provides 31 persistent, unexecuted semantic tests covering transaction separation, target-bound prepare selection, initialization/provider/toolchain/workspace custody, exact bytes versus bounded records, target-selection isolation, direction, occurrence multiplicity, ordering, refusal atomicity, envelope identity, ceilings, accounting, replay, privacy, historical compatibility, direction/profile firewalls, synthetic exclusion, independent review, and semantic-vector confinement.

The vector inventory provides 13 positive vectors, 11 negative prepare vectors, 10 negative incoming vectors, 20 one-field custody substitutions, independent exact/plus-one/overflow/unattempted ceiling families, five historical vectors, a broad synthetic-firewall corpus, and five `documentSymbol` vectors. Positive and negative expectations now agree with the corrected deterministic prepare-selection rule.

These artifacts are specifications, not evidence. They are planned and unexecuted and satisfy no evidence role.

### 10. Historical compatibility and synthetic firewall remain closed

Historical omitted-selector `CALLS_ONLY` remains byte-for-byte and behavior-for-behavior unchanged across requests, results, schemas, snapshots, checkpoints, replay, defaults, diagnostics, CLI/MCP behavior, and exit behavior. The additive profile cannot be selected by omission, alias, case variation, family name, outgoing selector, bidirectional selector, or fallback.

Fabricated, fake-server, fixture-built, hand-authored, replay-only, test-only, copied/resealed, self-signed, privileged-caller, flag/config/build-tag, digest-only, and semantic-vector-produced carriers remain non-production. They cannot fill roles or create writes, publications, registrations, admissions, dispatches, checkpoints, receipts, authority changes, review, or GO. No laundering or self-minting path is permitted.

### 11. Replay and no-reacquisition boundary is adequate

Replay is planned solely from retained records with filesystem, workspace, process launch, clock, and LSP callbacks forbidden. Missing, stale, reordered, or substituted records yield one refusal without retry, ambient read, or hidden reacquisition. Exact terminal and canonical outputs remain replay obligations rather than present evidence claims.

### 12. semantic-vector is correctly confined

The expectation inventory is independently authored before any optional semantic-vector use. semantic-vector may compare later observations against frozen expectations, but cannot choose direction, multiplicity, empty-range or duplicate policy, normalization, canonical order, refusal reasons, ceilings, accounting, identity, evidence roles, authority, review, admission, or GO. Validator absence cannot change domain expectations; validator success cannot satisfy a manifest role. A mismatch is diagnostic and cannot rewrite the inventory.

## Decision

The corrected four-file set closes the original I02/I15/V-P08 contradiction and now constitutes one coherent qualification-plan freeze for the exact incoming Go/gopls profile. The complete plan-stage requirements are adequately scoped, causally represented, and honestly bounded by missing evidence.

This decision authorizes only the plan freeze and the separately controlled next step described by that plan. It supplies no evidence, accepts no evidence role, performs no implementation, admits no production profile, and issues no `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` or other profile GO. The original blocked review remains unchanged; only its decision is superseded.

ADR0011_CALLS_INCOMING_PLAN_GO