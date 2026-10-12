# ADR0011 call-hierarchy Layer-1 RED oracle corpus v1 — independent byte review v3

## Scope and boundary

Reviewed the corrected corpus at `/Users/schwa/dev/lsp-trace-adr0011-c17` HEAD `742a2161202868239cf587de9977827d1db51150`. The pre-existing modified RED and corrected corpus files were left unchanged. This review used corpus and qualified-design bytes only. It did not execute the RED, tests, generators, or runtime code and does not authorize implementation or runtime behavior.

The review treated the corpus directory as the complete consumer boundary: `sessionruntime/testdata/callhierarchy-layer1-red-v1/manifest.json` plus exactly 25 files under `worlds/`. No design registry, documentation lookup, generator, runtime output, or ambient identifier resolver was used to obtain worker or execution records.

## Independent byte and manifest verification

A fresh read-only validator parsed all 26 corpus files and reported 301 checks passed, zero failed.

- Every file is canonical compact sorted-key UTF-8 JSON with exactly one terminal LF and no CR.
- The manifest contains exactly 25 unique names and paths in ascending lexical order. Its aggregate order is identical to that path order.
- Every path is normalized, relative, non-symlinked, confined beneath the corpus directory, and maps exactly to its declared world name. There are no missing or extra files under `worlds/`.
- Every declared byte length and SHA-256 matches the corresponding file bytes.
- The ordered concatenation is exactly 431,974 bytes with SHA-256 `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`.
- Worlds 1–23 are byte-for-byte identical to their HEAD versions and retain their qualified design/manifest pins.
- Every root has the exact literal provenance `{"schema":"A_SCHEMA","source_head":"A_HEAD"}`, matching root identity, record name, and path/name ordinal.

## Recursive corpus closure

The prior v2 blocker is corrected.

- World 24 contains exactly 16 inline complete worker objects, in order `worker-201` through `worker-216`. Each has the complete worker key set. No worker ID must be resolved outside the file.
- World 24 contains one exact root aggregate `round_trip_ledger`. For each of its seven dimensions, `before=0`, `delta` equals the sum of the 16 inline worker deltas, `after=delta`, `available=true`, and `known_zero` is true exactly when the sum is zero. Its terminal transition is the exact unavailable-before to `successful-empty` after transition.
- World 25 contains exactly 22 inline complete execution objects, in order `execution-301` through `execution-322`. Each has the complete execution key set. No execution ID must be resolved outside the file.
- World 25 contains exactly 11 ordered pairs. Every `enabled` and `disabled` endpoint is a repeated complete literal object equal to the corresponding inline execution, not an identifier. Every pair has the required enabled/disabled control contrast and exact external and transport equality.
- Across all 25 roots there are zero `$ref` keys, zero emitted `WORKER_*`, `EXEC_*`, or `CORE_*` identifier values, and zero unresolved IDs or references.

A corpus-only consumer can therefore obtain every expected worker and execution, validate World 24 aggregate accounting, and validate every World 25 pair without consulting documentation or any ambient registry.

## Semantic and opacity checks

- **S1:** World 3 preserves exact successful null; Worlds 4 and 5 preserve distinct nonempty incoming/outgoing result and response bytes.
- **S2:** Worlds 6–11 preserve six pairwise-distinct accepted inputs and six coherent, distinct failure tuples with unavailable results.
- **S3:** Worlds 12–14 preserve unavailable, available-empty, and available-nonempty payload distinctions. World 15 changes source pre/post while retained pre/post remain equal and `no_alias=true`.
- **S4:** All 25 roots are complete literal records with distinct canonical record bytes; recursive closure contains no reference or lookup dependency and no canned shared output.
- **S5:** Worlds 17–23 express refusal only through `refusal`; each retains the exact non-present `none` error observation with null code/message/data and serialized `null`.
- **Opacity:** The corpus exposes the qualified observations and opaque identities only; no goroutine, channel, mutex, pointer, memory-address, runtime-stack, or implementation-type detail appears.

## Disposition

The corrected corpus satisfies the qualified recursive design, exact byte and manifest pins, recursive closure, World 24 and World 25 aggregate requirements, S1–S5, and the Layer-1 opacity boundary.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_BYTES_GO`
