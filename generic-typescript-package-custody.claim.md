# Generic TypeScript package custody and omitted-resolution policy

## Claim

Caller-project relation admission derives package custody from the TypeScript checker-resolved declaration's nearest real package root inside the caller project's `node_modules`, requires the package manifest name/version to match immutable provider provenance, and retains exact declaration parent/member checks. Omitted module-resolution settings use a deterministic analysis-only modern policy: preserve Node16/NodeNext pairings, use Bundler for compatible ES modules (pairing an omitted module with ESNext), and retain Node10 for incompatible explicit module kinds. Explicit caller `moduleResolution` remains authoritative.

## Derivation

1. A synthetic `ember-concurrency@5.2.0` package placed declarations at `declarations/index.d.ts` behind `types` and `exports`; TypeScript resolved exact `AbstractTask.perform` and the relation retained that nested declaration custody.
2. A synthetic `@warp-drive/legacy@5.8.1` exposed only `./model` through package exports. Omitted Node10 produced exact TS2307 despite the declaration being present.
3. The analysis-only omitted policy changed that case to GREEN without changing explicit Classic, NodeNext, or Bundler settings.
4. Custody was generalized from relation-kind-specific paths to provenance-derived package identity plus nearest real package root. Wrong package names, symlink escapes, missing declarations, same-spelling members, and any/unknown receiver types remain rejected or explicitly blocked.
5. Focused caller tests passed 9/9 and the full provider package passed 89/89, including packed offline install and retained archive convergence.
