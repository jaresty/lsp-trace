# Evidence ledger and unresolved evidence

## Custody and revision

- Worktree: `/private/tmp/adr0011-incoming-calls-v1-6yiY4m`
- Exact repository HEAD: `4b910fc0c517cb7801fb51d17d295897230d11fa`
- Exact derived managed session: `sk1:9056e071454cbbd51661782b2964d4a8d5c266675df42622d95c5d5032abf0f9`, generation 1
- Session routing observed READY for the exact worktree, language `go`, profile `gopls`, position encoding `utf-16`.
- MCP server instance (not language-server authority): `binary_version=lsp-trace-mcp wip-adr0007`, source revision `3ac56b455813831fa8fb691942cf66acead35cb0`, dirty=false, custody=`SELF_MEASURED`, executable SHA-256 `sha256:f22613951a64f9101e924133a75f7783ec43d15856d9a4dc12e8301784549e81`, instance `si1:e2101c512878dc58a47142b758624bae`.

## Retained command/probe log

Commands are recorded exactly as issued; tool envelopes are summarized without elevating authority.

```text
git -C /private/tmp/adr0011-incoming-calls-v1-6yiY4m rev-parse HEAD
→ 4b910fc0c517cb7801fb51d17d295897230d11fa

git -C /private/tmp/adr0011-incoming-calls-v1-6yiY4m status --short
→ (empty before authoring)

lsp_session_v1_list(detail=full, uri=file:///private/tmp/adr0011-incoming-calls-v1-6yiY4m)
→ APPLICABLE_SESSION_NOT_FOUND

lsp_session_v1_list(detail=full)
→ one READY parent: project, generation 1, /Users/schwa/dev/lsp-trace

lsp_session_v1_derive_workspace(session_id=project, generation=1,
  workspace_uri=file:///private/tmp/adr0011-incoming-calls-v1-6yiY4m)
→ READY sk1:9056e071454cbbd51661782b2964d4a8d5c266675df42622d95c5d5032abf0f9 generation 1

lsp_trace_v2_structural_context(regex_locator=finishCallHierarchyLayer1GenericRefusal, projection=PROJECTED/INCLUDE)
→ SUCCEEDED; server-reported callers included RoundTrip and roundTripWithPrivateTransaction; outgoing calls included owner construction, finishAccepted, publish, requestTerminal, waitReleased.

lsp_trace_v2_structural_context(regex_locator=finishRoundTrip, regex max_work=100000, projection=PROJECTED/INCLUDE)
→ RESOURCE_LIMIT / REGEX_MAX_WORK; observed=135592, suggested max_work=135592.

lsp_session_v1_status(session_id=sk1:9056e…f0f9, generation=1, detail=full)
→ READY metadata listed above; no language-server binary/argv/env/initialize/original-frame custody.

lsp_trace_v1_incoming(session_id=sk1:9056e…f0f9, generation=1,
  uri=file:///private/tmp/adr0011-incoming-calls-v1-6yiY4m/sessionruntime/b4_owned_method_pair_lease.go,
  line=819, character=18, max_depth=2, max_nodes=50, relations=[CALLS],
  workspace_revision={kind:git,commit:4b910fc0…,custody:CALLER_ASSERTED})
→ PARTIAL; 50 nodes, 49 server-reported CALLS edges, truncated=true,
  source_graph_complete=UNKNOWN, workspace/server provenance UNKNOWN,
  repeated SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE diagnostics,
  semantic commitment sha256:f19e11b8f097c4175e2fc158a12b6a9e0e5f01cfd5e85268c768277a7af885eb.
```

The resource-limit result is retained as a typed bounded failure. It is not replaced by a textual CALLS claim. The live incoming probe proves only that this server reported a bounded, truncated relationship neighborhood; its diagnostics, caller-asserted revision, unknown source completeness, and unknown provider provenance are retained rather than repaired. Source windows are used only for source-visible ownership design.

## Source-derived facts

