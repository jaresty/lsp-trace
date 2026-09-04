# ADR 0003: Activate always-local Stage 2 lifecycle tools

Status: Accepted

## Context

The previous staged contract advertised six offline MCP tools and retained four lifecycle names as unadvertised `NOT_IMPLEMENTED` candidates behind production-containment language. The product is now explicitly local-development-only and needs useful lifecycle control of trusted developer-configured language servers.

## Decision

The default stdio MCP surface advertises twelve canonical tools: the existing six offline evidence tools, `lsp_trace_v1_incoming`, `lsp_trace_v1_slice`, and `lsp_session_v1_list`, `lsp_session_v1_status`, `lsp_session_v1_stop`, and `lsp_session_v1_restart`. Canonical names and their existing unversioned aliases route lifecycle and traversal calls to one process-local session runtime.

On Darwin, process startup uses the existing local process-group supervisor and readiness/lifecycle APIs. A session becomes lifecycle-visible only according to the runtime readiness state. Stop and restart retain bounded asynchronous operation records, cancellation behavior, teardown/reap requirements, immutable terminal outcomes, and monotonically succeeding restart generations.

Unsupported platforms keep the same ten-tool discovery contract. Operations that require starting a process fail explicitly without starting a child. Platform support does not hide tools and is not expressed as a production-containment gate.

## Trust and safety boundary

**WARNING:** Child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted. Only configure commands you trust.

The default MCP publication is twelve canonical tools in deterministic canonical-name order. Incoming and slice validate all caller input before runtime effects, require retained initialize evidence for call hierarchy and position encoding, and perform bounded transactions through `sessionruntime.RoundTrip`. Incoming delegates graph traversal to `internal/traverse`; slice composes `internal/slicer` exact-depth outgoing discovery with incoming traversal from the sorted deduplicated union of exact-depth frontier nodes and genuine successful empty outgoing leaves. Failed or null outgoing responses remain incomplete and never become leaves.

This decision does not claim hostile-code safety, native containment, remote execution, or privileged isolation. The server is a local developer tool, not a multi-tenant or production execution service.

## Consequences

- The six-tool and lifecycle-disabled assertions are obsolete as current authority.
- Historical Stage 1 fixtures may remain only when clearly labeled historical.
- Wave 2 activates bounded incoming traversal over exact READY managed-session generations; Wave 3 activates bounded slice traversal over the same exact-generation runtime.
- Existing lifecycle algebra, readiness correlation, cleanup, and terminal-generation tests remain authoritative.
