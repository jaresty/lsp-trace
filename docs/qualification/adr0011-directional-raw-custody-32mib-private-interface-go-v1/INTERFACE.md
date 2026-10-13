# Frozen private interface contract

## Scope and existing authority

All additions are package-private. `Manager.mu` is the common lock for the Manager-global one-slot directional ledger. `callHierarchyLayer1GenericOwner.mu` remains the sole owner/backing lock. The universal order is **`Manager.mu → owner.mu`**. No path may acquire `Manager.mu` while holding `owner.mu`; owner callbacks and waits run with neither Manager lock nor a nested reverse acquisition.

Generic Layer 1 remains direction-neutral. Directional policy is Manager metadata. No exported/public/default/global change, second owner, second terminal, second C15 charge, new mutex, goroutine, channel, callback system, or write-set expansion is admitted.

## Exact nonduplicating custody and terminal links

```go
type privateCallHierarchyDirectionalTerminalLinkV1 struct {
 manager *Manager
 reservation *privateCallHierarchyDirectionalReservationV1
}
type privateCallHierarchyDirectionalCustodyV1 struct {
 lease callHierarchyLayer1GenericLease
 slot uint8
 attempt privateCallHierarchyDirectionalAttemptIDV1
}
```

The existing generic owner adds direction-neutral `directionalRawV1 [7]privateCallHierarchyDirectionalOwnerBackingV1` and `directionalLinkV1`. A directional `callHierarchyLayer1GenericLease` carries `directionalV1 *privateCallHierarchyDirectionalTerminalLinkV1`. Each backing owns the only bytes and the one moved existing `privateB4ByteLeaseV2`; reservation/component/ledger records contain references and metadata only.

## Exact constants/types

Canonical complete fields and enums are in `sessionruntime_private.go.txt` and `lspwire_private.go.txt`. Typed limits remain 32 MiB per attempt, 32 MiB aggregate, one active slot, seven component slots. Refusal stage and reason remain an immutable pair with the previously frozen precedence.

## Exact signatures

```go
func newPrivateCallHierarchyDirectionalLedgerV1() privateCallHierarchyDirectionalLedgerV1
func (m *Manager) reserveCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, direction privateCallHierarchyDirectionV1, requested uint64) (*privateCallHierarchyDirectionalReservationV1, privateCallHierarchyDirectionalRefusalStageV1, privateCallHierarchyDirectionalRefusalV1)
func (m *Manager) transferCallHierarchyDirectionalComponentV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1, component privateCallHierarchyDirectionalComponentV1, backingID, capacity uint64, raw []byte, charge *privateB4ByteLeaseV2, source privateCallHierarchySourceFormV1, ownership privateCallHierarchyDirectionalOwnershipV1) (privateCallHierarchyDirectionalCustodyV1, privateCallHierarchyDirectionalRefusalStageV1, privateCallHierarchyDirectionalRefusalV1)
func (m *Manager) advanceCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1, from, to privateCallHierarchyDirectionalStateV1) (privateCallHierarchyDirectionalRefusalStageV1, privateCallHierarchyDirectionalRefusalV1)
func (m *Manager) viewCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1) (privateCallHierarchyDirectionalViewV1, bool)
func (m *Manager) observeCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1, terminal privateCallHierarchyDirectionalTerminalV1) (privateCallHierarchyDirectionalRefusalStageV1, privateCallHierarchyDirectionalRefusalV1)
func (m *Manager) refuseCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1, stage privateCallHierarchyDirectionalRefusalStageV1, reason privateCallHierarchyDirectionalRefusalV1, terminal privateCallHierarchyDirectionalTerminalV1) bool
func (m *Manager) releaseCallHierarchyDirectionalV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1) bool
func (m *Manager) releaseCallHierarchyDirectionalTerminalV1(owner *callHierarchyLayer1GenericOwner, r *privateCallHierarchyDirectionalReservationV1) bool
func (m *Manager) handoffCallHierarchySelectedRawV1(lease callHierarchyLayer1GenericLease, r *privateCallHierarchyDirectionalReservationV1, custody privateCallHierarchyDirectionalCustodyV1, selected privateCallHierarchyRawSelectionV1) (privateCallHierarchyDirectionalTransferV1, privateCallHierarchyDirectionalRefusalStageV1, privateCallHierarchyDirectionalRefusalV1, error)
func scanCallHierarchyRawArrayV1(raw []byte, maxMembers uint64) (privateCallHierarchyRawArrayV1, error)
func selectCallHierarchyPreparedOrdinalV1(array privateCallHierarchyRawArrayV1, ordinal uint64) (privateCallHierarchyRawSelectionV1, error)
func assembleIncomingCallHierarchyParamsV1(selected privateCallHierarchyRawSelectionV1) ([]byte, error)
func assembleOutgoingCallHierarchyParamsV1(selected privateCallHierarchyRawSelectionV1) ([]byte, error)
func (r *Reader) readWithBoundedIncompleteFrameV1(ctx context.Context, reserved []byte, limit uint64) (boundedIncompleteFrameV1, error)
```

