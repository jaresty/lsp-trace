# Capacity descriptor guard — child-local Craft plan

Scope: exact dirty worktree /Users/schwa/dev/lsp-trace-variable-v2-TSZxdY, sole writer, user-approved. Source fallback; no exact managed LSP session established in this thread. Parent independent acceptance remains required.

## Properties
- Invalid descriptor fails decode admission, including invalid unused branches.
- Invalid descriptor returns an error and no publication bytes at exported generator entry point.
- Actual descriptor input domain and generated V3 bytes stay unchanged.
- Existing semantic authority, duplicate raw-key gate, DTO, V1/V2/MCP and capacity/accounting remain untouched.

## Steps (implementation/verification completed; acceptance deferred)
1. Record baseline hashes; add runnable regressions reproducing all three supplied descriptor mismatches and root/publication bypass; record RED before implementation.
2. Add a local finite descriptor grammar validator and shared root accessor before interpretation. Disallow unsupported combinations, validate nested rules and branch references, reject cycles. Propagate publication errors through generator and byte-pin test.
3. Run focused GREEN, affected package/race checks, unchanged 2,120 corpus, deterministic publication pin, vet/gofmt/diff. No scale rerun unless scope changes.
4. Record exact changes and evidence in a separate report, preserving prior reports.

## Owned set
- PLAN-capacity-descriptor-guard.md (new)
- docs/capacity-descriptor-guard-report.md (new)
- internal/capacitydiagnostic/shape.go
- internal/capacitydiagnostic/shape_test.go (new)
- internal/capacitydiagnostic/genschema/main.go
- internal/censusresult/capacity_shape_test.go (publication call only)

Actual write set matches the six paths above. No other repository edits performed. Evidence and limits are recorded in docs/capacity-descriptor-guard-report.md. The affected package and race runs passed with 2,354 named passing records each, including the 2,120-case corpus. Parent independent acceptance remains pending. Baseline originals and logs under /tmp/capacity-descriptor-guard-*; current generated schema retained unchanged. All Go calls: GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off. No network/install/provider/acquisition/subagents/stage/commit/push.
