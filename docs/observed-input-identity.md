# Observed-input identity (additive v1)

`source.BuildObservedIdentity(ObservedIdentityRequest) (ObservedIdentityResult, error)`
consumes `InputRecorder.Evidence`, including actual failed reads. It does not call
`AssembleManifest` or change `BuildIdentity`, `operational-identity.v1`, historical
manifest bytes, or graph V2/V3 identities.

## API and authority boundary

Request:
- `Evidence source.InputEvidence`: the bounded recorder's complete retained result.
- `Acquisition source.AcquisitionContext`: required nonblank Adapter; optional
  WorkspaceURI and InvocationID, as in the existing acquisition context.
- `Revision *source.RevisionAttestation`: optional nonblank System and Revision.

Result:
- `Policy`: exactly `lsp-trace.observed-input-identity.v1`
  (`source.ObservedIdentityPolicyV1`).
- `SourceID`: readable path/content commitment only.
- `SnapshotID`: source plus the complete normalized observation manifest.
- `CollectionID`: snapshot plus acquisition context.
- `Manifest []byte`: owned canonical observed manifest, described below.
- `Acquisition` and detached `Revision`: retained context and metadata.

**Later host grants must bind the exact result Policy and SnapshotID.** SourceID,
CollectionID, legacy manifest hashes, and revision strings are not substitutes.
This API does not provision or admit trust. A Git revision annotation is not
Git attestation verification and changes none of the three IDs.

The API validates consistency of public Go values, not origin. A caller able to
construct an entirely self-consistent `InputEvidence` can invent observations;
this API cannot establish that those bytes were read. Production must pass the
actual recorder output from its acquisition seam, retain original InputEvidence
bytes/statuses, and independently provision host authority outside requests.
Recomputing a canonical receipt here is a validation comparison only: supplied
receipts are never replaced by manufactured records. No cryptography,
whole-workspace completeness, or external-process dependency capture is claimed.
Do not mutate a request concurrently with a build. Returned values are detached;
mutating one cannot change a later build or the original request.

## Validation and membership

- Require `lsp-trace.contributing-input-evidence.v1` and the explicit reason
  `unobserved dependencies are unaccounted for`. The output always says
  `INCOMPLETE`, including readable-only, empty-readable, all-failed and empty
  acquisitions. Empty acquisition is not authenticated absence.
- Accept only SOURCE, CONFIGURATION, DECLARATION, GENERATED_MAPPING. The receipt
  mechanism must equal `bounded-input/<CLASS>` and have no provenance revision.
- Require canonical root-relative paths: reject empty/dot/parent, absolute,
  unclean, backslash, NUL, and colon/URI paths. Item ID, item locator and provenance
  locator must match exactly. Non-UTF-8 identity strings are rejected rather than
  lossily converted to replacement characters by JSON. No Unicode normalization,
  case folding, symlink dereferencing, or filesystem re-check occurs here;
  `InputRecorder` owns actual root containment, including symlink read failures.
- Recompute the receipt using the existing `CanonicalizeReceipt`; require exact
  Receipt fields and canonical bytes (including the historical trailing LF), and
  require ID = `sha256:<SHA256(canonical_receipt)>`. Extra JSON fields, whitespace,
  forged hashes, mismatched class/path/status/provenance and changed bytes reject.
- READABLE requires ContentIdentity matching the actual supplied content digest
  and no Failure. A genuinely empty readable file has the SHA-256 empty-byte
  digest and still contributes its path to SourceID.
- UNREADABLE requires a canonical nonempty failure reason, **nil Content**, and
  no ContentIdentity. Even a nonnil zero-length Content slice is rejected. No
  empty-byte digest or receipt-JSON digest stands in for unreadable content.
- Require at most one record per path and per receipt ID. Duplicate observations,
  same-path multi-class records and conflicting read versions reject. The recorder
  may retain several versions; this policy deliberately cannot select first/last
  or discard one. The consumer must explicitly separate acquisitions or seek a
  future multiversion policy. Repeated identical reads already coalesced by the
  recorder cannot be recovered by this API.
