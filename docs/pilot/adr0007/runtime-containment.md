# G5 runtime containment draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** containment and capability requirements for a future isolated worker.

## Process boundary

The semantic backend runs only in a separate subprocess behind the frozen protocol boundary. It is never linked into the `lsp-trace` core process, CLI, MCP server, reusable feature-inventory workflow, or core module dependency graph.

The caller supplies assembled bytes, typed metadata, and explicitly admitted relationships. The worker receives no repository path, live workspace, source-store handle, session handle, selector resolver, or tool capability.

## Forbidden worker capabilities

The worker must not:

- access the network or DNS;
- download, update, or discover runtime/model artifacts;
- read source files or arbitrary filesystem paths;
- traverse repositories or follow symlinks;
- open live sessions or source-object stores;
- resolve, repair, broaden, or substitute selectors/ranges;
- invoke tools, commands, callbacks, or model-selected actions;
- add, suppress, or reinterpret server-reported relationships;
- silently fall back to another backend;
- emit authority, acceptance, ownership, correctness, production-use, completion, or feature-identity claims.

## Resource controls

A future runtime policy must bound and record:

- worker and descendant CPU time;
- resident and virtual memory;
- process and thread count;
- input bytes, output bytes, line size, and message count;
- context length and generation tokens;
- wall-clock deadline;
- temporary storage and file descriptors;
- index and cache growth;
- cancellation and forced-termination deadlines.

Limits apply to descendants, not only the direct worker process. Exceeding a limit produces a typed non-success disposition and preserves denominator accounting.

## Network denial

Network denial must be enforced outside the worker and verified by a conformance test. Application-level “do not use network” configuration is insufficient. Automatic download must also be disabled. A failed network-denial test blocks G5.

## Cancellation and termination

Cancellation is cooperative first. If the worker or descendants fail to close within the frozen deadline, the caller terminates the entire process tree, cleans temporary resources, records forced termination, and never reports successful completion.

A crashed, hung, escaped, or policy-violating worker cannot be restarted in place while retaining the same runtime identity. Restart creates a new runtime record and requires exact cache-identity evaluation.

## G5 rejection conditions

Keep the pilot disabled for in-process backend loading, network reachability, download capability, source traversal, tool execution, fallback behavior, unbounded descendants, unverified cleanup, incomplete resource accounting, or any containment proof that depends only on worker cooperation.
