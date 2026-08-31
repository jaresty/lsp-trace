# Product Requirements Document: lsp-trace

**Status:** Draft 0.1  
**Audience:** Principal engineers and initial implementers  
**Implementation:** Go  
**License:** To be selected before public release

## 1. Summary

`lsp-trace` is a standalone, language-neutral CLI that starts or connects to a Language Server Protocol server, resolves the symbol at a source position, and recursively follows incoming Call Hierarchy relations as far upward as that server can resolve. It emits a deterministic graph containing every visited code object, caller-to-callee edge, exact reported call-site range, terminal branch, cycle, capability gap, error, and explicit truncation boundary.

The tool does not identify domain-specific “roots,” interpret workflows, prove runtime execution, implement compiler semantics, or own source-control custody. It reports the static call relationships supplied by a language server and the exact boundaries of that evidence.

## 2. Problem

Language servers can often answer “who calls this symbol?” for one level through LSP Call Hierarchy. Developers investigating unfamiliar systems need the transitive answer: all resolvable callers, their call sites, and where every branch stops. Existing editor navigation requires repeated manual expansion, obscures branch completeness, and is difficult to automate or retain as evidence.

SCIP was evaluated against a pinned Ember JavaScript project. It provided useful symbol and cross-file reference identity but did not distinguish imports from calls or resolve important dynamic service operations. `lsp-trace` therefore targets the protocol operation that already represents calls: LSP Call Hierarchy. SCIP is neither a dependency nor an interchange requirement.

## 3. Users and jobs

### 3.1 Primary users

- Engineers orienting in unfamiliar codebases.
- Maintainers assessing callers before changing a symbol.
- Reviewers auditing the structural reach of a method or function.
- Automated analysis systems that need machine-readable call witnesses.
- Tool builders that want a neutral call-hierarchy graph without embedding an editor.

### 3.2 User jobs

Users need to:

1. Identify a symbol by a source position they already have.
2. Discover every caller the configured language server can transitively report.
3. See the code objects and exact call sites encountered along the way.
4. Know whether traversal completed or stopped because of a capability, error, timeout, or bound.
5. Re-run the same query and receive stable output when the workspace and server behavior are unchanged.
6. Feed the graph to another tool without adopting language-specific compiler APIs.

## 4. Product principles

### 4.1 Server-derived, not inferred

The core records only Call Hierarchy items and incoming-call relations returned by the server. It does not convert generic references into calls or infer framework behavior.

### 4.2 Complete within an explicit envelope

“As far as possible” means until every discovered branch reaches a natural or reported terminal, subject to visible user-configured safety bounds. No branch may disappear silently.

### 4.3 Evidence without runtime overclaim

An edge means the language server reported a static caller relationship. It does not prove the call executes in production, on every path, or under a particular configuration.

### 4.4 Deterministic representation

Equivalent server responses must produce equivalent canonical graph output regardless of response arrival order or traversal concurrency.

### 4.5 Language neutrality

The core depends on LSP structures, not TypeScript, Roslyn, Elixir, or framework-specific ASTs. Server-specific accommodations must remain isolated configuration or adapters.

### 4.6 Honest frontiers

Unsupported methods, unresolved symbols, server failures, cycles, external symbols, timeouts, and limits are first-class output—not logs that disappear from the result.

## 5. Goals

The first release must:

- Ship as a portable Go binary.
- Launch a configurable stdio language-server process.
- Initialize and shut down an LSP session correctly.
- Open the target source document when required.
- Resolve one or more `CallHierarchyItem` values through `textDocument/prepareCallHierarchy`.
- Recursively call `callHierarchy/incomingCalls` for every newly discovered item.
- Preserve each item’s opaque `data` field when requesting expansion.
- Emit deterministic JSON containing nodes, edges, call-site ranges, terminals, diagnostics, capabilities, limits, and completeness status.
- Detect repeated nodes and terminate cycles safely.
- Never silently truncate.
- Prove interoperability with `typescript-language-server` and `csharp-ls` through executable integration tests or exact retained blockers.

