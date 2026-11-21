# Bounded inventory enumeration consolidation

Status: bounded implementation and local verification complete; parent independent review required. External handoff: `/tmp/lsp-inventory-enumeration-jz2ZWF/REPORT.md`.

## Governing boundary
One applicationinventory DurableStore RunID/head/CAS ledger owns creation, enumeration, admission and evidence sealing. New candidate routing must not call scopejob.Start. Historical scope v1/v2 formats and read-only semantics are preserved; the common replay implementation is shared rather than duplicated. No native acquisition, provider, subprocess fixture, migration, notebook mutation or repository-wide metadata scan.

## Policy choice before implementation
Retain the existing <=64 MiB logical / <=64 packet execution profile. Enumeration uses the existing conservative 24 MiB batch quote, not a discounted invented currency. Reserve a fixed bounded setup/final-readback allowance in that same ledger before opening source metadata. Two batches can complete controlled >128-entry trees; larger jobs become paid PARTIAL without renewal. Whole-repository metadata funding is explicitly deferred. The 2 GiB admitted-evidence counter remains a post-retention acceptance bound, not total physical storage.

## Intended minimal changes
1. Establish a failing candidate CLI contract test: returns inventory RunID/status rather than separate scope owner.
2. Expose reusable process-local cursor and shared replay transition from scopejob without invoking its lifecycle. Keep legacy tests/readers and shared depth checks.
3. Freeze enumeration config before effects in inventory Create; retain existing creation quote and size limits. Add enumeration progress to the inventory head with bounded immutable segments beneath an inventory-owned namespace. No independent scope head/stop/ledger.
4. Claim all enumeration work by exact-predecessor CAS before effects. Process-local cursor only; failed/interrupted claims remain partial uncertainty. Close traversal to ReadyForAcquisition, never Describe or Seal. Admission is unavailable until verified enumeration closure; semantic acquisition remains unavailable in CLI.
5. Route candidate CLI and typed RunID status to the inventory owner. Keep legacy ID status read-only and ordinary census unchanged.
6. Controlled multi-batch, exhaustion, depth/siblings, cancellation/fault, strict config and preservation tests; offline normal/race, vet/build/format/diff. External report with exact write set and explicit limitations.

## Evidence
Read accepted design and creation repair report. Exact managed worktree session sk1:d972cfe4a8453e168ec8dbd34966138b2463596a36d4ee47633e0a00cd90f793 generation 1 reports Create -> createOwned with full definitions (request sc_10f616644225a1edda3784e50c732842). Prior review 00b6e6af-d3dd-426 is attributable under parent 01a1070d-1392-7797-8214-2c7836f6d572; initial bounded return was clipped and was not adopted as complete reasoning. Its attributable final return was subsequently retrieved; local source inspection and focused verification, not an assertion of exhaustive predecessor-process review, ground this handoff.

## Execution disposition
- All six implementation/verification steps completed within the declared bounded profile.
- Meaningful candidate-owner RED preceded routing changes; an additional closure-packet RED caught missing prepaid closure accounting and was repaired.
- First claim prepays fixed 9 MiB plus 24 MiB and three packets (batch pair + closure); later batches cost 24 MiB/two packets. Creation quote unchanged.
- Final controlled normal/race, legacy controls, named fixed io.Pipe two-admission compatibility test normal/race, vet/build and diff checks passed. No broad repository scan, native/provider acquisition, new fixture entry point, subagent, migration or full campaign.
- Same-owner admission reservation is tested without fabricating evidence. No genuine enumeration-to-native-acquisition workflow or whole-repository funding is claimed.
