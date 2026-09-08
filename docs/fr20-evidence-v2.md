# FR20 internal graph-provenance V2

This stage adds an internal, offline-replayable provenance envelope around the
[acquisition coordinator](acquisition-coordinator.md). It does **not** implement
retained-calls V2, new acquisition/export/analysis CLI or MCP operations, or
FR21 source-context evidence. Those are separate follow-on stages.

## API and exact admission

Package `internal/graphprovenance` exposes:

```go
func CaptureV2(ctx context.Context, result acquisition.Result, workspace string) ([]byte, error)
func CensusV2(result acquisition.Result) ([]BindingV2, error)
func ValidateV2(e EvidenceV2) error
func ValidateFor(raw []byte, family, version string) (string, error)
```

The final argument of `CaptureV2` is the host-selected workspace directory, not
an authority supplied by a language server. `CaptureV2` returns admitted JSON.
For incoming bytes, use `ValidateFor` before consuming `EvidenceV2`; a successful
`json.Unmarshal` alone is **not** admission. Select family `graph-provenance` and version `v2`. The explicit full version
`lsp-trace.graph-provenance.v2` is also accepted. Omitting a version does not
select V2. V1, V2 and unrelated families do not silently fall back to each other.

The schema registry exposes `graph-provenance/v2` through the existing generic
schema lookup. Structural validation alone does not admit provenance: composed
`graphprovenance.ValidateFor` is required. No new public tool is registered.

## Envelope and native graph

The envelope contains:

- Exact `schema_version`, policy
  `NATIVE_V3_CANONICAL_BYTES_TYPED_ACQUISITION_POST_CAPTURE_V2`,
  `ANALYZED_VERSION_UNVERIFIED` and `UNKNOWN_INCOMPLETE` ceilings; canonical host
  `workspace_uri`.
- `graph_bytes`: base64 encoding of one authoritative, canonical native graph-v3
  serialization, and its domain-separated `graph_digest`.
- `acquisition`: the complete typed `acquisition.Result` descriptor: request,
  explicit limits, all requested target rows, resolutions/traversals, actual
  request records, edge observations, usage, supplies and connection outcome.
- Actual `supplies`, later filesystem `captures`, their explicit capture budget,
  and the recomputable `bindings` census.

The embedded descriptor graph must agree with `graph_bytes`. The decoder restores
native fields hidden by the historical `graph.Graph` JSON representation,
including seeds/memberships, summary and optional slice metadata, using
`graph.DecodeNativeV3`; it checks canonical reserialization. It does not decode
native bytes into an incomplete plain `graph.Graph`, invent a single-at slice,
or replace the coordinator descriptor with a second accounting model.

`acquisition.ValidateResult` is mandatory before capture and admission. Its
coordinator replay/path/accounting checks remain the source of truth. Empty or
ambiguous roots, resolved-but-unadmitted identities, partial acquisition,
connected/disconnected targets and zero-hop aliases remain explicit. A readable
source file does not change a failed selector into a resolved target.

Wire-contract values are retained, including opaque JSON integer precision.
Whitespace-only outer formatting is supported; graph bytes themselves must stay
canonical. This does not promise preservation of Go allocation details such as
historically omitted empty slices, or authorize rewriting protocol payloads.

## Census pointer space

`CensusV2` returns sorted, unbound references over the logical projection:

- `/graph/...`: the native graph decoded from `graph_bytes`;
- `/acquisition/...`: the complete coordinator descriptor, excluding its redundant
  graph copy.

Capture/admission deterministically attach receipt IDs to that census. This is a
logical decoded pointer space, not JSON Pointers into the base64 string.

The census includes native nodes/ranges, call groups/sites, seed selectors and
memberships, diagnostics, portable/replay-input locators, plus **all requested
selectors**, target resolution/traversal metadata, request parameters/responses,
edge observations and connection witness references. URI-less document-symbol
ranges inherit their query URI; outgoing `fromRanges` use the queried caller and
incoming `fromRanges` use the response's `from.uri`, not the queried callee.
Edge-observation ranges join the native group's caller through an indexed lookup.
Node references outside the
admitted node table retain their available typed URI or an explicit non-source
classification. `SOURCE_ARTIFACT.locator` is used as its own URI, not misread as a
seed label. Free-text messages and opaque `data`/supply-observation bodies are not
mined for filenames. Opaque strings that look like URIs add no receipts.

Position locators have a tuple binding at the locator pointer, including the
original request, every requested target row, and captured supply parameters.
Connection `group_ids` and nested `occurrence_ids`, native seed
`reached_relation_ids`, and slice `outgoing_relation_ids` have individually
indexed `NON_SOURCE` bindings: these identify relations/occurrences, not source
coordinates.

Every V2 binding has `anchor_status`: `SOURCE_REFERENCE` for source references
without coordinate claims, `VALID_COORDINATES` for typed, present, ordered
coordinates (including empty ranges), `INVALID_COORDINATES` for unusable source
coordinate carriers, and `NON_SOURCE` for non-source references. Selection ranges
must be contained in their declaration range. Invalid coordinates retain the raw
carrier, URI attribution, and source receipts; they never become usable spans
merely because a file is readable. Composed admission recomputes every status and
rejects missing or changed rows. This V2-only contract does not change V1 bindings.

Native group IDs, call-site indices and pointers are not renumbered. The later
retained-calls exporter must map its occurrences to this admitted pointer space;
this stage does not invent occurrence IDs or an export context identity.