## 6. Non-goals

The first release will not:

- Implement parsing, type checking, compiler semantics, or generic reference-to-call conversion.
- Identify external endpoints, application roots, workflows, features, or runtime entry points.
- Infer dependency-injection bindings, reflection, macro expansion, routes, or framework registrations beyond what the server reports.
- Guarantee whole-program completeness.
- Prove runtime execution, reachability, frequency, or production behavior.
- Traverse outgoing calls.
- Merge results from multiple language servers.
- Download, checkout, or verify Git objects.
- Persist a global symbol database.
- Depend on SCIP, Sourcegraph, an editor, or a hosted service.
- Provide a graphical UI.

## 7. Terminology

**Target:** The source position supplied by the user and the Call Hierarchy item prepared from it.

**Node:** A normalized representation of one server-returned `CallHierarchyItem`.

**Incoming call:** An LSP `CallHierarchyIncomingCall` whose `from` item is a caller of the expanded callee.

**Edge:** A stored caller-to-callee relation containing all `fromRanges` reported for that relation.

**Terminal:** A discovered node that the traversal cannot or should not expand further, with an explicit reason.

**Frontier:** The set of unexpanded or incompletely expanded nodes when the graph is incomplete.

**Complete graph:** A traversal where every discovered node was expanded successfully or reached a natural terminal, with no safety bound, timeout, cancellation, or unhandled error preventing expansion.

**Static relationship:** A relationship reported by the server; it is not evidence that the call executes at runtime.

## 8. CLI contract

### 8.1 Primary command

```text
lsp-trace incoming [flags]
```

Required inputs:

```text
--workspace PATH
--server COMMAND
--at PATH:LINE:COLUMN
```

Initial optional inputs:

```text
--server-arg VALUE        repeatable
--server-env KEY=VALUE    repeatable
--language-id VALUE
--max-depth N             default 100; 0 means unlimited
--max-nodes N             default 10000; 0 means unlimited
--timeout DURATION        default 5m; 0 means unlimited
--request-timeout DURATION
--concurrency N           default 1 in MVP
--output PATH             default stdout
--pretty                   pretty-print JSON
--log-level LEVEL
--trace-lsp PATH          retain redacted JSON-RPC transcript
```

`LINE` and `COLUMN` are one-based at the CLI boundary. The client converts them to zero-based LSP positions. Invalid or ambiguous positions must fail before traversal unless the server explicitly returns multiple prepared items, in which case all items are represented as separate targets.

### 8.2 Process behavior

- JSON is written to stdout or `--output`.
- Human diagnostics and progress are written to stderr.
- Successful complete traversal exits `0`.
- Successful but explicitly incomplete traversal exits `2`.
- Invocation, protocol, or unrecoverable server failure exits `1`.
- Cancellation exits `130` when initiated by an interrupt.
- A JSON result should still be emitted for post-initialization incomplete traversals when enough state exists to describe the failure.

### 8.3 Future compatibility

The command structure reserves space for future `outgoing`, `serve`, or `query` commands but makes no commitment to implement them.

## 9. JSON output contract

The result is a versioned object. Field names and enum values are stable within a schema major version.

```json
{
  "schema_version": "lsp-trace.graph.v1",
  "invocation": {
    "workspace_uri": "file:///workspace",
    "target": {
      "uri": "file:///workspace/path/file.ts",
      "line": 12,
      "column": 8
    },
    "server": {
      "command": "typescript-language-server",
      "arguments": ["--stdio"]
    },
    "limits": {
      "max_depth": 100,
      "max_nodes": 10000,
      "timeout_ms": 300000
    }
  },
  "capabilities": {
    "call_hierarchy_provider": true
  },
  "targets": ["node-id"],
  "nodes": [],
  "edges": [],
  "terminals": [],
  "frontier": [],
  "diagnostics": [],
  "summary": {
    "node_count": 0,
    "edge_count": 0,
    "terminal_count": 0,
    "cycle_count": 0,
    "complete": true,
    "truncated": false
  }
}
```

