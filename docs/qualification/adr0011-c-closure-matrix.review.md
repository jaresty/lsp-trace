# ADR0011 Package P4 revised C-scope closure review

Audited implementation revision: `0d2f19fe38cf7e431a60208d5582d5a7cd6cc9d6`

Audited tree: `8820747197c23c9e17d1d1e0145f3001f1efcd6c`

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
- currently CLOSED active rows (21): C01, C03, C04, C05, C06, C07, C08, C09, C10, C11, C12, C13, C14, C16, C17, bytes_scanned, messages_processed, objects_materialized, events_emitted, documents_acquired, and logical_buffer_bytes_reserved;
- currently UNKNOWN active rows (2): C15 and C18.

Current result remains:

```text
C_EXIT_BLOCKED
```

The scope amendment removes C02 from the active denominator. Subsequent hermetic evidence requalification closes C05, C13, and C14 without production changes. C06 closes after causally verified synchronous settlement of retired, unowned backing. C07 closes with a versioned tracked hermetic packet proving the real private pre-block request boundary at -1ns, equality, and +1ns. C08 closes after binding the generation-scoped 60-second epoch to startup preflight and proving STOP retirement, successful RESTART succession, stale-generation refusal, and failed-RESTART non-admission. C09 closes with portable integrated evidence that the definition-selected private Manager path accepts exact decoded-remarshal sizes through 1,048,576 bytes and refuses 1,048,577 before accepted observation or downstream definition publication. C10 closes with independent portable evidence that the real private references route selects the 1,572,864-byte profile, continues at the boundary, and refuses +1 before accepted notification or result publication. C11 closes with portable integrated evidence that the private definition path accepts exact original result tokens through 524,288 bytes and refuses 524,289 before token copy, accepted response observation, or downstream result publication. C12 closes with independent portable evidence that the noncustodial private references path returns exact result bytes through 524,288, correlates accepted observation to session and generation, and refuses 524,289 before token copy or publication. The `messages_processed` row closes with a distinct transaction work account: every successfully decoded complete message counts once at semantic begin, later refusal does not refund it, malformed decode counts zero, and checked overflow freezes without mutation. The `bytes_scanned` row closes at implementation commit `e5e9c6f6b4fe9cafb3daf643a947c7b90c99555b` and tree `2bbda7c27868d00dfc4f1a8a39a28dcca80a1579`: every actual raw-scanner traversal is charged, including publication and replay paths; malformed work persists, oversize input charges zero, occurrence identity is enforced, typed overflow propagates through owner, proposal, scanner publication, and replay paths, the account seals terminally, and the ordinary path remains default-off. The production caller denominator is exactly 67 traversals of the 199-byte retained payload, so `67 * 199 = 13333`. Twenty-one of 23 active rows are closed; two remain unknown. The named checker is therefore expected to report exactly two closure failures: one status failure each for C15 and C18. The result remains `C_EXIT_BLOCKED`. C07–C12 subsequently migrated from mutable live-source comparison to matrix-pinned versioned selectors and immutable historical source-authority packets. That migration preserves prior execution evidence and claim ceilings; it is not requalification or recertification.

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

## events_emitted closure binding

The accepted denominator counts only accepted canonical events. Acquisition contributes exactly two fixed events plus two events per accepted acquisition element (`2+2A`). Capability contributes one event per accepted normalized `REGISTER` or `UNREGISTER` entry (`C`). Target publication contributes exactly two events per accepted original target ordinal (`2T`). Thus the selected family denominator is exactly `2+2A+C+2T`; no aggregate event-work cap or additional event family is inferred. Diagnostics, aliases, non-accepting retries, intermediate target `BEGIN`/`TERMINAL` fields, publication or replay bookkeeping, and resource-account operations do not charge.

