# Ownership and lifecycle splice v2

## Basis

Exact source at `ff74ed966d9580e60ea10a3cad887f92449aa755` already routes accepted and refused generic attempts through one `callHierarchyLayer1GenericOwner`, and STOP, RESTART, and Shutdown detach that owner and call `requestTerminal`/`waitReleased` outside `Manager.mu`. The proposal attaches one directional terminal binding to that existing owner rather than creating a parallel lifecycle authority.

## Splice

1. `sessionruntime.New` constructs the Manager-owned real account and directional attempt map; failure aborts construction.
2. Accepted and refused generic owner construction each calls `newDirectionalTerminalV1Locked` under `Manager.mu`.
3. Owner `releaseLocked` clears the directional binding through its exact-once terminal path before clearing retained observation bytes.
4. Explicit release and context cancellation converge through owner terminal release.
5. STOP and RESTART use the existing detach/request/wait loops; old-generation directional leases settle before reuse.
6. Shutdown closes admission, waits workers, detaches and joins all generic owners, then requests terminal release of the shared account.
7. Terminal attempts are removed before the next Manager-locked attempt creation; sequence and reservation generations remain fresh.

Lock order remains `Manager.mu -> callHierarchyLayer1GenericOwner.mu -> privateB4ByteAccountV2.mu`. No owner terminal path reacquires `Manager.mu`; no wait occurs under `Manager.mu`.

The package proves reachable lifecycle/account plumbing and compile coherence only. It does not prove semantic C15 completeness or production behavior.

AUTHORED_NOT_GO
