# Repository scope-job candidate — isolated v2, read-only v1 compatibility

## Product disposition

This is an explicitly gated, metadata-only candidate for Vertical 1. **It does not replace `census .`, and the ordinary-default vertical is not complete.** The existing normal route still owns its legacy bounded preparation/acquisition/task pipeline. Its single 16-file acquisition window is unchanged. No legacy v1/v2 head is migrated, renamed, reset or reinterpreted.

```sh
lsp-trace census . --scope-job-candidate
# Use --profile gopls when root go.mod/go.work evidence is absent.
# An optional --publication-root must name a canonical, private directory outside the workspace.
lsp-trace census job-status JOB_ID [--publication-root CANONICAL_PRIVATE_DIRECTORY]
```

The candidate emits a `lsp-trace.scope-job-start.v1` JSON receipt on stderr **after persisting the header and before enumeration**. It contains the job ID and status operation, so an interrupted process can be inspected. The final stdout report uses `lsp-trace.scope-job.v2` (including qualified historical inspection). New persisted headers also use that explicit incompatible version. Neither reuses inventory or diagnostic schema versions. Execute returned supported operations automatically as the calling agent; no human approval checkpoint or model API is involved. `status_operation` is inspection, not an instruction to poll indefinitely. No executable acquisition operation is returned: `capability_gap` explicitly names the missing multipart owner.

`SCOPE_READY` means the metadata scope attempt has closed, awaiting an unimplemented acquisition capability. It is **not** a publication, feature inventory, semantic assessment, readability proof or native-coverage claim. `denominator_complete` is true only when enumeration closed without unknown-descendant boundaries and the complete isolated job namespace was verified. Legacy flat-store inspection always withholds this flag. `PARTIAL` preserves known records and conservative frontier on exhaustion, interruption or timeout. `UNSEALED` on readback means the attempt lacks a terminal seal; it does not infer whether another process is alive. A pending claim explicitly preserves uncertain effects and full debit.

## Duration is not budget renewal

Owner steering superseded the earlier proposed 24-hour default. The frozen canonical configuration now contains **`job_deadline: null`**, and the header contains **`expires: null`** by default. Overnight pauses and later inspection do not expire the job, reset reservations or delete artifacts. Individual enumeration operations have a separately frozen 30-second timeout. A claim persists its start and absolute operation deadline before effects; the deadline is checked before opening/traversing source and between metadata steps, including after the last step. Filesystem syscalls are cooperatively bounded, not forcibly preempted or an OS sandbox.

Each status/readback operation has a separate cooperative deadline: at most the existing default operation timeout (30 seconds), shortened by the frozen job's operation timeout or an earlier caller deadline. `InspectContext` propagates caller cancellation; `Inspect` remains a compatibility wrapper with a bounded background context. CLI status uses an interrupt/SIGTERM context. The clock starts before reading the header and is not renewed at each packet. Checks run before/between legacy exact-name probes, between verified stream reads and segment/record processing, and during v2 owned-name validation. A cancellation or deadline returns a typed `context.Canceled` / `context.DeadlineExceeded` error and no authoritative report; CLI emits failure, not a partial balance, completeness flag or successful zero-spend result. All saved records remain unchanged. Start's terminal readback uses its own bounded operation, so cancellation of traversal can still yield the persisted PARTIAL report. Blocking filesystem syscalls and descriptor cleanup cannot be forcibly preempted; this is not an OS sandbox or a hard wall-clock responsiveness guarantee.

An optional fixed absolute job deadline is supported by the internal configuration API and tested; the candidate CLI intentionally offers no extra deadline flag. It is copied, hashed and never renewed. Existing legacy expiry contracts remain unchanged.

A directory offset is process-local. There is deliberately **no scope enumeration Resume operation**. After interruption, status observes the same configuration, reserved spending and unknown frontier; it neither reopens offsets nor grants another allowance. Future scheduling may use only legitimately unspent work through a later owner contract. It must not replay or renew expired native capabilities. No native capability exists in this candidate.

## Frozen source policy

The candidate has one supported profile: `gopls`, language `go`. Profile preflight is local and starts no server. It selects regular Go-extension files using the existing `SelectedSourceFiles` matcher. Tests, generated files and vendor Go files are included unless explicit `--include`/`--exclude` filters remove them. No gitignore policy is introduced.

