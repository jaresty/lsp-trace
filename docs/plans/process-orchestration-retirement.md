# Incremental process-orchestration retirement

Status: first approved unit implemented; independent source review completed; local pure
normal/race and compilation verified. Refreshed-byte acceptance and hosted CI pending.
Baseline: `08e00b587c3448906d585fd48e48fb5acd07492c`.
Approval: retire standalone `private-host-v3 run` first, preserve coverage, and discover
later candidates incrementally. This is not blanket approval to retire other routes.

## Direction and boundaries

Ordinary census with durable state is canonical. Keep managed external LSP processes.
Ordinary census is not a drop-in replacement for historical private scope, reports,
accounting, role separation or producer-death guarantees. Do not replace a genuinely
required forced-termination boundary with cooperative cancellation and call it equivalent.
Keep acquisition, transfer, admission, budget/custody, semantic identity and resume
contracts used by supported consumers—even when their filenames say `private`.
No funding-policy change, CLI admission redesign, qualification-runner activation,
historical artifact rewrite, automatic source-pin refresh, or new orchestration workflow.

## Repeatable bounded iteration

1. Choose one evidence-backed entry boundary. Discover its bounded callers, current
   requirement authority and shared consumers. Use exact-workspace LSP when available;
   distinguish source-only fallback from server-reported calls. No exhaustive upfront audit.
2. Map every affected assertion as retained, moved/replaced, or withdrawn. Add direct-core
   coverage before removing shared assertions. Withdraw only tests that inherently require
   the owner-retired operation; a private name or baseline failure is not sufficient reason.
3. Make one small retirement with a stable diagnostic before effects. Delete only proven
   exclusive code; keep uncertain/shared helpers. Update active recipes, not old artifacts.
4. Obtain independent review of caller evidence, assertion map, safety and test equivalence.
5. Run exact-revision hosted CI and inspect **all nine** test streams plus required jobs and
   postchecks. Count root tests, leaves and rollups separately. Inspect starts without
   terminal events and explicit skips; absence is not pass. Native fixtures run hosted only.
6. Refresh the failure ledger and actual post-change test inventory. Choose the next
   high-impact proven-redundant boundary from the remaining evidence, not a fixed queue order.
   A legitimate retired mechanism test may change; shared failures must be repaired.

Stop when a shared guarantee lacks a replacement assertion, a caller is unaccounted for,
a hard termination requirement remains, source custody is uncertain, or an independent
review disputes equivalence. Do not invent a new workflow to make the retirement possible.

Exit: no unexplained assertion loss; supported tests finish; all nine streams and required
jobs/postchecks green; skips and unfinished nodes explicitly reconciled; actual inventory
and remaining-failure ledger recorded at the tested revision. Aggregate count reductions
are not proof of preserved coverage or fewer defects.

## Rolling ledger

| Iteration | Paths / boundary | Retained or moved | Withdrawn | Evidence/status | Next unknown |
|---|---|---|---|---|---|
| 1 | `cmd/lsp-trace/private_host_product_v3.go`; four existing `private_host_product_*v3_test.go` files; new `private_host_product_retirement_v3_test.go`; paged-inventory skill + contract | `acquire`, transfer validation, direct `OpenTransfer`/`RunRetained`, actual CLI `resume`; budget, claims, pins, paired views, qualification and provisional identity assertions | standalone `run` success orchestration; outer coordinator signal delivery and terminal/report transport for that route | HEAD matched; exact LSP `APPLICABLE_SESSION_NOT_FOUND`, no derivation; source-only seam; diff/format checks clean; compiler-selected initialization audited; Darwin normal/race and Linux test compilation, Windows build, and two pure roots normal/race pass; native execution hosted-only; refreshed acceptance and hosted CI pending | compile/runtime validity of relocated fixtures; source-pin consumers; remaining unrelated failures |

The deleted production code is the switch's exclusive `case "run"` body and its exclusive
`errors`/`os/exec` imports. No shared function or internal package was deleted. Early refusal
precedes request reading and signal setup. The test-only retained-core fixture waits for a
controlled `acquire` child then opens the transfer directly; it is not a replacement CLI.
It tests cooperative core cancellation, not the withdrawn outer OS-signal mechanism.

### Assertion map (old line numbers refer to the baseline)

All files below are under `cmd/lsp-trace/`; owner is the supported operation/core named.

