# Mechanical Execution Plan `MIXED-SCOPE-V6-PREPARATION-v1`

## Authorization

The persistent focused guard `TestFreshMixedScopeManagedDocuments` compiles, runs unskipped, reaches `CaptureSnapshots`, and fails the named assertion `ASSERT_NO_FALLBACK_OR_URI_LEAKAGE` because current production submits a valid canonical outside-workspace URI to `PrepareManagedDocuments`.

## Retained properties

1. For the complete nominated URI set `N` and committed workspace `W`, a URI is requested from managed preparation exactly when the existing `eligibleFreshManagedDocumentURI(uri, W)` predicate accepts it.
2. Every ineligible nominated endpoint remains nominated and produces `SOURCE_UNAVAILABLE / EXACT_ENDPOINT_UNAVAILABLE` with zero resolver calls.
3. Managed preparation limits, request membership, returned-supply validation, and completeness account exactly over the eligible preparation set; existing session, generation, position-encoding, digest, and byte-length custody checks remain unchanged.
4. Historical V5 behavior, public schemas, and the prohibition on fallback or broader source authority remain unchanged.

## Write set

- `internal/censuscontinuation/capture.go`
- `internal/censuscontinuation/capture_managed_test.go` only if an assertion correction is mechanically required by the retained properties; no fixture redesign is authorized.

## Procedure

1. Retain the focused RED output and record its digest together with the focused test source digest.
2. In `CaptureSnapshots`, preserve `allURIs` as the sorted, deduplicated complete nomination URI union. Do not alter representative identities, constituent commitments, or V6 nomination construction.
3. Immediately after sorting `allURIs`, derive `eligibleURIs` by retaining only entries accepted by `eligibleFreshManagedDocumentURI(uri, workspace)`. Do not mutate or replace `allURIs`.
4. Use `eligibleURIs` consistently for every managed-preparation boundary:
   - `MaxDocuments` enforcement;
   - `PrepareManagedDocuments` input;
   - requested-URI map sizing and population;
   - returned-supply membership validation;
   - required-supply completeness checks.
5. Preserve all existing returned-document checks for digest, byte length, position encoding, session, generation, duplicate URI, and malformed supply.
6. Preserve later endpoint closure based on `managedDocuments`: eligible endpoints use prepared supply; outside-workspace endpoints have no prepared document, receive no resolver call, and emit the existing typed `SOURCE_UNAVAILABLE / EXACT_ENDPOINT_UNAVAILABLE` result.
7. Add no filesystem read, fallback, alternate workspace root, alternate session or generation, resolver call for an ineligible URI, public field, schema revision, or historical V5 change.

## Focused GREEN gate

Run exactly:

```bash
go test ./internal/censuscontinuation \
  -run '^TestFreshMixedScopeManagedDocuments$' \
  -count=1 -v
```

Require the test to pass while retaining these assertion identities:

- `ASSERT_OUTSIDE_WORKSPACE_ENDPOINT_RETAINED`
- `ASSERT_OUTSIDE_WORKSPACE_SOURCE_UNAVAILABLE`
- `ASSERT_ELIGIBLE_PREPARATION_BEFORE_RESOLVER`
- `ASSERT_OUTSIDE_WORKSPACE_ZERO_RESOLVER`
- `ASSERT_ELIGIBLE_EXACT_CUSTODY`
- `ASSERT_NO_FALLBACK_OR_URI_LEAKAGE`

## Verification widening

After focused GREEN, run sequentially:

1. partition-focused fresh-capture and reconstruction compatibility tests;
2. `go test ./internal/censuscontinuation`;
3. affected MCP contract and compatibility tests;
4. focused race tests for the changed behavior;
5. `go vet` for affected packages;
6. owned-file formatting gate, with unrelated repository-wide formatting reported separately;
7. `git diff --check` and owned-file diff inspection.

Do not run optional packages requiring `github.com/hybridgroup/yzma/pkg/llama`.

## Stop conditions

Stop without broadening the change if any check shows:

- a changed public schema or historical V5 behavior;
- an outside-workspace resolver call;
- a missing outside-workspace nomination;
- an unavailable outcome with the wrong code;
- weakened session/generation/encoding/workspace custody;
- fallback or broader source authority;
- a changed file outside the approved write set.
