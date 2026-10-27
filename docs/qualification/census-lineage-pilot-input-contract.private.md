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
