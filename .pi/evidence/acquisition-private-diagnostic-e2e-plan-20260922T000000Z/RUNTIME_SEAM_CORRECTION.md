# Runtime Seam Correction

This correction amends `ACCEPTED_MECHANICAL_EXECUTION_PLAN.md` after implementation-time source inspection disproved its CLI batch-adapter seam. All privacy, custody, taxonomy, public-compatibility, RED/GREEN, and verification requirements remain unchanged.

## Actual production seam

Production MCP uses `cmd/lsp-trace-mcp/census_runtime.go`:

- `censusRuntime.execute`
- `censusRuntime.acquire`
- `censusRuntimeBatchAcquirer.AcquireV5`

`cmd/lsp-trace/census_batch_acquirer.go` is forbidden and out of scope.

## Existing injection capability

`censusRuntimeBatchAcquirer` already owns:

```go
execute func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure)
```

`AcquireV5` defaults nil to `executeCensusPlannedBatch`. The production-neutral correction is to add a corresponding package-private field on `censusRuntime` and forward it when `censusRuntime.acquire` constructs `censusRuntimeBatchAcquirer`.

## Deterministic gateway E2E dependencies

To exercise the real gateway without live gopls:

1. Reuse existing injectable `censusRuntime.admit` for deterministic admitted session/config.
2. Add one package-private `discover` dependency on `censusRuntime`, defaulting nil to `censusRuntimeDiscover`; tests supply a deterministic complete `censusacquisition.Discovery`.
3. Add one package-private batch execute dependency on `censusRuntime`, defaulting nil through the existing acquirer behavior to `executeCensusPlannedBatch`; tests inject a deterministic normalized `operation.Failure`.
4. Keep real `mcp.Server.Serve`, registry resolution, `lsp_trace_v1_execute`, `lsp_trace_v1_census`, `privateCensusMCPBinding`, `censusRuntime.execute`, `censusRuntime.acquire`, `censusacquisition.Core`, and `censusRuntimeBatchAcquirer.AcquireV5`.
5. Do not call `binding.callDirect`, direct/canonical helpers, `Core.Run`, or CLI batch adapter from the main E2E.

The test may construct a minimal nonnil `hostSelectorRuntime`/Manager solely to satisfy production invariants, but must prove no LSP process, RoundTrip, PrepareDocument, or gopls launch occurred.

## Failure propagation ownership

`censusRuntimeBatchAcquirer.AcquireV5` currently returns `operation.NormalizeFailure(failure)` and loses request ordinal when converted to plain `error`. Add a package-private typed acquisition failure in `cmd/lsp-trace-mcp` carrying:

- closed private category derived at the exact branch;
- actual `request.Ordinal`;
- normalized operation failure code/category for acquisition failures only;
- no raw `Err` persistence.

The typed error may wrap the normalized failure for internal control flow, but the recorder must serialize only allowlisted fields. `censusRuntime.acquire` receives the Core error, records one private diagnostic best-effort, then returns the unchanged generic public acquisition failure.

## Corrected write-set delta

Authorized additions/replacements:

- `cmd/lsp-trace-mcp/census_runtime.go` for runtime dependency forwarding and typed failure propagation.
- A focused new `cmd/lsp-trace-mcp/census_gateway_acquisition_failure_test.go` for the real JSON-RPC gateway E2E.
- Existing accepted private recorder/config/mapping files remain authorized.

No changes to `cmd/lsp-trace/census_batch_acquirer.go` are authorized.

## Valid RED

The test sends newline-delimited JSON-RPC `tools/call` to `mcp.Server.Serve`, with `lsp_trace_v1_execute` nesting `lsp_trace_v1_census`. Deterministic admission/discovery produce at least one real batch; injected batch execution returns a normalized operation failure. The guard must observe:

- injected batch execution called exactly once;
- generic public `ACQUISITION_FAILED` through the execute envelope;
- no LSP/process activity;
- failure specifically because the configured private acquisition ledger has no record.

Only that missing-record assertion is the accepted RED.
