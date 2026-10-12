# ADR 0011 amendment: qualified CALLS profiles

- **Status:** Proposed additive amendment; default off
- **Date:** 2026-10-10
- **Amends:** [ADR 0011](0011-versioned-definition-reference-grouping-evidence.md)
- **Authorization:** qualification planning only; no production admission, implementation, public surface, migration, or historical rewrite

## Context

ADR 0011 preserves the historical omitted-selector `CALLS_ONLY` path. Its schemas, selectors, occurrence custody, snapshots, packets, checkpoints, replay behavior, CLI/MCP behavior, canonical bytes, defaults, and claim ceiling are historical contracts. This amendment does not retrofit qualification onto that path and does not migrate it.

A separately selected CALLS input may nevertheless need provider- and method-specific qualification before it participates in an additive Program C composition. Framework qualification and concrete acquisition qualification are different facts and must have different identities.

## Decision

Define the framework profile family `QUALIFIED_CALLS_V1`. It specifies the qualification roles and exact predecessor chain required for a new, explicitly selected CALLS profile. It is not itself a provider, method receipt, grouping policy, qualification receipt, production admission, or historical compatibility adapter.

Define the synthetic profile identity `CALLS_SYNTHCALLS_EXACT_V1`, with provider slug `synthcalls`, as permanently and irreversibly `TEST_ONLY`. Its identity is intrinsically non-production: production admission must reject it unconditionally even if every evidence role is complete and even if receipts, reviews, signatures, flags, configuration, build tags, or a claimed GO say otherwise. This prohibition cannot expire, be reviewed away, or be superseded for the same identity.

`CALLS_SYNTHCALLS_EXACT_V1` occupies a permanent synthetic namespace. It and every artifact in its identity domain are forbidden as production facts. No actor may bridge it into production by aliasing; renaming; conversion; wrapping or embedding; inheritance; predecessor, lineage, or successor reuse; schema-ID or canonical-domain reuse; receipt reuse; evidence copying or resealing; profile-ID substitution; or a production GO that names or transitively relies on the synthetic identity. A bridge attempt terminates with typed `SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN` before all production effects.

A production profile must be independently created under a distinct concrete profile ID, schema identity, and canonical identity domain. It requires an independently selected provider/method authority, manifest, predecessor closure, evidence set, qualified receipts, review, and GO that bind that exact production profile. It inherits zero facts and zero authority: no receipt, evidence byte, review, qualification, predecessor status, lineage, or GO transfers from the framework family, synthetic profile, alias, predecessor, successor, or equal-byte object. A GO is exact-profile-ID-and-canonical-domain scoped only; it implies nothing about a family, alias, predecessor, successor, lineage, or profile with equal bytes.

Selection of `CALLS_SYNTHCALLS_EXACT_V1` must be explicit and may reach test-only verification only. Omission continues to select the historical `CALLS_ONLY` behavior without passing through `QUALIFIED_CALLS_V1`.

## Non-retrofit compatibility boundary

The following are immutable compatibility requirements:

1. Historical omitted-selector `CALLS_ONLY` behavior remains unchanged.
2. Historical canonical bytes and identities remain unchanged.
3. Historical artifacts are neither reissued nor wrapped under a new schema as a condition of continued use.
4. No historical record gains a qualification receipt, profile selector, provider claim, or new field.
5. The additive path uses distinct family, profile, schema, identity, storage, checkpoint, and replay domains.
6. A compatibility projection, if separately accepted, remains a projection from accepted historical evidence and never replaces or upgrades its source.
7. Failure, absence, staleness, or refusal in the additive path cannot alter the historical path or cause an implicit fallback labeled as qualified.

## Claim ceiling

Every artifact governed by this amendment retains:

```text
authority = 0
accepted = false
completeness = UNKNOWN
```

A qualified CALLS profile establishes only that exact bounded server-reported CALLS evidence satisfied the named method/provider, occurrence, composition, replay, resource, privacy, and independent-review contracts. It does not establish runtime execution, dynamic dispatch, producer authentication, source completeness, feature identity, ownership, purpose, or production use.

## Qualification chains

The synthetic profile may exercise this exact test-only chain, but completion never changes its permanent production refusal:

