# FR23 final repair ledger

## Derivation

Scope is limited to the three findings in `REPAIR-INDEPENDENT-REVIEW.md` and the LOW test/provenance findings in `FINAL-INDEPENDENT-REVIEW.md`. The reviewed executable repair chain is `6d9dd5e → 677c6dc → 8071c62 → ae00a42 → b8597f3 → 9de2f66 → 9271525 → 02c6c36`; `9de2f66` precedes `9271525`. Its required feature baseline is `92a3f27 → 2ce91f6`, rather than the alternate `5e757c8 → 03a75ca` lineage.

1. **Startup attempt identity:** the returned identifier must be a manager-owned, fixed-format one-way derivation. Production now hashes an explicit domain separator, a random manager nonce, a strictly monotonic sequence, and optional package-private test entropy. No exported production seam accepts caller preimage bytes. Randomness failure is observable from `New`; sequence wrap fails closed. This makes no authentication or cross-process durability claim.
2. **Managed diagnostic validation:** a declarative table owns the implemented phase/terminal combinations, substep applicability, matched-response IO minima, readiness/document/capability IO shapes, requested/effective limits, and process applicability. Explicit early partial records remain valid only in allowed phase/terminal classes. The table does not claim unimplemented D01 coverage.
3. **Retained byte bounds:** both stores validate before cloning, enforce a per-string cap, and use overflow-safe addition of every actual retained string byte plus fixed scalar overhead. No per-element string estimate remains.

## RED

Commit `677c6dc` persisted reviewer guards before production edits. The focused run reached assertion-specific failures:

- `ASSERT_FR23_ATTEMPT_ID_PREIMAGE_WITHHELD`
- `ASSERT_FR23_MATRIX_*` and `ASSERT_FR23_REQUEST_MATCHED_IO_*`
- `ASSERT_FR23_DIAGNOSTIC_ACTUAL_STRING_BYTES`

The same run retained passing controls for legitimate partial records and invalid-before-store rejection.

## GREEN

Atomic production commits:

- `8071c62` — manager-owned hashed startup attempt IDs and package-private deterministic fixtures.
- `ae00a42` — declarative closed managed-diagnostic matrix.
- `b8597f3` — actual retained-string byte accounting with overflow/per-string guards.

Focused GREEN observations before final verification:

- identity/startup guards: 14 passed;
- matrix/runtime guards: 30 passed;
- both-store sizing/startup guards: 22 passed.

Final bounded verification (no full suite and no D01 run):

- `go test ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1` — 202 passed;
- `go test -race ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1` — 202 passed;
- focused `go vet` — clean;
- focused `go build` — clean.

A race-only ordering failure in the persisted readiness-completion guard exposed that the waiter could return before its diagnostic was retained. Commit `9271525` performs the minimal in-scope ordering correction; the assertion-specific race guard then passed 20 consecutive applications before the final race run.

## LOW test/provenance repair

The provider deadline helper now writes its PID to a sibling `0600` temporary file, closes it, and renames it over the final marker in the same directory. Test cleanup removes both names. The polling reader treats missing, empty, and temporarily unparsable marker content as transient until the existing three-second setup deadline; no timeout was increased.

Executed after the test-only repair:

- `go test -race ./internal/provider -run '^(TestDeadlineAlreadyExited|TestDeadlineCompletion|TestCancellationAndTimeoutAreDistinct|TestEveryReturnReapsChild)$' -count=20` — 280 passed in 1 package;
- `go test ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1` — 202 passed in 3 packages;
- `go test -race ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1` — 202 passed in 3 packages.
