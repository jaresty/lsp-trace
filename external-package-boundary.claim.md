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

## Red evidence

`go test ./internal/integratedconformance -run TestExternalPackageBoundary -count=1 -v` reported `Go test: 0 passed, 5 failed in 1 packages`. The output independently named all four required assertion identities; the parent test supplied the fifth failure. The baseline contained a core import of `lsp-trace/internal/emberglintprovider`, framework implementation identities, no external package metadata, and an Ember/Glint GoReleaser target.

A minimization perturbation temporarily removed the provider README. The package-boundary assertion failed and the README was restored, proving the metadata-only package cannot be reduced below its documented boundary while satisfying the guard.

## Implementation

- Removed the core Go command at `cmd/ember-glint-provider/` and implementation package at `internal/emberglintprovider/`.
- Removed the Ember/Glint provider build from `.goreleaser.yaml`; only the two core binaries remain.
- Removed `internal/b05lifecycle/`, whose Go tests and fixtures required and qualified the framework-specific production provider from core ownership.
- Added independently versioned npm metadata and installation documentation under `providers/ember-glint/`.
- Left sibling-owned analyzer, template, custody, protocol, and qualification prototypes in their existing ownership locations rather than duplicating them.
- Added structural guards for core import absence, core implementation matcher absence, package metadata, and core archive exclusion.
- Added no analyzer, protocol command, generic conformance, inventory, readiness, or production qualification implementation.

## Verification evidence

- Boundary plus ownership focus: `go test ./internal/integratedconformance -run 'TestExternalPackageBoundary|TestDisabledIntegratedConformance/ASSERT_PACKAGE_OWNERSHIP_ONLY' -count=1 -v` — `Go test: 7 passed in 1 packages`.
- Full repository after one isolated transient managed-process retry: `go test ./... -count=1` — `Go test: 3236 passed in 40 packages`.
- External package payload: `npm pack --dry-run` — `@lsp-trace/ember-glint-provider@0.1.0`, two files (`README.md`, `package.json`), no implementation or provider assets.
- `git diff --check` — pass.
- `goreleaser check` was unavailable because the executable is not installed; the committed structural archive guard validates the relevant configuration boundary.

## Remediation evidence at HEAD `25319f8`

### RED

Packed qualification attempt 09 exited `1` before emitting a response frame because `invokes-task-positive.gts` imports `@glimmer/component` while the isolated TypeScript program mapped only Ember Concurrency and Warp Drive declarations; the resulting TS2307 diagnostic escaped the process. After adding the missing declaration, attempt 09 reached admission and exposed an exact document-custody mismatch: the strict collector declared document ID `original` while the analyzer emitted `qualification-seed`.

### GREEN

The provider package tests pass 71/71. A fresh packed/offline install now executes the exact strict `INVOKES_TASK` collector request with exit 0, empty stderr, exactly one valid `Content-Length` frame, `COMPLETE_WITHIN_BOUNDS` coverage, and one `AbstractTask.perform` declaration-qualified observation. The existing same-spelling, unresolved, `any`, and `unknown` negative guards remain green. Canonical B05 qualification passes attempts 01–12; attempt 13 stops on an independently exposed TRIGGERS_RELOAD contract conflict and is not hidden or inferred around.

## Derivation

1. Pin a fixture-compilation-only `@glimmer/component@2.1.1` declaration using the lockfile integrity and exact declaration SHA-256; the empty superclass declares no task or reload behavior.
2. Include that declaration in package verification, exact packed-content assertions, and TypeScript `paths`/program roots so isolated compilation has no ambient dependency.
3. Preserve declaration-qualified `INVOKES_TASK` inference exclusively through the pinned Ember Concurrency declaration and existing negative boundaries.
4. Align the source-constrained analyzer's original document ID with the strict collector's immutable `original` document record; do not weaken custody validation.
5. Add the already pinned `@ember-data/model` compatibility path to the Warp Drive declaration after attempt 13 showed TS2307, then stop when the next result exposed conflicting TRIGGERS_RELOAD semantics (`Person.reload()` versus the analyzer/provenance `UserImportModel[]` loop contract). No diagnostics were suppressed, exceptions hidden, lifecycle handling weakened, or fallback inference added.

