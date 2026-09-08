# Preparatory multi-target acquisition coordinator

`internal/acquisition` is the next bounded FR20/AC16 preparatory slice after
`internal/retainedpath`. **It is not full FR20, public v2 support, qualification,
Program B admission, or deployment approval.** Legacy incoming/slice wrappers,
registrations, schemas, IDs and output are unchanged. Public CLI/MCP integration,
graph-provenance v2, publication/export preservation and end-to-end qualification
belong to the subsequent work, not this commit.

## API and ownership

```go
client := sessionclient.New(runtime) // internal/acquisition/sessionclient
result, err := acquisition.Acquire(ctx, client, acquisition.Request{
    Mode: acquisition.Slice, // or acquisition.Incoming
    Context: acquisition.AcquisitionContext{
        ID: contextID, SessionID: sessionID, Generation: generation,
        PositionEncoding: "utf-16",
    },
    Root: root, // ID must be "root"
    RequiredTargets: required, // ordered; unique IDs; at most 63
    Limits: limits,
})
```

Both upcoming managed surfaces must call this coordinator, not multiply legacy
`DiscoverPrepared`/`IncomingPrepared` traversals. The core depends on native
`graph`, LSP types and `retainedpath`, not either operation wrapper.
`sessionclient` depends on the core and `sessionruntime`, never incomingops or
sliceops. An external-package compilation assertion checks that the existing
`incomingops.SessionClient` also satisfies the typed interface.

The client contract has DocumentSymbols, PrepareCallHierarchy, IncomingCalls and
OutgoingCalls. `NewWireClient` accepts a neutral RoundTrip callback; each
`WireRequest` includes the declared context and effective wire byte/message
limits. It validates bounded JSON shape (including coordinate presence), accepts both
hierarchical DocumentSymbol and flat SymbolInformation, filters flat rows to the
requested document URI, and normalizes the latter from `location.range` while
retaining an explicit flat-shape marker for deterministic identity replay. It
accepts successful protocol null as empty. Direct typed clients must enforce
their own transport limits and structural decoding.
Every wire call row requires a non-null `fromRanges` JSON array: missing, null,
scalar or object values fail the entire wire response, yielding FAILED with no
edges from that response. `fromRanges:[]` is valid zero-site support; top-level
protocol null remains successful-empty. A direct typed client must supply a
nonnil `FromRanges` slice for each valid row; a nil slice is malformed and produces
PARTIAL accounting without admitting that row's edge.
`WithDocumentSupply` optionally adds a source-supply function. The managed adapter
forwards exact generation, deadline and wire limits and retains runtime errors;
it does not resolve session aliases or authenticate the declared context.

`Request` is an internal Go contract, not a registered public JSON schema.
Positions are zero-based in the declared session encoding. Symbol and complete
line/character selectors are exclusive. URIs must be canonical absolute,
nonopaque, query/fragment-free URIs (clean absolute path, lowercase scheme/host,
canonical escaping). IDs are caller labels, not symbol identity. Root ID `root`
is reserved. All targets have independent down/up depth in [0,64]. In INCOMING,
only up depth governs acquisition. Zero depth yields an explicit frontier.

## Deterministic phases and identity

The retained policy is
`ROOTS_FIRST_ORDERED_RESOLUTION_SORTED_ADMISSION_ROUND_ROBIN_BFS_V1`:

1. Resolve root, then required locators in caller order, before any expansion.
   Exact locator aliases (including language) reuse resolution and its request
   references while retaining every requested row. Different locators may supply
   the same URI at different versions; their observations are not URI-coalesced.
2. Symbol resolution iteratively scans the returned hierarchical forest. Zero
   matches is MISSING; multiple names are AMBIGUOUS. Validate range and selection
   containment. Probe from selection start, on that line, within the symbol's
   half-open range (a zero-width range admits its one point), at most 65 positions.
   **All preparations, including positional requests, share 65 global attempts.**
   If candidate positions remain at that ceiling, resolution is BUDGET_BLOCKED,
   not MISSING.
   The known identifier-not-found error may advance a symbol probe, but remains a
   charged failed request. Other errors remain failed/unsupported/blocked.
