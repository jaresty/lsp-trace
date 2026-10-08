# ADR0007 source text search private v4 strict contract

This directory is a prospective, implementation-independent v4 wire contract. It does not edit production, oracle, corpus attempts, generated oracle output, evaluator/oracle commands, freeze state, or GO state. v1-v3 are predecessor-locked inputs only.

## Validator input bundle

The standalone validator consumes an explicit bundle, not trust-only terminal fields:

- `raw_attempt_bytes`
- `terminal_bytes`
- `admitted_source_bytes`
- `admitted_binding_bytes`
- `tooling_manifest_bytes`
- `predecessor_manifest_bytes`
- `payload_freeze_binding_bytes`
- `schema_bytes`

The validator recomputes all pins from those bytes. Attempt digest is the SHA256 of exact raw attempt bytes. Admitted binding digest is the canonical admitted record plus LF, or `sha256:` plus sixty-four zeroes before admission. Tooling, predecessor, payload/freeze-binding, and schema bytes are supplied explicitly and recomputed.

## Exact accounting and custody vocabulary

Accounting keys are exactly:

`schema_version`, `J_files`, `Q_query_bytes`, `P_path_bytes`, `S_source_bytes`, `T_scanned_tuples`, `M_matches`, `R_ranges`, `U_utf16_units`, `B_output_bytes`, `W_work`, `failure_counters`.

Failure counters are exactly:

`INVALID_INPUT`, `CANCELLED`, `DEADLINE_EXCEEDED`, `ASSOCIATION_FAILED`, `ADMISSION_FAILED`, `RESOURCE_EXHAUSTED`, `OVERFLOW`, `INVARIANT_FAILED`.

Custody keys are exactly:

`schema_version`, `attempt_id`, `terminal_result_sha256`, `terminal_sequence0`, `terminal_count1`.

Replay keys are exactly:

`schema_version`, `canonical_attempt_sha256`, `admitted_binding_sha256`, `terminal_preimage_sha256`, `tooling_identity_sha256`, `freeze_binding_sha256`, `predecessor_lock_sha256`.

## Exact arithmetic and fixed point

All counters are checked uint64 with maximum `18446744073709551615`; decoding uses `json.Number`/`UseNumber`, never float64. Values above 2^53 remain exact integers.

`W_work` is exactly:

```text
50 + 3*J_files + 5*Q_query_bytes + 7*P_path_bytes + S_source_bytes + 11*T_scanned_tuples + 13*M_matches + 17*R_ranges + 19*U_utf16_units + 31*B_output_bytes
```

`B_output_bytes` is a fixed point over canonical terminal bytes with `custody.terminal_result_sha256` and the terminal preimage digest normalized to `sha256:` plus sixty-four zeroes. The validator iterates `B_output_bytes`/`W_work` monotonically for at most 64 rounds, requires final encoded length to equal `B_output_bytes`, and independently recomputes the terminal preimage digest. Custody digest equals replay terminal preimage digest.

Malformed raw input attempt id is `attempt-raw-sha256-` plus the full lowercase SHA256 of exact raw bytes.

## Terminal and failure rules

`COMPLETE` has `failure: null` and a non-null `range_union_candidate`. `FAILED` has `matches: []` and `range_union_candidate: null`. The validator enforces exactly one failure counter for failed terminals and zero failure counters for complete terminals.

Failure codes have closed stage/detail requirements:

- `INVALID_INPUT`: stage `raw`, detail `{reason}`
- `CANCELLED`: stage `control`, detail `{control}`
- `DEADLINE_EXCEEDED`: stage `control`, detail `{deadline}`
- `ASSOCIATION_FAILED`: stage `association`, detail `{source_id}`
- `ADMISSION_FAILED`: stage `admission`, detail `{source_id}`
- `RESOURCE_EXHAUSTED`: stage `scan`, detail `{limit}`
- `OVERFLOW`: stage `accounting`, detail `{counter}`
- `INVARIANT_FAILED`: stage `invariant`, detail `{invariant}`

## Candidate and match rules

The complete candidate is the full operation `RANGE_UNION` with `executedLocation=false`, `candidate_only=true`, exact qualified Location pins, admission digest, complete ordered match members, and recomputed candidate digest.

Matches require `start_byte < end_byte`, complete source tuples, deterministic LSP order by path byte, start, end, and ordinal, source-bound checks against supplied admitted source metadata/bytes, literal exact matching, and UTF-8/LSP UTF-16 position recomputation.

## Canonical JSON and schema execution

Canonical JSON is sorted keys, UTF-8, no insignificant whitespace, and one trailing LF. Duplicate, unknown, trailing, invalid UTF-8, noncanonical, and invalid uint64 raw forms are rejected.

The JSON Schema is supplied as bytes in the bundle and is executed by a stdlib schema walker tied to those bytes. It verifies Draft 2020-12 identity, required terminal members, and strict v4 sub-contract anchors. Tests include a schema mutation/drift rejection. This is not a claim of general-purpose JSON Schema library conformance.

## Nonrecursive payload/freeze boundary

The payload/freeze boundary is nonrecursive. The validator pins supplied payload/freeze-binding bytes and checks terminal fields against that pin; it does not recursively interpret production payload contents or oracle output.