### Verification

- Provider package: `npm test` — 71 passed.
- Canonical `./scripts/qualify-b05-frame6.sh` — attempts 01–12 passed; attempt 13 stopped with `typed provider result contains no accepted observations` due to the documented TRIGGERS_RELOAD contract conflict.
- Documentation: `./scripts/check-docs.sh` — pass.
- Go: `go test ./...` and `go test -race ./...` — 3314 passed in 41 packages for each; `go vet ./...` — pass.
- Release: `./scripts/release-check.sh` — `RELEASE CHECK PASS`.
- GoReleaser: `goreleaser release --snapshot --clean` — six platform archives built successfully.

## Generic caller-project JavaScript boundary at HEAD `40d98be`

### RED

`ASSERT_P2B_P3A_UNCHECKED_JS_SEMANTIC_UNCERTAINTY_NEVER_BECOMES_ABSENCE` executed against a caller-owned `checkJs:false` fixture and reached its own identity assertion: the loader returned only the two compiler-owned `INVOKES_TASK` identities and omitted the compiler-owned `TRIGGERS_RELOAD` identity. The initial missing-`typescript` setup error was explicitly discarded and is not the witness.

### GREEN

Caller-project JavaScript analysis now overlays `allowJs:true`, `checkJs:true`, and `noEmit:true` in memory while preserving the parsed caller project options and leaving project files unchanged. Compiler diagnostics return `BLOCKED` with `UNAVAILABLE` coverage and exact TS code/path/line/column/message text. Any/unknown receiver uncertainty returns explicit `BLOCKED`; bounded checked same-spelling controls remain `EMPTY`; admitted relations still require compiler-owned declaration identity and caller-local package custody. No spelling, regex, marker, or target-name fallback was added.

The focused assertion and provider package pass 87/87. Canonical B05 passes all 24/24 packed real-MCP attempts plus exact matrix and physical-provider checks. `./scripts/check-ci.sh` passes format, full test, vet, build, Python, shell, release, clean-tree policy, and GoReleaser dry-run gates. The older `qualification/source-constrained-synthetic` verifier remains blocked before its guard matrix by its pre-existing protected-baseline mismatch `B05 v2 bytes changed`; no baseline was rewritten and no pass is claimed.

### Native Market View replay

A disposable detached clone of read-only `/Users/schwa/dev/nais/03_repos/market-view-ui` at pinned commit `326718ae733cb26097bd30246276cecd371a4e79` returned `BLOCKED`/`UNAVAILABLE` for both `app/services/uploads.js` and `app/models/user-import.js`. No declaration identities resolved. Exact TS2792 diagnostics named unresolved caller dependencies: `@ember/service`, `@glimmer/tracking`, and `ember-concurrency` for uploads; `@ember/string`, `@warp-drive/legacy/model`, and `moment` for user-import. The replay therefore no longer collapses semantic uncertainty to bounded `EMPTY`.

## Derivation

1. A containing config with `checkJs:false` is not evidence that JavaScript relation absence has been semantically checked; overlay checking for the analyzed root without mutating caller files.
2. Preserve caller module resolution, `baseUrl`, `paths`, and `types` by spreading parsed options before the three analysis-only overrides.
3. Admit INVOKES_TASK and TRIGGERS_RELOAD only through TypeScript symbol/declaration identity plus package custody; apply the same caller-local custody policy to both relation families.
4. Project compiler diagnostics and unsafe any/unknown receiver identity as explicit `BLOCKED`/`UNAVAILABLE`, retaining exact diagnostic provenance instead of throwing or returning absence.
5. Preserve `EMPTY` only for successfully checked bounded sources whose known compiler identities establish that no requested relation exists.

## Generic `TaskForAsyncTaskFunction` admission at HEAD `90408d4`

### RED