3. Prepared items must have canonical matching source URI, valid ranges/kind and
   contain the candidate position. A named item must also match the returned
   symbol's name, kind and selection, with its range enclosed by the symbol.
   There is no spelling heuristic or reprepare. Multiple distinct valid prepared
   identities are AMBIGUOUS even if the node budget could admit only one. Exact
   duplicate identities are deduplicated. Namesakes in different URIs stay
   distinct. Native graph identity fields and prepared opaque data are retained.
4. Admit the resolved root first; admit remaining resolved identities in lexical
   native-ID order (stable ties). Admission is distinct from successful
   resolution. All admitted seeds take precedence over neighbors and consume one
   **global unique-node** budget, not one budget per target.
5. Rotate target queues, serving one BFS node per target per round, ordering each
   queue by depth then native ID. Sort neighbor admission by native ID. Cache
   each identity+direction query, including failures. Replay creates target-local
   depth/membership attribution, not another request, observation or support.
6. SLICE completes the outgoing phase for all targets, then starts each target's
   incoming BFS at its actual down-depth frontier and successfully empty
   outgoing leaves. Failed, malformed-only, budget-blocked, and cyclic nonempty
   nodes are not fabricated leaves. INCOMING starts only at each admitted target.
   Depth and visited sets are target-local; a frontier for one target can expand
   for another. No traversal stops early merely because a connection was found.
7. Admit both endpoints before retaining an edge. Preserve caller→callee
   orientation and native group IDs; invalid neighbor/item/site data is partial
   response evidence, not an edge. Duplicate response rows union native sites.

Determinism means identical ordered targets, limits, client outcomes and context
observations yield identical allocation and results. Reordering caller targets
may legitimately change request-budget allocation. There is no timing-independent
claim about live providers.

## Resource and evidence contract

Counts may be zero to intentionally block a resource. Hard configuration ceilings
are 10,000 nodes, 100,000 attempted operations, 64 MiB captured evidence,
100,000,000 path-work ticks, 16 MiB per response and 4,096 wire messages. Durations
must be positive; no defaults are silently applied. Context cancellation and
per-request deadlines are forwarded to the synchronous client. The managed
runtime enforces them for admitted notification/request pipe writes and response
reads, including stalled didOpen/didChange. Cancellation retires the exact owned
transport, joins I/O before returning, and returns REQUEST_CANCELLED or
REQUEST_TIMEOUT with no successful document version or supply. A failed/short
write likewise retires the stream and returns SESSION_POISONED. Successful
unchanged documents still return their version with no new supply observation.
The coordinator retains earlier evidence and records the failed supply attempt;
it does not start background goroutines to conceal an uncooperative client.

**Deadline scope and source-I/O policy:** these durations are cancellation
budgets, not an unconditional elapsed-time bound on file-backed Acquire. Optional
managed supply synchronously opens/reads a canonical workspace regular file, at
most 1 MiB, using the existing nonblocking Unix regular-file opener. Unsupported,
nonregular, out-of-scope, invalid UTF-8, and oversized inputs fail closed with
DOCUMENT_SUPPLY_UNAVAILABLE. Context is checked before source access and after
it returns; an expired read cannot proceed to supply. However os.OpenRoot,
metadata operations and regular-file reads may stall in the filesystem/kernel;
O_NONBLOCK and byte caps do not make regular-file I/O cancellable. This is the
same explicit limitation as `lsp-supplied-observations.md`, not a hard coordinator
filesystem guarantee. Do not admit automatic file-backed supply where a hard
end-to-end deadline is required: pass a RoundTrip-only runtime facade to
`sessionclient.New` (its method set must omit PrepareDocument), or a client with
already-available input and a genuinely cancellable supply primitive. The
RoundTrip-only policy produces no new supply observation; callers that require
one must reject that mode rather than fabricate it. Interruptible filesystem
isolation/preloading is separate work, not provided by this repair.

