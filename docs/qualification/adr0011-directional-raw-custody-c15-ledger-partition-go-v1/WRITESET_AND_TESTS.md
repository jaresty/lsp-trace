# Exact future write-set and causal test contract

No file listed here is authorized for mutation by this package.

## Production paths — exact set, no others

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` — new directional partition, IDs, one-slot ledger, components, refusal facts, and terminal convergence.
2. `sessionruntime/sessionruntime.go` — Manager fields, construction, admission-before-effect, STOP/RESTART/Shutdown/cancel integration.
3. `sessionruntime/b4_explicit_bytes_c15_private.go` — explicit general/directional partition and combined-ceiling integration while preserving the 24,600-byte legacy layout and current general behavior.
4. `sessionruntime/b4_owned_method_pair_lease.go` — bind the existing sole generic owner/terminal to directional reservation and exact release.
5. `sessionruntime/b4_c15_ownership_registry_private.go` — generated only after Stage B enumeration; no hand edit.
6. `internal/lspwire/read_frame.go` — private bounded complete/incomplete custody threading only; public/default behavior unchanged.
7. `internal/lspwire/successor_ingress.go` — thread an already admitted private directional bound only if used by the real opted-in Manager path.

Pinned dependencies, not write targets: `internal/lspwire/wire.go`, `internal/provider/provisioning.go`, `sessionruntime/b4_definition_private.go`, `sessionruntime/readiness_history_private.go`, and `sessionruntime/owned_method_pair.go`.

## Test paths — exact set, no others

1. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go`
2. `sessionruntime/call_hierarchy_directional_partition_integration_private_test.go`
3. `sessionruntime/call_hierarchy_directional_lifecycle_private_test.go`
4. `sessionruntime/call_hierarchy_directional_compatibility_private_test.go`
5. `internal/lspwire/read_frame_directional_private_test.go`
6. `internal/lspwire/successor_ingress_directional_private_test.go` only if production path 7 changes; otherwise it must not be created.

Docs/evidence successor paths are restricted to a new versioned qualification package plus deterministic generated Stage A/B artifacts; predecessor package paths remain immutable.

Any additional production, test, public API, dependency, schema, profile, registry, CLI/MCP, config, build-tag, history, predecessor, or lineage path is `REVISE_WRITESET`.

## Required tests

| Case | Setup | Required observation |
|---|---|---|
| directional exact limit | directional=33,554,432; general valid | accept exactly one slot; exact IDs/counters |
| directional plus one | directional=33,554,433 | pre-effect `DIRECTIONAL_LIMIT`; all state unchanged |
| aggregate exact | general=33,554,432; directional=33,554,432 | checked combined=67,108,864 accepted |
| aggregate plus one/overflow | combined=67,108,865 or uint64 overflow | pre-effect exact refusal; both partitions unchanged |
| existing/general baseline | fill/release around current 24,600 baseline and 33,554,432 max | existing equality/+1 behavior byte/value unchanged |
| concurrent partitions | general activity while one directional attempt active | independent counters; no borrowing; combined ceiling enforced |
| second directional attempt | incoming vs outgoing and same-direction collisions | exactly one winner; loser pre-effect `DIRECTIONAL_SLOT_BUSY` |
| independent semantics | same raw mechanics for incoming/outgoing | separate attempts, target sets, results, evidence, terminal IDs |
| alias | alias same partition/owner lifetime | zero bytes; cannot outlive or cross partition |
| transfer prohibition | attempt general↔directional transfer | pre-effect `PARTITION_OR_COMPONENT`; source intact |
| release/reuse | normal/error/refusal/cancel/STOP/RESTART/Shutdown | clear-before-release, exact-once decrement, fresh IDs on reuse |
| release races | release vs callback/view/handoff/Shutdown/RESTART | one winner, losers no-op, no orphan/stale access |
| precedence collisions | multiple simultaneously failing guards | earliest reached row retained; no relabel/effect |
| compatibility | disabled, D, R, generic, CALLS_ONLY | byte/value-identical history, predecessors, lineage, output; zero new work |
| deterministic enumeration | same exact manifest twice | byte-identical Stage A/B outputs and checksums |

Tests must assert no provider write, scanner read, allocation, copy, hash, publication, semantic output, counter mutation, or owner mutation occurs before a refused reservation. No test may raise a global constant or inject after the guard it claims to test.

AUTHORED_NOT_GO
