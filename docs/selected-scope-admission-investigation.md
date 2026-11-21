# Selected-scope admission correction and separate multipart proposal

## Disposition

The initial pass produced source findings and this report only. The continuation reproduced the defect with controlled RED tests, implemented the narrow correction, and passed focused GREEN/race/vet/build verification. Parent review remains required; no real acquisition or live CLI success is claimed.

Exact worktree: `/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY`; baseline HEAD `6a7a80494418193491dad4f47437545552d643aa`. Extensive pre-existing tracked/untracked changes were preserved through targeted edits. No stage/commit/push, provider/model/network/install, native acquisition fixture, public census start, retained-store access, subagent, or notebook mutation. The real acquisition authorization remains unused. Initial notebook search timed out. LSP listing did not establish exact-worktree binding; investigation used source fallback without creating a replacement server.

## Implemented

1. `internal/censusacquisition/discovery.go`: `SelectedSourceFiles` validates the complete canonical file list and patterns, then selects readable candidates using discovery's existing exclusion-first matcher. Shared canonical validation preserves duplicate rejection, path/URI agreement, and platform normalization. This returns metadata, not an execution capability.
2. `cmd/lsp-trace/census_folder_acquisition.go`: select before building the acquisition plan. Only selected members enter the runtime construction; the full original files still enter `DiscoveryAdapter`. Original paid records and their marshalled denominator digest are unchanged. Zero selection continues to refuse through existing `Build` empty-input behavior; it is not a complete empty application inventory.

No matcher syntax, enumeration owner, provider owner, retained-original API, schema, flag, grant, time/byte/packet ceiling, publication format, default full format, or private inactive compact candidate was changed. Filters are intentionally evaluated against the same full input at admission and discovery: admission limits runtime membership; discovery records exclusions without removing denominator members. No re-enumeration occurs.

## Custody and accounting findings

- Preparation intent includes original roots/includes/excludes; intent validation calls `ValidatePatterns` before funded enumeration. Recursive preparation appends paid cursor batches, retaining excluded paths; its reserve/readback/observe/readback sequence and exhaustion failure are unchanged.
- Cursor names are workspace-relative; directories are omitted only by the file adapter. Overlapping roots use existing `seen` deduplication; symlinks remain unsupported and are not followed.
- Discovery assigns denominator/ordinals from the full canonical list. Existing unreadable/unsupported dispositions precede filtering and remain incomplete, even when an exclusion pattern matches. Only readable selected files are supplied.
- `PrivateDiscoveryBindingV2` already separates `MemberDigest` from `DenominatorDigest`. The job's source authorization remains the full digest. Execute verifies selected member identity before runtime effects; runtime `admitURI` restricts source reads to those members.
- Supplied digests remain captured after actual owned supply; no input digest, request ID, native original, position, or capability is fabricated by selection. Retained M/P/S checks the original denominator digest and acquisition pointer, carries the original deadline, and remains one-shot. No retained-custody production change was necessary.
- Filtering does not refund paid metadata enumeration, make selected source reads free, or widen source-byte bounds.

## Tests and commands

