# Production Qualification and Release Policy claim

Status: CLAIMED
Baseline: `f5362543ac86e1355d51b899d3d482e7ae267343`
Frame: Production Qualification and Release Policy

## Goal

Define and guard a framework-neutral production lifecycle qualification that accepts the absolute executable path of an independently installed provider, retain the first real Ember/Glint external-provider witness, document installation and host registration, and make release admission depend on retained real qualification without bundling any provider analyzer in core archives.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-external-provider-f536254-v1/FRAMEWORK.md`
- `b05-lifecycle-release-admission.claim.md`
- `bootstrap-provider-declarations.claim.md`
- `cross-document-custody-mapping.claim.md`
- `frozen-glint-qualification.claim.md`
- `observation-adaptation.claim.md`
- `production-acceptance-activation.claim.md`
- `production-provider-executable.claim.md`
- `provider-admission-semantic-binding.claim.md`
- `script-symbol-extraction.claim.md`
- `template-observation-extraction.claim.md`

## Exact ownership

This frame owns tests, documentation, qualification/release scripts, retained qualification evidence produced from the real independently installed Ember/Glint package, and this claim only. It does not own or modify core Go implementation, provider implementation, fake providers, or testdata provider assets.

## Required policy and guards

1. Provider installation is independent of the core release and registration supplies an absolute executable path; lsp-trace does not download or discover it.
2. Generic production lifecycle qualification accepts a host-supplied externally installed executable path and does not assume a bundled analyzer.
3. The retained Ember/Glint witness is generated only from the real package path, never a fake or testdata executable.
4. The witness exercises MCP request → provider subprocess → normalized observations → semantic adapter → graph-v4 and preserves provider/protocol/request/adapter identity, complete contributor observation IDs, immutable original custody, and original anchors.
5. Replaying the same qualified input produces deterministic retained bytes and digest.
6. Unsupported Glint operations remain explicitly `BLOCKED` or unsupported and are never rewritten as empty relations, success, or inferred absence.
7. Omitting provider relations starts no provider and preserves exact historical graph-v3 output.
8. Core release archives contain no provider package or analyzer assets.
9. Release admission requires at least one retained passing real external-provider qualification, but never requires a provider executable or analyzer to be bundled into the core archive.
10. Baseline guards may remain compiling assertion-specific red until the owning implementation frames land.

## Enforcement sequence

1. Commit this claim as the first mutation.
2. Add persistent assertion-specific tests and release checks for each policy dimension.
3. Run focused guards on baseline and retain their exact assertion-specific red outcomes.
4. Add only documentation and qualification/release scripts needed to state and enforce the policy; do not implement core/provider behavior.
5. Verify changed paths remain within tests, docs, qualification/release scripts, retained real evidence, and this claim; commit cleanly and leave a clean worktree.

## Baseline and verification evidence

- Before policy mutation, `go test ./internal/qualificationpolicy -run TestProductionQualificationAndReleasePolicy -count=1 -v` reported 0 passed and 1 failed; the output independently named missing installation, absolute-path, no-discovery, and generic-qualifier assertions.
- After test/docs/release-policy changes, the same focused guard reported 1 passed; the full qualification-policy package reported 15 passed.
- `./scripts/test-release-evidence.sh` passed all 14 release contract assertions, including core archive exclusion.
- `sh -n scripts/qualify-external-provider.sh scripts/release-check.sh scripts/test-release-evidence.sh` passed with no diagnostics.
- The generic lifecycle test without an external path reported the assertion-specific red `ASSERT_EXTERNAL_PROVIDER_ABSOLUTE_PATH: LSP_TRACE_EXTERNAL_PROVIDER_PATH is required`.
- The hermetic release check intentionally remains red at `ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_QUALIFICATION` because no real independently installed external provider path or retained witness is available in this frame. No fake, testdata, repository-local, or synthesized evidence was substituted.
