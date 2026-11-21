# Repository scope-job candidate v1 — Vertical 1 only

## Craft / disposition before mutation

Only this worktree is writable. Existing dirty changes and legacy census contracts are preserved. Exact-worktree managed LSP lookup returned APPLICABLE_SESSION_NOT_FOUND; inspect filesystem source instead, without starting a server. No provider acquisition, model API, database, network, installation, staging or commits.

Inspected seams: census_runner.go routes normal folders through prepareCensusFolder before native catalog; recursive_cursor.go owns process-local offsets but retains a seen map and has per-invocation limits. Existing normal acquisition is a single 16-file partition. Therefore changing normal default behavior safely requires the deferred multipart integration. This pass adds an explicitly gated census scope-job candidate and ordinary job-status operation; it does NOT declare the normal-default vertical finished. Existing census . remains unchanged.

## Contract

* New, closed scope-job.v1 schema, distinct from legacy inventory heads and diagnostics. Configuration, source policy, start and optional absolute deadline are frozen and hashed. USER STEERING supersedes 24h: default overall deadline is explicitly null (disabled); per-operation timeout defaults to 30 seconds, independent of pauses/overnight status inspection. Actual retained cap remains 2GiB; all other finite defaults are provisional engineering bounds, not measured fit.
* Enumerate all selected roots without gitignore. Normalize duplicate/overlapping roots. Go-first source selection: supported unacquired Go, unsupported known language, unknown extension/non-source, excluded, unavailable and unsupported kind remain separate. Includes/excludes use existing filter semantics. Existing explicit prune policy retains an unknown-descendant boundary. No content readability or native coverage claims.
* A process-local, bounded DFS cursor performs one metadata operation per step, retaining only bounded directory depth and roots, not the whole repository. Symlinks never followed. Metadata and filter costs are funded even for unsupported files.
* Immutable bounded claim/result segments form a hash chain. Claims persist before filesystem effects; failed/pending claims keep their full logical reservation. Actual retained bytes and outstanding disk escrow are separate. Reserve final checkpoint capacity before enumeration. Reopen streams verification without loading all paths. Interruption never resumes directory offsets or creates new allowance; pending work and frontier remain unknown.
* Scope closure is SCOPE_READY with explicit acquisition-not-implemented gap, not an inventory. Exhaustion/interruption are terminal partial scope. Agent automatically drives implemented operations; no per-step human approval.

## Implementation checklist

- [x] Add bounded source cursor and policy/configuration validation.
- [x] Add frozen job envelope, prepaid immutable claim/result ledger, bounded integrity-checked readback and terminal scope report.
- [x] Add gated census candidate and census job-status routing before provider code; leave legacy route unchanged.
- [x] Test controlled >128-entry trees, budgets, disabled overall deadline/optional fixed expiry, bounded operation timeouts, interruption/corruption, filter precedence, canonical roots and symlink safety; prove zero provider calls.
- [x] Run focused offline tests/race/vet/build/format; publish external report and owned-change hashes.

## Execution disposition

41 focused race-enabled tests passed across scopejob, censusacquisition and cmd/lsp-trace; focused vet/build and diff whitespace checks passed. A production-function metadata-only probe traversed the exact worktree: 251,506 entries, 225,809 regular files, 25,697 directories, 2,914 supported-unacquired Go files; retained 89,719,001 encoded bytes. Ordinary `census job-status` readback succeeded using the canonical `/private/var/...` external store path. Probe counts describe that traversal, not an atomic or eternally current snapshot. No native acquisition, semantic interpretation or inventory publication occurred.

External report directory: `/private/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/lsp-scope-job-report-1i3utw3z`.

Parent independent review is still required. This is a gated scope candidate, not completion of ordinary-default Vertical 1; no hidden acquisition continuation or human checkpoint was introduced.

## Provisional budget derivation

Target reference supplied by owner: 225,799 regular files + 25,695 directories. Round entry capacity to 2^19; metadata work is entry capacity + directory-close steps + roots, bounded by 2*entries + 128. Bound path bytes at 1024, DFS depth at 128, root/filter/prune counts at 128 each. One step obtains at most one directory entry or closes one directory. Ledger batches contain at most 128 records; each record's escaped path, metadata and policy fields have a conservative encoded bound. Logical byte reservations cover bounded buffers and publication verification copies, not RSS. Retained bytes measure encoded on-disk artifacts, not reserved logical copies or filesystem allocation overhead. Exact constants and derivations will accompany implementation; no claimed measured fit.
