# B05 Lifecycle and Release Admission claim

Baseline: `0702d12`

Ownership is limited to test and release artifacts for B05 lifecycle qualification and release admission.

This frame will add a skip-free `TestProductionEmberProviderCompletesManagedB05Lifecycle` that executes only the packaged production provider binary, a bootstrap validation example, a retained production-lifecycle evidence contract, deterministic replay checks, exact provider/protocol/request identity checks, immutable original/virtual custody checks, expected qualified observation-kind and explicit coverage checks, prohibited-claim absence checks, complete contributor-ID checks, and graph-v3 omission parity checks.

Release, packaging, and GoReleaser checks will fail closed when the production provider binary or its lifecycle qualification is absent. Baseline assertion-specific compiling red is admissible. This frame will not create a provider executable, fabricate provider evidence, mutate retained external evidence, reinterpret Glint `BLOCKED` as relation absence, or change production implementation code.

## Result

At HEAD `c813a90`, the compiler-derived `UPDATES_STATE` observations now disclaim `render_occurrence` before `runtime_execution`, `runtime_mutation`, and `whole_source_completeness`. The focused assertion set passed `3/3`, the complete Ember Glint provider suite passed `73/73`, and canonical `./scripts/qualify-b05-frame6.sh` passed attempts 01–16. Attempt 17 advanced beyond `UPDATES_STATE must disclaim render_occurrence` and stopped at the first distinct blocker: `ASSERT_B05_FRAME6_EXACT_RELATION_updates-state-positive: want=UPDATES_STATE/1`; the provider returned two declaration-qualified relations, one for `count` and one for `enabled`. The failed default qualifier did not modify `qualification/retained/b05/qualification-matrix.v2.json` or otherwise dirty the tree beyond the intentional source, assertion, current provider evidence, and coordination-claim changes.

## Derivation

1. `providers/ember-glint/analyzers/script.mjs` is the compiler-owned source of `UPDATES_STATE` observation non-entailments; adding `render_occurrence` there preserves the existing deterministic list order and strict endpoint/custody behavior.
2. `ASSERT_UPDATES_STATE_STATIC_NON_ENTAILMENTS` fixes the exact ordered semantic ceiling, while `ASSERT_UPDATES_STATE_RETAINED_EVIDENCE_EXACT` verifies the current provider evidence against generated analyzer output.
3. The provider fixture was updated only for the new semantic disclaimer; historical B05 matrices were not manually altered.
4. Canonical qualification demonstrated that strict collector union behavior retained generic disclaimers (`callback_invocation`, `feature_identity`, `repaint`) while preserving the analyzer-specific `render_occurrence` and `runtime_mutation` disclaimers.
5. Because attempt 17 then failed on expected relation cardinality rather than semantic validation, attempts 18–24 and the conditional full/race/vet/docs/release/GoReleaser plus omitted-relations checks were not run.
