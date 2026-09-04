# External provider qualification

Baseline: `158e85f17d969cee94037bf0555b6504420f385d`

Run from `qualification/provider-qualification`:

```sh
npm ci --ignore-scripts
npm run qualify
npm test
```

The deterministic `report.json` records one executed outcome per candidate:

- Ember Template Compiler 7.2.0: **SCOPED_ROLE** `template-syntax-ast`. Its internal `_preprocess` entrypoint parsed the extracted bounded `<template>` body and returned exact locations for callback passage and argument reads. This is not a stable public API claim.
- Glint 1.5.2: **BLOCKED**. `loadConfig` could not find a Glint configuration for the isolated qualification workspace, so no Glint semantic claim is inferred.
- Tree-sitter 0.21.1 with `tree-sitter-typescript` 0.23.2: **SCOPED_ROLE** `typescript-concrete-syntax`. It parsed one exact `call_expression`; syntax alone does not establish symbol identity or framework semantics.
- TypeScript service 5.9.2: **PASS** for `typescript-symbol-definition`. `createLanguageService` resolved the exact `leaf` call to its definition in the bounded fixture.

Every result is static source evidence only. It prohibits claims of runtime execution, callback invocation from passage, repaint, feature identity, and whole-source completeness. A scoped role is not general provider qualification, and BLOCKED is never promoted from package presence or another provider's evidence.
