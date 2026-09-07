# Bounded graph provenance

This opt-in increment records **honest provenance, not authenticated analyzed
source**. It does not adopt the PRD's normative source snapshot policy or complete
AC1. It makes no filesystem freeze, server-consumption, dependency-completeness,
compiler-input-census, or whole-source claim. Independently approved byte custody
remains distinct from whether a server interpreted those bytes.

## CLI and MCP

```sh
lsp-trace slice --workspace /absolute/trusted/workspace \
  --server /absolute/path/to/gopls --at f0.go:2:6 \
  --graph-provenance --down-depth 5 --up-depth 1 \
  --timeout 60s --request-timeout 10s
```

The additive CLI route uses the existing local Darwin managed supervisor and the
**same `sliceops` operation as MCP**, not a second provenance implementation.
It requires exactly one `--at`; `--from-file`, `--seed-file`, and `--trace-lsp` are
unsupported in this mode. Managed bounds are depth 1..64, nodes 1..10000, and
1ms..60s timeouts. Defaults in this mode are downward/upward depth 2, 100 nodes,
5s operation timeout and 1s request timeout. Coordinates remain one-based on CLI,
zero-based in MCP. `--output` and `--pretty` change the outer presentation, never
the embedded graph bytes. Exit 0 means a bounded envelope was published, not that
traversal is complete; inspect the embedded graph summary. Other platforms fail
closed for this CLI supervisor; cross-build success is not runtime qualification.

For an already host-provisioned MCP session, call `lsp_trace_v1_slice`:

```json
{
  "session_id": "host-session-or-alias", "generation": 1,
  "start_mode": "at", "uri": "file:///absolute/trusted/workspace/f0.go",
  "line": 1, "character": 5, "down_depth": 5, "up_depth": 1,
  "timeout_ms": 60000, "request_timeout_ms": 10000,
  "graph_provenance": true
}
```

`symbol` may replace line+character in the managed operation. Omit `relations`,
provider/adapter selectors, languages/frameworks and revision options: relation
composition is explicitly unsupported with this mode. Source scope comes from
the host-owned session profile, never a new caller-supplied filesystem root.
The mode's JSON field names are exact/case-sensitive, and duplicate members reject
before decoding. Omitted mode preserves the existing CLI/MCP graph behavior and
bytes; a cached prepare does not cause a forced notification.

CLI creates a fresh session using identity labels `trust_domain=graph-provenance`,
`profile=cli`, `environment_reference=cli`. Matching those host bootstrap identity
labels and operation parameters permits exact CLI/MCP envelope replay; other
session identities legitimately change the envelope. The old unmanaged CLI has
its own invocation fields and is not advertised as byte-identical to this new
managed route. Language servers remain trusted local processes, not sandboxed.

## Evidence and mandatory census

Family `graph-provenance`, version `lsp-trace.graph-provenance.v1`, stores:

- `graph_bytes`: base64 of **exact unchanged legacy graph-v3 bytes**, including
  whitespace; `graph_digest` commits to those bytes with domain
  `lsp-trace.graph-provenance.v1:graph`, a zero byte, then the bytes (SHA-256).
- Separate `supply` and `captures` records; the same URI can carry different bytes.
- `bindings`: JSON pointers into the decoded graph with source attribution and
  receipt foreign keys. The verifier derives these pointers from graph records,
  not from a caller-provided list of expected binding IDs.
- Fixed `ANALYZED_VERSION_UNVERIFIED` and `UNKNOWN_INCOMPLETE` claim ceilings.

The census includes nodes and their ranges, edge endpoints and caller-owned
callsites, attributable diagnostics, invocation/seed positions and resolved URIs,
node-reference arrays, slice records and redundant native node locators. Opaque
LSP `data` is not interpreted as source references. Diagnostics without a known
node attribution are explicitly `NON_SOURCE` with no receipt keys; message text
is never parsed for paths. This version accepts managed single-at CALLS slices,
not sibling/dispatch discovery or relation compositions.

`LSP_SUPPLIED` means the actual successful didOpen/full-text didChange **write**
supplied the retained text. Runtime `DocumentRequest.CaptureSupply` returns owned
content from the same read used for that notification, exact JSON parameters,
session generation and document version. Invalid UTF-8 rejects rather than
silently accepting JSON replacement characters. Failed writes have no successful
supply observation. Cached/unchanged documents return no new supply, represented
by `NO_NOTIFICATION_OBSERVATION`; this does not assert the server never received
that document. The manager retains no new source buffers; returned buffers belong
to the requesting operation. They are historical even after restart.

