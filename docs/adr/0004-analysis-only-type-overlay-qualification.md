# ADR 0004: Qualify analysis-only type overlays in disposable descendants

Status: Accepted

## Context

Some source-constrained JavaScript relations are blocked because the checked source does not expose a safe receiver type. Qualification needs to determine whether an analysis-only environment change and a narrow type overlay improve relation recovery without changing the pinned source, provider admission, or installed runtime.

A useful result must distinguish an environment correction from the type overlay, preserve relation identity rather than comparing only counts, and fail closed when the provider cannot completely account for writes or value origins.

## Decision

`lsp-trace` owns a generic three-stage qualification campaign: `BASELINE`, `ENVIRONMENT_ONLY`, then `TYPE_OVERLAY`. The generic campaign knows only Git stages, preconditioned file edits, observations, canonical relation identity multisets, schema validation, and publication. It has no Ember, reload, task, collection, or framework-specific semantics.

The pinned source repository is immutable. A campaign verifies its requested commit and materializes a disposable Git descendant outside both the source repository and the `lsp-trace` repository. Each stage is independently committed and its parent, tree, and cleanliness are verified. A stage does not run when its predecessor is unconfirmed.

Adapters own semantic qualification. The first adapter is under `providers/ember-glint` and is deliberately narrow: Ember JavaScript; an unsafe receiver reached through `for...of`; an empty collection property initializer; a JSDoc imported element type; and an environment edit adding `ember-source/types`. It returns structured blocker and write-origin diagnostics plus preconditioned edit proposals. Heterogeneous, `any`/`unknown`, reassigned, unsupported, or incompletely enumerated writes block the overlay.

Campaign evidence has fixed authority `SOURCE_CONSTRAINED_ANALYSIS_INTERVENTION`, purpose `QUALIFICATION_ONLY`, `automatic_continuation: false`, `admission_mutated: false`, and `inventory_mutated: false`. The API exposes no inventory or admission mutation options.

Relation comparison is performed over canonical identity multisets, retaining duplicate multiplicity. Counts may be reported only as summaries. Identity drift is a distinct failure.

A caller selects the output path. The candidate is validated against the registered qualification schema before publication, then atomically renamed into place. Validation failure leaves the selected output untouched.

## Consequences

- Qualification cannot mutate the source repository, provider inventory, admission state, installed binary, bootstrap configuration, private repositories, or NAIS discovery.
- Environment-only and overlay effects remain separately attributable.
- Provider behavior outside the explicit adapter entry point is unchanged.
- The B05 manifest pins `326718ae733cb26097bd30246276cecd371a4e79` and records expected environment-only task/reload results of `6/BLOCKED` and overlay results of `6/1`; committed tests use miniature fixtures and do not execute that private source.
- The first adapter is not a general JavaScript data-flow engine. Unsupported writes and uncertain origin completeness intentionally remain blockers.
