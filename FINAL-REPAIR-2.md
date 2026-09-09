# TRANSPORT-CORE final repair 2

## Scope

Repaired the two blockers in `/tmp/lsp-trace-fr23-request-diagnostics/TRANSPORT-CORE-FINAL-REVIEW.md` on top of `c8c73f4`. No delegation, network, install, deploy, product/live-D01, runtime behavior, CLI behavior, full suite, or full race was exercised.

## Repairs

1. **Executable identity:** `ObserveIdentity` now rejects an executable leaf symlink before opening. For a regular supplied path it records the original `lstat`, opens and hashes the descriptor, canonicalizes, then rechecks the supplied path before and after canonical-target validation. Every observed regular entry is compared with the opened descriptor by file identity, size, and modification time. A supplied-path retarget after canonicalization returns `IdentityRaced`; unavailable initial validation remains fail-closed as `IdentityUnavailable`.
2. **Pending public compatibility:** `Pending.Accept` again uses the historical exported disposition rule for inactive IDs: an absent generation with non-empty retained generation history is `ResponseWrongGeneration`; empty history or a retained generation is `ResponseUnknown`. Bounded generation retention remains unchanged.
3. **Private observer metadata:** pending late/unknown detail is stored only in unexported event state. It is additive for same-package diagnostics and does not alter the exported `ResponseDisposition` returned by `Accept` or reported in `Event.Disposition`.

## Frozen and counterfactual guards

- `TestObserveIdentityRejectsExecutableSymlink` rejects a leaf executable symlink with `IdentityUnavailable`.
- `TestObserveIdentityOriginalPathSymlinkRetargetRace` uses the canonicalization seam as a deterministic barrier, replacing the original regular path with a symlink to another executable after canonical resolution; the result must be `IdentityRaced`.
- `TestPendingAcceptHistoricalPublicDisposition` freezes the base behavior for empty history, known-generation unknown IDs, and never-seen generations.
- `TestPendingObserverDetailDoesNotChangePublicDisposition` verifies private observer detail while independently checking exact public dispositions.
- The prior eviction test now expects the historical `ResponseWrongGeneration` disposition rather than freezing the regressed behavior.

Observed RED results before production repair:

- `ASSERT_PROCESS_IDENTITY_EXECUTABLE_SYMLINK_REJECTED: 1`
- `ASSERT_PROCESS_IDENTITY_ORIGINAL_PATH_RETARGET_RACE: 0`
- `ASSERT_LSPWIRE_PENDING_NEVER_SEEN_WRONG_GENERATION: 1`
- `ASSERT_LSPWIRE_PENDING_OBSERVER_PUBLIC_DISPOSITION: got=1 want=3`

## Focused verification

- Focused identity guards: `Go test: 4 passed in 1 packages`.
- Focused pending compatibility/observer guards: `Go test: 5 passed in 1 packages`.
- Complete focused package normal: `Go test: 82 passed in 3 packages`.
- Repeated focused normal (`-count=10`): `Go test: 820 passed in 3 packages`.
- Repeated focused race (`-race -count=3`): `Go test: 246 passed in 3 packages`.
- `go vet ./internal/lspwire ./internal/manageddiagnostic ./internal/managedprocess`: passed.
- `go build ./cmd/lsp-trace ./cmd/fake-lsp`: passed.
- `git diff --check`: passed.
