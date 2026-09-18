# Local narrow-pilot gate decisions

- **Status:** `LOCAL_GATE_DECISIONS_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Owner:** `PROJECT_OWNER`
- **Governance:** accepted local self-review exception

## G4 decision

Accept the native-runtime file-hash inventory, Mach-O/ABI inspection, ad hoc signature verification, installer manifest, SBOM source inventory, and OSV worker scan as sufficient for this local four-packet pilot. The native directory scan produced zero package components, so individual file hashes remain the native artifact identity.

This is a local exception to requiring a package-native SBOM for opaque native binaries. It does not authorize redistribution, production use, public exposure, or reuse outside this exact local tuple.

## G6 thresholds

The narrow pilot passes only if all conditions hold:

1. all four admitted TARGET packets receive exactly one terminal result;
2. all four results are `COMPLETE`;
3. all four results preserve the mechanically supplied consumer relationship;
4. zero results assert authority, acceptance, completeness, ownership, production use, or feature identity;
5. zero malformed, duplicate, wrong-mode, digest-mismatch, or oversized inputs are accepted as semantic work;
6. cancellation and deadline vectors close without successful completion claims;
7. descendant process-group cleanup passes;
8. model/runtime/worker/input digests match the selected bundle;
9. network denial remains active;
10. no source census or absence claim is made.

Any failed condition rejects this narrow pilot run. These thresholds apply only to the local four-packet diagnostic pilot and are not general engineering-retrieval qualification thresholds.

## Decision boundary

These decisions close the local interpretation of G4 and define G6 for the narrow pilot. They do not by themselves authorize execution. A separate enablement record must bind the exact worker, adapter, model, runtime, four inputs, schemas, conformance output, and these decisions by digest.