C17 is adjacent resource authority only, not authority for this work denominator. Its 8192/8193 boundary remains independently binding and unchanged. The transaction-owned work reservation precedes canonical visibility. The generic reservation layer and the capability and target paths prove panic rollback. The acquisition path proves callback-error and pre-finalization-failure rollback, but its real fixture exposes no pre-finalization panic injection boundary; that case remains explicitly `BLOCKED_NOT_RED`/unavailable and is not claimed. Identity mismatch, C17 refusal, and checked work overflow roll back unaccepted reservations.

Target evidence proves only that an already committed two-event pair persists across a later duplicate/refusal. It does not prove generalized partial-batch atomicity; a multi-target partial-refusal witness is unavailable and not claimed. Accepted canonical events remain cumulative across the supported later-failure case, replay, mixed-family interleaving, STOP/RESTART, and terminal sealing. Work overflow refuses before visibility, preserves the saturated count, and terminally freezes the account. Default-off results and all prior counters remain preserved.

The closure is bound to these immutable revisions and reviewed evidence:

- foundation: commit `58c6a71efbfe890c64e107989f84aefc4fd57bed`, tree `83465d317df2069ce06f4f20b9d469eb4954a02d`;
- integration: commit `5c09d1f5d45fb39b5644a03692adb2ac21124c2d`, tree `c07a1f4e41660912378d784221986724d55aed89`;
- reservation/settlement tests: `internal/adr0011work/events_emitted_red_test.go`, SHA-256 `c73e75a98d430f4cc6e86056a9591a8c5c6840b582725bc48410ec0c72a99652`;
- acquisition denominator tests: `sessionruntime/b4_acquisition_events_emitted_red_private_test.go`, SHA-256 `3d2693e4646e16e489614258ff859039e0bb8684608dd9f4848dc461deab7f8e`;
- capability denominator tests: `sessionruntime/adr0011_c17_capability_events_emitted_red_private_test.go`, SHA-256 `11ed8d1c8563d8c679d9ebd42cca8e927d41782fc5e4036523c3790f58d3e23f`;
- target denominator tests: `sessionruntime/adr0011_c17_targets_events_emitted_red_private_test.go`, SHA-256 `8a52dfcf5bbbd09a2d9154e20f9f5139700ebc2c562f94b642491e876b143b3e`;
- mixed-family, lifecycle, C17-adjacency, diagnostics, and default-off tests: `sessionruntime/b4_mixed_family_c17_qualification_private_test.go`, SHA-256 `a3bd85ab323ea4d789efdc5791e0feea01d7fe944b031151163d807c6b74f969`;
- canonical source manifest: `docs/qualification/adr0011-synthetic-source-pin.f62f1a5d70dfcb1a.manifest.json`, SHA-256 `f62f1a5d70dfcb1ae8573fb07b744ad0c9e0a0ba0278cd74c3cc11e5bd6b23f1`;
- source-pin review: `docs/qualification/adr0011-synthetic-source-pin.f62f1a5d70dfcb1a.review.md`, SHA-256 `fbee18235a78ad725a58fb67ed2f8a4143884ceb7c5a8fcd25293a0d39b1bb35`, verdict `REPIN_VERIFY_GO`, integration receipt `20261007200547-7439`;
- foundation review: verdict `FOUNDATION_GREEN_GO`, integration receipt `20261007185706-8064`;
- family reviews: accepted acquisition ordering/rollback/default-off review `20261006214038-2569`, corrected capability normalization/rollback/lifecycle review `20261006203848-6075`, corrected target publication/default-off review `20261006194838-3649`, and mixed-family ownership/lifecycle review `20261006220915-7648`;
- final independent semantic review: verdict `GREEN_REVIEW_GO`, integration receipt `20261007200547-8408`, independently verifying the exact three-family denominator, custody, reservation, supported rollback paths, overflow, lifecycle, mixed-family/default-off behavior, C15–C18 preservation, and prior-counter preservation; acquisition pre-finalization panic injection and multi-target partial refusal remain unavailable and outside the claim.

These results close only `events_emitted`. They do not recertify C17, close C15 or C18, or themselves close either producer row that remained at that review point.

## documents_acquired closure binding

