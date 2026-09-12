# Final representative multi-application replay qualification

This is an execution plan, not a qualification result. It never upgrades fixture, synthetic, historical, or zero-node evidence into representative runtime qualification.

## Outcome vocabulary

- `READY`: every listed prerequisite was observed and the command produced the expected typed result with matching revision, provider, session, generation, and custody.
- `BLOCKED_EXTERNAL`: implementation exists, but an independently supplied final binary, provider executable, workspace/revision, retained artifact, publication root, or fresh managed session is absent or unsuitable.
- `NOT_YET_IMPLEMENTED`: the repository contract explicitly marks the requested production surface as deferred or intentional RED.

Record `FAIL` separately when all prerequisites were present but execution or an assertion failed. Never rewrite `FAIL`, `BLOCKED_EXTERNAL`, or `NOT_YET_IMPLEMENTED` as `READY`.

## Inputs (explicit; no private paths are encoded here)

| Name | Required binding |
|---|---|
| `LSP_TRACE_BIN` | final `lsp-trace` executable; record `version` output, binary SHA-256, build revision, and modified state |
| `LSP_TRACE_MCP_BIN` | final `lsp-trace-mcp` executable with the same identity bindings |
| `GOPLS_BIN`, `CSHARP_LS_BIN`, `EMBER_PROVIDER_BIN` | independently selected absolute executable, version, and SHA-256; Ember provider is required only for JavaScript/provider rows |
| workspace inputs | application ID, canonical workspace root supplied out of band, exact Git revision and custody class, language ID, target URI/symbol or zero-based coordinates |
| session inputs | exact `session_id`, `generation`, state, initialize Call Hierarchy capability, position encoding, provider declarations, and bootstrap identity |
| artifact inputs | schema ID/version, exact SHA-256, byte length, selector or root-confined input mode, source/custody revision, provider/session/invocation identity |
| publication inputs | caller-approved temporary qualification root and safe relative selector; never a private evidence root |

Live MCP requests use `timeout_ms=60000` and `request_timeout_ms=60000`. CLI acquisition records the exact manifest-owned effective limits; current production discovery and seed replay use `timeout_ms=60000` and `request_timeout_ms=30000` and expose no inline override. Do not restart a managed session during qualification without reviewing its current status, generation, startup diagnostics, workspace/revision, executable identity, and bootstrap configuration. A restart creates a new generation and invalidates prior exact-generation readiness.

## Before the final build: repository-only checks

These checks can establish implementation and fixture readiness only.

```sh
git diff --check
./scripts/check-docs.sh
go test ./cmd/lsp-trace -run 'TestAutomaticDiscoveryRetainedSeedReplayQualification|TestDiscovery|TestGroupedSlice|TestVerifyPassage' -count=1
go test ./internal/programc ./internal/programcadmission ./internal/programccompose ./internal/captureset ./internal/hydratedevidence ./internal/hydratedinspection ./internal/passageverification -count=1
go test ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace-mcp -run 'TestToolProfilesPreserveFullAndCompactAdvertisement|TestCompactToolProfileProcessAdvertisementAndHiddenDispatch|ProgramC|CaptureSet|Hydrat|Passage|Legacy|Alias' -count=1
python3 scripts/test-program-c-profiles.py
./scripts/qualify-compatibility-release.sh
```

For the opt-in fake-LSP discovery/replay mechanism check only:

```sh
LSP_TRACE_QUALIFY_DISCOVERY_REPLAY=1 go test ./cmd/lsp-trace -run '^TestAutomaticDiscoveryRetainedSeedReplayQualification$' -count=1 -v
```

A pass is `READY` only for the synthetic mechanism. It is not a representative application/provider result. The `trace` facade is implemented and has public-process V5 parity guards. Production `census` and default/advanced/hidden-legacy MCP advertisement remain `NOT_YET_IMPLEMENTED` until their active implementation lanes are integrated; current `slice --from-file` discovery, Seeds V2 replay, and compact/full MCP profiles remain compatibility surfaces.

Run `./scripts/release-check.sh`, `go test ./...`, `go vet ./...`, and `go build ./...` only at the reviewed release-candidate revision. The release check inspects retained evidence and builds temporary binaries; it does not start external language servers or refresh representative evidence.

## Final-binary and fresh-session matrix

Run rows in order. Stop dependent rows when a prerequisite is not `READY`.

