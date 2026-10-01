# ADR 0011 target-discovery integration checklist

**Status:** PROPOSED, UNFROZEN, UNEXECUTED. This checklist organizes a private usability integration check. It is not ADR 0011 qualification, production admission, a qualification-matrix row, public enablement, or permission to retain live payloads. Repository fixtures must be synthetic. Private external workspaces may provide non-retained usability feedback, but their paths, names, symbols, source, responses, and traces must not enter repository history.

## Purpose

Verify the complete bounded path from a human-meaningful target query to an invocation-ready structural-context request:

```text
session alias
→ exact READY session, generation, and canonical workspace
→ bounded document/symbol candidates
→ explicit relation-family eligibility
→ exact invocation-ready URI/position request
→ structural context or one typed terminal recovery
```

The check measures whether a local user can construct a valid request. It does not establish feature identity, source completeness, runtime execution, ownership, importance, producer authentication, or relation evidence that the selected operation did not return.

## Frozen principles for this check

1. Resolution is bounded, fail-closed, generation-specific, and non-mutating.
2. Ambiguity never selects the first candidate.
3. A class or other non-callable symbol may be eligible for `REFERENCES_SYMBOL` or `RESOLVES_TO_DEFINITION` while remaining ineligible for `CALLS`.
4. CALLS eligibility is limited to exact methods, constructors, and functions. A class query may suggest callable children but must never silently become a method request.
5. Candidate, transaction-local target, occurrence, grouping-node, display, and durable evidence identities remain distinct.
6. Source text, LSP symbol metadata, source projection, `CALLS`, `REFERENCES_SYMBOL`, and `RESOLVES_TO_DEFINITION` are separately labeled evidence families. Source text cannot manufacture an LSP symbol or relation.
7. No failure may silently restart a session, select another generation, prepare another workspace, retry, increase a bound, traverse, hydrate, publish, retain, group, run Leiden, or generate a description.
8. Authority remains `0`, acceptance remains `false`, completeness remains `UNKNOWN`, and producer authentication remains absent.
9. Omitted server defaults and explicitly supplied values equal to those defaults are semantically equivalent. Explicit depth fields must not create a seed-depth mismatch that the omitted form avoids.
10. Artifact delivery mode is independent of graph completeness. Publishing a bounded large artifact by receipt rather than returning it inline must not by itself change a complete graph to `PARTIAL`; publication likewise cannot establish completeness.
11. Direct MCP exposure is a supported bounded mode. Its exact model-facing tool names, descriptions and JSON Schemas must fit a measured advertisement budget and complete an ordinary model request, or fail before registration with a specific size diagnostic rather than an opaque provider error.

## Required output of discovery

A successful discovery result must make one subsequent invocation possible without consulting undocumented defaults. It reports:

- caller-supplied session alias and resolved exact session ID;
- exact generation and READY state;
- canonical workspace root and target workspace-relative/canonical `file:` URI;
- negotiated position encoding;
- query text or exact input position;
- deterministic candidate order and bounded accounting;
- each candidate's name, symbol kind, URI, selection range, declaration range when available, and stable transaction-local candidate ID;
- explicit requested relation family and per-family eligibility or typed rejection for CALLS, references, and definitions; no implicit family selection;
- the exact chosen canonical `file:` URI plus zero-based line/character in the negotiated encoding;
- every required field and selected bound for an actually supported operation, including exact session ID, generation, workspace identity, and target URI/position; otherwise a typed stop rather than a pretend invocation;
- equivalent direct-MCP and canonical gateway request forms only where that operation is actually supported; no invented CLI/MCP endpoint;
- explicit evidence labels and claim ceiling;
- one terminal disposition, including observed-versus-limit accounting when a limit is involved.

Ordinary output omits raw JSON-RPC frames, source bodies, private paths outside the canonical workspace identity, server stderr, and retained selectors.

## Explicit relation-family routing (proposed integration contract)

The caller names exactly one relation family before operation selection. Resolve the alias to an **exact READY session ID and generation** and verify its canonical workspace, exact in-workspace `file:` URI, negotiated position encoding, and zero-based target line/character before dispatch. A stale generation or mismatched workspace is a terminal typed result for that attempt, not an invitation to substitute another session or URI. Candidate ranges remain evidence for choosing an exact point, not a relation claim.

