# Frozen private interface contract v3

**Token:** `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO`  
**Disposition:** `AUTHORED_NOT_GO`

## Boundary

This package freezes private compile-ready declarations only. All introduced identifiers are package-private. It changes no production/test source, public API, schema, registry, configuration, dependency, default, activation, evidence acceptance, or GO.

## Accounting and six components

`Manager.directionalRawV3` is the sole dedicated directional-raw subledger in the existing Manager accounting domain and lives exactly as long as Manager. Each admitted attempt creates one account sourced from that subledger. The subledger is metadata, never a recorder, backing owner, capacity source, or terminal owner. The existing `callHierarchyLayer1GenericOwner` remains the sole backing/terminal owner; `privateB4ByteAccountV2` and its leases remain the capacity source.

The exhaustive charged components are prepare frame, prepare result, selected handoff, directional frame, directional result, and retained external evidence. Each owned component has one prior reservation and corresponding active lease capacity. Alias bindings have zero charge and contain only owner lease ID, offset, length, and owner lifetime identity.

## Reservation

`reserveDirectionalComponentV3` accepts exact account, attempt, component, and byte count. It rejects nil/zero/invalid/stale/foreign/terminal/duplicate before effects; then checked-add overflow; exact per-attempt and aggregate 32 MiB bounds; source/lease/acquisition failures afterward. Exactly `32 << 20` may reserve. `(32 << 20)+1` refuses before read, allocation, copy, publication, provider effect, live-byte mutation, lease mutation, or owner mutation, while recording exactly one reached refusal preeffect fact.

## Lease-bearing acquisition

Owned acquisition is `privateCallHierarchyDirectionalAcquisitionV3`: bytes, capacity, and the complete active lease set travel together. Alias acquisition carries no bytes or lease charge; it carries a bounded `privateCallHierarchyDirectionalAliasV3` tied to one live owner lease. Owned copy source and alias/view source are distinct types. No method returns detached retained bytes; borrow callbacks cannot return or retain bytes; publication consumes a still-valid acquisition.

Transfer accepts a pointer to the complete source acquisition. Success installs complete bytes plus leases in the destination and zeros/invalidate source bytes, leases, alias, and validity atomically. Failure leaves the source as the one owner and destination unchanged.

## Terminal convergence and locking

Normal, complete-empty, malformed, protocol, transport, unsupported, timeout, cancellation, admitted refusal, explicit release, lease release, owner terminal request, STOP, RESTART, Shutdown, stale generation, callback/view/handoff races, and duplicate release converge on `releaseDirectionalTerminalV3`. One winner freezes terminal, invalidates aliases and handles, clears bytes, releases every lease exactly once, reconciles counters, assigns one nonzero release ID, removes the active attempt, and enables reuse only under a fresh identity. Losers are no-op. Double release and orphan lease are unrepresentable contract outcomes.

The complete nested order is `Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu`; locks release in reverse order. Owner-held code never acquires Manager. Account-held code never acquires owner or Manager. No callbacks, waits, allocation, provider work, hashing, publication, retry, channel, goroutine, try-lock, or asynchronous discharge occurs under the nested locks.

## Generic direction and target accounting

`CALLS_LSP_INCOMING_EXACT_V1` and `CALLS_LSP_OUTGOING_EXACT_V1` have independent identities, evidence, attempts, target sets, outcomes, terminals, reviews, and GO. Generic declarations contain no Go/gopls semantics. Each target accounting value is relative only to its exact target-set identity and profile. Attempted outcome ordinals and ordered unattempted frontier are disjoint and partition the target set exactly once; unknown denominator, incomplete custody/accounting, overflow, stale substitution, or bounded stop yields UNKNOWN.

## Invalid and refusal contract

Zero account, lease, transfer, release, attempt, component, direction, profile, ownership, state, stage, refusal (after refusal), terminal (after terminal), and lifetime values are invalid unless expressly named None/Unreserved. Nil Manager, owner, account, acquisition, source, destination, lease, or callback fails closed. Errors are returned only by acquisition/borrow/publication seams; typed stage/refusal values classify contract refusal without semantic terminal substitution.

## Stage A / Stage B

All 20 Stage A planned IDs from the committed TSV are constants immediately adjacent to their exact private wrapper symbols: six reservation/copy/alias triples, one whole-acquisition transfer, and one terminal release. Every wrapper is referenced explicitly by canonical planned call sites and delegates to the generic core; wrappers contain no independent semantics. Package verification parses the TSV and requires exact ID→symbol equality, 20 unique IDs and symbols, one adjacent definition and canonical reference per row, and zero extras or missing rows. They remain planned architecture obligations, never actual source-row identities or NECESSARY/REUSE classifications. Stage B begins only after separately authorized committed implementation and canonical enumeration.

AUTHORED_NOT_GO
