# Bounded retained analytics v2

Status: source-ready, unshipped. Deployment availability is unknown.

This local bounded analytics surface is qualified only with synthetic retained graph fixtures. It does not grant permission, authenticate production evidence, admit Program B, or authorize a production decision.

## Public routes

CLI routes are `bounded-retained-analysis-v2`, `bounded-retained-metrics-v2`, and `bounded-retained-ranking-v2`. MCP routes are `lsp_trace_v2_bounded_retained_analysis`, `lsp_trace_v2_bounded_retained_metrics`, and `lsp_trace_v2_bounded_retained_ranking` in additive outer composition layer 28. Historical v1 commands, aliases, schemas, defaults, bytes, and the 25-tool registry snapshot are unchanged.

Each route requires exact canonical retained graph bytes, its matching `operation`, at least one repeated relation `filter`, positive `max_work`, and explicit `claim_level` (`VERIFIED` or `UNVERIFIED`). Provenance is optional context and does not grant authority. Old graph families are rejected; no implicit adapter exists.

## Public schema mapping

| Public family | Version | Exact embedded contract bytes |
|---|---:|---|
| `bounded-retained-graph-v2` | `v2` | `lsp-trace.normative-retained-graph.v1` |
| `bounded-retained-analysis-v2` | `v2` | `lsp-trace.local-normative-analysis.v1` |
| `bounded-retained-metrics-v2` | `v2` | `lsp-trace.local-normative-metrics.v1` |
| `bounded-retained-ranking-v2` | `v2` | `lsp-trace.local-normative-ranking.v1` |

The mapping deliberately leaves package IDs and bytes unchanged. `schema-get`, validation, immutable no-replace publication, and offline publication verification remain shared public mechanisms rather than widening frozen verifier semantics.

## Result scope

Results retain package-owned canonical operation payloads, exact incremental accounting, COMPLETE/LIMIT status, input and result digests, relation filters, and synthetic qualification scope. Multiplicity, structural relation identity, independent support qualification, density, and ranking behavior are defined by the frozen package implementation.
