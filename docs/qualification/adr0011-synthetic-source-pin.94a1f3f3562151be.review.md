# ADR0011 synthetic source pin successor review — 94a1f3f3

## Disposition

**ACCEPT_FOR_SELECTOR_UPDATE.**

This acceptance is limited to selecting the reviewed source-byte manifest for the default-off private synthetic test path. It does not establish executable or binary identity, dependency closure, process provenance, producer authentication, occurrence qualification, production custody, public issuance, authority, semantic acceptance, or completeness outside the exact source census.

## Accepted candidate

- Manifest: `docs/qualification/adr0011-synthetic-source-pin.94a1f3f3562151be.manifest.json`
- Manifest SHA-256: `94a1f3f3562151be3def758fd441ab45afc1a34981acb2313733bb887ec8823f`
- Manifest byte length: 15,361
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Entries: 96
- Current source bytes: 650,859
- Aggregate: `sha256:804043a8be4312c23330c1472b13aa3afc60ad40515bf43545222d2037b56e2e`

## Independent verification

A separate read-only reviewer independently enumerated every direct non-test `.go` file in the eleven directories selected by `synthetic_source_pin.go`. It established exact path-set equality, strict duplicate-key-rejecting UTF-8 JSON, sorted unique safe paths, regular non-symlink files and ancestors, all byte lengths and SHA-256 values, the 2 MiB per-file, 16 MiB total and 256-entry limits, and the V1 aggregate.

The attributable producer-owned transcript established the mechanical facts. Its stricter final interpretation—that an additive successor could not change an existing source entry—was not adopted because the governing contract says a source change requires a new immutable manifest. Additivity applies to manifest artifacts: predecessor artifacts remain byte-identical while the new manifest exactly describes current source.

Integration receipt: `20261002000607-7104`.

## Predecessor delta

Added paths:

1. `internal/adr0011methodresult/b4_definition_bridge.go`
2. `internal/adr0011methodresult/b4_definition_private_adapter.go`
3. `internal/adr0011methodresult/b4_definition_response_custody.go`
4. `sessionruntime/b4_definition_private.go`

Removed paths: none.

Changed path:

- `sessionruntime/sessionruntime.go`: 101,515 bytes / `sha256:1aea61d1e737bd21adc30e91b214f0ca4d10da8c98eaad1e6a5db42568de65d2` to 103,099 bytes / `sha256:2ce0329b7df227342a07ea6e72cb848cb9f761038892efaec85e2d2992bee61b`.

## Immutable predecessors

- `adr0011-synthetic-source-pin.f8cf96e649a5fc05.manifest.json`: SHA-256 `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a`
- `adr0011-synthetic-source-pin.7a5f3200f8b985cc.manifest.json`: SHA-256 `7a5f3200f8b985cc6380b4d46e2bae4462e7bb55e2fd2c59964fb02c31490851`

Both remain unchanged. The rejected incomplete `6223a433…` candidate remains unselected.
