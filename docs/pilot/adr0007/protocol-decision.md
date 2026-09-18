# G1 protocol decision draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Decision:** `NDJSON_OVER_STDIN_STDOUT`
- **Authority:** Draft preparation under ADR 0007; not a frozen or approved protocol.

## Boundary

The semantic worker is a separate backend-neutral subprocess. The caller communicates with it through newline-delimited JSON (NDJSON) over stdin/stdout. The worker has no network access, cannot read source files, traverse repositories, invoke tools, or select a fallback backend.

This transport is an implementation direction for G1 preparation only. It does not create a public CLI, MCP surface, registry entry, core dependency, or stable public wire name.

## Required properties before G1 can pass

- One complete JSON object per line; embedded newlines must be escaped.
- Explicit request/response correlation identifiers.
- Versioned envelope and schema identifiers.
- Canonical JSON serialization and digest rules.
- Strict unknown-field, duplicate-field, ordering, and maximum-line-size policy.
- Fail-closed malformed-frame handling.
- Explicit operation, cancellation, timeout, shutdown, and terminal-accounting semantics.
- Caller-supplied assembled admissions only; no worker-side source acquisition.
- Resource limits covering the worker and descendants.
- Deterministic closure behavior for EOF, cancellation races, timeout, crash, and forced termination.
- Conformance vectors covering valid, malformed, duplicate, incomplete, and unbalanced records.

## Non-goals

This decision does not authorize implementation, inference, indexing, model acquisition, Yzma selection, pilot execution, qualification, shipment, or integration into `lsp-trace`, its CLI, or `lsp-trace-mcp`.

## Open G1 decisions

The following remain unresolved until a later freeze record:

- protocol-visible operation and control vocabulary;
- envelope and field schemas;
- canonicalization and digest algorithm;
- request ordering and concurrency policy;
- cancellation representation and race semantics;
- retry and shutdown policy;
- resource and line-size limits;
- backend capability negotiation;
- exact accepted terminal-list references;
- conformance vector contents and expected outcomes.
