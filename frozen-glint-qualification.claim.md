# Frozen Glint Qualification Workspace claim

Status: CLAIMED
Baseline: 0702d12
Frame: Frozen Glint Qualification Workspace

## Goal

Own a minimal frozen valid Ember/Glimmer plus Glint workspace using repository-pinned dependencies and supported configuration, then retain deterministic evidence from documented/supported Glint APIs for typed template resolution, exact original-source ranges, virtual-document mappings, immutable pinned-file operation, and explicit partial/failure outcomes.

## Claims and evidence read before mutation

- `/tmp/bar-frame-work-lsp-trace-ember-provider-0702d12-v1/FRAMEWORK.md`
- `bootstrap-provider-declarations.claim.md`
- `observation-adaptation.claim.md`
- `production-acceptance-activation.claim.md`
- `provider-admission-semantic-binding.claim.md`
- `qualification/provider-qualification/package.json`
- `qualification/provider-qualification/package-lock.json`
- `qualification/provider-qualification/qualify.mjs`
- `qualification/provider-qualification/qualify.test.mjs`
- `qualification/retained/provider-qualification/README.md`
- `qualification/retained/provider-qualification/report.json`
- `qualification/B05.md`
- `qualification/retained/b05/qualification-evidence.json`

The starting retained Glint result is `BLOCKED`: package presence is not support, and the existing operation failed because the isolated workspace had no Glint configuration. B05 separately leaves external adapter/provider availability blocked unless retained provider evidence passes.

## Exact ownership

- `frozen-glint-qualification.claim.md`
- minimal qualification workspace/configuration/fixtures under `qualification/provider-qualification`
- qualification runner and assertion-specific tests under `qualification/provider-qualification`
- deterministic retained report and its explanatory README under `qualification/retained/provider-qualification`

No provider executable, production collector/runtime, MCP lifecycle implementation, release packaging, Go Ember/Glimmer parser, or NAIS artifact is owned or modified.

## Required operation classifications

Each operation is independently classified as exactly `PASS`, `SCOPED_ROLE`, or `BLOCKED`, with exact executed evidence rather than package-presence inference:

1. valid frozen Ember/Glimmer plus Glint workspace and supported configuration;
2. deterministic typed template resolution;
3. exact original-source ranges;
4. virtual-document mapping in both relevant coordinate directions;
5. operation against immutable pinned input files without source mutation;
6. explicit partial and failure reporting that does not collapse unsupported, unavailable, failed, partial, bounded, empty, or transport-failed outcomes.

Unsupported documented APIs remain `BLOCKED` with the exact missing export, thrown diagnostic, or observed mismatch. Internal or unsupported entrypoints may be recorded only as `SCOPED_ROLE`, never upgraded to general support.

## Claim ceiling

No result establishes runtime execution, callback invocation from passage, task execution, repaint, feature identity, whole-source completeness, relation absence from unknown evidence, or production-provider readiness. Original-source coordinates remain authoritative; generated or virtual coordinates are subordinate and explicitly mapped.

## Enforcement sequence

1. This claim is the first repository mutation after reading every tracked claim and existing qualification/B05 evidence.
2. Add assertion-specific red tests for the missing frozen workspace and operation-level evidence; run them and retain named failures.
3. Probe only documented/supported Glint APIs from the repository-pinned dependency version; do not infer support from installed package names.
4. Implement the smallest qualification-only workspace, runner, report, and tests one observable change at a time.
5. Run the focused qualification tests and qualification command, verify deterministic replay and immutable fixture digests, then run applicable full repository tests.
6. Update this claim and retained README/report with exact evidence, verify no forbidden paths changed, commit cleanly, and leave a clean worktree.

## Red evidence

`node --test qualification/provider-qualification/frozen-glint.test.mjs` reported 0 passing and 1 failing test against the retained v1 report. The assertion-specific failure was `ASSERT_GLINT_VALID_FROZEN_WORKSPACE: report schema`, with actual `lsp-trace.provider-qualification.v1` and expected `lsp-trace.provider-qualification.v2`.

## Qualification evidence

- `workspace`: **PASS** — supported `ember-template-imports` configuration loaded with zero diagnostics using exact same-version environment packages.
- `typed-template-resolution`: **SCOPED_ROLE** — root-exported unstable `analyzeProject` returned a language server whose `getDefinition` mapped template `this.itemCount` to its exact typed getter declaration.
- `original-source-ranges`: **SCOPED_ROLE** — the analysis transform manager returned the exact original `.gts` token range.
- `virtual-document-mappings`: **SCOPED_ROLE** — original-to-virtual and virtual-to-original ranges round-tripped exactly, with the virtual `.ts` document retained only by URI, offsets, and digest.
- `immutable-pinned-files`: **PASS** — fixture, configuration, and lockfile SHA-256 values were equal before and after analysis.
- `partial-failure-reporting`: **BLOCKED** — the `@glint/core` 1.5.2 public root contract exposes no structured outcome vocabulary separating unsupported, unavailable, partial, bounded, empty, and transport-failed results. Diagnostics, arrays, optional values, and thrown errors are insufficient; package presence and internal files do not upgrade support.

Two consecutive runs produced byte-identical output and report SHA-256 `0b384f1e74a52a3505426ccf4ff5140945c2f04e06dea86b44e90bb55f1004fe`. Focused `npm test` reported 2 passing tests.

## Final verification

- Focused qualification: `npm test` — 2 passing tests.
- Existing B05 qualification: `go test ./internal/b05qualification -count=1` — pass.
- Narrow ownership overlap: `go test ./internal/integratedconformance -run 'TestDisabledIntegratedConformance/ASSERT_PACKAGE_OWNERSHIP_ONLY' -count=1 -v` — `Go test: 2 passed in 1 packages` after admitting only this claim and qualification workspace/report prefixes.
- Full repository: `go test ./... -count=1` — `Go test: 3231 passed in 40 packages`.
- `git diff --check` — pass.
- No provider executable, production collector/runtime, MCP lifecycle implementation, or release packaging file was changed.