The canonical work identity is exactly the `DocumentRequest.URI` string passed as `req.URI`; the work account performs no URI normalization. The first valid successful acquisition for that exact key in a transaction increments `documents_acquired` to one. A valid cache success, later fresh success, and explicit refresh for the same key all use that same identity and remain count one. Repeated and concurrent same-key calls still execute every admitted callback; deduplication affects only the committed work count. The user-directed cache decision for this closure expressly treats both cache and fresh valid success as successful acquisition under the same key.

A callback commits only after the production Manager validates successful complete supply and retained custody: exact request/result/supply URI, session, generation, document version, classification, method, content, parameters, language, version, and digest all agree. Invalid, stale, source-failed, mismatched, identity-refused, and overflow-refused attempts count zero. Context carries the account but does not alter transaction identity or document semantics. Caller-owned privacy admission remains with `RefreshDocumentSupply`; `CaptureSupply` behavior is preserved. Seal waits for admitted callbacks, concurrent duplicates count once, committed identity keys remain private, default-off callers preserve exact results, and C13/C14 source limits remain independently binding adjacent resource authority rather than authority for this work-counter closure.

The closure is bound to these immutable revisions and reviewed evidence:

- foundation: commit `43479997e7d7e20e6ab118b05cad68a3f2929488`, tree `4ff65db1a6980b4b55e0e3812f95996b14e17cfa`;
- integration: commit `9f89d580bf042b8d88309eaa4df2f463d58fb474`, tree `1429ecacd162760809d4a529f5ad9ce87c267348`;
- reservation, exact-key, first-success, duplicate-callback, concurrency, rollback, overflow, seal, coexistence, and identity-privacy tests: `internal/adr0011work/documents_acquired_red_test.go`, SHA-256 `bb5226c306d7b742f8120983ead4f41049deacc9eddb2b5e1feb06c4e6bc7aca`;
- integrated cache/fresh/refresh, invalid/stale/source refusal, canonical-policy, custody, context identity, concurrency, seal, and default-off tests: `sessionruntime/adr0011_c17_documents_acquired_red_private_test.go`, SHA-256 `527e6143be0608d8fd184dd02b06cedc73ebafb366b021e49f0120c9243bdbdb`;
- canonical source manifest: `docs/qualification/adr0011-synthetic-source-pin.6f5b75c7019e862d.manifest.json`, SHA-256 `6f5b75c7019e862d9627a94205eceffdfedfddbfd5e55ece453474e00b2581f5`;
- source-pin review: `docs/qualification/adr0011-synthetic-source-pin.6f5b75c7019e862d.review.md`, SHA-256 `935a2120d95fc034f66a969866244bd59dd7a6b83d75208e5b1b39787014369d`, verdict `REPIN_VERIFY_GO`, integration receipt `20261007212118-4708`;
- final independent semantic review: verdict `GREEN_REVIEW_GO`, as supplied by the user for this closure, verifying exact URI identity, cache/fresh same-key success, first-success counting, callback execution, zero-count refusals, context-only custody, concurrency, seal, privacy/default-off/caller preservation, and C13/C14 non-authority.

These results close only `documents_acquired`. They do not recertify C13 or C14 or change C15 or C18.

## logical_buffer_bytes_reserved closure binding

The selected denominator is cumulative exact capacity for every successful successor allocation reservation across seven pinned roles: prefetch, header, body, consume scratch, result token, original frame, and decoded message. It is not a live-byte gauge. Outstanding reservation ownership remains separate and private in the C15 ledger; releasing a lease reduces C15 live ownership but never refunds committed work. Successful retries and replay reserve and count anew, and a malformed message preserves the exact successful role reservations reached before failure.

The work account and exact occurrence identity are installed at both production successor-allocation-owner construction sites. Work overflow and identity mismatch refuse before C15 reservation or allocation. A later independent C15 refusal rolls back the pending work reservation, preserving the separation between this cumulative work counter and C15's 33,554,432-byte live-owned resource cap. Exact equality at `uint64` maximum succeeds; +1 refuses without counter mutation and terminally freezes the work account. Default-off outputs, leases, C15 accounting, and all prior work counters remain exact.