All physically enumerated metadata is charged, not just selected sources. Records preserve path, kind, size, mode, modification time and whether metadata was observed. Roles are separate: `SUPPORTED_UNACQUIRED`, `EXCLUDED`, `UNSUPPORTED_LANGUAGE`, `NOT_SOURCE` (unknown extension), `UNSUPPORTED_KIND`, `UNAVAILABLE`, and `DIRECTORY`. Unavailable/unsupported roles precede selection filtering. The original observed regular-file denominator is retained alongside selected counts, not replaced with the selected subset.

Roots are canonical local paths, sorted/deduplicated, with descendant roots removed when an ancestor already covers them. Explicit root depth is checked before publication and again before the cursor's first prefix metadata effect. Depth retains the original workspace-relative component convention (`.` counts as one); it does not count the absolute workspace prefix. NUL bytes in workspace, root, prune, or filter fields are rejected during configuration admission. Includes/excludes use existing validated pattern semantics and deterministic ordering. Explicit `--prune-dir` retains an excluded directory boundary with **unknown descendants**, never a fabricated descendant count of zero. Symlinks are not followed; explicit symlink roots are refused by the paid traversal. Workspace aliases are rejected before artifact creation. The store must be outside the workspace.

No source content is read or hashed. Segment hashes protect retained metadata bytes, not source-content identity. This is not an atomic filesystem snapshot. Files can change during or after enumeration; a future acquisition contract must revalidate source identity/content before effects.

## Finite starting defaults — engineering bounds, not owner-approved measurements

The owner accepted configurable starting defaults and a 2 GiB retained cap, **not a measured-fit guarantee or these logical numbers as previously approved allowances**.

| Dimension | Default | Derivation / meaning |
|---|---:|---|
| Overall deadline | Disabled (`null`) | Explicit owner correction; no large sentinel |
| Per-operation timeout | 30 seconds | Provisional bounded traversal lease; no renewal |
| Actual retained bytes | 2,147,483,648 | Owner-selected 2 GiB starting cap; encoded artifact bytes, not filesystem blocks/RSS |
| Entries | 524,288 | `2^19`; above supplied reference 225,799 files + 25,695 directories |
| Directories | 524,288 | Cannot exceed entry ceiling; includes root |
| Traversal steps | 1,048,704 | `2 * entries + 128`: entries, directory EOF/close, bounded roots |
| Records/steps per operation | At most 128 | Constant bounded buffers; no all-repository path map |
| Logical work | 1,082,262,528 | Unchanged ceiling; v2 first spends 32 fixed namespace/header/receipt units, then charges `(8 + 2×depth + roots + prunes + 2×filters)` per reserved step |
| Logical bytes | 206,193,033,216 | 8,193 batches × 24 MiB + 9 MiB fixed reserve; conservative encoded-buffer/copy/readback accounting, **not actual disk usage** |
| Packets | 16,388 | Two immutable packets per maximum batch, plus header and stop |
| Path bytes | 1,024 | UTF-8 path bound |
| DFS depth / open frames | 128 | Bounds descriptors and directory frontier |
| Roots, includes, excludes, prunes | 128 each | Fixed configuration ceilings |

Header reservation: 256 KiB, with 4 KiB carved out for namespace/receipt logical bytes in v2 (encoded head limit 252 KiB); fixed four-pass accounting is unchanged. Namespace admission and bounded receipt output are prepaid by 32 fixed logical-work units before directory creation; a lower work allowance rejects without artifacts. Directory allocation overhead is not encoded retained bytes. Claim bound: 2 MiB. Segment bound: 4 MiB. Stop bound: 2 MiB. Four encoded-buffer passes per claim/segment reserve 24 MiB logical bytes (encode, write, verify, later readback); fixed header/stop reserve is 9 MiB. These intentionally conservative reservations are consumed even when actual records are smaller. Work is a logical operation counter (metadata/path/filter operations), not an instruction counter or CPU/RSS sandbox. Closed upper ceilings prevent arithmetic overflow and malicious stored-config allocation.

Override defaults with `--scope-job-budget BUDGET_JSON`, a complete JSON Budget object using the exact `head.config.budget` field names shown in a report. Durations are integer nanoseconds. Partial budget objects, duplicate keys (even equal values), non-exact key spelling/case aliases, unknown fields, nested/non-numeric values, trailing JSON, zero/oversized bounds and unsafe configuration are rejected before opening the selected store or creating job artifacts. Whitespace and field order are immaterial; integer range checks remain typed. Failure is the existing single stderr JSON diagnostic `{"schema_version":"lsp-trace.census-diagnostic.v1","status":"FAILED","stage":"config","code":"INVALID_CONFIG","retry":true}` with nonzero exit, no start receipt and no stdout report. A supplied budget is never silently completed with fresh defaults. Configuration cannot be changed on `job-status`.