```text
QUALIFIED_CALLS_V1 framework contract
→ CALLS_SYNTHCALLS_EXACT_V1 TEST_ONLY profile
→ test-only method/provider custody
→ test-only admitted CALLS occurrences and multiplicity
→ test-only Stage 3 through Stage 7C receipts
→ test-only accounting/resource/privacy and compatibility results
→ permanent SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN
```

Any future production admission requires a separate exact, content-addressed chain beginning with a distinct production profile ID and canonical identity domain:

```text
QUALIFIED_CALLS_V1 framework contract
→ distinct exact production profile identity
→ independently produced method/provider custody receipt
→ independently produced admitted CALLS occurrence ledger with exact multiplicity
→ independently produced relation-family admission policy
→ independently produced Stage 3 grouping-input admission and composition receipt
→ independently qualified Stage 4 grouping-policy qualification receipt
→ independently produced Stage 5 composition/readiness receipt
→ independently produced Stage 6 representative receipt
→ independently produced Stage 7A source-context receipt
→ independently produced Stage 7B checkpoint/replay receipt
→ independently produced Stage 7C coupling/instability receipt
→ independently produced accounting/resource/privacy receipt set
→ independently produced historical compatibility receipt
→ exact-production-profile-scoped independent review artifact
→ exact-production-profile-scoped ADR0011_QUALIFIED_CALLS_PROFILE_GO
```

Every arrow binds exact schema identity, object identity, byte length, SHA-256 digest, predecessor role, policy identity, implementation identity where applicable, and freshness/validity constraints. Substitution, unknown version, absent payload, digest or length mismatch, stale validity, duplicate role, role-order ambiguity, unbalanced accounting, synthetic lineage, or cross-profile reuse fails closed. GO scope is the exact production profile identity and canonical domain named by the review; it conveys no family-wide, alias, predecessor, successor, or equal-byte authority.

## Required role contracts

### Method and provider custody

The concrete profile freezes the exact CALLS-producing method, provider slug and version, adapter, capability observation, managed session/generation, workspace revision and custody, position encoding, request identity, bounded response identity, implementation identity, and terminal outcome. Text matches never establish CALLS. Same-process custody does not authenticate the producer.

### Admitted occurrences and multiplicity

Each admitted call occurrence preserves caller-to-callee direction, exact server-reported call-site identity and range, relation-local ordinal, repeated equal-endpoint occurrences, provider/method receipt, document/revision custody, and terminal accounting. Pair aggregation cannot rewrite the occurrence ledger. Missing or malformed occurrence evidence admits zero occurrences.

### Relation-family policy

The additive profile admits only the explicitly named CALLS family/version. It cannot relabel D/R evidence, infer calls from source text, silently add another relation family, or broaden historical CALLS schemas. Unknown or undeclared families fail closed.

### Stage 3 grouping input

Stage 3 consumes only the exact admitted occurrence ledger and named relation-family policy. It binds occurrence denominator, exclusions, duplicates, canonical order, resource/privacy policies, and every derived contribution. It emits a separate immutable grouping-input/composition identity; it does not mutate historical Program C input.

### Independent Stage 4 grouping qualification

Stage 4 qualification is independent from method/provider qualification and Stage 3 admission. It binds a preregistered plan, calibration and held-out corpora, policy bytes, thresholds, seeds, algorithm/profile, expected groupings, complete metrics, adversarial cases, deterministic permutation/replay results, and independent reviewer acceptance. A provider receipt cannot qualify a grouping policy, and a grouping policy cannot qualify a provider.

### Stage 5 composition and readiness

Stage 5 proves the exact Stage 3 input and Stage 4 qualified policy can compose under a named implementation and readiness contract. It binds algorithm, numeric policy, seed inventory, limits, canonical partition, predecessor chain, and terminal accounting. Equal partitions from different predecessors have different identities.

### Stage 6 representative

Representative selection uses only eligible admitted CALLS evidence under a separately pinned representative policy. It records eligible and ineligible members, ties, omissions, chosen representative or typed unavailability, and cannot derive a representative from D/R prominence, source order, lexical prominence, or model judgment.

### Stage 7A source context