Existing exact terminal call sites are amended only in `b4_owned_method_pair_lease.go`:

```go
func (l callHierarchyLayer1GenericLease) Release() bool
func (o *callHierarchyLayer1GenericOwner) requestTerminal()
```

For a non-directional nil link, their current owner-only behavior is byte/value identical. For a directional link, each calls `releaseCallHierarchyDirectionalTerminalV1` **before acquiring `owner.mu`**. STOP, RESTART, Shutdown, cancellation, explicit generic lease release, and every teardown path therefore converge on the same Manager method.

## Transfer primitive

`transfer...` accepts `*privateB4ByteLeaseV2`. Under `Manager.mu → owner.mu`, success validates source handle, copies/installs bytes, moves the exact lease value into the owner slot, and assigns `*charge = privateB4ByteLeaseV2{}` in the same critical section. No releasable duplicate exists after success. Any validation/allocation/state/limit failure leaves bytes, source handle, owner slots, ledger, and counters unchanged. Nil or inactive/foreign source refuses.

## View and handoff races

`view...` and Manager handoff both acquire `Manager.mu → owner.mu`, validate active ledger pointer, attempt identity, owner/lease/link, slot validity, and terminal/release state, then return copied metadata and bounded custody reference. If release has won or begins first, they return false or `Release + TerminalRace` and no bytes. If view/handoff holds both locks first, it completes a copy/reference validation, unlocks, and later release clears backing; the returned custody reference cannot reacquire bytes after invalidation. Handoff is never a free function.

## Terminal integration and exact-once release

`releaseCallHierarchyDirectionalTerminalV1` acquires `Manager.mu`, then `owner.mu`. It validates the exact active reservation/link, chooses terminal/release only once, invalidates all custody references, clears owner bytes, releases each moved C15 lease once, assigns monotonically increasing nonzero `ReleaseID`, decrements live bytes/slot, removes `ledger.active`, marks released then reusable, clears both directional links, unlocks owner then Manager, and returns whether it won.

Explicit lease Release, requestTerminal, detached STOP/RESTART owners, Shutdown owners, cancellation, and concurrent duplicates all invoke this method. One wins; losers return false/no-op and never release a charge twice or change release identity. Existing generic `releaseLocked` then clears generic observation under owner-only synchronization after directional cleanup; non-directional paths are unchanged. No path invokes Manager while owner is held.

## Zero/error/refusal

Zero ledger is inert; zero state is unreserved; all other zero enums are invalid except `RefusalNone` paired with invalid stage before refusal. Nil Manager/owner/reservation/link/charge, foreign/stale identity, invalid stage/reason, overflow, limit, state, mutation, or terminal race fail without mutation. Closed-domain failures return stage+reason; scanner/selection/assembly/handoff syntax failures include `error`; Reader preserves typed result plus underlying error.
