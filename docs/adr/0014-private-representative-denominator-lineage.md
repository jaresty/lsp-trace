# ADR 0014: Retain a complete representative nomination denominator and community lineage

- **Status:** Accepted additive private implementation and qualification contract
- **Date:** 2026-10-25
- **Accepted:** 2026-10-25
- **Decision authority:** @jaresty (GitHub handle)
- **Decision owners:** LSP Trace maintainers
- **Accepted proposal SHA-256:** `85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda`
- **Independent review:** `ACCEPT` after all blocking findings were corrected; task `5957dec3-f1c9-419`
- **Accepted receipt-discovery amendment SHA-256:** `df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57`
- **Receipt-amendment independent review:** `ACCEPT`; task `ca520100-8448-475`
- **Depends on:** [ADR 0011](0011-versioned-definition-reference-grouping-evidence.md), [ADR 0012](0012-private-census-to-candidate-orchestration-bridge.md), and [ADR 0013](0013-resource-blocked-program-c-instability.md)
- **Extends without modifying:** historical Program C receipts, community-instability v1/v2/v3, Stage 6 representative mechanics, and the Describe-preparation gap-ledger v1

## Context

The qualified private workflow now advances through all 395 frozen Program C constituents and reaches `WAITING_EVIDENCE`. The retained gap ledger truthfully reports that representative nomination and candidate preparation are `NOT_COMPUTED` and that these evidence requirements are `NOT_RETAINED`:

- representative nomination denominator;
- community-to-representative lineage;
- complete retained source-byte closure;
- `FULL_DEFINITION` display bindings;
- caller-occurrence V3 custody.

Existing Stage 6 mechanics can deterministically select a representative or a typed non-selection for each admitted community and selection context. They do not resolve the frozen Program C head, reconstruct exact communities from retained source custody, define the complete expected nomination population, or retain lineage into the inventory successor.

Program C transition receipts retain result bytes, digests, lengths, ordinals, and predecessor bindings. Complete instability artifacts bind community artifact hashes but do not themselves retain all community bytes, CALLS occurrences, or Stage 6 selection contexts. Direct caller reconstruction, checkout reads, language-server reacquisition, historical fixture reuse, or community omission would violate the accepted custody boundary.

## Decision required

Add a private, immutable bridge that derives and retains two coupled artifacts from verified frozen Program C custody:

1. a complete representative nomination denominator;
2. a total community-to-representative lineage bound to that denominator.

No implementation begins merely because this proposal exists. Implementation requires independent review, explicit acceptance by the decision authority, and a frozen proposal digest.

## Scope and authority

This contract authorizes only private implementation and qualification of denominator and lineage custody. It does not authorize:

- Describe packet construction;
- candidate preparation;
- feature identity, purpose, ownership, value, or completeness claims;
- stakeholder correction or semantic acceptance;
- public CLI or MCP registration;
- production admission, release, or deployment.

Every artifact remains:

```text
authority=0
accepted=false
featureIdentity=UNRESOLVED
completeness=UNKNOWN
effects=NONE
reviews=NOT_REQUESTED
implementationDecision=UNISSUED
production_admission=REFUSED_MISSING_EVIDENCE
source_graph_complete=UNKNOWN
absence_inference=PROHIBITED
```

## Exact predecessor custody

Derivation begins only from:

- the verified terminal Program C head;
- its exact ordered transition plan;
- every immutable Program C receipt in committed-prefix order;
- the exact frozen Census receipt and availability ledger;
- exact retained Native V5 constituent bytes resolved through the already verified publication resolver;
- the accepted Program C bindings, source pin, policy identities, limits, seeds, and replay count.

The bridge may deterministically recompute Program C from exact retained Native V5 bytes solely to materialize a new immutable carrier. This is not checkout fallback or language-server reacquisition. Each recomputed result must match the retained receipt and instability evidence before it can contribute positively.

### Complete receipt inventory

The terminal Program C head names only its latest receipt. Before denominator derivation, a repository-owned read-only verifier must establish every receipt in plan-ordinal order. It must either:

1. deterministically reconstruct every claim, head, and receipt selector from the initial head, exact plan entry, bindings, recomputed result bytes, and predecessor selector; or
2. verify a separately retained content-addressed complete receipt manifest linked from verified terminal custody.

For historical v2 instability result bytes, the receipt schema is not encoded in the plan, bindings, result bytes, or predecessor selector. The verifier therefore performs bounded repository-owned schema probing at each ordinal:

1. derive the complete canonical v3 receipt and selector;
2. derive the complete canonical v4 receipt and selector;
3. load both derived selectors without listing the store;
4. admit exactly one candidate only when its retained bytes exactly equal the derived canonical bytes;
5. reject zero matches as missing and two matches as ambiguous/duplicate.

For result bytes that require v4, only the canonical v4 candidate is admissible; a v3 representation is invalid rather than an alternative. No other schema candidate may be probed. This rule is deterministic and accepts no caller-supplied schema or selector.

The verifier requires exact ordinal coverage, canonical receipt bytes, exact result bytes/digest/length, claim-predecessor equality, and a reconstructed final head byte-identical to the independently verified terminal head. Caller-supplied receipt selectors are prohibited. A missing, foreign, duplicate, unreachable, or unverifiable receipt in the reconstructed chain contributes to `I`; it cannot be omitted. Unrelated or CAS-orphaned objects outside the reconstructed selector set neither prove nor disprove chain completeness and need not be enumerated.

The bridge must not:

- read ambient checkout source;
- invoke or reacquire an LSP session;
- accept caller-supplied ordinals, selectors, processed sets, limits, seeds, retry policy, skip policy, stage state, or outcomes;
- substitute historical Stage 6 fixtures, candidates, groups, Describe packets, or semantic results;
- reinterpret a resource-incomplete constituent as complete or empty.

## Denominator unit

The nomination unit is one exact **Program C run/community selection context**:

```text
constituent ordinal
× canonical run identity
× canonical community ordinal and membership
× exact Stage 6 selection context
```

Every eligible run/community contributes exactly one denominator entry. Multiple seeds and replays remain distinct. Byte-identical community memberships from different run identities remain distinct entries and cannot be collapsed.

A denominator entry binds:

- workflow identity and terminal Program C head;
- transition plan and receipt selector;
- constituent ordinal and immutable constituent selector;
- Program C outcome and receipt disposition;
- Native V5 selector, digest, and byte length;
- instability schema, selector, digest, and byte length;
- run ID, seed, replay ordinal, and run status;
- community artifact digest and exact canonical bytes;
- community ordinal and canonical ordered members;
- exact admitted CALLS occurrence identities and relation identities;
- source predecessor and custody identities;
- Stage 6 selection-context identity, execution-bundle identity, prepared targets, reached nodes, and exact relation-occurrence closure;
- Stage 6 policy, implementation, qualification, limits, and claim-ceiling identities.

## Deterministic Native V5 to Stage 6 adapter

For each eligible run/community, a repository-owned adapter creates one singleton `RepresentativeComposition`, one singleton `RepresentativeGrouping`, and exactly one `RepresentativeSelectionContext`:

- each `(edge, CallSites[ordinal])` pair becomes exactly one `RepresentativeEvidence` record; a zero-call-site edge is rejected;
- evidence identity is a canonical digest over constituent, execution bundle, relation ID, occurrence ordinal, endpoints, and exact call-site range;
- relation identity is the Native V5 `relation_id`; `From`/`To` are `caller_node_id`/`callee_node_id`; ordinal is canonical edge/call-site order; call site is copied exactly;
- predecessor and custody bind the Native V5 selector, complete receipt selector, execution bundle, and source publication; disposition is `INCLUDED` only for admitted `CALL_RELATION` evidence under `CALLS_ONLY_FROZEN_CENSUS_V1`;
- the recomputed community artifact supplies a singleton partition containing its exact sorted members;
- Native V5 seed memberships for the exact execution bundle, seed label, and seed position map `PREPARED_TARGET` endpoint IDs to `PreparedTargets`, `REACHED_NODE` endpoint IDs to `ReachedNodes`, and `CALL_RELATION` endpoint IDs to `RelationIDs`;
- `OccurrenceIDs` is the complete closure of every admitted call-site occurrence for those relation IDs;
- composition/grouping input, policy, qualification, limits, accounting, digest, and byte-length fields are derived from canonical predecessor and adapter bytes, never caller text;
- `ModelJudgment` and `ModelAdvice` are empty and cannot be caller-authored;
- context identity is a canonical digest over the exact constituent, run, community, seed-membership closure, and adapter contract.

Unknown membership kinds, duplicate membership/evidence/context identities, missing relations, partial occurrence closure, noncanonical ordering, mixed execution bundles, or mismatched seed labels/positions reject derivation. Exactly one derived context must exist for each denominator entry. Zero or multiple candidate contexts fail closed; arbitrary seed selection, context collapse, and context splitting are prohibited absent a separately accepted contract.