## Persistence and accounting

The existing private-root, strict verified-read and immutable bound-file publication primitives are reused. Filenames contain only a random job ID, ordinal and artifact role; no source names or configuration secrets. A frozen header seals configuration, policy, start and optional deadline. Each bounded claim links the prior segment hash and persists the cumulative debit **before** filesystem effects. Its segment links the claim hash and contains at most one batch of records and bounded frontier. There is no growing full-state rewrite.

Before any optional traversal, the header reserves terminal-report bytes. Each operation admits claim + result + terminal escrow against remaining retained capacity. Actual retained bytes and outstanding disk escrow are distinct from logical copied bytes. Successful publication replaces disk escrow with actual encoded retained bytes; it does not refund logical spending. Missing/failed results retain the entire claim debit and outstanding disk escrow. No retry/requote is offered for a pending claim.

Status streams bounded segments, checks closed JSON, hashes, links, cumulative spend, limits and the terminal summary. Record validation also checks workspace-relative root membership, metadata/kind/role compatibility and the same frozen source-selection matcher used by traversal. Frontier paths must be unique, within roots and depth; terminal reason/completion relationships are checked. A bounded replay validates the canonical explicit-root cursor and DFS ancestor stack across segments. Each explicit root requires one direct disposition before advancing; cardinality cannot substitute duplicate roots for omitted roots. Before appending a successfully opened DIRECTORY frame, replay enforces the same simultaneous-frame ceiling as the live cursor. This is separate from component depth: `.` and `a` each have component depth one, but together require two frames. Pruned/unavailable root dispositions do not open frames, and later sibling roots start with the emptied stack rather than a cumulative directory count. Implicit directory EOF pops consume reserved steps. The outgoing frontier must be the unvisited root suffix followed by a prefix of the evidenced active ancestor stack. No invented child frame or skipped parent/root is allowed. On error the persisted conservative frontier remains admissible without claiming progress it does not prove. Canonical ancestor-overlap normalization removes redundant roots before traversal; a prune covering an explicit root still produces a direct root boundary, not invented descendant coverage. Pruned/unavailable boundaries remain legitimate unknown descendants, including a closed `SCOPE_READY` attempt with `denominator_complete:false`.

New jobs use IDs `v2-<32 lowercase hex>` and a fresh private child `scope-<JOB_ID>/` containing `packet-head.json`, `packet-stop.json`, and bounded ordinal claim/segment files. Lookup derives this one child directly from the ID; it never lists the shared store. Creation is exclusive, validates owner/privacy, uses pinned-root child operations, refuses symlinks/escape, and syncs the parent. Pre-header cleanup removes only an empty child; a late failure never deletes a committed header. Start preserves the job ID on failures after header publication, including failed terminal inspection. Explicit store > environment > default selection is unchanged.

For v2, status visits **every immediate child name**, rejecting all unaccounted names, even another job's filename, sparse far ordinals, orphan segments and temporary/unexpected files. The unchanged name visitor reads at most `budget.packets + 1` names in chunks <=128; siblings in the shared parent have no bearing on the bound or result. Header-only and last-pending states remain readable. `namespace_verification: COMPLETE_PER_JOB` is explicit. Retained bytes and packet caps are per job, not a shared disk quota; directory/filesystem allocation overhead and aggregate sibling disk consumption are not included. No global fit guarantee is made. Concurrent same-job mutation is not an atomic snapshot.

**Legacy limitation:** flat v1 has no bounded complete ownership index. Read-only inspection performs exact-name probes for missing claim/segment slots through `(budget.packets-2)/2` inclusive, at most `budget.packets+2` probes, constant memory, independent of unrelated siblings. It detects gaps/orphans in that legal range (including a boundary sentinel), but cannot prove absence of arbitrary unexpected names or sparse ordinals outside it without scanning the shared directory. It reports `LEGACY_LEGAL_ORDINALS_ONLY`, `unknown_remainder:true`, and `denominator_complete:false`, even for a retained READY stop. This is a weaker, explicit historical-inspection contract, not a uniform per-job namespace guarantee or new execution allowance. Very large historical packet caps can require up to roughly 33.5 million exact-name probes; the status operation deadline may therefore refuse inspection before the legal range is exhausted. It must not skip later evidence or return success on an incomplete scan. Future inspection starts a new bounded administrative operation, but does not renew any job budget or alter records. No source traversal occurs. New sibling jobs cannot change this qualification or invalidate unchanged historical status.

