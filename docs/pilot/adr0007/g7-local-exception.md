# G7 local-project governance exception draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Exception:** independent approval is foregone for this local project.
- **Accountable owner:** `PROJECT_OWNER`
- **Approval mode:** documented owner self-review.
- **Accepted:** `2026-09-18` by `PROJECT_OWNER`.
- **Decision:** `ACCEPTED_WITH_LOCAL_EXCEPTION`.

## Scope

This exception applies only to the local ADR 0007 pilot-preparation and any later isolated pilot explicitly covered by the same accepted exception. It does not apply to shared, public, production, multi-party, or shipped use.

## Rationale

The project has one accountable local owner and no separate approval organization. Requiring a second approver would not be operationally available. The owner accepts responsibility for protocol, data, privacy, runtime, supply-chain, evaluation, and acceptance decisions.

## Risks accepted

- self-review may miss protocol, privacy, security, supply-chain, evaluation, or semantic-quality defects;
- owner incentives and confirmation bias are not independently checked;
- acceptance evidence has reduced governance independence;
- this exception must not be represented as independent validation or external approval.

## Compensating controls

- keep all gates fail-closed and explicitly statused;
- preserve immutable digests, source boundaries, lineage, terminal accounting, and rejection records;
- keep the worker isolated, network-denied, and non-public;
- require reproducible conformance and evaluation artifacts;
- disclose self-review in every acceptance and pilot report;
- prohibit shipment, public CLI/MCP exposure, core integration, and production use without a new governance decision;
- reopen governance if another operator, contributor, stakeholder, or deployment boundary is introduced.

## Acceptance requirements

Before this exception can close G7, the project owner must explicitly accept this document as the governing local exception and record the acceptance date, exact bundle scope, risks, compensating controls, and expiry/reconsideration trigger.

This exception is accepted for the stated local-project scope. It closes the independent-approval requirement only within that scope; it does not authorize pilot execution, implementation, shipment, public CLI/MCP exposure, core integration, or production use.
