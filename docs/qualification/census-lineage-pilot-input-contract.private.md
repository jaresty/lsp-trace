# Private lineage pilot: observed inputs only

This bounded prerequisite phase reads observed bytes for exactly two required
kinds: `SOURCE_SELECTION` and `PARTITION_PLAN`. This is a finite **input-only**
inventory, not the complete monitored Census inventory, a generic artifact-family
admission mechanism, or runtime admission. Authority is zero and accepted is
false in both fresh private envelopes:

- `lsp-trace.census-lineage-pilot-source-selection.private.v1`
- `lsp-trace.census-lineage-pilot-partition-plan.private.v1`

Explicit JSON DTOs retain workspace, ordered nonempty source identities, partition
size, identity, files, partition ordinals and membership. They do not freeze the
Go serialization of `censusacquisition.SourceFile` or `censussourceplan.Plan`.
Constructors use `censussourceplan.Build`; only construction may normalize file
order. Parsing requires the retained selection order to equal Build's order.
Every plan is reconstructed using that selection and retained partition size,
then compared completely, including envelope authority, plan authority, identity,
files, ordinals and members. `Validate` alone is insufficient. The plan also
retains the SHA-256 and length of the exact canonical selection bytes. Both
parsers reuse the existing strict machinery and require byte-for-byte canonical
JSON; unknown fields, duplicates, omissions, trailing data, null alternatives and
noncanonical spellings/order are rejected rather than repaired.

`InputSource.OpenInput` supplies read-only readers for the two fixed kinds.
`ObserveInputs` consumes at most each explicit byte limit plus one detection byte,
closes every opened reader, and fails on open/read/close errors, empty inputs or
limit violations. Private ceilings are 4 MiB selection, 16 MiB plan and 10,000
files. Explicit caller limits must be positive and may only tighten these ceilings;
partition size remains bounded by Build (1–1024). Byte limits apply before JSON
parsing; the file-count limit applies to parsed selection membership. Readers must
obey `io.Reader` and `io.Closer` contracts. Filesystem no-follow/transport safety
belongs to the source adapter; this unit supplies no filesystem adapter and makes
no filesystem ownership or monitoring-completeness claim. Limits bound bytes,
not elapsed time; blocking transports need their own cancellation/deadline policy.

Observed bytes are owned copies. Digests and lengths are derived from those
copies, never trusted from callers. Optional expected metadata is comparison-only
and must describe precisely the two required kinds once each; arbitrary kinds
are rejected. The opaque `ObservedInputs` has no caller-set trusted boolean and
returns fresh byte/DTO/nested collection copies, two artifact metadata records
(sorted by kind), and a value-copy linkage record for existing manifest hash and
length fields. Its zero value contains no validated input. This unit does not
construct a successful monitored manifest, call `Coordinate`, publish artifacts,
or operate any store or head. **Zero writes** occur in the input unit.

The monitoring gate is still unavailable. Probe, receipt, publication and
monitoring dependencies remain unqualified and not yet admitted. This unit is
neither monitoring-qualified, durable-sealed, Census-complete nor runtime-admitted.
The immutable v1 pilot manifest/head wires, reopen behavior and source-plan
behavior are unchanged. Later runner and monitor work must establish their own
dependency and admission contracts; input validation is not a substitute.

## Additive production read-only preflight phase

`Preflight` adds a filesystem adapter without changing `ObserveInputs` or the
immutable pilot wires. Its request contains an existing durable root,
workflow/lineage labels, and exactly two content-addressed comparison references
for `SOURCE_SELECTION` and `PARTITION_PLAN`. No manifest, plan DTO, expected head
revision, trusted boolean, or execution policy is accepted. The root must pass
`ValidateDurableRoot`, but this pathname check grants no acquisition authority.
`publication.OpenObservedRoot` independently binds the same clean-absolute-path,
no-symlink and `/private/tmp` exclusion policy to descriptor-traversed acquisition.
On Linux and macOS it starts from an owned `/` descriptor and opens each directory
component relative to its pinned parent with
`O_DIRECTORY|O_NOFOLLOW|O_CLOEXEC`. The final descriptor must identify a private
current-user directory. An ancestor replaced by a symlink between the initial
check and acquisition is rejected, including aliases into `/private/tmp`.

The opaque read-only `ObservedRoot` retains that exact admitted descriptor. To
reuse existing bounded content-read logic it also acquires an `os.Root` via the
pathname, but accepts that secondary capability only if its descriptor-reported
identity exactly equals the traversed root identity, before any content read.
This is not a before/after pathname check, and uses no `/dev/fd` assumptions.
Changing the path to a different directory during secondary acquisition rejects
and closes both capabilities. Rename after successful admission does not redirect
reads: observation uses only the pinned capabilities.

Every owned intermediate directory is closed after the next acquisition;
failures close every remaining acquired capability and join all cleanup errors.
Handles returned alongside an acquisition error are also closed. The successful
root is owned by preflight, borrowed by its input adapter, and closed before
returning; either root-capability close failure invalidates all result state.
Existing `OpenRoot` and other committed publication API behavior is unchanged.

The additive `Root.ReadContentObserved` admits only one canonical lowercase
`sha256:<64 hex>` basename directly beneath that pinned root. On Linux and macOS
it opens descriptor-relative with `O_NOFOLLOW`, `O_CLOEXEC`, and `O_NONBLOCK`
(the latter avoids a substituted FIFO blocking at open). Other platforms fail
closed. It checks regular 0600 current-user/single-link metadata, pre/open/post
identity and metadata, exact byte length and content digest, and propagates read
and file-close failures with no bytes returned. This establishes only the digest
and length of the bytes actually read under the checked custody conditions. It
does not establish file immutability, a continuous coverage interval, or that the
file will remain unchanged after observation. There is no selector traversal,
root creation, chmod, cleanup, write, ambient checkout fallback, or root-path
reopen during observation. Existing publication read APIs are unchanged.

The adapter uses repository-owned byte/file ceilings and passes actual observed
bytes through `ObserveInputs` for full canonical selection/plan coherence.
Explicit context is required; cancellation is checked before reads, between read
calls, and at safe validation/return boundaries. Regular-file reads are bounded
in bytes, not in elapsed time: arbitrary blocked kernel I/O is not promised to be
interruptible. Callers must not concurrently close a borrowed root capability.

The opaque result reports authority zero and `MONITORING_NOT_QUALIFIED`. It does
not create a manifest, call Put/CAS/Coordinate, execute Census, seal durability,
or admit runtime execution. The eventual mutating monitored runner must still
qualify its probe, receipt, publication, and monitoring dependencies separately.
