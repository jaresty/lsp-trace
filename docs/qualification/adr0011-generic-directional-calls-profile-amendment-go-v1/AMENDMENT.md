# ADR 0011 generic directional CALLS profile amendment candidate

**Requested review token:** `ADR0011_GENERIC_DIRECTIONAL_CALLS_PROFILE_AMENDMENT_GO`  
**Candidate state:** `AUTHORED_NOT_GO`  
**Authority ceiling:** `authority=0`, `accepted=false`, repository/runtime completeness `UNKNOWN`  
**Effect:** architecture correction only; no implementation, test mutation, schema, activation, registration, publication, admission, or GO

## 1. Decision and supersession

This amendment freezes provider-neutral directional CALLS semantics separately from provider bindings. It supersedes only gopls-specific semantic assumptions in the directional CALLS qualification architecture. It does not revoke, rewrite, or weaken the accepted opaque raw-custody/32 MiB architecture or its accepted production/test write-set amendment.

Two generic profiles are independent contracts:

- `CALLS_LSP_INCOMING_EXACT_V1`, canonical domain `lsp-trace.adr0011.calls.lsp.incoming.exact.v1`;
- `CALLS_LSP_OUTGOING_EXACT_V1`, canonical domain `lsp-trace.adr0011.calls.lsp.outgoing.exact.v1`.

Two provider bindings are independent contracts:

- `CALLS_GO_GOPLS_INCOMING_EXACT_V1`, binding only to `CALLS_LSP_INCOMING_EXACT_V1`;
- `CALLS_GO_GOPLS_OUTGOING_EXACT_V1`, binding only to `CALLS_LSP_OUTGOING_EXACT_V1`.

Incoming does not inherit outgoing and outgoing does not inherit incoming. A provider binding may narrow provider/runtime/toolchain/workspace/evidence eligibility, but cannot add, delete, reinterpret, relax, or replace generic direction semantics, identity fields, occurrence accounting, terminal distinctions, completeness rules, diagnostics, or review obligations. Equal bytes or shared environment artifacts permit content-addressed reference only, never semantic, evidence, review, acceptance, or GO inheritance.

## 2. Generic incoming contract

`CALLS_LSP_INCOMING_EXACT_V1` is provider-neutral LSP call-hierarchy acquisition with these exact semantics:

1. A target callee is established before preparation under an independently identified target-selection contract.
2. `textDocument/prepareCallHierarchy` is a separate ordered transaction. Its complete ordered result and selected zero-based item ordinal are retained. Selection must be deterministic and target-bound; missing, null, empty, no-match, ambiguous, malformed, partial, stale, substituted, foreign, or over-limit preparation selects nothing.
3. The selected prepared item is passed byte-identically, or through an explicitly qualified exact semantic-preservation rule, to `callHierarchy/incomingCalls`. Re-encoding alone never proves original-byte identity.
4. For queried callee `B`, returned member ordinal `r` with `from=A`, and call-site ordinal `s` in `fromRanges`, the generic relation is `A -> B`. Each admitted `fromRanges[s]` is one server-reported CALLS occurrence in caller `A`; preserve member/range/request/expansion/prepared-item ordinals, duplicate multiplicity, parallel edges, self-loops, source URI, encoding, endpoint items, and exact range.
5. Missing/null member `fromRanges` is malformed. The treatment of a present empty `fromRanges` must be frozen in the exact generic profile evidence before qualification and cannot be supplied by a provider binding after the fact.
6. Complete top-level `null` and `[]` may be successful-empty only if the generic profile’s reviewed terminal policy explicitly distinguishes and admits each form. Missing result, protocol error, unsupported, malformed, partial, timeout, cancellation, resource refusal, and policy refusal remain distinct and cannot be relabeled empty.

No source text, references/definition occurrence, documentSymbol result, endpoint-name match, reverse outgoing edge, repository scan, runtime trace, or provider heuristic may create or repair a generic incoming occurrence.

## 3. Generic outgoing contract

`CALLS_LSP_OUTGOING_EXACT_V1` is provider-neutral LSP call-hierarchy acquisition with these exact semantics:

