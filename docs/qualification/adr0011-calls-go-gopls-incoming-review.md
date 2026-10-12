# ADR 0011 Go/gopls incoming CALLS qualification-plan review

- **Review type:** independent exact-profile plan-freeze review
- **Repository HEAD:** `c9f8e5a6ab13608aec7a12c1b7128e0343485052`
- **Scope:** four untracked author-owned plan files, accepted qualified-CALLS framework, and the current real incoming source path
- **Plan decision:** blocked pending one semantic correction
- **Profile qualification:** not performed; no profile GO issued
- **Production admission:** `REFUSED_MISSING_EVIDENCE`

## Review boundary

This review decides only whether the four-file qualification plan is coherent enough for `ADR0011_CALLS_INCOMING_PLAN_GO`. It does not require any evidence role to be `PRESENT`, does not qualify `CALLS_GO_GOPLS_INCOMING_EXACT_V1`, and does not authorize implementation, evidence acquisition, registration, activation, publication, migration, or `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`.

The accepted framework source is `docs/qualification/adr0011-qualified-calls-profile-review-v2.md`, which accepted the additive qualified-CALLS framework while preserving independent concrete-profile qualification, historical `CALLS_ONLY`, exact identity custody, the synthetic firewall, and the distinction between plan/framework acceptance and production admission.

## Exact author-source pins

| Source | Lines | Bytes | SHA-256 |
|---|---:|---:|---|
| `docs/qualification/adr0011-calls-go-gopls-incoming-profile-plan.md` | 107 | 10,279 | `7086863a3d08a48ecb79b4a1c069680417d51409998ebfa6a2210d9ce8ad4c9c` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-causal-tests.md` | 134 | 11,802 | `50c808e404044fd6567cdac025c5d0518ba4dfe51ef0ec8963031c72394e23d7` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-vector-inventory.md` | 133 | 9,359 | `86c0a29a4c7e765be23263b328d82ea38afe732025207ef7b2b0ba77a9d99cd9` |
| `docs/qualification/adr0011-calls-go-gopls-incoming-evidence-manifest.json` | 98 | 8,364 | `dae05b8ce028181975e760ad608416b61d2243b6e4b6b7450e1ecf0e538249f0` |

Total: **472 lines**, **39,804 bytes**.

## Mechanical manifest result

- exact profile: `CALLS_GO_GOPLS_INCOMING_EXACT_V1`;
- canonical domain: `lsp-trace.adr0011.calls.go.gopls.incoming.exact.v1`;
- required roles: **45**;
- `MISSING`: **35**;
- `PLANNED`: **10**;
- `PRESENT`: **0**;
- authority: **0**;
- accepted: **false**;
- completeness: **`UNKNOWN`**;
- production admission: **`REFUSED_MISSING_EVIDENCE`**.

This is honest plan-stage accounting. Evidence being absent is not a reason to block plan acceptance; it is the reason profile qualification remains unavailable.

## Real incoming source-path comparison

The current incoming path is materially represented by:

1. `internal/lsp/client.go`: calls `callHierarchy/incomingCalls`, decodes the raw result, and reports `null` through a boolean;
2. `internal/acquisition/wire.go`: sends bounded method requests, preserves method and marshaled params at the adapter boundary, distinguishes `null`, validates shape, and strictly decodes responses;
3. `internal/acquisition/coordinator.go`: selects incoming versus outgoing, constructs caller-to-callee edges for incoming results, canonical-sorts candidates, validates ranges/items, and feeds the current graph path;
4. `internal/acquisition/evidence.go`: reconstructs incoming orientation by swapping queried and returned endpoints, then uses `graph.MergeEdge` and validates a canonical call-site union.

That path supports the plan's need for a distinct exact profile. Current normalized graph behavior is not an occurrence ledger: canonical sorting and `graph.MergeEdge` do not by themselves preserve response-member ordinal, range ordinal, duplicate equal occurrences, parallel occurrence identity, or all pre-normalization evidence. The plan correctly refuses to treat current graph normalization, source text, or `documentSymbol` as qualified exact-profile CALLS evidence.

## Findings

### 1. Exact profile and directional isolation are correctly scoped

All four files consistently name only `CALLS_GO_GOPLS_INCOMING_EXACT_V1`. Outgoing evidence cannot satisfy an incoming role or be direction-flipped into this profile. Bidirectional behavior is limited to a policy/composition receipt over separately qualified incoming and outgoing predecessors; it creates no acquisition fact and defines no third bidirectional acquisition profile.

### 2. Prepare and incoming transactions are correctly separated

The plan, manifest, I01, and custody vectors require distinct prepare request/result evidence and distinct evidence for every incoming request/result. They preserve request, response, terminal, order, limit, and capture-completeness identity and prohibit deriving prepare from incoming or collapsing both operations into one receipt.

### 3. Material blocker: prepared-item selection rule is contradictory

The four-file freeze does not state one coherent rule for multi-item prepare results:

- I02 requires a multiple-item response to be selectable by explicit response ordinal and tests field/ordinal substitution;
- V-P08 treats multiple prepare items plus an explicit selected index as a positive vector;
- the profile plan likewise retains selected array index/ordinal and says the qualification owner must bind the selected item;
- but I15 states that **only the explicitly qualified one-item complete case may select a target**, making a complete multiple-item result ineligible even when the exact ordinal is supplied.

