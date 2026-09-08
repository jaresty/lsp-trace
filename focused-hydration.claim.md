# Internal focused hydration checkpoint

Branch: `checkpoint/focused-hydration-027e3804`.
Baseline: `432ed29` (`fix(provider): supervise deadline through transport and process completion`). `22c733d` is the earlier public-v2 CLI flag documentation correction, not the provider fix.
RED commit: `4c0718e`.
Implementation/test/documentation commit: `81bd22da1c1abb247f8957a3cdc7ee9a1cef7431`.
This ledger is committed separately after the implementation. Parent can cherry-pick the three consecutive commits, or squash them as one checkpoint; do not integrate RED alone.

Worktree: `/var/folders/bc/qbdy42vs4jn56zwrb2y09l9m0000gn/T/pi-agent-027e3804-8501-4a4-db49fcb0`.
No previous worker was resumed. No main/product repository, runtime, process, provider, public CLI/MCP, shared schema registry, or publication code was changed. No network/install/deploy/delegation operation or full-suite/race/provider qualification was run.

## Derivation

This executing child loaded the delegated local-derivation and route contracts, listed Bar skills, loaded bar-workflow and discovery, and executed its own single craft build:

`bar build craft --subject "Focused hydration selection over admitted native graph provenance bytes, preserving complete core catalog and exact authority mappings"`

Bar expanded craft into make/witness/ground/gate/falsify/atomic and full. Token and composition contracts were loaded locally. The stated-property phase governed exact requested occurrence coverage, typed native joins, complete core source catalogs, NON_SOURCE exclusion, retained-range/body/authority boundaries, externally bound validation, all exact joined receipts, and unsupported-family errors. No parent derivation was substituted.

Read FR21 (especially points 10/11), AC17, docs/hydrated-evidence.md, existing core types/admission/selection/validation/paging/wire, and native V1/V2 census. Read `/tmp/lsp-trace-fr20-fr21-integration/CLAIM.md` to confirm actual original artifact provenance and deletion boundary.

### RED / GREEN evidence and limits

