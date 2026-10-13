# Harness evidence

Verdict: `INTERFACE_COHERENCE_ONLY`. This is compile/type and package-verifier coherence only, not RED, GREEN, C15 closure, acceptance, implementation, or production readiness.

Exact reproducible command from this package directory:

```sh
python3 verify_package.py --harness
```

The verifier independently reads the committed two-stage `STAGE_A_PLANNED_OBLIGATIONS.tsv` and derives its exact 20-row ID-to-symbol map. It independently parses the byte-identical V3 snapshot, recomputes ID-constant/function adjacency from exact line positions, counts definitions, counts every `m.<symbol>(` call reference across the full snapshot, derives the first exact canonical `compilePlannedV3` call line for each symbol, and counts exact occurrences of that selected line. It then requires every complete `BRIDGE.tsv` row to equal that independently derived record. `BRIDGE.tsv` counts, rules, constants, symbols, IDs, and call sites are never authority for those checks.

The two reference fields are intentionally distinct: `total_reference_count` is all snapshot call references, while `canonical_call_occurrence_count` is the exact occurrence count of the selected canonical planned call line and must equal 1. The non-unit totals are `reserveDirectionalPrepareFrameV3=4`, `transferDirectionalAcquisitionV3=4`, and `releaseDirectionalTerminalV3=3`; all other symbols currently total 1.

Every run performs disposable, in-memory, one-defect negative perturbations. The required assertion-specific observations are:

- false ID → `BRIDGE_EXACT_ID_SET`;
- false constant → `BRIDGE_EXACT_V3_CONSTANT`;
- arbitrary adjacency rule → `BRIDGE_EXACT_ADJACENCY_RULE`;
- arbitrary definition count → `BRIDGE_EXACT_DEFINITION_COUNT`;
- forged total reference count → `BRIDGE_EXACT_TOTAL_REFERENCE_COUNT`;
- forged canonical selected-line occurrence count → `BRIDGE_EXACT_CANONICAL_CALL_OCCURRENCE_COUNT`;
- foreign canonical call → `BRIDGE_EXACT_CANONICAL_CALL_SITE`;
- extra row → `BRIDGE_EXACT_ROW_COUNT`;
- missing row → `BRIDGE_EXACT_ROW_COUNT`;
- duplicate row ID → `BRIDGE_UNIQUE_IDS`.

Each case must print `SELFTEST_FAIL_OBSERVED <case> <assertion>`. The unmodified rows must then print `SELFTEST_RESTORE_PASS`; otherwise verification stops nonzero.

The compile portion creates a fresh temporary module from exact package snapshot bytes, runs `gofmt -w`, `go test ./...`, `go test -run ^$ ./...`, and `go vet ./...`, prints exact snapshot hashes and Go version, then deletes the temporary directory. Successful output ends `INTERFACE_COHERENCE_ONLY` and `BRIDGE_PACKAGE_VERIFY_OK`.

No repository interface, source, or test file is copied from production into the package, modified, or executed as a repository test.

AUTHORED_NOT_GO