| Order | Coordinate and action | Expected typed outcome | Earliest executable state |
|---:|---|---|---|
| 1 | Final binary identity and installed-version checks | exact version/revision/modified-state and SHA-256 match the reviewed candidate; otherwise `BLOCKED_EXTERNAL` or `FAIL` | final binary |
| 2 | Go/gopls fresh managed session; list then status exact generation | session `READY`, Call Hierarchy advertised, position encoding retained, workspace revision matches | final binary + fresh gopls session |
| 3 | C#/csharp-ls fresh managed session for School Surveys; list then status exact generation | same readiness evidence as row 2, with exact School Surveys revision and C# language identity | final binary + fresh csharp-ls session |
| 4 | JavaScript/Ember provider, if claimed | independently installed provider version/identity, declared capabilities, real MCP/provider transport, exact revision and custody | final binary + provider + managed substrate |
| 5 | One exact incoming and one bounded slice per application | schema-valid V5 acquisition; exact session/generation/provider/revision custody; non-empty expected calls/ranges or an honest typed incomplete/error | rows 2–4 |
| 6 | Automatic file/directory discovery to retained canonical Seeds V2 | closed operational census; V5 retains canonical Seeds V2 bytes and discovery custody | fresh session for each app |
| 7 | Replay retained Seeds V2 with `--seed-file` | native graph identity/digest and server-call parity with discovery; replay does not forge discovery custody | row 6 artifact |
| 8 | Ungrouped versus grouped Program C | ungrouped V5 unchanged; grouped run publishes V5 first and emits deterministic Leiden presentation using fixed seed/top-k values | row 7 |
| 9 | Trace parity | CLI/MCP or exact-target/discovery paths selected for comparison have equal admitted artifact bytes/digests and preserve provider/session/revision custody | rows 5–7 |
| 10 | Multi-capture composition and composite admission | exact constituent replay, compatibility, canonical order, custody set, and opaque composite admission pass; no cross-capture calls invented | compatible row 5/7 V5 captures |
| 11 | Hydration, including a local artifact greater than 1 MiB | root-confined/content-addressed/verified-selector ingress succeeds within hydration limits; direct inline/oversize policy remains typed and no custody upgrade occurs | final binary + explicit artifact input |
| 12 | Passage verification | `lsp-trace.passage-verification.v1`; exact artifact/inspection/seed/node/URI/range/passage digest; Graph V3 may return `SOURCE_BYTES_UNAVAILABLE`, qualified V5 source snapshots may verify bytes | row 11 admitted artifact |
| 13 | Capture-set publication and inspection | private canonical selector, exact digest/length/schema/identity verification, atomic no-replace receipt; inspection does not aggregate constituent custody or admit direct Leiden | explicit temporary publication root + capture-set manifest |
| 14 | MCP profiles and legacy compatibility | current full=32 and compact=10 compatibility behavior; aliases callable but unadvertised; hidden dispatch parity and canonical envelope tool names | final MCP binary |
| 15 | Whole-run census, once available | every enumerated file/symbol/capture/batch disposition reconciles; incomplete work fails closed; no endpoint/source-completeness overclaim | only when production census/capture-set producer is implemented |

Rows 6–10 must be repeated for at least one Go/gopls application and School Surveys/C#, and for JavaScript/Ember only when that provider coordinate is a release claim. D01/DASL retained grouped evidence is a replay input and regression witness; it does not replace a fresh final-binary acquisition. Preserve its recorded build revision, provider/version, session/generation, invocation, source/custody revision, graph digest, grouping seed, and top-k parameters.

## Representative command templates

Discover exact sessions without changing them:

```sh
"$LSP_TRACE_MCP_BIN" --bootstrap-config "$BOOTSTRAP_CONFIG" --tool-profile full
# MCP: lsp_session_v1_list {}
# MCP: lsp_session_v1_status {"session_id":"$SESSION_ID","generation":$GENERATION}
```

Issue live requests only after status is `READY`:

```json
{"session_id":"$SESSION_ID","generation":$GENERATION,"uri":"$FILE_URI","line":0,"character":0,"start_mode":"at","up_depth":2,"down_depth":2,"max_nodes":10000,"timeout_ms":60000,"request_timeout_ms":60000,"workspace_revision":{"kind":"git","commit":"$REVISION","custody":"CALLER_ASSERTED"},"fail_on_unknown_revision":true}
```

Automatic discovery, replay, and grouping use the final CLI and an independently reviewed server command or profile:

```sh
"$LSP_TRACE_BIN" slice --production-v5 --workspace "$WORKSPACE" --server "$SERVER" --language-id "$LANGUAGE_ID" --from-file "$SCOPE"
"$LSP_TRACE_BIN" slice --production-v5 --workspace "$WORKSPACE" --server "$SERVER" --language-id "$LANGUAGE_ID" --seed-file "$SEEDS_V2"
"$LSP_TRACE_BIN" slice --production-v5 --workspace "$WORKSPACE" --server "$SERVER" --language-id "$LANGUAGE_ID" --seed-file "$SEEDS_V2" --output "$GRAPH_SELECTOR" --group-by leiden --community-seed 19 --pagerank-top-k 20 --hub-top-k 20
```

Use `program-c compose`, composite admission/Leiden, hydrated inspection, passage verification, and capture-set inspection only with exact inputs reported by the preceding row. Copy the final binary's own `--help` syntax into the run ledger before execution; do not infer flags from a retained older binary.

## School Surveys prerequisite and zero-node boundary

A fresh qualified School Surveys session requires: the final reviewed `lsp-trace-mcp` binary; a host-authored bootstrap entry pointing to an independently trusted absolute `csharp-ls` executable; the canonical School Surveys workspace at the exact reviewed Git revision; required SDK/project restore already complete under the operator's authority; successful initialize with usable Call Hierarchy and retained position encoding; initialized completion; successful document supply/`didOpen` and `documentSymbol` for the target; and a newly observed exact `session_id`/`generation` in `READY`. Review status and diagnostics first; do not restart merely to obtain freshness.

A retained zero-node artifact cannot distinguish at least these causes: initialization/capability failure, document supply or symbol preparation failure, wrong language/workspace/revision, stale generation, wrong URI, wrong zero-based position/encoding, a valid target with no reported calls, or bounded traversal incompleteness. Node count alone contains neither the startup/readiness evidence nor enough coordinate-attempt evidence to adjudicate initialization versus coordinates. Therefore it remains `BLOCKED_EXTERNAL` until a fresh exact-generation run retains both lifecycle/capability evidence and the attempted target coordinates.

## Run ledger (one record per row)

Record: timestamp; application; command/request digest; binary version/revision/SHA-256; provider executable/version/SHA-256; workspace revision and custody; session ID/generation/state; position encoding; input schema/digest/length/selector; exact effective global/request limits; output schema/digest/length; census; typed outcome; claim ceiling; reviewer; and prerequisite row IDs. Keep private paths and source bytes outside this repository.
