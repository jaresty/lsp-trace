# ADR0011 remaining C — historical plan status

This companion distinguishes preserved planning records from later reported decisions. It is documentation maintenance, not implementation authorization or a new qualification review.

> **Current sequencing successor:** [P1 integration-first private composition plan](adr0011-p1-composition-integration-plan.md) records later Unit 2 acceptance/merge and the authorized real reader-to-manager handoff. The checkpoint below predates those events; its blocked/design-only statements are historical, not current execution instructions. The original and V2 consolidated plans remain unchanged.

## Preserved plan lineage

- [Original consolidated plan](adr0011-remaining-c-consolidated-plan.proposed.md) records the pre-P0-portability observation point.
- [V2 consolidated plan](adr0011-remaining-c-consolidated-plan.v2.proposed.md) supersedes that planning proposal, records P0 acceptance, and binds the original plan's exact digest.

Both files are preserved unchanged as historical planning records. Their present-tense status, open decisions, proposed execution sequences, source locations, and authorization statements describe their respective observation points, not current execution permission. In particular, the original's pending P0 work and V2's preparation-only Unit 1 prerequisites are superseded by later decisions summarized below. Neither document should be used alone as a current work queue.

## Later decision checkpoint

The following summarizes the parent-reported decisions at preparation of this companion; it does not independently re-adjudicate the local evidence:

- **P0:** accepted at `fdff9da474033afa23fb13b0c1b72bbf6c29a936`.
- **P1 Unit 1:** accepted as `ACCEPT_BOUNDED_UNIT1_UNDER_REVISED_EVIDENCE_CONTRACT`: 48 behavioral rows, six source-order checks, 155 qualification invocations including preserved failed attempts, and one separate passing compatibility invocation. Observation repair and baseline reconciliation were accepted. Original pre-enforcement chronology remains missing; prospective qualification does not reconstruct it. The final record corrects invocation 114 to mutant survival and 116 to panic. Git records the acceptance commit as `885eb4d3` (`Record bounded ADR0011 P1 Unit 1 acceptance`).
- **P1 Unit 2:** remains blocked. The V7 selected model was refused on capacity grounds; this does not prove all compliant models impossible. Subsequent feasibility assessment did not establish fit or non-fit.
- **Accounting boundary:** Unit 2 owns retained payload backings, independent copies, custody metadata, and receipts. Processing allocations can be excluded only under an enforceable separately bounded owner that accepts responsibility. Transfers and remaining borrowers must never create an accounting gap. Without that owner, processing remains Unit 2's responsibility.
- **Authorized next design work only:** a narrow bounded-processing contract for the selected adapter/bridge path, covering allocation ceilings, reserve-before-allocation, coupled limits and overlapping lifetimes, custody acceptance, output/failure ownership, and existing reservation headroom. No processing owner or candidate fit has yet been established by the reported assessment.
- **Unchanged Unit 2 limits:** manager-wide 16 slots / 64 MiB and per exact session generation 4 slots / 16 MiB, including applicable active, retired, and quarantined custody. The existing 14 counterexamples remain preserved.
- **Not accepted or authorized by this document:** Unit 2 runtime implementation, full P1, broader C, D, occurrence admission, public enablement, or capacity increases. Unit 1 acceptance does not discharge those separate gates.

## Local evidence references and portability

Relevant local records reported by the parent include:

- `.pi/evidence/adr0011-p1-unit1-final-acceptance-v1/`
- `.pi/evidence/adr0011-p1-unit2-contract-design-v7/REJECTION.md`
- `.pi/evidence/adr0011-p1-unit2-accounting-boundary-decision-v1/DECISION.md`
- `.pi/evidence/adr0011-p1-unit2-feasibility-v2/FEASIBILITY.md`

The plans also reference earlier `.pi/evidence/` records. These are ignored, machine-local historical references, not artifacts supplied by committing these documents. A fresh checkout cannot independently verify their contents from these links alone. This companion neither republishes those records nor converts reported review outcomes into fresh execution evidence. Portable test fixtures and tracked acceptance material must be evaluated on their own explicitly supplied identities and scope.