- `focused-red.log`, committed in `4c0718e`: new assertion-bearing tests compiled against a present-but-empty stub API. Tests actually executed and rejected absent node/edge mappings, unknown/duplicate accounting, catalog/NON_SOURCE policy, privacy/ranges/receipts, endpoints, asserted states, unsupported admission and actual replay. The stub validator also accepted empty manifests, producing actual tampering assertion failures. This is not a missing-symbol/compilation RED.
- `focused-mutation.log`: a temporary Go overlay removed the independent native-coverage audit; the persisted `TestFocusedIndependentCoverage` failed with `ASSERT_INDEPENDENT_COVERAGE: coherently omitted native callsite accepted`. The smaller artifact was rejected; original source was never overwritten. Temporary mutation runner/overlay removed.
- `focused-final-tests.log`: `go test ./internal/hydratedevidence -count=1 -v` passed all existing core and new focused tests (3.021s reported). The restored independent-coverage guard passed.
- `go vet ./internal/hydratedevidence` passed with no diagnostics (`focused-vet.log` is intentionally empty). `git diff --check` and staged diff checks passed.
- All Go commands used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off`.
- Two supplemental V1 fixture construction attempts failed before hydration assertions: plain JSON decode lost custom V3 seed representation, then the synthetic added edge lacked seed relation membership. The fixture was corrected to use `graph.DecodeNativeV3` and explicit reached/outgoing membership. These are test-construction failures, not hydration RED or production repairs. Both were searched in nn before repair.
- Counterfactual coverage is bounded: the compiling stub witnesses top-level absent behavior and the independent auditor has a selective reduction witness. Do not reinterpret this as a separate minimal-mutation failure/pass pair for every internal assertion or perfect exhaustive Bar falsification coverage. Passing tests are not full FR21 or deployment approval.

## Actual interfaces

New code: `internal/hydratedevidence/focused.go`.
Tests: `focused_test.go`, `focused_native_test.go` in that package.
API documentation: `docs/hydrated-evidence.md`, section Internal focused-selection checkpoint.

```go
focus := hydratedevidence.DefaultFocusRequest()
focus.NodeIDs = []string{/* exact native node IDs */}
focus.RelationIDs = []string{/* exact native edge relation IDs */}
focus.SidecarRecordIDs = []string{/* exact digest-qualified sidecar catalog record IDs */}
focus.IncludeBodies = true // false by default; overrides core policy body bit
focus.WholeFile = false    // separate explicit expansion
focus.EndpointContext = false
focus.PositionEncoding = "" // V2 native encoding; supply explicitly for V1 conversion
result, err := hydratedevidence.HydrateFocused(input, focus)
err = hydratedevidence.ValidateFocused(input, focus, result)
err = hydratedevidence.Validate(input, result.Request, result.Bundle)
text, err := hydratedevidence.Text(input, result.Request, result.Bundle)
```

`Input` still holds exact artifact/sidecar bytes, never paths. `FocusRequest.CorePolicy` is the existing bounded `Policy`; zero limits are not silently defaulted. Result contains exposed `FocusManifest`, unchanged core `Request`, and core `Bundle`.

- Typed exact `nodes[i].id` joins `nodes[i]/range`; typed exact `edges[i].relation_id` joins every `edges[i]/call_sites[j]`, with caller URI and retained native range. V2 uses `/graph/...`. No substring/name/pointer-prefix ownership inference. No acquisition-bookkeeping sweep.
- Node range is a retained item range, not a complete containing construct. Endpoint node ranges are opt-in and do not replace relation sites. Zero-site edges retain IDs/endpoints and `NO_CALL_SITES` rather than inventing context.
- Manifest records `PRESERVE_OCCURRENCES`, `ALL_EXACT_BOUND_RECEIPTS`, `RETAINED_NODE_RANGE`, exact input digests and exact focus request/policy digest. Requested order is nodes, relations, sidecars; each list preserves order/duplicates. Unique generated core origin IDs preserve every occurrence/site/receipt.
- All exact joined receipts are selected in source-ID order. Same-URI supplied and post-capture receipts remain distinct; no latest/analyzed-version claim. Source classification/receipt hashes/version metadata remain in the full core catalog.
- Requested unknown IDs remain explicit. Unsupported record types, no sites, absent bindings/sources/ranges, invalid retained anchors and NON_SOURCE exclusions are distinct manifest dispositions. MAPPED is an identity join, NOT an exported/complete-context status; site OriginIDs address the full core dispositions for privacy, source availability, invalid coordinates and budgets.
- NON_SOURCE bookkeeping is excluded and counted, not selected with empty source IDs. No misleading missing-source warning is created for it. Underlying catalog remains intact and inspectable.
- **Full admitted source catalog remains in Bundle.Sources.** Core Validate requires it. Focus selection is not implemented by trimming or forging an artifact/catalog. Existing core Text still prints full source metadata and selected origins/spans; a future renderer can use the manifest without weakening bundle validation.
- Only existing digest-bound `hydrated-sidecar.v1` records are supported, always CALLER_ASSERTED/NON_AUTHORITATIVE. Arbitrary kind labels remain opaque claims, not provider adapters/native CALLS. Receipt handles are not sidecar record selectors. Other main/sidecar families fail existing admission explicitly.
- Validator re-admits original externally supplied input/selection, compares a fully re-derived plan, separately enumerates native graph coverage/receipt foreign keys, and calls existing core Validate. It never calls Hydrate or HydrateFocused. Shared native admission/typed projection is not a second independent parser or authentication proof. Rehashed dropped requested IDs/sites, wrong policies, unknown-as-resolved and authority changes reject.
- Core origin limit additionally bounds requested focus occurrences, expanded sites (including no-receipt sites), and expanded receipt selections. Focus input is capped at 4 MiB and 1024 bytes per ID; whole result JSON obeys MaxOutputBytes. No new paging algorithm or wall-clock/RSS guarantee.

## Original FR20 replay and measured bytes

Unchanged committed copy: `internal/hydratedevidence/testdata/focused-fr20.v2.json`.
Original: `/tmp/lsp-trace-fr20-fr21-integration/final-artifacts/TestFR20HydrationIntegration_fake-wire/graph-provenance.v2.json`.
Exact SHA-256: `8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf`.
The prior actual public CLI fake-wire run produced these bytes and deleted its checkout. This checkpoint copied them byte-for-byte and tested the digest; it did not regenerate/re-marshal the main graph, run a CLI/provider, or access the deleted source checkout. Separate disposable synthetic V1 fixtures do not replace this original fixture.

Actual selected relations:
- `sha256:e16c80b01aa9de78ff896b411d93746a6bfa749c1a80bc7b1c3bd251df81fa4b`: `/graph/edges/0/call_sites/0`, two exact caller receipts.
- `sha256:42df1c10ff307e342f79d0a3d3f28cf115a501f6274619801c612e8705bef707`: `/graph/edges/1/call_sites/0`, one exact caller receipt.

Final observed measurement (`TestFocusedActualFR20Measurement`):

| Quantity | Bytes/count |
|---|---:|
| Selected core Text, UTF-8 bytes | 6,032 |
| All-catalog core Text, UTF-8 bytes | 249,648 |
| Focus result JSON bytes | 13,582 |
| Unique selected span content bytes | 1 |
| Unique full retained source content bytes | 9 |
| Catalog sources / records | 5 / 272 |
| Selected core origins | 3 |

Both Text measurements include the full core source catalog and use retained-range mode with bodies enabled. All-catalog baseline selects every catalog record/source pair, with empty source ID for records without sources. Hash-based deduplication of content is a measurement convention only: identical nine-byte contents across receipts still retain all five identities. The source bytes, selected span bytes and rendered output bytes are not interchangeable. Whole-file opt-in separately recovers exact nine-byte retained receipt bodies and hashes with unavailable original coordinate authority.

## Stopping boundary / parent handoff

Mandatory package tests and targeted vet passed; implementation stopped without optional public expansion. Full FR21, D01 seven-edge/five-file trial, public CLI/MCP equivalence, public schema registration, output paging/publication, real provider sidecar adapters and deployment remain unqualified/deferred. D01 artifact path/bytes were not supplied. No universal reduction ratio, containing-function, source completeness, independent support, or analyzed-version claim is made.

Parent should inspect the committed diff and local derivation/evidence, then choose integration. Do not treat this ledger as full public feature or deployment approval.
