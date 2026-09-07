# Authorized bounded retained structural metrics

## Decision and baseline

The user explicitly authorized autonomous structural METRICS on baseline
`8c74fc3b3ca748e539306ff0acf1e886c9a693d6`, in the active
`feature/normalized-relations-provider` checkout, following independently passed
bounded topology and encoded-carrier preflight work. This is a new family:
`bounded-retained-metrics/v1`, artifact `lsp-trace.bounded-retained-metrics.v1`.
The existing bounded-retained-analysis/v1 enum, policy and artifact formulas are
unchanged. No PRD, historical manifest, source authority, or PROGRAM_B_ADMITTED
flag changes. PageRank/PPR is a separate assignment after metrics review.

## Authority and input

Only fully admitted retained-calls/v1 is accepted, using the same historical
caller-to-callee, unit-per-group policy and all retained endpoints, including
isolates. UNREPORTED is still an edge with zero witnesses. Parallel historical
groups remain separate. Context, provenance, source, support and authentication
ceilings are inherited unchanged through **one exact embedded** base64
`input_bytes`. There are no per-edge source copies. Node row IDs resolve the
underlying retained endpoint table; IDs remain artifact-scoped, not semantic IDs.
`scope` is `HISTORICAL_ARTIFACT_SCOPED_UNVERIFIED_INCOMPLETE`.

`status: COMPUTED_OVER_RETAINED_GRAPH` means the entire admitted retained graph
was measured, **not** that retained evidence is source-complete, authenticated,
or a complete dependency universe. Coherently changing an entire public input
can produce another admissible historical input, not authentication.

## Fixed v1 formulas

Policy: `retained-CALLS-unit-group-structural-metrics/v1`. Parameters: exactly `{}`.
There are no optional algorithm modes, weights, work knobs or decimal encodings.

Global fields:

- `node_count = n`: all retained endpoints.
- `group_count = m`: all retained groups.
- `reported_occurrence_count`: sum of group occurrence-ID lengths, independently
  crosschecked against the uniquely joined retained occurrence table. This is
  **not** total source sites or support_total.
- `unreported_group_count`: groups with UNREPORTED callsite state.
- `self_loop_group_count`: groups whose caller equals callee.
- `distinct_nonloop_pair_count = q`: unique ordered caller/callee pairs excluding
  equal endpoints; parallel groups collapse only for this counter.

`nodes` is lexical by exact ID. Each row has `id`, `in_group_degree`,
`out_group_degree`, `in_distinct_neighbors`, `out_distinct_neighbors`.
A loop contributes one to each degree and its own ID once to each neighbor set.
Parallel groups increase degrees but do not increase neighbor cardinality.
Isolates have four zeros. Empty graphs have an empty node list.

`in_degree_histogram` and `out_degree_histogram` are arrays of
`{degree,node_count}`, sorted by increasing degree. Every observed degree,
including zero, has one positive-count bin; absent degrees have no bin. Empty
graphs have empty histograms. Each histogram's counts sum to n and its
sum of degree times count equals m. Each direction's node degrees sum to m.

`density.policy` is `DIRECTED_DISTINCT_NONLOOP_PAIRS`. For n >= 2,
`density.status` is `DEFINED` and `density.value` is the exact, **unreduced**
rational `{numerator:q,denominator:n*(n-1)}`. No float/rounded decimal is emitted.
For n < 2, status is `UNDEFINED_DENOMINATOR` and value is JSON null, never an
invented zero. A two-node edgeless graph has a defined value 0/2.

No component counts, weighted degrees, all-pairs analysis, betweenness, or ranking
are included.

## Identity and independent validation

Canonical JSON sorts object keys and retains array order, uses Go JSON string
escaping and exact json.Number decoding (not float64). Output is compact JSON
plus one LF; digest canonicalization itself has no trailing LF. Hashes use
`sha256:` plus lowercase hex over UTF-8 domain, one NUL, then canonical JSON.

- Basis domain `lsp-trace.bounded-retained-metrics.v1:basis`: object
  `{policy,input_bytes,parameters}`. Input bytes have JSON base64 representation.
- Result domain `lsp-trace.bounded-retained-metrics.v1:result`: all result fields,
  with `digest` set to the empty string.

Exact original input whitespace changes identity. Input is embedded once;
source timestamps already in that input remain part of identity, but metrics add
no timestamp, request identity or runtime duration. Map iteration cannot affect
output order. Permuting table traversal with identical basis bytes gives identical
metric bytes; permuting actual input bytes intentionally changes the basis.

