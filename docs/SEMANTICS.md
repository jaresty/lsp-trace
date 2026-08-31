# Graph semantics

An edge means the configured language server reported a static incoming Call Hierarchy relation. It does not prove runtime execution, reachability on a deployed configuration, frequency, or whole-program completeness. Discovery proceeds from callee to callers, while each edge is serialized in execution orientation: caller to callee.

Completeness is bounded by server capability and responses plus visible depth, node, timeout, cancellation, and error frontiers. Unsupported Call Hierarchy never authorizes a generic-reference fallback. Cycles describe graph structure; they are not roots or proof of runtime recursion.

The qualification fixtures include `StaticButNotExecuted`, whose call is behind a branch normal fixture execution does not take. A server may still report that edge, demonstrating the distinction between static evidence and observed execution.

## Completeness and bounds

`summary.complete` is true only when every discovered node was expanded successfully or reached a natural terminal. It is false when capability, cancellation, timeout, server error, invalid response, or a safety bound prevents complete expansion. `summary.truncated` is narrower: it reports depth or node-count truncation, not every cause of incompleteness. Consumers must inspect `terminals`, `frontier`, and `diagnostics` rather than infer a reason from either Boolean alone.

A terminal records a node that was expanded or intentionally stopped; a frontier records a discovered node that remains unexpanded. Reasons are stable machine-readable values defined by the schema policy. `MAX_DEPTH` and `MAX_NODES` identify user bounds. The global `--timeout` bounds the whole command and produces `GLOBAL_TIMEOUT` when traversal-wide time expires; `--request-timeout` bounds each LSP request and produces `REQUEST_TIMEOUT`. Cancellation surfaces as `CANCELLED`, and an interrupt causes exit status `130` after JSON emission when possible.

## V3 custody and identity

V3 preserves server-relative completeness, caller-to-callee call edges, zero-support discovery nominations, and valid zero-discovery output. Caller metadata is explicitly `CALLER_ASSERTED`. Tool-derived identity is limited to successfully resolved seed URI/content-digest pairs and a domain-separated aggregate scoped `RESOLVED_SEED_CONTENTS`; failures never become identities, and no repository, clock, server, version, or workspace-wide truth is inferred.

The embedded semantic digest commits to canonical bundle meaning. The detached sidecar commits separately to exact serialized bytes. `lsp-trace verify PATH` checks both offline and emits no graph on success. Neither digest authenticates who produced the bundle.

## Traversal completeness and provenance

The v2 graph document is also the invocation receipt. Its `invocation` object packages the effective workspace, target, server command and arguments, and traversal limits. Its `tool` object identifies `lsp-trace`. Invocation provenance is caller-authoritative: the provenance flags for invocation ID, caller, source, source revision, server version, timestamp, and tool version are copied exactly, while omitted values are `"UNKNOWN"`. No invocation provenance is inferred from ambient process, repository, current clock, resolved path, language server, or CI state. These receipt additions do not change the stream contract: stdout remains one graph JSON document and diagnostics remain on stderr.

`evidence_semantics` states the claim ceiling directly. A call edge supports only that the configured server reported the canonical caller-to-callee relation. It does not prove runtime execution, feature identity, whole-source completeness, or independent source confirmation. Discovery relations nominate separate investigation and contribute zero call support. V3 assigns stable identities to all three relation families and separately binds each seed occurrence to the call relations and discovery evidence produced by that seed; a shared source URI does not collapse distinct seed occurrences.

V3 `process_context` embeds no inherited or explicit environment values and no working-directory path. It records domain-separated identities for the effective inherited-plus-override environment and cwd, the effective variable count, and explicit redaction state. These fields bind the issuer's tool-derived execution-context claim into the semantic receipt; offline verification checks their structure and receipt commitment but cannot independently reconstruct or authenticate hidden process inputs. `replay_input_manifest` separately records source, protocol-transcript, and stderr artifacts with explicit `PRESENT` or `ABSENT` state.

Within that document, `evidence_receipt` is the evidence-accounting envelope for optional discovery nominations and associations. Every projected record is a discovery nomination with zero support contribution. Its domain-separated stable identity denotes the caller-supplied source revision, relation kind, direction, locator, and semantic endpoints—not its array position or discovery order. Consumers must not count these records as call edges, caller support, or proof that a nominated or associated symbol participates in execution.

`trace_receipt.content_digest` commits to the canonical compact v2 graph content with `trace_receipt` omitted. The exclusion avoids self-reference. A matching digest proves that the receipt content is unchanged under this encoding; it does not establish that caller-supplied provenance is true or promote structural evidence into runtime evidence.

Traversal completeness is server-relative, not source completeness. For example, source inspection may show `CrossModuleCallers.aliased_cross_file/1` calling `Calls.leaf/1`, yet a server may return zero callers for `leaf/1`. In that result, `NO_INCOMING_CALLS` means only that the configured server reported an empty incoming-call list for that prepared item; it does not erase the known source caller or prove no caller exists.

Terminal provenance is carried by the terminal's `node_id`, `reason`, and optional `message`, interpreted alongside the invocation's server command, capabilities, target, and retained server version. The v1 graph does not embed a source-inspection oracle or server version, so those belong in qualification evidence rather than new graph fields. A complete traversal can therefore be incomplete relative to source truth, and a source-known/server-zero-caller discrepancy blocks the relevant interoperability qualification without changing `lsp-trace.graph.v1` semantics.