| Requested family | Admissible target and route | Ineligible or unavailable route |
| --- | --- | --- |
| `CALLS` | Exact function, method, or constructor only; use an actually supported direct `lsp_trace_v2_structural_context` request or `lsp_trace_v1_execute` with `request.operation="lsp_trace_v2_structural_context"` and the same canonical arguments. Include explicit relation selection where the supported contract requires it. | Class and other non-callable targets stop as `INVALID_TARGET_KIND`; callable children are suggestions, never auto-selection. No references/definition or text fallback can manufacture `CALLS`. |
| `REFERENCES_SYMBOL` | A class or other non-callable target may be eligible under its **separately admitted** method, exact query-target receipt, source/applicability and capability gates, and an actually exposed endpoint. | Until those gates and endpoint are present, stop as `FAMILY_NOT_ENABLED` or `OPERATION_UNSUPPORTED`; do not present the proposed private path as a public MCP/CLI operation or route it through CALLS structural context. |
| `RESOLVES_TO_DEFINITION` | A class or other non-callable query point may be eligible under separately admitted method, query-target/source/applicability and capability gates, and an actually exposed endpoint. | Until then, the same typed not-enabled/unsupported stop applies; no implicit method invocation or fabricated definition result. |

Before any provider request, validate family selection, exact target and transport support. Distinguish `INVALID_REQUEST` (missing/conflicting/malformed selectors), `UNKNOWN_SESSION`/`UNKNOWN_TARGET`, `STALE_GENERATION`, `WORKSPACE_MISMATCH`, `AMBIGUOUS_TARGET`, `INVALID_TARGET_KIND`, `FAMILY_NOT_ENABLED`, and `OPERATION_UNSUPPORTED`; preserve the operation's actual typed domain outcome when it proceeds. These labels are proposed integration dispositions, not a claim that every named code is a currently registered public error. Rejected routes issue **zero provider requests**. A supported route emits its complete invocation-ready request with chosen bounds; an unsupported or unadmitted route emits a typed stop and at most an explicitly unexecuted next-attempt template. Source-text candidates, symbol metadata, server-reported CALLS, D/R method results, source projection and retained custody are independently labeled; neither text nor shared location establishes a CALLS edge. All results retain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `NO_PRODUCER_AUTHENTICATION` as applicable.

## Synthetic fixture set

Fixtures use neutral names and synthetic workspaces only.

| ID | Fixture | Required behavior |
| --- | --- | --- |
| UX-01 | Exact callable method | One method candidate is CALLS-, references-, and definitions-eligible as supported by the selected provider; emit an invocation-ready request without traversal. |
| UX-02 | Exact class with callable children | Class is explicitly non-callable for CALLS; bounded callable children may be suggested but none is selected. Class eligibility for references/definitions is evaluated independently. |
| UX-03 | Ambiguous symbol name | Return all admitted candidates within the bound and an ambiguity disposition; do not choose by order, proximity, or recency. |
| UX-04 | Stale generation | Return the current exact replacement session/generation when safely known and an invocation-ready next-attempt template; do not restart, rebind, or retry. |
| UX-05 | Alias/session absent | Distinguish unknown alias, missing exact session, non-READY state, and stopped generation. Do not collapse them into target-not-found. |
| UX-06 | Workspace mismatch | Distinguish canonical session workspace, prepared/source workspace, URI outside workspace, alias/symlink spelling, and document-not-registered outcomes. No ambient-path fallback. |
| UX-07 | Candidate truncation | Report candidate count observed, count returned, selected bound, deterministic continuation/paging information if supported, and that absence from the page is not absence from the workspace. |
| UX-08 | `TARGET_NOT_FOUND` with symbols present | Report matching scope, candidate names/kinds/ranges, omitted count, relation-specific guidance, and whether containment or exact-name matching failed. |
| UX-09 | `PREPARE_RETURNED_NO_ITEM` | Identify preparation as the failed stage, preserve zero relation claims, and provide only safe next actions; no source-text substitution. |
| UX-10 | `SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE` | Keep server-reported call-site and caller/display ranges distinct, identify the violated invariant, and do not repair or manufacture a CALLS edge. |
| UX-11 | Bounded resource failure | Name the exhausted resource, observed and declared values, partial/frontier disposition, and a safe explicit adjustment when one exists. Do not append request-schema help to a valid domain failure. |
| UX-12 | Invalid request shape | Report the missing/conflicting field and a corrected request fragment. Schema guidance is allowed here because validation, not domain execution, failed. |
| UX-13 | Explicit document-regex/source-text lookup | Label results as source-text candidates only. Require a later exact LSP locator step before any LSP symbol or relation claim. |
| UX-14 | Same point across relation families | Preserve separate CALLS, references, and definition eligibility/results and provenance; never deduplicate or relabel one family as another. |
| UX-15 | Direct MCP advertisement budget | Configure direct exposure, capture the exact serialized model-facing tool advertisement and per-tool description/schema bytes, and complete one ordinary model request. Over-budget configuration must fail before registration with a typed size diagnostic; a generic provider `500` is unacceptable. Compare gateway-only and direct request sizes without treating connectivity or tool count alone as usability proof. |
| UX-16 | Explicit trace-depth parity | Trace one exact seed with omitted depth fields and with every documented default supplied explicitly. Require identical seed depth, traversal semantics, terminal status and graph identity; reject a `seed 0 depth mismatch` or any explicit-default-only failure. Repeat at selected non-default depth boundaries. |
| UX-17 | Large graph publication semantics | Produce a bounded graph too large for inline return and publish it by immutable receipt. Keep `COMPLETE`/`PARTIAL` graph semantics independent from `INLINE`/`PUBLISHED_BY_RECEIPT` delivery; receipt delivery alone cannot cause `PARTIAL`. Verify selector, artifact length, digest, readback and truthful truncation accounting. |

