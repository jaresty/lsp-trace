# Bounded retained analytics v2

Status: source-ready, unshipped. Deployment availability is unknown.

This local bounded analytics surface is qualified only with synthetic retained graph fixtures. It does not grant permission, authenticate production evidence, admit Program B, or authorize a production decision.

## Public routes

CLI routes are `bounded-retained-analysis-v2`, `bounded-retained-metrics-v2`, and `bounded-retained-ranking-v2`. MCP routes are `lsp_trace_v2_bounded_retained_analysis`, `lsp_trace_v2_bounded_retained_metrics`, and `lsp_trace_v2_bounded_retained_ranking` in additive outer composition layer 28. Historical v1 commands, aliases, schemas, defaults, bytes, and the 25-tool registry snapshot are unchanged.

Each route requires its matching `operation`, at least one relation `filter`, positive `max_work`, and exactly one retained-graph carrier: inline canonical bytes (JSON string or raw object) or `publication_selector`. A selector is relative to the process-pinned publication root and binds its selector, exact artifact schema ID, SHA-256 digest, and byte length. Absolute, escaping, missing, symlinked, wrong-family, wrong-length, and wrong-digest selectors are rejected before analytics; inline bytes and a selector are mutually exclusive. Old graph families are rejected and no implicit adapter exists.

Results are documented unverified local deterministic evidence and preserve the certified package payload bytes exactly. Rooted selectors separately verify input schema, digest, and byte length before evaluation; authenticated provenance remains an optional publication receipt/verifier concern and never adds fields to or otherwise mutates the result payload. A caller-supplied digest or provenance label alone establishes no authority, and tamper or substitution fails admission.

CLI and MCP return byte-identical canonical artifacts for equivalent carriers at both COMPLETE and LIMIT. MCP uses additive v2-specific artifact, publication, compact-publication, publication-error, and domain-error envelope schemas. Output publication, when configured, retains the existing owner-only atomic no-replace semantics.

## Public schema mapping

| Public family | Version | Exact embedded contract bytes |
|---|---:|---|
| `bounded-retained-graph-v2` | `v2` | `lsp-trace.normative-retained-graph.v1` |
| `bounded-retained-analysis-v2` | `v2` | `lsp-trace.local-normative-analysis.v1` |
| `bounded-retained-metrics-v2` | `v2` | `lsp-trace.local-normative-metrics.v1` |
| `bounded-retained-ranking-v2` | `v2` | `lsp-trace.local-normative-ranking.v1` |

The mapping deliberately leaves the four package IDs unchanged. `schema-get`, validation, and exact receipt verification support all four v2 family mappings; immutable no-replace publication remains the shared public mechanism. Historical 25-tool composition and its schema/alias bytes remain unchanged, while the additive current composition remains exactly 28 canonical tools.

## Result scope

Results retain package-owned canonical operation payloads, exact incremental accounting, COMPLETE/LIMIT status, input and result digests, relation filters, and synthetic qualification scope. Multiplicity, structural relation identity, independent support qualification, density, and ranking behavior are defined by the frozen package implementation.
