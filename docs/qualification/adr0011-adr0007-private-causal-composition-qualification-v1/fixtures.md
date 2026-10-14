# Fixtures

The wrapper tests use in-memory ADR0007 attempts and wrapper-only ADR0011 envelope fixtures.

## ADR0007 fixture

- Attempt ID: `attempt-adr0011-wrapper-composition`
- Query: `needle`
- Source URI: `file:///tmp/adr0011-wrapper.go`
- Source text: three literal `needle` matches in one admitted source

The ADR0007 baseline is produced by direct `adr0007.Evaluate` and canonicalized with `adr0007.Canon`; the wrapper stores those canonical bytes and SHA-256 without changing ADR0007 output.

## ADR0011 fixture model

The tests use wrapper constructors for pinned envelopes:

- `AcceptedSuccessEnvelope("incoming")`
- `AcceptedSuccessEnvelope("outgoing")`
- `AcceptedFailureEnvelope("incoming")` or `AcceptedFailureEnvelope("outgoing")`

Success envelopes carry no selected item ordinals because committed artifacts do not expose an accepted selected ordinal set. They carry the frozen direction ordinal identity (`one_final_disposition_ordinals=5900` plus direction result/artifact identity hash).

Dependency marks reference only ADR0007-created `MatchID`s. Marks carry direction, action (`OMIT` or `ORDER`), relation kind `CALLS`, matching pinned profile ID, matching evidence hash, and the matching frozen direction ordinal identity. `SelectedOrdinal` must remain zero.

## Negative fixtures

The tamper table mutates one field at a time for unknown directions, duplicate directions, package/artifact/profile/evidence drift, unknown match IDs, duplicate mark IDs, relation-kind escalation, cross-direction/profile/evidence drift, self-consistent selected ordinal drift, self-consistent ordinal identity drift, mark-only ordinal identity drift, resource bounds, MaxReferences, and cancellation.
