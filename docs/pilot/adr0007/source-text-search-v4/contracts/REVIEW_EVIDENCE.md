# ADR0007 v4 contract second correction review evidence

Correction target: `2c63ddcad5a202205025e7c050e7b8cb346ca9d1`; reviewer blocker: `c3a8d823`.

Scope guard: no production, oracle, corpus, generated oracle output, evaluator/oracle command, freeze, or GO edits.

Implemented checks:

- Recursive Draft-like schema execution for every keyword used by the supplied schema: `type`, `const`, `enum`, `required`, `additionalProperties`, `properties`, `items`, `minItems`, `maxItems`, `minimum`, `maximum`, `minLength`, `pattern`, `oneOf`, and `if`/`then`.
- Terminal bytes are decoded with `json.Number`/`UseNumber` and validated against supplied schema bytes before typed contract validation.
- Schema recursively defines every terminal object/array/variant. Schema object count: 30. Bare object count: 0.
- Exact `W_work` weighted formula with checked uint64 arithmetic.
- `B_output_bytes` fixed-point recomputation normalizes both `custody.terminal_result_sha256` and `replay.terminal_preimage_sha256` to zero digest, converges within 64 iterations, and requires computed B to equal declared B before W is checked.
- Explicit stage/admission map: early pre-admission stages `raw`, `control`, and `association` (`INVALID_INPUT`, `CANCELLED`, `DEADLINE_EXCEEDED`, `ASSOCIATION_FAILED`) require `admission.completed=false`; late post-admission stages `admission`, `scan`, `accounting`, and `invariant` (`ADMISSION_FAILED`, `RESOURCE_EXHAUSTED`, `OVERFLOW`, `INVARIANT_FAILED`) require `admission.completed=true`.
- Failure detail objects are closed by schema and checked against code/stage relation.
- Candidate validation compares operation, `executedLocation=false`, `candidate_only=true`, admission digest, complete ordered match members, recomputed candidate digest, and every qualified Location pin source/byte/line/UTF16 start/end coordinate.
- Positions must be exactly one-to-one with matches, unique by match identity.
- Payload digest, admission digest, admission membership/order, tooling manifest, predecessor manifest, and freeze-binding pins are recomputed from explicit validator bundle bytes.

Fixture counts:

- Positive fixtures: 8, including an above-2^53 exact-number boundary vector.
- Negative fixtures: 46.

Mutation coverage includes nested schema type drift, early/late stage-admission violations, code/detail type drift, B mismatch, replay normalization mismatch, all candidate pin coordinate classes, duplicate/missing position, payload digest, admission digest, admission membership/order, schema drift, uint64 overflow, max-uint64 boundary, weighted overflow, and source/literal/UTF16 association failures. Each negative asserts exact rejection code/path.
