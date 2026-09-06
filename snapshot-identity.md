# Snapshot identity claim

Owner: snapshot-identity agent; baseline 4969599. Proposed bounded opt-in policy, not globally adopted.

Coordination correction: this committed `snapshot-identity.md` is the authoritative claim; parent transfers it to the shared namespace. Earlier shared copy is stale. No subsequent outside-worktree writes.

## Intended files/symbols
- NEW internal/source/identity_policy.go: IdentityPolicyV1, IdentityRequest, IdentityResult, RevisionAttestation, AcquisitionContext, BuildIdentity.
- NEW internal/source/identity_policy_test.go: assertion-specific contract, perturbation and legacy reconciliation tests.
- NEW docs/snapshot-identity-policy.md: exact opt-in construction and compatibility limits.
- Existing internal/graph/snapshot_identity.go and internal/source/snapshot_trust_bridge.go are read-only compatibility references; no existing public bytes changed.

## Proposed interface (dependency contract)
`BuildIdentity(IdentityRequest) (IdentityResult, error)` in package source.
Request: Receipts []ManifestReceipt; Decisions []ManifestDecision; Revision *RevisionAttestation; Acquisition AcquisitionContext.
RevisionAttestation: System, Revision strings (metadata claim, not authenticated authority).
AcquisitionContext: Adapter, WorkspaceURI, InvocationID strings.
Result: Policy string; SourceID, SnapshotID, CollectionID, LegacyManifestID strings; Manifest []byte; Revision *RevisionAttestation; Acquisition AcquisitionContext.
IMPORTANT join rule: ManifestReceipt.Digest MUST be the acquired-byte content digest (`Receipt.ContentIdentity.Digest`), never the digest of provenance-bearing canonical receipt JSON. ManifestReceipt.ID may reference the canonical receipt digest. Exclude unreadable inputs with explicit decisions; they cannot claim source content. The API validates digest syntax/accounting, not independent truth of supplied bytes. Adapter-context changes alone preserve source/snapshot, but differing receipt IDs/provenance change SnapshotID while SourceID remains stable.
SourceID hashes canonical included path/digest pairs only; SnapshotID binds SourceID and complete canonical custody manifest (receipts/decisions); CollectionID binds SnapshotID and acquisition context. Revision attestation is copied metadata, excluded from all IDs. LegacyManifestID remains exact SHA256(manifest), explicitly distinct from graph.SnapshotIdentity(manifest, artifactReceipts). No automatic aliasing or trust admission.

## Governing goal and dimensions
Separate content identity, receipt-bearing snapshot identity, acquisition identity and optional Git revision; preserve historical V2/V3 and bridge IDs. Cover ordering, changed path/bytes/receipt/decision, cross-adapter stable source/snapshot but distinct collection, Git absence/presence, immutable returned data and invalid manifests.

## Enforcement sequence
Baseline live: `go test ./internal/graph ./internal/source` => 153 passed in 2 packages. Run mandated Bar build; assertion-specific RED with incomplete implementation; minimal GREEN; independently perturb guarded dimensions; focused and full tests; commit. Receipt/trust/composition owners consume additive BuildIdentity; do not change bridge or legacy graph policy. Pending joins belong to parent.

## Artifacts and test receipts

Implemented the three new files listed above, plus this local claim. Exact policy preimages, field semantics, caller requirements, and non-equivalence with historical constructors are documented in `docs/snapshot-identity-policy.md`. No existing production or test file changed.

- Mandated `bar build make witness ground gate falsify atomic --subject ...` executed before implementation. Generated token-help requests were not followed because the assigned instruction expressly prohibited token discovery; substantive generated workflow applied.
- Assertion-specific RED on empty API stub: `go test ./internal/source -run TestIdentityPolicyContract -v` => 0 passed, 16 failed, including 15 named subtest failures (legacy manifest, order, bytes, path, receipt, decision, adapter, workspace, invocation, Git, excluded bytes, immutability, accounting, revision validation, context validation).
- GREEN: `go test ./internal/source ./internal/graph -count=1` => 175 passed in 2 packages, including unchanged historical V2/V3 canonical-byte fixtures.
- Race: `go test -race ./internal/source -count=1` => 50 passed.
- Fixed source/snapshot/collection vectors independently generated using Python JSON + big-endian framing + SHA256, then checked in Go. Additional lineage/exclusion-reason, length-framing, revision-change and real CanonicalizeReceipt cross-adapter tests pass.
- Independent destructive perturbations, each restored before the next: drop CollectionID context => ASSERT_IDENTITY_ADAPTER fails; drop SnapshotID manifest => ASSERT_IDENTITY_RECEIPT fails; drop SourceID digests => ASSERT_IDENTITY_BYTES fails. No perturbation switches or weakened checks retained.
- First full suite: 3373 passed, 2 failed, 5 skipped. Both failures were the integratedconformance dirty-path ownership assertion (new docs path), which reads `git status`, not a persisted ownership registry. Shared guard untouched. Clean-commit rerun: `go test ./... -count=1` => 3375 passed in 41 packages. No guard bypass or ownership edit.
- `git diff --check` passed.

## Derivation

Governing goal: provide an explicit opt-in, versioned identity policy connecting included source bytes, receipt-bearing snapshots and acquisition collections without changing historical identifiers.

Dimensions: deterministic canonical ordering; separate path/content, receipt/lineage/decision, and acquisition-context commitments; optional revision annotations outside identity and trust; immutable owned manifest/revision data; independently varied source/receipt/context inputs; unchanged V2/V3 bytes and historical graph/source formulas.

Enforcement: existing manifest validation/accounting precedes domain-separated length-framed SHA256; named assertion-specific RED precedes implementation; GREEN, independently encoded fixed vectors, cross-adapter receipt construction and three destructive perturbations validate distinct boundaries; focused/race suites and clean-commit full suite gate handoff.

Remaining limits: this is an implemented policy proposal, not global adoption. Callers must map acquired-byte digests (not receipt JSON hashes), supply actual-read evidence and choose the exact identity field for independently provisioned trust. Excluded bytes are not committed by the historical manifest. Optional revision annotation proves no Git authority. Parent owns joins into actual acquisition/execution/publication/CLI/MCP/offline; no deployment or Program A/B acceptance is claimed.
