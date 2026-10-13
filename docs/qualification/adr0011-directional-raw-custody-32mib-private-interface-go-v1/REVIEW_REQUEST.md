# Independent review request

Requested token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_GO`

Current status: `AUTHORED_NOT_GO`.

A GO must confirm:

1. Every new declaration is private and the exact signatures/fields/constants compile as retained.
2. The Manager-global ledger is protected by existing `Manager.mu`, with universal order `Manager.mu → callHierarchyLayer1GenericOwner.mu` and no reverse acquisition.
3. Custody references the existing generic lease; bytes and the moved existing C15 lease live only in the sole owner, while reservations/ledger remain metadata only.
4. Transfer accepts `*privateB4ByteLeaseV2`; success atomically installs and zeros the source, while failure leaves it unchanged and no releasable duplicate exists.
5. View and Manager-owned handoff have exact release-race outcomes under both locks.
6. The directional generic lease/owner link routes explicit Release, requestTerminal, STOP, RESTART, Shutdown, cancellation, and duplicate races through one Manager terminal-release method before owner locking.
7. That method clears backing, removes the active ledger entry, assigns release identity, and makes the slot reusable exactly once; non-directional behavior remains unchanged.
8. Observe/refuse serialize under the same lock order and exactly one terminal wins; release cannot overwrite it.
9. Refusal stage/reason, zero/invalid semantics, and precedence are complete.
10. Limits and the exact state machine/race rules remain closed.
11. Generic Layer 1 remains opaque/direction-neutral with no duplicate authority or machinery.
12. C15 and directional accounting remain separately exhaustive and nonduplicating.
13. The exact production+test write-set is unchanged; historical tests remain immutable.
14. The disposable harness passes compile/vet for exact signatures and terminal call sites only.
15. Dependency/package checksums, starting HEAD, and final docs-only status read back exactly.

Return the token only for this private-interface contract. It authorizes no RED, implementation, repository production/test mutation, execution, activation, commit, or admission. Otherwise return `REVISE` or `REJECT` with exact reasons.

Disposition: `AUTHORED_NOT_GO`.