## Constituent classification

Every Program C plan constituent appears exactly once in constituent accounting.

### `COMPLETE`

A complete constituent is eligible only when:

1. exact retained Native V5 bytes resolve through verified publication custody;
2. the six declared runs recompute under the accepted seeds and replay inventory;
3. every recomputed community artifact hash agrees with the hashes bound by retained instability bytes;
4. run identities, community order, members, occurrences, and selection contexts validate canonically.

Every admitted run/community becomes one denominator entry and one singleton Stage 6 input.

### `EMPTY`

An empty constituent is terminally evaluated with zero denominator entries. It remains explicit in constituent accounting. It does not nominate a representative and does not establish feature absence.

### `COMMITTED_INCOMPLETE_MATCHING_RESOURCE`

A resource-incomplete constituent remains explicit in the declared constituent denominator but is ineligible for nomination, Describe preparation, successful-instability denominators, and absence or stability inference.

It contributes zero nomination entries and a typed exclusion record. Its complete runs and community hashes cannot be promoted into Stage 6 through this bridge.

### Any other state

Missing, malformed, stale, mixed-version, failed, indeterminate, conflicting, substituted, unreceipted, or unknown predecessor state makes the denominator artifact `INCOMPLETE`. It cannot be treated as empty or silently omitted.

## Denominator artifact

Add a canonical, content-addressed private artifact:

```text
ProgramCRepresentativeDenominator
```

It contains:

- the complete ordered constituent classification inventory;
- the complete ordered nomination-entry inventory;
- exact predecessor selectors, digests, lengths, and identities;
- balanced accounting;
- authority ceilings;
- an immutable selector derived from canonical bytes.

Canonical order is:

```text
constituent ordinal
→ run ID
→ community canonical member bytes
→ community ordinal
```

Unknown fields, duplicate identities, duplicate ordinals, missing records, noncanonical order, identity substitution, trailing bytes, arithmetic overflow, or failed accounting reject the artifact.

## Denominator accounting

Let:

- `P` = declared Program C constituents;
- `C` = complete constituents;
- `E` = empty constituents;
- `R` = committed-incomplete matching-resource constituents;
- `I` = invalid, missing, or otherwise incomplete constituents;
- `D` = eligible run/community denominator entries.

Mandatory equations:

```text
P = C + E + R + I
classified_constituent_records = P

evaluated_constituents = C + E
excluded_constituents = R
unresolved_constituents = I

D = sum(exact admitted community count across every eligible COMPLETE run)
denominator_entry_count = D
```

Let `K = accepted_seed_count × replay_count_per_seed`; under the frozen bindings, `K = 2 × 3 = 6`. Additional mandatory conservation equations are:

```text
terminal_run_records = K × (C + E + R)
eligible_complete_run_records = K × C
empty_run_records = K × E
resource_excluded_run_records = K × R

D = sum(canonical community count over every eligible complete run)
selection_context_count = D
```

Every expected run identity occurs exactly once in its applicable class. All multiplication and summation use checked arithmetic.

A complete denominator artifact requires `I = 0`. `R` may be nonzero because ADR 0013 permits structural-prefix advancement, but every such record remains explicitly excluded and contributes zero entries.

`D = 0` is a complete zero-entry denominator only when all `P` constituents are terminally classified, `I = 0`, and every constituent is `EMPTY` or explicitly resource-excluded. This does not establish feature absence.

## Lineage artifact

Add a second canonical, content-addressed private artifact:

```text
ProgramCRepresentativeLineage
```

It binds one terminal Stage 6 record to every denominator entry.

Each denominator entry is evaluated as one singleton Stage 6 computation: one partition community, one selection context, one `RepresentativeResult`, and one `RepresentativeRecord`. The aggregate result outcome and its sole record outcome must agree and be exactly one of:

```text
SELECTED
NO_CALLS_REPRESENTATIVE
CALLS_EVIDENCE_INELIGIBLE
```

`EMPTY_COMMUNITY` is valid only for a separately verified zero-entry aggregate and creates no lineage record. `MIXED_COMMUNITY_OUTCOMES` is prohibited in the singleton lineage path.

The lineage record binds:

```text
denominator entry
→ exact community artifact and members
→ exact selection context
→ exact admitted CALLS evidence closure
→ Stage 6 result digest and canonical bytes
→ representative record ordinal (zero) and exact canonical bytes
```

