# ADR0007 descriptor-publication blocker

## Bounded pre-install evidence

- Managed project session was READY on installed executable digest `sha256:fafd606a5036971ef087a68702c7171ac92fa1afe1c8c484313cf92b31743e2f` with workers=0.
- Exact continuation root metadata: owner-only root and `continuations`, `objects`, `objects/sha256`, and `catalogs` parents are mode 0700; configured object ceiling is 67108864 bytes.
- Census and continuation roots have distinct inode identities by configuration.
- Continuation root retained six objects and no descriptor. Retained checkpoint stages were CENSUS_COMMITTED/RUNNING, PROGRAM_C_COMPUTED/RUNNING, and PROGRAM_C_COMPUTED/FAILED_CAPTURE; no REQUESTS_RENDERED/RUNNING checkpoint was retained before this run.
- Existing tracing variable: `LSP_TRACE_RUNTIME_TRACE_PATH`. It records a bounded Go runtime flight trace and is currently dumped only by structural-context source-projection failure handling. It does not cover census continuation descriptor publication and was not enabled; it did not materially help.
- Added private fixed-enum descriptor publication classification while preserving the generic public error string and existing public diagnostic projection.
- Focused pre-install results: `Go test: 12 passed in 1 packages`; descriptor suite: `Go test: 5 passed in 1 packages`.

## Pending

- Install/reconnect changed binary.
- Exact no-model census stop at DESCRIBE_REQUESTS.
- Capture bounded private descriptor cause if failure persists.
- Fix only confirmed defect, then replay exact selector.