The real owner exposes no test-only seam between work admission and C15 reservation. The direct account layer proves Seal settlement, but the production-owner causal Seal injection remains explicitly `BLOCKED_NOT_RED`; it is unavailable and is not claimed by this closure.

The closure is bound to these immutable revisions and reviewed evidence:

- foundation: commit `0fb4e41a02db54eba57dc488a268e8415f13b45d`, tree `091814b704a56c2b4cfee0d2af9ef75d3e8547c2`;
- integration: commit `0d2f19fe38cf7e431a60208d5582d5a7cd6cc9d6`, tree `8820747197c23c9e17d1d1e0145f3001f1efcd6c`;
- reservation, settlement, overflow, identity, coexistence, and exact snapshot tests: `internal/adr0011work/logical_buffer_bytes_reserved_red_test.go`, SHA-256 `595fb3b19755da88873f9c8f9a9b2314f301398cab7981e1053698aa487be36c`;
- seven-role production denominator and exact capacity-formula test: `internal/lspwire/successor_allocation_denominator_private_test.go`, SHA-256 `c8aa3833c4305647bfcfd7b2442f8cbd7bdf4f46b06028300911da5ea2af6cda`;
- integrated exact reached sums, malformed/replay behavior, no-refund release, C15 distinction, pre-C15 mismatch/overflow, both construction sites, and default-off tests: `sessionruntime/adr0011_c17_logical_buffer_successor_red_private_test.go`, SHA-256 `6fb22900ba9274c93efbf61fddf32fd6c1260660f6b9f55220b6d581c0f53842`;
- canonical source manifest: `docs/qualification/adr0011-synthetic-source-pin.17e4a67fbebd0227.manifest.json`, SHA-256 `17e4a67fbebd0227488515f90562d05b2981ccf77a3978ac5a0a860d3efce80f`, selecting 119 files and 950,476 bytes with aggregate digest `sha256:0f605406e07d2f70e6765f9820a2c5677c6087744a60e6b318ce4631bd73d098`;
- source-pin review: `docs/qualification/adr0011-synthetic-source-pin.17e4a67fbebd0227.review.md`, SHA-256 `3ad5264ec574a0635d57ad2aba64bd85d76e4fe93bb008901986ec1c374d2341`, verdict `REPIN_VERIFY_GO`, integration receipt `20261007223225-1205`;
- foundation review: verdict `FOUNDATION_GREEN_GO`, integration receipt `20261007214645-3123`;
- final independent semantic review: verdict `GREEN_REVIEW_GO`, integration receipt `20261007223225-3565`, independently verifying the seven-role denominator, exact custody and capacity, cumulative no-refund behavior, retry/malformed reached successes, C15 rollback and independent authority, pre-C15 work refusal, both construction sites, default-off parity, and prior-counter preservation while leaving the production-owner Seal injection explicitly unavailable.

These results close only `logical_buffer_bytes_reserved`. They do not recertify or close C15 or C18, and they do not infer resource-cap closure from work-counter evidence. No repair evidence pin is added because the reviewed canonical successor pin is sufficient.

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

## Checker guarantees

The checker validates the exact ordered historical inventory, revised scope identity, accepted amendment bytes, predecessor bytes, audited Git history, tracked packet paths, contained regular files, evidence hashes, concrete named test selection, observed test names/counts, non-wildcard qualification, and cross-cutting fields. It fails closed for every active non-CLOSED row. At this closure state, its expected closure result is exactly two failures—one status failure each for C15 and C18—and `C_EXIT_BLOCKED`. Any additional integrity failure is a separate packet blocker.

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

This packet documents a revised C-only scope. It does not authorize or claim D readiness, public/API/schema changes, occurrence admission, private indexing, path demonstration, push, release, or closure of either remaining active unknown row. All six work accounts remain private and default-off and introduce no public schema. The `logical_buffer_bytes_reserved` closure does not close C15 and does not claim the unavailable production-owner Seal injection.
