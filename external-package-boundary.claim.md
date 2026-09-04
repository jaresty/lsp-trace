# External Package Boundary claim

Status: CLAIMED
Baseline: `f5362543ac86e1355d51b899d3d482e7ae267343`
Frame: External Package Boundary

## Goal

Remove Ember/Glint framework-specific command and provider implementation from core Go ownership and core GoReleaser builds. Establish `providers/ember-glint` as a separately installable, independently versioned repository-local package boundary with no import edge from core Go.

## Claims read before mutation

All tracked `*.claim.md` files at the baseline were read before this claim was created. In particular:

- `production-provider-executable.claim.md` owns the core Go command and package this frame removes.
- `script-symbol-extraction.claim.md`, `template-observation-extraction.claim.md`, and `cross-document-custody-mapping.claim.md` reserve reusable JavaScript prototype ownership without assigning analyzer or protocol-command implementation to this frame.
- `frozen-glint-qualification.claim.md` owns qualification-only Glint evidence and is not moved into the provider package by this frame.

## Exact ownership

- Remove `cmd/ember-glint-provider/`.
- Remove `internal/emberglintprovider/`.
- Narrow `.goreleaser.yaml` so core builds and archives contain no Ember/Glint provider binary or assets.
- Add `providers/ember-glint/README.md` and package metadata/versioning files sufficient for independent installation.
- Preserve only reusable prototype JavaScript source whose ownership does not overlap sibling analyzer or protocol frames, by moving or copying it under `providers/ember-glint/` if such source exists at this baseline.
- Add structural boundary guards under the existing integrated-conformance test ownership surface.
- Maintain this claim with red/green evidence.

## Required assertions

- `ASSERT_EXTERNAL_BOUNDARY_CORE_GO_NO_EMBER_GLINT_IMPORTS`
- `ASSERT_EXTERNAL_BOUNDARY_CORE_GO_NO_SEMANTIC_MATCHERS`
- `ASSERT_EXTERNAL_BOUNDARY_PROVIDER_PACKAGE_INSTALLABLE_VERSIONED`
- `ASSERT_EXTERNAL_BOUNDARY_CORE_ARCHIVES_EXCLUDE_PROVIDER`

## Exclusions

This frame does not implement an analyzer, protocol command, generic provider conformance, configured inventory, readiness state machine, production qualification, or release policy. It does not move sibling-owned qualification evidence or claim sibling analyzer/protocol behavior.

## Enforcement sequence

1. This claim is the first repository mutation.
2. Add committed structural guards and observe assertion-specific red results against the baseline ownership leak.
3. Remove the core Go implementation and release target.
4. Establish the minimal external package metadata/documentation boundary and preserve only non-overlapping reusable prototype source.
5. Run the identical guards green, focused/full tests, release configuration checks, and diff hygiene.
6. Record evidence, commit all changes, and leave the worktree clean.
