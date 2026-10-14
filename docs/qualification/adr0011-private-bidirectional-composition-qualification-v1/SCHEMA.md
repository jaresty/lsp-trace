# Private bidirectional composition schema sketch

This is source-visible package schema documentation, not a published public schema.

## Top-level object

Ordered keys only:

1. `incoming`: `DirectionalEnvelope`
2. `outgoing`: `DirectionalEnvelope`

Unknown top-level keys are invalid. Missing or swapped keys are invalid composition inputs.

## DirectionalEnvelope

Closed keys in order:

1. `schema`: `lsp-trace.adr0011.private-bidirectional-composition.direction-envelope.v2`
2. `direction`: `incoming` or `outgoing`, matching the top-level slot
3. `status`: `SUCCESS` or `FAILURE`
4. `authority`: `0`
5. `accepted`: `false`
6. `production_admission`: `REFUSED_NOT_REQUESTED_NOT_GRANTED`
7. `production_refusal`: `true`
8. `target_set_relative_completeness`: object with `status: UNKNOWN`
9. `frontier`: `UNKNOWN`
10. `raw_provider_bytes_policy`: `NO_READ_NO_COPY_NO_REMARSHAL_REFERENCE_BY_PATH_HASH_ONLY`
11. `ceiling`: `{per_direction_raw_bytes:33554432, aggregate_live_bytes:33554432, combined_ceiling_admitted:false, ceiling_change:false}`
12. `opaque_custody`: `true`
13. `calls_only`: `true`
14. `immutable_reference_only`: `true`
15. `peer_failure_effect`: `PEER_UNCHANGED_NOT_ERASED_NOT_RELABELED`
16. `payload`: one of the payloads below

## Success payload

`payload_type` is `accepted_directional_success_reference`. The payload pins one accepted directional profile/evidence result and its own generic semantic dependency. It preserves only same-direction counts, failure reference, final-summary reference, ledger reference, numeric-bounds reference, raw-provenance reference, custody references, and forbidden-synthesis refusals.

## Failure payload

`payload_type` is `fixture_native_directional_failure`. This is a private evidence-only failure type for composition evaluation only. It is valid only when all of the following hold:

- `failure_disposition` is `FAILURE`;
- `direction` matches the envelope and top-level slot;
- `failure_code` begins with `INCOMING_` for incoming or `OUTGOING_` for outgoing;
- `failure_stage` is present;
- `message_digest` is a string and exact full match for lowercase `sha256:[0-9a-f]{64}`; uppercase hex, short/long hex, nonhex, missing prefix, extra suffix, whitespace, and nonstring values are invalid;
- `evidence_ref` is present and private fixture-native;
- `private_evidence_only` is `true`;
- `counts_claimed`, `success_claims`, `inverse_inference`, and `peer_evidence_used` are all `false`.

## Invalid input class

Invalid composition input is rejected before representation. It includes schema drift, pin/hash/profile drift, direction swap/relabeling, missing direction, unknown key, semantic alias, malformed failure `message_digest`, cross-direction evidence reference, peer erasure, peer evidence copying, failure-to-success promotion, merged failure, missing evidence, authority/completeness escalation, raw-copy/remarshal, ceiling changes, CALLS-only drift, public/default-on exposure, inverse synthesis, deduplication, union graph claim, ledger merge, and ADR0011->ADR0007/Stage8 claims.
