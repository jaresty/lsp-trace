# ADR 0014 private multi-seed v2 measurement evidence

- **Record:** immutable measurement evidence; corrections require a successor record
- **Repository HEAD measured:** `71aea02ffd1da4042860db697deafc7cb83e7a20`
- **Harness:** `internal/censuscontinuation/programc_v2_multiseed_measurement_test.go`
- **Harness SHA-256 / byte length:** `44e9c3c05acd72fa536d0bc047a9173625e40f29c632f699fded360ad987a1c2` / `29127`
- **Retained output:** `docs/qualification/adr0014-private-multiseed-v2-mechanics-measurement-v1/measurement.json`
- **LF-terminated output SHA-256 / byte length:** `40cdd09eb7d2fc84b97f45ff5672ac73c86d7b19f46e635df636556619f4fa31` / `1073`
- **Raw JSON SHA-256 / byte length:** `b4dbadaa546c8ee6d9f10f77b5b15e04e0032fc1743063caf84602b1821d5964` / `1072`
- **Independent measurement verdict:** `ACCEPT_MEASUREMENT`; isolated review task `feed307e-259e-489`

## Exact invocation

The harness-defined invocation is:

```sh
LSP_TRACE_ADR0014_V2_MEASURE=1 \
LSP_TRACE_PROGRAM_C_RECEIPT=/private/tmp/lsp-trace-census-adaptive5-complete-receipt.json \
LSP_TRACE_PROGRAM_C_LEDGER=/private/tmp/lsp-trace-census-adaptive5-constituent-availability-v1.json \
LSP_TRACE_PROGRAM_C_RETAINED_STATE=/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/TestRunFrozenCorpusProductionCustodyToGapLedger1865247123/001 \
LSP_TRACE_ADR0014_V2_MEASUREMENT_OUTPUT=/private/tmp/adr0014-v2-frozen-measurement.json \
go test ./internal/censuscontinuation \
  -run '^TestADR0014FrozenCorpusMultiSeedV2Measurement$' \
  -count=1 -timeout=2h -v
```

Exact regular non-symlink input files used by the successful invocation were:

```text
receipt path=/private/tmp/lsp-trace-census-adaptive5-complete-receipt.json
receipt sha256=ecc4a9280e1b3896de6c9799485bc642111e4140ccfc0f458e03e6e25441a751
receipt bytes=201291
ledger path=/private/tmp/lsp-trace-census-adaptive5-constituent-availability-v1.json
ledger sha256=2fe7d6d4713e29b85dfeb73b5d9bdf5b7edec39d2cc317f9588d414a0c372bee
ledger bytes=236389
```

The harness independently derives and checks these receipt and ledger SHA-256 values and lengths at execution time and opens retained state through the exact pilot-local path using head namespace `census-program-c-transition/heads/private/v1`.

The retained-state path and state identity are **pilot-local only**. This record does not establish portable retained-state custody, producer authentication, source completeness, or availability outside that pilot path.

## Frozen result

The LF-terminated output is retained byte-for-byte as `measurement.json`. Its frozen primary accounting is:

```text
constituents=395
complete=268
empty=2
resource_incomplete=125
eligible_runs=1608
eligible_run_community_results_D=15537
seed_group_results_G=530232
max_seed_groups/source=63
max_memberships/source=1312
max_memberships/group=154
max_group_occurrences/group=212
max_sum_group_occurrences/source=1120
max_relevant_string_bytes=235
max_communities/run=15
max_members/run=214
max_candidates/community=67
max_composition_occurrences=1120
max_outer_work=7186
max_run_source_work=16021
```

The complete aggregate counters remain authoritative only as bytes in `measurement.json`; this note does not replace or reserialize them.

## Claim ceiling

```text
authority=0
accepted=false
measurement_scope=BOUNDED_FROZEN_CORPUS_RESOURCE_ACCOUNTING
retained_state_custody=PILOT_LOCAL_PATH_ONLY
portable_retained_state_custody=NOT_ESTABLISHED
semantic_authority=NONE
public_authority=NONE
production_admission=NOT_AUTHORIZED
source_graph_complete=UNKNOWN
completeness_outside_exact_verified_terminal_inventory=UNKNOWN
absence_inference=PROHIBITED
```

The verdict accepts this bounded measurement as design evidence only. It does not accept the ADR amendment, qualify an implementation, authorize production code, or establish denominator, lineage, successor, candidate-preparation, CLI, or MCP authority.
