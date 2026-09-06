# Opt-in operational identity policy v1

Status: bounded policy proposal implemented by `source.BuildIdentity`; not a global migration or program acceptance. Callers explicitly opt in using this new API. Existing graph schemas, source trust bridges, and historical IDs are unchanged.

## Contract

`IdentityPolicyV1 = "lsp-trace.operational-identity.v1"`.

`BuildIdentity(IdentityRequest) (IdentityResult, error)` consumes existing `ManifestReceipt` and `ManifestDecision` values, optional `*RevisionAttestation{System, Revision}`, and `AcquisitionContext{Adapter, WorkspaceURI, InvocationID}`. Adapter is required; workspace and invocation may be empty when unavailable. Missing values remain missing, not invented. A present revision requires both nonblank fields; Git is optional and no Git command is run. This annotation is not authenticated attestation evidence and cannot substitute for host-provisioned trust.

`ManifestReceipt.Digest` MUST identify acquired bytes (e.g. `Receipt.ContentIdentity.Digest`), not canonical receipt JSON containing provenance. `ManifestReceipt.ID` may reference the canonical receipt digest. Callers own that mapping and actual-read evidence. The builder checks manifest structure/accounting and digest syntax, not the truth of caller-supplied source claims. Unreadable inputs cannot claim acquired source content; account for them by explicit exclusion.

Let `H(kind, components...)` mean SHA256 of UTF-8 bytes of `lsp-trace.operational-identity.v1:` + kind + NUL, then each component prefixed by its unsigned 64-bit big-endian byte length. Every result is lowercase `sha256:<hex>`.

1. `Manifest`: exact bytes from `AssembleManifest`, including final newline. Included receipts sort by path; excluded decisions sort by their receipt's path. Existing accounting, duplicate, parent-closure and cycle checks apply.
2. `SourceID = H("source", content)`, where content is compact Go JSON of an array of included `{ "path": ..., "digest": ... }` entries in manifest order, fields in that order. Empty content is `[]`. Receipt IDs, parent IDs, exclusion reasons, revision annotations, and acquisition context are not source content.
3. `SnapshotID = H("snapshot", UTF8(SourceID), Manifest)`. This commits the complete canonical custody manifest. Receipt changes can change SnapshotID without changing SourceID.
4. `CollectionID = H("collection", UTF8(SnapshotID), context)`, where context is compact Go JSON with fields `adapter`, `workspace_uri`, `invocation_id` in that order, including empty strings. It is deterministic, not a generated UUID. Reusing identical context intentionally gives identical collection identity; callers wanting distinct acquisitions must supply distinct invocation IDs.
5. `LegacyManifestID = SHA256(Manifest)` in the same digest string format, for explicit compatibility with `BindSnapshotTrust`, NOT an alias for SnapshotID.

Go JSON string escaping is part of v1 (including HTML escaping); no Unicode, URI, path or whitespace normalization beyond existing manifest validation is implied. A different encoding requires another policy version.

The result includes `Policy`, all four IDs, the canonical `Manifest`, copied revision annotation, and acquisition value. Manifest and revision storage are independently owned. IDs describe build-time bytes: mutating returned Manifest does not update IDs; rebuild before using changed bytes.

## Reconciliation, not equivalence

- Historical `graph.SnapshotIdentity(manifest, canonicalArtifactReceipts)` has domain `lsp-trace:source-snapshot:v1` and two length-framed components. It incorporates admitted artifact receipts and is NOT this policy's SnapshotID.
- Historical `graph.SnapshotBindingIdentity` remains its own three-component construction.
- Historical source `BindSnapshotTrust` uses raw SHA256(manifest), exposed here only as LegacyManifestID.
- V2/V3 canonical graph bytes and their existing regression fixtures remain unchanged. No identity is rewritten, inferred equivalent, or accepted as another policy's authority key.

Downstream trust and receipt joins must explicitly choose the policy and the particular identity field. A trust receipt for LegacyManifestID must not authenticate SnapshotID without separately provisioned authority for that exact SnapshotID.

## Boundaries

SourceID identifies only included acquired path/content pairs, not a whole repository or all dependencies. The existing custody manifest stores excluded decision ID/state/reason, not excluded content digests; changing excluded bytes alone therefore changes none of these IDs. Changed exclusion reasons do change SnapshotID. Completeness, omission detection beyond supplied manifest accounting, receipt authenticity, authority provisioning and provider semantics belong to other frames.

Across adapters, identical included paths/bytes give the same SourceID. Identical complete manifests also give the same SnapshotID. Changing receipt provenance through a different receipt ID changes SnapshotID but not SourceID. Changing acquisition context alone changes only CollectionID. Adding/changing a revision annotation changes none of the IDs; it does not prove that revision produced these bytes.
