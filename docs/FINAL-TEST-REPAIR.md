# FR23 final test repair

## Scope

This follow-up fixes only the LOW verification-harness and provenance findings from `/tmp/lsp-trace-fr23/FINAL-INDEPENDENT-REVIEW.md`. It changes no production behavior, increases no timeout, runs no full suite or D01 path, and makes no public, deployment, install, network, or product-repository claim.

## Exact provenance

The independently reviewed executable repair chain is:

`6d9dd5e → 677c6dc → 8071c62 → ae00a42 → b8597f3 → 9de2f66 → 9271525 → 02c6c36`

The required feature baseline immediately before that repair chain is `92a3f27 → 2ce91f6`. The alternate `5e757c8 → 03a75ca` lineage was not substituted. In particular, `9de2f66` precedes `9271525`.

The source commits were cherry-picked onto this worktree in actual parent order. Their local rewritten commits through the reviewed endpoint are:

`6b8a289 → 94dc4d2 → acfbe2d → 3edb607 → db16fb1 → 5272c0c → 7a8cee2 → 5d0e39e → 72b868b → ea13b9e`

## Test-only repair

`internal/provider/runtime_deadline_test.go` now publishes the helper PID by creating/truncating a sibling temporary file with mode `0600`, writing the complete PID, closing the file, and renaming it over the final marker in the same directory. Cleanup removes both the final and temporary names. The polling reader ignores missing, empty, and temporarily unparsable content until its existing three-second setup deadline.

## Executed results

- Selected deadline/cancellation/reap race guard:
  `go test -race ./internal/provider -run '^(TestDeadlineAlreadyExited|TestDeadlineCompletion|TestCancellationAndTimeoutAreDistinct|TestEveryReturnReapsChild)$' -count=20`
  — **280 passed in 1 package**.
- Focused FR23 normal run:
  `go test ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1`
  — **202 passed in 3 packages**.
- Focused FR23 race run:
  `go test -race ./internal/manageddiagnostic ./sessionruntime ./internal/provider -count=1`
  — **202 passed in 3 packages**.

These are bounded internal-chain results only.
