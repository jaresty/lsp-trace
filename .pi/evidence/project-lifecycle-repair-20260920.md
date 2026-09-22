# Managed project lifecycle repair evidence

Private local evidence. No source bodies.

## Before

- Managed server instance: `si1:a250ac8fb235b282ad40dceeb51a583b`
- Executable custody: `SELF_MEASURED`
- Executable SHA-256: `sha256:a9486f42855af43f433f38c89343f68915632f2cf1b7b93ad9d5fc496f35ae93`
- `project`: session `sk1:949e027243da188035cc5c6532c4cb10df4210f2bdde80acc58b7b7b9af5f283`, generation 1, READY, started `2026-09-20T21:36:37.012295-07:00`
- `d01-csharp`: session `sk1:0fd6bbef7da1dbd871650446858abcd9a10a47a7c1709c8d937b523a456fee76`, generation 1, READY, started `2026-09-20T21:36:37.045214-07:00`
- Managed census: sessions 2, generations 2, requests 0, children 2, workers 0.
- Current MCP host PID 70872 owns project gopls PID 70873 (PGID 70873), gopls telemetry PID 70876, and NAIS csharp-ls PID 70877 (PGID 70877).
- Older separate repo-configured MCP host PID 70584 owns csharp-ls PID 70589 and no gopls.
- No stale/orphan project gopls exists in the current process tree; host operator action is not justified.

## Actions and after-state

- Supported restart request `offline-4` entered STOPPING, then list request `offline-7` observed project generation 1 POISONED with teardown failure `SESSION_POISONED`; d01-csharp remained generation 1 READY and workers remained 0.
- Post-teardown process census found no gopls under managed host PID 70872; NAIS csharp-ls PID 70877 remained unchanged. No host termination was performed.
- Supported exact-session retry request `offline-8` failed with `REAP_INCOMPLETE`: `host operator must inspect and reap the trusted local child before retrying`.
- Full direct-child census of trusted host PID 70872 found only NAIS csharp-ls PID 70877. Recorded project gopls PID 70873 (parent 70872, PGID 70873) no longer exists, so there is no exact stale child available for safe termination. Only parent PID 70872 can reap a departed child; signaling or reconnecting that shared host was not proven safe for NAIS and was not attempted.
- Final safe boundary: project generation 1 remains POISONED; d01-csharp generation 1 remains READY on instance `si1:a250ac8fb235b282ad40dceeb51a583b`, csharp-ls PID 70877 unchanged, workers 0 in the last managed census.
- Census continuation was not executed because the required READY lifecycle checkpoint failed. No selector, replay, model, or worker was created.
