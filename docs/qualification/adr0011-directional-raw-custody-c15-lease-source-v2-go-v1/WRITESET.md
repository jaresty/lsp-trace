# Exact immutable future write-set

Status: `AUTHORED_NOT_GO`. This package changes none of these production/test files.

## Production: exactly six

1. `sessionruntime/call_hierarchy_directional_raw_v1_private.go` — required-new private mechanics; successor lease-bearing acquisition/account/alias/transfer declarations.
2. `sessionruntime/sessionruntime.go` — one Manager subledger; reserve-before-effect integration and terminal convergence under existing lock.
3. `internal/lspwire/read_frame.go` — private bounded acquisition result carrying bytes with corresponding lease source; defaults unchanged.
4. `internal/lspwire/successor_ingress.go` — conditional already-reserved bound and acquisition threading only; byte-identical if unused.
5. `sessionruntime/b4_owned_method_pair_lease.go` — sole direction-neutral owner/terminal integration, atomic acquisition transfer, alias invalidation, exact release.
6. `sessionruntime/b4_c15_ownership_registry_private.go` — generated only from accepted `UNRESOLVED=0` enumeration; never hand-edited.

## Tests: exactly four new files

1. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go`.
2. `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go`.
3. `internal/lspwire/read_frame_bounded_incomplete_v1_private_test.go`.
4. `internal/lspwire/successor_ingress_directional_v1_private_test.go`.

The existing selector/package ownership remains unchanged. Future tests must additionally prove every component reservation tuple, lease-bearing no-escape acquisition, zero-charge alias lifetime, atomic bytes+lease transfer in both success/failure, every terminal/race release convergence, no double/orphan, and exact C15 delta reconciliation.

Without adding files, `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go` owns planned adversarial schedules for terminal release versus account reserve, alias declaration/use, acquisition transfer, and ordinary account/lease release. Each schedule must prove the only nested order is `Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu`, account-held paths never acquire owner/Manager, one terminal winner, bounded completion without deadlock/livelock, exact counter/lease/alias postconditions, and duplicate losers as no-op. `sessionruntime/call_hierarchy_directional_raw_v1_private_test.go` owns deterministic lock-contract/refusal and terminal-discharge postconditions. The two existing `internal/lspwire` test files remain transport-only and must not fabricate account locking. These are planned obligations only; no test is created or run here.

## Hard stop

There is no fallback or expansion. If any architecture, interface, RED, implementation, enumeration, generation, compatibility, or release obligation requires an unlisted production/test path, historical test/fixture/corpus/golden mutation, public surface, dependency, schema/profile/registry/config/build-tag/package-boundary change, the exact disposition is:

`BLOCKED_WRITESET_EXPANSION_REQUIRED`

No implementation or test mutation may proceed under this package.
