# State, lock, terminal, and ownership contract

## Lock order

Every Manager-global ledger/component/state/view/handoff/terminal/release operation acquires:

```text
Manager.mu -> callHierarchyLayer1GenericOwner.mu
```

Unlock order is owner then Manager. No owner-locked path may call Manager. Existing `callHierarchyLayer1GenericLease.Release` and owner `requestTerminal` inspect their immutable directional link before owner locking; linked paths call Manager, while unlinked paths retain current owner-only behavior.

## State machine

```text
unreserved -> reserved -> prepared/raw-owned -> selected/handoff
-> directional/raw-owned -> observed | refused -> released -> reusable
```

All arrows and views are Manager methods under both locks. Refusal stage precedence remains reservation < prepare custody < selection < handoff < directional custody < observation < release; within stage: invalid/foreign/stale, overflow, limit/slot, state, mutation, terminal race.

Observe/refuse select one immutable terminal winner. Release selects one immutable release winner and cannot alter terminal. Illegal transition, stale identity, post-terminal transfer/handoff, post-release view, duplicate release, overflow, timeout/partial relabeling, or reverse lock attempt is fail-closed/no-op.

## View/handoff versus release

- Release wins first: active ledger entry/link is removed or terminalized; view returns false and handoff returns release-stage terminal-race refusal with no bytes.
- View/handoff wins first: it validates/copies while holding both locks, then unlocks. Release may proceed afterward. Returned copies remain copies; custody references become invalid and cannot reacquire backing.
- No callback executes under either lock. No wait occurs while Manager lock is held.

## Atomic transfer

The source parameter is `*privateB4ByteLeaseV2`. Success under both locks installs bytes and the exact lease in one owner slot, then zeros the caller handle before unlock. Failure changes neither source nor destination. Alias has no lease/capacity and cannot escape. This forbids two independently releasable handles for one charge.

## Every terminal path

| Existing path | Directional action | Existing non-directional action |
|---|---|---|
| `callHierarchyLayer1GenericLease.Release` | linked Manager terminal release first | unchanged direct owner release |
| owner `requestTerminal` | linked Manager terminal release first | unchanged owner terminal release |
| STOP detached owner | existing requestTerminal call converges | unchanged |
| RESTART detached owner | existing requestTerminal call converges | unchanged |
| Shutdown owner list | existing requestTerminal call converges | unchanged |
| cancellation/normal terminal | existing owner terminal call converges | unchanged |
| duplicate/race | same Manager method returns false | existing exact-once behavior |

The Manager terminal method clears directional owner backing and ledger under `Manager.mu → owner.mu`, assigns nonzero release identity, makes slot reusable, and clears links. It then returns; existing generic release/drain continues without reacquiring Manager under owner lock.

## Ownership

| Object | Owns bytes? | Owns C15 lease? | Lock |
|---|---:|---:|---|
| generic owner directional slot | yes | one moved existing lease | Manager→owner |
| custody reference | no | no | validated under Manager→owner |
| component entry | no | no | Manager→owner |
| reservation/ledger | no | no | Manager |
| view/transfer result | copied bytes or invalidatable reference only | no | produced under Manager→owner |

C15 rows remain allocation/alias/transfer/release authority. Directional rows remain reservation/metadata authority. They join by attempt, owner slot/backing ID, operation, and release identity; totals never merge and no charge is nested beneath the directional ledger.