### 9.1 Node

Each node contains:

```text
id
name
kind
detail
uri
range
selection_range
data
```

`data` is retained losslessly as JSON when present. The public output may include it by default in v1 because omitting it would prevent replay and diagnosis. A later redaction option may replace it with a hash while preserving the original internally during execution.

The stable node ID is derived from canonicalized URI, range, selection range, symbol kind, name, and detail. Opaque `data` is excluded from identity because servers may use unstable transport tokens. Collisions must be detected and diagnosed rather than silently merged.

### 9.2 Edge

Each edge contains:

```text
caller_node_id
callee_node_id
call_sites
```

`call_sites` contains every canonicalized `fromRange` reported for the caller/callee pair. Duplicate edges are merged only when their normalized endpoints match; call-site ranges are unioned and sorted.

Edges are always stored in execution orientation:

```text
caller → callee
```

although discovery proceeds from callee toward callers.

### 9.3 Terminal and frontier reasons

Initial reason enum:

```text
NO_INCOMING_CALLS
PREPARE_RETURNED_NO_ITEM
INCOMING_RETURNED_NULL
EXTERNAL_URI
UNSUPPORTED_CALL_HIERARCHY
SERVER_ERROR
INVALID_SERVER_RESPONSE
REQUEST_TIMEOUT
GLOBAL_TIMEOUT
CANCELLED
MAX_DEPTH
MAX_NODES
NODE_ID_COLLISION
```

Cycles are graph structure rather than terminal roots. Nodes in cycles remain expandable once; incoming callers outside the cycle are still traversed.

### 9.4 Deterministic ordering

- Nodes sort by canonical URI, selection range, range, kind, name, detail, then ID.
- Edges sort by caller ID, callee ID, then call-site ranges.
- Targets, terminals, frontier entries, and diagnostics sort by stable keys.
- Maps with user-visible serialization use stable field order through structs.
- Timestamps and wall-clock durations are excluded from canonical graph content. Optional execution metadata must not affect graph equality.

## 10. Traversal algorithm

The MVP uses deterministic reverse breadth-first traversal.

1. Canonicalize workspace and target URIs.
2. Initialize the server and verify Call Hierarchy capability.
3. Open the target document if required by the server.
4. Call `textDocument/prepareCallHierarchy` at the requested position.
5. Insert every returned target item at depth zero.
6. For each depth, sort the pending nodes by stable node identity.
7. Call `callHierarchy/incomingCalls` for each not-yet-expanded node.
8. For each incoming result, normalize the caller, add caller-to-callee edge and call-site ranges, and enqueue an unseen caller at depth plus one.
9. Mark a node `NO_INCOMING_CALLS` when the server returns an empty list.
10. Continue until the queue is empty or an explicit bound prevents expansion.
11. Compute strongly connected components for cycle reporting after traversal.
12. Canonicalize and serialize the graph.

The MVP may execute requests sequentially to maximize determinism and simplify server compatibility. Later bounded concurrency is permitted only if canonical output and protocol safety remain unchanged.

A node is expanded at most once. Multiple paths reaching the same node add edges but do not trigger duplicate expansion.

## 11. LSP lifecycle and capability behavior

The client must:

- Spawn the configured server without a shell by default.
- Communicate over stdio using `Content-Length` framed JSON-RPC.
- Send `initialize` with workspace URI, process ID, client information, and Call Hierarchy client capabilities.
- Preserve the server’s initialize result and report whether `callHierarchyProvider` is absent, false, true, or options-shaped.
- Send `initialized` only after successful initialization.
- Read and tolerate server notifications and requests unrelated to the active operation.
- Reply correctly to required server-to-client requests or return MethodNotFound for unsupported requests where permitted.
- Send `textDocument/didOpen` with exact source text and configured or inferred language ID.
- Preserve `CallHierarchyItem.data` exactly when requesting incoming calls.
- Attempt `shutdown` and `exit` on normal completion.
- Kill the child process after a bounded grace period when shutdown fails.
- Capture stderr independently from JSON-RPC stdout.

