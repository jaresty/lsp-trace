# NDJSON adapter conformance run

- **Status:** `PARTIAL_CONFORMANCE_PASS`
- **Pilot:** `PILOT_DISABLED`
- **Output:** `conformance-output.jsonl`
- **Output SHA-256:** `44c0635b4f4f1e3e54c690e2a766315a39fcb54499545151ac5feeebc90795cc`

## Vectors

| Vector | Result |
|---|---|
| Valid TARGET request | `COMPLETE` response |
| Duplicate message ID | `DUPLICATE_INPUT` |
| CENSUS acquisition | `POLICY_MISMATCH` |
| Incorrect input digest | `POLICY_MISMATCH` |
| Malformed JSON | `INVALID_INPUT` |
| Blank line | ignored |

The valid vector exercised the pinned worker, model, and native runtime through the adapter. Negative vectors failed closed with typed statuses. This is a partial run; the full frozen vector set still needs lifecycle, cancellation, size-limit, process-limit, dependency-invalidation, and deletion cases.