For `SELECTED`, it additionally binds the selected node, distance, SCC members, and incoming predecessor witnesses. A non-selection remains an explicit terminal record; it is not omitted and does not establish semantic absence.

## Lineage accounting

Let:

- `D` = denominator entries;
- `S` = `SELECTED` records;
- `N` = `NO_CALLS_REPRESENTATIVE` records;
- `X` = `CALLS_EVIDENCE_INELIGIBLE` records;
- `U` = missing, invalid, or unqualified records.

Mandatory equations:

```text
D = S + N + X + U
stage6_singleton_result_count = D
lineage_record_count = S + N + X
```

A complete lineage artifact requires `U = 0` and:

```text
lineage_record_count = S + N + X = D
```

Every denominator identity appears exactly once. Every lineage identity references exactly one denominator identity. Foreign, duplicate, missing, reordered, or substituted records reject the artifact.

## Immutable storage and heads

Use separate private namespaces for immutable denominator/lineage/qualification objects, a denominator head, and a lineage head.

Expose exactly three separately invokable one-CAS operations:

1. retain and read back the denominator object, then CAS the denominator head against the exact loaded denominator revision;
2. after close/reopen, verify the denominator head, retain and read back the lineage object, then CAS the lineage head against the exact loaded lineage revision and verified denominator selector;
3. after close/reopen, verify both evidence heads and CAS a v2 inventory-successor head against the exact verified v1 successor revision.

No operation automatically starts the next. The v2 successor transition marks only these two requirements `RETAINED`, preserves the remaining three as `NOT_RETAINED`, keeps candidate preparation `NOT_COMPUTED`, and remains `WAITING_EVIDENCE`.

CAS outcomes remain:

```text
not committed      → CONFLICT
commit uncertain   → INDETERMINATE_EXECUTION
```

Neither outcome permits retry, duplicate publication, prefix movement, predecessor rewrite, or silent continuation.

A reopened committed head returns byte-identical artifacts without recomputation or republication. Process restart re-verifies frozen predecessor custody. Immutable objects orphaned by a failed CAS may be reused only after exact canonical-byte, selector, digest, length, predecessor, and qualification verification.

## Stage 6 reuse boundary

The bridge may reuse deterministic mechanics from `internal/programcrepresentative`, including:

- exact partition and selection-context validation;
- relation-occurrence closure validation;
- deterministic CALLS-only selection;
- selected and typed non-selection records;
- canonicalization, replay, limits, and balanced accounting.

Historical fixtures, test-only qualification tokens, historical representative results, and synthetic causal worlds provide design and regression evidence only. They cannot qualify or seed the frozen-corpus artifacts.

Qualification publication is explicitly acyclic:

1. publish a canonical `ProgramCRepresentativeQualificationSubject` binding the accepted ADR digest, adapter contract, exact Stage 6 implementation identity, policy, limits, corpus identity, source pin, complete receipt inventory, Program C terminal head, and claim ceilings;
2. obtain and publish an independent-review receipt binding the subject's exact selector, digest, byte length, review scope, affirmative verdict, and reviewer/writer separation;
3. publish a final `ProgramCRepresentativeQualification` envelope binding the subject and review-receipt selectors, digests, and byte lengths;
4. publish denominator, lineage, and successor evidence in that order.

Neither the subject nor review receipt may reference the final envelope or any denominator, lineage, or successor head. Each denominator entry binds the final qualification-envelope selector and byte length without creating a dependency on a not-yet-published denominator head.

Before Stage 6, a repository-owned verifier resolves and verifies the subject, review receipt, and final envelope; checks the receipt's exact scope, affirmative verdict, and reviewer/writer separation; then jointly verifies the committed denominator head and requires corpus, predecessor, adapter, policy, limit, and qualification identities to agree. Each lineage record binds both the final qualification-envelope selector/length and denominator selector/length.

The verifier rejects test helper tokens, test trust anchors, stale validity, foreign corpus or predecessor identities, non-affirmative or out-of-scope review, non-independent review, and caller-supplied boolean qualification. Only after this repository-owned verification may it provide the narrow qualification predicate required by the existing Stage 6 mechanics.

## Gap-ledger integration

After both heads are independently verified, one inventory-successor transition migrates a strictly verified `lsp-trace.inventory-successor-head.private.v1` generation-1 head to `lsp-trace.inventory-successor-head.private.v2`. The v2 head binds the exact prior successor selector plus denominator selector/byte length and lineage selector/byte length.

From that v2 head, the repository derives `lsp-trace.describe-preparation-gap-ledger.private.v2`, in which only:

