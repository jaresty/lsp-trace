# ADR 0011 references issuance-barrier — expanded implementation-contract review snapshot

**Reviewed base (full commit):** `b477ed9dda7a3e9c1d9957423f8ff099e8d6c109`.

**Decision requested, not recorded:** `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION | REVISE | REJECT`. This is a local, single-user **implementation-contract** review, not producer conformity, runtime qualification, `REFERENCES_SYMBOL_V1` enablement, or `ADR_0011_COMPLETE`. The earlier `adr0011-references-contract-freeze-artifacts.manifest.md` remains a historical nine-file snapshot at its own base; do not use its hashes for this expanded review.

The three `.review.md`/acceptance files below are the **previously accepted, unchanged boundary**; including their exact bytes does not re-open or re-accept them. The addendum, selected policy JSON, issuance schema, and contract tests are the proposed normative implementation delta. Other proposed markdown and diagnostic/private-raw schema/tests are **cited supporting context**: if they conflict with the accepted boundary, stop; if older draft wording conflicts with the addendum's prospective requirements, the addendum controls this implementation-only submission. The paused, uncommitted references producer and all unrelated dirty files are excluded; cited runtime seams are observations, not reviewed implementation bytes.

Each SHA-256 below is over the exact blob from `git show b477ed9dda7a3e9c1d9957423f8ff099e8d6c109:<path>` (no newline normalization). A reviewer must recompute and compare **all 20** values before deciding. Manifest text lives in a later, separate commit to avoid self-reference.

| Reviewed path | SHA-256 |
| --- | --- |
| `docs/qualification/adr0011-production-admission-gate.review.md` | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |
| `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` |
| `docs/qualification/adr0011-references-contract-implementation-acceptance.md` | `754cbb79890ec8a6cdb5571a2498734df266f6bc1c922008241f2bb5a65b0d5d` |
| `docs/qualification/adr0011-local-qualification-privacy-v1.proposed.md` | `19ff9197bd09c34aae65c67722090e075764c7f8f560526eca738872b5df2921` |
| `docs/qualification/adr0011-references-contract-freeze.addendum.proposed.md` | `133a80005632698ab659edf6d76788d86054c715d6aa7873f2e1891703d91819` |
| `docs/qualification/adr0011-references-diagnostic-policy.proposed.md` | `487fd5b103ed27523cff5911b862c45ea1533893a9e57c8aa1ec48bf9981c051` |
| `docs/qualification/adr0011-references-issuance-barrier-amendment.proposed.md` | `9a06c7c95decda2e17ed8266318ddf0f2764b62afe9bb296006338f285b6917d` |
| `docs/qualification/adr0011-references-issuance-handoff.packet.md` | `be1247f39bb463b7dd9927e60aecb9bbc3ba9e483cb919bdc5a72e3036da4eb0` |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `bb56f523b3e8500bbd13c1a3205cbc0ce86a0f823b3998b02f5b890e597c1981` |
| `docs/qualification/policies/adr0011-references-admission-policy-v1.proposed.json` | `483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3` |
| `docs/qualification/policies/adr0011-references-method-policy-v1.proposed.json` | `2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f` |
| `docs/qualification/policies/adr0011-references-privacy-policy-v1.proposed.json` | `8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f` |
| `docs/qualification/policies/adr0011-references-retention-policy-v1.proposed.json` | `5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68` |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `9ff7208900fda69c037720b266e7a6f529de150f442d5c2870c53397f60de7cf` |
| `docs/qualification/schemas/adr0011-references-diagnostic.proposed.schema.json` | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` |
| `docs/qualification/schemas/adr0011-references-private-raw.proposed.schema.json` | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` |
| `docs/qualification/schemas/references_freeze_contract_test.go` | `8700ff032e82ee78fcb7f7ac04ee73dcf318882d62709bae79251fedef4e17ee` |
| `docs/qualification/schemas/references_issuance_proposed_test.go` | `86e8b2c15ccf8dfeadc9f649fafac9f259d515c284dd73c645f3e544d92d94f1` |
| `docs/qualification/schemas/references_diagnostic_proposed_test.go` | `2a67736d7fb357b95353ce08d0a4e6578b5d79da3d21f6a0d4936015b5f3c1fa` |
| `docs/qualification/schemas/references_private_raw_proposed_test.go` | `616ee101e1b3f9f0d7ae1182adee9e7ce7a07b6477e2cf1b2d5e71b5f3a59c7d` |

**Verification boundary:** focused offline `go test ./docs/qualification/schemas -count=1` (26 tests) and offline `go vet ./docs/qualification/schemas` passed at the reviewed base. These are draft contract/shape counterexamples, **not** producer execution, independently authenticated LSP observations, full 162-row qualification, or evidence that a live final can issue `T=1,A=P`. Before independent ACCEPT the paused writer stays paused; any implementation acceptance authorizes implementation/testing only.
