# G7 ownership decision worksheet

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Purpose:** convert the existing ownership matrix into explicit decisions without assigning identities by inference.

## Required decisions

Each row requires a named accountable human, authority basis, independent approver, decision date, scope, conflict disclosure, and referenced artifact digest.

| Domain | Accountable human | Authority basis | Independent approver | Conflict disclosed | Decision |
|---|---|---|---|---|---|
| Protocol/schema and terminal accounting | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Corpus admission and source authority | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Privacy, access, and encryption | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Retention, deletion, backups, and tombstones | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Runtime containment and resources | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Model/runtime supply chain, SBOM, vulnerability | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| License and redistribution | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Evaluation design, labels, and leakage controls | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Threshold calibration and test unlock | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |
| Pilot acceptance or rejection | `PROJECT_OWNER` | `LOCAL_PROJECT_AUTHORITY` | `EXCEPTION_ACCEPTED` | `OWNER_SELF_REVIEW` | `ACCEPTED_WITH_LOCAL_EXCEPTION` |

## Separation rules

- Authors cannot solely approve their own control domain.
- Model/backend selectors cannot solely approve model quality or supply-chain admission.
- Calibration analysts cannot access locked test labels or authorize test unlock alone.
- Operators cannot convert process success into semantic authority.
- Pilot acceptance must be independent of implementation and evaluation production roles.
- A team name or process is not a substitute for an accountable human identity.

## Current local-project exception proposal

The project owner is accountable for all domains because this is a local project. The project intends to forego independent approval and use documented owner self-review instead.

This is a deliberate control reduction, not evidence that independent approval exists. The owner must record the rationale, affected risks, compensating controls, scope, and explicit acceptance before treating this as an approved exception. The exception must not be generalized to shared, public, production, or multi-party use.

## G7 blocking conditions

G7 is accepted with the explicitly recorded local-project exception. The exception does not authorize pilot execution or convert self-review into independent approval outside the stated scope.