The runtime admits one protocol owner per exact generation. Concurrent
PrepareDocument, RoundTrip, Stop and Restart return LIFECYCLE_CONFLICT rather than
wait behind blocked notification I/O. Cancellation releases the owner only after
pipe interruption and worker join. Subsequent use of the retired generation
fails; a later explicit stop/restart reuses its retained close/reap observations
without attempting a frame on the retired stream. Restart clears document state;
old generation calls fail STALE_GENERATION after replacement. Retirement captures
the child object at admission and never closes a replacement looked up by ID.
A cooperative request cancellation notification is best-effort, bounded to 5 ms
of cleanup, followed by retirement and read join. OS scheduling, pipe close and
process reap add cleanup latency: there is no real-time scheduling guarantee.
Custom Starter wire children must provide close/teardown-interruptible I/O and
honor the existing teardown context; arbitrary uninterruptible Reader/Writer
implementations are not supported. Process/group supervision authority is
unchanged; this repair makes no stronger containment or source-identity claim.

- Every attempted document-symbol request, preparation probe, neighbor query and
  optional source-supply operation costs one global request. Cache reuse costs
  none. A blocked record is explicitly `attempted:false`, not a wire request.
- Evidence bytes count bounded JSON encodings of request parameters and normalized
  response/error values, including each encoder newline. This is **not** raw
  JSON-RPC transport bytes or the size of the whole final artifact. The wire
  adapter separately bounds transport responses/messages; effective response
  limits are reduced to the remaining capture budget. Metadata and bounded
  graph/accounting projections are not charged again as new provider evidence.
- Capture overflow retains no misleading truncated JSON payload: it reports
  CAPTURE_INCOMPLETE, records consumed bytes and blocks use of that response.
  Resource-blocked resolutions/expansions remain explicit on every affected row.
  Malformed normalized JSON evidence is CAPTURE_FAILED rather than falsely
  attributed to a byte limit.
- Each request record carries exact method, owner target, optional queried node,
  declared context, budget-before snapshot, attempted/outcome status, captured
  params/response and byte consumption. Alias resolution request IDs and cached
  expansion references preserve attributions without fabricating attempts.
- `EdgeObservations` joins actual successful neighbor responses to native group
  IDs and exact sites. Validation recomputes these joins from captured responses,
  requires each retained edge's canonical site set to equal the union of its
  supported observations, and rejects duplicate observation joins. Missing and
  invented sites fail even if graph receipts and path pointers are coherently
  resealed. Repeated response rows and repeated actual observations still union
  sites without increasing support. These are not independent-support claims.
- Supply observations are optional historical facts of supply, **not** frozen
  bytes, analyzed-source identity, compiler consumption or authenticated receipts.
  A nil runtime supply remains absent; repeated same-URI versions remain separate.

## Result and native graph boundary

`Result` carries the request/policies, graph, every target row, directional
expansion/layer/frontier sets, request records, supply observations, edge
observations and shared usage. It keeps resolution, admission, expansion and
connection distinct. Successful-null is SUCCESS_EMPTY, never FAILED. Directional
records distinguish SUCCESS_NONEMPTY, PARTIAL, FRONTIER, FAILED, UNSUPPORTED,
BUDGET_BLOCKED and NOT_APPLICABLE. Root failure does not suppress required rows.
`AcquisitionComplete` is only this bounded coordinator's accounting; path-search
completion is independent, and neither establishes source completeness.

The native graph uses unchanged v3 nodes, groups, call sites, seeds and receipts.
All requested labels have matching invocation/seed records, including failures.
Native memberships are reconstructed from each target's actual/replayed
expansions. `Graph.Slice` is nil: no false historical single-at summary is made.
The historical canonicalizer recomputes incoming counters from graph edges; those
legacy counters are **not** the coordinator's actual directional request counts.
A partial-acquisition diagnostic prevents the legacy canonicalizer from promoting
partial acquisition to complete. `ValidateReferences`, v3 semantic validation,
and coordinator joins run before return; tests additionally use the unchanged
native structural schema validator.

Native semantic/execution receipts retain their historical scope. The central
result is not wrapped in a fake graph-provenance v1 envelope. Future v2 must bind
this accounting and carry source/evidence joins and offline replay authority.
Consistency validation does not authenticate client claims.

`ValidateResult` additionally replays the deterministic acquisition scheduler
against the retained operation records. It does not call `Acquire`, clients,
supply primitives, or public operations. Each scheduled operation must consume
exactly its next record with matching target owner, locator/query parameters,
node, declared context (including generation), and full pre-usage snapshot.
Normalized successful and failed captures must re-encode exactly with their
recorded byte consumption. Resolution status, selected prepared payload (including
opaque data), ordered request references, root-first admission, cache attribution,
target-local depths/layers/starts/frontiers, memberships, supplies and graph must
match replay. Exact locator aliases may reference an earlier owner's matching
resolution; cache hits require an earlier exact identity-and-direction query.
They do not invent additional requests or observations.

