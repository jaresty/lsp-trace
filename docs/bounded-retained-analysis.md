# Authorized bounded retained analysis

## Decision and baseline

The user explicitly authorized a separate, versioned bounded retained-CALLS
analysis contract on the clean baseline
`2bcbec2fdbff0f81dc4a3277685390732f1e5791`, on the active
`feature/normalized-relations-provider` checkout. This decision permits directed
projection, bounded shortest paths, and explicitly weak/strong components before
the normative Program B gate. It is **not** satisfaction or waiver of
`PROGRAM_B_ADMITTED`, and is not normative `projection.v1`. No PRD, requirement
matrix, historical manifest/envelope, or admission flag is changed by this decision.
Authorization was recorded before implementation and executable public-route RED.

## Authority boundary

Only fully validated `retained-calls/v1` is eligible. Its provenance remains
unverified and incomplete. All node, execution and group IDs remain historical
artifact-scoped keys, not semantic identities. A coherent public forgery does not
acquire authentication through analysis. No dispatch synthesis, hidden endpoints,
source acquisition, ownership/business interpretation, community detection,
ranking, product access or normative acceptance is involved.

The result embeds **one** exact retained input as base64 `input_bytes`. This is
not a fresh capture: it retains the admitted contexts, endpoints, groups,
occurrences, membership, binding, supply and capture tables and the original
provenance bytes. Context bundles preserve frontier, terminal, traversal,
diagnostic and scope-limit references. Offline validators resolve witnesses using
these retained tables without opening a source checkout. Source receipts and
ceilings are not upgraded. `scope` is always
`HISTORICAL_ARTIFACT_SCOPED_UNVERIFIED_INCOMPLETE`.

## Public contract

Family: `bounded-retained-analysis`, version: `v1`.
Artifact identity: `lsp-trace.bounded-retained-analysis.v1`.
Fixed projection policy: `retained-CALLS-unit-group/v1`.

```sh
lsp-trace export-retained-calls provenance.json > retained.json
lsp-trace bounded-retained-analysis --operation PROJECT retained.json > project.json
lsp-trace bounded-retained-analysis --operation PATH \
  --start EXACT_CALLER_NODE_ID --end EXACT_CALLEE_NODE_ID retained.json > path.json
lsp-trace bounded-retained-analysis --operation COMPONENTS \
  --mode WEAK retained.json > weak.json
lsp-trace bounded-retained-analysis --operation COMPONENTS \
  --mode STRONG --max-work 100000 retained.json > strong.json

lsp-trace schema get --family bounded-retained-analysis --version v1
lsp-trace validate --family bounded-retained-analysis --version v1 path.json
lsp-trace bounded-retained-analysis --operation PROJECT \
  --output selected.json retained.json
lsp-trace verify --family bounded-retained-analysis --version v1 selected.json
```

`PATH|-` is the retained input file or stdin; it is never a source checkout.
Flags precede the input path. Node IDs come from `nodes` in PROJECT or the retained
endpoint table. PROJECT forbids start/end/mode. PATH requires both exact IDs and
forbids mode. COMPONENTS forbids start/end and requires explicit WEAK or STRONG;
there is no silently selected component mode. Historical `validate` and `verify`
defaults are unchanged. `--output` publishes an immutable generation selector
without replacement; it is distinct from redirecting stdout to an artifact file.

MCP adds `lsp_trace_v1_bounded_retained_analysis` (alias
`lsp_trace_bounded_retained_analysis`) through additive registration, not by
rewriting the historical thirteen-tool manifest. Together with retained export,
there are now fifteen runtime tools. Example arguments:

```json
{"input":"<exact retained-calls JSON text>","operation":"PROJECT"}
{"input":"<exact retained-calls JSON text>","operation":"PATH","start":"<node ID>","end":"<node ID>","max_work":100000}
{"input":"<exact retained-calls JSON text>","operation":"COMPONENTS","mode":"STRONG","output_selector":"components.json","detail":"compact"}
```

The shared offline handler is used by both public transports. `input` may also be
a JSON object; for this new operation and its family-explicit MCP validation, the
server preserves the exact object-value bytes before generic map decoding can
round nested numbers. Input text is JSON, **never a path**. Prefer text when the
client needs exact original formatting: a client serializing an object may already
have changed its bytes before transmission. MCP schema_get/validate use
`{"schema":{"family":"bounded-retained-analysis","version":"v1"}}`, with
`input` added for validate. Validation returns the exact input bytes unchanged.

MCP `output_selector` requires a configured publication root. Results exceeding
1 MiB require it; compact mode also requires it. Publication and compact envelopes
are separately versioned and retain immutable artifact receipts. MCP verify
continues its existing publication-custody contract; family-explicit semantic
validation is a separate validate call. Analytical artifact bytes match CLI bytes
for identical inputs/parameters; transport envelopes may contain request IDs,
durations and publication metadata that are **not** analytical identity.

## Projection and algorithms

`nodes` is the lexical list of all endpoint IDs, including isolates. `edges` is
sorted by historical group ID and has one caller-to-callee directed edge for
every retained group, including parallel groups and self-loops. Every weight is
the integer `1` **per group**, not per callsite, range or support count. Each edge
carries context/execution/group IDs, callsite state, and the original ordered
occurrence-ID witnesses. `UNREPORTED` remains a real edge with an empty witness
list; it does not imply an invented zero-range callsite.

