# Review request

Requested token: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_TEST_WRITESET_GO`

Current status: `AUTHORED_NOT_GO`.

Review only whether this amendment accurately freezes test ownership for the already accepted design. Do not reopen behavior, numeric policy, architecture, Phase 0, profile authority, or production admission.

## Required reviewer findings

A GO response must state all of the following explicitly:

1. The complete future write-set is exactly the six production targets and four new test targets in `WRITESET.md`; no unlisted production/test path may change.
2. The six production targets are unchanged from the accepted design, including the absent required-new file, conditional successor target, sole-owner restriction, and generated-only registry restriction.
3. Each new test file belongs to its stated existing Go package/seam and may remain a dedicated private same-package file.
4. Every listed RED obligation has exactly one file owner; the disjoint anchored sessionruntime selectors are exactly `^TestADR0011DirectionalRawV1Normal` and `^TestADR0011DirectionalRawV1Race`, and the lspwire prefixes remain separately owned.
5. Race ownership is exclusively `sessionruntime/call_hierarchy_directional_raw_v1_race_private_test.go`; it has no build tags or special compilation semantics, compiles/runs under ordinary package tests, and qualification uses `go test -race` with the anchored race selector while the anchored normal focused command excludes it.
6. Boundary, custody, C15 relationship/non-duplication, disabled/`CALLS_ONLY` compatibility, history/predecessor/lineage immutability, incoming/outgoing independence, release/reuse, and failed-read prefix coverage are complete.
7. Historical tests and qualification files in `DEPENDENCIES.sha256` are pinned dependencies only and must remain byte-identical.
8. Current >16 MiB real-path behavior remains `BLOCKED_NOT_RED`; this amendment does not authorize tests, implementation, execution, activation, or commit.
9. Package mismatch, historical-test mutation, or need for any unlisted path is a hard stop with no fallback widening.
10. Checksums and target HEAD read back exactly.

Only after those findings may the reviewer return the exact token. The token authorizes only adoption of this write-set amendment as a future phase prerequisite; it does not authorize that phase.

Absent that exact reviewed token, remain `AUTHORED_NOT_GO`.
