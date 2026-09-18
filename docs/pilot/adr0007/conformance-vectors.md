# G1 conformance-vector draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** required cases for a future frozen protocol test suite.

Each vector must bind canonical input bytes, expected disposition, applicable terminal list, resource limits, and an immutable vector digest. These cases are a draft inventory, not executed qualification evidence.

| ID | Scenario | Expected disposition |
|---|---|---|
| G1-FRAME-001 | One valid request object per NDJSON line | Accept and correlate normally |
| G1-FRAME-002 | Blank line | Reject or ignore according to frozen policy; never reinterpret as a request |
| G1-FRAME-003 | Malformed JSON | Fail closed; emit a typed protocol failure or terminate per policy |
| G1-FRAME-004 | Multiple JSON values on one line | Reject the frame |
| G1-FRAME-005 | Embedded unescaped newline | Reject the frame |
| G1-FRAME-006 | Unknown required envelope field | Reject under frozen unknown-field policy |
| G1-FRAME-007 | Duplicate JSON field | Reject under frozen duplicate-field policy |
| G1-FRAME-008 | Oversized line | Reject before semantic processing |
| G1-CORR-001 | Unknown correlation ID in response | Reject response; do not attach it to another request |
| G1-CORR-002 | Duplicate response for one request | Reject duplicate; preserve first terminal closure |
| G1-CORR-003 | Out-of-order responses | Accept only if frozen ordering policy permits; otherwise reject |
| G1-ADMIT-001 | Exact assembled admission with valid digest and length | Admit without source access |
| G1-ADMIT-002 | Digest mismatch | Reject admission |
| G1-ADMIT-003 | Missing custody, revision, or policy identity | Reject admission |
| G1-ADMIT-004 | Caller supplies an unadmitted relationship | Reject the record; worker must not add it |
| G1-ADMIT-005 | Worker attempts source/path/network access | Conformance failure; pilot remains disabled |
| G1-TERM-001 | Every admitted member completes | Operation may close `COMPLETE` if equation balances |
| G1-TERM-002 | One member is invalid | Record exactly one invalid-member outcome; denominator remains balanced |
| G1-TERM-003 | Duplicate input | Record duplicate outcome and canonical-member reference |
| G1-TERM-004 | Omitted member outcome | Reject operation closure as unbalanced |
| G1-TERM-005 | Extra member outcome | Reject operation closure as unbalanced |
| G1-TERM-006 | Operation fails before evaluation | Preserve full denominator and zero evaluated members |
| G1-CANCEL-001 | Cooperative cancellation before evaluation | Close request without claiming completion |
| G1-CANCEL-002 | Cancellation during evaluation | Close each begun member exactly once |
| G1-CANCEL-003 | Cancellation after final response | Treat as duplicate/no-op according to frozen policy |
| G1-CANCEL-004 | Deadline expiry | Apply timeout disposition; never report success |
| G1-CANCEL-005 | Forced process-tree termination | Record forced termination and incomplete closure |
| G1-CACHE-001 | Exact identity replay | Cache hit permitted |
| G1-CACHE-002 | Changed source digest | Cache miss and new record |
| G1-CACHE-003 | Changed model, prompt, policy, or runtime digest | Cache miss and new record |
| G1-CACHE-004 | Approximate or family-level identity match | Cache miss |
| G1-LIFE-001 | EOF with no active request | Orderly stop |
| G1-LIFE-002 | EOF with active request | Close active work according to frozen failure policy |
| G1-LIFE-003 | Startup policy/configuration failure | Enter failed state; accept no work |
| G1-LIFE-004 | Backend unavailable | Return model/backend failure without fallback |
| G1-PRIV-001 | Deletion by admission ID | Invalidate dependent products and emit deletion receipt |
| G1-PRIV-002 | Revoked dependency | Invalidate dependent products and prevent stale reuse |
| G1-BACKEND-001 | Two backend executions | Distinct runtime/backend identities and records |
| G1-BACKEND-002 | Backend-specific type crosses boundary | Reject protocol violation |

## Freeze requirements

Before G1 approval, each vector needs exact serialized input, expected output/error shape, terminal accounting expectations, resource limits, and pass/fail assertions. The final vector set must be digest-bound by the G1 manifest and run without test-data tuning after test unlock.