1. A target caller is established before preparation under an independently identified target-selection contract.
2. `textDocument/prepareCallHierarchy` is a separate ordered transaction with the same deterministic target-bound selection and exact terminal distinctions as incoming, under the outgoing profile’s own identity and evidence.
3. The selected prepared item is passed byte-identically, or through an independently qualified exact semantic-preservation rule, to `callHierarchy/outgoingCalls`.
4. For queried caller `A`, returned member ordinal `r` with `to=B`, and call-site ordinal `s` in `fromRanges`, the generic relation is `A -> B`. Each admitted `fromRanges[s]` is one server-reported CALLS occurrence in queried caller `A`; preserve every ordinal, duplicate, parallel edge, self-loop, endpoint item, URI, encoding, and exact range.
5. Present empty `fromRanges` contributes zero occurrences while remaining a retained successful member. Missing/null `fromRanges` is malformed.
6. Complete top-level `null` and `[]` are distinct successful-empty source forms when admitted by the reviewed generic terminal policy; missing payload and all failure/refusal classes remain distinct.

No incoming direction flip, source text, references/definition occurrence, documentSymbol result, repository scan, runtime trace, or provider heuristic may create or repair a generic outgoing occurrence.

## 4. Independent generic identity, evidence, and review

Each generic direction has its own profile ID, canonical identity domain, schema/policy identities if ever privately introduced, target-set identity, request/response identity, occurrence ledger, terminal/accounting record, evidence manifest, reviewer scope, amendment lineage, and future qualification token. Neither direction can satisfy the other’s missing role.

A generic profile describes LSP method semantics and qualification invariants; it does not assert that any provider implements, supports, or correctly executes them. Generic evidence may qualify only provider-neutral fixtures and contract behavior. Provider observations cannot silently become generic semantic authority.

Generic qualification requires independent review of exact frozen bytes, role completeness, identity-domain separation, direction correctness, multiplicity/ordinal preservation, typed failures, target-relative completeness, diagnostics separation, raw-custody compatibility, and no-effect/default compatibility. Review remains direction-specific.

## 5. Provider-binding contracts

A provider binding must bind, without altering generic semantics:

- exact generic profile ID and canonical-domain digest;
- exact provider/server implementation bytes, digest, length, version report, launch argv, working directory, and behavior-relevant environment;
- exact runtime, adapter, LSP method names, negotiated capabilities and position encoding;
- exact language/toolchain/build flags/module/workspace files and immutable locators or bytes;
- exact repository revision, dirty-state proof, workspace root, target files and target-set identity;
- exact managed session/generation, limits, privacy/retention policy, request/response custody, terminal/accounting records, evidence manifest, and independent review.

`CALLS_GO_GOPLS_INCOMING_EXACT_V1` binds Go/gopls incoming evidence only to `CALLS_LSP_INCOMING_EXACT_V1`. `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` binds Go/gopls outgoing evidence only to `CALLS_LSP_OUTGOING_EXACT_V1`. No binding may inherit another binding’s provider correctness, occurrence ledger, target-set accounting, diagnostic, review, acceptance, or GO. Shared immutable artifacts require independent relevance, freshness, exact-profile binding, and role verification.

## 6. Target-set-relative completeness only

Completeness is relative only to a named immutable target set and named acquisition policy. It never establishes repository, workspace, source-graph, runtime, behavioral, language, provider-family, or production completeness.

For each direction, freeze before execution:

- `target_set_id`: domain-separated identity over exact target-set manifest bytes and selected policy identities;
- `denominator_count`: exact number of distinct target records in that manifest, or `UNKNOWN` when the manifest cannot be completely verified;
- target identity: canonical target ordinal plus exact URI, position or symbol-selection identity, source/revision custody, and profile/domain binding;
- accounting: `denominator = attempted + unattempted`; `attempted = complete_success + complete_empty + failed + unsupported + cancelled + timed_out + refused + partial`; every target ordinal appears exactly once;
- frontier: exact target ordinal/operation stage and reason at which bounded work stopped, plus the ordered set of unattempted targets;
- partial rule: partial wire, partial parse, partial target execution, missing role, unknown denominator, overflow, stale/substituted custody, or incomplete accounting yields `completeness=UNKNOWN` and no completeness claim;
- zero rule: a verified denominator of zero is target-set-empty, not repository-empty; successful zero occurrences over a nonzero verified denominator means only zero admitted occurrences for that exact target set under that exact direction/profile/provider/evidence.