PATH uses BFS. Queue expansion order is lexical `(next node ID, group ID)`;
first discovery wins, giving the lexically first sequence of these pairs among
shortest paths. Missing IDs are rejected (including a missing equal pair);
a present `start == end` is a valid zero-hop path. FOUND has ordered nodes,
group IDs and a parallel ordered array of each edge's occurrence witnesses.
`NOT_FOUND_IN_RETAINED_GRAPH` means exhaustive search of this admitted bounded
graph only, never absence from source, runtime or a complete dependency universe.

WEAK ignores direction **only for membership**. STRONG uses iterative Kosaraju
DFS and transpose traversal, without recursion. Both retain original edge
directions and witnesses in the result. Members are lexical; components are
ordered by their first member. Component IDs bind the exact basis, explicit mode
and sorted membership. Empty graphs have no components. Isolates remain singleton
components and self-loops require no special identity or inferred semantics.

## Bounds and incomplete results

- Retained input: at most 8 MiB; analytical artifact: at most 16 MiB.
- JSON nesting: at most 64 before recursive duplicate/schema validation.
- Admitted topology: at most 4,096 nodes and 8,192 groups; existing retained-call
  row and provenance validation limits still apply.
- `max_work`: integer 1..1,000,000, default 1,000,000. The Go constructor treats
  omitted/zero MaxWork as default; public explicit zero is rejected.
- Existing MCP framing remains at most 4 MiB per wire message. Escaping/base64
  expansion means this transport can accept less than the offline byte ceiling.

These are fixed v1 ceilings, not configurable architecture. Byte and nesting
preflight bounds precede schema/tree decoding; admitted topology bounds precede
adjacency allocation. Validation and canonicalization are additionally bounded by
these fixed sizes; `max_work` counts analytical traversal steps, not JSON parsing,
validation CPU time or wall-clock time. PROJECT ticks per node and group; BFS per
dequeued node and inspected edge; components per root, DFS frame step, queued
node and inspected membership edge. The default covers every admitted topology.

Traversal exhaustion emits `status: INCOMPLETE`, `reason: LIMIT`; observed
cancellation emits `INCOMPLETE/CANCELLED`. Both retain the full projection/basis
but empty path and component results, never partial partitions or not-found.
Pre-admission byte, nesting, topology or parameter failures are explicit input
errors mentioning the limit, not graph reachability conclusions. Cancellation is
observed at bounded algorithm steps; retained-input admission itself is offline,
size-bounded and not an interruptible source operation. Empty graphs also honor
pre-existing cancellation. CLI artifact production exits successfully for a valid
INCOMPLETE result; consumers must inspect analytical `status`, not infer graph
completeness from a successful transport envelope or exit status.

## Identity and semantic validation

Canonical JSON uses sorted object keys, original array order, Go JSON escaping,
and `json.Number` preservation; nested integers are not converted to float64.
SHA-256 digests have `sha256:` followed by lowercase hex. Every domain is UTF-8
followed by one NUL before canonical JSON:

- Basis domain `lsp-trace.bounded-retained-analysis.v1:basis`: the object
  `{policy, input_bytes, parameters}`; input bytes use JSON base64 representation.
- Result domain `lsp-trace.bounded-retained-analysis.v1:result`: the entire result
  with `digest` set to the empty string.
- Component domain `lsp-trace.bounded-retained-analysis.v1:component`: the array
  `[basis_digest, mode, sorted_members]`.

No timestamp or fresh request identity is added to analytical results. Original
retained metadata remains part of the basis. Whitespace changes to retained input
change the basis; authenticated identity is never inferred from that digest.
Changing public provenance metadata can change context IDs while leaving
historical group IDs unchanged; both remain unverified.

The composed validator checks recursive duplicates and exact member spellings,
closed structural schema, retainedcalls.ValidateFor and tables-only Reconstruct,
then regenerates the fixed projection. Independent proof checks use reverse
shortest distances plus greedy lexical choice for paths, and per-block directed
connectivity plus quotient acyclicity for strong components (connected blocks and
no inter-block edges for weak components). This is not just hash checking or an
algorithm claiming its own correctness. Deterministic producer replay additionally
checks canonical ordering, component IDs and exact resource-limit outcomes.
CANCELLED can only be validated as an honest absence of an analytical conclusion;
it is not authenticated evidence that a runtime cancellation event occurred.

## Verification coverage and remaining limits

Tests cover all 512 directed three-node topologies against an independent
Floyd-Warshall oracle, plus diamond tie-breaking, parallel groups, empty graphs,
isolates, self-loops, two-site and unreported groups, direction reversal, missing
IDs, limits/cancel, fixed independent digest vectors and coherent-reseal mutations.
Real CLI/MCP tests check exact bytes, nested large-integer object inputs, schema and
semantic validation, immutable publication, compact/error variants and oversized
results. The existing installed-gopls six-file/five-group/ten-call fixture now runs
export -> projection -> path -> weak/strong components **after source deletion**.

These tests qualify bounded retained-input behavior only. Native Ember qualification,
source authentication, semantic identity and normative Program B admission remain
outside this contract. This implementation awaits parent and independent review;
it does not self-approve or deploy. Execution commands/results are retained in
`/tmp/lsp-trace-bounded-analysis-2bcbec2/CLAIM.md` for this work session.
