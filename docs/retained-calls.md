# Retained CALLS export

The frozen V1 export remains bounded A3. The additive FR20 V2 export is now
source-implemented in both CLI and MCP, but is not deployed qualification,
provider authentication, or downstream-analysis enablement.

## V1 (frozen bounded A3)

This additive offline family is **retained-calls**, version
`lsp-trace.retained-calls.v1`. It accepts only fully admitted
`graph-provenance/v1` envelopes containing managed single-at CALLS graph-v3.
It is not normative `relations.v1`, full FR6, authenticated semantic identity,
a new traversal, or acceptance evidence. Historical graph IDs and bytes are
unchanged. No source checkout, language server, network, or export timestamp is
needed.

## Commands

```sh
lsp-trace export-retained-calls envelope.json > retained.json
lsp-trace export-retained-calls --output selected.json envelope.json
lsp-trace schema get --family retained-calls --version v1
lsp-trace validate --family retained-calls --version v1 retained.json
lsp-trace verify --family retained-calls --version v1 selected.json
```

`PATH|-` accepts an envelope file or stdin. Exit 0 means export/publication
succeeded, not that the original traversal or dependency census was complete.
`--output` installs a no-replace CLI generation selector using the existing exact
byte receipt. `verify PATH` without the explicit family remains graph-v3-only.
Inspect and validate keep their existing meanings; export is a separate operation.

MCP `lsp_trace_v1_export_retained_calls` (alias
`lsp_trace_export_retained_calls`) takes `{"input":"<original envelope JSON>"}`.
Use inline JSON **text** for exact-byte custody through map-based clients; object
input remains admitted only by this frozen V1 tool and the transport reencodes it.
The canonical `lsp_trace_v2_export_retained_calls` tool accepts only an exact
Graph Provenance V2 JSON **string** plus optional `output_selector` and `detail`;
it has no alias or version field. The CLI and MCP V2 paths share the explicit V2
offline operation and return identical canonical bytes. No MCP input string is
interpreted as a server-side path.

`output_selector` uses the existing host publication root and immutable no-replace
publication. `detail: "compact"` requires a selector and keeps the full export in
the selected artifact. Schema retrieval and composed validation accept the new
family. The runtime has fourteen canonical tools: the unchanged historical
thirteen-tool manifest plus an additive export registration and separately
versioned export envelopes. Historical manifest/envelope schemas stay unchanged.

## Identity contract

All commitments are `sha256:` followed by lowercase hexadecimal SHA-256 of
UTF-8 domain, one zero byte, and payload. Domains:

- `lsp-trace.retained-calls.v1:input`: **exact** input envelope bytes, retained in
  base64 `input_bytes`. Outer whitespace and key order affect this digest.
- `lsp-trace.retained-calls.v1:context`: canonical decoded typed
  `graphprovenance.Evidence`, including **exact embedded `graph_bytes`** as base64.
- `lsp-trace.retained-calls.v1:occurrence`: canonical JSON array
  `[context_id, relation_id, caller_node_id, callee_node_id, caller_uri, range]`.

Canonical JSON uses Go `encoding/json` compact serialization: decoded object keys
sorted lexically, array order retained, standard JSON string escaping including
Go HTML escaping, and typed integer decimal serialization without float64
conversion. Nested retained raw JSON numbers preserve their JSON number lexemes;
opaque raw data is not interpreted. Optional typed fields follow the v1 field
contract. No pretty-print whitespace or original object-key ordering survives.
Range objects use `start` and `end`, each with `line` and `character` (zero-based).
Canonical source capture and binding array order is already enforced by input
admission. The context table has exactly one row; endpoints and groups retain
native graph order; occurrences follow edge/callsite order; group memberships
retain native membership-ID order; node memberships retain their filtered order.

Outer envelope reformatting changes only the exact input commitment. Changed
embedded graph bytes (including their whitespace), session, generation, retained
source observations, or other decoded context deterministically change context
and occurrence IDs. Historical relation IDs remain **group keys**, never new
semantic logical identities. Fixed independent SHA-256 vectors are executable in
`internal/retainedcalls/export_test.go` for input, context, and occurrence domains.

## Tables and scopes

- `contexts`: graph digest; envelope session/generation/supply-status/claim
  metadata; original bundle invocation, execution identity, tool/version claims,
  capabilities, status/diagnostics, slice and seed context. No new invocation.
- `endpoints`: retained native nodes, including historical IDs and opaque data.
- `groups`: historical relation/execution IDs and endpoint IDs, exact `/edges/i`
  pointer, original relation receipt/support, relation memberships, and outgoing
  slice participation. Support is contributed **once per group**.
- `occurrences`: exactly one row per distinct retained range, caller URI and
  original `/edges/i/call_sites/j` binding with its URI receipt joins. Repeated
  identical reports were merged upstream and cannot be counted here.
- `node_memberships`, `bindings`, `supply`, `captures`: original records at their
  actual scope, including non-source bindings and failed/absent-byte outcomes.
  A source binding is attribution, not proof of analyzed-source equivalence.

