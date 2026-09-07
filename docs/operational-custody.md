# Bounded operational custody

The explicit `execute` operational mode reads actual files and publishes a
classified **byte bundle**, not a provider/LSP trace or framework analysis. It
consumes `InputRecorder.Evidence` and `BuildObservedIdentity`; it never maps failed
reads to historical `ManifestReceipt` content digests.

## Operation input

The output directory must exist. `source_root` is a canonical absolute filesystem
path; each listed input is a unique canonical root-relative path. URI, parent,
absolute and conflicting/multi-class paths reject before reads. `os.Root` contains
actual reads, including symlink escape attempts (retained as unreadable outcomes).
Native drive-qualified roots are supported; UNC roots are not. Cross-build checks
do not constitute Windows runtime qualification.

The byte bundle accepts only opened regular files. On Unix, the scoped open uses
`O_NONBLOCK` before handle-based type checking, so even a concurrent replacement
with a FIFO cannot block the open. Directories and FIFOs retain unreadable failure
receipts; contained relative symlinks to regular files still work, and escaping
symlinks still fail. Windows uses `os.Root`'s reserved-device/namespace restrictions
and checks the opened type (no filesystem FIFO nodes). Other unsupported runtime
platforms fail closed for acquisition. This does not promise cancellation of slow
regular-file I/O, remote mounts, or regular-looking pseudo-files; no abandoned
read goroutines are introduced. Already observed failure evidence remains retained.

```json
{
  "root": "/absolute/output-directory",
  "operational": {
    "source_root": "/absolute/source-directory",
    "inputs": [
      {"path": "input.go", "class": "SOURCE"},
      {"path": "config.json", "class": "CONFIGURATION"},
      {"path": "types.d.ts", "class": "DECLARATION"},
      {"path": "mapping.json", "class": "GENERATED_MAPPING"}
    ],
    "require_authenticated": false
  }
}
```

```sh
lsp-trace execute --request-id run-1 --input request.json
```

MCP calls `lsp_trace_v1_execute` with `{"request": <the same input object>}`.
Optional operational fields:
- `trust_receipt`: base64 of the exact presented trust receipt bytes.
- `git_attestation`: optional existing `GitAttestationEvidence` representation
  (`EvidenceType`, `SourceSnapshotIdentity`, `CommitIdentity`).
- `revision`: optional `{system,revision}` annotation, not authority.

Unknown fields reject, including grants, context, policy overrides and trust config
paths. In operational requests, member names are exact and case-sensitive,
including nested input fields; legacy-only supplied-source case decoding remains
unchanged.
Recursive duplicate JSON members reject before direct/CLI decoding and before
MCP converts raw wire arguments to maps; escaped duplicate names also reject.
`operational` is exclusive with `source`, even an explicitly empty source.
An empty operational input list rejects; all listed reads are required when
`require_authenticated` is true. All four classes are recorded roles in the byte
bundle; their content is not parsed or executed.

Each actual `ReadInput` result becomes `outputs[]` with `id = input:<path>`, path,
status and returned bytes. That retained output binds exactly its observed receipt
through `BindContribution`. The evidence includes both these consumed output bytes
and original `InputEvidence` content, canonical receipts, statuses and bindings.
There is no producer declaration substituted for an actual read.

## Publication permission is not authentication or completeness

`require_authenticated` defaults to false (permissive): missing files and rejected
trust may publish explicit bounded evidence. Read failures have nil content, no
content identity, and their original canonical failure receipts. They affect the
observed snapshot, not source-content identity.

With `require_authenticated: true`, publication requires **both**:
1. Every requested file read successfully.
2. The exact acquired policy/snapshot and presented receipt/evidence are admitted
   by the independently provisioned host store.

Even a host grant approving the exact partial snapshot cannot turn failed required
reads into execution success. Its retained admission may say `AUTHENTICATED`
(custody of those outcomes), while `acquisition_status` is `PARTIAL`,
`publication_permitted` is false and execution fails.

Every observed manifest remains `INCOMPLETE` with
`unobserved dependencies are unaccounted for`, including positive host approval.
`READABLE` means requested reads succeeded, **not** dependency completeness. There
is no authenticated source-wide census or semantic/Program A/B acceptance.

The historical custody stage `Admitted` boolean remains a **permission to continue
publication**, not authenticated authority. Inspect `operational.admission.status`
for `MISSING_TRUST`, `REJECTED`, or `AUTHENTICATED`; do not promote stage success to
trust. Legacy supplied-source execution still uses its original synthetic
`hermetic/input.go` path, old bytes/formulas and nonauthoritative `MISSING_TRUST`
behavior when `operational` is omitted.

## Privileged host startup

CLI and MCP expose a startup-only flag:

```sh
lsp-trace execute --custody-trust-config /trusted/host-grants.json \
  --request-id run-1 --input request.json
lsp-trace-mcp --custody-trust-config /trusted/host-grants.json
```

The file is strict JSON `{ "grants": [ ... ] }`. Each grant has:
- `identity_policy`: exactly `lsp-trace.observed-input-identity.v1`.
- `grant`: existing `schema.HostTrustGrant` JSON, with `Receipt` (base64), `Context`
  (`TrustAuthenticationContext` fields), and optional `GitAttestation`.

