# ADR 0011 Go/gopls incoming CALLS exact-profile qualification plan

**Profile:** `CALLS_GO_GOPLS_INCOMING_EXACT_V1`  
**State:** qualification-plan freeze only; `PLANNED`, `UNEXECUTED`, `DEFAULT_OFF`  
**Authority ceiling:** `authority=0`, `accepted=false`, `completeness=UNKNOWN`  
**Production admission:** `REFUSED_MISSING_EVIDENCE`

This file plans qualification; it produces no implementation, runner, evidence, receipt, review, admission, registration, publication, migration, or GO. The authoring gate is `ADR0011_CALLS_INCOMING_PLAN_GO`. A later evidence package may proceed only under `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO`, jointly scoped to the exact profile ID `CALLS_GO_GOPLS_INCOMING_EXACT_V1` and its exact canonical identity domain. Neither token is issued here.

## Profile boundary

The sole acquisition profile is incoming Go call hierarchy from an exact gopls process:

- language: Go;
- server/provider: exact captured gopls binary;
- preparation method: `textDocument/prepareCallHierarchy`;
- edge method: `callHierarchy/incomingCalls`;
- normalized relation: server-reported `CALLS`, caller to callee;
- canonical domain: `lsp-trace.adr0011.calls.go.gopls.incoming.exact.v1`.

Outgoing acquisition is independent: it requires its own profile identity, custody, vectors, evidence, review, and GO. Bidirectional output is only a policy/composition receipt over separately qualified incoming and outgoing evidence unless a future amendment explicitly adds extra acquisition semantics. This plan does not define a third bidirectional acquisition profile.

Historical omitted-selector `CALLS_ONLY` remains byte-for-byte and behavior-for-behavior unchanged. No historical selector, request, result, schema, snapshot, checkpoint, replay, CLI/MCP behavior, canonical identity, or claim ceiling is wrapped, migrated, reissued, or retrofitted.

## Frozen acquisition transaction

Qualification must retain the two LSP operations as separate ordered transactions. They may not be collapsed into one derived record.

1. **Prepare transaction**
   - exact `textDocument/prepareCallHierarchy` request identity and bytes (or a bounded method record that preserves the exact method, params bytes, response bytes, ordering, limits, and terminal outcome);
   - exact result cardinality and raw response order for every returned item; a complete result may contain one or more items;
   - target binding derived before prepare selection: for a documentSymbol path, exact canonical URI, name, kind, range, and selectionRange; for a direct-position path, exact canonical URI plus the requested position contained by both range and selectionRange;
   - deterministic selection precedence: an explicit ordinal, when supplied, is considered first but succeeds only when it is in range and its item satisfies the target binding; otherwise, with no explicit ordinal, exactly one target-bound match selects, zero matches refuses as no match, and more than one match refuses as ambiguous;
   - selected prepared-item array index/ordinal and canonical full-item identity, including name, kind, URI, range, selectionRange, detail/tags/data when present; a one-item result is the ordinary ordinal `0` case and is not a separate cardinality rule;
   - proof that the selected prepared item is the exact item passed to incomingCalls without substitution or lossy remarshal.
2. **Incoming transaction(s)**
   - one separately identified `callHierarchy/incomingCalls` request per expanded prepared item;
   - exact request item identity, exact result bytes or bounded method record, response order, request ordinal, expansion ordinal, and terminal outcome;
   - each returned member retained as one occurrence before pair normalization.

If exact JSON-RPC bytes are unavailable, the bounded method record must preserve enough original bytes and framing/accounting to distinguish missing, null, empty array, malformed JSON, malformed member, partial capture, server error, cancellation, timeout, ceiling refusal, and complete response. Re-encoding alone cannot claim original-wire identity.

## Required custody

The evidence plan freezes and cross-binds:

- initialize request/result and capability custody, including client call-hierarchy capability, server `callHierarchyProvider`, negotiated position encoding, initialization options, and terminal initialization state;
- gopls executable bytes, SHA-256 digest, byte length, filesystem locator, reported `gopls version`, launch argv, working directory, and process environment relevant to behavior;
- Go toolchain bytes/digests or immutable locators, `go version`, `go env` fields affecting loading/build selection, build flags/tags, module/workspace files, and environment allowlist;
- repository commit, dirty-state proof, workspace root URI/path, target file bytes/digest/length/URI, language ID, position encoding, session ID, and exact generation;
- request timeouts and finite message/byte/request/node/depth/occurrence/range/work ceilings;
- admission-envelope identity binding profile, canonical domain, provider, methods, session/generation, workspace revision, target, limits, predecessor roles, and terminal outcome.

Same-process custody is not producer authentication. A reported version string without binary bytes/digest is insufficient. A clean commit without dirty-state proof is insufficient.

## Target selection custody

`textDocument/documentSymbol` is a separate target-selection input. It may locate a candidate position and must retain its own request/result, response ordering, selected symbol identity, selectionRange, normalization, ambiguity/absence/malformed outcomes, and custody. It is never CALLS evidence, never a substitute for `textDocument/prepareCallHierarchy`, and never authorizes an edge. Only a separately successful prepare transaction selects the prepared item used by incomingCalls.

