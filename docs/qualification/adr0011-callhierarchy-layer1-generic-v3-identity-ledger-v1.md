# ADR-0011 call-hierarchy layer-1 generic v3 identity ledger v1

Status: **REVIEW_REQUIRED**

## Authority and scope

This document describes the frozen ledger in `docs/qualification/adr0011-callhierarchy-layer1-generic-v3-identity-ledger-v1.json`. Base HEAD: `bf8741551899882efa06c3bee847337c794b7503`. It grants no corpus, RED, source, test, runtime, activation, acceptance, or production authority. The only binding authority is each record's direct literal `binding_identity`.

No record inherits or defaults `profile_identity`, `binding_identity`, or `workspace_nonce_hex`. There is no shared value expansion and no runtime derivation rule. The producer algorithm below is exact historical authoring provenance and an independent consistency check; consumers MUST use the frozen literals.

## Counts and exact literals

- Occurrence records: 85
- `opaque-incoming` literals: 45
- `opaque-outgoing` literals: 40
- Unique occurrence addresses: 85
- Unique typed occurrence identities `(occurrence_address, exact-JSON request_id)`: 85
- Distinct request-id values: 52; repeated values remain separate occurrences because their addresses differ.
- Profile literal in every record: `corpus-profile-go`
- Workspace nonce literal in every record: `0101010101010101010101010101010101010101010101010101010101010101`

## Anti-oracle provenance

The enumerator and source-file Git blob provenance remain recorded verbatim in the JSON ledger. Assignment and reproduction use only `occurrence_address` and the exact JSON-typed `request_id`. They do not read or use method, expected output/identity, world or pair semantic names, implementation behavior, sibling values, or any mismatch diagnostic.

## Exact typed occurrence identity bytes

For each record, construct this two-member JSON object:

```text
{"occurrence_address": <the exact JSON string>, "request_id": <the exact original JSON value with its parsed JSON type preserved>}
```

Canonical identity bytes are produced exactly by:

```python
(json.dumps(typed_identity, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
```

This means:

1. keys are sorted, yielding `occurrence_address` before `request_id`;
2. `request_id` is neither stringified nor numerically coerced;
3. there is no insignificant JSON whitespace;
4. strings use Python 3 `json.dumps(..., ensure_ascii=False)` escaping;
5. exactly one U+000A is appended after `}`;
6. the result is UTF-8 without BOM, so the final byte is exactly `0x0a`.

## Exact ranking and tie-break

For each canonical identity byte string, compute SHA-256 and encode it as 64 lowercase ASCII hexadecimal characters. Sort ascending by the tuple:

```text
(sha256_lower_hex, occurrence_address)
```

The secondary key is Python string ascending lexicographic order. Every address in this ledger is ASCII, so that order is byte-for-byte identical to ascending UTF-8 address bytes. Zero-based ranks 0 through 44 are `opaque-incoming`; ranks 45 through 84 are `opaque-outgoing`. This reproduces the original partition exactly; only the authorized literal prefixes changed.

## Executable identity-only reproduction

The following reads the ledger but computes assignments solely from each record's `occurrence_address` and exact JSON-typed `request_id`; `binding_identity` is read only in the final equality assertion.

```python
import hashlib
import json

ledger_path = "docs/qualification/adr0011-callhierarchy-layer1-generic-v3-identity-ledger-v1.json"
with open(ledger_path, "r", encoding="utf-8") as stream:
    ledger = json.load(stream)
records = ledger["records"]

def identity_bytes(record):
    typed_identity = {
        "occurrence_address": record["occurrence_address"],
        "request_id": record["request_id"],
    }
    return (
        json.dumps(
            typed_identity,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )
        + "\n"
    ).encode("utf-8")

ranked = sorted(
    records,
    key=lambda record: (
        hashlib.sha256(identity_bytes(record)).hexdigest(),
        record["occurrence_address"],
    ),
)
reproduced = {
    record["occurrence_address"]: (
        "opaque-incoming" if rank < 45 else "opaque-outgoing"
    )
    for rank, record in enumerate(ranked)
}
assert len(reproduced) == 85
assert sum(value == "opaque-incoming" for value in reproduced.values()) == 45
assert sum(value == "opaque-outgoing" for value in reproduced.values()) == 40
assert all(
    reproduced[record["occurrence_address"]] == record["binding_identity"]
    for record in records
)
print(json.dumps(reproduced, ensure_ascii=False, sort_keys=True, separators=(",", ":")))
```

## Records canonical SHA-256

Canonical hash scope is the JSON ledger's `records` array only:

```text
UTF-8(JSON(records, ensure_ascii=false, sort_keys=true, separators=(',',':')) + LF)
```

SHA-256 lower-hex: `d626cbda96d875a2f3d0c04fa1a00d20c72481c5b74f742e0af8b5c4943802c5`

Validation MUST confirm the stored digest, 85 unique addresses, 85 unique typed occurrence identities, exact request-id JSON types and values, direct literals, exact 45/40 counts, and a bijection between all stored records and all reproduced assignments.

## Fail-closed mismatch rule

Any future corpus, structural-address, request-id type/value, count, binding, profile, nonce, reproduction, or canonical-hash mismatch **MUST block** the consumer or qualification step and **MUST NEVER alter this ledger**. Correction requires separately authorized successor evidence; it must not silently rewrite this v1 ledger.
