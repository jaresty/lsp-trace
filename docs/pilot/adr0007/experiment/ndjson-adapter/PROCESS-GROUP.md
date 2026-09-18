# Process-group termination conformance

- **Status:** `PASS`
- **Pilot:** `PILOT_DISABLED`

The adapter now starts each worker in a separate process group using `Setpgid`. On deadline or cancellation, it sends `SIGKILL` to the negative process-group ID and waits for the direct child before emitting the terminal response. This covers descendants created by the worker process group rather than only the direct child.

Smoke tests passed after the change:

- normal request: `COMPLETE` with embedded worker payload;
- cancellation control: `CANCELLED` acknowledgement;
- target request: `TIMEOUT` with `context canceled` after group termination.

An explicit descendant-spawn fixture was run. The fixture wrote its child PID, the request was cancelled, and a post-run `kill -0` check reported `child_alive=0`. Parent and child cleanup therefore passed the local process-group fixture.
