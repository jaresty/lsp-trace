# Source-constrained synthetic qualification

This project reconstructs two provisional call resolutions from exact source without modifying or parsing NAIS and without adding call relations to the Go framework.

## Custody and materialization

The committed `vendor/` corpus is sufficient for offline verification. To reproduce it from authoritative inputs:

```sh
mkdir -p /tmp/source-constrained-packages
npm pack ember-concurrency@5.2.0 @warp-drive/legacy@5.8.1 @glint/template@1.7.4 typescript@5.9.3 --pack-destination /tmp/source-constrained-packages
node qualification/source-constrained-synthetic/materialize.mjs \
  /path/to/market-view-ui.git \
  /tmp/source-constrained-packages
```

`materialize.mjs` reads Git commit `326718ae733cb26097bd30246276cecd371a4e79` with `git show`, verifies both path/blob/SHA-256 triples, verifies npm tarball SHA-512 integrity, and extracts only the required declaration members. It writes only beneath this project. Package retrieval is an explicit build-time operation; committed CI does not need network access.

`package-lock.json`, `tsconfig.json`, and `glint.config.json` pin package, TypeScript, and Glint identities. `provenance.json` is the declaration ledger. A declaration with an annotation not present in source is rejected.

## Offline verification

```sh
node qualification/source-constrained-synthetic/verify.mjs
node --test qualification/source-constrained-synthetic/source-constrained-synthetic.guard.test.mjs
node qualification/source-constrained-synthetic/perturbations.mjs
```

The analyzer checks exact vendored digests and source shapes, package-lock integrity, declaration provenance, the protected B05 v2 Git blob, relation-specific positive and negative cases, and policy boundaries. It deterministically emits `qualification-evidence.json` with `RESOLVED_SYNTHETIC`, explicit before/after target status, analyzer/package identities, `coverage_boundary`, and `failure_boundary`.

The executable cases distinguish:

- `uploadComplete.perform`: `TaskForAsyncTaskFunction` → `Task` → `AbstractTask.perform`;
- `upload.reload`: `UserImportModel` → `@warp-drive/legacy Model.reload`;
- unrelated and contradictory receivers: unresolved;
- unsupported collection elements: unresolved.

These are synthetic qualification results only. `provisional-policy.json` advertises no authoritative relations, forbids `PRODUCTION_RESOLVED`, preserves the B05 v2 blob, and keeps `PROGRAM_B_ADMITTED` false.
