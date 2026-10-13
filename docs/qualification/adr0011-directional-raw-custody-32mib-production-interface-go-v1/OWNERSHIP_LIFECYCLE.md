# Ownership and lifecycle

## Ownership

The actual `Manager` privately owns the directional ledger and its shared real C15 account. Each directional attempt record points to that account and retains only real `privateB4ByteLeaseV2` values. Each generic owner owns exactly one directional terminal binding.

## Lock order

`Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu`.

No owner-terminal path reacquires `Manager.mu`. Terminal attempt records are reclaimed by the next Manager-locked attempt creation, so fresh reuse preserves lock order.

## Lifecycle convergence

- zero/init: `sessionruntime.New` creates the account and empty attempt map; first owner gets sequence 1.
- accepted/refused completion: both production owner constructors obtain a fresh Manager-ledger terminal.
- cancel/release: existing generic-owner terminal release releases all attempt leases once and clears the binding.
- STOP/RESTART: existing generic-owner retirement requests terminal and waits; the attached directional terminal therefore converges on the same path.
- Shutdown: closes admissions, waits workers, detaches all generic owners, requests/waits each terminal, then requests terminal release of the shared account.
- reuse: the next Manager-locked attempt removes terminal records and creates a fresh sequence/account record; it never reuses a released terminal.

The splice does not claim behavioral qualification. Compile and structural checks are `PRODUCTION_SPLICE_COMPILE_ONLY`.

Disposition: `AUTHORED_NOT_GO`.
