# Retained CALLS export (bounded A3)

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
input is also admitted but the transport reencodes that object. The CLI and MCP
use the same offline handler and return identical export bytes for identical
input bytes. No input string is interpreted as a path by MCP.

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
