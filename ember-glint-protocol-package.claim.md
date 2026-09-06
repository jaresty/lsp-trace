# Ember/Glint Protocol Package claim

Status: CLAIMED
Baseline: f5362543ac86e1355d51b899d3d482e7ae267343
Frame: Ember/Glint Protocol Package

## Goal

Create an independently runnable and installable external provider package under `providers/ember-glint` with executable ownership, strict bounded Content-Length transport, generic request and observation envelopes, stable `ember-glint@1` identity, provider-side analyzer custody and mapping, closed lifecycle outcomes, deterministic canonical logical bytes and digest, and honest capability metadata.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-external-provider-f536254-v1/FRAMEWORK.md`
- every tracked `*.claim.md` at baseline f536254

## Exact ownership

- `providers/ember-glint/**`
- `ember-glint-protocol-package.claim.md`

No core Go package or command, release policy, release archive, provider inventory, qualification artifact, or NAIS artifact is owned. Installation will be explicit and offline; the package will never download or implicitly discover analyzers or executables.

## Analyzer boundary

The protocol package may import narrow analyzer modules under `providers/ember-glint` when sibling frame files are present. It will not duplicate TypeScript, Ember template, Tree-sitter, Glint, custody, mapping, or semantic extraction rules. When sibling files are absent, it will define narrow injected interfaces and use test doubles only.

## Retained behavior

1. Accept exactly one bounded frame with one strict ASCII `Content-Length` header and exact CRLF framing; reject malformed, duplicate, oversized, truncated, trailing, or invalid JSON input.
2. Decode a generic versioned request schema and emit a generic versioned observation schema with stable provider identity `ember-glint@1`.
3. Keep analyzer invocation, custody handoff, and output-to-observation mapping inside the provider package while delegating analyzer semantics through narrow interfaces.
4. Report only a closed set of bounded lifecycle outcomes, preserving unsupported, unavailable, failed, partial, bounded, empty, and successful distinctions.
5. Produce deterministic canonical logical JSON bytes and a SHA-256 logical digest independent of Content-Length framing and input object insertion order.
6. Advertise only implemented relation/language/framework capabilities and immutable positive limits.
7. Supply an explicit package-local installation/executable entry point requiring no network access.
8. Remain independently runnable and testable without importing or adding any core Go package or command.

## Enforcement sequence

1. This claim is the first repository mutation.
2. Add committed assertion-specific tests and a compile-valid present-but-wrong package surface.
3. Run focused tests and retain assertion-specific red evidence before implementation.
4. Implement the smallest package behavior required by the guards.
5. Perturb each guarded dimension where practical, observe assertion-specific failure, restore, and observe pass.
6. Run package tests, syntax/package checks, repository tests applicable without downloading, forbidden-path diff checks, and `git diff --check`.
7. Commit the complete package and claim, then verify the worktree is clean.

## 2026-09-06 production correction

### Incident and RED

The installed provider was stale despite sharing package version `1.0.0`: its analyzer SHA-256 was `985bef...` and retained the `.ts|gts` guard, while clean HEAD `441b1c31936b35607d16363b484d9300ad4f9511` contained the `.js|ts|gts` analyzer and packed those bytes. Production therefore required reinstall rather than a Market View `.js` source change.

Before implementation, `ASSERT_STRICT_ANALYZER_THROW_IS_ONE_STRUCTURED_FAILURE` reproduced an escaping `diagnostic witness` exception, and `ASSERT_GJS_SOURCE_CONSTRAINED_RELATIONS_FAIL_CLOSED` reproduced the throwing `source-constrained JavaScript or TypeScript file seed required` guard. The focused run passed 17 and failed exactly these two assertions.

### GREEN and custody

Strict collection now uses the same analyzer-settlement containment as the generic path. Analyzer exceptions produce one valid framed response, zero observations, `UNKNOWN` coverage, established failure code `TRANSPORT_FAILED`, and process exit 0. Source-constrained `.gjs` input returns explicit `UNAVAILABLE` with reason `SOURCE_CONSTRAINED_INPUT_NOT_SUPPORTED`; it never becomes `EMPTY`, regex failure, or fallback relation evidence. Unsupported relations, analyzer failures, and missing/unsafe project dependencies remain distinct.

Mutable bytes under `1.0.0` made install identity ambiguous, so the independently versioned package and lockfile advance atomically to `1.0.1` while semantic provider identity remains `ember-glint@1` and retained historical evidence remains unchanged. Fresh `1.0.1` custody values are:

- tarball `lsp-trace-ember-glint-provider-1.0.1.tgz` SHA-256 `f1c472ae892e442e19f5f461d94e3cf4dccfdb86dc57d82d7f91660f952c908d`
- npm integrity `sha512-fDl33GigRQRv/TjX53Sjm1YWEYDB3FxunGnO2HVhYxnCpTVl+Fm797sbqLpnCjaDnkxA4Do6WDf9DYNqWsyPCw==`
- npm shasum `aeb5485fa41687be26641379af12d9c2a2b5acfc`
- analyzer SHA-256 `b98bb1e5bef7709dfd58203433221a9ecd28d423c7ce12c64e0215ed45a39528`
- executable SHA-256 `d1629def49e9777e5865a5f7d0969d7aadecef30a5c3087b2a29b70987a2861c`

The offline pack/install assertion checks tarball digest presence, npm integrity/shasum, manifest version and entry points, canonical installed analyzer realpath, packed/installed/source analyzer digest equality, and installed/source executable digest equality. Provider tests pass 91/91; caller retained qualification, B05 historical/current, Go 3342/3342, race 3342/3342, vet, release/schema/archive/MCP checks, and GoReleaser all pass.

## Derivation

1. Preserve immutable Market View custody; production byte drift is corrected by repacking and reinstalling the provider, never by changing caller `.js` source.
2. Convert analyzer rejection into data at the provider boundary so strict framing survives while retaining the established failure algebra.
3. Reject `.gjs` only for source-constrained `INVOKES_TASK`/`TRIGGERS_RELOAD` analysis unless compiler-owned Glint mapping can prove support; do not infer or fall back.
4. Distinguish install identity with package version `1.0.1` and verify content custody end to end so stale same-version bytes cannot qualify.
5. Preserve stable semantic identity `ember-glint@1`, protocol schemas, and historical evidence bytes.
