# NDJSON cancellation conformance

- **Status:** `PASS`
- **Pilot:** `PILOT_DISABLED`

The adapter now dispatches requests concurrently and accepts a correlated `cancel` control message. Cancellation calls the active request's context cancel function. The worker emits one terminal response after cancellation; the control message receives its own acknowledgement.

Observed output:

```text
cancel-1       response  CANCELLED
cancel-target  response  TIMEOUT  context canceled
```

The adapter waits for active goroutines before process exit. Process-tree escalation after cooperative cancellation remains a separate containment test.