| Baseline test / lines | Assertion inventory | Current destination / disposition |
|---|---|---|
| `private_host_product_v3_test.go`, `TestHostProductCLIProductionAdmissionBeforeClaim`, 119–134 | closed-store refusal; no original claim | same test through `acquire`; its identical setup admission precedes readiness/claim |
| same file, `TestHostProductCLIActualTwentyOneInvocation`, 135–221 | controlled provider/build/setup/readiness; task config/scope/application/revision pins (171–175); submit/read errors; 20 completed, 0 pending, PROVISIONAL, 40 calls, 20 candidates, !Accepted, Authority=0 (203–206); paired machine/human bytes (207–216); original claim (217–219) | `TestHostProductRetainedCoreTwenty`; shared pins/errors now in `runHostProductFixtureCommand`; counts/authority/pairs/claim retained. Single CLI `run` and its outer output transport withdrawn |
| `private_host_product_resume_v3_test.go`, `TestHostProductCLIActualBudgetPauseResume`, 88–139 | initial budget pause 1/19/2; explicit next revision; resumed 20/0/40; no reservation refund; increasing report revision; prior report and claim byte identity; prepared reference continuity | `TestHostProductRetainedCoreBudgetPauseCLIResume`: initial direct core, actual CLI resume unchanged; single-command initial orchestration withdrawn |
| `private_host_product_cancel_v3_test.go`, `TestHostProductCLIHostCancelNoOriginalReplay`, 16–59 | cancellation 1/19/3; removed source; spent original refuses before source read; prior counts/calls; exact claim and prepared reference | `TestHostProductRetainedCoreCancelNoOriginalReplay`: direct core cancellation then actual `acquire` refusal; outer `run` SIGTERM mechanism withdrawn |
| same file, `TestHostProductCLIStaleScopeBeforeClaim`, 60–85 | changed source refuses with STALE_SOURCE_GRANT_REFUSED before claim | unchanged supported `acquire` test |
| `private_host_product_medrepair_v3_test.go`, roots 19–29 / fixture 31–278 | six resume modes: source, corrupt_ledger, stale_tip, bootstrap, transfer, runtime_grant; nineteen prior-publication modes: partial, earlypartial, machine_mismatch, human_mismatch, outside_paths, counts, candidate, ledger, profile, lifecycle, noncanonical, complete, checkpoint, reservation, config, status, gap, ledger_tip, citation_overflow | all cases retained; only initial paused-state construction changed to direct core; actual resume/early failure branches unchanged |
| same fixture, 99–118, 221–275 | overflow prerequisite/progress binding; qualified failed report retaining counts/gaps/candidates; !Accepted/Authority=0/UNKNOWN/UNRESOLVED/PROHIBITED; all prior views and claim immutable; new paired views; ledger tip/generation/calls retained; previous-not-current validation; local-consistency-vs-unknown qualification and admitted paths exact | assertions retained unchanged; supported retained admission/publication owners |
| new retirement tests | six run argument variants: missing argument/path, malformed file, directory, empty path, extra argument; exact status/diagnostic; empty stdout, no stdin reads, unchanged fixture modes/contents/names; other dispatcher refusal paths | new `TestHostProductRetiredRun` and `TestHostProductRetirementDoesNotReplaceOtherDispatch`; pure filesystem fixtures, no provider or subprocess. Snapshot is not a global syscall/process observer; effect exclusion also rests on the inspected early return |

The closed-store test has a pre-existing limitation: it does not mark the mailbox ready or
assert the exact refusal reason, so readiness short-circuiting could also satisfy it. Source
ordering, not this test alone, establishes closed-store-first admission. Legacy proof labels
`ASSERT_ACTUAL_CLI_TWENTY_PRODUCT` and `ASSERT_ACTUAL_CLI_PAUSE` now name initial direct-core
assertions; they are not evidence of single-command CLI equivalence.

No test was skipped or deleted because it was private. Historical source-pinned qualification
must identify withdrawn route coverage explicitly; do not refresh pins blindly. The two new pure roots passed normally and under race (six named subtests plus two root
results each run); these are not passes for the relocated native fixtures. Native relocated
fixtures still require hosted execution.

## Discoverable candidate queue (not a deletion order)

- Standalone V3: this first unit; shared helpers and retained operations remain.
- Cohort: inspect legacy `run`, test consumers and shared application machinery before disposition.
- Application / start-source: separate legacy acquisition orchestration from newer inventory
  operations and Python foreground startup/ready contracts; no whole-file retirement.
- Program C: prefer in-process computation by default in its future unit. Independently
  identify and justify any still-supported hard-termination requirement and seek explicit
  disposition; historical process isolation is not a permanent exemption. This unit leaves
  Program C unchanged.

Refresh this queue after each review/CI iteration; it is neither complete nor authorization.

## Current CI baseline, not a closure claim

Retained report: `/tmp/lsp-pipe-backup-052c6ef/REPORT.md`, run `46113068363`, exact baseline HEAD.
Its nine-stream ledger reports 49 failed test nodes (41 leaves + 8 rollups), eight package
failures and four timeout observations. Four started-without-terminal nodes require accounting;
Darwin descriptor/bridge and owner-start evidence was interrupted, not green.
The predecessor source analysis attributes at least 17 failure nodes to private **census**
fixtures; these are distinct from standalone `private-host-v3 run`. That attribution does not
authorize their deletion. This unit proves zero baseline CI failures fixed. Reconcile the exact
failure ledger, unfinished nodes, skips, source-pin failures and postchecks after hosted execution.