A complete target-set run can claim only: “all targets in target set T were accounted under profile P and evidence E.” It cannot claim all calls, callers, callees, symbols, files, packages, repositories, runtime edges, or provider behavior were covered.

## 7. Diagnostics ownership and non-substitution

Generic diagnostics own only provider-neutral contract classifications: malformed generic LSP shape, missing/null/empty distinctions, ordinal/range/accounting violations, direction violations, identity-domain mismatch, target-set denominator/frontier state, limit/refusal stage, raw-custody presence/completeness class, and generic policy/schema mismatch.

Provider-specific diagnostics own provider launch/version/capability behavior, provider error codes permitted by privacy policy, toolchain/workspace loading, adapter/runtime failures, provider-specific method deviations, environment mismatch, and provider-evidence custody failures.

Neither diagnostic class substitutes for the other. A provider diagnostic cannot establish generic semantic validity, and a generic diagnostic cannot establish provider correctness or explain provider-specific behavior. Diagnostics are not occurrences, success receipts, evidence-role completion, completeness, acceptance, or GO. Redacted diagnostics cannot replace required opaque raw custody; opaque raw bytes do not by themselves establish semantic success.

## 8. The 5900-target corpus is gopls qualification evidence only

The corpus at commit `7f09e5357b117d20449fc16097631e32d4eead7f`, package manifest SHA-256 `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d`, and declared 5900 targets is qualified Go/gopls directional qualification corpus/evidence only.

It is not the semantic definition of either generic profile. It is not a universal denominator, target universe, representative sample, portability proof, conformance oracle, or completeness basis for another provider, toolchain, language, workspace, revision, target-selection policy, incoming run, or outgoing run. Another provider may reference those exact bytes only as an explicitly non-authoritative test input; it must define and review its own target-set identity, denominator, applicability, custody, evidence, and outcomes.

## 9. Compatibility with accepted architecture

This amendment preserves without retyping or reinterpretation:

- sole generic opaque terminal custody owner;
- separate Manager-owned 32 MiB directional reservation ledger, one active slot, exact release/reuse and C15 non-duplication rules;
- original-frame/result/prefix custody, byte/length/SHA-256 handoff checks, incomplete-prefix custody-only semantics, and no-history-change invariant;
- separate incoming/outgoing assemblers and wrapper state;
- `CALLS_ONLY` and disabled-path exact bytes/defaults/behavior;
- Definition/Reference qualification architecture patterns for exact identities, source/revision custody, role-complete evidence, strict terminal distinctions, independent diagnostics, target-relative accounting, independent review, and authority ceilings.

Definition and Reference occurrences, ledgers, schemas, identities, evidence, diagnostics, reviews, or admission are not retyped as CALLS and cannot be imported as CALLS occurrences. CALLS evidence cannot rewrite D/R occurrences.

Any generic profile requirement that needs a public schema/API/selector/config/profile/registry/build tag, public diagnostic, public activation, global/default change, or alteration of accepted raw-custody ownership/accounting is a hard stop: `BLOCKED_PUBLIC_SURFACE_OR_RAW_CUSTODY_CHANGE`.

## 10. Exact impact on halted private interface and frozen write-set

The untracked package `docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1/` is halted and remains byte-identical. This amendment does not edit, adopt, activate, or approve it.

Provider-neutral reusable elements:

- `internal/lspwire/read_frame.go` bounded original-frame/incomplete-prefix mechanics;
- direction-neutral raw-array scanner and ordinal selector;
- sole generic owner/terminal, Manager reservation ledger, C15 mapping, exact transfer/view/release mechanics;
- transaction-scoped private bound and raw byte/length/digest checks;
- separate incoming/outgoing assembler functions as mechanical request construction seams;
- nil/disabled/`CALLS_ONLY` compatibility and history/lineage non-interference.

Names/claims that must be generalized or explicitly scoped before any later implementation review:

- any occurrence of “Go/gopls” as semantic authority for scanner, selector, incoming/outgoing direction, terminal, denominator, or generic diagnostics must become generic LSP profile language;
- gopls/toolchain/workspace/runtime fields must move to provider-binding records, not generic profile identity;
- the 5900-target manifest must be labeled `CALLS_GO_GOPLS_*` qualification corpus/evidence, never generic denominator;
- future qualification tokens, manifests, diagnostics, and occurrence ledgers must name either the exact generic direction or exact provider binding, never an ambiguous “directional CALLS” family authority;
- existing private names containing `callHierarchyDirectional` may remain mechanical implementation names only if review proves they do not encode gopls semantics; no rename is authorized by this amendment.

Frozen future production/test write-set remains exactly the accepted set in `adr0011-directional-raw-custody-32mib-test-writeset-go-v1/`: six production targets and four new test targets. This amendment adds no production or test path. The exact affected documentation for a future separately reviewed amendment is limited to:

- `docs/qualification/adr0011-calls-go-gopls-incoming-profile-plan.md` and its evidence/review companions;
- `docs/qualification/adr0011-calls-go-gopls-outgoing-profile-plan.md` and its evidence/review companions;
- `docs/qualification/adr0011-incoming-outgoing-calls-qualification-v1/` architecture/evidence/review records;
- `docs/qualification/adr0011-incoming-outgoing-raw-custody-32mib-design-v1/` only to add supersession references, never to change accepted custody semantics;
- `docs/qualification/adr0011-callhierarchy-target-corpus-v1/` only to relabel provider-binding applicability without changing corpus bytes;
- the halted private-interface package only after an explicit amendment-reviewed unhalt; until then it is not a write target.

No listed file is modified here. No unlisted file may be added to the future write-set without an explicit versioned amendment and independent review.

## 11. Causal future qualification split

Future work, if separately authorized, must proceed in this causal order:

1. independently review this amendment candidate and issue the exact token only for its bytes;
2. freeze and review `CALLS_LSP_INCOMING_EXACT_V1` generic contract fixtures/evidence;
3. independently freeze and review `CALLS_LSP_OUTGOING_EXACT_V1` generic contract fixtures/evidence;
4. bind and qualify `CALLS_GO_GOPLS_INCOMING_EXACT_V1` against the accepted incoming generic profile and its own target set/evidence;
5. independently bind and qualify `CALLS_GO_GOPLS_OUTGOING_EXACT_V1` against the accepted outgoing generic profile and its own target set/evidence;
6. only then request a separate implementation/test-write authorization under the unchanged raw-custody architecture and frozen write-set.

Failure or incompleteness at one step cannot be repaired by a later step. Generic qualification does not qualify gopls; gopls qualification does not qualify generic semantics; one direction does not qualify the other.

## 12. Rejected alternatives

Rejected:

- one bidirectional generic profile or identity domain;
- one gopls profile defining generic LSP semantics;
- incoming/outgoing inheritance or conversion;
- treating the 5900-target corpus as a generic or cross-provider denominator;
- repository/runtime completeness claims;
- diagnostic substitution for raw evidence, semantic evidence, or provider evidence;
- D/R occurrence retyping or CALLS inference from D/R/source text;
- public schema/API/activation to express the generic split;
- changing global/default limits or accepted raw-custody ownership;
- expanding the production/test write-set implicitly;
- editing or adopting the halted private-interface package in this phase.

## 13. Claim ceiling and disposition

This candidate asserts only a proposed architecture correction over the exact pinned dependency bytes. It establishes no runtime behavior, provider correctness, source completeness, qualification evidence, accepted review, activation, or production authority. `authority=0`, `accepted=false`, all runtime/repository completeness remains `UNKNOWN`, and every future evidence role remains `MISSING` or `PLANNED` until separately produced and reviewed.

`ADR0011_GENERIC_DIRECTIONAL_CALLS_PROFILE_AMENDMENT_GO` may approve only this amendment’s exact bytes and dependency closure. It cannot authorize implementation, tests, evidence acquisition, schema publication, activation, profile qualification, production admission, or either provider-direction GO.

AUTHORED_NOT_GO
