# ADR0011 Package P4 revised C-scope closure review

Audited implementation revision: `07d241b777d89250bc902208759d2a940aaa0ec2`

Audited tree: `e17fd21aefa8b0425449d623af8318a5fd0a4c14`

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
- currently CLOSED active rows: C01, C03, C04, C05, C06, C07, C08, C09, C10, C11, C12, C13, C14, C16, C17, bytes_scanned, messages_processed, and objects_materialized;
- currently UNKNOWN active rows: C15, C18, and three work counters.

Current result remains:

```text
C_EXIT_BLOCKED
```

The scope amendment removes C02 from the active denominator. Subsequent hermetic evidence requalification closes C05, C13, and C14 without production changes. C06 closes after causally verified synchronous settlement of retired, unowned backing. C07 closes with a versioned tracked hermetic packet proving the real private pre-block request boundary at -1ns, equality, and +1ns. C08 closes after binding the generation-scoped 60-second epoch to startup preflight and proving STOP retirement, successful RESTART succession, stale-generation refusal, and failed-RESTART non-admission. C09 closes with portable integrated evidence that the definition-selected private Manager path accepts exact decoded-remarshal sizes through 1,048,576 bytes and refuses 1,048,577 before accepted observation or downstream definition publication. C10 closes with independent portable evidence that the real private references route selects the 1,572,864-byte profile, continues at the boundary, and refuses +1 before accepted notification or result publication. C11 closes with portable integrated evidence that the private definition path accepts exact original result tokens through 524,288 bytes and refuses 524,289 before token copy, accepted response observation, or downstream result publication. C12 closes with independent portable evidence that the noncustodial private references path returns exact result bytes through 524,288, correlates accepted observation to session and generation, and refuses 524,289 before token copy or publication. The `messages_processed` row closes with a distinct transaction work account: every successfully decoded complete message counts once at semantic begin, later refusal does not refund it, malformed decode counts zero, and checked overflow freezes without mutation. The `bytes_scanned` row closes at implementation commit `e5e9c6f6b4fe9cafb3daf643a947c7b90c99555b` and tree `2bbda7c27868d00dfc4f1a8a39a28dcca80a1579`: every actual raw-scanner traversal is charged, including publication and replay paths; malformed work persists, oversize input charges zero, occurrence identity is enforced, typed overflow propagates through owner, proposal, scanner publication, and replay paths, the account seals terminally, and the ordinary path remains default-off. The production caller denominator is exactly 67 traversals of the 199-byte retained payload, so `67 * 199 = 13333`. Eighteen of 23 active rows are closed; five remain unknown. The named checker is therefore expected to report exactly eight closure failures: one status failure each for C15 and C18, plus one status and one missing-rule failure for each of the three remaining work-counter rows. The result remains `C_EXIT_BLOCKED`. C07–C12 subsequently migrated from mutable live-source comparison to matrix-pinned versioned selectors and immutable historical source-authority packets. That migration preserves prior execution evidence and claim ceilings; it is not requalification or recertification.

## bytes_scanned closure binding

The closure is bound to these immutable files and independent results:

- implementation: commit `e5e9c6f6b4fe9cafb3daf643a947c7b90c99555b`, tree `2bbda7c27868d00dfc4f1a8a39a28dcca80a1579`;
- production denominator, default-off, identity, overflow, and terminal tests: `internal/adr0011acquisition/owner_bytes_scanned_red_darwin_test.go`, SHA-256 `7a3e0fb35d9e03e970a1397f02d6f39a7b107738559bb8cdc176030165db632c`;
- malformed replay test: `internal/adr0011acquisition/raw_scanner_work_private_test.go`, SHA-256 `4c8747ea62a7f5ad57ae223b4c2a323f36088e4b840376a517fc2d4fb13509b7`;
- scanner publication/replay, work-unit identity, oversize-source, and typed replay-overflow test: `internal/adr0011acquisition/scanner_record_test.go`, SHA-256 `2f1423f1b4a955b78f8f45a798f113222b31a5ad26d53050d68d91bf2d438abd`;
- canonical source manifest: `docs/qualification/adr0011-synthetic-source-pin.83e9c0e3de15f522.manifest.json`, SHA-256 `83e9c0e3de15f52272ec37ebdd81b595afe93a525e566188557d9cbb406cf8b5`;
- source-pin review: `docs/qualification/adr0011-synthetic-source-pin.83e9c0e3de15f522.review.md`, SHA-256 `75ce82332636f480ebf35fe831de586ab13aa1fed429eb9dd23ea78746a280cc`, verdict `REPIN_VERIFY_GO`, integration receipt `20261007171209-0471`;
- final independent semantic review: verdict `GREEN_REVIEW_GO`, integration receipt `20261007171208-3384`, independently verifying typed causes through all replay/publication paths, exact `199 * 67 = 13333`, identity, overflow, terminal seal, malformed replay charging, oversize zero, and default-off behavior.

