# Bounded actual-target smoke result

Status: `SMOKE_COMPLETED` with `RUN_VERIFICATION_PASSED`.

Scope: one copied first-target fixture (`--max-targets 1`), direction `both`, private worktree `/private/tmp/adr0011-incoming-calls-v1-6yiY4m`. The full 5900-target workload was not executed.

Observed first target:

- target id: `931e3eb151c212c8ceaee1371cb26fb2b0688196b1e15452890c64bb52ff2d9e`
- source: `acquisitionops/executor.go`
- source sha256: `ee9c628822977134baf33dab8bdf9af39629117d96f63afada1c4880ff86a6df`
- target byte range start: `1117`
- symbol byte offset: `1122`
- LSP position: `{line:35, character:5}`
- identifier: `DecodeManifest`

Observed real gopls results:

- incoming: `SUCCEEDED_NON_EMPTY`, `result_count: 2`
- outgoing: `SUCCEEDED_NON_EMPTY`, `result_count: 1`
- ledger verification: `RUN_VERIFICATION_PASSED`
- didOpen cache evidence: incoming sent `didOpen`; outgoing reused same URI/version with `didOpen_sent: false`

Authority: 0. Completeness: UNKNOWN. Production admission: refused/not requested/not granted. Manager custody: not claimed.
