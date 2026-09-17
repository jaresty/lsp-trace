# ADR 0008 Qualification Exception: L04 Host Rendering

## Decision

`L04` has an **accepted external exception** for lsp-trace release decisions when observed through `pi-mcp-adapter` 2.32.1.

This exception does not change the reviewed qualification verdict. `L04` remains `FAIL`, and `implementation_qualified` remains `false`.

The machine-readable decision is:

- `qualification/adr0008-source-projection-exception-l04.v1.json`

The unchanged reviewed execution is:

- `qualification/adr0008-source-projection-matrix.execution.5d392f9710225bd1761af67fd9d851c2ff4b2f82.json`

## Observed issue

The host adapter appends `Expected parameters` guidance to MCP runtime errors whenever an advertised input schema exists. This conflates runtime or domain failure presentation with request-schema validation failure presentation.

The behavior is externally owned by `pi-mcp-adapter`; lsp-trace must not compensate by weakening its envelopes, changing its advertised schemas, or introducing host-specific output.

## Scope

The exception means only that the observed `L04` presentation defect is not a blocker for an lsp-trace release.

It does not:

- convert `L04` to `PASS`;
- qualify the ADR 0008 implementation;
- establish that every canonical server envelope is correct;
- waive request validation or privacy requirements;
- authorize a local patch to Pi or `pi-mcp-adapter`;
- alter the historical execution record.

## Revalidation

Re-run `L04` without assuming this exception when any of the following occurs:

1. the installed `pi-mcp-adapter` version differs from 2.32.1;
2. the adapter changes direct-tool error rendering;
3. the `L04` criteria change; or
4. lsp-trace changes the affected MCP envelope or advertised input schema.

If the exception is still needed, record an additive successor rather than editing this record.
