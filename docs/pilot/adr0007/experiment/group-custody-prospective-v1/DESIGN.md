# ADR 0007 bounded Group custody design v1

Status: `DESIGN_PREPARATION_ONLY`

This private design consumes the immutable bounded Search v11 qualification and prepares complete Group accounting without executing or qualifying Group.

## Immutable basis

- integrated main tree: `e1407f77353b0ccfff13e693570f0e3fc472e579`
- Describe integration commit: `404404127053229c8bd154d7e0fa15cf66485186`
- Search integration commit: `4bc75e97d845401bd0090535c1f9cb3d1bf97229`
- Search freeze: `sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301`
- Search custody verdict: `SEARCH_CUSTODY_GO`

The additive main history orders Search before Describe, but the integrated tree contains both byte-compatible milestones.

## Contract decisions

1. Group consumes all 24 Search ledger members, never only the returned top five.
2. Group accepts only a complete, balanced, independently reviewed Search account bound to the exact freeze above.
3. An already retained fixed-seed/fixed-resolution Leiden observation may nominate provisional candidates. Group v1 does not run Leiden.
4. A partition or community is not a feature. Community IDs are evidence metadata, not Group identities.
5. Representatives are navigation-only and cannot establish membership, description, completeness, or identity.
6. V1 prohibits overlapping candidate membership. Every input ordinal receives exactly one terminal member outcome.
7. Assigned members require complete digest-bound source evidence and retained server-reported `CALLS`/community evidence. Text occurrences cannot establish `CALLS`.
8. Cancellation, timeout, resource exhaustion, backend failure, or policy mismatch commits no Group result; the terminal attempt retains bounded counters and cause.
9. One producer attempt and one independent reviewer attempt are allowed per predeclared assignment. Delivery replay is idempotent; conflicting bytes are retained separately and never overwrite committed state.
10. Outputs remain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.

## Closed ADR outcomes

Operation outcomes are exactly:

- `COMPLETE`
- `INDEX_UNAVAILABLE`
- `INDEX_MISMATCH`
- `CANCELLED`
- `TIMEOUT`
- `RESOURCE_LIMIT`
- `BACKEND_FAILURE`
- `POLICY_MISMATCH`

Member outcomes are exactly:

- `GROUPED`
- `UNMATCHED`
- `FILTERED_BY_POLICY`
- `DUPLICATE_MEMBER`
- `INVALID_MEMBER`

Detailed failures use closed cause codes; attempt states are not operation outcomes.

## Explicit exclusions

No Group execution, model invocation, production dispatch, CLI/MCP/public registration, hostile-filesystem or multiprocess hardening, feature acceptance, stakeholder acceptance, authority elevation, ADR 0011 dependency, or candidate-group commit `448a1f4f` dependency is authorized.

A later immutable freeze and independent `GROUP_DESIGN_GO` are required before separate execution authorization may be requested.
