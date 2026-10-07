# ADR0011 Package P4 revised C-scope closure review

Audited implementation revision: `a314e8cac860317b3c3046b0e5533054098ff570`

Audited tree: `ce3e2296edc8178b58dcc1b2490f400d14b2485e`

Hermetic prerequisites: `c8dc1ac804cda716d5aef8e5d3ad7a46fed42430` and `42fe29db5cfc07f53617d273985c5a3e8bc9c6b3` (included in audited history)

Machine matrix: `docs/qualification/adr0011-c-closure-matrix.json`

Named checker: `scripts/check-adr0011-c-closure.py`

Revised contract: `docs/qualification/adr0011-c-closure-scope-v2.md`

Accepted amendment: `docs/qualification/adr0011-c02-supersession-amendment.json`

## Scope amendment

`ADR0011-C02-SUPERSEDED-BY-C01-V1` retires C02 as an independent pre-decode requirement. C01 remains authoritative for generic complete-original-frame admission before body allocation. Existing post-decode capability-family validation remains required for admitted frames.

The original C02 requirement and its evidence remain unchanged in the predecessor plan and retained matrix fields. Supersession is a formal scope decision, not implementation closure and not a reusable waiver.

A streaming classifier was rejected because it would duplicate bounded JSON-RPC parsing and correlation solely to identify capability-family membership before normal decode. C01 already supplies the generic pre-allocation refusal. A second parser would add disagreement, lifecycle, and cleanup risk without changing admitted production behavior.

## Revised inventory and outcome

Future C exit is assessed against `REVISED_C_SCOPE_V2`, not the original C01–C18 active scope:

- historical inventory: 24 rows;
- active rows: 23;
- superseded inventory row: C02 only;
- currently CLOSED active rows: C01, C03, C04, C05, C06, C07, C08, C09, C10, C11, C12, C13, C14, C16, C17;
- currently UNKNOWN active rows: C15, C18, and all six work counters.

Current result remains:

```text
C_EXIT_BLOCKED
```

The scope amendment removes C02 from the active denominator. Subsequent hermetic evidence requalification closes C05, C13, and C14 without production changes. C06 closes after causally verified synchronous settlement of retired, unowned backing. C07 closes with a versioned tracked hermetic packet proving the real private pre-block request boundary at -1ns, equality, and +1ns. C08 closes after binding the generation-scoped 60-second epoch to startup preflight and proving STOP retirement, successful RESTART succession, stale-generation refusal, and failed-RESTART non-admission. C09 closes with portable integrated evidence that the definition-selected private Manager path accepts exact decoded-remarshal sizes through 1,048,576 bytes and refuses 1,048,577 before accepted observation or downstream definition publication. C10 closes with independent portable evidence that the real private references route selects the 1,572,864-byte profile, continues at the boundary, and refuses +1 before accepted notification or result publication. C11 closes with portable integrated evidence that the private definition path accepts exact original result tokens through 524,288 bytes and refuses 524,289 before token copy, accepted response observation, or downstream result publication. C12 closes with independent portable evidence that the noncustodial private references path returns exact result bytes through 524,288, correlates accepted observation to session and generation, and refuses 524,289 before token copy or publication. Fifteen of 23 active rows are closed; eight remain unknown.

## Exact amendment binding

The checker accepts `SUPERSEDED_BY_C01` only when all of the following match exactly:

1. row ID `C02`;
2. successor `C01`;
3. amendment ID `ADR0011-C02-SUPERSEDED-BY-C01-V1`;
4. tracked amendment path and SHA-256;
5. byte-identical predecessor path and SHA-256;
6. canonical SHA-256 of the complete predecessor C02 row after restoring its historical `UNKNOWN` status and removing only supersession metadata;
7. revised contract path and SHA-256;
8. revised active/historical row counts.

No other row may use the superseded status. The checker retains the exact ordered 24-row historical inventory while evaluating closure over 23 active rows.

## Remaining active blockers

| Rows | Blocker | Minimum corrective increment |
|---|---|---|
| C15 | The allocation census withholds aggregate acceptance. | Complete ownership, alias, copy, transfer, release, and failure-path accounting. |
| C18 | Real adjacent collision witnesses are incomplete. | Add production-path collisions through retention/readback. |
| Six work counters | Independent transaction work semantics and exact traces remain incomplete. | Define and verify each named work counter without inferring closure from resource caps. |
| Cross-cutting source pins | Current portable evidence is incomplete. | Track or generate every selected fixture from a clean checkout. |
| Cross-cutting package ownership | No owner exposes the complete six-counter trace. | Add the private transaction-owned ledger only under separate authorization. |

## Checker guarantees

The checker validates the exact ordered historical inventory, revised scope identity, accepted amendment bytes, predecessor bytes, audited Git history, tracked packet paths, contained regular files, evidence hashes, concrete named test selection, observed test names/counts, non-wildcard qualification, and cross-cutting fields. It fails closed for every active non-CLOSED row.

Mutation controls must cover at least:

- C02 changed back to UNKNOWN;
- any retained historical C02 owner, rule, witness, boundary, commit, test, artifact, or blocker changed;
- C02 supersession on another row;
- wrong successor, amendment ID, path, or hash;
- modified predecessor bytes;
- active UNKNOWN rows;
- missing witnesses;
- zero selected tests;
- wildcard-only qualification.

## Claim ceiling

This packet documents a revised C-only scope. It does not authorize or claim production behavior changes, D readiness, public/API/schema changes, occurrence admission, private indexing, path demonstration, push, release, or closure of any remaining active unknown row.