New v2 headers carry `work_model:"policy-v2"`; the schema/ID combination identifies the isolated layout and fixed starting debit. Its per-step coefficient remains `8 + 2*depth + roots + prunes + 2*(includes+excludes)`. Original marker-free v1 keeps the historical smaller coefficient and zero starting work; the exact already-produced intermediate v1/policy-v2 variant keeps its policy coefficient and zero starting work. Both are strictly decoded, read-only, and hash-linked to their original claims. No field removal, head rewrite, migration, expiry renewal, refund or new execution occurs. Unknown schema/layout/model tuples return `ErrUnsupportedVersion`; malformed known shapes, duplicate/aliased keys and broken hash links fail integrity. Segment/claim/stop shapes are unchanged and are interpreted only under their hash-linked head's format. Legacy census formats are untouched.

### Version compatibility matrix

| Header schema / model | ID / layout | Read disposition | New execution |
|---|---|---|---|
| scope-job.v1 / absent | 32-hex / original flat | Strict original closed shape; historical debit; qualified legal-ordinal inspection | Never |
| scope-job.v1 / policy-v2 | 32-hex / intermediate flat | Exact known intermediate shape; historical debit; same qualification | Never |
| scope-job.v2 / policy-v2 | v2-32-hex / isolated child | Strict v2; fixed namespace debit; complete bounded owned-name inspection | Start only, no Resume |
| Any other tuple | Any | Explicit unsupported-version error, or integrity failure for malformed known shape | Never |

An original closed v1 reader rejects the new v2 header; it is not made permissive. Current reports identify v2 independently of the historical persisted head version.

Status reads no sources and cannot mutate configuration, spend or expiry. Administrative integrity inspection is explicitly outside the execution allowance, bounded by stored packet/config ceilings and the separate cooperative status deadline, and can be attempted after optional expiry; it is not a hidden new traversal allowance. Hashes provide integrity, not authentication against an actor able to rewrite the entire private store.

## Source closure for the three-blocker repair

| Owner | Inspected responsibility / repair boundary |
|---|---|
| `internal/scopejob/config.go` | Canonical roots, policy/budgets, strict sealed JSON, explicit schema constants |
| `internal/scopejob/job.go` | Start/receipt ordering, frozen debit, streaming ledger, tuple dispatch, terminal and namespace verification |
| `internal/scopejob/progress.go` | Root cursor and evidenced DFS transitions; bounded state, no repository-sized uniqueness set |
| `internal/scopejob/legacy_namespace.go` | Read-only flat-store exact-name gap probes and explicit completeness limit |
| `internal/scopejob/cursor.go` | Production metadata traversal, direct root dispositions, conservative error frontier; unchanged |
| `internal/publication/{selector,boundfile,boundfile_read_checked,directory,names}.go` | Existing pinned capability, immutable publication, strict custody reads, sync, bounded name visitor; unchanged by this repair |
| `internal/publication/private_child.go` | Narrow private child creation/open and empty-only cleanup using existing pinned primitives |
| `cmd/lsp-trace/census_scope_job.go`, `census_durable_inventory.go` | Metadata-only entrypoints, receipt and status, explicit > env > default store selection; unchanged |

The proof is intrinsic consistency of retained metadata, not authentication or proof that every filesystem child was present/unique. No all-nested-path map, PKI, fixture identity pin, source-content read, or complete filesystem snapshot claim is introduced. An actor able to coherently replace private artifacts remains outside hash authentication guarantees.

### Subsequent frame-depth/status repair boundary

The bounded follow-up changes `progress.go` frame admission, `job.go` / `legacy_namespace.go` inspection cancellation, the checked publication reader's additive context variant, and CLI status context propagation. Existing publication API callers retain their background-context behavior; child admission, name visitor, version dispatch, stored shapes, defaults and ordinary census routing are unchanged. The preceding source-closure table describes the earlier three-blocker repair, not a claim that these follow-up owners remain unmodified.

## Verification boundary

Controlled tests cover >128 entries and >16 selected files, selected/unsupported/excluded accounting, root overlap, unknown prune boundaries, symlink safety, finite budget dimensions, disk escrow, invalid configuration before artifacts, fixed optional deadline, disabled default deadline, operation timeout, pending reservation readback, corruption, legacy-version refusal and provider-factory spies. The source tree probe uses the same `scopejob.Start` production function, not native acquisition or a second synthetic inventory path.

The gated route is useful and testable now. Integrating its preflight into ordinary `census .` without breaking small-census behavior, introducing a paid multipart acquisition owner, global catalog assembly and semantic/review continuations remain future work. No automatic hidden legacy flag or fake acquisition-ready operation bridges that gap.
