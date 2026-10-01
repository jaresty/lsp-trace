# ADR 0011 — two-prerequisite implementation handoff

## Exact implementation-only contract decisions

- Production admission proposal `adr0011-production-admission-prerequisite.proposed.md` raw SHA-256 `d2989cc6db0d5bb8a389707a34dcf218f49f2927aaba4899395e84e6f5f3956a`: independent bounded review returned `ACCEPT_FOR_IMPLEMENTATION_ONLY`. Earlier two versions returned REVISE; this decision applies only to the final bytes.
- Lifecycle proposal `adr0011-retention-tombstone-prerequisite.proposed.md` raw SHA-256 `fe8206e2c6af38e7869372653f055007699b0c30018a945351c5031599e8df7d`: independent bounded review returned `ACCEPT_FOR_IMPLEMENTATION_ONLY`. Earlier version returned REVISE. Reviews judged contract files, **not** implementation or runtime evidence.

## Overlap and writer ownership

Both require a closed dependency graph and verified no-replace publication/readback in `internal/adr0011acquisition`; admission owns `owner.go`, query/target/response/record replay and `final_record.go`, while lifecycle must inspect the same final ledger's dependency refs and invalidate active inputs on restart. Both touch `internal/publication`'s bound-file/readback/root custody boundaries. `sessionruntime` owns managed keyed request and limits, not lifecycle selection. Therefore two implementation writers would race on the acquisition/publication closure. Use **one writer** for shared interfaces, with independent read-only falsification/review and distinct test/evidence packets for admission and lifecycle. An implementation plan must pin exact touched files and isolate unrelated dirty work before writing.

## Stop boundary

Do not equate contract acceptance with implemented admission, verified cleanup, a production enablement decision or references occurrence qualification. Production remains default-off, historical public RED assertions remain unchanged until a separately authorized enablement gate, rows 015–017 remain unexecuted (0/162), and no live raw authorization, index, definitions, grouping or public selector follows. After bounded implementation and separate falsification packets, stop for independent implementation decisions before revising first-tranche preflight.
