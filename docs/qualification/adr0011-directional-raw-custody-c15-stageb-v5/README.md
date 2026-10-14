# ADR0011 directional raw custody C15 StageB v5 qualification package

Status: `STAGEB_C15_AUTHORED_NOT_GO`  
Source HEAD: `49334ce23de37c0094f50d21d57ab251a46130ca`  
Authority: `0`; accepted: `false`; implementation GO: `false`; final GO: `false`.

This package is a docs/evidence-only StageB v5 qualification surface for the current post-GREEN worktree. It mechanically enumerates current source rows and provides provisional classifications for review. It does **not** independently accept C15 semantics, production admission, or implementation GO.

## Correction-safe boundary

- Actual row IDs are derived from current source identity (`path`, `function`, `line`, `operation_kind`, `component`, `backing_identity_rule`, source-line hash). Planned StageA IDs are never reused as actual IDs.
- Mechanical enumeration cannot accept semantic classification. The `NECESSARY`, `REUSE`, and `FALSE_POSITIVE` values are provisional source-evidence labels with contrary evidence/falsifiers and authority ceiling 0.
- `UNRESOLVED=0` here means this producer found no unresolved row in its bounded source package; it is not independent acceptance.
- Superseded and negative predecessor bundles remain preserved in lineage tables and `NEGATIVE_AND_SUPERSEDED_BUNDLES.md`.

## Key results

- Actual source rows: `91`.
- Duplicate actual IDs: `0`.
- Omitted actual rows under this enumerator: `0`.
- Provisional UNRESOLVED rows: `0`.
- Source candidates discovered: `194`; explicit exclusions `108`; unclassified `0`.
- StageA planned IDs reconciled: `20` / 20; duplicates `0`; omissions `0`.
- Earlier 47+1 model identities reconciled: `48` / 48; duplicates `0`; omissions `0`.

## Partition conclusions for review

- Request and response are preallocated independently through `Lease.Allocate("request-frame", ...)` and `Lease.Allocate("response-frame", ...)`, each delegating to Manager `privateDirectionalV5.allocate`, which charges the directional partition and creates C15 object records before the sole retained raw `make`.
- Result, element, Reader, and View rows are aliases/views into the response-frame backing, not allocations; they have explicit `ALIAS_VIEW_*` rows.
- The inherited general C15 table/account/lease machinery is classified `REUSE`, separate from directional request/response partition rows.
- Manager/generic owner is the sole terminal path: `genericTerminal.requestTerminal()` -> generic owner release callback -> `Reservation.Terminate()` -> `releaseAccount()` -> C15 frame object releases and account terminal release.

## Files

- `ACTUAL_SOURCE_ROWS.tsv` — source-derived actual rows with exact file/function/line/op/hash.
- `PROVISIONAL_CLASSIFICATIONS.tsv` — provisional labels, rationales, falsifiers, predecessors, authority ceiling.
- `SOURCE_CANDIDATES.tsv` — independent pattern-family source candidates and ROWED/EXCLUDED dispositions.
- `SOURCE_CANDIDATE_EXCLUSIONS.tsv` — explicit evidence-grounded candidate exclusions with falsifiers.
- `STAGEA_PLANNED_RECONCILIATION.tsv` — all 20 accepted StageA planning IDs mapped without promotion.
- `MODEL_47_PLUS_1_RECONCILIATION.tsv` — all 48 preserved/planned identities accounted.
- `DUPLICATES_OMISSIONS.json` — counts and duplicate/omission/candidate checks.
- `SELF_TESTS.json` — verifier tamper self-tests for deletion, duplication, reclassification, source drift, and extra unclassified candidates.
- `SOURCE_MANIFEST.sha256` — current source/predecessor hashes used by the package.
- `logs/source-enumeration.log` — generation and verification receipt.
- `verify_stageb_v5.py` — deterministic verifier/re-enumerator.
- `CHECKSUMS.sha256` — package checksums.

Run:

```bash
python3 docs/qualification/adr0011-directional-raw-custody-c15-stageb-v5/verify_stageb_v5.py --source /private/tmp/adr0011-incoming-calls-v1-6yiY4m
```

`STAGEB_C15_AUTHORED_NOT_GO`