The composed validator first bounds the outer artifact and known decoded
retained/provenance/graph carriers, then checks closed schema and exact recursive
field spellings, historical retained admission, and tables-only reconstruction.
The independent native proof sorts incidence lists into runs rather than using
the producer's map-based node accumulators. It crosschecks every endpoint ID,
degree, neighbor cardinality, histogram, loop/unreported counter, occurrence
table cardinality and ordered-pair density. Deterministic replay then checks
policy, basis, digest, ordering, parameters and completion status. Coherently
resealed counter, ID, missing-isolate, ceiling and policy mutations are rejected.
The lower schema-only semantic route deliberately fails closed and directs callers
to `boundedmetrics.ValidateFor`; structural shape alone is not admission.

Persistent tests include an independent three-node adjacency-matrix oracle for
all 512 directed topologies, empty/isolate/self-loop/parallel/two-site/unreported/
dense specimens, a stdlib Python full-artifact oracle, fixed Python/Go basis and
result vectors, and coherent-reseal mutations. Python is optional for ordinary Go
tests; fixed independent vectors always run. This session ran the Python oracle.

## Public routes

```sh
lsp-trace bounded-retained-metrics retained.json > metrics.json
lsp-trace bounded-retained-metrics --output metrics-selector.json retained.json
lsp-trace schema get --family bounded-retained-metrics --version v1
lsp-trace validate --family bounded-retained-metrics --version v1 metrics.json
lsp-trace verify --family bounded-retained-metrics --version v1 metrics-selector.json
```

Input is a retained artifact file or `-` for stdin. Flags precede the path.
`--output` publishes an immutable generation selector without replacement.
Default verify/validate behavior remains graph-only; family selection is explicit.

MCP adds `lsp_trace_v1_bounded_retained_metrics`, alias
`lsp_trace_bounded_retained_metrics`. The shared offline handler serves CLI and
MCP. Historical thirteen-tool manifest and existing fifteen tool contracts are
preserved; runtime discovery now has sixteen tools. Metrics input and all five
envelope variants have distinct additive schemas.

```json
{"input":"<exact retained JSON text>"}
{"input":"<exact retained JSON text>","output_selector":"metrics.json","detail":"compact"}
```

Input can also be a JSON object; server-side decoding preserves exact raw object
value bytes and large integers before generic map decoding. Text is JSON, never
a filesystem path. Client object serialization can already have changed bytes;
use text for original formatting. Schema get/validate use
`{"schema":{"family":"bounded-retained-metrics","version":"v1"}}`, with
`input` added to validate. Validation returns exact admitted bytes unchanged.
MCP verify retains its existing graph-only custody/semantic contract; it does
not add metrics verification. Use family-explicit validate for metrics semantics
and CLI family-explicit verify for a metrics generation selector.

Output over 1 MiB or compact detail requires `output_selector` and a configured
publication root. Unsafe selectors and overwrites fail closed. Compact/publication
receipts preserve the full artifact; transport success and summary are not
source-completeness claims. CLI and MCP artifact bytes match for exact same input.

## Resource and cancellation policy

- Input: 8 MiB maximum; artifact: 16 MiB maximum.
- Topology: 4,096 endpoints and 8,192 groups. Existing retained-call table limits
  also apply (including 50,000 occurrences).
- Known JSON carriers: nesting maximum 64 **before** recursive duplicate/schema
  processing, including base64 decoded retained, provenance and graph values.
  Carrier allocation is bounded; duplicate/case-folded carrier assignments cannot
  hide malformed earlier values. Source content/opaque receipts are not JSON
  carriers and remain byte-identical, even if they resemble deeply nested JSON.
- MCP retains the existing 4 MiB per-wire-message ceiling; encoding expansion can
  make the accepted wire payload smaller than the offline input maximum.

Producer metric work is O(n log n + m), with O(n+m) topology storage; proof uses
O(n log n + m log m) sorting and O(n+m) storage. JSON admission/canonicalization is
bounded separately by byte, carrier-depth and table limits. There is no tunable
work budget or partial-result status. Limits and cancellation return errors and
**no metric artifact**. Cancellation is checked before admission, during bounded
node/group work and before delivery; size-bounded historical admission, sorting,
canonicalization and validation are not wall-clock interruptible source work.
An empty graph honors pre-existing cancellation too.

## Derivation

The supplied user contract fixes formulas and authority boundaries; implementation
is deductive rather than a new discovery or approval exercise. Compiling public
RED tests preceded production changes and failed specifically for the missing
family, canonical/alias tool, and executable metric route. GREEN is checked through
the same routes, independently accumulated table and matrix proofs, Python hash
vectors, source-deleted installed-gopls chain, immutable publication, malformed
encoded carriers and adversarial coherent reseals. Existing historical fixtures
and semantic guards remain meaningful; only intended runtime cardinality/order
expectations advance from fifteen to sixteen.

Execution logs and exact command outcomes are recorded in
`/tmp/lsp-trace-bounded-metrics-8c74fc3/CLAIM.md`. This is review-ready implementation
evidence, not self-approval, deployment, normative Program B admission or ranking
qualification.
