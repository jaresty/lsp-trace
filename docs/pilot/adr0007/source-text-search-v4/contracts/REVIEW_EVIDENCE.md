# ADR0007 v4 contract correction review evidence

Correction target: `ce9cd5dc4b4f14e6325003f110275899fb2a3d5d`.

Scope guard:

- No production package edits.
- No oracle package edits.
- No corpus attempt edits.
- No generated oracle output edits.
- No evaluator/oracle command edits.
- No freeze or GO action.

Implemented checks:

- Exact `W_work` weighted formula with checked uint64 arithmetic.
- `B_output_bytes` fixed-point recomputation with normalized terminal-result/preimage digest fields and max 64 iterations.
- Exact accounting/custody/replay key vocabulary.
- Exact malformed raw attempt id from raw bytes SHA256.
- `json.Number`/`UseNumber` integer decoding; no float64 uint64 path.
- Closed failure counters, failure stages, and code-specific detail shapes.
- Complete candidate `RANGE_UNION`, candidate-only, not executed Location, ordered members, qualified pins, and digest recomputation.
- Source-bound match validation, literal exactness, deterministic order, and UTF-16 LSP position recomputation.
- Explicit validator input bundle with raw attempt, terminal, admitted source, tooling manifest, predecessor manifest, payload/freeze binding, and schema bytes.
- Stdlib schema walker tied to supplied schema bytes plus schema mutation/drift test.

Fixture counts:

- Positive fixtures: 7.
- Negative fixtures: 22.

Positive fixtures cover complete, invalid input/parse, admission failure, resource exhaustion, cancellation, deadline, and overflow terminals.

Negative fixtures cover W formula, B fixed point/encoded length, failure counters, custody constants, raw attempt digest, malformed identity, failure stage/detail correlation, candidate operation, executed Location flag, candidate members, candidate digest, empty match, literal mismatch, UTF-16 position mismatch, deterministic order/source association, source-byte metadata, tooling pin, schema drift, above-2^53 exact-number drift, max-uint64 boundary, uint64 decode overflow, and weighted arithmetic overflow.
