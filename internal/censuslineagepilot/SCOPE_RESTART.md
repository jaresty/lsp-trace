# Private retained fixed-scope process restart

`readRetainedFeatureScope` is observation-only. It accepts exact existing primary,
backup, and journal roots plus a finite locator: raw head digest/length, head key,
raw scope digest/length, and metadata digest/length. These are lookup/comparison
pins, not authority, permission, a caller-selected scope, or a recovery capability.
There are no default real paths, mutable selectors, stages, ordinals, raw plans,
trust booleans, or historical source constants.

## Reconstruction boundary

The reader admits descriptor-bound current-user private roots with the existing
no-follow/outside-`/private/tmp` publication API, requires distinct identities,
and compares all three identities to the exact pinned head binding. Only
`RETAINED_INPUT_BUNDLE` is admissible; CLAIMED, READY, FAILED, unknown, absent,
or changed heads refuse before payload/receipt reads. No orphan discovery occurs.

Both exact raw scope-manifest and metadata copies bootstrap the reader. Strict
canonical JSON decoding uses the existing immutable layouts. Every one of the
nine ordered role references is read from both roots, hash/length/equality checked
under repository-owned limits, even when storage deduplicates identical bytes.
The selection and partition plan use the existing strict parsers and `Build`
reconstruction; `deriveScopeRetentionPlan` and `validateScopeRetentionPlan` require
byte-identical deterministic manifest and metadata. Parsed fields alone never
admit a scope. Raw storage digest remains distinct from the domain scope digest.

The existing `verifyPrefix` walks exactly the terminal head-linked receipt chain
with the reconstructed scope, checks every successful object and final metadata
in both copies, and deterministically reconstructs the exact terminal head,
generation, and index. The head is reread for equality at completion. This is a
bounded observation, **not an atomic filesystem snapshot**. Root/object close
errors and cancellation refuse with zero opaque output and joined causes.
Returned scope/head/metadata/receipt accessors deep-copy their data. No root or
write interface is returned; no advance, CAS, publication, retry, owner recovery,
expiry, or deletion is performed.

Source root/file identities in the manifest describe the historical retained
observation, **not the current checkout**. The reader never reopens source paths
or calls `freezeFeatureScope`. Authority remains 0, accepted false, completeness
UNKNOWN, identity UNRESOLVED, revision CALLER_ASSERTED_NOT_AUTHENTICATED. There is
no authenticated revision, checkout snapshot, runtime/monitoring admission,
Census/LSP acquisition, or generic store framework.

## Disposable process evidence

`TestScopeRestartActualProcessWithoutSource` retains a disposable fixed scope via
the existing fixture owner, deletes only its explicitly test-owned source fixture,
then launches the test binary in a fresh process. Its narrowly guarded helper
receives **only root paths and the finite locator** on stdin, with no inherited
source/rehearsal environment or serialized scope bytes. Distinct and deduplicated
fixtures must reconstruct the same raw/domain scope and leave every retained
filename, bytes, mode and modification time unchanged. A fresh object in the
original process is not the claimed evidence. This test does not imply an OS
sandbox; the source fixture is actually absent.

```sh
LSP_TRACE_SCOPE_REHEARSAL= LSP_TRACE_SCOPE_RESTART= \
  go test ./internal/censuslineagepilot -run '^TestScopeRestart' -count=1 -v
LSP_TRACE_SCOPE_REHEARSAL= LSP_TRACE_SCOPE_RESTART= \
  go test -race ./internal/censuslineagepilot -run '^TestScopeRestart' -count=1
go vet ./internal/censuslineagepilot
git diff --check
```

## Opt-in real observation — after independent review only

No real retained rehearsal/restart was invoked during implementation. The separate
`TestScopeRealRetainedRestartReadOnly` entrypoint cannot invoke the write-capable
rehearsal. Unset `LSP_TRACE_SCOPE_RESTART` skips it; any nonempty value other than
`read-retained-inputs` refuses. The exact generated locator packet file is required.
No source path/revision is accepted by this entrypoint.

Mechanically derive the packet from the **prior native generated rehearsal report**
(the JSON object, not edited prose). Given `r` as that report and the exact three
existing root paths, decode the last step's `head_bytes_base64` to `h` and emit
compact JSON in this field order (no trailing newline; strict canonical encoding):

```text
{"roots":[PRIMARY,BACKUP,JOURNAL],"locator":{"head_key":h.key,"head":{"id":r.final_head.id,"byte_length":r.final_head.byte_length},"scope":{"id":r.scope.id,"byte_length":r.scope.byte_length},"metadata":{"id":h.binding.metadata,"byte_length":r.metadata_bytes_per_copy}}}
```

Before supplying it, mechanically check SHA-256/length of the decoded head against
`r.final_head`, `h.key` against the digest suffix of `r.final_head_selector`, and
`h.binding.scope` against `r.scope.id`. These derivations do not grant authority;
the reader independently checks actual retained bytes and current root identities.
The locator packet intentionally contains no source bytes or observed source root.
Use lowercase `sha256:` digest IDs, integer lengths, the indicated field order,
and no whitespace outside string values.

```sh
LSP_TRACE_SCOPE_REHEARSAL= \
LSP_TRACE_SCOPE_RESTART=read-retained-inputs \
LSP_TRACE_SCOPE_RESTART_LOCATOR_FILE="$GENERATED_LOCATOR_PACKET" \
  go test ./internal/censuslineagepilot \
  -run '^TestScopeRealRetainedRestartReadOnly$' -count=1 -v
```

Output is native test-log JSON, not a durable report publication. Preserve the
native output separately if needed; the reader writes nothing into retained roots.

## Next document-supply boundary

Only after independent review and the exact real read-only restart observation
may a later checkpoint design document-supply from the reconstructed opaque scope.
This change does not supply Manager documents, change Manager APIs, admit runtime
execution/monitoring, or generalize retention. Preserve fixed four nominations,
two context-only sources and external-frontier exclusions at that boundary.
