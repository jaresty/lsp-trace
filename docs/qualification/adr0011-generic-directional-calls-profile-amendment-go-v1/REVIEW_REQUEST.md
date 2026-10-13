# Independent review request

Review only the candidate token `ADR0011_GENERIC_DIRECTIONAL_CALLS_PROFILE_AMENDMENT_GO` against the exact package bytes and dependencies. Do not implement, mutate tests, activate profiles, publish schemas, edit the halted private-interface package, or issue any provider/profile/production GO.

## Required checks

1. Verify authoring HEAD `2faac2671987f6e6f7c680fe4f072064cfe48136` and dependency commit `0e2583b2a535701da152e756253b4b96df55272b`.
2. Verify every checksum in `DEPENDENCIES.sha256` and `SHA256SUMS` from exact bytes.
3. Verify `docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1/` remains byte-identical to the halted manifest digest `d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c` and has no edit from this candidate.
4. Confirm separate generic incoming/outgoing profile IDs, canonical domains, semantics, evidence, accounting, diagnostics, and review; neither inherits the other.
5. Confirm separate Go/gopls provider bindings can narrow environment/evidence but cannot change generic semantics or inherit each other.
6. Confirm completeness is target-set-relative only, with exact denominator identity, closed accounting, frontier, partial handling, and `UNKNOWN`; no repository/runtime completeness.
7. Confirm generic diagnostics and provider diagnostics have separate ownership and cannot substitute for raw custody, semantic evidence, provider evidence, success, completeness, or GO.
8. Confirm commit `7f09e5357b117d20449fc16097631e32d4eead7f`, manifest `316afe2501aebe4195894d1334be566ae49eb32c88276ebee7b2e4d86062f44d`, and 5900 targets are scoped only to Go/gopls qualification corpus/evidence.
9. Confirm accepted opaque raw custody, separate 32 MiB ledger, sole terminal, C15 non-duplication, disabled/`CALLS_ONLY` compatibility, and Definition/Reference architecture patterns remain unchanged without D/R occurrence retyping.
10. Confirm the exact halted-interface and production/test-write-set impact identifies reusable mechanics, required generalization, affected documents, and prohibits expansion without another reviewed amendment.
11. Confirm the causal future qualification split, rejected alternatives, `authority=0`, `accepted=false`, `UNKNOWN` completeness, and no public schema/activation.
12. Hard-stop review as `REVISE` if generic semantics require public surface or any accepted raw-custody architecture change.

## Permitted result

The reviewer may issue `ADR0011_GENERIC_DIRECTIONAL_CALLS_PROFILE_AMENDMENT_GO` only for the exact package digest and dependency closure, or return `REVISE`/`REJECT`. The token is amendment-review authority only and supplies no implementation, test, evidence-acquisition, schema, activation, qualification, admission, or production authority.

AUTHORED_NOT_GO
