# Two-stage enumeration and adjudication contract

Status: `AUTHORED_NOT_GO`.

## Stage A: PRE-IMPLEMENTATION

Stage A separates source-backed present observations from future architecture obligations.

### Existing-source contract

An `EXISTING_SOURCE` row is admissible only for an exact current source locator in these bounded families:

- `Manager` as the current accounting-domain owner;
- current C15 account type, creation, and lifetime;
- current lease reservation, alias, transfer, and release capability;
- current terminal cleanup;
- nearby lease/borrow APIs that are explicitly observed as lifetime-bound and not reusable.

Allowed statuses are `EXISTING_OBSERVED` and `EXISTING_NOT_REUSABLE`. These are observations, not `NECESSARY`, `REUSE`, future design approval, or proof that the current API satisfies the directional contract.

### Planned-obligation contract

A planned row has no source line or source hash. It must use exactly one status from:

- `PLANNED_RESERVATION`
- `PLANNED_COPY`
- `PLANNED_ALIAS`
- `PLANNED_TRANSFER`
- `PLANNED_RELEASE`

It binds an exact proposed file, symbol, component, attempt/account/lease identity, terminal paths where applicable, and predecessor lineage. A row uses the exact `GENESIS` sentinel or a sorted `|`-joined set of stable planned row IDs. The generator validates predecessor closure and requires missing predecessors, cycles, and duplicate row definitions/branches to equal zero. `NECESSARY`, `REUSE`, `FALSE_POSITIVE`, and source-semantic `UNRESOLVED` are forbidden in Stage A planned rows because implementation source does not exist.

### Stage A completeness

`PREIMPLEMENTATION_COMPLETE` requires all 47 preserved v1 identities plus one newly introduced planned reservation architecture obligation accounted, duplicates `0`, omissions `0`, planned predecessor closure with cycles/missing predecessors/branches `0`, all six components covered by reservation/copy/alias obligations, whole-acquisition transfer present, and terminal convergence release present. The model-reported 48 is unpinned because no exact full adjudicator output is available to copy and hash-prove. Completeness says nothing about actual implementation source semantics.

## Stage B: POST-IMPLEMENTATION

Stage B begins only from an exact committed implementation revision and canonical source manifest. It does not accept line-0, absent-source, or planned rows as actual rows.

The canonical actual row ID is:

```text
sha256("adr0011.c15.two-stage.actual.v1\0" ||
       source_path || "\0" || symbol || "\0" || operation_kind || "\0" ||
       component || "\0" || backing_identity_rule)
```

A deterministic source enumerator must emit every actual reservation, owned copy, alias/borrow, transfer, release, accounting owner/account lifecycle, and terminal cleanup relationship in scope. Two runs from the same committed manifest must be byte-identical. Actual duplicates and omissions must be zero.

A separate independent pass classifies each actual row as exactly:

- `NECESSARY`
- `REUSE`
- `FALSE_POSITIVE`
- `UNRESOLVED`

Every non-`UNRESOLVED` classification requires exact source evidence, contrary evidence or a falsifier, adjudicator identity, and revision. `IMPLEMENTATION_GO` requires `UNRESOLVED=0`; this contract does not return or authorize that gate.

## Correction lineage

Planned and actual namespaces never alias. `STAGE_B_ACTUAL_CLASSIFICATION.tsv` contains `backing_identity_rule`, which participates in the actual row ID formula. `STAGE_B_PLANNED_ACTUAL_LINEAGE.tsv` separately records complete planned/actual predecessor and successor sets without determining actual identity.

- `REALIZED`: one planned obligation maps to one actual row.
- `SPLIT`: one planned obligation maps to multiple independently source-derived actual rows.
- `MERGED`: multiple planned obligations map to one independently source-derived actual row.
- `DISAPPEARED`: a planned obligation has no actual row; `actual_successor_ids` is explicitly empty and the disappearance must be justified or block GO.
- `EMERGENT`: an actual row has no planned predecessor and must be independently adjudicated.

Each lineage record has a stable `lineage_record_id`, event, complete `planned_predecessor_ids`, `actual_predecessor_ids`, `planned_successor_ids`, and `actual_successor_ids`, plus justification and adjudicator revision. Split and merge records list every predecessor and successor; no row is silently overwritten. A disappeared obligation is not a source absence claim until the complete committed manifest and canonical enumeration establish it.

## Gate boundary

The exact token `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_TWO_STAGE_CONTRACT_GO` qualifies only this ordering and schema. Interface/RED work remains blocked until that token. Implementation remains separately blocked and is never authorized by Stage A completeness or this package.
