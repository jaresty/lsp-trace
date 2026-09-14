# Ownership and approvals

Status: `PREREQUISITES_DRAFT`; pilot: `PILOT_DISABLED`. Names are deliberately blank; no owner is assigned by this scaffold.

Every required cell must later contain a named accountable human identity, authority basis, scope, decision, date, and referenced artifact digest. Team names and processes may be consulted or responsible, but cannot replace the accountable identity.

| Domain | Accountable identity | Required independent approval |
|---|---|---|
| Protocol/schema and terminal accounting | `UNASSIGNED` | protocol conformance approver |
| Corpus admission and source authority | `UNASSIGNED` | data/source authority |
| Privacy, access and encryption | `UNASSIGNED` | privacy/security approver |
| Retention, deletion, backups and tombstones | `UNASSIGNED` | records/privacy approver |
| Runtime containment and resources | `UNASSIGNED` | security/operations approver |
| Model/runtime supply chain, SBOM and vulnerability | `UNASSIGNED` | security approver |
| License and redistribution | `UNASSIGNED` | license/legal approver |
| Evaluation design, labels and leakage controls | `UNASSIGNED` | evaluation-method approver |
| Threshold calibration and test unlock | `UNASSIGNED` | independent test custodian |
| Pilot acceptance or rejection | `UNASSIGNED` | acceptance authority |

## Separation and conflicts

- Artifact authors cannot solely approve their own control domain.
- Model/backend selectors cannot solely approve model quality or supply-chain admission.
- Calibration analysts cannot access locked test labels or authorize test unlock alone.
- Operators cannot convert process separation, successful execution, or report production into semantic authority.
- Generated outputs always remain `authority=0` and `accepted=false`.
- A conflict disclosure records financial, reporting-line, authorship, operational and evaluation interests; unresolved conflicts block the relevant gate.
- Pilot acceptance authority must be distinct from implementation and evaluation production roles under a documented authority relationship. Different people or processes alone do not prove independence.
