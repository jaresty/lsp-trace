# ADR0011 directional raw-custody C15 ledger partition design v1

Status: `AUTHORED_NOT_GO`  
Review token requested: `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEDGER_PARTITION_GO`  
Target HEAD: `87b5c9c76c93861bd2759313e93e757260aea8d3`

This documentation-only successor evaluates both permitted accounting models against exact source and freezes the dedicated Manager-owned directional subledger. It does not implement behavior, mutate production/tests, run production tests, create a compile contract, activate a profile, issue GO, or commit.

Normative order:

1. `ARCHITECTURE.md`
2. `C15_ENUMERATION_AND_MIGRATION.md`
3. `WRITESET_AND_TESTS.md`
4. `SOURCE_HASHES.tsv`
5. `REVIEW_REQUEST.md`
6. `verify_package.py`
7. `SHA256SUMS`

Immutable evidence: all predecessor packages, including blocked V1/V2/V3 and the retained blocked-v7 external bundle wherever externally held, remain evidence only and byte-immutable. This package copies none of their bytes and grants them no new authority.

Boundary: private, authority `0`, default off, generic LSP `CALLS_ONLY`; D/R and ordinary generic behavior remain unchanged. Incoming and outgoing remain semantically independent while using the same mechanical directional-bucket policy.

AUTHORED_NOT_GO
