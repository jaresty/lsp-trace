# Accepted Mechanical Execution Plan: Private Census Acquisition Diagnostic E2E

This plan supersedes the unresolved-decision section of `MECHANICAL_EXECUTION_PLAN.md` (SHA-256 `e9b6c42c1703cf7c974a69eb3f2a8d0cc3d94d5ff3240a1238d91a65b7016b7b`) while retaining its RED/GREEN assertions, perturbation matrix, privacy constraints, and verification tiers.

## Frozen decisions

1. **Separate record contract.** Acquisition diagnostics use a new private record schema and recorder. They must not extend or mix with `ManagedPreparationDiagnosticRecorder`. Custody mechanics may be extracted/reused only without changing managed-preparation record bytes or strict parsing.
2. **Custody.** Require a canonical absolute clean file path; existing non-symlink parent directory exact mode `0700`; regular ledger file exact mode `0600`; bounded record/line/total bytes and record count; strict complete NDJSON; no mutation on corruption or custody drift; serialized append; append followed by `Sync()`. Recorder failure is secondary and cannot alter the primary census result.
3. **Fingerprint.** Persist only existing `censusrequest.Receipt.Fingerprint`. Do not recompute identity from or retain `Receipt.Semantic`, `CanonicalJSON`, operation request bytes, sources, includes, excludes, workspace, URI, or path.
4. **Closed taxonomy.** Map the existing exhaustive batch constants one-to-one:
   - `CENSUS_BATCH_INVALID_INPUT` → `INVALID_INPUT`
   - `CENSUS_BATCH_SESSION_DRIFT` → `SESSION_DRIFT`
   - `CENSUS_BATCH_CANCELLED` → `CANCELLED`
   - `CENSUS_BATCH_ACQUISITION_FAILED` → `ACQUISITION`
   - `CENSUS_BATCH_ADMISSION_FAILED` → `ADMISSION`
   Unknown codes are invalid and must not silently map to `ACQUISITION`.
5. **Ordinal truth.** Batch failures carry `BatchRequest.Ordinal` in `censusBatchFailure`. Failures before planning/batch assignment omit ordinal; never synthesize zero. Admission-time session drift/cancellation therefore has no ordinal; per-batch drift/cancellation preserves its real ordinal.
6. **Normalized operation evidence.** For `ACQUISITION`, retain only the normalized `operation.Failure.Code` and an allowlisted stable category derived from existing failure semantics. Never persist `Err`, raw diagnostic text, raw LSP payload, or request ID.
7. **Public compatibility.** Public output remains the existing generic acquisition diagnostic. Construct expected canonical bytes with `censusresult.NewDiagnostic(StageAcquisition, ordinal)` and `censusresult.MarshalDiagnostic`; pin exact bytes/digest. Private recording is additive and cannot alter the public schema or bytes.
8. **Production dispatch E2E.** Add the main guard in `cmd/lsp-trace-mcp`, preferably `census_candidate_qualification_test.go` or a narrowly named adjacent test. Enter through real MCP `tools/call` for `lsp_trace_v1_execute` nesting `lsp_trace_v1_census`; do not call direct/canonical binding helpers, `censusRuntime.execute`, or `Core.Run` as the E2E assertion path.
9. **Deterministic injection.** Inject failure through the existing package-private initialized census batch session/runtime dependency beneath production dispatch. Do not replace the census binding and do not crash live gopls.
10. **Temporary custody.** E2E owns a dedicated `t.TempDir()` child directory explicitly mode `0700` and ledger file mode `0600`; Go test cleanup owns deletion.
11. **No side effects.** Injected acquisition failure creates no base/continuation publication, trace, managed-preparation diagnostic, worker, or model artifact.

## Authorized maximum write set

- New package `internal/censusdiagnostic/` for private record validation, strict readback, and custody-safe recorder, with focused tests.
- `cmd/lsp-trace/census_batch_acquirer.go` and focused tests for typed code/ordinal propagation.
- `cmd/lsp-trace-mcp/bootstrap.go` for a distinct optional acquisition-diagnostic path.
- `cmd/lsp-trace-mcp/census_runtime.go`, `census_completion.go`, and the exact production-construction file required to inject/attempt the recorder.
- `cmd/lsp-trace-mcp/census_candidate_qualification_test.go` or one adjacent narrowly scoped E2E test file.
- Narrow focused tests for mapping, recorder-secondary behavior, public byte compatibility, and configuration custody.

Public schemas, `internal/mcp/transport.go`, continuation schemas, managed-preparation record schemas, and worker/model code are forbidden unless the writer stops and escalates with a named RED proving necessity.

## Required RED

Before production implementation, the production-shaped E2E must compile, enter real gateway dispatch, reach the deterministic batch acquisition failure, produce the existing generic public `ACQUISITION_FAILED`, and fail specifically because no private acquisition diagnostic exists. Compile failures, fixture failures, direct-Core tests, live-server failures, and failures before gateway dispatch are invalid REDs.

Additional focused REDs cover all five mappings, honest ordinal omission/preservation, prohibited-field absence, strict unknown/duplicate/trailing/corrupt/overflow rejection, custody drift, append bounds, `Sync()` failure, and recorder-secondary behavior.

## GREEN and verification

1. Main E2E GREEN with one private record and unchanged public bytes.
2. `internal/censusdiagnostic` focused package tests.
3. Census batch adapter and MCP census focused tests.
4. Affected package suites.
5. Focused race tests for recorder/concurrent append and E2E path.
6. Affected-package vet.
7. Owned-file gofmt, `git diff --check`, and write-set review.
8. One supported broader suite excluding optional yzma packages.

Stop on public byte/schema drift, privacy leakage, fabricated ordinal, unknown-code collapse, managed-preparation schema changes, recorder error replacing the primary failure, non-deterministic/live-server dependence, or any unapproved file.
