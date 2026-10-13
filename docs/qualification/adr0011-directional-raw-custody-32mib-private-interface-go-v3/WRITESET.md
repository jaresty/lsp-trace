# Exact immutable future production/test write-set

This interface phase changes none of these files. A separately authorized implementation is confined to exactly six production and four new test paths.

## Production — exactly six

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go`
2. `sessionruntime/sessionruntime.go`
3. `internal/lspwire/read_frame.go`
4. `internal/lspwire/successor_ingress.go`
5. `sessionruntime/b4_owned_method_pair_lease.go`
6. `sessionruntime/b4_c15_ownership_registry_private.go` — generated only after accepted postimplementation enumeration; never hand-edited.

## Tests — exactly four new files

1. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go`
2. `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go`
3. `internal/lspwire/read_frame_bounded_incomplete_v1_private_test.go`
4. `internal/lspwire/successor_ingress_directional_v1_private_test.go`

Canonical planned RED call sites explicitly invoke all 20 TSV-named wrappers for both directions. They additionally cover every constructor, exact 32 MiB/+1 and checked-overflow shapes, broad zero/nil/invalid/error/refusal shapes, owned versus alias/view lifetimes, complete bytes+lease transfer success and failure-source-preservation signatures, nil callback refusal, publication refusal, all terminal outcomes, STOP/RESTART/Shutdown/stale-generation, complete Manager→owner→account lock-order and account-held inversion-refusal surfaces, exact C15 lease-bearing reconciliation, three concurrent transfer/release/borrow race-shaped goroutine calls with deterministic WaitGroup completion, and a separate sequential duplicate release. These are compile/interface shapes only and assert no runtime behavior.

No existing test, fixture, corpus, golden, testdata, public contract, dependency, schema, profile registry, configuration, build tag, package boundary, predecessor, or provider binding may change. No expansion or fallback exists.

`BLOCKED_WRITESET_EXPANSION_REQUIRED`

AUTHORED_NOT_GO
