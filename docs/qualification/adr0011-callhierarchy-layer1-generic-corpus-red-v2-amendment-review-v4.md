# ADR-0011 call-hierarchy Layer-1 generic corpus RED v2 amendment review v4

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`

Independent rereview was performed against exact repository HEAD `413afb35f282ce468e3eae457df24ea295a983f5` and the byte-encoding-only working-tree correction to `docs/qualification/adr0011-callhierarchy-layer1-generic-corpus-red-v2-amendment.md`. Prior committed reviews remain unchanged. This review is limited to that correction and creates no corpus, RED, implementation, behavior, activation, generated-output, hash, or commit authority beyond the amendment's existing staged gate.

## Exact four-path verification

The correction defines exactly these four required paths and no alternative byte representation:

1. `driver.derived.external.notifications_bytes`
2. `driver.derived.external.responses_bytes`
3. `driver.derived.external.result_bytes`
4. `driver.derived.external.server_error_bytes`

All four fields are mandatory. Availability is represented by the field value, never by omission. Driver validation rejects omitted fields, unknown or extra representations, uppercase hexadecimal, odd-length strings, nonhexadecimal characters, prefixes including `0x`, base64 strings, and numeric byte arrays.

## Canonical null, empty, and hexadecimal rules

The corrected rules are exact and non-overlapping:

- JSON null means the governing v1 source is unavailable.
- `""` means the source is available and its byte sequence is empty.
- A non-empty JSON string means available nonempty bytes and must contain lowercase, prefix-free, even-length hexadecimal.

The availability cross-rules are complete:

- `result_bytes` is null if and only if v1 `result.available` is false;
- `server_error_bytes` is null if and only if v1 `error.available` is false, irrespective of `error.present`;
- `notifications_bytes` and `responses_bytes` are each null if and only if v1 `external.available` is false.

Under true availability each field must be a string; under false availability it must be null. The correction explicitly rejects disagreement between either external field and the shared `external.available` state.

## Exact v1 byte equality

For available values, hexadecimal decoding must equal byte-for-byte the corresponding committed v1 source:

- `notifications_bytes` -> `external.notifications_hex`;
- `responses_bytes` -> `external.responses_hex`;
- `result_bytes` -> `result.bytes_hex`;
- `server_error_bytes` -> `error.serialized_hex`.

The amendment forbids JSON reserialization and semantic reconstruction. Independent enumeration of all 85 complete committed v1 record occurrences confirmed that every available source value is canonical lowercase even-length hexadecimal and round-trips to exactly the same bytes.

The 22 occurrences with `error.available == false` carry an empty `error.serialized_hex` storage literal in v1. The correction resolves that representational ambiguity by governing v2 nullability solely by `error.available`: those v2 `server_error_bytes` values are null, not available-empty strings. This does not weaken byte equality because unavailable bytes are not decoded; when availability is true, decoded equality is mandatory.

## Canonical serialization and semantic boundary

Canonical envelope serialization recursively orders object keys lexically, preserves array order, uses compact separators and no insignificant whitespace, and ends each file with exactly one LF. The exact nested order is therefore `driver`: `derived`, `request`, `transport`; `driver.derived`: `external`; and `driver.derived.external`: `notifications_bytes`, `responses_bytes`, `result_bytes`, `server_error_bytes`.

The correction introduces no semantic field or ambiguity. It adds only opaque byte encodings derived from committed v1 bytes. The existing prohibitions remain intact: no `result.form`, token/form label, body or payload subspan, offset/length/digest, request or response semantic payload field, direction, symbol/item/range/call-site/caller/callee/provider meaning, target meaning, semantic world name, or semantic dispatch key.

## Reconfirmed gate

The byte-encoding correction fully specifies the four paths, availability/nullability, available-empty representation, canonical hexadecimal syntax, exact v1 decoded-byte equality, recursive canonical key ordering, and strict rejection behavior. It does not disturb the prior v3 GO findings concerning provenance, `162 -> 145` expectation accounting, self-contained envelopes, generic causal dispatch, or the finite owner/view/lease boundary.

`ADR0011_CALLHIERARCHY_LAYER1_GENERIC_CORPUS_RED_AMENDMENT_GO`
