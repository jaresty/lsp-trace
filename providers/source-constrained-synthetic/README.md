# Source-constrained synthetic provider

Non-production provider for source-constrained discovery evidence. It is independently installable with `npm pack`/`npm install` and has identity `source-constrained-synthetic-provider@1.0.0`.

The host must register the installed absolute executable and explicitly select this provider. It is never eligible for `auto`, never changes `ember-glint@1` capabilities, and emits only `PROVIDER_REPORTED`, `SOURCE_CONSTRAINED_SYNTHETIC`, `PROVISIONAL_DISCOVERY` evidence.

The package executes TypeScript 5.9.3 compiler/type-checker analysis against `.ts` seeds. It vendors pinned ember-concurrency 5.2.0 and @warp-drive/legacy 5.8.1 declarations, tsconfig, dependency integrity, declaration/config digests, and negative-boundary provenance.

`INVOKES_TASK` requires the checker-resolved `TaskForAsyncTaskFunction → Task → AbstractTask.perform` chain. `TRIGGERS_RELOAD` requires a `UserImportModel` element from a source-declared `UserImportModel[]` collection and the checker-resolved @warp-drive/legacy `Model.reload` member declaration. Exact source call ranges and checker/declaration identities are carried in protocol-retained anchors and endpoint IDs.

Comments and marker strings have no semantics. Same-spelling unrelated methods, `any` receivers, contradictory types, unsupported collection elements, and unqualified calls emit no relation. Source diagnostics, unknown receivers, and unresolved declarations fail closed with a bounded checker failure.

No Ember or Warp Drive semantics are implemented in core Go. This evidence remains non-authoritative and does not support runtime execution, callback invocation, repaint, feature identity, or whole-source completeness.
