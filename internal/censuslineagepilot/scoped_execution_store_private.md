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

## Adapter boundary and remaining prerequisite

`newScopedExecutionStorePrivate` binds the existing private owner's three admitted
output identities and coordinator-derived selector. Every store operation rechecks
all three borrowed roots and identities; selector substitution fails before root IO.
The existing owner still binds input pins, executable/session/workspace and fixed
policy as provenance only. Files and a new owner cannot reconstruct a lost Pending.

This adapter is NOT live-qualified. It rejects identical output identities and
closed/replaced capabilities, but does not establish ancestor-disjoint topology among
outputs or against input roots. The retained-reader result does not retain live input
capabilities or their actual paths; ContentRetentionRoot exposes identity equality,
not an ancestor relation. A future bounded registration/topology preflight must
establish that relation without a raw-root/FD escape hatch or unsupported same-user
attacker claim. Do not connect a live entrypoint or output retention until then.

## Verification limits

Normal, race and vet checks cover publication and censuslineagepilot, including old
guards. Linux code is cross-compiled on the Darwin host, not runtime-tested here.
Unsupported-platform production packages and publication tests are cross-compiled;
the pilot's existing untagged scoped_execution_claim_review_private_test.go prevents
Windows pilot test compilation because its fixtures are Darwin/Linux-only. That
legacy file is not changed. No new protocol-wide Bar formal coverage is claimed.