Direct position selection remains distinct from symbol selection. Any documentSymbol-assisted probe sequence retains every attempted prepare request in order; no failed probe may be erased from accounting.

## Occurrence and normalization contract

For each incoming response member at response ordinal `r` and each `fromRanges` member at range ordinal `s`:

- pre-normalization orientation is `queried_callee <- returned_from_caller` exactly as represented by the method;
- post-normalization orientation is `returned_from_caller -> queried_callee`;
- retain response ordinal `r`, range ordinal `s`, request ordinal, expansion ordinal, and selected prepared-item ordinal;
- retain the exact returned `from` item and every `fromRanges` entry in response order;
- admit one occurrence per returned member/range according to the frozen empty-range rule; never infer a range;
- preserve duplicate equal ranges and repeated equal endpoint pairs as distinct occurrences;
- preserve parallel edges and self-loops as occurrences; any later pair union, deduplication, or self-loop exclusion is a separately named normalization/composition policy and cannot rewrite the occurrence ledger;
- canonical sorting may produce a derived order but must preserve original ordinals.

The qualification owner must freeze whether a non-null empty `fromRanges` member contributes a zero-site edge record or zero occurrences before evidence production. `null`/missing `fromRanges` is malformed and cannot be treated as empty.

## Refusal and terminal matrix

Qualification must distinguish and persist typed outcomes for:

- prepare result missing, `null`, empty, no target-bound match, multiple target-bound matches without a valid explicit ordinal, explicit ordinal out of range or bound to a nonmatching item, malformed, partial, stale, foreign-workspace, foreign-session/generation, substituted, reordered, or ceiling-blocked;
- incoming result missing, `null`, empty, malformed top level, malformed first/later member, partial prefix, stale, foreign, substituted, server error, unsupported, timeout, cancellation, or ceiling-blocked;
- selected-item mismatch, changed opaque `data`, changed URI/range/selectionRange/kind/name, response reordering, ordinal collision, duplicate role, admission-envelope mismatch, capability mismatch, gopls/toolchain/environment mismatch, repository/file mismatch, and encoding mismatch.

Incoming `null` and `[]` are both successful-empty only if the frozen provider contract explicitly qualifies both forms; they remain distinguishable evidence. Prepare missing, `null`, or `[]` never selects a target. Any prepare no-match, ambiguity, out-of-range ordinal, reorder, substitution, partial, or malformed outcome refuses before incoming dispatch and before every production-visible effect. No fallback, repair, ambient reread, hidden reacquisition, alternate method, self-refresh, or best-effort prefix may yield qualified success.

## Ceilings and no-effect refusal

Before any execution, freeze finite numeric ceilings independently for initialization, prepare, incoming expansion, wire messages, wire bytes, response bytes, total evidence bytes, requests, nodes, depth, occurrences, ranges, time, logical work, and enforceable allocation. Test exact-limit behavior and plus-one refusal. Ceiling refusal occurs before any production-visible write, publication, registration, admission, dispatch, checkpoint, receipt, or authority change, with balanced attempted/unattempted and captured/uncaptured accounting.

## Planned phases

1. Freeze exact schemas, identity domain, selector isolation, numeric ceilings, and admission envelope.
2. Freeze historical `CALLS_ONLY` compatibility bytes and synthetic firewall controls.
3. Freeze independently authored vectors and persistent semantic tests.
4. Plan exact environment, initialization/capability, prepare, incoming, target-selection, occurrence, normalization, refusal, replay, privacy, and accounting evidence roles.
5. After `ADR0011_CALLS_INCOMING_PLAN_GO` only, separately authorize evidence production.
6. Produce immutable evidence, if authorized, without changing this plan's authority.
7. Independently review exact bytes and predecessor closure.
8. Permit `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` only when every required role is PRESENT, valid, fresh, exact-profile bound, independently reviewed, and all tests pass. GO has no family, outgoing, bidirectional, historical, alias, successor, or equal-byte implication.

## semantic-vector boundary

The vector inventory and every expected direction, multiplicity, ordinal, normalization step, typed outcome, refusal reason, canonical byte expectation, and accounting equation must be authored and frozen before semantic-vector is considered. semantic-vector is an optional test-only validator. It cannot determine, infer, generate, repair, or approve direction, multiplicity, normalization, expected outcomes, evidence roles, authority, review, or GO; its output cannot satisfy a manifest role.

## No-GO handoff

Current handoff is **NO GO**. The only requested next decision is whether the four-file qualification-plan freeze is coherent enough for `ADR0011_CALLS_INCOMING_PLAN_GO`. All evidence roles are `MISSING` or `PLANNED`; none is `PRESENT`; `production_admission` remains `REFUSED_MISSING_EVIDENCE`. No evidence acquisition or implementation begins from this handoff, and `ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO` is unavailable.