Missing/wrong grant identity policy rejects startup, not merely operation
admission. The schema-owned store pins receipt bytes/snapshot but has no policy
field, so the wrapper independently pins the source policy per exact receipt.
The receipt's `trust_policy_id` is the host's trust policy; it is **not** an alias
for the observed source identity policy.

Trusted Go hosts can inject:

```go
store, err := custodyevidence.NewHostTrustStore(privilegedGrants)
// Handle err; never derive privilegedGrants from the current request/result.
executor := execution.NewProductionExecutorWithTrust(store)
```

`execution.LoadHostTrustStore(path)` is a startup convenience, not a request
parser. Host configuration must independently approve the receipt bytes,
identities, provisioning context and optional Git evidence before construction.
The wrapper uses **the acquired Identity.Policy and Identity.SnapshotID**. SourceID,
CollectionID, old manifest hashes or a caller's snapshot label cannot substitute.
Missing grants, changed bytes/snapshot, policy mismatch, substituted receipt bytes
or optional evidence mismatch do not authenticate. Store construction freezes
approved bindings; replacing the store is how a host changes/revokes approval.

The fixture in `operational_host_test.go` models an independently approved
pre-execution measurement; it does not obtain a grant from the operation's output.
**This is host-approved exact-byte allowlisting, not cryptographic verification**
of an `ED25519_SIGNATURE` label. Process/configuration owners are trusted; this is
not a sandbox against someone who can launch the process with arbitrary host
configuration. No proof of external-process dependency capture is implied.

## Artifacts, validation and failures

The existing execution response gains optional `operational` evidence. Successful
execution writes the same evidence as `root/artifact.json` and an exact-byte
publication receipt as `root/receipt.json`. Evidence family:
`operational-custody`, version `lsp-trace.operational-custody.v1`.

```sh
lsp-trace schema get --family operational-custody --version v1
lsp-trace validate --family operational-custody --version v1 /output/artifact.json
```

MCP `lsp_trace_v1_schema_get` and `lsp_trace_v1_validate` support this family.
For exact outer-byte preservation through MCP validation, pass the original JSON
**text as a string** in `input`, plus
`schema: {family: "operational-custody", version: "v1"}`. This is inline text, never
a path lookup. Optional `output_selector` publishes those exact validated bytes
under the MCP startup `--publication-root`. Object input also validates, but the
historical MCP map argument transport re-encodes object field order; do not use it
to preserve an existing outer-byte receipt.

`custodyevidence.ValidateFor` is the shared composed validator: structural schema
first, then source-owned `BuildObservedIdentity` recomputation from retained
InputEvidence, exact identity/manifest comparison, output-byte and contribution
joins, acquisition/permission outcomes and reported trust-binding consistency.
The lower-level `schema.ValidateFor` intentionally fails closed for this family:
`source` already imports `schema`, so importing source back into schema would
create a cycle. It must not report shape-only admission as complete validation.

Offline semantic validation **does not reauthenticate a historical host decision**
or prove public Go evidence structs originated in real reads. Live admission is
performed exclusively through the host wrapper/store. Exact-byte integrity of the
flat publication pair uses `verification.VerifyReceipt(artifact, receipt)`.
Historical CLI `custody`/MCP `verify` are graph-generation-selector operations;
those are not newly generalized to this flat evidence family.

On rejected authenticated-required execution, nothing is published. Error
`diagnostics` retain `operational_failure_evidence:<JSON execution artifact>`,
including completed stage results and available original read evidence. A
cancellation after a read retains already acquired receipts even before identity
construction. Such early failure diagnostics are explicitly pre-identity, not a
claim that a full operational evidence schema has passed.

Both operational and active legacy execution now sync the pinned output directory
after installing the artifact, before issuing a `CHECKED` exact-byte receipt, and
again after installing the receipt. File sync alone never supplies that claim.
Supported-platform directory-sync errors fail execution; Windows uses the existing
`UNAVAILABLE_ON_PLATFORM` policy. The receipt records completed artifact-parent
sync, not a guarantee against every power-loss scenario. No schema or enum meaning
changes: this repairs the producer to meet the existing v1 durability contract.
Historical immutable receipts are neither rewritten nor reauthenticated; legacy
artifact/identity bytes remain unchanged. Supported-platform successful receipt
bytes remain unchanged; Windows now truthfully reports unavailable durability.

Publication is two existing immutable writes, not a new atomic pair transaction.
If the artifact write succeeds but the receipt write fails, execution fails and
retains evidence; the first artifact may remain without a completed receipt pair.
No existing file is silently overwritten or deleted. Use a fresh destination for
replay; equivalent source/context yields identical evidence bytes. Request ID is
the acquisition InvocationID; changing it may change CollectionID, not SnapshotID.

Source bytes can be sensitive: successful artifacts and failure diagnostics both
retain them. Hosts run with their existing filesystem privileges. This increment
adds no remote authorization layer, live deployment, provider/LSP instrumentation,
external framework semantics, acceptance flags or consumer repository changes.
