# ADR0011 cumulative complete-frame portability (P0)

Scope: packaging the accepted successful-complete-frame increment only. Not full C,
partial/failed-byte charging, D, public admission, or producer authentication.

## Identities and discovery

`manifest.json` is the exact original (SHA-256
`b44515d1f806842069824b1c8e7039edc9e080a020a64dcbfc7352acdeaad61e`).
`relocation.json` is a distinct new identity binding that manifest, all twelve
frames in order, their lengths/hashes/aggregates, and the inert historical source.
The active loader still checks the original manifest hash and all original pins;
its only path change is to this package-local directory. The relocation metadata
is a provenance binding, not a replacement validation policy.

The historical RED source is byte-identical at
`historical/c_cumulative_complete_frame_test.go.txt`, SHA-256
`e6ef991bd2819325dbf411ddd23e08f3dd49e9b784255ab46ad83be632ff18af`.
It is not a Go compilation unit. Its helper declarations/bodies are extracted into
`../../c_cumulative_fixture_helpers_test.go`; the historical test function is not
active. The successor's assertions are unchanged. Existing A/B testdata and
per-frame originals are reused without edits.

## Bounded reproduction

Supported verification environment: macOS darwin/arm64, Go 1.26.5, Git, Python 3.
Use a local Go installation and preinstalled module cache; set `GOPROXY=off`,
`GOSUMDB=off`, `GOTOOLCHAIN=local`. Missing prerequisites are BLOCKED_NOT_RED,
not a skipped pass. No network, notebook, developer-specific absolute path, or
ignored evidence is needed for these named tests. Run from repository root:

```sh
go test -json -count=1 -timeout=120s ./internal/adr0011methodresult ./internal/lspwire -run '^(TestCCumulativeCompleteOriginalFrameSuccessor|TestCManagerCompleteOriginalFrameBoundary|TestCObservedB4SuccessorEquivalenceAndOrder|TestCFailedManagerReadSkipsSuccessorAndPrivatePublisher|TestCPrivatePublisherNotificationPanicNeutral|TestCumulativeDisabledCaptureBeforeBodyObserver|TestCumulativeFailedReadsDoNotCharge)$'
go test -list . ./internal/adr0011methodresult ./internal/lspwire
go test -json -count=1 -timeout=180s ./internal/adr0011methodresult ./internal/lspwire
```

Check that all seven named top-level tests run, with no skipped selected tests;
the cumulative successor must run independent A/B controls, at-cap and plus-one.
The listing must omit `TestCCumulativeCompleteOriginalFrameBoundary` exactly
(not its successor). Preserve full output and exit status. Default-package
failures from unrelated historical tests must be reported, not repaired or
reclassified as portability success.

The P0 execution packet records a clean local clone of actual HEAD plus an
explicitly enumerated, hash-frozen **candidate overlay**: accepted runtime bytes,
active successor/helper/compatibility tests, and this fixture tree. No ignored
evidence is copied into that checkout. That proves candidate-overlay reproduction,
NOT a committed tracked-only revision. Source files here are ordinary addable
files; no force-add or ignore exception is required. Nothing is staged/committed
by P0. The literal committed fresh-checkout gate remains pending commit
authorization and a later committed-revision run.

## Historical RED replay: explicit disposable procedure only

Do not reactivate the archive in this checkout or default tests. P0 does not claim
a fresh historical failure reproduction. For a separately authorized replay:

1. Create a disposable checkout of the independently established historical RED
   revision/runtime bundle. Verify BOTH historical runtime hashes against that
   historical execution's provenance; current accepted GREEN pins and successor
   RED pins do not establish historical-test runtime identity. If those bindings
   are unavailable, stop BLOCKED_NOT_RED. Do not edit runtime to manufacture RED.
2. Verify the archive hash above, then copy it byte-for-byte into that disposable
   package as `c_cumulative_complete_frame_test.go`. Exclude the newly extracted
   helper and successor there to avoid duplicate helper declarations.
3. Copy these exact twelve frames and original manifest into the historical
   loader's expected relative `.pi/evidence/adr0011-c-cumulative-wire-v1` directory
   in that disposable checkout only. Verify every pin. Keep the archive unedited.
4. Explicitly run only `go test -json -count=1 -timeout=120s
   ./internal/adr0011methodresult -run '^TestCCumulativeCompleteOriginalFrameBoundary$'`.
   Retain full logs, exit code, source/runtime/input hashes. Require controls and
   complete delivery before attributing `SEMANTIC_RED_CUMULATIVE_PLUS_ONE`.
   Fixture/time/setup failures are BLOCKED_NOT_RED, not semantic RED.
5. Remove the disposable replay tree. Never merge its historical runtime or
   active historical test back into the candidate/default package.