An unsupported Call Hierarchy capability yields a structured incomplete result and exit `2`, not an invented references fallback.

## 12. Limits and resource control

Traversal safety is controlled by depth, node count, global timeout, per-request timeout, and cancellation.

When a limit triggers:

- Already observed nodes and edges remain in output.
- Every discovered unexpanded node is listed in `frontier` with the applicable reason.
- `summary.complete` is false.
- `summary.truncated` is true for depth/node limits and false for capability or server failures.
- The CLI exits `2` after emitting the result.

No default limit may silently behave as unlimited. Unlimited settings require explicit zero values.

## 13. Error taxonomy

### 13.1 Invocation errors

Malformed positions, missing workspace, unreadable target, invalid duration, missing server executable, and conflicting flags fail before server initialization.

### 13.2 Lifecycle errors

Spawn failure, malformed JSON-RPC framing, failed initialization, premature process exit, failed source read, and shutdown timeout are retained with phase and causal message.

### 13.3 Request errors

Each request error records method, node when applicable, JSON-RPC code, server message, and whether traversal continued. A failure expanding one branch should not erase successful branches unless the protocol stream becomes unusable.

### 13.4 Data-integrity errors

Invalid ranges, invalid URI, missing required item fields, and stable-ID collision produce explicit diagnostics. The tool must not repair malformed server data silently.

## 14. Architecture

Suggested package boundaries:

```text
cmd/lsp-trace          CLI assembly and exit policy
internal/jsonrpc       framed transport and request correlation
internal/lsp           minimal protocol structs and lifecycle client
internal/server        subprocess management and stderr capture
internal/traverse      reverse BFS and expansion state
internal/graph         normalization, identity, SCCs, ordering, schema
internal/source        URI, document text, and language-ID handling
internal/report        canonical JSON and human diagnostics
```

The project should define the minimum LSP types required rather than importing an editor framework. A focused maintained protocol package may be adopted if it demonstrably preserves `json.RawMessage` data, supports Call Hierarchy, and does not force unrelated client behavior.

Traversal depends on a narrow interface so it can be tested without a real server:

```text
PrepareCallHierarchy(document, position) → items
IncomingCalls(item) → incoming calls
```

The graph package has no subprocess or LSP transport dependencies.

## 15. Testing strategy

### 15.1 Unit tests

Test URI normalization, range conversion, item identity, collision detection, edge merging, call-site union, stable sorting, summary accounting, terminal classification, and SCC detection.

### 15.2 Fake LSP subprocess

Ship a deterministic fake server fixture that communicates over real stdio framing and can model:

- A linear chain.
- Branching callers.
- A diamond with one caller reached twice.
- Direct and multi-node recursion.
- Duplicate call-site ranges.
- Multiple prepared target items.
- Empty versus null incoming results.
- Unsupported capability.
- Delayed response and timeout.
- Branch-local JSON-RPC errors.
- Malformed responses.
- Premature server exit.
- Required preservation of opaque item data.
- Responses returned in varying orders.

Golden JSON results must remain byte-identical across randomized response ordering.

### 15.3 Integration tests

Initial optional integration suites:

**TypeScript:** Create a tiny pinned fixture with a known branching and recursive call graph. Run `typescript-language-server --stdio`, trace a leaf, and assert exact callers and call sites.

**C#:** Create an equivalent pinned fixture, run `csharp-ls`, and assert exact callers and call sites.

Integration tests must report server version and capability response. They may be skipped when the executable is unavailable, but release qualification requires retained passing runs for both initial servers.

**Elixir:** A later capability probe asks whether ElixirLS implements the required Call Hierarchy methods with useful caller identity and ranges. Absence or weakness does not alter the core or automatically authorize a references/compiler-tracer fallback.

