# B05 Lifecycle and Release Admission claim

Baseline: `0702d12`

Ownership is limited to test and release artifacts for B05 lifecycle qualification and release admission.

This frame will add a skip-free `TestProductionEmberProviderCompletesManagedB05Lifecycle` that executes only the packaged production provider binary, a bootstrap validation example, a retained production-lifecycle evidence contract, deterministic replay checks, exact provider/protocol/request identity checks, immutable original/virtual custody checks, expected qualified observation-kind and explicit coverage checks, prohibited-claim absence checks, complete contributor-ID checks, and graph-v3 omission parity checks.

Release, packaging, and GoReleaser checks will fail closed when the production provider binary or its lifecycle qualification is absent. Baseline assertion-specific compiling red is admissible. This frame will not create a provider executable, fabricate provider evidence, mutate retained external evidence, reinterpret Glint `BLOCKED` as relation absence, or change production implementation code.

## Result

At baseline HEAD `2e03e16`, the B05 Frame 6 qualification oracle now records exact expected multiplicity per seed: every negative expects zero relations, each deliberately single-relation positive expects one, and `updates-state-positive` expects exactly two. The focused multiplicity guard passed both missing-second and spurious-third rejection cases. Canonical `./scripts/qualify-b05-frame6.sh` passed the complete Ember Glint provider suite `73/73` and B05 attempts 01–20, then stopped on the first distinct blocker at attempt 21: `renders-from-positive` returned `RESOURCE_EXHAUSTED` with `typed provider result contains no accepted observations`. Attempts 22–24 and the conditional provider/full/race/vet/docs/release/GoReleaser, omitted-relations zero-start/V2/V3 parity, and clean-tree gates were not run. No analyzer or retained evidence was changed.

## Derivation

1. `providers/ember-glint/fixtures/updates-state-positive.ts` deliberately contains two independently compiler-qualified writes, `this.enabled = true` and `this.count++`, so exact retained evidence requires two `UPDATES_STATE` relations rather than a generic positive count of one.
2. The oracle now checks exact relation count before checking every emitted relation kind, preventing both missing and extra selected-kind relations from passing.
3. The `UPDATES_STATE` specialization requires two distinct declaration-derived `STATE_VALUE` endpoints, one shared declaration-derived state producer, and two distinct single write anchors.
4. `TestB05Frame6ExactMultiplicityGuard` persistently proves that one relation and three relations both fail the exact-two oracle; no `>= 1` or first-relation-only check remains.
5. The existing 12-seed loop still issues incoming and slice twice per seed, preserving the 24-attempt replay contract and read-only retained-evidence policy.
6. The canonical run advanced through the corrected UPDATES_STATE attempts and exposed the next independent blocker at attempt 21, so the stop-on-first-distinct-blocker rule excluded all later gates.