- `Manager.RoundTrip` creates generic attempt lineage before dispatch and funnels multiple pre-effect failures through `finishCallHierarchyLayer1GenericRefusal`.
- The normal finalizer calls `finishAccepted`, constructs/publishes `callHierarchyLayer1GenericOwner`, clears attempt state, decrements workers, unlocks, then requests and joins terminal release.
- `callHierarchyLayer1GenericOwner` protects copied observations with a mutex/condition; callback admission increments `active`; release closes admission, waits for active callbacks, zeroes observation, and releases once.
- STOP/RESTART detach exact generation owners and join terminal release before deletion/successor startup.
- Incoming production calls `traverse.Incoming` via `incomingops.SessionClient`, whose call path uses `Manager.RoundTrip` for prepare/incoming/outgoing LSP methods.
- Outgoing public trace execution independently builds a SliceV3 request and invokes `acquisitionorchestration.ExecuteExplicitTracePrepared` or `ExecuteExplicitTrace`.
- `docs/acquisition-coordinator.md` states non-null `fromRanges:[]` is valid zero-site support; missing/null/non-array fails the response.
- Exact-transport originals are proposed, informative dependencies only. They do not select or justify any numeric limit in this package. The prior contrary statement is superseded by `numeric-bound-audit.log`.

## Bound source hashes

```text
ad06c64bf3e267587dc94197a80ece1a03bc7933bd9263b4bb1ac78cad69fb8d  sessionruntime/sessionruntime.go
b53e51b89effa526337b96fcf883078da332b9b1a1457fcd1fc44a3db2e585e9  sessionruntime/b4_owned_method_pair_lease.go
040622acaf54ea77fe79f3333b10857cfce0bef09bce81f448bd4ee0a619b884  incomingops/executor.go
52b1ae3217877c1c1d6db00fab796b17d940bd6a9f823dec6e15c4fe47e40455  cmd/lsp-trace-mcp/trace_executor.go
65a165b8c10f08fd40ea2b7fe6478c72cd79040f058298b63f8872ec809c4cd2  docs/acquisition-coordinator.md
6fbb54cf37ec6efe86cb42e8717f9610716b8a84e432a57367135b8da9e265ae  docs/qualification/originals/generic-lsp-exact-transport-v1.json
```

## Evidence classification

| Claim | Evidence kind | Ceiling |
|---|---|---|
| Generic owner/finalization/lifecycle shape | exact source bytes + bounded server-reported structural neighborhood | architecture seam only; no runtime completeness |
| incoming/outgoing production separation | exact source bytes | source-visible call paths only |
| empty-array meaning | retained repository contract text | project contract, not exact-profile provider proof |
| numeric limits | retained exact policy original | proposed/private limit authority; implementation witnesses still required |
| exact worktree session READY | prospective managed status | session availability only |
| provider behavior | one managed profile name/status plus bounded LSP probes | no authentication, authorship, completeness, or production authority |

## Unresolved evidence (only)

These are GO blockers, not deferred design work:

1. Full language-server executable bytes, length, SHA-256, version output, launch argv, cwd, behavior-relevant environment, Go toolchain/module/workspace inputs, and launch-chain custody.
2. Exact initialize request/result and initialized notification original frames, capability negotiation, and binding to the same responding process.
3. Provider-verified exact workspace revision/dirty state and target source bytes at request time; caller assertion alone is insufficient.
4. Complete original request/response frame custody for the exact incoming three-transaction chain and exact outgoing three-transaction chain.
5. A prospective exact-profile fixture containing a real non-empty incoming response, an incoming zero-site member, and corresponding independently interpreted provider documentation/evidence.
6. A separate prospective exact-profile outgoing fixture with caller-side ranges and independent provider identity.
7. Independent authority reviewers for incoming and outgoing, with non-interchangeable signatures/decisions.
8. Accepted governing requirements or retained measurements, explicit margins, and causal exact-limit/+1 runtime evidence for every numeric row. `numeric-bound-audit.log` establishes the current hard-stop and exact shadowing guards.
9. **Resolved architecture input:** exact outgoing census is frozen in `outgoing-structural-census.log`; server/provider completeness remains unknown and is not inferred.
10. Disabled and CALLS_ONLY byte-identity witnesses covering provider writes, responses, transaction order/keys, cancellation, scheduling, and final legacy artifact.
11. Updated C15 ownership census for every new owner/alias/copy/transfer/release and independent release/lifetime review.
12. Separate production/public-surface admission review after implementation and qualification; neither is supplied here.

No other unknown is used to block this architecture package.