### 15.4 Negative semantic test

Documentation and fixtures must demonstrate that a reported static edge is not necessarily executed. At least one fixture should contain a statically reported caller behind a branch that the test runtime does not take.

## 16. Security and trust boundaries

Language servers execute with the invoking user’s permissions and may execute project build logic, restore dependencies, read workspace files, access the network, or emit sensitive data. `lsp-trace` must state this clearly.

The tool must:

- Never invoke a shell unless the user explicitly requests shell mode in a future version.
- Pass server arguments as an argument vector.
- Avoid logging environment values by default.
- Treat server stderr and opaque item data as potentially sensitive.
- Make full LSP transcript retention opt-in.
- Write output files with normal user-only protections subject to platform umask.
- Avoid automatic network access of its own.
- Never execute repository hooks or Git commands.

Sandboxing the language server is a deployment concern and a possible future feature, not an MVP guarantee.

## 17. Observability

Human stderr diagnostics should identify lifecycle phase, active request method, node counts, and frontier cause without intermixing with JSON stdout. Log levels are `error`, `warn`, `info`, and `debug`.

Opt-in protocol tracing records sent and received JSON-RPC messages with deterministic sequence numbers. It excludes environment variables and supports future redaction. Protocol traces are diagnostic artifacts and are not embedded in canonical graph output.

## 18. Milestones

### M0 — Protocol proof

Build a disposable Go client that initializes `typescript-language-server`, prepares `apiURL()` in a controlled fixture, performs one incoming-call request, and preserves opaque item data. Exit criterion: exact callers and call sites are printed from actual LSP responses.

### M1 — Deterministic traversal core

Implement graph normalization, reverse BFS, terminals, limits, cycles, fake-server tests, and canonical JSON. Exit criterion: all fake-server scenarios pass and randomized response order yields byte-identical output.

### M2 — TypeScript integration

Package the CLI and pass the pinned TypeScript fixture end to end. Exit criterion: expected nodes, edges, ranges, and natural terminals match the fixture.

### M3 — C# integration

Add no language-specific core behavior; configure `csharp-ls` and pass the equivalent C# fixture. Exit criterion: the same schema and traversal semantics hold.

### M4 — Release hardening

Finalize documentation, exit codes, security guidance, cross-platform builds, license, reproducible release process, and schema compatibility policy. Exit criterion: release checklist and acceptance criteria pass on macOS and Linux.

### M5 — Elixir capability decision

Probe ElixirLS against a controlled fixture. Record supported methods, returned identities/ranges, and limitations. Decide separately whether a fallback adapter belongs in this project.

## 19. Risks

### 19.1 Inconsistent Call Hierarchy support

Servers may omit the capability, return incomplete graphs, require documents to be open, or use unstable data. Mitigation: capability-first behavior, opaque-data preservation, fake-server coverage, and server-qualified integration claims.

The claim that two initial servers are interoperable is false until both retained integration tests pass.

### 19.2 Unstable node identity

Names and detail strings may be insufficient or unstable. Mitigation: include canonical URI and ranges, detect collisions, retain raw items, and test repeatability. The identity design fails if unchanged server output produces different canonical IDs or distinct items merge silently.

### 19.3 Graph explosion

Common utility symbols may have large caller graphs. Mitigation: explicit node/depth/time bounds, sequential MVP traversal, deduplication, and visible frontier output. Completeness fails whenever a bound triggers.

### 19.4 Server side effects

Language servers may build or restore projects. Mitigation: explicit trust warning and user-owned workspace preparation. The tool cannot claim read-only analysis merely because its own client writes no project files.

### 19.5 Static/runtime confusion

Users may interpret the graph as runtime reachability. Mitigation: schema terminology, documentation, and negative semantic fixture. The product fails its evidence contract if output or docs describe an LSP edge as observed execution.

### 19.6 Pressure to add parsers and frameworks

