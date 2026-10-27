# Private fixed-scope retention rehearsal

This is an opt-in Go test in this package, not a public command, CLI, or MCP API.
It reads the fixed six inspect/filter scope files once (four nomination, two
context), freezes exact source/selection/plan bytes, then uses the existing
one-CAS bundle owner. It performs no Census or LSP acquisition. A revision is
**CALLER_ASSERTED_NOT_AUTHENTICATED**, never verified Git provenance.

## Inputs and invocation

The caller must supply three **precreated exact existing private directories**:
primary, backup, journal. They must be distinct filesystem identities, owned by
the current user, mode 0700, and admitted by the production retention-root owner
(outside temporary directory trees, with its no-symlink rules). Initially all
three must be empty. The harness creates no directories and never chmods,
recreates, overwrites, expires, or deletes existing run state. Fixture cleanup
belongs to the preexisting test utilities, not the rehearsal.

After review, from the repository containing this implementation:

```sh
LSP_TRACE_SCOPE_REHEARSAL=retain-inputs \
LSP_TRACE_SCOPE_SOURCE="$SOURCE_ROOT" \
LSP_TRACE_SCOPE_REVISION="$ASSERTED_40_LOWERCASE_HEX_REVISION" \
LSP_TRACE_SCOPE_PRIMARY="$EXISTING_PRIVATE_PRIMARY" \
LSP_TRACE_SCOPE_BACKUP="$EXISTING_PRIVATE_BACKUP" \
LSP_TRACE_SCOPE_JOURNAL="$EXISTING_PRIVATE_JOURNAL" \
go test ./internal/censuslineagepilot -run '^TestScopeRealInputRetentionRehearsal$' -count=1 -v
```

There are no default source/output paths, caller ordinals, stages, retries,
expected heads, raw plans, budget overrides, or policy controls. Unset/empty
`LSP_TRACE_SCOPE_REHEARSAL` skips the test. Any nonempty value except the exact
`retain-inputs` sentinel refuses. All five other inputs are required when opted in.
Use `-count=1` to prevent cached results. The command above is a proposed real
rehearsal invocation, **not evidence that a real rehearsal has been executed**.

## Stops and terminal verification

The loop has at most twice (unique objects plus metadata) advances, with each
advance performing at most one CAS. Pending capabilities pass only within the
same live owner. Any error, conflict, failed, indeterminate, refused, or unexpected
result stops immediately without retry—even if a pending capability exists.
Known commits and exact receipts remain in the report. Lost pending remains an
explicit abandoned claim, never recovered by scanning orphan receipts.

Nonempty roots require the exact current binding/head. A changed source pin or
assertion cannot silently create an alternate key in an already used journal.
Existing CLAIMED/FAILED state is observed and stops; a READY prefix refuses
resumed execution. An existing finalized exact bundle can be verified read-only.
On successful completion a **fresh owner using the same frozen scope**, not a
checkout reread, verifies the terminal head and both copies with zero CAS.
This is **not an actual process-restart test**. Root identities are pinned and
compared on subsequent acquisitions; no whole-execution admission, atomic
checkout snapshot, actor monitoring, or expiry schedule is claimed.

## Machine report

The test logs compact canonical JSON generated from observations (no edited
summary). Authority remains zero, accepted false, completeness UNKNOWN, feature
identity UNRESOLVED. It includes raw-byte SHA-256/length for scope, selection,
plan, exact known head/receipt bytes and CAS receipts per step, final head and
receipt IDs, actual role/reference/unique-object counts, and verification/failure.
Raw head/receipt/linked receipt bytes are base64 JSON strings so malformed failure
evidence remains reportable.

Primary and backup data bytes are separate from per-copy reference metadata.
Each root reports measured regular-file count, logical bytes (`st_size`), and
allocated file bytes (`st_blocks * 512`); the latter excludes directory/volume
and filesystem-wide overhead and is not a free-space guarantee. Journal bytes
include its head, receipts, and lock file. Final namespace names/counts and copy
lengths must match the verified bundle or the report refuses. Reference payload
counts all nine references, whereas unique payload counts deduplicated objects;
primary and backup remain separate copies even when references deduplicate.
Policy reservation ceilings and per-owner charged logical/work bytes are shown
separately; none is represented as actual physical usage or whole-run admission.

Output is **stdout/test-log only**, not a durably retained summary. The caller
must preserve the native test output independently if desired, including failure
output; a finalized bundle does not make the stdout summary durable. There is no
automatic deletion, expiry, retry, or inventory-success claim.

## Fixture-only checks

```sh
LSP_TRACE_SCOPE_REHEARSAL= go test ./internal/censuslineagepilot -count=1
LSP_TRACE_SCOPE_REHEARSAL= go test -race ./internal/censuslineagepilot -run 'TestScope(Rehearsal|Bundle)' -count=1
go vet ./internal/censuslineagepilot
git diff --check
```
