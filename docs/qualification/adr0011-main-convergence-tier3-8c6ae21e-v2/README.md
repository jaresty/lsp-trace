# ADR0011 main-convergence exact-candidate Tier 3 dormant-integration qualification v2

## Decision

`ADR0011_MAIN_CONVERGENCE_CANDIDATE_TIER3_DORMANT_INTEGRATION_AUTHORED_GO`

This is a bounded authoring/qualification result for **dormant generic integration**, not production composition. Source commit `8c6ae21e3709b80f6abae0ca8b1df185d81d99d9`, tree `712a950f7f6c0b1c6a09f6d8178808dca88dbc0d`, immediate parent `320edd24aa02cceb0a88d3d5284728ee026da59e`. The underlying convergence merge is `2e38a92084073a676803465eee6177ec5c72abae`, with frozen parents `6fd25ba4dddb43b191316e3d01dc8c96b5e3b066` and `57225a1e9e7282ed7f58a7a19f8ca538c3d6105e`.

Qualification ceilings remain exactly:

- `authority=0`
- `accepted=false`
- `completeness=UNKNOWN`
- `featureIdentity=UNRESOLVED`
- `production_admission=REFUSED_MISSING_EVIDENCE`

The private ADR0011→ADR0007 chain is reference/test evidence only. This package proves preservation of the inactive/test-only composition seam under affected-scope gates. It proves no production registration or advertisement, no directional profile activation, no production-equivalent runtime chain, no source completeness, and no feature identity. It does **not** claim whole-repository green or production composition.

## v1 disposition and immutability

The untracked predecessor package `docs/qualification/adr0011-main-convergence-tier3-8c6ae21e-v1/` is blocked and superseded by this v2 only for Tier-3 accounting/reporting. It remains authoritative for its retained raw runs and failure logs and was not edited. Its decision was `BLOCKED_TEST_ONLY_COMPOSITION_AND_MISSING_PRODUCTION_EVIDENCE` because its README conflated two distinct frozen catalogs. `logs/v1-manifest-before.txt` and `logs/v1-manifest-after.txt` verify every file named by v1 `MANIFEST.sha256`; `logs/v1-manifest-declared-before.sha256` and `logs/v1-manifest-declared-after.sha256` are byte-identical snapshots of that declaration, and the manifest file’s own before/after SHA-256 receipts are identical.

Both v1 and v2 are untracked at the stated HEAD/tree. Their byte integrity is self-manifest-backed, not Git-tree-backed.

## Resolved accounting without normalization

The independently accepted report is retained byte-for-byte as `ACCOUNTING_RESOLUTION.md`, with provenance in `logs/accounting-report-provenance.txt`. `verify_accounting.py` and `logs/accounting.log` independently fail closed on the exact tracked bytes.

- Frozen C15 **v1 predecessor** candidate catalog: `sessionruntime/testdata/c15-census-v1/candidates.tsv`; 3,805 headerless two-field rows; 541,478 bytes; SHA-256 `13d45a61cb9ad2e5dad8d944458a9b7f459c9a2cd0336cb76700fa801599ca6e`.
- Authoritative frozen C15 **v2 successor** candidate catalog: `sessionruntime/testdata/c15-census-v2/candidates.tsv`; 3,847 headerless two-field rows; 547,527 bytes; SHA-256 `536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2`.
- Frozen v2 is frozen v1 plus exactly 42 IDs: 18 `ephemeral-runtime`, 24 `external-library`; all are `excluded` with empty owner and nonempty category/reason.
- Removed IDs: 0. Shared candidate changes: 0. Shared classification changes/reclassifications: 0.
- Current main-convergence enumeration is a separate 4,155-row dataset; the locations file has one header plus 4,155 data rows.

No count is normalized and the bare phrase “historical census” is intentionally avoided.

## Resolved overlap semantics and parent differential

The six overlap paths preserve the semantic union recorded by the blocked v1 evidence:

1. ADR0011 document retains bounded definition/reference grouping design together with authority ceilings, no-CALLS promotion, and separately authorized future surfaces.
2. `internal/integratedconformance/harness_test.go` retains the ownership union while declaring disabled hermetic reference tests and no production registration, advertisement, containment, or Stage 2 authority.
3. `internal/censuscontinuation/continuation_test.go` retains continuation assertions.
4. `internal/censuscontinuation/stage_matrix_test.go` retains stage-matrix assertions.
5. `sessionruntime/b4_explicit_bytes_c15_private.go` retains the C15 explicit-byte implementation union.
6. `sessionruntime/diagnostic_backing_c15_private.go` retains the C15 diagnostic-backing union.

`logs/overlap-blob-matrix.tsv` pins every overlap blob at both frozen parents, the convergence merge, immediate parent, and candidate. Every candidate overlap blob equals the immediate-parent blob, so cleanup commit `8c6ae21e` introduces no overlap-path change. `logs/source-and-parent-differential.txt` records exact commit/tree/parents, merge base, immediate-parent name-status, and parent-vs-candidate stats. Candidate affected-scope tests then execute the resolved semantics; this is the bounded no-regression basis and is not a whole-repository claim.

## Dormancy and production refusal

`logs/authority-and-dormancy.txt` retains the exact source excerpts. The integrated harness owns no production registration or advertisement. The ADR keeps omitted selection `CALLS_ONLY`, keeps historical CALLS-only bytes unchanged, and requires separate authorization for future CLI/MCP surfaces. The pilot remains `PILOT_DISABLED`. No directional definition/reference/mixed profile activation is admitted.

Accordingly, dormant generic integration is authored and affected-scope qualified, while production admission remains `REFUSED_MISSING_EVIDENCE`.

## Affected-scope verification

Fresh candidate reruns retained command, environment, UTC timestamps, elapsed nanoseconds, exit status, and output:

- strict role-labeled accounting verifier: PASS;
- ADR0007 production-ownership registration test: PASS;
- ADR0007 exclusions-remain-unowned test: PASS;
- ADR0007 adjacent-path rejection test: PASS;
- complete `internal/integratedconformance` package: PASS;
- complete `internal/censuscontinuation` package: PASS;
- `scripts/check-ci.sh`: PASS;
- frozen-v2 successor test: retained FAIL, classified below.

## Failure boundary

`TestPrivateB4C15V2SuccessorCatalog` currently reports `ASSERT_C15_V2_FINAL_CANDIDATES got=4155 want=3847`. The test and frozen catalogs are byte-identical to immediate parent `320edd24`; the test intentionally freezes the older 3,847-row successor contract while the current generator enumerates 4,155 rows. This inherited frozen-contract/current-generator divergence is retained without repair, regeneration, or acceptance. It does not contradict the direct exact-byte validation of frozen v2 and does not qualify the current generator as frozen v2.

Repository-wide failures from blocked v1/Tier 2 remain classified, not accepted: inherited qualification-package compile boundaries; intentional named RED/prospective policy tests; a ten-minute top-level timeout after reported failures; and platform-unavailable `@glint/core`. This v2 did not rerun exhaustive repository-wide selectors because no source/test/fixture/historical artifact changed and the tiered acceptance boundary is affected scope. No failure is promoted to acceptance and no whole-repository-green claim is made. See `FAILURE_CLASSIFICATION.md`.

## Reproduction and integrity

Exact commands are in `COMMANDS.txt`. `MANIFEST.sha256` covers every v2 file except itself and is verified directly with `shasum -a 256 -c`. Deterministic generated artifacts are checked by rerunning the accounting verifier and comparing receipts; `logs/accounting-repeat.diff` must be empty. Final status is restricted to exactly the untracked v1 and v2 package directories.
