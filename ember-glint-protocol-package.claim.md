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
