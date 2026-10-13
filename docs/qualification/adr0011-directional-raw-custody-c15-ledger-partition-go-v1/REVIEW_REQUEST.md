# Review request

Return exactly `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEDGER_PARTITION_GO` only if all checks pass for this package’s final `SHA256SUMS`; otherwise return `REVISE` with exact blockers.

1. HEAD is exactly `87b5c9c76c93861bd2759313e93e757260aea8d3` and source/predecessor hashes verify.
2. Prior packages and the externally held blocked-v7 bundle remain immutable evidence.
3. Both permitted models are evaluated; dedicated directional subledger is selected; simple constant increase and shared enlarged bucket are rejected with source-backed rationale.
4. Same Manager/accounting recorder and sole generic terminal/backing owner remain; no second terminal/recorder/owner exists.
5. Existing/general max is 33,554,432 and retains current behavior including the 24,600-byte legacy layout; directional max is separately 33,554,432; one directional slot; checked Manager combined max is exactly 67,108,864 without double-counting the legacy table.
6. Account, partition, attempt, reservation, release, lease, alias, and generation identities are exact; aliases bind the same partition; cross-partition transfer/borrowing is prohibited.
7. Reservation and failure/refusal precedence is exact and pre-effect; exact-limit, plus-one, overflow, collision, and state-unchanged consequences are reviewable.
8. Incoming/outgoing semantics, attempts, evidence, target sets, outcomes, and reviews remain independent while mechanical bucket policy is shared.
9. STOP, RESTART, Shutdown, cancellation, explicit release, duplicate release, races, settlement, and fresh-identity reuse converge on the existing terminal authority.
10. C15 Stage A preserves prior rows and adds only planned successor obligations; Stage B requires committed implementation, source-derived actual IDs, independent classification, correction lineage, deterministic regeneration, and zero unresolved before later GO.
11. Required exact-limit, plus-one, aggregate/global, concurrent-partition, second-slot, release/reuse, lifecycle, precedence, race, compatibility, and deterministic-enumeration tests are complete.
12. Future write-set includes `sessionruntime/b4_explicit_bytes_c15_private.go`, names every permitted production/test/docs path, and authorizes no others.
13. Type/field/private API impact is sufficient for the next production-interface successor but does not claim an exact compile contract.
14. Authority is 0, default off, generic LSP CALLS_ONLY, D/R/generic/history/predecessor/lineage/public behavior unchanged.
15. Deterministic verifier and package checksums pass; status remains `AUTHORED_NOT_GO`.

This token reviews architecture-package bytes only. It authorizes no RED, implementation, production/test mutation, activation, Stage B adjudication, commit, production admission, or implementation GO.

AUTHORED_NOT_GO
