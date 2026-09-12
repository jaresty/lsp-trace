# MCP advertisement profile contract

Status: preparation contract for ADR 0004; not yet implemented

This document fixes the next MCP registry simplification boundary. It does not change the current registry. In particular, it does not add `lsp_trace_v1_trace` or `lsp_trace_v1_discover`, rename a tool, alter a schema, change dispatch, or assign either future tool an operation number.

## Terms

- **Dispatchable**: accepted by exact canonical name through `lsp_trace_v1_execute` with the operation's existing request, response, error, envelope, schema, limit, and behavior contract.
- **Advertised**: returned as a direct MCP tool by the selected presentation profile.
- **Default**: the small ordinary-agent workflow surface.
- **Advanced**: the default surface plus specialist, administrative, and durable compatibility-reader operations.
- **Hidden legacy**: not advertised by any profile, but canonically dispatchable during the compatibility window.

Aliases are never advertised. Advertisement does not change availability or dispatch semantics. Profile selection is process-lifetime immutable.

## Normative profile sets

The future **default** advertisement set is exactly:

```text
lsp_trace_v1_capabilities
lsp_trace_v1_discover            # intentionally unmet: not implemented or numbered
lsp_trace_v1_execute
lsp_trace_v1_inspect_hydrated
lsp_trace_v1_program_c_leiden
lsp_trace_v1_trace               # intentionally unmet: not implemented or numbered
lsp_trace_v1_verify
```

The future **advanced** advertisement set is exactly the default set plus:

```text
lsp_session_v1_list
lsp_session_v1_restart
lsp_session_v1_status
lsp_session_v1_stop
lsp_trace_v1_bounded_retained_analysis
lsp_trace_v1_bounded_retained_metrics
lsp_trace_v1_bounded_retained_ranking
lsp_trace_v1_custody_execute
lsp_trace_v1_export_retained_calls
lsp_trace_v1_filter
lsp_trace_v1_inspect
lsp_trace_v1_program_c_compose
lsp_trace_v1_program_c_instability
lsp_trace_v1_schema_get
lsp_trace_v1_validate
lsp_trace_v2_bounded_retained_analysis
lsp_trace_v2_bounded_retained_metrics
lsp_trace_v2_bounded_retained_ranking
lsp_trace_v2_export_retained_calls
lsp_trace_v2_verify
lsp_trace_v2_verify_retained_calls
```

The **hidden legacy** set is exactly:

```text
lsp_trace_v1_incoming
lsp_trace_v1_slice
lsp_trace_v2_incoming
lsp_trace_v2_slice
lsp_trace_v3_incoming
lsp_trace_v3_slice
```

The three current-operation classes are disjoint and cover all 32 current operations. Advanced advertises `default ∪ advanced-only`; hidden legacy advertises nothing. Hidden legacy names remain discoverable through operation description/capabilities metadata and invocable only through canonical execute when their direct tools are hidden.

### Classification rationale

ADR 0004 explicitly makes versioned acquisition and historical `slice`/`incoming` spellings legacy. Durable readers, validators, verifiers, exports, lifecycle administration, custody execution, and specialist analytics remain advanced rather than legacy because the ADR preserves historical evidence access and defines advanced as specialist and administrative. Leiden is the ordinary derived-analysis lane; composition and instability remain advanced because they require specialist admissibility/accounting decisions.

## Frozen compatibility ledger

Operation numbers are append-only compatibility identities, not lexical display positions.

| No. | Canonical operation | Target class |
|---:|---|---|
| 1 | `lsp_trace_v1_capabilities` | default |
| 2 | `lsp_trace_v1_schema_get` | advanced |
| 3 | `lsp_trace_v1_validate` | advanced |
| 4 | `lsp_trace_v1_verify` | default |
| 5 | `lsp_trace_v1_inspect` | advanced |
| 6 | `lsp_trace_v1_filter` | advanced |
| 7 | `lsp_trace_v1_execute` | default |
| 8 | `lsp_session_v1_list` | advanced |
| 9 | `lsp_session_v1_status` | advanced |
| 10 | `lsp_session_v1_restart` | advanced |
| 11 | `lsp_session_v1_stop` | advanced |
| 12 | `lsp_trace_v1_incoming` | hidden legacy |
| 13 | `lsp_trace_v1_slice` | hidden legacy |
| 14 | `lsp_trace_v1_export_retained_calls` | advanced |
| 15 | `lsp_trace_v1_bounded_retained_analysis` | advanced |
| 16 | `lsp_trace_v1_bounded_retained_metrics` | advanced |
| 17 | `lsp_trace_v1_bounded_retained_ranking` | advanced |
| 18 | `lsp_trace_v1_inspect_hydrated` | default |
| 19 | `lsp_trace_v2_export_retained_calls` | advanced |
| 20 | `lsp_trace_v2_verify_retained_calls` | advanced |
| 21 | `lsp_trace_v2_slice` | hidden legacy |
| 22 | `lsp_trace_v2_incoming` | hidden legacy |
| 23 | `lsp_trace_v2_verify` | advanced |
| 24 | `lsp_trace_v3_slice` | hidden legacy |
| 25 | `lsp_trace_v3_incoming` | hidden legacy |
| 26 | `lsp_trace_v2_bounded_retained_analysis` | advanced |
| 27 | `lsp_trace_v2_bounded_retained_metrics` | advanced |
| 28 | `lsp_trace_v2_bounded_retained_ranking` | advanced |
| 29 | `lsp_trace_v1_custody_execute` | advanced |
| 30 | `lsp_trace_v1_program_c_leiden` | default |
| 31 | `lsp_trace_v1_program_c_compose` | advanced |
| 32 | `lsp_trace_v1_program_c_instability` | advanced |

Operations 31 and 32 MUST remain composition and A-08 instability respectively. New operations MUST be appended after 32; profile work MUST NOT fill, move, reuse, or reinterpret an existing number.

For every operation 1–32:

1. exact canonical resolution remains available;
2. `lsp_trace_v1_execute` retains one schema branch for that canonical name (except execute itself, which is not recursively branched);
3. hidden status changes only direct advertisement, never canonical execute routing;
4. aliases, schemas, envelopes, limits, availability, and runtime behavior remain unchanged by profile selection.

## Current state and RED boundary

Today `full` advertises all 32 operations and `compact` advertises 10. Neither existing profile is silently redefined by this preparation change. The exact default/advanced advertisement tests are opt-in and intentionally RED under `LSP_TRACE_RUN_ADR0004_RED_GUARDS=1` because:

- `lsp_trace_v1_trace` is not implemented;
- `lsp_trace_v1_discover` is not implemented;
- default/advanced/hidden-legacy production profile behavior does not yet exist.

All non-opt-in tests are regression guards and MUST remain GREEN. A failure prefixed `REGRESSION` means current 32-operation compatibility changed. A failure prefixed `INTENTIONAL RED` means a future ADR 0004 advertisement requirement remains unmet; it does not authorize weakening a current compatibility guard.

## Implementation gate for the next step

A later production change may implement profile advertisement only after separately deciding tool-profile names/configuration and adding trace/discover contracts. That change must make the opt-in target-profile guard GREEN without changing the frozen ledger or canonical execute branches. Hard removal of a legacy canonical name remains out of scope until a future MCP protocol version and separate migration evidence authorize it.
