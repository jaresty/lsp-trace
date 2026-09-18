# G1 lifecycle and cancellation draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** draft semantics for the isolated NDJSON worker; not a frozen wire contract.

## Process lifecycle

1. **STARTING** — caller starts the isolated worker with network access denied and fixed resource limits.
2. **READY** — worker has validated its configuration and is ready to accept requests.
3. **BUSY** — worker is evaluating admitted work.
4. **STOPPING** — caller has requested orderly shutdown; no new work is accepted.
5. **STOPPED** — stdin is closed, stdout is drained or deliberately discarded, and the process has exited.
6. **FAILED** — startup, protocol, policy, or runtime failure prevents continued operation.

These labels are descriptive draft terms and are not yet protocol-visible enum values.

## Request termination

A request may end by normal completion, explicit cancellation, deadline expiry, resource limit, malformed input, policy rejection, backend failure, worker failure, or caller disconnect. Each admitted member receives exactly one terminal outcome under the applicable ADR 0007 list once evaluation begins.

Cancellation is cooperative first:

1. caller sends a correlated cancellation control message;
2. worker stops admitting new work for that request;
3. worker checks cancellation at bounded safe points;
4. worker emits a final correlated closure if possible;
5. caller closes the request only after validating terminal accounting.

If the worker does not close within the frozen cancellation deadline, the caller terminates the worker process tree, marks the request with the applicable failure/timeout disposition, and records that forced termination occurred. Forced termination must not be represented as successful completion.

## Race rules to freeze

- cancellation versus normal completion;
- cancellation versus timeout;
- cancellation versus worker crash;
- duplicate cancellation;
- cancellation after final response emission;
- EOF while a request is active;
- response after caller-side forced termination;
- restart and retry identity behavior.

## Shutdown

Orderly shutdown rejects new requests, allows bounded in-flight closure, drains or records unread output, closes stdin, waits for the worker and descendants, and records the final process result. A timeout escalates to process-tree termination. Restart creates a new runtime identity and cannot reuse an exact result unless all cache-identity inputs, including runtime identity policy, permit it.

## Safety boundary

The worker cannot convert transport success, process survival, a complete response frame, or a balanced denominator into semantic authority, acceptance, correctness, ownership, production use, or feature identity.
