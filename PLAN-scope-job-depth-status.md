# Bounded frame-depth and status repair

Sole writer in exact dirty worktree; no other writers/subagents, notebook writes, old-store mutation, network/install, providers, native acquisition, public census, stage/commit/push or whole-tree metadata scan. Parent independent review required.

Local Craft read in full before mutation. Evidence: complete supplied independent report/depth overlay/logs and prior handoff, actual progress/cursor/config/job/namespace/read/CLI owners. Attributable reviewer 0943e50d-23bd-4a0 inspected through parent 01a1070d-1392-7797-8214-2c7836f6d572 (producer_locator/producer_owned), including concrete depth failure result fb882e3c1615ffd95ef47dd6cf7825a5dea5ab4e870cbb89267c77fd7e8fc955. Exact LSP lookup returned APPLICABLE_SESSION_NOT_FOUND; no server started.

## Ground / Craft
1. Replay may append a successful DIRECTORY frame iff existing frame count is below Budget.Depth. Path-component depth is independent; root unavailable/pruned dispositions do not append frames. Preserve paid pops, unvisited roots and frontier ancestry.
2. InspectContext is cooperatively bounded and cancellable; cancellation/deadline returns typed errors with no authoritative report. Compatibility Inspect uses a bounded background context. A caller cannot extend the status cap. Start's final readback remains a separately bounded cleanup/readback operation so an interrupted traversal can still return its persisted PARTIAL result.
3. Both legacy variants preserve historical debit and exact persisted bytes; all legal ordinal gap checks must finish for success. v2 remains isolated and verifies all owned names. No new execution allowance.
4. Default JobDeadline remains null. Inspection timeout is a new operation deadline, not overall expiry. No syscall preemption or atomic filesystem snapshot claim.

## Steps
- Reproduce unchanged independent depth test RED (done: one assertion failure, one passing live control; external red.jsonl).
- Add the pre-append DFS-frame guard; regression/positive boundary controls including cross-segment depth and sibling roots.
- Add context-aware bounded inspection and minimal publication streaming-read context support, plus CLI signal context. Use existing 30-second default operation timeout as hard status cap; honor a shorter frozen operation timeout from operation start, never restart the timer after header read. Existing non-status publication API remains unchanged.
- Add deterministic cancellation/probe/stream-read controls, byte-preservation and successful version accounting tests. No huge ordinal scans; private injected probe seam and small controlled fixtures only.
- Run unchanged depth normal/race, original seven probes normal/race, existing focused 168-result gate plus additive tests, publication read controls, vet/build/diff. Record exact write set and scoped before/after hashes externally.

## Intended write scope
This plan; scopejob progress/job/legacy_namespace and new regression test; publication checked-reader and context tests; CLI scope-job status owner and cancellation test; candidate documentation. No default budget/config/version/namespace admission changes.

Evidence directory: /tmp/scopejob-depth-status-repair-20261120-B (unique to this repair).

## Execution disposition
Implemented, not accepted. Unchanged original depth overlay reproduced one assertion RED plus one passing live control before production edits, then 2/2 GREEN normal/race. Expanded gate: 195 results / 53 top-level, one existing opt-in skip in each mode; all original 168 passing identities remain present. Unchanged prior seven: 7/7 each mode. Unchanged adversarial overlay: 17 results / 3 top-level each mode. Checked-reader gate: 38 / 6 each mode. Four external reductions produced intended runtime failures for probe checks, stream checks, frozen deadline renewal, and lost CLI context; unoverlaid post-reduction controls: 32 / 7 each mode. Vet, external-output build and diff check passed. Exact scripts/logs, bounded hashes and final write set are in the external report. No old retained stores or unrelated dirty files were changed.
