# NDJSON deadline conformance

- **Status:** `PASS`
- **Pilot:** `PILOT_DISABLED`
- **Behavior:** caller supplies `deadline_ms`; accepted range is 1–90000 ms.
- **Test:** `deadline_ms=1`
- **Result:** `TIMEOUT` with `context deadline exceeded`.

The adapter now bounds each worker invocation with a caller-visible deadline. Cooperative cancellation messages and process-tree escalation remain separate follow-up work.