Unsupported servers may invite embedded compiler logic. Mitigation: keep fallbacks outside MVP and require an explicit extension decision. The language-neutral claim becomes false if core traversal branches on source language.

## 20. Unknowns and resolution questions

### Server capability unknown

Specific unknown: Does `csharp-ls` implement `prepareCallHierarchy` and `incomingCalls` with exact call-site ranges for the intended fixture? Resolution question: What capability and response are observed from the installed server version? Owner milestone: M3.

### Protocol interpretation unknown

Specific unknown: Do servers require byte-for-byte preservation of `CallHierarchyItem.data`, or can serialization reorder object keys safely? Resolution question: Does a fake server rejecting altered data pass through the client unchanged? Owner milestone: M0/M1.

### Identity unknown

Specific unknown: Can two legitimate hierarchy items share URI, ranges, kind, name, and detail while differing only in opaque data? Resolution question: Do TypeScript and C# fixtures or captured responses produce such collisions? Owner milestone: M2/M3.

### Workspace readiness unknown

Specific unknown: Which servers require dependencies, project loading, or build output before Call Hierarchy works? Resolution question: What minimum reproducible setup is required for each integration fixture? Owner milestone: M2/M3.

### Scale unknown

Specific unknown: At what graph size does sequential traversal become impractical? Resolution question: What node count and request latency are observed on synthetic 1K and 10K node fake graphs? Owner milestone: M1/M4.

### Elixir support unknown

Specific unknown: Whether ElixirLS provides useful Call Hierarchy at all. Resolution question: Can a controlled leaf function be traced to all known callers with exact ranges? Owner milestone: M5.

## 21. Acceptance criteria

The MVP is accepted only when:

1. A clean checkout builds one Go binary with `go build ./...`.
2. `go test ./...` passes without requiring external language servers.
3. The fake-server suite covers linear, branching, diamond, cycle, null, error, timeout, truncation, and opaque-data cases.
4. Equivalent shuffled server responses produce byte-identical canonical JSON.
5. Every observed edge preserves caller, callee, and all reported call-site ranges.
6. Every discovered node is expanded, naturally terminal, or present in the explicit frontier.
7. Triggering max depth, max nodes, timeout, cancellation, or a branch error produces `complete: false` with the correct reason.
8. Unsupported Call Hierarchy produces structured output and exit `2`; it never falls back to generic references silently.
9. At least one retained TypeScript integration run returns the exact expected graph.
10. At least one retained C# integration run returns the exact expected graph.
11. Output and documentation identify relations as language-server-reported static calls, not runtime execution.
12. The CLI sends shutdown/exit on success and terminates an unresponsive child after its grace period.
13. Stdout remains parseable JSON while logs remain on stderr.
14. The release documentation states language-server trust and side-effect boundaries.
15. No SCIP, editor, hosted service, Git, parser, or language-specific compiler dependency exists in the core binary.

## 22. MVP cut line

MVP includes one-shot incoming traversal over stdio, source-position targeting, deterministic JSON, explicit bounds/frontiers, cycle representation, fake-server coverage, and proven TypeScript/C# interoperability.

MVP excludes outgoing traversal, daemon mode, editor extension, graph visualization, references fallback, AST enrichment, framework adapters, Git integration, persistent indexes, multi-server composition, and Elixir fallback implementation.

If either initial language server lacks usable Call Hierarchy, the MVP does not hide that fact by implementing a language-specific fallback. It reports the retained blocker and triggers a product decision: change the supported-server claim, select another LSP server, or authorize an adapter in a later version.

## 23. Release decision

Proceed from M0 to implementation only if an actual TypeScript language server returns at least one incoming call with a stable caller item and exact call-site range. Continue to a cross-language MVP only if C# demonstrates the same protocol shape without language-specific traversal logic.

The project succeeds by making the language server’s caller graph observable, deterministic, and honest about its boundaries—not by producing a perfect call graph.
