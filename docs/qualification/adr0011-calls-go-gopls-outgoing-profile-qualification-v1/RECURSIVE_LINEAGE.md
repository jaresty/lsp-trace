# Recursive lineage

- Initial untracked outgoing candidate was a prior NO_GO package. Its file hashes are retained in `MANIFEST.json` under `lineage.prior_untracked_no_go_candidate`; `NO_GO.md` was replaced rather than upgraded in place.
- Accepted prerequisite: generic outgoing semantics package for `CALLS_LSP_OUTGOING_EXACT_V1`, committed at prerequisite commit `17057a8c53af8e8fe66be9c37788bbf59d10c5cd`, pinned by `CHECKSUMS.sha256` `8c0efb403147ff70ad4def97809c65ec026c4984fdd8c8b5f7e1f9d0e31de6db`.
- Accepted prerequisite: outgoing custody rerun addendum package, pinned by `CHECKSUMS.sha256` `3ed97c1031986186e5da49e1488f46554314c2bf945f02bca73217146d6abeca`.
- Accepted evidence prerequisite: `OUTGOING_CUSTODY_CORRECTION_EVIDENCE_GO` run at HEAD `03359dc6344ee71c828cc3a22af42af6683333ae`, run id `outgoing-custody-correction-v1`, final checksums `b01bcfd5018432eed725af79a41ce049ac3ba2941d7d2f3a40077367047bb93b`, final seal `a511c0f2ff435a30ba963f7b70671880959cf234bf37c7dc7a195b7886ba795c`, artifacts `53640`.
- Historical outgoing workload is retained only for equality/difference comparison. It is not a custody-role source for this finalizer.
- No incoming provider/profile/evidence authority, incoming ledger, incoming raw corpus, or incoming run dependency is used. Provider-neutral architecture references remain separate from directional provider qualification.
- This package explicitly forbids delegation, commit, incoming authority/profile modification or dependency, composition, bidirectional or ADR0011-to-ADR0007 composition, Stage 8 change, ceiling change, custody-owner change, runtime/public/API/schema/CLI/MCP change, integration, push, publication, release, deployment, and production admission.
- The verifier now asserts exact recursive schema/boundary conditions, canonical semantic keys, exact prerequisite/evidence pins, exact run-comparison identities, and assertion-bound tamper failures rather than relying on checksum-only protection.
