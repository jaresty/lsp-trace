# Search v11 and Group v1 successor status

- **Date:** 2026-10-07
- **Status:** additive current-status record
- **Authority:** 0
- **Accepted:** false

## Frozen predecessor records

`experiment/search-custody-prospective-v11/PREDECESSORS.json` is bound into Search freeze `sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301`. Its blocked `v1` through `v10` predecessors remain unchanged.

`experiment/group-custody-prospective-v1/PREDECESSORS.json` is bound into Group freeze `group-freeze-f66b64da93d0ec4797d821fb788ec7afabeaca96c68375d9689bc88ad0ce6771`. Its blocked `Group execution`, `public CLI/MCP`, `ADR0011`, and `candidate-group` predecessors remain unchanged. In particular, candidate-group at `448a1f4f` remains wholesale blocked pending migration.

## Successor status

Search v11 reached bounded `SEARCH_CUSTODY_GO` at main `4bc75e97`, source `9c44d561`, for the exact frozen deterministic campaign only.

Group v1 reached bounded `GROUP_CUSTODY_GO` and the already-present `INTEGRATION_GO` at main `7f96fae9` for the exact frozen deterministic campaign only. Its accounting is infrastructure repair `1`; semantic retry / repair / substitution `0 / 0 / 0`.

These successor results do not mutate frozen predecessor bytes or reclassify blocked, rejected, superseded, `NOT_USEFUL`, or unevaluated history. They preserve `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.