- All readable observations enter SourceID, including configuration/declaration/
  mapping inputs and observations without a bound output. Contribution bindings
  report retained use, **not** inclusion/exclusion decisions. No readable input
  membership is silently inferred from contribution references.
- Contributions require nonblank unique IDs and unique observed receipt refs.
  Empty references require `missing input binding for contribution: <ID>`.
  Failed observations require `failed input acquisition: <receipt ID>`. Incomplete
  reasons must be nonblank and unique. Additional reasons are retained and
  committed, not treated as a completeness proof or silently discarded.

## Canonical observed manifest

Version: `lsp-trace.observed-input-manifest.v1`
(`source.ObservedManifestVersionV1`). This is a new internal identity serialization,
not a replacement source-custody manifest or a registered public schema family.
Composition must supply publication/transport schema and semantic validation later.

UTF-8 Go `encoding/json.Marshal` bytes (including its default HTML escaping),
without a trailing LF, with fields in this order:

1. `schema_version`
2. `status`: always `INCOMPLETE`
3. `inputs`: sorted by exact path, each with fields in this order:
   - `path`, `class`, `status`, `receipt_id`, `canonical_receipt` (base64 bytes)
   - `content_digest` only for READABLE
   - `failure: {reason: ...}` only for UNREADABLE
4. `contributions`: sorted by ID; each has `id`, `receipt_ids` (sorted).
5. `incomplete_reasons`: sorted strings.

Empty arrays encode as `[]`. No receipt/content is reinterpreted or trimmed.
Canonical receipts already commit acquisition status, path/class mechanism,
content identity or failure reason; their exact original bytes are retained in
this manifest. Readable source bytes themselves remain in the caller's
InputEvidence, not duplicated in the identity manifest.

Failure path, class, status, reason, canonical receipt or hash changes affect
SnapshotID and CollectionID. Failure-only changes do not affect SourceID. Changing
UNREADABLE to READABLE adds the actual readable path/digest to source content, so
it changes SourceID as well. Contribution and incomplete-reason changes affect
snapshot/collection but not source; observation ordering changes no identity.

## Hash domains and framing

Let `P = lsp-trace.observed-input-identity.v1`. For kind K and byte components C:

`H(K, C...) = sha256(P + ":" + K + NUL + concat(uint64be(len(Ci)) + Ci))`

Each returned hash string is lowercase hex prefixed by `sha256:`. Lengths are
UTF-8 byte counts. The new domain is used for **all** three IDs, including source;
none silently aliases an old identity even for readable-only evidence.

- `sourceBytes`: compact JSON array of `{path,digest}` readable entries, sorted by
  path, digest from validated `Receipt.ContentIdentity.Digest` (not receipt ID).
- `SourceID = H("source", sourceBytes)`.
- `SnapshotID = H("snapshot", UTF8(SourceID), Manifest)`.
- `CollectionID = H("collection", UTF8(SnapshotID), contextBytes)`.
- `contextBytes`: compact JSON `{adapter,workspace_uri,invocation_id}` in that order,
  retaining empty optional strings.

Empty sourceBytes is `[]`, even with many failed observations. Failure records
remain explicit in Manifest, so empty observation and all-failed acquisition
have the same SourceID but different SnapshotID.

## Qualification and remaining join

Persistent tests use real recorder reads for mixed/all-failed/empty acquisitions,
actual changed bytes and changed failure reason/status, conflicting versions and
all four classes. Coherent test forgeries isolate validation from accounting.
Fixed vectors were independently encoded with Python JSON, uint64 big-endian
framing and SHA-256. Isolated reductions exercise snapshot, content, acquisition,
receipt hash, canonical bytes and ownership boundaries. Historical identity vectors
and source/graph/schema suites remain part of qualification.

Production source reads, trusted host startup, exact policy/snapshot grant binding,
public evidence schemas, failure transport, publication custody and CLI/MCP parity
are **not implemented by this correction**. Parent review precedes that next phase.
