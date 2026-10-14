# Frozen contract

## Hard boundary

The corrected implementation does not modify ADR0007 source-text-search internals. It calls unchanged `internal/adr0007sourcetextsearchv4private.Evaluate` and stores ADR0007 canonical bytes and digest in the wrapper result.

When `Request.Composition == nil`, `Compose` returns exact ADR0007 forwarding:

- same terminal result as direct ADR0007 `Evaluate`
- same canonical bytes from `adr0007.Canon`
- same canonical SHA-256
- no wrapper composition candidate
- no direction dispositions
- zero wrapper accounting

## Ownership separation

ADR0007 retains ownership of source admission, source bytes/raw provider bytes, literal matching, `MatchID` creation, baseline ordering, positions/Location pins, and ADR0007 `RangeUnionCandidate` construction/digest.

The wrapper may only construct its own separate `CompositionCandidate` from existing ADR0007 `MatchID`s and committed ADR0011 bidirectional envelope metadata. It does not mutate ADR0007 candidates.

## Ordinal identity contract

The immutable artifacts do not expose a selected item/ordinal set. They expose directional result/package identity and `one_final_disposition_ordinals=5900` for both incoming and outgoing. Therefore:

- callers cannot choose selected ordinals;
- success envelopes must carry empty `SelectedOrdinals`;
- marks must carry `SelectedOrdinal=0` as the zero sentinel, never an accepted ordinal;
- marks bind to canonical direction ordinal identity via `OneFinalDispositionOrdinals=5900` and the direction-specific ordinal identity hash;
- envelope and mark ordinal identity drift fail pre-effect.

Hash preimages:

- incoming: `direction=incoming|ordinal_semantics=ONE_FINAL_DISPOSITION_PER_TARGET_ORDINAL|one_final_disposition_ordinals=5900|selected_ordinals=NOT_EXPOSED|result_sha256=e9c8cce4b77dc91a8ed435166fbf88ce9d6e12bbc28bd4ad0db4fbbb45cc4154|artifact_sha256=07212f4175aeb115dda5828761b3338b56233704673e3a3bbf861373f174e3eb`
- outgoing: `direction=outgoing|ordinal_semantics=ONE_FINAL_DISPOSITION_PER_TARGET_ORDINAL|one_final_disposition_ordinals=5900|selected_ordinals=NOT_EXPOSED|result_sha256=e6a60c88f4bd00d2e220d160765c8ee903903523cb86cfb7d814e31a7c433c9f|artifact_sha256=07212f4175aeb115dda5828761b3338b56233704673e3a3bbf861373f174e3eb`

## Candidate semantics

The wrapper-owned candidate remains candidate-only:

- `Operation = ADR0011_PRIVATE_DEPENDENCY_MARKED_RANGE_UNION_PROJECTION`
- `CandidateOnly = true`
- `ExecutedLocation = false`
- `Authority = 0`
- `Accepted = false`
- `Completeness = UNKNOWN`
- `FeatureIdentity = UNRESOLVED`

It does not call `adr0007locationv5.Evaluate` and does not infer accepted feature identity or completeness.

## Accepted pins

The wrapper hard-pins repository HEAD, accepted bidirectional package/artifact, incoming/outgoing profile IDs, generic profile IDs, evidence/result hashes, provider package checksums, provider manifests, and ordinal identity hashes. Any drift fails before candidate effect.
