# ADR0011 directional raw custody 32 MiB production interface Go v1

Target token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRODUCTION_INTERFACE_GO`

This documentation-only successor contains an exact proposed production splice for commit `950a3f1ae1bd2cffa8e246b51819b7a7930a68a4`. It does not mutate campaign source, authorize implementation, or claim behavioral qualification.

The corrected proposal initializes a Manager-owned directional ledger with the real opaque `privateB4ByteAccountV2`, creates a directional attempt terminal for each accepted or refused generic call-hierarchy owner, releases attempt leases through that owner's existing terminal lifecycle, reclaims terminal attempt records before fresh reuse, and requests terminal release of the shared account only after Shutdown joins all generic owners.

`verify_package.py` independently derives exact20 from the committed binding bridge, rejects forged map/wiring variants, clones exact HEAD, applies the exact patch, mechanically derives reverse dependents, and executes compile-only tests and vet. `COMPILE.log` is an observation, never authority.

Evidence is labeled `PRODUCTION_SPLICE_COMPILE_ONLY`. No RED/GREEN, C15 closure, activation, or implementation acceptance is claimed.

Disposition: `AUTHORED_NOT_GO`.
