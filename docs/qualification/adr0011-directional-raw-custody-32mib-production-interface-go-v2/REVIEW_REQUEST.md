# Independent review request

Review target: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRODUCTION_INTERFACE_V2_GO`  
Exact HEAD: `ff74ed966d9580e60ea10a3cad887f92449aa755`  
Current disposition: `AUTHORED_NOT_GO`

Run `python3 verify_package.py` and independently review:

1. V2 supersedes production-interface V1 for implementation guidance only and depends on the accepted ledger partition.
2. The real existing Manager/account/recorder/generic-owner terminal authority is retained.
3. General maximum remains 33,554,432 including the 24,600 baseline; directional maximum is separately 33,554,432; one directional slot; checked combined maximum is 67,108,864.
4. Exact 32 MiB directional reservation remains possible with the general baseline charged; +1/general/global/overflow refusal is pre-effect.
5. Opaque leases carry immutable partition identity; aliases remain same-partition; cross-partition transfer is prohibited.
6. New, accepted, and refused owner paths are wired; release/cancel/STOP/RESTART/Shutdown and reuse converge through existing terminal authority.
7. Stage A is exact20 and PLANNED-only with migration hooks; no semantic C15 closure or Stage B claim exists.
8. The exact patch touches only four paths within the accepted expanded seven-path set and no test path.
9. A fresh disposable exact-HEAD checkout applies the patch, derives the reverse closure, compile-checks it, vets affected packages, and runs focused private lifecycle probes.
10. Adversarial verifier checks reject simple constant increase, shared bucket, cross-partition transfer, fabricated logs, unwired lifecycle paths, and extra files.
11. Source hashes and final checksums match; logs remain non-authoritative observations.

A successful review may return only `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRODUCTION_INTERFACE_V2_GO`, meaning exact proposal-package coherence. It does not authorize RED/GREEN, implementation, production/test mutation, semantic C15 closure, Stage B, activation, commit, or production admission.

AUTHORED_NOT_GO
