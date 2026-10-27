# Private execution compare-absent dependency

This is an additive private dependency, not live integration or an acceptance decision.
No input-bundle publication, legacy CAS, legacy selector reader, Manager admission,
output retention, finalize, resume or recovery behavior is changed.

## Operation and evidence

`ContentRetentionRoot.OpenScopedExecutionJournalPrivate` admits only one exact
`feature-scope-execution-head-private-v1-<64 lowercase hex>.json` selector. It borrows
the admitted root and exposes neither root, filesystem path, nor descriptors. Reads
and writes are bounded to 16 KiB before copying/writing. The caller closes the root;
closing a journal consumes only that wrapper. Close/use are not concurrent.

Load uses the existing descriptor-bound no-follow private-file observer, including
exact metadata/identity checks and joined close errors. Only ENOENT at the initial
lstat, with no mixed error or cancellation, is absence. ENOENT at open or a later
observation is uncertainty. A present zero-byte file is a nonnil slice.

CompareAbsent creates one unlinked private source, writes, syncs and rereads its exact
bytes. Its sole namespace commit is Darwin `Fclonefileat` with both
`CLONE_NOFOLLOW_ANY | CLONE_RESOLVE_BENEATH` from that source FD or Linux
`Linkat(AT_EMPTY_PATH)` from an O_TMPFILE source. The Darwin flags are mandatory:
a dangling-destination-symlink test demonstrated that the flagless clone can create
the symlink target. With the flags, that test observes ELOOP and no target creation;
it is refusal/uncertainty, not a synthesized predecessor conflict. Unsupported flags
are errors, never permission to retry flagless. These operations refuse an existing
destination atomically; there is no check-then-overwrite, namespace retry, flock,
unlock, legacy CAS call, or readback promotion. Filesystem/kernel lack of support is
an error with no fallback. Other platforms explicitly refuse.

A receipt is constructed for that actual compare-absent operation only after a
successful install syscall. Directory sync, final observation and all owned closes
are attempted and original errors are joined. Any known commit retains its receipt
and exact submitted digest/length alongside later errors. Only clean EEXIST produces
an affirmative stale-predecessor result; EEXIST mixed with cleanup or cancellation
remains indeterminate, retaining the causes. The final observation is verification,
not an attempt to recover a failed install.

The receipt retains the existing `FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE`
qualification. Disposable-root tests establish exact capability-reopen observation and
atomic no-overwrite under concurrent claim attempts, not power-loss durability,
production filesystem support, producer authentication, or protection against a
same-user attacker. Injected faults establish error/receipt handling, not real disk
failure rates or recovery.

## Root-topology preflight and adapter boundary

`preflightScopedExecutionRootsPrivate(ctx, restart, exactInputPaths, outputRoots)`
privately mints an opaque, invocation-local `scopedExecutionTopologyPrivate`.
The retained reader now records its three actual admitted directory identities in
its private result, not caller assertions. Preflight reconstructs **read-only
ObservedRoot** wrappers from the exact primary, backup and journal paths the reader
opens, matches each live descriptor identity to those pins and the retained head,
and retains them until proof Close. No write wrapper is opened on input, and no
second bundle walk, URI rebasing or source-workspace policy is introduced.

`publication.CheckRetentionRootTopologyPrivate` is a narrow checking helper, not a
proof/authority constructor. It compares all three output roots mutually and each
against **all three input roots**, rejecting equality and ancestry in either
direction. It holds a bounded chain of no-follow component descriptors from `/`,
matches the final component to the exact admitted descriptor, compares device/inode
identities through `os.SameFile`, then rechecks every parent/name edge. This reuses
publication's existing no-follow opening layer; source-freeze's specialized file
observation is not substituted for retained-root admission. The 256-component bound
per root fails closed. Output paths under `/tmp` or `/private/tmp` are forbidden;
existing publication root admission is not relaxed. Temporary source checkouts
remain valid under the unchanged source-reading policy.

`newScopedExecutionStorePrivate(ctx, owner, outputRoots, topology)` **requires** the
matching opaque proof. Output capabilities/ordered identities and the owner's
input-head/scope/metadata pins must match it before opening the coordinator journal.
Construction and every Load/CAS validate all live roots, identities and exact paths
again, so a replaced/retired root, swapped path, mixed proof, or consumed proof fails
closed before journal IO. The store-agnostic test interface cannot construct a
production proof. There is no exported arbitrary-proof constructor or serialization.
Files and a new owner cannot reconstruct a lost Pending; provenance remains
executable/session/workspace/fixed-policy evidence only.

**Lifetime contract: NONCONCURRENT borrowed roots.** The whole future invocation
owns its output roots and must finish **all** owner/store calls, close stores, close
the topology proof's owned read-only input wrappers, then close outputs. No sharing
of wrappers across concurrent invocations, concurrent use/Close, or asynchronous
Close is supported. Independent invocations must open independent wrappers; they may
still contend atomically on the same coordinator namespace. Topology proof does
**not** change the borrowed-root API contract or supply a root lease. Close errors
are preserved, and consumed proof capabilities cannot be resurrected from copies.

These are safe-boundary identity/topology observations, not an atomic namespace
snapshot, resumable authority, arbitrary same-user deletion/rename protection,
power-loss durability, or permission to execute. The adapter is **NOT
live-qualified**. Live entrypoint wiring and output retention/budget remain
**Pending**; no real retained roots or live entrypoint are exercised by this unit.

## Verification limits

This topology unit passed normal and race runs of publication and censuslineagepilot
on Darwin (905 tests per run), focused topology/store tests (112), vet, gofmt and
`git diff --check`. Two narrowly targeted implementation reductions were observed
failing their own guards (`overlap admitted` and `stale proof admitted`), restored,
and observed passing. This is not complete Bar formal coverage: no exhaustive
assertion-by-assertion mutation/adequacy fixed point is claimed. No Linux runtime or
unsupported-platform qualification is claimed by this unit. Disposable owned home-directory
fixtures exercise disjoint roots, separately opened identical inodes, both ancestry
directions for every output pair and every input/output role, no-follow symlink
components, path swaps before preflight/store/operations, mixed proofs, consumed or
closed roots, and joined close faults. Recursive input-tree snapshots assert no
preflight/store-input writes. A controlled during-traversal rename/symlink test
checks descriptor-edge revalidation and all owned handle closes. These tests are
not a claim of mount-alias runtime qualification or concurrent namespace immunity.
