# Phase19 diagnostic follow-up — capacity refusal V3

## Disposition

Diagnostic-only implementation in the dirty `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY` worktree. Sole writer; no staging/commit/push. No admission-budget, quote, response-size, item-count, accounting, deadline, partitioning or job-policy change. Application-scale design remains separate.

Start now returns a bounded typed capacity error at its existing preadmission guard, after preparation, saved state and delivery construction, but before durable head CAS/acceptance publication. It remains `errors.Is(err, ErrDurableInventory)` compatible. Quote validity precedes bytes, work and packets; equality is admitted. Impossible spent/work counters fail safely instead of unsigned subtraction wrapping. Invalid quotes omit metrics; insufficient dimensions contain only unit/spent/protected/limit/available/deficit. Work metrics are work units (32 per quoted byte); packets at this Start guard have spent=0. No full head, path, environment, content, actor identity, task or run locator is serialized.

Normal-start operation diagnostics use new closed `lsp-trace.census-operation-diagnostic.v3`, operation=start, action=STOP, retry=false. Generic failures and legacy V1/V2 contracts remain unchanged. No repeated acquisition or automatic retry permission follows. Same-user trust assumptions remain; no PKI.

## Contract validation boundary

V3 is a separate DTO/codec; V1 and V2 schemas/enums were not widened. The leaf `internal/capacitydiagnostic` package avoids the aggregate censusresult/schema import cycle found by the first focused compile. censusresult exposes aliases/wrappers.

The standalone Draft 2020-12 schema validates closed shape, enum and numeric bounds. Full contract validation is the **composed** `schema.ValidateFor(..., schema.FamilyCensusCapacityDiagnostic, "v3")` or the strict codec. Arithmetic consistency and raw duplicate/trailing JSON rejection are semantic/wire checks, not capabilities claimed for plain JSON Schema. Parity tests exercise both full entry points with valid byte/work/packet/invalid-quote cases and missing/null/unknown/duplicate/trailing/enumeration/arithmetic/unsafe-number mutations.

## Executed controlled evidence

All Go commands used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off`. No live provider/model/network or installs. Existing `lsp_trace_controlled_describe_fixture` only; `census_evidence_scale_test.go` and its fixed 0/256/1024 matrix are unchanged.

`/tmp/capacity-refusal-v3-scale-final.log`:
- Exact 1024-padding test: CAPACITY_BYTES, spent 14,825,236, protected 54,463,091, limit 67,108,864, available 52,283,628, deficit 2,179,463 bytes. Preparation/original checks passed; no durable task/head published.
- Unchanged matrix: padding 0 and 256 PUBLISHED_VERIFIED; padding 1024 PRETASK_REFUSAL, CAPACITY_BYTES, spent 14,825,180, protected 54,463,091, limit 67,108,864, available 52,283,684, deficit 2,179,407 bytes. Total required 69,288,271 bytes. No durable head.
- This is numeric refusal verification, **not capacity success**.

The supplied external observation (spent 14,825,416; protected 54,463,091; total 69,288,507; deficit 2,179,643) was not reproduced byte-for-byte. Its exact arithmetic is exercised by the scalar guard test `supplied_exact_scalar_observation`, not misrepresented as fresh scale measurement. An initial over-specific golden assertion failed; another repeat also varied protection to 54,461,595. Fixture temporary paths are not fixed, and exact root-cause attribution of every byte variation was not investigated. The final fixture test checks actual strict numeric consistency, exact reason, unchanged fixed limit, refusal and absence of head, rather than tuning fixtures to force previous totals.

## Verification ledger

- `TestStartCapacityRefusalBoundaries`: byte/work/packet equality and one-over; invalid quote; first-failure precedence; hostile uint64 quote/spent/work; exact supplied scalar observation. Calls the same guard used by Start. Synthetic work-boundary cases isolate work classification; they do not assert that work binds before bytes on ordinary valid Start accounting.
- `TestCapacityDiagnosticParity`: closed codec/composed-schema positive and negative cases.
- `TestCensusCapacityDiagnosticCarrier`: errors.Join preserves typing, raw cause remains undisclosed; generic route unchanged.
- `TestCensusCapacityExact1024` and unchanged `TestCensusEvidenceSizeMatrix`: passed, log above.
- Counterfactual reduction: replacing the actual Start return with generic ErrDurableInventory produced V1 RUN_UNAVAILABLE and `TestCensusCapacityExact1024` failed at the diagnostic assertion. `/tmp/capacity-refusal-v3-red.log`. Typed return restored; final scale suite passed. Earlier golden-total failures are fixture-test corrections, not claimed RED evidence for the implementation.
- Focused core/censusresult: final command reported 34 passed in 2 packages.
- `/tmp/capacity-refusal-v3-unit-final.log`: focused Start/capacity/completion/durable/legacy tests passed; internal/schema had no directly matching tests in that invocation. Schema parity is actually exercised from censusresult's external test package.
- `/tmp/capacity-refusal-v3-cli-final.log`: focused CLI diagnostic/stored-version tests passed.
- `/tmp/capacity-refusal-v3-race.log`: focused core, codec, carrier and legacy diagnostic race tests passed.
- `/tmp/capacity-refusal-v3-vet.log`: vet passed for applicationinventory, capacitydiagnostic, censusresult, schema and cmd/lsp-trace.
- `/tmp/capacity-refusal-v3-legacy.log`: legacy diagnostics, exact schema handler behavior and MCP successor-schema tests passed.
- gofmt on owned Go files and git diff --check passed. No broad suites, cross-build or production qualification claimed.

## Exact owned changes

Existing files (only narrow additions/replacements owned; other dirty content is not this execution's work):
- `internal/applicationinventory/durable.go`: replace generic Start guard block with typed guard call at the same position.
- `cmd/lsp-trace/census_runner.go`: typed capacity dispatch inside the existing operation-diagnostic branch.
- `internal/schema/schema.go`: V3 family registration (three lines).
- `internal/schema/validation_core.go`: leaf import and semantic validator dispatch.
- `docs/census-durable-inventory.md`: V3 semantics, validation boundaries and bounded follow-up observations.

New files:
- `PLAN-capacity-refusal-v3.md`
- `internal/applicationinventory/durable_capacity_refusal.go`
- `internal/applicationinventory/durable_capacity_refusal_test.go`
- `internal/capacitydiagnostic/diagnostic.go`
- `internal/censusresult/capacity_diagnostic.go`
- `internal/censusresult/capacity_diagnostic_test.go`
- `internal/schema/schemas/lsp-trace.census-operation-diagnostic.v3.schema.json`
- `cmd/lsp-trace/census_capacity_diagnostic.go`
- `cmd/lsp-trace/census_capacity_diagnostic_other.go`
- `cmd/lsp-trace/census_capacity_diagnostic_test.go`
- `cmd/lsp-trace/census_capacity_scale_test.go`
- `docs/capacity-refusal-v3-report.md`

## Evidence/process limits

Fresh local Craft/PLAN preceded mutation. Exact-worktree managed session derived READY generation 1; structural projection failed at client schema validation (position/source unknown, symbol/projection missing). Implementation seam confirmation therefore used explicitly degraded filesystem source, not claimed server-reported CALLS or projected source evidence. No predecessor's substantive conclusions were adopted as verified transcript evidence; the provided external metrics remained an input to reproduce/check.

The tests qualify bounded controlled execution and numeric explanations only. Full arbitrary-scale behavior, live model quality, independent actors, physical containment and hostile same-user replacement are not established. Standalone JSON Schema alone is not semantic parity. No claim of exhaustive per-assertion perturbation coverage is made.