`ASSERT_GENERIC_TASK_SIX_DECLARATION_BACKED_RECEIVERS_QUALIFY` ran through the real source-constrained checker against a constructed caller package with `ember-concurrency@5.2.0`, public `TaskForAsyncTaskFunction<Fn>`, and inherited `AbstractTask.perform`. Before production mutation the result was assertion-specific `EMPTY` instead of `COMPLETE` because caller admission required a receiver/base named `Task`.

### GREEN

`INVOKES_TASK` now requires a non-`any`/non-`unknown` receiver, a compiler-resolved member symbol and declaration, exact `perform`/`AbstractTask.perform` member-parent identity, physical containment in the nearest caller `node_modules` package root, and manifest-authenticated `ember-concurrency@5.2.0` provenance. It does not use public alias text, structural method presence, fallback package names, or arbitrary receiver inheritance. Unsafe requested candidates are never admitted; a wholly unsafe seed remains `BLOCKED`, while a mixed seed retains only independently safe declaration-backed observations. `TRIGGERS_RELOAD` is unchanged and PROGRAM_B remains unadmitted.

The persistent modern fixture qualifies six explicit generic receiver call sites and rejects same-spelling local, wrong declaration parent, wrong package name/version, symlink escape, unknown, unresolved, two `any` task calls, and one `any` reload. Provider tests pass 89/89.

### Native Market View replay

A disposable shared clone of read-only `/Users/schwa/dev/nais/03_repos/market-view-ui` was detached at `326718ae733cb26097bd30246276cecd371a4e79`. `corepack pnpm@10.32.1 install --frozen-lockfile --offline --ignore-scripts` reused 1,798 cached packages and installed `ember-concurrency@5.2.0`. Because the pinned `jsconfig.json` omits Ember's ambient virtual-module declarations, the unmodified-config replay stopped at exact TS2307 for `@ember/service`; a disposable analysis-only `types: ["ember-source/types"]` overlay then produced `COMPLETE`, exactly six `INVOKES_TASK`, zero `TRIGGERS_RELOAD`, and byte-identical repeated results.

All six observations resolve to `package=ember-concurrency@5.2.0`, physical path `node_modules/.pnpm/ember-concurrency@5.2.0_@babel+core@7.29.0_@glint+template@1.7.4/node_modules/ember-concurrency/declarations/index.d.ts`, symbol `AbstractTask.perform`, declaration SHA-256 `1ca4672f7e39b0e3c16f099b501d3b8000ece297764398634770a56c15ce115a`, and compiler chain `TaskForAsyncTaskFunction→Task→AbstractTask.perform`. Canonical result digest was `dcee273bf61c37eefe4b9d05f6a25c5115e46167ca687af5377e40f29d0cc0ed`. The two `any` task calls and one `any` reload emitted no observations.

## Derivation

1. Receiver aliases are presentation-level names; exact compiler member declaration identity and authenticated package custody are the admission authority.
2. Exact parent/member identity rejects same-spelling structural methods and arbitrary inheritance without requiring one public alias or base name.
3. Realpath containment plus nearest package manifest name/version rejects fallback package labels, wrong versions, wrong roots, and symlink escapes.
4. Unsafe evidence is candidate-local: it cannot qualify, cannot erase independently authenticated calls in the same source, and still blocks a seed with no safe observation.
5. The unchanged reload declaration predicate continues to require `Model.reload` and authenticated Warp Drive custody; no PROGRAM_B admission follows.

### Verification

- Persistent RED then focused GREEN: `ASSERT_CALLER_OMITTED_MODULE_RESOLUTION_USES_NODE_COMPATIBLE_POLICY`.
- Provider package: `npm test --prefix providers/ember-glint` — 89/89.
- Caller retained MCP guard: `./scripts/test-caller-project-javascript-qualification.sh` — pass.
- B05 historical/current: `./scripts/test-b05-qualification.sh` — immutable historical blob and current 24-attempt release selection pass.
- Go: `go test ./... -count=1` — 3321/3321; `go test -race ./...` rerun after disposable replay cleanup.
- CI/release: `./scripts/check-ci.sh` and `./scripts/release-check.sh` — pass, including parity, omission, archive, GoReleaser configuration, and release builds.
