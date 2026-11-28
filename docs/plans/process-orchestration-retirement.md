# Incremental process-orchestration retirement

Status: first approved unit VERIFIED at `55681170cfa5d3ef04215e48faa763801b48efb6`
by bounded hosted run `46135449970`. Second approved unit (Program C self-reexec)
is implemented for independent review; its hosted CI and source-pin disposition remain pending.
First-unit baseline: `08e00b587c3448906d585fd48e48fb5acd07492c`.
Second-unit baseline: `55681170cfa5d3ef04215e48faa763801b48efb6`.
Approval: retire standalone `private-host-v3 run`, then explicitly approved Program C
in-process consolidation; preserve coverage and discover later candidates incrementally.
This is not blanket approval to retire other routes.

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
| 1 | `cmd/lsp-trace/private_host_product_v3.go`; four existing `private_host_product_*v3_test.go` files; new `private_host_product_retirement_v3_test.go`; paged-inventory skill + contract | `acquire`, transfer validation, direct `OpenTransfer`/`RunRetained`, actual CLI `resume`; budget, claims, pins, paired views, qualification and provisional identity assertions | standalone `run` success orchestration; outer coordinator signal delivery and terminal/report transport for that route | HEAD matched; exact LSP `APPLICABLE_SESSION_NOT_FOUND`, no derivation; source-only seam; diff/format checks clean; compiler-selected initialization audited; Darwin normal/race and Linux test compilation, Windows build, and two pure roots normal/race pass; native execution hosted-only; bounded hosted closure at `55681170`, run `46135449970`: Darwin 40 PASS + one controlled-child SKIP; Linux pure selection eight PASS nodes; nine streams complete, 593 invocations, no missing/unfinished tests or timeout observations | remaining unrelated failures and source-pin consumers; this is not overall CI closure |
| 2 | Program C typed-bundle execution, owner connection, both hidden main dispatches; shared validator/input bound retained | canonical import with independent expectation, exact execution/full replay, detached bytes, cooperative cancellation, one-shot owner/custody/accounting; portable assertion replacements | self-reexec worker, process-group kill/reap/RSS sampling, outer worker wire protocol and unsupported-supervisor gating | bounded normal/race selection and compile-only platform matrix; assertion map and mutation witnesses retained externally; independent review/hosted CI pending | source-pinned historical qualifications require explicit review, not blind refresh; no baseline failure key proven fixed |

In iteration 1, the deleted production code is the switch's exclusive `case "run"` body and its exclusive
`errors`/`os/exec` imports. No shared function or internal package was deleted. Early refusal
precedes request reading and signal setup. The test-only retained-core fixture waits for a
controlled `acquire` child then opens the transfer directly; it is not a replacement CLI.
It tests cooperative core cancellation, not the withdrawn outer OS-signal mechanism.

### First-unit assertion map (old line numbers refer to its baseline)

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
must identify withdrawn route coverage explicitly; do not refresh pins blindly. At the initial
first-unit local checkpoint, the two new pure roots passed normally and under race (six named
subtests plus two root results each run); those runs did not execute relocated native fixtures.
Their subsequent bounded hosted coverage is recorded in the first-unit ledger above.

## Discoverable candidate queue (not a deletion order)

- Standalone V3: bounded first-unit hosted closure verified; shared helpers remain.
- Program C: current approved second unit, in-process by default; finish independent review
  and exact-revision hosted verification before accepting this candidate. No supported
  hard-termination requirement was established by the bounded current-source lookup.
- Cohort: next bounded discovery candidate; inspect legacy `run`, test consumers and shared
  application machinery before disposition. No retirement is authorized by queue position.
- Application / start-source: subsequent bounded discovery; separate legacy acquisition
  orchestration from newer inventory operations and Python foreground startup/ready contracts.
  No whole-file retirement or all-at-once orchestration removal.

Refresh this queue after each review/CI iteration; it is neither complete nor authorization.

## Current CI baseline, not a closure claim

Latest parent-verified retained report: `/tmp/lsp-cap4-backup-7cad769/REPORT.md`,
run `46135449970`, revision `55681170`. All nine artifacts completed: 593 invocations,
no missing/unfinished tests or timeout observations. Overall remaining failures are 38 nodes
(34 leaves + four rollups), with 11 package-failure invocation events spanning four distinct
shard/package keys (not 11 distinct logical packages). Batched inventory is 2,329 roots;
the prior 2,337 was an arithmetic error, not lost coverage.
These prior receipts were supplied as the accepted first-slice baseline, not re-certified by
this Program C edit. No exact Program-C-to-remaining-failure-key mapping is claimed, and this
unit proves zero baseline CI fixes. Hosted execution must reconcile remaining failures,
source-pin failures, skips and postchecks; a smaller test count is not evidence of green CI.

## Program C second-unit contract and review packet