These results close only `bytes_scanned`; they do not recertify or change any other row.

## objects_materialized closure binding

The closure denominator is exactly each successfully constructed `internal/adr0011methodresult.Item`. Reservation occurs before construction, but `ObjectsMaterialized` increments only after the item exists. Malformed, `null`, and empty inputs count zero. Equal-valued candidates at distinct parser ordinals each count. A later failure does not refund an already materialized item, while replay constructs and counts the item again.

C16 is adjacent resource-cap accounting only and is not authority for this work-counter closure. The production private/default-off Manager path carries exact transaction account custody through the definition reservation and borrow. Identity mismatch and checked overflow refuse before construction; overflow preserves the saturated value and terminally freezes the account. Seal waits for admitted construction and then freezes the terminal snapshot. Default-off result and lease behavior remains unchanged. This work counter adds no public field, wire value, API, or schema.

The closure is bound to these immutable files and independent results:

- foundation: commit `2504b07082fcc080926ba14a0a141b0d26cc14b2`, tree `6f59c92446e4570a2efddcb39299ac64b0d7e2b4`;
- integration: commit `07d241b777d89250bc902208759d2a940aaa0ec2`, tree `e17fd21aefa8b0425449d623af8318a5fd0a4c14`;
- account reservation/settlement tests: `internal/adr0011work/objects_materialized_red_test.go`, SHA-256 `6532529ed51e9370761d11aea014dd5e00bc767ca4ac27643d0423180965335a`;
- exact parser-item denominator tests: `internal/adr0011methodresult/parser_c16_private_test.go`, SHA-256 `b5abe1759bde80e14632b22cbfd42a8b1abdc78a88b70d82e5f70d966e679437`;
- integrated custody/default-off tests: `sessionruntime/adr0011_c17_objects_materialized_red_private_test.go`, SHA-256 `d9c988def2be64188ee834fc75dda4c1cc925bc0553de440564029c7bb937352`;
- canonical source manifest: `docs/qualification/adr0011-synthetic-source-pin.65493e108d04b60f.manifest.json`, SHA-256 `65493e108d04b60f2304fa298da019a0fece435b6eea02740581141f1b661118`;
- source-pin review: `docs/qualification/adr0011-synthetic-source-pin.65493e108d04b60f.review.md`, SHA-256 `f7027c91cc2490b1ca3f39e5a7da84d4b8dd514642551f3b1aa6eb908d0d8ac6`, verdict `REPIN_VERIFY_GO`, integration receipt `20261007181024-2602`;
- foundation review: verdict `FOUNDATION_GREEN_GO`, integration receipt `20261007174223-4136`;
- final independent semantic review: verdict `GREEN_REVIEW_GO`, integration receipt `20261007181024-3261`, independently verifying the denominator, exact custody, reserve/commit timing, retry/replay, typed failures, terminal lifecycle, default-off behavior, and preservation of C15–C18 and `messages_processed`.

These results close only `objects_materialized`; they do not recertify or change C16 or any other row.

The current matrix also records an evidence-only successor pin for `sessionruntime/adr0011_c17_messages_processed_red_private_test.go`, SHA-256 `63661dd7b07374891222d60d32a89d6e64e4a46ddecd9e3214fcbac2b03ba837`. Under integration commit `07d241b777d89250bc902208759d2a940aaa0ec2`, the shared-account fixture now passes the exact `owner.Transaction`; independent GREEN review explicitly confirmed that this preserves the existing `messages_processed` contract. This successor updates only the immutable test-file evidence hash: it does not change the row's `CLOSED` status, owner, rule, selected test names/count, behavioral contract, or closure authority.

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
| Three work counters | Independent transaction work semantics and exact traces remain incomplete. | Verify each remaining named work counter without inferring closure from resource caps. |
| Cross-cutting source pins | Current portable evidence is incomplete. | Track or generate every selected fixture from a clean checkout. |
| Cross-cutting package ownership | The private transaction-owned ledger exists, but three producer rows are not yet wired and independently verified. | Wire each remaining producer at its selected semantic boundary and preserve exact transaction identity. |

## Checker guarantees

The checker validates the exact ordered historical inventory, revised scope identity, accepted amendment bytes, predecessor bytes, audited Git history, tracked packet paths, contained regular files, evidence hashes, concrete named test selection, observed test names/counts, non-wildcard qualification, and cross-cutting fields. It fails closed for every active non-CLOSED row. At this closure state, its expected closure result is exactly eight failures—one status failure each for C15 and C18, plus status and missing rule for `events_emitted`, `documents_acquired`, and `logical_buffer_bytes_reserved`—and `C_EXIT_BLOCKED`. Any additional integrity failure is a separate packet blocker.

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

This packet documents a revised C-only scope. It does not authorize or claim D readiness, public/API/schema changes, occurrence admission, private indexing, path demonstration, push, release, or closure of any remaining active unknown row. The `objects_materialized` work account remains private and default-off and introduces no public schema.
