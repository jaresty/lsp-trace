# State, ownership, accounting, and synchronization

## Independent directional state

Incoming and outgoing each bind one exact generic profile identity, target identity, target-set accounting record, preparation transaction, selected prepared-item ordinal, directional transaction, occurrence ledger, terminal, disposition, frontier, and release identity. No state or evidence is inherited across directions.

```text
unreserved -> reserved -> prepared/raw-owned -> selected/handoff
-> directional/raw-owned -> accounted -> terminal -> released -> reusable
```

A refusal may terminalize at the exact reached stage. Terminal is immutable. Release is separately exact-once and cannot rewrite terminal classification.

## Lock order

Every stateful operation acquires:

```text
Manager.mu -> callHierarchyLayer1GenericOwner.mu
```

Unlock order is owner then Manager. No callback or wait runs under either lock. No owner-locked path calls Manager. Existing lease `Release` and owner `requestTerminal` inspect the directional link before owner locking and route linked paths through Manager; nil/unlinked paths preserve existing behavior.

## Sole ownership and transfer

| Object | Bytes | C15 lease | Role |
|---|---:|---:|---|
| generic owner slot | sole owner | one moved existing lease | opaque backing |
| custody | no | no | invalidatable reference |
| component/reservation/ledger | no | no | metadata/accounting |
| view/transfer | copied metadata/bytes or bounded alias | no | synchronized handoff |

Success moves the pointer-supplied C15 lease and zeros its source atomically. Failure mutates nothing. Directional reservation accounting never duplicates C15 charging.

## Lifetime and races

Borrowed input exists only for the call. Copied values have independent bytes or metadata. Aliased/view custody is non-owning and valid only while exact attempt/slot/link/release identity remains active. Release marks lifetime released and invalidates all aliases.

If release wins first, view returns false and handoff returns `Release/TerminalRace` with no bytes. If view/handoff holds both locks first, its bounded copy/validation completes before release. Selected ordinal is copied unchanged. No free-function handoff exists.

## Reservation and release

Reservation validates nil/identity/direction/profile, overflow, one-slot availability, and exact 32 MiB per-attempt/aggregate bounds before effects. `32 MiB + 1` has no effect. Release clears backing, releases the moved lease once, removes active ledger identity, decrements live bytes/slot, assigns one nonzero release ID, clears links, and permits exact slot reuse. Duplicate release returns false/no-op.

## Target accounting

The target-set record carries an exact target-set digest, denominator-known bit, denominator, attempted/unattempted totals, all terminal class totals, exact attempted target ordinal/outcome records, frontier stage/reason, and ordered unattempted ordinals. Attempted ordinals are unique; unattempted ordinals are strictly ordered from the frontier; the sets are disjoint and cover the exact target set once. The two accounting equations in `INTERFACE.md` are mandatory. Unknown or inconsistent accounting is partial/UNKNOWN, never complete.

## Compatibility

Layer 1 remains opaque and direction-neutral. D/R identities, occurrences, ledgers, schemas, diagnostics, and evidence are never retyped as CALLS. `CALLS_ONLY`, disabled paths, defaults, history, and lineage remain unchanged.

AUTHORED_NOT_GO
