# Successor impact on private interface v2

Status: design contract only; not a compile contract and not permission to edit immutable v2.

## Required successor types

A later interface v3 (or other explicitly reviewed successor) must express at least:

```go
type privateCallHierarchyDirectionalAccountIDV2 uint64
type privateCallHierarchyDirectionalLeaseIDV2 uint64
type privateCallHierarchyDirectionalTransferIDV2 uint64

type privateCallHierarchyDirectionalAccountV2 struct {
    ID privateCallHierarchyDirectionalAccountIDV2
    Attempt privateCallHierarchyDirectionalAttemptIDV1
    Direction privateCallHierarchyDirectionV1
    LiveBytes, ReservedBytes uint64
    State privateCallHierarchyDirectionalStateV1
    Components [6]privateCallHierarchyDirectionalComponentAccountV2
}

type privateCallHierarchyDirectionalLeaseV2 struct {
    ID privateCallHierarchyDirectionalLeaseIDV2
    Account privateCallHierarchyDirectionalAccountIDV2
    Attempt privateCallHierarchyDirectionalAttemptIDV1
    Component privateCallHierarchyDirectionalComponentV1
    Capacity uint64
    Active bool
    BackingID uint64
}

type privateCallHierarchyDirectionalAliasV2 struct {
    OwnerLease privateCallHierarchyDirectionalLeaseIDV2
    Offset, Length uint64
}

type privateCallHierarchyDirectionalAcquisitionV2 struct {
    Account privateCallHierarchyDirectionalAccountIDV2
    Attempt privateCallHierarchyDirectionalAttemptIDV1
    Component privateCallHierarchyDirectionalComponentV1
    Bytes []byte
    Capacity uint64
    Leases []privateCallHierarchyDirectionalLeaseV2
    Alias *privateCallHierarchyDirectionalAliasV2
    Valid bool
}
```

Names are architecture identifiers only. A successor interface must independently prove package typing and may refine representation without weakening the invariants.

## Required successor signatures

```go
reserveDirectionalComponentV2(account, attempt, component, bytes) (reservation, stage, reason)
acquireDirectionalOwnedV2(reservation, source) (acquisition, stage, reason, error)
declareDirectionalAliasV2(ownerAcquisition, component, offset, length) (acquisition, stage, reason)
transferDirectionalAcquisitionV2(source *acquisition, destination) (transferID, destinationAcquisition, stage, reason)
borrowDirectionalV2(acquisition, fn) error
publishDirectionalV2(acquisition, destination) (receipt, stage, reason)
releaseDirectionalTerminalV2(account, terminal) bool
```

The successor lock contract is exact: any nested account operation uses `Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu` and releases in reverse order. Account-local entrypoints must never acquire owner or Manager locks while holding `privateB4ByteAccountV2.mu`; they return/retry through a Manager-owned entrypoint after dropping the account lock. Terminal release is the sole nested-discharge coordinator and uses a non-callback account primitive.

Contract requirements:

- reserve identifies exact component/bytes/attempt/account and precedes every owned copy/effect;
- owned acquisition creates or receives bytes only together with corresponding active leases;
- alias declaration creates no lease/charge and binds to one owning lease range;
- transfer consumes the complete source handle atomically and never accepts detached `raw []byte` plus detached lease;
- borrow cannot retain or return bytes;
- publication requires a still-valid acquisition and cannot create an uncharged retained copy;
- release is the sole convergence point;
- reserve, alias, transfer, ordinary release, and terminal release expose no callback or reverse-lock path from the account;
- terminal release invalidates aliases/bytes and discharges leases/account counters atomically under the complete order.

## Exact v2 corrections

Immutable v2 must not be edited. Its successor must change these design-level contracts:

| v2 contract | successor requirement |
|---|---|
| transfer accepts detached raw bytes and one `*privateB4ByteLeaseV2` | transfer accepts one lease-bearing acquisition containing all bytes and corresponding leases |
| custody metadata owns neither bytes nor lease | acquisition is the non-escapable lifetime handle for bytes plus leases (or bounded alias) |
| seven generic component slots | six exhaustive charged component classes; aliases are rows bound to owners, not charged slots |
| validation can occur after caller has raw bytes | reservation and acquisition occur before every owned copy/effect |
| one moved charge is sufficient | lease set must correspond exactly to all retained owned capacities |

No claim is made that these declarations compile against current source. This is sufficient design detail for a later separately reviewed interface update only. A successor must reject account-first, owner-first-then-Manager, post-unlock discharge, account callback, try-lock/timeout/asynchronous discharge, and duplicate owner/account/terminal alternatives.

## Frozen compatibility and authority

The successor must preserve generic provider-neutral profile identity, independent incoming/outgoing contracts, sole owner/terminal, exact terminal distinctions, target-set accounting, no-history-change, disabled/`CALLS_ONLY` value equality, authority `0`, default off, `CALLS_ONLY`, and unchanged Definition/Reference behavior. It cannot import Go/gopls semantics into the generic interface.
