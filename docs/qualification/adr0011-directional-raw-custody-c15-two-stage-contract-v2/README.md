# ADR0011 directional-raw custody C15 two-stage contract v2

Status: `AUTHORED_NOT_GO`  
Target review token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_TWO_STAGE_CONTRACT_GO`  
Source HEAD: `a1cd77950ffa45ebed52310878f772f3c7344756`  
Authority: `0`; accepted: `false`; implementation authority: `none`.

This documentation-only successor corrects the gate ordering of `adr0011-directional-raw-custody-c15-enumeration-v1` without modifying it. The v1 package, including its `UNRESOLVED.tsv`, result, counts, generator, and run manifest, remains immutable negative evidence. Its absent-source rows are not normalized, overwritten, or semantically classified here.

## Honest 47 + 1 identity accounting

The immutable v1 files contain exactly 47 unique identities: 33 current-source candidates and 14 absent-successor proposals. No exact full adjudicator transcript output is present in the task artifacts, so the earlier model-reported count of 48 is an **unpinned inconsistent report**, not a hash-proven evidence artifact and not 48 independently supplied identities.

`IDENTITY_ACCOUNTING_47_PLUS_1.tsv` has the precise denominator: 47 exact preserved v1 row IDs plus one newly introduced v2 planned reservation architecture obligation. The new obligation is not attributed to v1 or to an adjudicator transcript. Its cause is mechanical: v1 `generate.py` iterates six `ALLOCATE` and six `ALIAS` proposals and appends one `TRANSFER` and one `RELEASE`, but never emits the architecture/interface-required pre-effect reservation operation. Accounting reports duplicates `0`, omissions `0`, and all `47 + 1` identities accounted.

This correction does not infer source semantics for the absent proposed file. The introduced identity is a planned obligation only.

## Two stages

### Stage A — PRE-IMPLEMENTATION

`STAGE_A_EXISTING_SOURCE.tsv` contains only narrow current-source observations for the Manager accounting owner, account creation/lifetime, lease reserve/alias/release and transfer capabilities, terminal cleanup, and nearby lifetime-bound nonreusable borrows. Its only classifications are `EXISTING_OBSERVED` and `EXISTING_NOT_REUSABLE`; neither means future necessity or reuse approval.

`STAGE_A_PLANNED_OBLIGATIONS.tsv` contains exact future obligations only. Each row is one of `PLANNED_RESERVATION`, `PLANNED_COPY`, `PLANNED_ALIAS`, `PLANNED_TRANSFER`, or `PLANNED_RELEASE`, with proposed file, symbol, component, attempt/account/lease identity, terminal paths where applicable, and predecessor lineage. Stage A contains no `NECESSARY` or `REUSE` classification.

Stage A completeness means:

1. all 47 preserved v1 identities plus the one newly introduced planned reservation obligation are mechanically accounted with duplicates and omissions equal to zero;
2. every architecture component has exact planned reservation, copy, and alias obligations;
3. whole-acquisition transfer and terminal release obligations are present;
4. current-source observations stay narrow and source-backed;
5. no absent source is semantically classified.

Stage A completeness is architecture-obligation completeness, not source-semantic completeness and not implementation GO.

### Stage B — POST-IMPLEMENTATION

After an implementation is separately authorized and committed, a canonical source enumerator must regenerate actual rows from an exact committed implementation source manifest. Actual row IDs are source-derived and must not reuse planned IDs. `STAGE_B_ACTUAL_CLASSIFICATION.tsv` is intentionally header-only before implementation.

An independent review then classifies every actual row as exactly `NECESSARY`, `REUSE`, `FALSE_POSITIVE`, or `UNRESOLVED`. `UNRESOLVED=0`, exact source-manifest verification, deterministic duplicate/omission checks, and complete planned-to-actual correction lineage are required before any later `IMPLEMENTATION_GO` can be considered.

Planned-to-actual lineage is carried separately in `STAGE_B_PLANNED_ACTUAL_LINEAGE.tsv` and is many-to-many and correction-safe: disappeared plans are recorded as `DISAPPEARED` with an empty actual successor set; one plan producing several actual rows is `SPLIT`; several plans producing one actual row is `MERGED`; ordinary correspondence is `REALIZED`; unplanned actual rows are `EMERGENT`. Each lineage record contains complete planned/actual predecessor and successor sets. Actual identities always derive from committed source path, symbol, operation, component, and `backing_identity_rule`. Planned IDs remain lineage evidence and never become actual row IDs.

## Gate ordering

The requested two-stage token qualifies these contract bytes only. It is not interface GO, RED/test GO, implementation GO, production admission, activation, commit authority, or permission to mutate production/tests. Interface and RED remain blocked until the two-stage contract receives its exact review token. Implementation remains blocked until separately authorized after interface and RED gates. Stage B cannot run until a committed implementation exists, and this package does not authorize creating one.

Normative order:

1. `TWO_STAGE_CONTRACT.md`
2. `IDENTITY_ACCOUNTING_47_PLUS_1.tsv`
3. `STAGE_A_EXISTING_SOURCE.tsv`
4. `STAGE_A_PLANNED_OBLIGATIONS.tsv`
5. `STAGE_B_ACTUAL_CLASSIFICATION.tsv`
6. `STAGE_B_PLANNED_ACTUAL_LINEAGE.tsv`
7. `COUNTS.json`
8. `SOURCE_MANIFEST.sha256`
9. `REVIEW_REQUEST.md`
10. `RUN_MANIFEST.sha256`

`AUTHORED_NOT_GO`
