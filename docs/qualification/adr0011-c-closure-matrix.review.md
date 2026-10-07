# ADR0011 Package P4 consolidated C01–C18 closure review

Audited revision: `746efcad9967b211e17238562165fdb872ebc787`

Audited tree: `8d5c2f33b33193de7ccf9df84dba0d3106a1bf4d`

Hermetic prerequisite: `9e1d9e48da1272991657f715581786b6581136c0` (included in audited history)

Machine matrix: `docs/qualification/adr0011-c-closure-matrix.json`

Named checker: `scripts/check-adr0011-c-closure.py`

## Outcome before independent exit review

`C_EXIT_BLOCKED`

The matrix contains exactly 24 required rows: C01–C18 and all six work counters. Five rows close and nineteen remain unknown. No unknown row is waived or promoted from adjacent evidence. Independent review corrected the initial misclassification of `documents_acquired`: its adjacent C13 resource-cap test does not establish an independently selected work-counter row.

| Status | Rows |
|---|---|
| CLOSED | C01, C03, C04, C16, C17 |
| UNKNOWN | C02, C05–C15 except C16/C17, C18, all six work counters |

## Exact blockers and minimum corrective increments

| Rows | Blocker | Minimum corrective increment |
|---|---|---|
| C02 | Required capability family and integrated 2,097,152/+1 boundary are absent. | Add tracked capability-family fixtures and one readiness/acquisition integration test covering initialize request/response, initialized, register/unregister, and selected pending/error/unmatched frames at cap/+1. |
| C05 | Direct 64/65 ingress test passes, but required integrated terminal-response test depends on untracked `.pi/evidence`. | Replace the fixture dependency with tracked hermetic data and execute 64th terminal acceptance plus pre-capture 65th refusal. |
| C06 | 16,387/16,388 owner boundary and readiness acquisition pass, but history-to-transaction attachment/release fixture is non-portable. | Make the attachment fixture tracked/hermetic and rerun exact borrower release and retirement checks. |
| C07, C09, C11 | Implemented isolated boundaries lack tracked portable P2 packets binding source, command, output, and clean-checkout execution. | Add one tracked C07–C12 packet with exact commands/logs/hashes and integrated definition-path witnesses. |
| C08 | Portable packet and STOP/RESTART transaction-clock continuity are absent. | Add controlled-clock STOP/RESTART/cancellation tests proving the attempt epoch is continuous and cannot reset per request. |
| C10, C12 | Direct references-profile tests do not prove the integrated references route selects the 1,572,864 remarshal limit and 524,288 token limit. | Add sessionruntime/acquisition references integration tests for both limits at cap/+1. |
| C13, C14 | Pure accounting passes; selected real-path tests are blocked by missing untracked `.pi/evidence`. | Replace the source fixture with tracked hermetic data and rerun pre-transfer/pre-materialization 256/257 and 4 MiB/+1 tests. |
| C15 | The checked-in census explicitly withholds aggregate acceptance and remains structurally incomplete. | Complete the ownership census for aliases, copies, capacities, transfers, releases, failure paths, and obtain aggregate independent acceptance. |
| C18 | State-machine tests and real success/preflight refusal pass, but real collision witnesses do not cover deadline through readback. | Add production-path pairwise/reached-stage collisions for every adjacent precedence edge, including object/event, retention, and readback, with losing effects suppressed. |
| `bytes_scanned` | No selected owner, rule, witness, or test. | Add a transaction-owned work ledger field counting actual scan bytes including rescans and an isolated exact trace. |
| `messages_processed` | Adjacent message count exists, but attempted/decoded/validated semantics and classes are undefined. | Define the increment stage/classes and add matched, unmatched, notification, request, malformed, retry, and rollback trace tests. |
| `objects_materialized` | C16 resource accounting exists but is not an explicitly selected work-counter trace. | Bind the named work row to a transaction ledger, prove post-success increment and rollback/no-double-charge independently of the resource cap. |
| `events_emitted` | C17 admission accounting exists but is not an explicit successful-emission work trace. | Add post-emission increment semantics, family identity, rollback, retry, and no-double-charge trace. |
| `documents_acquired` | C13 resource accounting exists, but the initially cited test name selected zero tests and the corrected resource test still does not establish an independent work trace. | Define the named work-counter rule in the transaction ledger and add a real-path distinct/retry/rollback trace separate from the 256-document resource cap. |
| `logical_buffer_bytes_reserved` | Live/cumulative byte accounting exists, but cumulative work identity across aliases/copies is not selected. | Define successful reservation-capacity work semantics and test reserve/transfer/release/alias histories without decrementing cumulative work. |
| Cross-cutting source pins | Six selected integration tests still require untracked `.pi/evidence`; the tracked synthetic pin predates C17/C18. | Track/generate every required fixture from a clean checkout and publish current C17/C18-inclusive hashes. |
| Cross-cutting package ownership | No package owns a complete six-counter transaction trace. | Add one private transaction-owned six-field ledger in runtime; preserve method-result admission, C18 precedence, and acquisition terminal-custody boundaries. |

## Charging, cleanup, lifetime, compatibility, and boundaries

- Private transaction lifetime, C13–C17 cleanup, and ordinary/default-off compatibility are implemented and have focused passing tests.
- `privateB4ByteAccountV2` distinguishes outstanding `Live` bytes from monotonic `Cumulative` reservation facts; this does not by itself define the requested `logical_buffer_bytes_reserved` work row.
- C16 reserves before materialization and rolls back failed callbacks; C17 reserves identities before append/emission and rolls back failed batches.
- C18 now owns private reached-stage precedence and acquisition-owned terminal custody, but consolidated collision coverage is incomplete.
- Retention/readback remain outside `internal/lspwire`; no D implementation, index, occurrence admission, path demonstration, public/API, or schema work was performed.
- Exact-session LSP evidence was used only where a READY exact-workspace session returned server-reported CALLS; `APPLICABLE_SESSION_NOT_FOUND`, `TRUNCATED`, and completeness `UNKNOWN` were preserved elsewhere.

## Checker execution

Baseline:

```text
ADR0011_C_CLOSURE rows=24 closed=5 unknown=19 failures=33
C_EXIT_BLOCKED
checker_exit=1
```

Temporary-mutation controls all passed:

```text
unknown_rows: exit=1 marker=True result=PASS
missing_witness: exit=1 marker=True result=PASS
zero_selected_tests: exit=1 marker=True result=PASS
wildcard_only: exit=1 marker=True result=PASS
```

The checker validates the exact ordered row inventory, binds the audited implementation commit/tree and prerequisite ancestry to packet history, requires the three packet paths to be tracked, requires every row commit to be in the audited history, verifies repository-contained regular evidence files and SHA-256 bindings, executes concrete named test selections, compares observed top-level test identities and counts with declarations, rejects wildcard-only `-run` patterns, checks cross-cutting fields, and requires all rows closed. It cannot grant semantic closure merely because the JSON is structurally valid.

## Independent consolidated exit review

The independent review returned `C_EXIT_BLOCKED`. It corrected `documents_acquired` from CLOSED to UNKNOWN, required executable test-selection and Git identity/range checks, and identified final declared-name binding plus tracked-packet requirements. Those artifact defects were corrected before packet commit. The substantive result remains five CLOSED and nineteen UNKNOWN rows.

## Claim ceiling

This report prepares Package P4 and records the exact blocked state. It does not claim full C closure, D readiness, public acceptance, occurrence admission, private index construction, path demonstration, producer authentication, push, or release.