## Routing test matrix (proposed; not executed)

| Case | Input and expected disposition | Provider requests / parity assertion |
| --- | --- | --- |
| UX-01 | Exact READY session/generation/workspace and URI/position, explicit CALLS, exact method/function/constructor; emit complete direct V2 and canonical gateway forms only when both are supported. | One explicitly invoked request per supported surface; compare arguments and evidence labels, not envelopes. Unsupported surface is a typed stop, not a parity failure against a nonexistent operation. |
| UX-02 | Exact class with CALLS selected: `INVALID_TARGET_KIND`; separately show references/definition eligibility only if their method, query-target, source/applicability, capability and exposed-endpoint gates are met. | Zero provider requests for rejected routes; no callable-child auto-selection and no public D/R endpoint pretense. |
| UX-14 | Same point with separately selected CALLS, references, definitions: retain distinct provenance, eligibility, orientation and terminal outcomes; no family default or cross-family deduplication. | Zero provider requests for each unavailable family; parity only for an actually supported operation, never text-to-CALLS. |
| UX-04/05 | Stale generation, unknown alias/session, or non-READY session: typed stage-specific stop and unexecuted correction only when safely known. | Zero provider requests; no restart/rebind/retry. |
| UX-06 | Outside, alternate-spelling, unregistered, or prepared/source-mismatched workspace URI: typed stage-specific stop. | Zero relation-provider requests; no ambient-file fallback. |
| UX-03/12/13 | Ambiguous exact target; malformed/missing/conflicting family or position; regex/source-text candidate without exact LSP locator: respective `AMBIGUOUS_TARGET`/`INVALID_REQUEST`/source-text-only stop. | Zero relation-provider requests; no first-match, inferred family or textual CALLS. |

These are proposed assertions and fixtures to construct, not test results or D/R qualification evidence.

## Integration scenarios

### A. Happy-path calls-to-context

1. Resolve a configured alias to one exact READY session and generation.
2. Display canonical workspace and encoding.
3. Search one synthetic URI for an approximate method name under explicit candidate/work/byte limits.
4. Select one exact callable candidate explicitly.
5. Produce direct and gateway invocation-ready structural-context requests only when each operation is supported; otherwise stop with the typed unavailable result.
6. Invoke exactly once through each supported surface against the same generation.
7. Compare semantic arguments, terminal status, authority/completeness labels, and relation/source labeling.

### B. Class-versus-callable recovery

1. Search for an exact class name.
2. Show that CALLS rejects `SymbolKind=CLASS` without implying provider failure.
3. Show bounded callable children as suggestions only.
4. Show independent references/definitions eligibility for the class.
5. Require explicit user selection before any method request.

### C. Stale-generation next attempt

1. Submit a request against a known stale generation.
2. Return `STALE_GENERATION`, not `SESSION_NOT_FOUND` or `TARGET_NOT_FOUND`.
3. Identify the current exact READY generation when host state permits disclosure.
4. Emit a corrected request template without executing it.
5. Confirm observed provider request count remains zero for the stale attempt.

### D. Workspace and registration diagnostics

Exercise canonical workspace match, prepared/source mismatch, outside-workspace URI, alternate URI spelling, unregistered document, and preparation returning no item. Every case must identify the failing stage and must not use ambient files, alternate sessions, or source-text fallback.

### E. Bounds and partial-result explanation

Exercise candidate, page, message, byte, request, node, depth, document, and work limits independently where the selected operation exposes them. Each response distinguishes discovery, structural traversal, and source-projection accounting. A projection failure cannot rewrite established structural facts; structural truncation cannot be hidden by successful projection.

### F. Direct-exposure and trace-default compatibility

