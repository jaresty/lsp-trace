# Durable selected Describe and default inventory continuation

## Current phase 15 disposition

- [x] Deterministic held-input deadline-notification regression and narrow error classification; repeated race execution.
- [x] Candidate/proof confirmation with actual killed-writer cutpoints, late-proof refusal and post-expiry reopening; guard-removal RED/restored GREEN.
- [x] One durable manifest/cursor/predecessor/disposition ledger; all original preparations retained before producer exit; shared Describe/catalog/review/reconcile/publication semantics.
- [x] Default normal-folder automatic store/task entry, legacy routes retained; two-lead fresh-process completion and publication/reopen in both declared modes.
- [x] Corruption-sealed accounting and one persistent byte/work/packet/semantic budget; conservative claimed slices never refunded.
- [x] Isolate Unix owner assumptions; Linux durable continuation cross-build with explicit native-lifecycle refusal.
- [ ] Linux runtime and portable native lifecycle qualification; Windows ownership/publication/lifecycle implementation and full cross-build.
- [ ] Exhaustive filesystem syscall/power-loss interruption qualification and live semantic/actor acceptance (not claimed by controlled tests).

See `docs/census-durable-inventory.md` and `durable-inventory-20261119-15/RESULT.md` in the progress ledger. The remaining sections are historical scope/checkpoints, not current default-route limitations.

The owner explicitly accepts same-user application-owned persistent store authority. This supersedes the issuer-registry blocker in PLAN-retained-selected-spike.md; no external issuer, registry, signature or arbitrary bundle import is required or permitted.

## Scope

First milestone: genuine selected preparation -> owner-issued durable Describe task -> producer exits -> separate process reads bounded original artifacts and submits a semantic answer through CompleteSelectedDescribeV3. Do not report full inventory completion from this milestone. Extend the existing controller afterward, not a parallel semantic algorithm.

## Contract

- Configured application-store root is trusted local configuration, not a request-provided artifact path. Run/task IDs are lookup handles only. Same-user malicious whole-store rewrite and rollback are outside the guarantee.
- Issue accepts only an opaque completed PrivateSelectedResultV2 and validates through PrepareSelectedDescribeV3. Persist only the semantic attempt's selected row/unit/request, host/prompt/actor bindings, handoff/checkpoint and completed owner provenance. Never restore acquisition capability.
- Reuse publication exact-FD no-replace/readback and CAS for immutable content and authoritative state. Hashes detect corruption, not authenticate an arbitrary DTO.
- Claim before answer interpretation; malformed answers consume the task. Duplicate/concurrent/stale submissions cannot repeat semantics. A claimed task after crash is uncertain and non-replayable.
- Absolute wall-clock deadlines, not paused clocks: run <=180 seconds; task <=30 seconds including caller waiting; cumulative task allowance <=120 seconds. Native <=15 seconds remains spent. 64 packets, logical <=64 MiB, work <=1<<32 remain upper bounds. Every later read/submit reserves persistent usage before work; no renewal.
- Artifacts are individually selected and paged; metadata/status never imports caller content. Outputs remain provisional, authority 0, accepted false, completeness unknown; caller identity declarations are unauthenticated.

## Checklist

- [x] Implement owner-issued durable Describe custody with existing publication primitives.
- [x] Qualify actual independent-process success and bounded negative integrity/lifecycle controls (not exhaustive syscall crash injection).
- [x] Connect explicit `--durable-describe` normal-folder start and ID-based read/task/submit/result commands without requiring live host child; exactly one lead only.
- [x] Continue existing controller grouping/review/publication stages (implemented in phase 15, not implied by the earlier milestone).
- [x] Focused offline tests/race/vet and format/diff checks passed. Protocol: `docs/census-durable-describe.md`; phase report and ledger: `durable-describe-20261119-13` under the supplied progress-ledger directory.

## 2026-11-19 repair checkpoint (successor request still incomplete)

- [x] Regress caller-only CAS deadlines with real separate-process lock holders; derive pending/claim/final/confirmation contexts from original task/run bounds; preserve expired verified-result reopen.
- [x] Replace uncancellable stdin read with owned pollable input and joined cancellation; qualify helper counterfactual and genuine held-open CLI subprocess.
- [x] Qualify two simultaneous OS submit processes (one completion), actual killed claimed process, kernel lock release after holder death, and verified result reopen after original task expiry.
- [ ] Qualify all interrupted description/COMPLETE/confirmation fsync/close boundaries. A late committed confirmation is invalidated on observed failure, but death before invalidation remains unqualified.
- [x] Phase 15 implements multi-lead durable controller, shared budgets, next task, separate review, publication/reopen, and default no-manual-setup entry.
- [ ] Port folder/CLI lifecycle and isolate remaining platform assumptions; no Linux runtime or Windows support qualification added here.

Focused race tests, affected sourcecontinuation/applicationinventory suites, tagged normal-folder distinct-declared/held-input processes, scoped vet and diff checks passed. Exact-worktree projection now succeeds (`sc_ea3eb01bd19377db935fc37af0e4d29a`); this is source evidence only. Details and retained logs: `durable-repairs-20261119-14/RESULT.md` under the existing progress ledger directory. No external issuer blocker, renewed consent request, default-flow completion claim, or live actor independence claim.

## Milestone result

Controlled normal-folder A-exit/B-Describe success is implemented. Final race fixtures pass same-thread and distinct-role declarations, real 16-second post-exit wait beyond native lifetime, real semantic expiry refusal, exact query/accounting retention, original source absence, individually paged artifacts and verified result reopen. The task records the existing role prompts and explicit unverified/not-yet-reviewed assurance. App wall deadline is anchored at invocation start (<=180s), not renewed after preparation. A separate 30s maximum semantic interval includes waiting. DTOs remain rejected; issuance, submit and usage are one-shot/persistent as applicable.

Not completed: durable multi-item controller state, next-task advancement, grouping, separate sequential review and inventory publication. The ordinary connected path stays unchanged without the explicit milestone option. This is a functioning first slice, not full inventory completion or a new authority blocker.

No network, installs, live source/model, commits, staging, or pushes. Preserve unrelated dirty files. Exact-worktree managed session derived READY (offline-3); attempted projected-source calls rejected by client schema before dispatch, so implementation seam reasoning is filesystem-degraded.
