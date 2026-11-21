# Incremental inventory owner — first slice

## Craft / boundaries
Extend DurableStore itself, using its existing inventory-run head CAS and semantic task controller. No scopejob child, staging owner, new CLI, provider process, enumeration or default switch. Preserve dirty legacy work. This slice is bounded, not repository-scale.

1. [x] Establish a tagged R-only io.Pipe positive selected-result fixture using actual M/P/S. REFERENCING_OCCURRENCE was refused; explicit REFERENCED_SYMBOL succeeded without changing ownership guards. No SELF fallback.
2. [x] Add explicit incremental head/config/admission formats. Create pre-evidence ID/config and finite cumulative reservations; immutable linked admission records keep the head small. Reserve and bind admission identity before retention effects. Interrupted claims are terminal partial uncertainty, not replay/refund candidates.
3. [x] Seal ordered admitted records into the existing bounded semantic state under the same run ID/head/accounting. Protect completion before issuing tasks. No overall job expiry; finite fresh task leases and cumulative semantic allowance. Legacy head v2 remains unchanged.
4. [x] Focused controlled positive/fault/CAS tests and legacy selectors, normal/race, offline vet/build/diff. Document exact evidence and limits for independent parent review.

## Funding policy
Create now commits a self-contained charged Creating head before config publication. Its versioned logical quote is 8*canonicalConfigBytes + 32*(65536+16384) + 32768; read-only status remains uncharged inspection. Admission and Seal spend bounded slices durably before effects, with no refund. Seal inherits all prior spending. Known semantic completion is quoted only at Seal; unknown future admissions are not promised funding. MaxAdmittedEvidenceBytes defaults provisionally to 2 GiB and checks only successfully committed evidence bytes AFTER retention, excluding orphan/config/record/semantic artifacts. The actual user requirement for a 2 GiB total-retained-storage cap is UNMET/deferred; this is not an approved substitution. Future acquisition needs an explicit funded adapter, not a new grant. See PLAN-incremental-create-repair.md for the narrow creation repair and explicit v3 refusal/v4 format policy.

## Deferred retirement
Existing scope Start/lifecycle/status aggregation remains live and unchanged. After integration the inventory owner must replace those responsibilities; this capability slice does not claim their deletion.
