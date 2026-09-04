# External provider qualification

Baseline: `0702d12`

Run from `qualification/provider-qualification`:

```sh
npm ci --ignore-scripts
npm run qualify
npm test
```

The workspace pins `@glint/core`, `@glint/environment-ember-loose`, and `@glint/environment-ember-template-imports` at `1.5.2`, Ember at `7.2.0`, and TypeScript at `5.9.2`. `tsconfig.json` uses the supported `ember-template-imports` environment over one bounded `.gts` Glimmer component fixture.

The deterministic `lsp-trace.provider-qualification.v2` report records one independently classified Glint operation:

- **PASS** `workspace`: public `loadConfig` plus root-exported `analyzeProject` loaded the pinned environment and returned zero Glint diagnostics.
- **SCOPED_ROLE** `typed-template-resolution`: `analyzeProject(...).languageServer.getDefinition` resolved template `this.itemCount` to its exact typed getter definition. `analyzeProject` is explicitly documented in the package declaration as unstable, so this is not stable provider API qualification.
- **SCOPED_ROLE** `original-source-ranges`: the returned transform manager mapped the virtual token span to the exact original `.gts` token and range. The manager is obtained through the unstable analysis API.
- **SCOPED_ROLE** `virtual-document-mappings`: `getTransformedRange`, `getOriginalRange`, and `getTransformedContents` produced a deterministic subordinate virtual `.ts` document and exact bidirectional round trip. Original coordinates remain authoritative.
- **PASS** `immutable-pinned-files`: SHA-256 values for the fixture, `tsconfig.json`, and lockfile were identical before and after analysis.
- **BLOCKED** `partial-failure-reporting`: the `@glint/core` 1.5.2 public root contract exposes diagnostics, arrays, optional values, and thrown errors, but no structured status vocabulary distinguishing unsupported, unavailable, partial, bounded, empty, and transport-failed outcomes. Package presence and internal files do not upgrade this result.

Two consecutive qualification runs produced byte-identical console output and retained report SHA-256 `0b384f1e74a52a3505426ccf4ff5140945c2f04e06dea86b44e90bb55f1004fe`. `npm test` reported 2 passing tests.

The prior Ember Template Compiler, Tree-sitter, and plain TypeScript roles retain their original narrow ceilings. Every result is static source evidence only. Nothing here supports runtime execution, callback invocation from passage, repaint, feature identity, whole-source completeness, relation absence from unknown evidence, production-provider readiness, or release packaging.