```text
REPRESENTATIVE_NOMINATION_DENOMINATOR = RETAINED
COMMUNITY_TO_REPRESENTATIVE_LINEAGE   = RETAINED
```

Each retained requirement binds its exact selector and byte length. Representative nomination changes from:

```text
NOT_COMPUTED, count=null
```

to:

```text
COMPUTED, count=D
```

including `D=0`, only after the lineage artifact balances. Candidate preparation remains:

```text
NOT_COMPUTED, count=null
```

The workflow remains `WAITING_EVIDENCE` until retained source closure, `FULL_DEFINITION` display bindings, and caller-occurrence V3 custody are independently qualified.

## Corrections and history

Published artifacts and heads are immutable. A correction creates a new generation referencing the prior head and records a finite delta. It cannot rewrite Program C receipts, predecessor artifacts, denominator entries, lineage records, or prior gap ledgers.

## Required qualification

Before private implementation can be accepted:

1. freeze the accepted ADR bytes and digest;
2. prove complete ordinal receipt discovery, deterministic bounded v3/v4 probing, exactly-one candidate admission, receipt verification, and reconstructed-final-head equality without caller selectors;
3. prove zero-match and dual-match receipt candidates fail closed while unrelated shared-store objects do not affect reconstructed-chain completeness;
4. prove exact reconstruction from retained Native V5 bytes without checkout or LSP fallback;
5. prove the field-level Native V5 adapter, exact relation-occurrence closure, and singleton context invariant;
6. prove every complete-run community artifact hash matches retained instability evidence;
7. prove all six run identities are conserved and multiple seeds/replays remain distinct denominator entries;
8. prove `EMPTY` contributes zero entries but remains classified;
9. prove resource-incomplete contributes zero entries and remains explicitly excluded;
10. prove omission, duplication, permutation, substitution, stale identity, malformed bytes, and mixed versions fail closed;
11. prove denominator and lineage equations, overflow handling, and limit boundaries;
12. prove every denominator identity has exactly one singleton Stage 6 result and terminal lineage record;
13. prove selected-node, witness, community, context, occurrence, receipt, qualification, and head substitution rejection;
14. prove all three one-CAS operations, close/reopen replay, conflict, cancellation, and uncertain-CAS behavior;
15. prove deterministic v1→v2 successor migration and that only the two authorized gaps become retained;
16. prove all authority ceilings and the remaining three gaps stay unchanged;
17. publish and verify the acyclic qualification sequence—subject, independent-review receipt, then final envelope—before denominator derivation;
18. prove the subject and receipt do not reference the envelope or downstream evidence heads, and prove reviewer/writer separation;
19. run a bounded frozen-corpus qualification from freshly verified custody.

## Alternatives rejected

### Count communities without lineage

Rejected. A balanced count without attributable records permits selective omission or substitution.

### Retain lineage without a frozen denominator

Rejected. Individually plausible records cannot establish complete coverage.

### Collapse identical communities across seeds or replays

Rejected. Historical run identities are distinct evidence contexts and Stage 6 requires one context per community.

### Admit resource-incomplete communities

Rejected. ADR 0013 permanently excludes those constituents from nomination and Describe preparation.

### Reuse historical Stage 6 fixtures or representatives

Rejected. Code reuse does not transfer custody, corpus qualification, or authority.

### Read checkout source or reacquire LSP evidence

Rejected. The bridge must derive only from verified retained predecessor custody.

### Mark all five evidence gaps retained together

Rejected. Each gap has independent provenance and qualification requirements.

## Acceptance record

The proposal at SHA-256 `85daf073ecb8836d3b5762bd44d6a6ba911d14e23a81847dd9741a274084ebda` received independent verdict `ACCEPT` after three review rounds. On 2026-10-25, @jaresty explicitly selected **Accept ADR 0014**, authorizing private implementation and qualification only under this contract.

This acceptance does not authorize public CLI/MCP exposure, semantic acceptance, candidate preparation before all required evidence is retained, production admission, release, or deployment.

### Accepted receipt-discovery amendment

After acceptance, implementation planning exposed that historical v2 result bytes do not identify whether their immutable transition receipt used schema v3 or v4. The decision authority selected bounded repository-owned v3/v4 probing as the correction. The exact amendment bytes at SHA-256 `df8d12f62d3eff9cd7553cbd65db0e6b9b6561d326c5d220f4ae8b387a374d57` received independent verdict `ACCEPT` and were explicitly accepted by @jaresty on 2026-10-25 for private implementation and qualification only.
