# G6 evaluation-threshold worksheet

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** calibration inputs and required decisions; no test data unlocked.

## Evaluation strata

The frozen plan must evaluate separately:

- exact `TARGET` one-item retrieval;
- bounded `NEIGHBORHOOD` retrieval;
- closed `CENSUS` retrieval;
- public or synthetic source strata;
- each supported typed corpus and item type.

No stratum may substitute for undisclosed private-source behavior, and no acquisition mode may be represented as another.

## Required metric decisions

Before test unlock, the accountable evaluation owner must freeze:

- retrieval cutoffs `k`;
- Recall@k and nDCG computation;
- unsupported-claim rate and severity rules;
- candidate-site precision/recall;
- review-target precision/recall;
- optional grouping precision/recall and false-merge rate;
- missing, invalid, abstention, and failure rates;
- reviewer-efficiency measure;
- latency, memory, context, and index-size budgets;
- replay equivalence tolerances;
- uncertainty method and minimum effect size.

## Threshold table to complete

| Metric | Required baseline | Pilot threshold | Uncertainty rule | Automatic stop |
|---|---|---|---|---|
| Recall@k | structural / lexical / random | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| nDCG@k | structural / lexical / random | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Unsupported-claim rate | lexical/structural context | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Candidate-site precision/recall | structural and lexical | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Review-target precision/recall | structural and lexical | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Grouping precision/recall | random/null where applicable | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| False-merge rate | random/null where applicable | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Abstention/failure rate | none | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Reviewer efficiency | baseline workflow | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Latency | resource budget | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |
| Memory/context/index size | resource budget | `UNASSIGNED` | `UNASSIGNED` | `UNASSIGNED` |

## Calibration rules

Thresholds must be selected using only training/calibration labels and frozen before test labels are revealed. The test set cannot be used for prompt, model, retrieval, grouping, threshold, or resource tuning. Minimum effects, uncertainty treatment, and automatic stops must be recorded with the threshold decision.

A passing semantic metric cannot override an authority, privacy, lineage, supply-chain, containment, denominator, replay, or ownership failure. A failed required threshold keeps the pilot disabled and produces an immutable rejection record.

## Unlock gate

An independent test custodian may unlock test inputs only after F0–F6 artifacts, digests, leakage audit, environment pins, supply-chain evidence, and approval identities are complete. This worksheet is not an unlock authorization.
