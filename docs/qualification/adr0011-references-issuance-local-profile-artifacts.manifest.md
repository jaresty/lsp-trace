# ADR 0011 — local privacy separation, draft artifact snapshot

**Snapshot commit:** `87f694104e3215daed4c3574b4c323cc639ae5f9`. SHA-256 is of exact `git show <commit>:<path>` bytes. This new snapshot supersedes the *candidate-document view* in `adr0011-references-issuance-revise-artifacts.manifest.md`; that earlier manifest remains a historical snapshot, not current acceptance. This manifest itself is an index created after the snapshot and is pinned by its own commit. **Decision remains `REVISE`; no implementation authorization.**

| Snapshot artifact | SHA-256 |
| --- | --- |
| `docs/qualification/adr0011-local-qualification-privacy-v1.proposed.md` | `a8ecaab2cc02510a863df07ba49be183c99d5f6a18a2722866357eeef54a9089` |
| `docs/qualification/adr0011-references-issuance-barrier-amendment.proposed.md` | `f00970f774dbce8ee25f3486e3b5b770e143da16e38a3d6ef9866216aab59cc2` |
| `docs/qualification/adr0011-references-diagnostic-policy.proposed.md` | `54f6ba79d6fb4cea82dcb1f01d89cc7332a500d0e793dbc71bb14be4449221ce` |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `560b5a79b50d973a650e6fd9a2d31bb17910b21bbcb5a811eb814cbdbec682dc` |
| `docs/qualification/adr0011-references-issuance-handoff.packet.md` | `72fba30e4c958e2d2766e19c4765dbd3631b17ca249fc4daaaaf2292d7f124d6` |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `a4c70c0287a5b3c869fab75d4877e001a7e809655df221b4a26d5e1a65e54aee` |
| `docs/qualification/schemas/adr0011-references-private-raw.proposed.schema.json` | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` |
| `docs/qualification/schemas/adr0011-references-diagnostic.proposed.schema.json` | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` |
| `docs/qualification/adr0011-production-admission-gate.review.md` | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |
| `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` |

**Nonclaims and next gate:** `LOCAL_QUALIFICATION_PRIVACY_V1` is proposed, not independently selected or implemented. The three modes distinguish default in-memory operation, self-authored secret-free synthetic fixtures, and explicit opt-in same-user live capture. No organizational IAM or permanent privacy quarantine is required for this local-only design; bounded issuance transaction integrity and verified final receipt remain mandatory. The scanner/event seam, source/revision/policy/raw predecessor records, measured independent limits and paused uncommitted implementation are not pinned by this manifest. Hosted/shared/CI-retained capture needs a separate policy. The next independent implementation-contract review must examine these exact bytes plus the missing contracts and return `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION | REVISE | REJECT`; production qualification remains a later gate. Historical omitted-selector `CALLS_ONLY` compatibility is unchanged.

Verify each row using `git show 87f694104e3215daed4c3574b4c323cc639ae5f9:<path> | shasum -a 256`. A later edit requires a new snapshot; never rewrite the previous digest claims.
