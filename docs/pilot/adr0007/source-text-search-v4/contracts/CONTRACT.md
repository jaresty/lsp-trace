# ADR0007 source text search private v4 strict contract

This is a prospective, implementation-independent v4 wire contract. It does not edit production, oracle, corpus attempts, generated oracle output, evaluator/oracle commands, freeze state, or GO state. v1-v3 are predecessor-locked inputs only.

## Validator input bundle

The standalone validator consumes explicit named bundle roles: `raw_attempt_bytes`, `terminal_bytes`, `admitted_source_bytes`, `admitted_binding_bytes`, `tooling_manifest_bytes`, `predecessor_manifest_bytes`, `payload_freeze_binding_bytes`, and `schema_bytes`. Each role is a repository-artifact byte string, not a digest-text substitute and not arbitrary alternate bytes. Raw attempt identity is the SHA256 of `raw_attempt_bytes`; source content identity is the SHA256 of each admitted source byte string; admission, tooling, predecessor, and freeze pins are SHA256 over the exact role bytes supplied in the bundle. Swapped roles, digest-text hashing in place of role bytes, and source bytes inconsistent with the terminal metadata are rejected.

`Derive(input-without-terminal)` is the independent contract authority. Its input type intentionally excludes `terminal_bytes`; candidate terminals are never used as a seed, template, failure-shape source, match-id source, or accounting source. It parses only the raw attempt and named artifact/source bytes, admits the named source bytes, recomputes request/source/control/search semantics, source metadata, match ranges, UTF-16 positions, candidate pins, accounting, custody/replay/payload digests, and fixed-point terminal hashes, and returns exactly one canonical terminal byte sequence. `Validate(bundle)` first enforces schema and local invariants on the candidate terminal, then exact-compares `terminal_bytes` to `Derive(input-without-terminal)`; no alternative canonical output is valid.

## Schema execution

The supplied schema is Draft 2020-12 shaped and recursively defines every object, array, and variant. The validator executes every keyword used by this schema: `type`, `const`, `enum`, `required`, `additionalProperties`, `properties`, `items`, `minItems`, `maxItems`, `minimum`, `maximum`, `minLength`, `pattern`, `oneOf`, and `if`/`then`. There are no bare object placeholders.

## Stage/admission map

Early pre-admission failures require `admission.completed=false`: `INVALID_INPUT/raw`, `CANCELLED/control`, `DEADLINE_EXCEEDED/control`, `ASSOCIATION_FAILED/association`.

Late failures require `admission.completed=true`: `ADMISSION_FAILED/admission`, `RESOURCE_EXHAUSTED/scan`, `OVERFLOW/accounting`, `INVARIANT_FAILED/invariant`.

## Accounting and fixed point

Accounting keys are exactly `schema_version`, `J_files`, `Q_query_bytes`, `P_path_bytes`, `S_source_bytes`, `T_scanned_tuples`, `M_matches`, `R_ranges`, `U_utf16_units`, `B_output_bytes`, `W_work`, and `failure_counters`. Failure counters are exactly `INVALID_INPUT`, `CANCELLED`, `DEADLINE_EXCEEDED`, `ASSOCIATION_FAILED`, `ADMISSION_FAILED`, `RESOURCE_EXHAUSTED`, `OVERFLOW`, and `INVARIANT_FAILED`.

`W_work = 50 + 3*J_files + 5*Q_query_bytes + 7*P_path_bytes + S_source_bytes + 11*T_scanned_tuples + 13*M_matches + 17*R_ranges + 19*U_utf16_units + 31*B_output_bytes` with checked uint64 arithmetic and maximum `18446744073709551615`.

`B_output_bytes` is the canonical terminal length fixed point after normalizing both `custody.terminal_result_sha256` and `replay.terminal_preimage_sha256` to the zero SHA256 digest. The validator iterates at most 64 times, requires computed B to equal declared B, then checks W.

## Candidate, source, and payload rules

A complete terminal has operation `RANGE_UNION`, `executedLocation=false`, `candidate_only=true`, ordered match members, exact qualified Location pins, recomputed admission digest, and recomputed candidate digest. Pins compare source id, byte start/end, line start/end, and UTF16 start/end.

Matches require `start_byte < end_byte`, deterministic path-byte/start/end/ordinal order, supplied source-byte metadata agreement, exact literal bytes, and recomputed UTF16 LSP positions. Positions are unique and exactly one-to-one with matches.

Payload digest is SHA256 of explicit supplied payload/freeze-binding bytes. Admission digest and membership/order are recomputed from supplied admitted binding/source records using the contract’s source-admission-v2-equivalent canonical bytes plus LF semantics; before admission the digest is the zero SHA256 digest.
