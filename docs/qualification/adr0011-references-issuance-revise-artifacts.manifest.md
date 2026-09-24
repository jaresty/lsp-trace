# ADR 0011 references issuance REVISE — immutable artifact inventory

**Snapshot commit:** `5472748c3a92b31e52d0a24e15be3fa4d1306af7`. Each SHA-256 below hashes the exact file bytes at that commit (`git show <commit>:<path> | shasum -a 256`). This inventory fixes the identity of **draft/reviewed bytes**, not their acceptance, producer authentication, runtime enforcement or production qualification. This inventory file is a later transport/index artifact, not a member of the hashed snapshot; its own identity is the commit that introduces it.

| Artifact at snapshot commit | SHA-256 of exact bytes | Status |
| --- | --- | --- |
| `docs/qualification/adr0011-references-contract-implementation-acceptance.md` | `754cbb79890ec8a6cdb5571a2498734df266f6bc1c922008241f2bb5a65b0d5d` | Earlier implementation contract decision, not occurrence QUALIFIED. |
| `docs/qualification/adr0011-production-admission-gate.review.md` | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` | Accepted baseline contract. |
| `docs/qualification/adr0011-occurrence-falsification-matrix.review.md` | `c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2` | Accepted baseline matrix. |
| `docs/qualification/adr0011-references-issuance-barrier-amendment.proposed.md` | `b1c8eb0cb8a3dd6ad82ce8cf48d7c8fd71813d63c36432b7d50d484100667dc2` | PROPOSED / REVISE; no delta acceptance. |
| `docs/qualification/adr0011-references-diagnostic-policy.proposed.md` | `85be60302536008b7e365106e7c03748f9a75e2a7388885f0648b47ddaa4bf83` | PROPOSED; code disclosure denied by default, no selected access/limits. |
| `docs/qualification/schemas/adr0011-references-issuance-records.proposed.schema.json` | `a4c70c0287a5b3c869fab75d4877e001a7e809655df221b4a26d5e1a65e54aee` | Unregistered shape proposal, not predecessor or producer authority. |
| `docs/qualification/schemas/adr0011-references-private-raw.proposed.schema.json` | `3b1f36e240c7136b977a4f9a18ed6d0f3bb220e47f84241429d3b9fc55845f28` | Unregistered manifest-shape proposal; no owner-bound raw capture. |
| `docs/qualification/schemas/adr0011-references-diagnostic.proposed.schema.json` | `b469b2becabc5b7657648c32a29324e4390f745dab3fd5721527e2c99a1f21bf` | Unregistered shape proposal; rejects code-erased server-error D. |
| `docs/qualification/adr0011-references-issuance-handoff.packet.md` | `614db5d31075af72bd200a666dfca2384292c412222929d01accd9c5e0e85015` | Explanatory handoff, not an acceptance decision. |
| `docs/qualification/adr0011-references-issuance-revise-submission.draft.md` | `45df792920bed067a6b9a5ed95afd47ee6db3ed37ecfc2af8e91930445c9df3a` | REVISE response/worklist; not ready for ACCEPT. |

**Excluded from an acceptance snapshot:** the uncommitted test-owned references implementation in `internal/adr0011methodresult/`, `internal/adr0011acquisition/`, and `cmd/lsp-trace-mcp/`; any unselected source/revision/policy/raw/scanner/event predecessor records or implementation digests; real provider evidence and the 162-row matrix executions. No accepted access roster, global allocation/deadline, private backup/retirement, or quarantine policy bytes appear above. A future submission must pin those separately and get a new independent decision; do not infer `ACCEPT_REFERENCES_ISSUANCE_BARRIER_DELTA_FOR_IMPLEMENTATION` from these hashes.

**Verification:** for each table entry run `git show 5472748c3a92b31e52d0a24e15be3fa4d1306af7:<path> | shasum -a 256` and compare. If any file or policy changes, issue a new manifest instead of overwriting this snapshot. Historical omitted-selector `CALLS_ONLY` must remain byte-compatible. Production references and all downstream gates remain paused.