Source context binds each selected occurrence and representative to exact evidence, item, selection, and full-definition display ranges; exact source bytes/digest/length; document version, revision, session/generation, encoding; display-resolution receipt; privacy policy; and terminal selected/unavailable/ambiguous/withheld/limit-omitted accounting. Ambient checkout reads and hidden reacquisition are forbidden.

### Stage 7B checkpoint and replay

Checkpoint/replay binds every required predecessor and reconstructs canonical outputs using retained named artifacts only. Replay cannot query an LSP server, inspect a workspace, substitute current documents, infer a missing policy, or use a newer implementation/profile. Byte inequality or unavailable required payload yields one typed failure and no partial qualified result.

### Stage 7C coupling and instability

Stage 7C separately binds relation-scoped `Ca`, `Ce`, `I = Ce/(Ca+Ce)` (zero denominator gives `I=0`), node universe, self-loop handling, parallel-neighbor deduplication, omissions, completeness accounting, seed/run inventory, label-independent community matching, unmatched communities, failed runs, thresholds, and publication-blocking outcome. Historical CALLS coupling and instability bytes remain unchanged.

### Accounting, resources, and privacy

Each stage has closed operation/member outcomes and balances declared, begun, evaluated, terminal, successful, failed, omitted, missing, admitted, excluded, and emitted denominators as applicable. Independent hard limits cover requests, messages, documents, candidates, occurrences, ranges, source bytes, canonical bytes, output bytes, time, memory/allocation where enforceable, and logical work. Privacy withholding is explicit and cannot leak withheld bytes through errors, diagnostics, checkpoints, replay, or hashes that enable disclosure.

## Resource ceiling

`QUALIFIED_CALLS_V1` requires a pinned finite ceiling contract before execution. At-limit behavior must be qualified; limit-plus-one must fail or terminate with a typed accounted disposition before partial production-visible effect. Neither this amendment nor `CALLS_SYNTHCALLS_EXACT_V1` chooses numeric ceilings. The synthetic identity remains production-forbidden even with exact ceilings; a distinct production profile is `INCOMPLETE` and default-off absent its own exact numeric ceilings and independent enforcement evidence.

## Canonical identity

Each additive object uses a domain-separated identity over its canonical schema/version, profile, full canonical bytes, predecessor role list, predecessor identities/digests/lengths, policy identities, implementation/environment identities, limits, privacy partition, accounting, and outcome. Canonical parsers reject duplicate/unknown keys, trailing values, invalid UTF-8, noncanonical encodings, invalid ordinals/ranges, overflow, and undeclared fields. Resealing semantically changed content never preserves identity or admission.

## Production admission

Production admission is default-off and fail-closed. `CALLS_SYNTHCALLS_EXACT_V1` is unconditionally rejected before all production effects, regardless of complete evidence, review, receipts, signatures, flags, configuration, build tags, privileged callers, or claimed GO. A GO naming the synthetic identity is invalid and itself triggers `SYNTHETIC_PROFILE_PRODUCTION_FORBIDDEN`.

The producer cannot self-mint, self-review, infer, copy, reseal, convert, inherit, wrap, or alias required evidence. Test fixtures and test-only receipts are never production-admissible. A runtime flag, profile name, successful test, matching digest without available bytes, report prose, author assertion, synthetic lineage, or this amendment cannot supply a production role.

Only an independent reviewer of a **distinct exact production profile** may issue `ADR0011_QUALIFIED_CALLS_PROFILE_GO`, scoped jointly to that profile ID and canonical identity domain. The review must bind that profile's independently produced evidence manifest and exact artifact bytes. It cannot name, reuse, depend on, or derive authority from `CALLS_SYNTHCALLS_EXACT_V1`. Any missing, planned, stale, substituted, unverified, synthetic-derived, cross-profile, or self-issued role prevents GO. No evidence, review, receipt, signature, configuration, build tag, or future action can promote the synthetic identity; the author of this amendment makes no GO claim.

## Consequences

The design creates a separate qualification path for explicitly selected CALLS profiles while preserving the historical default exactly. It increases evidence and review cost, but prevents provider qualification, grouping qualification, composition readiness, source context, replay, representative selection, and derived metrics from being collapsed into one self-attested receipt.
