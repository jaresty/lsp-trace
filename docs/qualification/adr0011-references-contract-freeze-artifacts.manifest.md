# ADR 0011 references issuance contract-freeze review snapshot

Reviewed base commit (full Git ID): `2e32f5cb3824d9aa8e745ba73a2ba2782b0d1eac`.

This manifest is a separate later commit to avoid hashing itself. Every SHA-256 below covers the exact full blob at `2e32f5cb3824d9aa8e745ba73a2ba2782b0d1eac:<path>` (including or excluding trailing LF exactly as stored). Recompute with `git show 2e32f5cb3824d9aa8e745ba73a2ba2782b0d1eac:<path> | shasum -a 256`; do not hash the mutable working tree. These nine files alone form this new review set:

| Reviewed artifact | SHA-256 |
| --- | --- |
| `docs/qualification/adr0011-references-contract-freeze.addendum.proposed.md` | `a7624798d351c54dedf7f8bebe1ecb2bd343a616faddcd43ddd76d9752851ad6` |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `659d8395bf7c2efdd0e377fc9eb150ade502112343f2111caff09f6e140532e5` |
| `docs/qualification/policies/adr0011-references-admission-policy-v1.proposed.json` | `9ab611947e26481c0b9f58ba4dbddb3412162dd9de8d0386ebdaff8ff22a578b` |
| `docs/qualification/policies/adr0011-references-method-policy-v1.proposed.json` | `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f` |
| `docs/qualification/policies/adr0011-references-privacy-policy-v1.proposed.json` | `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f` |
| `docs/qualification/policies/adr0011-references-retention-policy-v1.proposed.json` | `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68` |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `9ff7208900fda69c037720b266e7a6f529de150f442d5c2870c53397f60de7cf` |
| `docs/qualification/schemas/references_freeze_contract_test.go` | `d8fec8b85f9f98c61c0bada519b2ded6aa1a14fb6e926c79dfe806e98a8e92d3` |
| `docs/qualification/schemas/references_issuance_proposed_test.go` | `86e8b2c15ccf8dfeadc9f649fafac9f259d515c284dd73c645f3e544d92d94f1` |

Previously accepted references contract/matrix at `ed91b585` retain their historical accepted SHA-256 values and are **not re-accepted or modified** here. Other preexisting privacy, diagnostic and handoff drafts are background, not part of this nine-file new freeze: their dirty working-tree versions are **not** represented by this base or hashed as reviewed artifacts. The malformed-only private-raw draft schema is not the new valid-result raw role; its diagnostic-only scope is unchanged. Historical omitted-selector `CALLS_ONLY` bytes remain unchanged.

Paused test-owned implementation (`cmd/`, `internal/`, sessionruntime), ADR 0007/census, `.pi/evidence`, other unrelated dirty files and untracked producer work are explicitly excluded from review and both commits. This is implementation-contract review only: no producer authentication, source completeness, runtime issuance or full-matrix qualification is asserted. Independent decision requested: `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION | REVISE | REJECT`; no ACCEPT is recorded by this manifest.
