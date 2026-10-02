# ADR0011 synthetic source pin successor review — 5ba8c5d9

## Disposition

**ACCEPT_COMPLETE_ADDITIVE_SOURCE_PIN_SUCCESSOR_FOR_SELECTOR_UPDATE_ONLY.**

This acceptance is limited to selecting the reviewed source-byte manifest for the default-off private synthetic test path. It does not establish executable or binary identity, dependency closure, process provenance, producer authentication, occurrence qualification, production custody, public issuance, authority, semantic acceptance, or completeness outside the exact source census.

## Accepted candidate

- Manifest: `docs/qualification/adr0011-synthetic-source-pin.5ba8c5d954ee24ff.manifest.json`
- Manifest SHA-256: `5ba8c5d954ee24ff67bad9e756c243917b7005dda9b5391a714105551bf554d5`
- Manifest byte length: 15,361
- Schema: `ADR0011_SYNTHETIC_SOURCE_PIN_V1`
- Entries: 96
- Current source bytes: 650,859
- Aggregate: `sha256:58a520bd6e959677166ecb68be2e3187d44a2b97e9f6ef7eeca9539831d42544`

## Independent verification

A separate read-only reviewer independently derived the eleven selected directories from `synthetic_source_pin.go`, enumerated every direct non-test `.go` file, and established exact path-set equality. It also verified strict duplicate-key-rejecting UTF-8 JSON, sorted unique safe paths, regular non-symlink files and ancestors, every byte length and SHA-256 value, the 2 MiB per-file, 16 MiB total and 256-entry limits, the V1 aggregate, the full manifest hash, and predecessor immutability.

The attributable retained transcript reported `OMITTED []`, `EXTRA []`, `ERRORS []`, and `VERDICT ACCEPT`, then returned `ACCEPT_COMPLETE_ADDITIVE_SOURCE_PIN_SUCCESSOR_FOR_SELECTOR_UPDATE_ONLY`.

Integration receipt: `20261002001743-9517`.

## Immediate predecessor delta

Relative to immutable candidate `94a1f3f3562151be3def758fd441ab45afc1a34981acb2313733bb887ec8823f`:

- Added paths: none.
- Removed paths: none.
- Changed path: `internal/adr0011acquisition/production_attempt_custody.go`, unchanged length 14,307 bytes, from `sha256:3afa0b127599d0480d949367099d08f2ad73d85f4e5ae8a73da828aa05ed5459` to `sha256:a68d17d6df009c7e4652b22ed9b93973d090eba0e5cb117a6743af521790e7ef`.

## Immutable predecessors

The reviewer verified these artifacts by complete content hash and found their bytes unchanged:

- `adr0011-synthetic-source-pin.94a1f3f3562151be.manifest.json`: `94a1f3f3562151be3def758fd441ab45afc1a34981acb2313733bb887ec8823f`
- `adr0011-synthetic-source-pin.7a5f3200f8b985cc.manifest.json`: `7a5f3200f8b985cc6380b4d46e2bae4462e7bb55e2fd2c59964fb02c31490851`
- `adr0011-synthetic-source-pin.f8cf96e649a5fc05.manifest.json`: `f8cf96e649a5fc051fa6475f8b68fe38bcc346e52fd459bb946d99951f57269a`

The rejected incomplete `6223a433…` candidate remains unselected. The superseded `94a1f3f3…` candidate remains immutable.
