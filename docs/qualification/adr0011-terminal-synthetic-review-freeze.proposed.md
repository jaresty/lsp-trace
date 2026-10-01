# ADR 0011 terminal synthetic review — immutable input freeze (proposal only)

Disposition entering review: `ACCEPT_FOR_NEXT_SYNTHETIC_REVIEW_STAGE_ONLY`. This file freezes the **exact uncommitted, unaccepted** inputs for a read-only terminal synthetic review. The hashes below are SHA-256 of complete file bytes (not JSON reserializations or excerpts); reject review if any byte count or hash differs. This freeze file is not included in its own pins. None of these files is authorized for production admission, public exposure, lifecycle merge, qualification-row accounting, commit, or push.

| Role | Path | Bytes | SHA-256 |
|---|---|---:|---|
| Proposed admission schema | `docs/qualification/schemas/adr0011-production-admission-v1.proposed.schema.json` | 32197 | `7dff6ab60810b5b01439f506f178b46c7afb562d52dadb8f6065222c3a8d7df8` |
| Synthetic replay fixture and oracle | `docs/qualification/schemas/adr0011_prerequisite_replay_proposed_test.go` | 143413 | `025de0da797acc8264009f15f8452cfd99147f586bc27d536ac1cb9119e4936d` |
| Identity-role proposal | `docs/qualification/adr0011-production-record-identity-delta.proposed.md` | 20540 | `77a9ade3e568f439e0b1d3cf22a2500dfefc5af7302f2ec7d61646988ec0c3ba` |
| Synthetic replay evidence | `docs/qualification/adr0011-production-admission-schema-evidence.proposed.md` | 26971 | `6442c90723a108db6fa2c45cd07f71e85f72295888ae0b863abd8ee732933fe5` |
| Prior prerequisite and review manifest | `docs/qualification/adr0011-prerequisite-review-manifest.proposed.md` | 14375 | `4c9e13a10798c19138c8af4ab5540951384d8fcf83b8bcc58627f60a41370b9e` |
| Prior production-admission prerequisite | `docs/qualification/adr0011-production-admission-prerequisite.proposed.md` | 9222 | `d2989cc6db0d5bb8a389707a34dcf218f49f2927aaba4899395e84e6f5f3956a` |
| Accepted admission gate (review baseline, not proposal acceptance) | `docs/qualification/adr0011-production-admission-gate.review.md` | 14438 | `d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9` |

Parent verification at freeze: `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./docs/qualification/schemas -run '^TestADR0011OneLocation' -count=1` — 17 passed; full `go test ./docs/qualification/schemas -count=1` — 187 passed; `git diff --check` passed. The five principal proposal files are untracked and uncommitted. These tests are synthetic and in-memory; they do not establish host custody, no-replace publication, fresh external readback, real provider observations, or a qualified row.

## Terminal synthetic review request

Independently verify the pins before inspecting complete proposal/schema/fixture bytes and relevant historical predecessor schemas. Decide `ACCEPT_TERMINAL_SYNTHETIC_ONLY` or `REVISE`; cite concrete lines and a schema-valid, coherently rehashed counterexample for any rejection. Specifically:

1. Reconcile original bytes and role/schema IDs through pre-invocation custody, declaration, keyed WRITE/READ, source, target, result, historical proposal/candidate/events, attempt manifest, terminal ledger, admitted-occurrence ledger, accounting record, inventory and final. Derive N/B/T/E/E_B/E_T/P/A and the one admitted occurrence; do not infer production custody from constructed in-memory maps.
2. Verify exactly 21 immediate final refs; the three ancillary ledger/accounting refs and distinct attempt manifest remain inventory selections. Confirm `accounting_digest` hashes canonical accounting-record bytes, not a surrogate; the old P=0/P=A=1 fixture is not counted as a positive.
3. Audit **remaining identity-field mutation coverage**, distinguishing an isolated final-field mismatch from a coherent mutation with rehashed dependent original records. Include transaction/query occurrence/declaration IDs, request key, invocation ID, session/generation and wire ID, target/result/source identity, occurrence ID/ordinal/range/URI, policy/schema/implementation digests, manifest and inventory identity, accounting digest and selector/length/ref identities. For each field family, report tested, checked but not attacked, or absent; identify the first intended guard and whether preceding guards would pass. Do not invent a live evidence source to close a synthetic gap.
4. Challenge strict WRITE and READ envelopes, matched key, exact READ-result/raw-payload bytes, events/journal/terminal state, admitted subset, canonical original bytes and final readback boundary with independently chosen rehashed substitutions. Separate early selected-original rejection from semantic arithmetic rejection.
5. State exactly what this synthetic review establishes and what remains blocked for production, lifecycle, real rows, and publication. Do not alter pinned inputs during review; a correction requires a new freeze and independent re-review.
