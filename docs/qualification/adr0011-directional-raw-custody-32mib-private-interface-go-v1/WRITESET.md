# Immutable production and test write-set reference

This interface changes none of these files. A later separately authorized phase is confined to exactly:

## Production (six)

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` — new private declarations/mechanics frozen here.
2. `sessionruntime/sessionruntime.go` — add the Manager-global directional ledger field; reservation-before-effect, Manager-owned handoff, and transaction-scoped bound/link installation only; all under existing `Manager.mu`.
3. `internal/lspwire/read_frame.go` — private bounded incomplete-prefix result/method only.
4. `internal/lspwire/successor_ingress.go` — conditional already-reserved bound/prefix threading only; otherwise byte-identical.
5. `sessionruntime/b4_owned_method_pair_lease.go` — add only direction-neutral owner backing/link and directional generic-lease link; route existing lease `Release` and owner `requestTerminal` through the linked Manager method before owner locking; retain existing owner lock, callback drain, requestTerminal/waitReleased, and generic terminal authority; no direction semantics, second owner/charge, or second terminal.
6. `sessionruntime/b4_c15_ownership_registry_private.go` — generated only after accepted enumeration; never hand-edited.

## Tests (four new files only)

1. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go` — prefix `TestADR0011DirectionalRawV1Normal`.
2. `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go` — prefix `TestADR0011DirectionalRawV1Race`, no build tag.
3. `internal/lspwire/read_frame_bounded_incomplete_v1_private_test.go` — prefix `TestADR0011BoundedIncompleteFrameV1`.
4. `internal/lspwire/successor_ingress_directional_v1_private_test.go` — prefix `TestADR0011SuccessorIngressDirectionalV1`.

Every exact RED selector listed in `docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1/WRITESET.md` calls only symbols declared by this interface or existing pinned package-private seams. All stateful calls carry the existing generic lease and route through Manager methods under `Manager.mu → owner.mu`; transfer uses `*privateB4ByteLeaseV2`, handoff is a Manager method, and terminal tests compile explicit Release/requestTerminal/teardown convergence. No test may call a reservation mutation method or acquire Manager while owner is held. The disposable harness compiles representative calls for every frozen signature; the later RED files remain uncreated and unauthorized.

No existing test, fixture, corpus, golden, testdata, public contract, schema, profile, registry, build tag, package boundary, or qualification predecessor may change. No expansion or fallback write-set exists. Current >16 MiB real-path behavior remains `BLOCKED_NOT_RED`.
