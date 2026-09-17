# ADR 0008 common source-projection qualification review

## Review identity

- Reviewed revision: `6020d43fcb09bcee4f6b5cdcd6cd0c7d2780038d`
- Worktree: `/Users/schwa/dev/lsp-trace`
- Execution record: `qualification/adr0008-common-projection.execution.6020d43fcb09bcee4f6b5cdcd6cd0c7d2780038d.json`
- Governing matrix: `qualification/adr0008-source-projection-matrix.v1.json` (unchanged)
- Historical full review: `qualification/ADR0008-SOURCE-PROJECTION-QUALIFICATION-REVIEW-5d392f97.md` (unchanged)
- Scope: `COMMON_PROJECTION`, cells `C01–C06`
- Result: **BLOCKED**; `common_projection_qualified=false`; `implementation_qualified=false`

## Revision boundary

Revision `6020d43fcb09bcee4f6b5cdcd6cd0c7d2780038d` contains both additive paging activations:

- LIVE operation 36 through explicit paging and source-projection V3;
- RETAINED operation 41 through explicit paging and the same shared V3 pager.

Requests without paging retain their predecessor paths. Paging adds no graph facts and preserves `authority=0`, `source_graph_complete=UNKNOWN`, and `graph_facts_added=0`.

## Commands executed

| Purpose | Command | Outcome |
|---|---|---|
| Common projection and custody-specific runtimes | `go test -count=1 -v ./internal/sourceposition ./internal/sourceprojection ./internal/sourceprojectionv2 ./internal/sourceprojectionv3 ./internal/liveprojection ./internal/retainedprojection ./internal/retainedoperation` | PASS |
| Contract, MCP, gateway, process, and CLI surfaces | `go test -count=1 -v ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace-mcp ./cmd/lsp-trace` | PASS |
| Full repository suite | `go test -count=1 ./...` | PASS |
| Release guard | `./scripts/release-check.sh` | PASS, ended `RELEASE CHECK PASS` |

The execution record binds SHA-256 digests for all four logs and all eleven `crossmode-v2` fixture files.

## Verdict summary

| Verdict | Count | Cells |
|---|---:|---|
| PASS | 1 | C03 |
| FAIL | 0 | none |
| BLOCKED | 5 | C01, C02, C04, C05, C06 |
| NOT_RUN | 0 | none |

The common track remains BLOCKED because its barrier rule requires every cell to PASS at one reviewed revision.

## Cell adjudication

### C01 — BLOCKED

Current execution proves exact cross-mode fixture artifacts, canonical ordering, exact retained/live custody bindings, and custody-specific physical identities. It does not independently mutate every identity field, and it lacks one byte-level permutation oracle spanning every projected-unit form. The complete C01 predicate is therefore not satisfied.

### C02 — BLOCKED

Current execution adds explicit UTF-8, UTF-16, UTF-32, LF, CR, CRLF, non-BMP boundary, overlapping span, and endpoint/relation citation evidence. It still lacks an explicit BOM vector and one integrated corpus proving every empty/cross-line whole range with complete independent citation attribution. C02 remains BLOCKED.

### C03 — PASS

One revision now executes every C03 clause:

- canonical selection and reconciliation (`candidates=selected+omitted`);
- one mutually exclusive cause per omission;
- exact byte, range, object, and work boundaries;
- zero as a real bound;
- whole-range and whole-record atomicity;
- cumulative page, page-count, response-byte, object, range, source-byte, and work accounting;
- complete encoded response-byte accounting including continuation cursors;
- deterministic replay and request/custody/limit-bound cursor continuation;
- overflow, oversized record, duplicate record, invalid JSON, tamper, and cumulative-limit closure;
- LIVE and RETAINED runtime continuation through the shared pager.

No retry, repair, hidden limit increase, or budget reset was observed.

### C04 — BLOCKED

Graph-byte neutrality, authority zero, unknown source completeness, and zero added graph facts are repeatedly asserted. The packet does not explicitly execute every required source addition, removal, text, reordering, and overlap mutation against one before/after graph-byte oracle. C04 remains BLOCKED.

### C05 — BLOCKED

Metadata-only defaults, explicit body opt-in, privacy-before-acquisition, zero retained lookup for metadata-only requests, and raw-supply/private-diagnostic non-serialization all execute. The complete restricted, withheld, ancillary, unavailable, unauthorized, arbitrary-path, secret, command, environment, and raw-error marker corpus does not. C05 remains BLOCKED.

### C06 — BLOCKED

Current execution covers malformed carriers, mixed carriers, invalid UTF boundaries, missing bindings, duplicate/invalid pager records, changed request/custody/limits, cursor tamper, overflow, oversized atomic records, and budget failures. It does not provide the matrix-required exhaustive assertion-specific RED corpus for every unknown/duplicate/missing field, invalid identity/range/encoding/privacy/status/limit, and reordered/mixed-page case. C06 remains BLOCKED.

## Global status

- `COMMON_PROJECTION`: **BLOCKED**
- Historical `L04`: remains **FAIL**; this review neither reruns nor revises it.
- `implementation_qualified`: **false**
- Canonical operations: `41`
- Compact tools: `12`
- Operation 44: absent/forbidden
- Schema evolution: additive only

## Claim ceiling

This review qualifies C03 only. It does not qualify the full common projection track, retained or live production behavior, ADR 0007 interoperability, source completeness, authority, acceptance, shipment, or deployment. Passing broad tests does not promote any other cell.
