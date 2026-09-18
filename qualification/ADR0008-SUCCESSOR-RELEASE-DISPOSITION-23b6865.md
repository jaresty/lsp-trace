# ADR 0008 Successor Release Disposition — 23b6865

## Decision

ADR 0008 is **ready for lsp-trace release under the accepted external L04 exception**.

This successor consolidates existing reviewed results; it does not rerun cells or alter historical records.

- `COMMON_PROJECTION=PASS` (`C01–C06=PASS`)
- `RETAINED_OBJECT_RESOLVER=PASS` (`R01–R04=PASS`)
- `L04=FAIL` remains unchanged
- `release_ready=true`
- `implementation_qualified=false`
- continued lsp-trace work does not wait for the external adapter fix

The machine-readable decision is:

- `qualification/adr0008-successor-release-disposition.23b6865088f68142ed0b3243f3acee064217558b.v1.json`

## Why these statuses differ

`implementation_qualified` is the strict frozen-matrix verdict. Because L04 remains failed, it remains false.

`release_ready` is the operational disposition. The accepted exception establishes that L04 is externally owned by `pi-mcp-adapter` and is not a blocker for an lsp-trace release. The qualified lsp-trace tracks therefore proceed without waiting for that dependency.

## External follow-up

The verified adapter repair is proposed upstream as:

- <https://github.com/nicobailon/pi-mcp-adapter/pull/596>

An official release containing the repair may trigger an additive L04 rerun. The PR, its merge, or a private build does not itself change the verdict.

## Preserved ceilings

- Graph authority remains `0`.
- Semantic acceptance remains `false`.
- Source-graph completeness remains `UNKNOWN`.
- Canonical operations remain `41`; compact-advertised tools remain `12`.
- Released contracts remain immutable; evolution remains additive.

## Historical integrity

The governing matrix, historical L04 execution, accepted exception, and R01–R04 qualification records remain unchanged. Any later L04 result must be recorded additively.