These are incompatible expectations for the same input class. Consequently the prepared-item selection policy, ambiguity boundary, canonical identity, positive corpus, and refusal corpus are not frozen. Before plan GO, the author must choose exactly one rule and make the plan, manifest role semantics, I02, I15, V-P08, and relevant substitution vectors agree. Either:

1. exact explicit ordinal selection of a complete multi-item result is permitted, with missing/out-of-range/duplicate-policy ambiguity refused; or
2. every multi-item prepare result is refused, in which case I02 and V-P08 must cease describing a positive selected-item path.

This is a plan-coherence defect, not missing production evidence.

### 4. Capability, executable, environment, repository, and session custody are adequate as planned roles

The plan and vectors independently bind initialize request/result, call-hierarchy capabilities, position encoding, initialization state/options; exact gopls bytes, digest, length, version, argv, working directory, and behavior-relevant environment; Go toolchain/version and relevant `go env`, build flags/tags, module/workspace bytes; commit, dirty-state proof, workspace root, target file bytes/digest/length/URI/language, session, and generation. Substitution vectors require changed identity or typed refusal rather than silent reuse.

### 5. Wire/canonical method custody and ordering are adequate

Exact JSON-RPC bytes are preferred. The bounded alternative must preserve original method, params/result bytes, ordering, framing/accounting, limits, terminal outcome, and enough information to distinguish missing, null, empty, malformed, partial, error, cancellation, timeout, and ceiling refusal. Re-marshaled semantic equality cannot claim original-wire equality. Original response/range/request/expansion/selected-item ordinals remain required even when a derived canonical sort exists.

### 6. Direction, multiplicity, and occurrence semantics are adequate apart from the selection blocker

The planned pre-normalization orientation is queried callee `B <- A` and the post-normalization CALLS orientation is caller `A -> B`. One occurrence is retained per returned member/range under the separately frozen empty-range rule. Duplicate equal ranges, repeated equal members, parallel edges, and self-loops remain separate occurrences with original ordinals. Any pair union, deduplication, self-loop exclusion, or canonical order is a separately identified transform and cannot replace the occurrence ledger.

The empty-`fromRanges` rule is explicitly deferred for freezing before evidence production; missing or null `fromRanges` is already refused. That deferral is acceptable for this plan-stage review because it is named as an unresolved policy and cannot yield admission meanwhile.

### 7. Refusal matrix, envelope, accounting, and ceilings are adequate

The plan distinguishes missing, null, empty, partial, malformed, stale, foreign, substituted, error, unsupported, timeout, cancellation, and ceiling outcomes. Partial or malformed transactions admit zero qualified occurrences; no prefix admission, repair, ambient reread, hidden reacquisition, fallback, self-refresh, or alternate method is allowed. The admission envelope binds the exact profile/domain, provider, methods, environment, target, limits, predecessor roles, terminal outcome, and occurrence ledger. Finite independent ceilings cover messages, bytes, requests, nodes, depth, occurrences, ranges, time, work, and enforceable allocation, including exact-limit and plus-one no-effect refusal with closed accounting.

### 8. `documentSymbol` boundary is correct

`textDocument/documentSymbol` is separately custodial target-selection input only. Its request/result, order, selected symbol, ranges, normalization, and failures are retained. It is never CALLS evidence, never a replacement for `textDocument/prepareCallHierarchy`, never edge authority, and cannot repair direction or CALLS semantics. Every prepare probe remains separately accounted.

### 9. Causal/vector coverage and semantic-vector boundary are otherwise adequate

The specification provides 31 named causal tests, 13 positive vectors, 9 negative-prepare vectors, 10 negative-incoming vectors, 20 one-field custody substitutions, independent ceiling/accounting families, five historical-compatibility vectors, synthetic-firewall vectors, and five `documentSymbol` vectors. These cover the requested identity, isolation, transaction, custody, direction, multiplicity, refusal, replay, privacy, compatibility, and authority boundaries.

semantic-vector is optional, test-only, unadopted, and unexecuted. Expectations must be independently authored first. The validator cannot choose direction, multiplicity, empty-range policy, normalization, expected outcomes, ceilings, evidence, review, admission, or GO, and its output cannot satisfy a manifest role. The prepared-item contradiction must be corrected by human/domain custody, not resolved by validator behavior.

### 10. Historical compatibility and synthetic firewall are correct

Historical omitted-selector `CALLS_ONLY` remains byte-for-byte and behavior-for-behavior unchanged across requests, results, schemas, snapshots, checkpoints, replay, defaults, diagnostics, CLI/MCP behavior, and exits. The additive profile cannot be reached by omission or fallback.

Fabricated, fake-server, fixture, replay-only, hand-authored, copied/resealed, self-signed, privileged-caller, flag/config/build-tag, and semantic-vector carriers remain non-production. They cannot fill missing roles or create writes, publications, registrations, admissions, dispatches, checkpoints, receipts, authority, review, or GO.

## Decision

The incoming plan is comprehensive and correctly distinguishes plan acceptance from exact-profile qualification. Missing evidence is expected and honestly refused. However, the contradictory multi-item prepared-selection rule prevents the four files from constituting one frozen semantic contract. Correct that rule consistently across the plan, manifest semantics, causal tests, and vectors, then rerun an independent plan review with new hashes.

No profile GO is issued.

ADR0011_CALLS_INCOMING_PLAN_BLOCKED