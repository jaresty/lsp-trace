# Evaluation plan scaffold

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`; test data remains locked.

## F0–F8 freeze sequence

- **F0 Purpose and claims:** allowed retrieval/navigation claims, prohibited authority claims, hypotheses, minimum effects and rejection rule.
- **F1 Corpus:** public or synthetic typed strata, exact provenance, inclusion/exclusion, sample sizes, and TARGET/NEIGHBORHOOD/CENSUS coverage.
- **F2 Labels:** independent relevance, work-context, candidate-site, review-target and optional grouping labels; adjudication and disagreement handling.
- **F3 Splits and leakage:** immutable train/calibration/test identities; repository/source/revision/family deduplication; prompt/model selection cannot inspect test labels.
- **F4 Baselines:** structural-only, lexical and random/null baselines with exact configurations.
- **F5 Metrics:** exact `k`; Recall@k, nDCG, unsupported-claim rate, candidate-site/review-target precision and recall, optional grouping precision/recall and false-merge rate, missing/invalid/abstention/failure rates, reviewer efficiency, latency, memory, context and index size.
- **F6 Calibration:** numeric pass/fail thresholds, uncertainty method, replay tolerances, resource budgets and automatic stops fixed using training/calibration only.
- **F7 Test unlock:** independent custodian verifies F0–F6 digests, leakage audit, environment/supply-chain pins and approval matrix before revealing or executing test inputs.
- **F8 Report:** immutable report identity binds plan, bundle, corpora, splits, labels, executable/runtime/model identities, outcomes, denominators, metrics, deviations and acceptance/rejection decision.

No numeric value is supplied by this scaffold. Every numeric threshold must be calibrated, recorded and digest-bound before F7; it cannot be weakened after test unlock. Public and synthetic strata report separately and cannot stand in for undisclosed private-source behavior.

## Stops and promotion boundary

Stop or reject on leakage; missing labels or strata; denominator imbalance; unsupported authority/acceptance claims; privacy, lineage, supply-chain or runtime failure; baseline/effect/uncertainty miss; replay/resource/failure-rate miss; or post-unlock tuning. Preserve negative and incomplete results. Passing F8 does not enable the pilot or authorize implementation, shipment, CLI, MCP, registry, or core-module integration.

Feature inventory is optional downstream evaluation with separate semantic adjudication, stakeholder correction, unsupported-claim and false-merge gates. Structural or semantic grouping cannot accept feature identity.
