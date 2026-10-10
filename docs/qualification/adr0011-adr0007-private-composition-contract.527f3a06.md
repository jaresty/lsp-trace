# ADR 0011 → ADR 0007 private composition contract

- **Schema:** `lsp-trace.adr0011-adr0007-private-composition-contract.v1`
- **Status:** frozen candidate for independent qualification
- **Authority:** `0`
- **Accepted:** `false`
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`
- **Authorization receipt:** `20261010153748-5419`
- **Predecessor audit:** `docs/qualification/adr0011-stage7-consolidated-audit.943b2208.json`, SHA-256 `943b22081153d15faaa749a42395430a4718b11c9c4e0b2111d4fdc363de7528`
- **Design context only:** commit `4f5743fffec09e3f5f97c0a55e5369c4ed1e0128`, provisional spelling `lsp-trace census --catalog`

This contract authorizes no public surface, execution, migration, publication, release, or deployment. It freezes the private boundary that a later separately authorized implementation must satisfy. Existing `census --catalog`, CLI/MCP schemas, operation registries, defaults, selectors, readers, and bytes remain unchanged.

## 1. Composition

```text
qualified Stage 7 artifacts
→ source-bearing composition packet
→ ADR 0007 Describe
→ Search
→ provisional Group
→ Location
→ Source Text Search
→ immutable candidate generation
```

`Describe`, `Search`, `Group`, `Location`, and `Source Text Search` are versioned private stage names in this contract. They do not assert an existing public operation or freeze a future public spelling. A stage consumes only admitted canonical predecessor bytes; it may not inspect an ambient checkout, language server, network, mutable index, clock, process environment, or hidden host context.

## 2. Canonical encoding and exact carrier

Every request, packet, checkpoint, stage result, failure, and generation is strict duplicate-free JSON with lexicographically stable arrays where order is not semantic, preserved ordinal arrays where order is semantic, UTF-8, no insignificant whitespace, and exactly one trailing LF. Identity is:

```text
sha256(domain || 0x00 || schema || 0x00 || canonical_payload_without_identity)
```

Every `ExactCarrierV1` contains:

```text
role, schema, identity, sha256, byte_length, canonical_bytes
```

Admission recomputes canonical form, schema, identity, digest, and length and performs semantic parsing. A caller-supplied identity is never authority. Same identity with different bytes is `IDENTITY_CONFLICT`. Unknown or duplicate fields, alternate JSON spelling, trailing values, CRLF, missing LF, overflow, invalid enum, unbalanced accounting, or a validly resealed semantic mutation is rejected atomically.

All integer arithmetic is checked. Ratios use reduced nonnegative integer rationals with typed zero denominators. Floating point is forbidden in identity-bearing computation.

## 3. Required request

`PrivateCompositionRequestV1` binds these exact carriers:

1. Stage 4 grouping policy and immutable grouping input;
2. Stage 4 qualification receipt, evidence records, trust anchors, and validity epoch;
3. Stage 5 policy-composition result and readiness receipt;
4. Stage 5 grouping result, every successor result, seed inventory, algorithm and parameters;
5. Stage 6 representative result and disclosures;
6. Stage 7A source-context result;
7. Stage 7B snapshot, each community-target packet, checkpoint, replay request, and replay result;
8. Stage 7C relation-scoped coupling artifact;
9. Stage 7C cross-seed instability artifact;
10. source/workspace binding;
11. workflow and stage policies;
12. privacy and resource policies.

The source/workspace binding contains exact workspace identity, repository identity when retained, source revision, session identity, generation, position encoding, admitted document identities, document versions/revisions, source digests and lengths. An unavailable field is an explicit typed value, never an empty wildcard.

The policy inventory contains exact carriers for composition, Describe, Search, Group, Location, Source Text Search, candidate generation, privacy, resources, checkpointing, arbitration, and replay. It also binds relation families, grouping seeds, instability threshold/boundary, representative policy, source-context limits, every stage limit, and qualification epoch.

The request identity commits to the ordered role inventory and each carrier's schema, identity, digest, length, and exact bytes. The generation identity commits to the request identity plus this contract identity. Any identity-bearing change creates a different generation.

## 4. Admission and substitution rejection

Admission is atomic and ordered:

1. enforce per-carrier and aggregate resource limits before allocation;
2. strictly parse every carrier;
3. recompute carrier identity, digest, length, and semantic derivations;
4. verify source/workspace custody and source-context occurrence bindings;
5. verify the Stage 3→4→5→6→7A→7B→7C predecessor chain;
6. verify grouping-policy qualification, composition, readiness, epoch, and trust custody;
7. verify replay packet/checkpoint/request/result links and byte-identical reconstruction;
8. verify representative eligibility and disclosures;
9. verify coupling scope/family evidence and historical CALLS reproduction;
10. verify instability uses supplied successor result bytes and exact seed/pair accounting;
11. verify privacy/resource policies and this contract identity;
12. compute request and generation identities.

Every role is unique and mandatory unless its outcome is explicitly allowed below. Swapping two valid carriers, replacing one carrier with another generation, changing only schema/identity/digest/length, changing nested bytes and resealing, stale qualification, stale source revision, or conflicting checkpoint causes one typed failure with zero downstream evaluations, checkpoints, generations, and publication effects.

## 5. Source-bearing packet

`SourceBearingCompositionPacketV1` is the sole input to Describe. Its identity binds:

- request and generation identities;
- community and target identities;
- representative disposition and disclosures;
- exact Stage 7B packet and replay-result carriers;
- each occurrence identity and relation family;
- independently selected query/reference and target roles;
- evidence, item, selection, display, and context ranges;
- exact selected source bytes with document/session/generation/version/revision/digest/length custody;
- source-context terminal outcome per role;
- coupling and instability identities and relevant disclosures;
- privacy redactions/withholdings;
- stage limits and accounting;
- unchanged claim ceiling.

No source may be reacquired. `SELECTED` requires exact retained bytes. `UNAVAILABLE`, `AMBIGUOUS`, `WITHHELD`, and `LIMIT_OMITTED` carry no invented body and retain their denominator position.

## 6. Representative behavior

A CALLS-bearing community may carry the exact Stage 6 representative selected under historical prepared-root/reached-node/SCC/distance/predecessor semantics. REFERENCES_SYMBOL and RESOLVES_TO_DEFINITION evidence is visible but representative-neutral.

A community with no eligible CALLS representative terminates representative selection as `NO_CALLS_REPRESENTATIVE`. It remains disclosed and accounted. No D/R node is promoted to representative. Candidate generation may retain a typed omission record but may not manufacture a Describe target or context-complete claim.

## 7. Deterministic stage contract

Each stage request identity binds the generation, stage name/version, exact predecessor result identities, exact stage policy, privacy/resource policies, and ordinal work inventory. Each stage result binds the request plus one terminal outcome for every denominator member.

1. **Describe V1** consumes source-bearing packets and yields one of `COMPLETE`, `ABSTAINED`, `NO_CALLS_REPRESENTATIVE`, `SOURCE_CONTEXT_INCOMPLETE`, `WITHHELD`, `LIMIT_OMITTED`, or typed failure. Generated text bytes, backend/profile identity, prompt/instruction identity, and all cited source identities are retained. No uncited source claim is packet-derived.
2. **Search V1** consumes the immutable Describe corpus and exact query/policy bytes. Every corpus member is `MATCHED`, `NOT_MATCHED`, `WITHHELD`, `LIMIT_OMITTED`, or failed. Score representation, if any, is an exact policy-defined integer/rational, never floating point.
3. **Provisional Group V1** consumes only admitted Search results. Groups are deterministic membership sets ordered by canonical member identity. Unmatched members remain explicit. Group labels are presentation only and never identity or feature authority.
4. **Location V1** consumes group/member and exact retained source-location custody. It selects no new source and produces exact location carriers or explicit unavailable/ambiguous/withheld/limit outcomes.
5. **Source Text Search V1** searches only exact admitted document bytes under an exact pattern/query policy. It retains pattern bytes, match ordinals, exact ranges, document identity/digest/version, limits, and every omission. It cannot repair Location or Stage 7 source context.
6. **Candidate Generation V1** consumes the five exact stage results. Candidate identity binds generation, deterministic constituent membership, every predecessor identity, source/location/text-search evidence, disclosures, omission/failure accounting, and claim ceiling. Candidate order is canonical identity order. A candidate is a review input, not accepted feature identity.

## 8. Outcomes and publication gate

A generation has one state:

```text
ADMITTED
STAGE7_VERIFIED
DESCRIBE_COMPLETE
SEARCH_COMPLETE
GROUP_COMPLETE
LOCATION_COMPLETE
SOURCE_TEXT_SEARCH_COMPLETE
CANDIDATE_ASSEMBLED
CANDIDATE_COMMITTED
REFUSED
```

Typed refusal codes include:

```text
PREDECESSOR_INVALID
PREDECESSOR_SUBSTITUTED
IDENTITY_CONFLICT
SEMANTIC_FORGERY
QUALIFICATION_MISSING
QUALIFICATION_STALE
READINESS_UNAVAILABLE
COMBINED_POLICY_UNAVAILABLE
MANDATORY_EVIDENCE_INCOMPLETE
SOURCE_CONTEXT_INCOMPLETE
GROUPING_UNSTABLE
ACCOUNTING_UNBALANCED
RESOURCE_BOUND
PRIVACY_WITHHELD
SEMANTIC_REPLAY_MISMATCH
CHECKPOINT_CONFLICT
CONCURRENT_CONFLICT
CANCELLED
TIMEOUT
BACKEND_FAILURE
PUBLICATION_BLOCKED
```

Missing, incomplete, failed, stale, withheld, unstable, or unqualified mandatory evidence prevents `CANDIDATE_COMMITTED`. D/R-only, mixed, and empty evidence remains visible and replayable but publication-blocked unless every required independent family/composition qualification exists; even then representative rules still apply. `COMBINED` never silently substitutes per-family metrics. Zero-result is a successful terminal generation only when every denominator is terminal, accounting balances, and the canonical candidate list is empty; it is distinct from incomplete or failed.

No result is externally published by this private contract. Here, `CANDIDATE_COMMITTED` means committed to the private immutable generation store only.

## 9. Accounting

Each stage records exact `expected`, `evaluated`, `complete`, every named non-success terminal count, `failed`, `missing`, `omitted`, `terminal`, and checked work/byte/request/output counts. Invariants:

```text
expected = terminal + missing
evaluated = sum(all evaluated terminal outcomes)
complete_generation ⇒ missing = 0
each expected member has exactly one terminal outcome or is explicitly missing
candidate_expected = candidate_included + candidate_omitted + candidate_failed
```

Duplicate semantic content does not disappear from denominators; it receives a terminal `DUPLICATE_OF` outcome referencing the canonical identity. Stage accounting never rewrites predecessor accounting. Cumulative accounting retains admitted/replayed objects and bytes; documents, unique source bytes, ranges, occurrences, communities, targets; Describe nominations/requests/responses/abstentions/preparation failures; Search corpus members/outcomes; grouped/unmatched members; Location and text-search outcomes; candidates; checkpoints; ownership attempts; and replay effects.

Any overflow or imbalance refuses before checkpoint or candidate commit.

## 10. Checkpoints, resume, and replay

A `CompositionCheckpointV1` is committed only after a complete stage. It binds generation/request identity, completed stage and result, exact predecessor chain, cumulative accounting, next stage, limits, ownership epoch, and previous checkpoint identity. Checkpoints form a hash chain.

Resume validates the entire supplied chain and resumes exactly at `next_stage`. Completed stages are never rerun. A missing or failed stage has no completion checkpoint. A stale, skipped, reordered, foreign, semantically forged, or accounting-conflicting checkpoint returns `CHECKPOINT_CONFLICT` with zero effects.

A `CompositionReplayRequestV1` binds generation identity, terminal generation/checkpoint identity, every retained carrier, and exact replay limits. Replay is workspace-free and side-effect-free. It reconstructs every packet, stage request/result, checkpoint, accounting object, and final generation from retained bytes. Success requires byte-identical canonical output. Otherwise exactly one typed atomic failure is returned.

## 11. Idempotency and concurrency

A complete generation is reusable only after all exact bytes and semantic derivations verify. An equivalent request returns the same canonical generation bytes and records no new stage, checkpoint, candidate, or publication effect.

Concurrent equivalent requests arbitrate with create-only/CAS ownership over generation identity:

- one owner may evaluate;
- waiters may observe only committed checkpoints;
- a loser verifies and returns the winner's complete generation or resumes from the exact next committed checkpoint after ownership transfer;
- arrival order cannot affect stage/member/candidate order or identity;
- same identity with any differing byte or policy returns `CONCURRENT_CONFLICT`;
- cancellation or owner loss commits no partial stage and transfers only from the last valid checkpoint.

Exactly one complete canonical generation may exist for one generation identity.

## 12. Qualification/readiness rules

CALLS-only composition requires valid grouping qualification/readiness and complete stable Stage 7 evidence. D/R-only and mixed inputs additionally require independently verified family and composition qualification; production cannot self-issue it. Unavailable combined policy returns `COMBINED_POLICY_UNAVAILABLE`. Missing or stale receipt returns `QUALIFICATION_MISSING` or `QUALIFICATION_STALE`. Qualification changes eligibility only; it never raises authority, acceptance, completeness, or feature identity.

## 13. Claim ceiling

Every request, stage artifact, checkpoint, failure, replay, and candidate generation carries exactly:

```text
authority=0
accepted=false
completeness=UNKNOWN
featureIdentity=UNRESOLVED
```

The composition cannot establish semantic truth, runtime use, ownership, purpose, product value, complete repository coverage, accepted feature identity, or public readiness.

## 14. Compatibility and exclusion boundary

Historical Stage 3–7 artifacts, ADR 0007 Describe/catalog bytes and readers, target packets, checkpoints, selectors, defaults, and exit behavior remain unchanged. New parsers are additive and strict. No migration is implied.

The future private implementation write-set is exactly:

```text
internal/adr0011adr0007composition/contract.go
internal/adr0011adr0007composition/identity.go
internal/adr0011adr0007composition/admission.go
internal/adr0011adr0007composition/packet.go
internal/adr0011adr0007composition/stages.go
internal/adr0011adr0007composition/state.go
internal/adr0011adr0007composition/accounting.go
internal/adr0011adr0007composition/checkpoint.go
internal/adr0011adr0007composition/replay.go
internal/adr0011adr0007composition/arbitration.go
internal/adr0011adr0007composition/compose.go
internal/adr0011adr0007composition/contract_causal_red_test.go
internal/adr0011adr0007composition/qualification_test.go
internal/adr0011adr0007composition/testdata/**
```

No other production or test path is authorized by CONTRACT_GO. Any required change outside this write-set stops implementation and requires contract amendment and independent requalification.

Explicitly excluded:

```text
cmd/**
internal/mcp/**
public schema registries
existing ADR 0007 or ADR 0011 artifact schemas
existing census/catalog implementation
help, completion, skill, advertisement, and capability surfaces
migration, publication outside the private store, push, release, deployment
```

## 15. Implementation gate

Private implementation may begin only after this contract and its causal specification are immutable, independently reviewed, and receive `ADR0011_ADR0007_COMPOSITION_CONTRACT_GO`. That result does not authorize public execution or exposure.
