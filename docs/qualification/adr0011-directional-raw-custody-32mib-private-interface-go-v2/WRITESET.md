# Exact immutable future production/test write-set

This interface phase changes none of the following files. A separately authorized future implementation is confined to exactly these paths.

## Production (six)

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` — new private generic declarations/mechanics frozen here.
2. `sessionruntime/sessionruntime.go` — Manager-global ledger field, pre-effect reservation, Manager-owned handoff, and transaction-scoped bound/link installation under existing `Manager.mu`.
3. `internal/lspwire/read_frame.go` — private bounded original-frame/incomplete-prefix result and method only.
4. `internal/lspwire/successor_ingress.go` — conditional already-reserved bound/prefix threading only; otherwise unchanged.
5. `sessionruntime/b4_owned_method_pair_lease.go` — direction-neutral backing/link only; linked terminal routes through Manager before owner locking; preserve sole owner, existing C15 lease, and generic terminal authority.
6. `sessionruntime/b4_c15_ownership_registry_private.go` — generated only after accepted enumeration; never hand-edited.

## Tests (four new files only)

1. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go` — `TestADR0011DirectionalRawV1Normal*`.
2. `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go` — `TestADR0011DirectionalRawV1Race*`; no build tag.
3. `internal/lspwire/read_frame_bounded_incomplete_v1_private_test.go` — `TestADR0011BoundedIncompleteFrameV1*`.
4. `internal/lspwire/successor_ingress_directional_v1_private_test.go` — `TestADR0011SuccessorIngressDirectionalV1*`.

`compileAllPlannedCallSites` calls `compilePlannedCallSitesForDirection` once for incoming and once for outgoing. For each direction it typechecks all claimed planned RED seams: exact 32 MiB and +1 reservation; zero/nil/invalid manager, lease, profile, direction, target, charge, custody, source, raw array, and ordinal shapes; preparation, selection, both assemblers, synchronized handoff, view, every disposition, every terminal, every refusal, every lifetime, attempted ordinal/outcome records, ordered frontier/unattempted ordinals, accounting equations, explicit release, lease release, owner terminal request, terminal release, race-shaped repeats, and duplicate release. These are compile-only canonical calls; RED files remain uncreated and unexecuted.

No existing test, fixture, corpus, golden, testdata, public contract, dependency, schema, profile registry, build tag, package boundary, predecessor, or provider binding may change. `CALLS_GO_GOPLS_*` bindings and their qualification evidence are outside this generic interface and outside this write-set.

No expansion or fallback exists. Any public/dependency/duplicate/architecture change is `BLOCKED_PUBLIC_SURFACE_OR_RAW_CUSTODY_CHANGE`.

AUTHORED_NOT_GO
