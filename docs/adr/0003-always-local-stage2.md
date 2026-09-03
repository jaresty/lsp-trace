# ADR 0003: Activate always-local Stage 2 lifecycle tools

Status: Accepted

## Context

The previous staged contract advertised six offline MCP tools and retained four lifecycle names as unadvertised `NOT_IMPLEMENTED` candidates behind production-containment language. The product is now explicitly local-development-only and needs useful lifecycle control of trusted developer-configured language servers.

## Decision

The default stdio MCP surface advertises ten canonical tools: the existing six offline evidence tools and `lsp_session_v1_list`, `lsp_session_v1_status`, `lsp_session_v1_stop`, and `lsp_session_v1_restart`. Canonical names and their existing unversioned aliases route lifecycle calls to one process-local session runtime.

On Darwin, process startup uses the existing local process-group supervisor and readiness/lifecycle APIs. A session becomes lifecycle-visible only according to the runtime readiness state. Stop and restart retain bounded asynchronous operation records, cancellation behavior, teardown/reap requirements, immutable terminal outcomes, and monotonically succeeding restart generations.

Unsupported platforms keep the same ten-tool discovery contract. Operations that require starting a process fail explicitly without starting a child. Platform support does not hide tools and is not expressed as a production-containment gate.

## Trust and safety boundary

**WARNING:** Child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted. Only configure commands you trust.

This decision does not claim hostile-code safety, native containment, remote execution, or privileged isolation. The server is a local developer tool, not a multi-tenant or production execution service.

## Consequences

- The six-tool and lifecycle-disabled assertions are obsolete as current authority.
- Historical Stage 1 fixtures may remain only when clearly labeled historical.
- Stage 3 incoming/slice traversal tools remain reserved `NOT_IMPLEMENTED`.
- Existing lifecycle algebra, readiness correlation, cleanup, and terminal-generation tests remain authoritative.