Supplies are compared in operation order to complete returned Supply payloads,
including opaque observations/versions, and joined to the request's canonical URI,
locator and declared context/generation. The validator does not interpret opaque
observation fields as authenticated runtime identity. Missing/incomplete captures
cannot reveal their discarded payload: validation checks their disposition and
bounded declared consumption and does not fabricate response evidence for replay.

Acquisition-wide cancellation is a declared sticky boundary in unattempted
request records or cache-cancellation expansion records. A request-local timeout
alone does not assert global cancellation. These boundary claims are checked for
consistent scheduling and subsequent results, not authenticated offline events.
The shared scheduler is guarded separately by the persisted independent
three-node all-pairs oracle; sharing code is not independent proof of scheduling.

## Connections and handoff

After acquisition finishes, `PathInput` projects the final canonical graph to
`retainedpath.Edge`: native relation ID is GroupID; occurrences are ordered
`/edges/<index>/call_sites/<index>` pointers into this result's native graph.
Zero-site groups retain an empty witness list. No new artifact digest enters the
preimage, avoiding a graph/result digest cycle. Upcoming export must map these
pointers to its occurrence IDs, not drop them.

Each required target uses the **same** `retainedpath.Search` and independent
`retainedpath.Prove` kernel as existing bounded analysis, with one sequentially
shared path-work budget. The declared projection is
`NATIVE_CALLS_CALLER_TO_CALLEE_UNIT_GROUP_HOPS_V1`. SLICE asks root→required;
INCOMING asks required→root. Resolved/admitted equal endpoints can yield zero-hop
FOUND regardless of expansion failure. Unknown/unadmitted endpoints are
NOT_EVALUABLE. Only exhausted retained-graph search can return
NOT_FOUND_IN_RETAINED_GRAPH; pathwork/time exhaustion is INCOMPLETE with no partial
witness. Intermediate nodes, groups and all group callsite pointers remain in the
result. Negative retained-graph answers never claim source/runtime unreachability.

The root row itself has no root/required connection query and remains
NOT_EVALUABLE; each required row records effective endpoints, direction, status,
reason, witness and consumed pathwork. Alternative-path enumeration is not an
option of this bounded API.

Validation repeats `retainedpath.Search` in original required-target order using
one shared declared work budget and compares status, reason, witness and work;
`Prove` remains a separate topology check, not resource proof. Zero-hop FOUND
costs one work tick. A CANCELLED report must have an empty witness and describe
an unfinished search prefix at its reported work count; cancellation is then
sticky for later targets. A previously declared global acquisition cancellation
permits no subsequent completed path. This is cancellation-report consistency,
not proof of the time or origin of a cancellation event. A full internally
consistent replacement history is not detected as forgery by these checks.

## Regression scope

The persisted pure-Go guards cover connected/disconnected chains, both modes,
root ambiguity/missing, source-qualified namesakes, positional ambiguity,
duplicate locator/identity aliases, range/probe limits, null/errors/malformed
wire ranges, exact opaque data, cache replay and ancestor depth, root-first node
admission, round-robin request fairness, all resource bounds, cancellation and
managed restart failures, source-supply versions, no dangling edges/support
inflation, native schema/receipt joins and shared-kernel witness/work parity.
Compiling empty-coordinator RED preceded production; controlled smaller cache and
validator reductions are separately rejected. See the session claim for exact
commands and retained RED/GREEN logs. These fake fixtures do not qualify a native
provider or the still-unimplemented public FR20 path.

The validator-repair regression suite reverses the independent review's eight
false-acceptance probes into persistent rejection assertions, adds malformed
array/typed-nil and cancellation-prefix distinctions, and preserves the independent
4,096-case three-node oracle plus 252 request/node/evidence boundary combinations.
The repair does not change the preceding runtime cancellation API or its explicit
regular-filesystem I/O caveat. It remains preparatory work pending independent
review, not a claim of full FR20 readiness.