An edge with no retained callsite ranges remains a group with
`callsite_state: UNREPORTED`, an empty occurrence list and its original support.
No zero range is invented. An actually retained all-zero range is preserved as
one occurrence. Two retained ranges do not double a group's support.

`Reconstruct(Tables)` receives **no input envelope or graph bytes**. It rebuilds
edges, endpoints, historical execution/group IDs, sorted distinct ranges,
relation receipts/support totals, memberships and source-binding joins. Full
semantic validation independently extracts the native CALLS projection from the
admitted input and compares it with this reconstruction. It also compares the
entire canonical table projection, including bundle context and scoped source
records. Dropped, added, duplicate, reclassified, resealed or substituted rows
cannot pass by merely retaining valid foreign keys or resealing the input digest.

## Admission and ceilings

Closed v1 input and recursively duplicate/case/unknown-field rejection compose
with full graph-provenance and graph-v3 admission. Lower schema-only validation
fails closed: JSON shape is not semantic admission. Limits: 48 MiB original
input, 8 MiB embedded graph, 128 MiB export, 50,000 group/occurrence/endpoint rows;
all existing source-read, census and source-byte limits remain in force. The
existing MCP transport has a 4 MiB request-line cap and 1 MiB inline-output cap;
publication does not bypass the request cap. These are per-operation bounds,
not a global memory budget or unlimited transport promise.

Per-callsite incoming/outgoing acquisition method, actual request/response IDs,
independent numeric support, provider qualification, authenticated provider
version, analyzed-source authentication and repeated-identical-report counts
are explicitly `UNAVAILABLE`. Original tool/server version assertions remain
bundle-scoped; no unavailable identifier is synthesized. `IncomingRequestSuccesses`
is a **bundle-derived metric**, not actual per-request evidence. LSP supply
means didOpen/full-text didChange, **not call acquisition method**.

Analyzed version stays `ANALYZED_VERSION_UNVERIFIED`; dependency completeness
stays `UNKNOWN_INCOMPLETE`. Coherently forged public input and tables can pass
consistency checks: neither export, hashes, reconstruction nor publication
independently reauthenticates a producer, source read/write, or server analysis.
The source-bearing input and source bytes remain sensitive; no automatic
redaction or whole-source completeness is claimed.

## Qualification

Hermetic tests separately exercise fake-wire repeated reports and empty ranges,
merged-range ceilings, tables-only replay after source deletion, coherent table
mutation rejection, absent source bytes, context changes, exact-byte versus
canonical identity, and strict/budget admission.

```sh
LSP_TRACE_GRAPH_PROVENANCE_GOPLS=/absolute/installed/gopls \
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  go test ./cmd/lsp-trace-mcp \
  -run '^TestGraphProvenanceRealGoplsCLIAndMCP$' -count=1 -timeout=180s
```

The real-server fixture still requires five distinct non-seed files. It now
contains two callsites for each of five groups, deletes sources, then tests
export, CLI/MCP byte parity, schema retrieval, full validation, selected CLI
verification, overwrite rejection and MCP publication. Fake-wire findings are
not real-provider qualification. This bounded increment awaits independent
review and does not self-approve Program A/B, AC1, FR6, or product acceptance.

## FR20 internal V2 API

```go
func ExportV2(input []byte) ([]byte, error)
func ReconstructV2(t TablesV2) (ProjectionV2, error)
func ValidateFor(raw []byte, family, version string) (string, error)
```

`ExportV2` accepts only fully admitted `graph-provenance/v2`, through its exact
`ValidateFor` dispatch. `retained-calls/v2` (or the full version
`lsp-trace.retained-calls.v2`) selects composed artifact validation. No omitted
version, V1 envelope, or unrelated family silently selects V2. Generic schema
retrieval exposes the additive schema; core schema-only validation still fails
closed because shape alone cannot establish semantic admission. Public V1 export
and downstream analysis continue to request V1 explicitly. Public source now
also exposes `lsp_trace_v2_export_retained_calls`; this is implemented, not a
claim of deployed availability or qualification.

### Tables and reconstruction

`TablesV2` contains no original envelope or embedded native graph bytes:

- `context`: exact input commitment, policy-bound context ID, native graph digest,
  host workspace URI, and capture budget.
- `endpoints`: native nodes and opaque payloads, retaining original native IDs.
- `groups`: native relation ID, `/edges/i` pointer, execution ID, caller/callee,
  separately named export ID, native receipt/support and occurrence IDs.
- `occurrences`: export ID and group ID, original `/edges/i/call_sites/j` pointer,
  explicit `/graph/edges/i/call_sites/j` binding pointer, and distinct range.
- `native_fields`: one explicitly named/value row for **every** native top-level
  field other than nodes, edges, receipt and memberships. This preserves seeds,
  invocation, diagnostics, frontiers, portable/replay locators, semantic receipt,
  quality, summary and any slice/non-CALLS provenance fields without a hidden
  graph fallback. Current coordinator replay rejects invented sibling/dispatch
  relations even though graph-v3 can represent them; this exporter does not widen
  that admission boundary.
