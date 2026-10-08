# ADR0007 v4 manifest roles before final freeze

`PAYLOAD_MANIFEST.json` and `TOOLING_CENSUS.json` are preserved byte-for-byte as inner identity inputs for existing v4 terminal replay and validator fixtures. Their historical excludes are not outer-scope omission claims.

`PRE_FREEZE_MANIFEST.json` is the deterministic outer pre-freeze census for normative v4 bytes. It covers the v4 documentation/contracts/schema/review evidence/cases/oracle expected outputs/bundles/provenance/manifests, production/oracle/contract-validator packages, v4 commands, pinned sourceadmission bytes, dependency locks, and predecessor evidence. The final `FREEZE.json` remains intentionally absent and outside this pre-freeze root.

`FREEZE_ENVELOPE.dryrun.json` is a verifier input for the final-envelope dry-run mutation only; it is not the final freeze envelope and is excluded from the pre-freeze root to avoid embedding the root inside itself.