## Actual supplies versus later captures

A supply row joins a successful, captured `source/prepareDocument` request and its
actual coordinator `Supply`. An observed notification retains its original
`DocumentSupply` observation and an `LSP_SUPPLIED` receipt with method, parameters,
content, source name, exact session/generation and document version. These are
coordinator request IDs, not invented JSON-RPC notification IDs.

Known notification observations must match the current runtime `DocumentSupply`
contract, including exact-number method/parameter/content/version consistency.
Observed `didOpen.languageId` must exactly equal its joined coordinator supply
language. Nonempty explicit locator language is used verbatim by the runtime and
must agree; empty input may use a configured default or inferred language, which
this envelope does not invent. Available same-URI language observations must
agree within the retained exact session/generation. `didChange` has no language
field: only available supply language linkage is compared. Missing/cached
notification observations do not fabricate a `didOpen` comparison. This language
linkage does not freeze source bytes across independent document versions.
Malformed/unsupported nonempty notification shapes are rejected rather than
fabricated into receipts. A missing/null observation is explicitly
`NO_NOTIFICATION_OBSERVATION`, with no invented notification receipt. This includes
cached/absent observations. Different versions of the same URI get separate
receipts; duplicate URI/version receipt keys are rejected.

After acquisition, capture performs at most one workspace-scoped, bounded regular
file attempt per distinct referenced URI. The post-acquisition bytes are not
substituted for supplied bytes. Readable, missing, nonregular, unreadable,
oversized, invalid/virtual/out-of-scope URI, cancelled and exhausted-budget
outcomes remain distinct. Failed receipts contain no content.

`capture_budget` records `OPENED`/`UNAVAILABLE` root outcome, attempts and charged
bytes. Successful reads charge actual bytes. Failed attempted reads conservatively
charge their permitted bound. Classification without a read charges nothing.
Offline admission replays this policy, requires exhausted-budget dispositions,
and disallows reads after cancellation or successful reads from an unavailable
root. These are internally consistent reports, not proof of filesystem history.

## Identity preimages

Let `D(domain, payload)` be `"sha256:" + hex(SHA256(domain || NUL || payload))`.

- Graph digest: `D("lsp-trace.graph-provenance.v2:graph", graph_bytes)`.
- Receipt ID: `D("lsp-trace.graph-provenance.v2:receipt", json.Marshal(receipt))`
  with that receipt's `ID` set to the empty string. Source bytes and all supplied
  metadata participate through the receipt fields. This is not a self-digest.
- The existing `source.CanonicalizeReceipt` records item identity/locator,
  acquisition disposition, provenance, and (for readable content) SHA-256 of
  acquired bytes; unreadable content requires a failure reason and no bytes.
  Admission reconstructs that canonical receipt. It does not persist byte/line
  counts, UTF-8 state, or newline maps. V2 separately checks actual content length,
  capture budgets, and supplied-content UTF-8 validity.

The graph digest identifies graph bytes, **not the entire acquisition context**.
A future retained-calls V2 context identity must commit the admitted envelope and
its own explicit policies; it must not silently reuse a V1 or graph-only identity.

## Resource contract

Limits are independent, checked without truncation:

| Limit | V2 ceiling |
| --- | ---: |
| Envelope | 192 MiB |
| Native graph bytes | 32 MiB |
| Known decoded JSON carrier depth | 64 |
| Known numeric token bytes / exponent magnitude | 64 / 1,024 |
| Coordinator/native node ceiling | 10,000 |
| Requested target rows | 64 |
| Captured request/edge-observation records | 100,000 each |
| Accounted graph/descriptor metadata rows | 1,000,000 |
| Census rows / native call-site rows | 50,000 |
| Accounted census pointer/URI bytes | 8 MiB |
| Expanded binding/receipt join bytes | 8 MiB |
| Actual supply receipt rows | 64 |
| Known notification observation carrier | 4 MiB |
| Per-file readable capture | 1 MiB |
| Capture attempts | 64 |
| Total charged capture bytes | 4 MiB |

Combined byte/census caps can reject an otherwise valid coordinator result; they
never silently lower its supplied limits or clip its rows. In particular this
stage does not apply the future retained-calls 4,096-node ceiling. Existing
coordinator request/evidence/path/time/message limits must be explicitly present.

Depth is checked before duplicate-member scans and historical recursive
validators, including the decoded `graph_bytes` carrier. Raw graph row counts are
checked before historical graph schema/semantic traversal. Source content remains
bytes; opaque `data` is not promoted to a recursively interpreted source carrier.
The underlying regular-file primitive has the existing scoped/nonblocking-open
protections; this does not claim a hard kernel deadline for every filesystem read.

## Compatibility and follow-on boundary

Graph-provenance V1 and retained-calls V1 schemas, identities, canonical bytes and
closed admissions are unchanged. Native graph-v3 serialization is unchanged;
the new decoder is used only by the V2 provenance path. No retained-calls V2
schema/exporter, public acquisition tool, analytics API or new authentication
claim is included here.

The next stage can consume `EvidenceV2.Acquisition` plus the authoritative graph,
receipts and census after composed admission, without rereading source files or
inventing coordinator accounting. Source-free replay and tamper tests cover this
handoff, but neither validation nor digest consistency proves producer identity,
source freeze, compiler completeness or runtime behavior.