Every graph-referenced URI gets a separate `POST_TRAVERSAL_CAPTURE` receipt.
A later B capture never replaces an A supply or claims that the graph analyzed B.
Concurrent supplies between protocol transactions remain possible. Traversal is
not frozen and no successful digest or write upgrades the analyzed-version label.

Reads use canonical file URIs, the host workspace's `os.Root`, and the existing
nonblocking Unix regular-file opener. Lexical/symlink escapes, virtual URIs,
noncanonical URIs, missing/nonregular files, byte limits, cancellation and budget
exhaustion remain explicit outcomes without fabricated content or silent omission.
Regular-file I/O may still be slow or uncancellable. Root replacement and files
changing during a read do not become source-snapshot guarantees.

Fixed acquisition bounds: 8 MiB embedded graph; 1 MiB per source read, 4 MiB total
post-traversal bytes/read budget, and 64 attempted source reads. Successful reads
charge actual bytes; failed reads conservatively charge the attempted bound.
Remaining graph references retain budget-failure receipts. There is no claim to
capture configurations, imports, declarations or compiler inputs absent from the
graph census. The offline envelope input cap is 48 MiB. These are per-operation
bounds, not a global bound on all clients' retained artifacts.

Receipt canonical bytes reuse `source.CanonicalizeReceipt`; failures have no
content identity. The record ID is SHA-256 over domain
`lsp-trace.graph-provenance.v1:receipt`, zero byte, and compact record JSON with an
empty `id`. Supply and post-capture records do not share an observed-identity
snapshot, so conflicting same-path versions are never collapsed.

## Offline validation and publication

```sh
lsp-trace schema get --family graph-provenance --version v1
lsp-trace validate --family graph-provenance --version v1 envelope.json
```

MCP `lsp_trace_v1_schema_get` and `lsp_trace_v1_validate` use the same family.
Pass the envelope as inline JSON **text in `input`** to preserve its exact outer
bytes. Object input also validates, but historical MCP map transport can reencode
field order. Use `output_selector` under the existing host `--publication-root`
for immutable MCP publication. CLI `--output` uses its existing generation
selector publication; these publication layouts are intentionally distinct.

`graphprovenance.ValidateFor` performs schema validation, legacy graph-v3 semantic
validation, a graph-derived census, exact graph digest and receipt hash/content
recomputation, supply-parameter/generation/version consistency, classification
checks and all required joins. The lower-level schema-only entry fails closed
rather than reporting shape-only validation as full admission. Validation needs
no source checkout, session, language server, or filesystem source reads.

**Offline consistency is not independent reauthentication.** Public Go/JSON
structs can be forged coherently, including notification parameters and purported
historical acquisition results. The validator rejects inconsistent reclassification
but cannot prove an actual read, write, producer identity or server interpretation.
Existing byte-publication integrity likewise does not confer that authority.
Source-bearing artifacts are sensitive. No automatic redaction is promised.

## Qualification

Hermetic tests include actual fake-wire A→B supply interleaving, separate A supply
and B post-capture, successful restart/stale-generation rejection, cached absence,
mandatory binding/receipt removal and reclassification, lexical/symlink escapes,
virtual/missing inputs, byte/file budgets, exact input handling and offline checks.
Real-server qualification is explicit and dependency-free:

```sh
LSP_TRACE_GRAPH_PROVENANCE_GOPLS=/absolute/installed/gopls \
  GOPROXY=off GOSUMDB=off go test ./cmd/lsp-trace-mcp \
  -run '^TestGraphProvenanceRealGoplsCLIAndMCP$' -count=1 -timeout=180s
```

It builds six local Go files, requires five distinct non-seed graph files, compares
real CLI/MCP envelope bytes and managed omitted graph bytes, then removes the
source fixture before separate CLI/MCP offline validation, schema retrieval and
publication checks. Optional `LSP_TRACE_GRAPH_PROVENANCE_RETAIN=/absolute/dir`
retains fixture evidence (never bootstrap environment configuration). Fake-wire
and real-server results are distinct; neither is Program A/B or AC1 acceptance.