1. Start from one fresh bounded session with gateway-only exposure and one with direct exposure.
2. Capture exact model-facing advertisement bytes and per-tool description/schema accounting before any model request.
3. Require one ordinary request to succeed in both modes; distinguish MCP connection success from model-request usability.
4. Trace the same exact seed with depth fields omitted, explicitly set to documented defaults, and set to selected non-default boundaries.
5. Require omitted/default-equivalent requests to produce the same seed depth and graph semantics.
6. For a graph delivered by publication receipt, record semantic completeness and transport delivery separately and verify the published bytes by length, digest and fresh readback.

## Surface parity

For each applicable scenario compare **only actually supported operations**:

- direct MCP operation, if exposed;
- MCP gateway `lsp_trace_v1_execute` using the canonical unqualified operation name, if supported;
- local CLI only when an equivalent command actually exists. Do not infer a public D/R CLI or MCP operation from a private proposal.

Parity means equivalent selectors, required fields, bounds, terminal semantics, evidence labels, authority ceiling and omitted-versus-explicit-default behavior—not byte-identical transport envelopes. Historical operations and omitted-selector CALLS behavior remain unchanged. A hidden-but-callable operation must be identified as such rather than reported unsupported. Direct exposure must be compared with gateway-only exposure using exact serialized advertisement accounting; `tools/list` success alone is insufficient.

Record declared and observed counts for session/status calls, document-symbol calls, preparation calls, relation calls, retries, publications, and retained objects. Forbidden implicit actions must remain zero.

## Measurements

Record at least:

- calls from alias plus approximate symbol to a valid invocation-ready request;
- calls from alias plus approximate symbol to first structural context;
- stale-generation calls to a corrected unexecuted next-attempt request;
- wrong-family attempts before valid selection;
- proportion of fixtures resolved from one bounded discovery response;
- candidate counts observed/returned/omitted;
- provider requests by stage and relation family;
- response summary bytes versus underlying raw provider payload bytes when privately measurable without retention;
- number of hidden retries, restarts, fallbacks, traversals, publications, or retained objects, which must be zero;
- exact direct and gateway-only model-facing advertisement bytes, estimated tokens, per-tool description bytes, per-tool schema bytes, and largest-tool identity;
- omitted/default-explicit/non-default depth values, resulting seed depth and graph identity;
- graph completeness independently from inline-versus-publication-receipt delivery.

No latency, allocation, or throughput claim may be made without a separately controlled measurement.

## Acceptance checklist

The integration check is acceptable only when all are true:

- [ ] UX-01 through UX-17 have synthetic fixtures or an explicit `INCOMPLETE` disposition.
- [ ] Happy-path requests are invocation-ready without undocumented field discovery.
- [ ] Classes never silently become callable targets.
- [ ] References and definitions are not restricted by CALLS-only callable rules.
- [ ] Stale generation produces a corrected but unexecuted next-attempt request.
- [ ] Workspace, registration, preparation, target matching, traversal, and projection failures remain distinguishable.
- [ ] Truncation and every exercised limit expose deterministic accounting and partial/frontier disposition.
- [ ] Schema help appears only for request-validation failures.
- [ ] Source text never becomes LSP symbol or relation evidence.
- [ ] Explicit relation-family routing has no implicit family selection; rejected routes have zero provider requests and supported routes emit complete invocation-ready fields or a typed stop.
- [ ] Direct/gateway parity passes only for actually supported operations; CLI differences are documented and typed.
- [ ] Direct exposure remains within the measured model-facing advertisement budget and completes an ordinary request, or fails before registration with a typed size diagnostic.
- [ ] Omitted and explicitly supplied default trace depths are semantically equivalent and never produce a seed-depth mismatch.
- [ ] Graph completeness is reported independently from inline or publication-receipt delivery, with exact published length/digest/readback evidence.
- [ ] Declared and observed provider/action counts agree.
- [ ] Hidden retry, restart, fallback, traversal during discovery, publication, and retention counts are zero.
- [ ] Responses preserve `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `NO_PRODUCER_AUTHENTICATION` where applicable.
- [ ] No private external source, path, name, response, trace, or golden file appears in the repository packet.

## Result and stop boundary

The result is one proposed integration report with a per-fixture disposition of `PASS`, `FAIL`, or `INCOMPLETE`, exact synthetic fixture and implementation revisions, selected limits, surface versions, observed action counts, and unresolved usability defects.

`PASS` means only that the bounded target-discovery experience met this checklist in the tested synthetic environment. It does not qualify references or definitions, count an ADR 0011 matrix row, admit grouping input, authorize production/live retention, enable a public operation, establish performance, or accept ADR 0007 descriptions.

Stored-procedure or database target discovery is outside this checklist. It requires an independent typed provider/evidence path and must not be represented as LSP CALLS or source-text inference.
