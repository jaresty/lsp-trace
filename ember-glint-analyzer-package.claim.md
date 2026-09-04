# Ember/Glint Analyzer Package claim

Status: CLAIMED
Baseline: f5362543ac86e1355d51b899d3d482e7ae267343
Frame: Ember/Glint Analyzer Package

## Goal

Move and adapt the existing template extraction, script extraction, cross-document custody/mapping, and frozen Glint qualification work into an independently installable analyzer package under `providers/ember-glint`. The package will emit only independently supported observations, preserve exact original anchors and optional virtual mappings, and report missing Glint configuration as explicit `UNAVAILABLE`/`BLOCKED` rather than an empty successful result.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-external-provider-f536254-v1/FRAMEWORK.md`
- `bootstrap-provider-declarations.claim.md`
- `frozen-glint-qualification.claim.md`
- `observation-adaptation.claim.md`
- `production-provider-executable.claim.md`
- `script-symbol-extraction.claim.md`
- `template-observation-extraction.claim.md`
- `cross-document-custody-mapping.claim.md`
- `provider-admission-semantic-binding.claim.md`
- `production-acceptance-activation.claim.md`
- `b05-lifecycle-release-admission.claim.md`

## Exact ownership

- `ember-glint-analyzer-package.claim.md`
- analyzer modules under `providers/ember-glint/`
- analyzer fixtures under `providers/ember-glint/fixtures/`
- package-local dependency manifests and lockfiles under `providers/ember-glint/`
- focused package tests under `providers/ember-glint/`

Explicitly unclaimed: core Go, the framing executable, provider registry or inventory, release policy or packaging, MCP lifecycle/composition, and NAIS artifacts.

## Retained behavior

1. The package is independently installable and owns pinned Ember compiler, Glint, TypeScript service, and Tree-sitter dependencies only within the FRAMEWORK qualification ceilings.
2. Template observations use qualified Ember compiler AST evidence and preserve exact original-source anchors.
3. Script-symbol observations use the pinned TypeScript service for definitions and Tree-sitter only for exact original concrete-syntax anchors.
4. Frozen Glint analysis advertises and emits only relations independently supported by executed qualified operations; scoped or blocked capabilities are not upgraded.
5. Original anchors remain authoritative and exact; virtual mappings are optional, subordinate, deterministic, and retained when available.
6. Missing or invalid Glint configuration produces explicit `UNAVAILABLE`/`BLOCKED`, never an empty successful observation result.
7. Outputs and tests are deterministic and do not claim callback execution, task execution, runtime behavior, repaint, feature identity, whole-source completeness, or relation absence from unknown evidence.

## Enforcement sequence

1. This claim is the first repository mutation.
2. Add package-local assertion-specific deterministic tests against a compile-valid present-but-wrong analyzer surface.
3. Run the focused test command and retain assertion-specific red evidence before production implementation.
4. Move/adapt the smallest existing analyzer and frozen qualification surfaces into the independent package.
5. Re-run focused tests deterministically, inspect qualification ceilings and changed paths, and run applicable repository checks.
6. Record evidence here, commit all owned changes, and leave a clean worktree.

## Red evidence

`node --test providers/ember-glint/test/analyzer.test.mjs` against the compile-valid unavailable stub reported 0 passing and 4 failing tests. The result independently named:

- `ASSERT_ANALYZER_ADVERTISES_ONLY_INDEPENDENT_RELATIONS`
- `ASSERT_ANALYZER_PRESERVES_EXACT_ORIGINAL_ANCHOR_AND_OPTIONAL_MAPPING`
- `ASSERT_ANALYZER_MISSING_GLINT_CONFIG_BLOCKED_NOT_EMPTY`
- `ASSERT_ANALYZER_DETERMINISTIC_AND_RELATION_FILTERED`

## Implementation

- Added an independently installable package with exact pins for Ember 7.2.0, Glint 1.5.2, TypeScript 5.9.2, Tree-sitter 0.21.1, and Tree-sitter TypeScript 0.23.2.
- Adapted the existing Ember template AST extractor, TypeScript/Tree-sitter script definition extractor, cross-document custody helper, frozen Glint fixture/configuration, and supported Glint operation into package-local modules.
- Added a coordinator that advertises only `BINDS_ARGUMENT`, `SCRIPT_SYMBOL_DEFINITION`, and the frozen-operation-scoped `TYPED_TEMPLATE_DEFINITION` relation.
- Preserved delegated observation objects, exact original byte/range/text anchors, and optional subordinate virtual mappings without normalization.
- Added explicit `BLOCKED`/`GLINT_CONFIG_UNAVAILABLE` with `UNKNOWN` coverage for missing Glint configuration; this path is distinct from empty successful analysis.
- Retained Glint typed-template support as `SCOPED_ROLE` and made no runtime, callback, task, repaint, feature identity, whole-source completeness, or unknown-evidence absence claim.

## Verification evidence

- Focused coordinator green: 4 passing, 0 failing.
- Full package green, first run: 7 passing, 0 failing.
- Full package deterministic replay: 7 passing, 0 failing.
- Exact installed dependency census: `@glint/core` and both Glint environments 1.5.2, `ember-source` 7.2.0, `typescript` 5.9.2, `tree-sitter` 0.21.1, and `tree-sitter-typescript` 0.23.2.
- `git diff --check`: pass.
- Enumerated changed paths are confined to this claim and `providers/ember-glint/**`; no core Go, framing executable, registry, or release-policy path changed.
- Pre-commit full repository suite: 3243 passing, 3 failing, 2 skipped. Two integrated-conformance failures are the expected dirty-worktree ownership check observing this uncommitted claim; the remaining B05 lifecycle failure requires the production executable and release artifacts explicitly owned by other frames and forbidden here.
