# MCP advertisement profile contract

Status: current advertisement contract for ADR 0004

The current profiles below remain the historical contract. The proposed incompatible `compact-vnext` generation is governed by [ADR 0012](adr/0012-context-bounded-mcp-vnext.md); no vNext profile is active or authorized yet.

This document fixes the MCP registry simplification boundary. The current registry implements profile-filtered advertisement and canonical `lsp_trace_v1_trace` operation 33. It does not add `lsp_trace_v1_discover`, rename historical tools, or renumber operations 1–32.

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
lsp_trace_v1_trace
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

At the operation-33 baseline, the three classes were disjoint and covered the 33 then-current operations: six implemented default operations, 21 advanced-only operations, and six hidden-legacy operations; the listed but intentionally absent `discover` operation was not counted. Advanced advertises `default ∪ advanced-only`; hidden legacy advertises nothing. Hidden legacy names remain discoverable through operation description/capabilities metadata and invocable only through canonical execute when their direct tools are hidden.

Later append-only operations do not retroactively change that baseline count. The compatibility ledger below reaches operation 37, and later runtime/profile reports may include operations through 41. Those are distinct accounting epochs. A count is valid only when it names its ledger high-water mark, profile, and whether it counts canonical dispatchable operations or directly advertised tools.

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
| 33 | `lsp_trace_v1_trace` | default |
| 34 | `lsp_trace_v1_census` | default |
| 35 | `lsp_trace_v1_structural_context` | default (hidden but callable in compact) |
| 36 | `lsp_trace_v2_structural_context` | default (hidden but callable in compact) |
| 37 | `lsp_trace_v1_structural_delta` | default (hidden but callable in compact) |

Operations 31–37 are append-only. Profile work MUST NOT fill, move, reuse, or reinterpret an existing number. Operations 35, 36, and 37 remain callable through canonical execution but are not directly advertised by the compact compatibility profile represented by this ledger.

### Cardinality accounting

This document previously contained unqualified counts of ten, 12, and 11 compact tools alongside 33-, 37-, and 41-operation statements. They described different prospective, ledger, and runtime snapshots and MUST NOT be read as one timeless cardinality contract.

- **Operation-33 baseline:** 33 canonical dispatchable operations; six implemented default, 21 advanced-only, and six hidden legacy. The absent proposed `discover` operation is excluded.
- **Operation-37 ledger:** the append-only table in this document contains 37 canonical identities. Direct-advertisement status remains the per-row/profile decision, not the ledger count.
- **Later runtime compatibility snapshot:** `full` has reported 41 directly advertised operations and `compact` has reported an 11-tool curated surface. Those runtime counts require generated inventory evidence at the exact reviewed revision and are not derived from the operation-33 profile sets.
- **Proposed `compact-vnext`:** has no accepted cardinality. Its tool set and byte budget are governed by ADR 0012 and require separate qualification.

Any test, review, or migration claim that cites a cardinality MUST retain the source revision, registry high-water mark, profile name, canonical-operation count, directly-advertised-tool count, and serialized advertisement bytes. Generated registry/profile inventory is authoritative for a reviewed revision; prose counts are historical context only.

For every canonical operation:

1. exact canonical resolution remains available;
2. `lsp_trace_v1_execute` retains one canonical validation branch for that operation (except execute itself, which is not recursively branched);
3. MCP advertisement uses a bounded dispatcher presentation schema containing the canonical operation-name enum and an opaque arguments object, while dispatch still validates arguments against the selected operation's canonical schema;
4. the compact compatibility profile advertises its revision-bound curated set and its serialized tool metadata remains at or below the historical 40 KiB ceiling; `compact-vnext` receives a separate qualified budget;
5. hidden status changes only direct advertisement, never canonical execute routing;
6. aliases, canonical schemas, envelopes, limits, availability, and runtime behavior remain unchanged by profile selection.

## Current state and RED boundary

At the operation-33 baseline, the process default was `default`; it advertised the six implemented default operations and omitted the intentionally absent `lsp_trace_v1_discover`. `advanced` advertised those six plus the 21 advanced-only operations. The six hidden-legacy operations were omitted from both listings but remained canonically dispatchable with unchanged contracts.

Later explicit compatibility profiles remain separate revision-bound snapshots: one inspected runtime state reported `full` advertising 41 operations and `compact` advertising an 11-tool curated surface. They are not the production default and do not change registry membership or dispatch semantics. Before relying on either count for migration or vNext parity, regenerate and retain the exact registry/profile inventory at the target revision.

The future exact default advertisement test remains opt-in and intentionally RED under `LSP_TRACE_RUN_ADR0004_RED_GUARDS=1` because:

- `lsp_trace_v1_discover` is not implemented or numbered.

All non-opt-in tests are regression guards and MUST remain GREEN. A failure prefixed `REGRESSION` means the applicable revision-bound compatibility inventory or the frozen historical 1–32 ledger changed. A failure prefixed `INTENTIONAL RED` means a future ADR 0004 advertisement requirement remains unmet; it does not authorize weakening a current compatibility guard.

## Implementation gate for the next step

Adding the future discover default tool remains blocked until its canonical operation, versioned contracts, and append-only operation number are separately implemented. That later change must make the opt-in future-default guard GREEN without changing the frozen ledger or existing canonical execute branches. Hard removal of a legacy canonical name remains out of scope until a future MCP protocol version and separate migration evidence authorize it.
