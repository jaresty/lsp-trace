# ADR 0011 — references issuance-barrier exact-predecessor review

**Reviewed base (full commit):** `c817e09f5d7235309f3b0e6a29ebea476f67f5f2`.

**Independent decision requested, not recorded:** `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION | REVISE | REJECT`. The prior 21-file custody snapshot returned **REVISE**: exact write parameters and the subordinate document-symbol target result/selection did not have complete prospective replay identities. This snapshot adds mandatory write-body/params byte spans, keyed wire ID, retained target-result bytes, exact target query/selection fields and identity formulas, plus synthetic counterexamples. Earlier manifests remain historical, not current review hashes.

The first three files are accepted unchanged references baselines, not re-opened. The addendum, issuance schema, four selected policy byte files and tests constitute the prospective implementation delta; other included documents and diagnostic artifacts are cited supporting context. An addendum cannot override an accepted baseline. The paused uncommitted producer, unrelated dirty files, real provider results and unsupported hosted/shared/remote deployment modes are excluded. No schema or synthetic test authenticates a producer or qualifies an occurrence.

SHA-256 values below cover exact `git show c817e09f5d7235309f3b0e6a29ebea476f67f5f2:<path>` blobs without newline normalization. Independently verify **all 22** entries before disposition. This manifest is committed separately to avoid self-reference.

| Reviewed path | SHA-256 |
| --- | --- |
| `docs/qualification/adr0011-production-admission-gate.review.md` | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |
| `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` |
| `docs/qualification/adr0011-references-contract-implementation-acceptance.md` | `754cbb79890ec8a6cdb5571a2498734df266f6bc1c922008241f2bb5a65b0d5d` |
| `docs/qualification/adr0011-local-qualification-privacy-v1.proposed.md` | `19ff9197bd09c34aae65c67722090e075764c7f8f560526eca738872b5df2921` |
| `docs/qualification/adr0011-references-contract-freeze.addendum.proposed.md` | `c5ea950dfafc9afcef12087c3c9685d19c6fbb4a0df0782a23a290d746c53c89` |
| `docs/qualification/adr0011-references-diagnostic-policy.proposed.md` | `487fd5b103ed27523cff5911b862c45ea1533893a9e57c8aa1ec48bf9981c051` |
| `docs/qualification/adr0011-references-issuance-barrier-amendment.proposed.md` | `9a06c7c95decda2e17ed8266318ddf0f2764b62afe9bb296006338f285b6917d` |
| `docs/qualification/adr0011-references-issuance-handoff.packet.md` | `be1247f39bb463b7dd9927e60aecb9bbc3ba9e483cb919bdc5a72e3036da4eb0` |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `81dc81925f79f672448e1530c1b24acdb036f6ebf49d975dcdf69843aa18bf33` |
| `docs/qualification/policies/adr0011-references-admission-policy-v1.proposed.json` | `483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3` |
| `docs/qualification/policies/adr0011-references-method-policy-v1.proposed.json` | `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f` |
| `docs/qualification/policies/adr0011-references-privacy-policy-v1.proposed.json` | `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f` |
| `docs/qualification/policies/adr0011-references-retention-policy-v1.proposed.json` | `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68` |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `bf49de9460fee5a68ba13204f633dc72d2dca566fa5f92d704bc19b5514618ba` |
| `docs/qualification/schemas/adr0011-references-diagnostic.proposed.schema.json` | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` |
| `docs/qualification/schemas/adr0011-references-private-raw.proposed.schema.json` | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` |
| `docs/qualification/schemas/references_freeze_contract_test.go` | `8700ff032e82ee78fcb7f7ac04ee73dcf318882d62709bae79251fedef4e17ee` |
| `docs/qualification/schemas/references_issuance_proposed_test.go` | `820a1638305e52bf010e9220281c12f41b6984f3cd2d0048ed0e38e34f9a9fd6` |
| `docs/qualification/schemas/references_diagnostic_proposed_test.go` | `2a67736d7fb357b95353ce08d0a4e6578b5d79da3d21f6a0d4936015b5f3c1fa` |
| `docs/qualification/schemas/references_private_raw_proposed_test.go` | `616ee101e1b3f9f0d7ae1182adee9e7ce7a07b6477e2cf1b2d5e71b5f3a59c7d` |
| `docs/qualification/schemas/references_host_git_contract_test.go` | `ba17eae44d962aa5b00d9736d79390ddb9849dabddabdeb980cc2afb39fa56ae` |
| `docs/qualification/schemas/references_predecessor_contract_test.go` | `80b386b94f177cdf5f946edca10da0ccb1adee71687f686b302e633bf52ae8db` |

Offline `go test ./docs/qualification/schemas ./internal/adr0011querytarget -count=1` passed 70 tests; offline `go vet ./docs/qualification/schemas` and `git diff --check` passed. The test-owned selector is a synthetic behavior control, not an admitted producer. Implementation remains paused pending exact independent **ACCEPT**. Even ACCEPT permits implementation/testing only; full 162-row real clean Go/gopls qualification, definitions, grouping, held-out CUE, Leiden and CLI/MCP exposure remain separately gated. Keep `authority=0`, `accepted=false`, `completeness=UNKNOWN`, `NO_PRODUCER_AUTHENTICATION`; do not emit `ADR_0011_COMPLETE`.
