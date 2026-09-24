# ADR 0011 — references issuance-barrier implementation-contract review, custody correction

**Reviewed base (full commit):** `e07c6cf77de4177627b7f32c84fb34010147faf9`.

**Independent decision requested, not recorded:** `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION | REVISE | REJECT`. The prior host-Git snapshot at `8c30196bef7dde71b11e53572d4bed4d566cf33a` returned **REVISE**: it encoded exact commands/outputs but omitted the accepted distinct `HOST_OBSERVED_GIT` custody. This snapshot adds the closed mandatory custody field, separated from the caller-asserted workspace revision, and synthetic rejection controls. All earlier manifests retain their own historical meaning; none hashes this corrected base.

The first three entries are accepted unchanged baselines and are not re-opened. The addendum, issuance schema, selected four policy-byte artifacts and tests are the prospective implementation delta. Other included documents, diagnostic schemas and tests are cited supporting context. The addendum controls older proposed wording but cannot override an accepted baseline. Paused uncommitted producer code and unrelated dirty files are excluded. The selected local single-user scope is complete; hosted/shared/remote modes are unsupported, not deferred implementation prerequisites.

SHA-256 is over the exact `git show e07c6cf77de4177627b7f32c84fb34010147faf9:<path>` blob, without newline normalization. Verify **all 21** entries before independent disposition; this manifest is committed separately to avoid self-reference.

| Reviewed path | SHA-256 |
| --- | --- |
| `docs/qualification/adr0011-production-admission-gate.review.md` | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |
| `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` |
| `docs/qualification/adr0011-references-contract-implementation-acceptance.md` | `754cbb79890ec8a6cdb5571a2498734df266f6bc1c922008241f2bb5a65b0d5d` |
| `docs/qualification/adr0011-local-qualification-privacy-v1.proposed.md` | `19ff9197bd09c34aae65c67722090e075764c7f8f560526eca738872b5df2921` |
| `docs/qualification/adr0011-references-contract-freeze.addendum.proposed.md` | `f3c44be71f3a909f95a969a15e01ce6edd4d1ad720fb08c47a6ff71ae70b0bb6` |
| `docs/qualification/adr0011-references-diagnostic-policy.proposed.md` | `487fd5b103ed27523cff5911b862c45ea1533893a9e57c8aa1ec48bf9981c051` |
| `docs/qualification/adr0011-references-issuance-barrier-amendment.proposed.md` | `9a06c7c95decda2e17ed8266318ddf0f2764b62afe9bb296006338f285b6917d` |
| `docs/qualification/adr0011-references-issuance-handoff.packet.md` | `be1247f39bb463b7dd9927e60aecb9bbc3ba9e483cb919bdc5a72e3036da4eb0` |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `487ac8fa141b6af41b37835c9459bdd84a524065fa4babd82d7e6a1da4b47c4d` |
| `docs/qualification/policies/adr0011-references-admission-policy-v1.proposed.json` | `483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3` |
| `docs/qualification/policies/adr0011-references-method-policy-v1.proposed.json` | `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f` |
| `docs/qualification/policies/adr0011-references-privacy-policy-v1.proposed.json` | `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f` |
| `docs/qualification/policies/adr0011-references-retention-policy-v1.proposed.json` | `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68` |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `88c3f91c67b9abcefe59e33a5a5c8460ded4a021a8ffce3de211d7e8a79bea9f` |
| `docs/qualification/schemas/adr0011-references-diagnostic.proposed.schema.json` | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` |
| `docs/qualification/schemas/adr0011-references-private-raw.proposed.schema.json` | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` |
| `docs/qualification/schemas/references_freeze_contract_test.go` | `8700ff032e82ee78fcb7f7ac04ee73dcf318882d62709bae79251fedef4e17ee` |
| `docs/qualification/schemas/references_issuance_proposed_test.go` | `6bc5b8cba9c95b95f464b3ead0b3890e1092558c3c2c42fad4094d4f9380ee71` |
| `docs/qualification/schemas/references_diagnostic_proposed_test.go` | `2a67736d7fb357b95353ce08d0a4e6578b5d79da3d21f6a0d4936015b5f3c1fa` |
| `docs/qualification/schemas/references_private_raw_proposed_test.go` | `616ee101e1b3f9f0d7ae1182adee9e7ce7a07b6477e2cf1b2d5e71b5f3a59c7d` |
| `docs/qualification/schemas/references_host_git_contract_test.go` | `ba17eae44d962aa5b00d9736d79390ddb9849dabddabdeb980cc2afb39fa56ae` |

Offline `go test ./docs/qualification/schemas -count=1` passed 44 tests; offline `go vet ./docs/qualification/schemas` and `git diff --check` passed. These are proposed shapes and synthetic counterexamples, not producer authentication or real clean Go/gopls evidence. Implementation remains paused pending an independent ACCEPT of this exact package. Neither implementation acceptance nor this manifest qualifies the 162-row matrix, definitions, grouping, held-out CUE, Leiden, CLI/MCP, or `ADR_0011_COMPLETE`. Maintain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, `NO_PRODUCER_AUTHENTICATION`.
