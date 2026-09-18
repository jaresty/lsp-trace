# G5 runtime containment verification

- **Status:** `PARTIAL_VERIFICATION`
- **Pilot:** `PILOT_DISABLED`
- **Worker:** `experiment/pilot-worker/worker`

## Completed checks

- Worker source imports only standard packages plus Yzma; no explicit network, subprocess, or repository traversal package is imported.
- Worker accepts caller-supplied model, library, and prompt paths; it does not select paths or fetch them.
- External sandbox control `(version 1)(deny network*)` prevented `/usr/bin/curl` from starting, returning `Operation not permitted` with exit code `71`.
- Native dependency inspection is recorded in `runtime-verification.md`.

## Not yet executed

- worker execution under the network-denied profile;
- read/traversal restriction with only admitted files exposed;
- process-tree CPU, memory, file, and descendant limits;
- cancellation and forced-termination cleanup;
- malformed NDJSON handling—the recovered worker is prompt-file based and is not yet an NDJSON protocol worker;
- end-to-end model load and four-packet Describe run.

## Decision

The external network-denial control passes, but G5 is not complete. The current worker is an experiment executable, not yet the frozen NDJSON pilot worker. No inference or pilot enablement is authorized by this record.
