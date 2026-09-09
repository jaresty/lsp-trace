# Runtime Final Repair

Base: `c47032f`; reviewed runtime series through `87a5b32` was replayed without conflict.

## Repairs

- Diagnostic handles now contain only package-private semantic fields and a package-private pointer-identity capability issued by `Manager`. Snapshot lookup validates capability identity and the authoritative stored attempt/session/generation/operation tuple. No capability secret bytes are exported or serialized.
- Completed diagnostic operation retention is trimmed both at creation and terminal completion. Active operations remain retained; the oldest completed operation is evicted deterministically. `diagnosticEvictions` is incremented exactly at deletion.
- Readiness cancellation now tears down/closes the readiness-owned transport, joins the reader result, records the shutdown-late event, and only then closes the diagnostic collector and publishes the terminal readiness snapshot.

## Guards and evidence

RED, exact focused guards:

- `ASSERT_RUNTIME_HANDLE_UNFORGEABLE` failed because `DiagnosticSessionHandle.AttemptID` was exported.
- `ASSERT_RUNTIME_HISTORY_CONTINUOUS_BOUND` failed with `order=10000 map=10000` after active operations completed without a later creation.
- Independent review established the missing readiness-reader join; the committed guard uses a blocked `hang` reader and requires an accounted late shutdown event before the closed snapshot.

GREEN:

- Million-operation continuous-bound and exact-eviction stress: `go test ./sessionruntime -run 'Test(DiagnosticHandlesExposeNoConstructibleSemanticFields|DiagnosticCompletedHistoryContinuouslyBounded|ReadinessCancellationJoinsReaderBeforeDiagnosticClose)$' -count=1` — 3 passed.
- Focused normal repeated: `go test -count=3 ./internal/lspwire ./internal/manageddiagnostic ./internal/managedprocess ./sessionruntime -run 'Test(Diagnostic|Readiness|RoundTrip)'` — 102 passed.
- Focused race repeated: `go test -race -count=3 ./sessionruntime -run 'Test(DiagnosticHandlesExposeNoConstructibleSemanticFields|ReadinessCancellationJoinsReaderBeforeDiagnosticClose|DiagnosticOperationSnapshotRequiresExactAttemptAndIsImmutable|ReadinessReturnsClosedAttemptAuthorizedOperation)$'` — 12 passed.
- `go vet ./internal/lspwire ./internal/manageddiagnostic ./internal/managedprocess ./sessionruntime` — passed.
- `go build ./cmd/lsp-trace ./cmd/fake-lsp` — passed.
- `git diff --check` — passed.

Compatibility scope intentionally excludes unavailable external provider dependencies. No network, dependency installation, deploy, product/live-D01, full suite, full race, or CLI sink was exercised. Existing focused runtime, transport, diagnostic, process, JSON-handle exclusion, nil-diagnostics, and command build coverage passed in the commands above.