All Go verification used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off`.

RED:

`go test ./cmd/lsp-trace -run '^TestCensusFolderSelectedScopeAdmission$' -count=1`

Before production edits, one-of-18 and boundary-16 failed: constructor calls were 0 instead of 1. Boundary-17, empty, and invalid-pattern denial passed. The constructor spy always returns an error before creating any provider/process. This independently demonstrates preprovider over-rejection, not successful acquisition.

GREEN: the same command reported 6 passed. Admitted cases call the refusal-only constructor once; denied cases call it zero times and create no acquisition directory. Paid preparation records remain byte-value equivalent.

`go test ./internal/censusacquisition -run '^(TestSelectedSourceFiles|TestDiscovery)' -count=1` initially reported 54 passed. Subsequent tests added overlapping-root/file-root/symlink metadata coverage.

`go test ./internal/inventorysuccessor -run '^TestLocalFolder(OwnedCaptureBinding|AcquisitionFreshBoundAdmission)$' -count=1` reported 5 passed. Includes post-reservation member tampering rejected before constructor, source-authorization retention, no original release on failure, reserve readback before effect, no replay/refund/reinitialization, and fixed ceiling refusal.

`go test ./internal/source -run '^TestBoundedRecursiveCursor$' -count=1` reported 1 passed.

Final:

```sh
go test -race ./internal/censusacquisition ./cmd/lsp-trace ./internal/inventorysuccessor -run '^(TestSelectedSourceFiles|TestDiscovery|TestCensusFolderSelectedScopeAdmission$|TestLocalFolderOwnedCaptureBinding$|TestLocalFolderAcquisitionFreshBoundAdmission$)' -count=1
go vet ./internal/censusacquisition ./cmd/lsp-trace ./internal/inventorysuccessor
go build -o /tmp/lsp-trace-selected-scope-build ./cmd/lsp-trace
git diff --check
```

All passed; race run reported 72 tests. Build artifact was not executed. Cases cover requested workspace-relative patterns, nested paths, Windows pattern separators and existing Windows canonical normalization, include/exclude precedence, invalid excluded identities/patterns, retained unavailable/unsupported dispositions, full denominator/ordinals, raw LSP positions, and source-root overlap. No exhaustive suite or native happy-path fixture was run.

## Exact source-only scope audit

A temporary metadata-only test walked the two requested directories and called the production selection helper with the exact requested patterns. It was removed after use. Its output remains at `/tmp/lsp-selected-scope-audit.log`.

- `cmd/lsp-trace`: 342 non-directory entries, including the newly added excluded test.
- `internal/applicationinventory`: 29 non-directory entries.
- Exactly **57 selected files**, requiring **4 logical parts (16/16/16/9)** at the unchanged ceiling.

These are filesystem counts, not paid preparation output or acquired coverage.

## Remaining constraints and multipart decision — NOT implemented

An earlier source-visible preparation constraint also remains: `PrepareRecursiveFolder` schedules at most 16 batches, each cursor read at most 8 directory entries. The observed directories exceed that traversal capacity. Selection after enumeration intentionally does not evade this paid budget. Even if preparation is separately admitted, 57 selected files still exceed the one-partition acquisition ceiling.

The local acquisition owner requires ordinal zero, exact existing discovery limits, and one fresh window. Existing `PrivateDiscoveryOwnerV2` offers partition reservation but binds resource rules to a specific binding; repeated calls to the local opener create fresh authorization rather than one cumulative job. Removing the partition-count check or looping fresh owners would be unsound. A job-scoped binding/authorization design is required before multipart implementation; it may reuse the existing kernel but must explicitly bind all selected partitions and their shared full denominator.

Smallest next decision: separately disposition preparation capacity, then approve or reject design of a single cumulative local-folder job contract. Do not authorize acquisition, raise ceilings, or extend deadlines merely by accepting this correction.

Multipart acceptance criteria:

1. One original denominator, selected partition membership, cumulative counter vector, and job deadline; no per-part reset, refund, renewal, or second preparation.
2. Preserve native 15-second and application 180-second maxima and all existing 64-packet/byte bounds; application time is not extra native time.
3. One global inventory with exclusions/unavailable dispositions retained. Logical groups are not source partitions; partition outputs alone cannot establish the whole denominator.
4. Retain originals through provider cleanup and global reconciliation, with no duplicated M/P/S or replay of one-shot owners.
5. Controlled tests prove 16/17 boundaries, provider-zero denial, cumulative exhaustion/failure accounting, original identity continuity, and cleanup before final publication. A separately authorized real attempt follows parent review and gates only.

## Exact final write-set

Production edits: `cmd/lsp-trace/census_folder_acquisition.go`; `internal/censusacquisition/discovery.go`.

Tests: new `cmd/lsp-trace/census_folder_scope_admission_test.go`; new `internal/censusacquisition/selected_source_files_test.go`; edited pre-existing untracked `internal/inventorysuccessor/private_folder_acquisition_test.go`.

Documentation: `PLAN-selected-scope-admission.md`; this report.

Transient audit test was created/run/removed; external audit log and build artifact are named above. Git diff includes pre-existing changes and cannot be interpreted as this pass's complete delta; in particular `PreparedWorkspaceFiles` predated this correction.