- `memberships`, `receipt_present`, `support_total`: original native accounting.
  Support is once per native group, not per site, alias, cached request replay or
  acquisition observation. No independent-support claim is added.
- `acquisition`: complete typed request/limits/context, every target and its
  resolution/admission/outgoing/incoming/connection, all actual request records,
  expansions/layers/frontiers, observations, supplies, usage and completeness.
- `connections`: a row for **every** target, preserving all native path nodes
  (including intermediates), and separately named exported group/occurrence IDs.
  Original coordinator witnesses remain in the acquisition target records.
- `bindings`, `supplies`, `captures`: every admitted census and source row,
  including exact content/canonical-receipt bytes, failed outcomes, invalid
  anchors and separate same-URI document versions.

Empty exported arrays are `[]`, including zero-site occurrence lists and typed
acquisition collections. `native_null_arrays` and `acquisition_null_arrays`
explicitly preserve historical native empty allocation distinctions for exact
reconstruction. Optional omitted wire fields stay omitted; opaque JSON and
source bytes are not normalized as typed arrays. Markers cannot refer to a
nonempty collection, unknown pointer or duplicate path.

`ReconstructV2` builds edges from groups and occurrences, joins endpoints and
bindings, assembles the native object from explicit fields, re-admits graph-v3,
and reconstructs the full coordinator result. It checks native-to-exported
witness joins independently of the mapping producer, invokes coordinator replay,
and recomputes the mandatory source census and capture/supply joins through V2
provenance admission. No filesystem or server is consulted. Invalid anchors stay
`INVALID_COORDINATES`; a readable file never upgrades them.

Artifact validation admits the original input, re-derives all required tables,
and separately compares tables-only reconstruction against the original native
serialization, coordinator descriptor and full source/census records. Export
also performs this independent conservation check before returning. Shared
producer loss of intermediate nodes is rejected even when the producer is reused
for expected-table derivation. Coherently rehashed missing rows or substituted
ranges, statuses, owners, supplies and mappings do not become admissible merely
because public commitments are internally consistent.

### V2 identity preimages

Using the same `D(domain, payload)` convention as V1:

- Input: `D("lsp-trace.retained-calls.v2:input", originalEnvelopeBytes)`.
- Context: `D("lsp-trace.retained-calls.v2:context", canonical([policy,inputDigest]))`.
- Group: `D("lsp-trace.retained-calls.v2:group", canonical([contextID,
  nativeRelationID,nativePointer,executionBundleID,callerNodeID,calleeNodeID]))`.
- Occurrence: `D("lsp-trace.retained-calls.v2:occurrence", canonical([contextID,
  exportGroupID,nativePointer,bindingPointer,range]))`.

Policy is `EXACT_ENVELOPE_NATIVE_GROUP_DISTINCT_SITE_TYPED_ACQUISITION_V2`.
Canonical preimages sort object keys, retain array order and JSON number lexemes,
use Go JSON escaping, and never convert numbers through float64. Native graph
reconstruction instead retains native typed serialization, including opaque raw
JSON ordering required by native semantic commitments. Original envelope
whitespace changes **V2 context and export IDs**, unlike V1. No export identity
enters the original native PathInput witness or graph/envelope digest, so there
is no digest cycle. Identities establish integrity, not producer authentication.

### Independent resource contracts

| Boundary | Limit |
| --- | ---: |
| Admitted provenance V2 input | 192 MiB (upstream graph allowance 32 MiB) |
| Native graph accepted by this export | 8 MiB |
| Export endpoints / native groups | 4,096 / 8,192 |
| Occurrences / bindings | 50,000 each |
| Export JSON including trailing newline | 384 MiB |
| Interpreted JSON carrier depth | 64 |
| Known numeric token length / exponent magnitude | 64 / 1,024 |

The coordinator's declared 10,000-node limit is preserved, not silently changed
to 4,096. An actual result exceeding export limits is wholly rejected; it is not
truncated. `LimitErrorV2` identifies export byte/node/group resource rejection.
Upstream provenance budgets and their errors continue to apply independently.
Known base64 JSON carriers are preflighted before recursive validation; source
content and opaque `data` remain opaque. These are per-artifact limits, not a
peak-memory guarantee. Future analysis has its own 8 MiB/4,096-node/8,192-group
contract and is **not** enabled by this export API.

### Qualification boundary

Persisted tests cover frozen V1 bytes/rejection, source-free connected and
incoming multi-hop paths, disconnected targets, zero-hop aliases, missing and
ambiguous roots, unadmitted/partial targets, empty and zero-site groups, exact
numbers, same-URI versions, all-row deletion, rehashed substitutions, depth and
resource boundaries, and a controlled shared-producer reduction. This is a
hermetic fake-Go-provider qualification, not real-public-gopls qualification,
complete FR20, FR21 source-context/span selection, normative `relations.v1`,
source authentication, or independent numeric support. Existing authentication
and completeness ceilings continue unchanged. V2 export is source-implemented
for CLI and MCP; deployment qualification and downstream version adapters remain
subsequent stages.
