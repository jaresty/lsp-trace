# ADR0011 directional raw custody 32 MiB production interface Go v2

Status: `AUTHORED_NOT_GO`  
Target token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRODUCTION_INTERFACE_V2_GO`  
Exact source HEAD: `ff74ed966d9580e60ea10a3cad887f92449aa755`

This documentation-only successor supersedes production-interface V1 for implementation guidance only. It depends on the accepted `adr0011-directional-raw-custody-c15-ledger-partition-go-v1` architecture boundary and contains an exact proposed patch against current source. It does not mutate production or tests, authorize implementation, claim RED/GREEN, perform semantic C15 closure, run Stage B, activate behavior, or authorize commit.

The patch preserves the real Manager, `privateB4ByteAccountV2`, C15 recorder, and generic owner/terminal authority while adding distinct general and directional partition identities and counters. General remains capped at 33,554,432 bytes with the existing 24,600-byte legacy baseline. Directional is separately capped at 33,554,432 bytes with one slot. Checked combined live bytes cannot exceed 67,108,864.

`verify_package.py` derives the exact20 PLANNED-only bridge from the immutable committed binding, validates the derived dependency manifest, rejects simple constant increase, a shared bucket, cross-partition transfer, fabricated log authority, missing lifecycle wiring, and extra files, then applies the patch to a fresh exact-HEAD checkout. It mechanically derives the full affected reverse closure, compiles and vets that closure, and runs disposable focused private lifecycle and race probes. Logs are observations only.

Authority: `PRODUCTION_INTERFACE_LIFECYCLE_PLUMBING_ONLY`.

AUTHORED_NOT_GO
