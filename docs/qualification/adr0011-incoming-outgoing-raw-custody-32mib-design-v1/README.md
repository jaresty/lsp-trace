# ADR0011 incoming/outgoing raw-custody 32 MiB design v1

Status: `AUTHORED_NOT_GO`

Design only: no production/test implementation, execution, corpus/evidence mutation, profile activation, public API/schema/CLI/MCP change, commit, or deployment is authorized.

## Immutable pins

- HEAD `7f09e5357b117d20449fc16097631e32d4eead7f`
- parent `4b910fc0c517cb7801fb51d17d295897230d11fa`
- target-corpus manifest SHA-256 `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d`
- 5900 targets
- predecessor `docs/qualification/adr0011-incoming-outgoing-calls-qualification-v1/` remains byte-identical and excluded from this package manifest

## Frozen design boundary

The sole generic terminal owner remains direction-neutral. A new Manager-owned directional raw-custody ledger is separate from C15 backing accounting. V1 permits one active slot, one per-attempt ceiling of `33554432`, and one aggregate-live ceiling of `33554432`, with atomic reservation before effect and exact release/reuse.

Incoming and outgoing share only generic scanner/selector mechanics; they retain separate assemblers, attempts, reservations, semantics, evidence, and review. Failed transport reads retain only bounded actually consumed bytes as an incomplete opaque prefix at the exact scanner boundary, never as reconstructed or complete data.

Required approval token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_DESIGN_GO`.

No history change is permitted: disabled and `CALLS_ONLY` executions must preserve readiness-history entries, bytes, digests, directions, ordering, identities, receipt/borrower counts, lifecycle transitions, predecessor commitments, and generic attempt lineage byte-for-byte or value-for-value. Prefix mapping IDs are directional correlation metadata only and cannot alter, replace, append to, reorder, or reinterpret predecessor/history semantics.

Any pin drift, public/global change, duplicate owner/terminal, merged C15/directional accounting, reconstruction, impossible private seam, or history/lineage semantic or byte-value change is a hard stop and requires `REVISE`.