The new `internal/programc/typed_bundle_execution_private.go` entry executes synchronously:
independent opaque expectation → detached candidate import → existing `ExecuteReport` →
**full `CheckReport` replay** → detached output → final context check → release. Failures
return nil bytes. Context checks also precede import and separate stages. There is no timeout
goroutine, arbitrary caller-supplied runner, replacement algorithm, or fabricated process evidence.
`ExecutionObservation` reports only elapsed time and actual completed Imported/Executed/Checked
stages. These fields are not authorization or physical resource measurements.

`community.Leiden` in `partitionMath` has no context parameter; full replay repeats real work.
Cancellation suppresses output after synchronous work returns, not while an uncooperative call
is running. Hard wall-time, sampled RSS, forced termination, process-group cleanup/reap, and
worker transport guarantees are explicitly withdrawn. Tests request cancellation just inside
execution/replay stages after the preceding context check, run the fixed synchronous stage,
and assert no output release afterward; they do not claim midpoint interruption.

The private owner retains candidate copy/hash, consume-before-check, selected-head revalidation,
sticky uncertainty, no refunds, and confirmed `observe` before release. Its exact 14-coordinate
charge vector is unchanged. Obsolete transport, observation-method and cleanup allowances are
reserved **logical credits**, not measured process work or physical/wall guarantees. No funding,
production-startup, Linux admission, source/cohort integration or policy change is included.

`validCanonicalCommunities` moved byte-for-byte to `core.go`; graph algorithm bodies are unchanged.
The shared exported `MaxWorkerInputBytes int64 = 64 << 20` moved there unchanged. Its historical
name denotes the decompressed-input admission bound used by the existing in-process
`censuscontinuation/candidate_group.go:603–604` consumer, not a surviving worker. That consumer
was not edited. Both command mains lost only hidden worker dispatch and the unused import.

### Assertion migration and evidence limits

The six original test files contained 58 roots. Before deletion, the external map enumerated
273 assertion sites rather than only roots. The completed map additionally addresses 10 assertions
in the removed test-only observer bridge and correctly names retained helper-function ownership:
`/tmp/lsp-programc-retirement-55681170/coverage-map.md` (283 sites, seven files).
Original byte backups are retained alongside it. Current affected inventory is 26 roots:
17 Program C and nine inventory-owner roots. This numerical reduction is not coverage evidence.

| Original assertion family | Retained destination | Withdrawn clauses only |
|---|---|---|
| supervisor exact outcome | `TestExecutionCoreExactOutcome`; existing unchanged core identity/determinism tests | RSS samples and observation ceiling |
| edgeless reconstruction / canonical partition | `TestExecutionCanonicalCommunities` | worker reconstruction envelope metadata |
| typed dispatch, opaque declaration, two-seed schedules, commitment substitutions | `typed_bundle_execution_external_test.go` | outer request/response protocol and unsupported-supervisor refusal |
| typed stage/window cancellation | `TestBundleExecutionCancellationStages`, `TestBundleExecutionBoundaries` | wall/RSS kill, PID, Wait, delivery/reap, stdout transport limit |
| observer noninterference / report corruption | portable fixture test plus `TestBundleExecutionReportBarrier`; public-entry replay notification count | none of the computation/check assertions |
| five owner integration roots | same roots, portable controlled-filesystem fixtures; execution/replay cancellation through real owner path | worker-stage files, physical containment and cleanup evidence |
| nine owner accounting/admission roots | retained in `private_typed_bundle_operation_v2_test.go`, actual execution stage observations | PID marker clauses; the tenth, process-observation-method coverage root is retired |

The external packet contains initialization audit, exact compiler selection, actual RED/GREEN
logs, source hashes, affected test inventory and patch. Six bounded perturbations demonstrated
failure and restoration for full replay/corrupt reports, detached input, cancellation suppression,
one-shot consumption, observation-before-release, and exact accounting. This is not exhaustive
mutation coverage of all 283 original sites.

Local verification is limited to named pure algorithm/serialization tests and synthetic private
filesystem owner tests, normal and race. Each mode passed 36 roots / 106 test nodes with no
failures or skips: Program C 27 roots / 66 nodes, owner nine roots / 40 nodes. This includes
ten unchanged core roots beyond the 26 affected roots. The compiler-selected initializer audit
precedes those runs; generated mains contain no hidden child dispatch. Both affected packages and both command
test binaries compile on Darwin arm64/amd64, Linux amd64/arm64 and Windows amd64. Compile-only
results do not establish execution or production admission on those platforms. No native/provider
fixture was run locally; no repository-wide suite or hosted CI was rerun for this candidate.

Historical `qualification/program-c/i-07-resource-envelope.v1.json` and external candidate runners
remain unchanged. Their historical limits are not renewed by this work. Any source-pinned
qualification affected by the source changes remains a review obligation, not a reason to refresh
pins blindly. Stop this unit for independent review before commit; proceed to the shared
requirements map and next bounded cohort/application discovery only after its disposition.
