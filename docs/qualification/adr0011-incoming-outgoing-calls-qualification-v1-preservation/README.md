# ADR 0011 incoming/outgoing CALLS qualification v1 preservation

Status: `BLOCKED_UNJUSTIFIED_NUMERIC_BOUNDS`

This receipt durably preserves the predecessor package at:

`docs/qualification/adr0011-incoming-outgoing-calls-qualification-v1/`

The predecessor remains byte-for-byte unchanged. Its blocked decision is retained as negative evidence and is superseded for raw-custody architecture only by the independently reviewed package committed at `0e2583b2a535701da152e756253b4b96df55272b`:

`docs/qualification/adr0011-incoming-outgoing-raw-custody-32mib-design-v1/`

Supersession does not convert the predecessor into GO, rewrite its numeric audit, grant profile authority, or authorize implementation beyond the reviewed raw-custody write-set.

## Frozen identities

- Source revision when predecessor was authored: `4b910fc0c517cb7801fb51d17d295897230d11fa`
- Predecessor `MANIFEST.sha256` file SHA-256: `1ce27d0f9c21bfafd65631e6238faf46c817fb1f920c19ae74f3b7c433a2f805`
- Predecessor `DEPENDENCIES.sha256` file SHA-256: `6110847fc208642131fe90688a3338fb1af7630e23388a3a2f8d458ec3a18d44`
- Superseding design commit: `0e2583b2a535701da152e756253b4b96df55272b`
- Superseding design tree: `3e72f873853117b20b7e86a9c874fd7e38491a81`

`PREDECESSOR.sha256` enumerates every preserved predecessor file and must verify from the repository root.
