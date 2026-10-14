# Write set

## New files

- `internal/adr0011adr0007composition/compose.go`
- `internal/adr0011adr0007composition/compose_test.go`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/README.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/causal-evidence.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/fixtures.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/frozen-contract.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/manifests.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/recursive-ledger.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/verifier-tamper-checks.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/write-set.md`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/run_mutation_gates.py`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/checksums.sha256`
- `docs/qualification/adr0011-adr0007-private-causal-composition-qualification-v1/logs/*.log`

## Removed unauthorized identities

- `internal/adr0011adr0007composition/composition.go`
- `internal/adr0011adr0007composition/composition_test.go`

## Reverted/removed from rejected implementation

Tracked ADR0007 source-text-search files were restored to HEAD:

- `internal/adr0007sourcetextsearchv4private/search.go`
- `internal/adr0007sourcetextsearchv4private/types.go`

Rejected ADR0007-private selector files remain absent:

- `internal/adr0007sourcetextsearchv4private/private_causal_composition.go`
- `internal/adr0007sourcetextsearchv4private/private_causal_composition_test.go`

## Prohibited files not changed

No public API, public schema, CLI, MCP, Stage8, main, or production composition files were changed.
