# Public focused hydration checkpoint

## Derivation

Executing child: isolated worktree branched from `9f85e18` as
`public-focused-hydration`; no delegation. Parent should copy this claim to
`/tmp/lsp-trace-public-hydration/CLAIM.md` when integrating: the child obeyed the
higher-priority restriction to write only inside its isolated worktree.

The child loaded nn skills/capture discipline and local-delegation/routing
protocols, read the full supplied independent review and required focused/core
paging/validation/Text/docs/inspect surfaces, listed Bar skills, loaded workflow
and composition contracts, and ran its own exact local build:

```
bar build craft --subject "Public focused hydration wrapper over approved HydrateFocused and ValidateFocused; offline CLI/MCP parity, visible manifest outcomes, strict input/privacy, snapshot paging, ownership registration, persistent real-process RED before production and bounded GREEN before commit."
```

Route: craft, stated-properties, executable, no orchestration. Initial discovery
mistakes (`bar workflow show craft`, then treating the pack as a token) were
reported and corrected through `bar lookup craft`; the actual governed build
used the named pack, not a manually guessed expansion. Bar expanded it to
`make witness ground gate falsify atomic`. Witness/full phrase loads were
corrected after their authoritative output was retrieved. No claim of an
independent implementation review or perfect retrospective protocol repair.

Retained properties: (1) one shared offline CLI/MCP operation with validated
manifest and explicit FULL/PAGE context; (2) closed typed preflight and bounded
explicit file reads, never source fallback; (3) separate privacy/whole-file/
endpoint opt-ins and native/asserted authority; (4) stateless snapshot-bound
paging and full focused reassembly; (5) legacy inspection and narrow ownership.

RED procedures were persisted before production: `89080ee` adds actual CLI and
MCP subprocess assertions on the unchanged committed FR20 fixture and the five
root ownership cases. `ec669a2` retains `public-hydration-red.log`:

- `PUBLIC_CLI_VALIDATED_FOCUS FAIL: exit status 1 flag provided but not defined: -hydrated`
- `PUBLIC_MCP_VALIDATED_FOCUS FAIL` with unknown canonical tool.
- Each of the five exact root files rejected as unowned; unowned negatives passed.

`2a9d77e` persists additional guards before counterfactual applications. It is a
guard-only intermediate commit depending on the following implementation; merge
the complete branch, not that intermediate commit alone.

## Implemented boundary and exact APIs

CLI: `inspect ARTIFACT --hydrated --node ID/--relation ID` (repeatable), separate
`--sidecar FILE` / `--sidecar-record ID`, `--include-bodies`, `--whole-file`,
`--endpoint-context`, `--position-encoding`, eight bounded core-policy flags,
`--json`, `--page`, `--cursor`. Human full view is default; paging requires JSON.
No catalog/request-file/publication flag is claimed.

MCP: new explicit `lsp_trace_v1_inspect_hydrated`, no alias. Exact JSON artifact
text and sidecar JSON strings, typed focus selectors/policy, separate source
opt-ins, and optional page/cursor. No host paths, provider adapters, acquisition,
output_selector, loose floating-point parameter round-trip, or diagnostic JSON
payloads. Current runtime tool count 21; historical manifest 13 and previous
retained-calls composition 20 are unchanged.

Both transports use `operation.InspectHydratedHandler`. Semantic implementation:
`hydratedinspection.Decode` -> `Inspect` -> `hydratedevidence.HydrateFocused` ->
`ValidateFocused`; paged output calls existing `NewSnapshot`/`Page`. Every page
repeats the full manifest and effective focus/core request. The public cursor
wraps the core cursor and manifest digest, preventing changed unknown selections
from reusing an otherwise identical core snapshot. No hidden cache is used.

Output is the operation-specific closed contract `lsp-trace.inspect-hydrated.v1`,
with schema ID
`https://jaresty.github.io/lsp-trace/mcp/schemas/output-inspect-hydrated.v1.schema.json`.
`InputSchema`, `OutputSchema`, `ValidateInputJSON`, and `ValidateOutputJSON` are
owned in `internal/hydratedinspection/schema.go` and registered in the MCP
contract layer. No standalone generic CLI/MCP validate family is advertised.

`ValidateFull(originalRequest, view)` requires independently retained original
artifact/sidecar bytes and effective focus parameters. `Reassemble(originalPageRequest,
allViews)` requires the complete ordered page sequence; it calls core Reassemble
then focused validation. A single page's schema validity does not establish full
focused coverage. Original input bodies are intentionally not re-embedded into a
privacy-disabled output to manufacture self-contained validation.

`FocusedText` validates and safely quotes manifest-only unknown/zero-site/unmapped
outcomes before rendering the same core Text machinery with only selected source
metadata. The full source catalog is preserved in machine evidence. Core
`Bundle.Complete` is never interpreted as aggregate focus completeness.

## Inputs and observed outputs

Primary unchanged fixture:
`internal/hydratedevidence/testdata/focused-fr20.v2.json`, 154,035 bytes,
SHA-256 `8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf`.
The original source checkout is absent. No graph is reconstructed for this
measurement or the real-process parity tests. Exact relations:

- `sha256:e16c80b01aa9de78ff896b411d93746a6bfa749c1a80bc7b1c3bd251df81fa4b`
- `sha256:42df1c10ff307e342f79d0a3d3f28cf115a501f6274619801c612e8705bef707`

Observed: core Text 6,032 bytes; focused human Text 5,480 bytes including manifest;
5 catalog sources, 3 selected origins, 3 spans. Every selected core span text is
preserved. Public paging with core max_page_bytes 4096 reassembles 3 pages.
Ranges, whole-file exact bytes/hashes, endpoint opt-in, mixed native/asserted
receipt bindings, unknown IDs even when Bundle.Complete=true, no NON_SOURCE
missing warnings, and privacy-disabled secret absence are tested.

A separately labeled V1 fixture is derived offline from the original V2 graph
and already-retained capture bytes using existing native canonicalization and
Census. It changes one edge to zero sites and binds a V1 single-at invocation;
there is no acquisition or source read. It demonstrates visible NO_CALL_SITES
while another selected node makes Bundle.Complete=true. It is not called the
original FR20 replay. Existing legacy CLI/operation inspect tests also passed.

## RED/GREEN exact commands and evidence

All Go executions used:
`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOTELEMETRY=off`.
No broad/race/provider-heavy suites ran concurrently; final packages used `-p 1`.

Initial RED:
```
go test ./cmd/lsp-trace-mcp ./internal/integratedconformance -run 'TestHydratedPublicOffline|TestFR20FR21Ownership' -count=1 -v
```
Exit 1, retained in root `public-hydration-red.log`.

Final GREEN:
```
go test -p 1 ./internal/hydratedinspection ./cmd/lsp-trace ./cmd/lsp-trace-mcp ./internal/mcp ./internal/mcpcontract ./internal/operation ./internal/integratedconformance ./internal/hydratedevidence -run 'TestPublic|TestHydratedCLI|TestInspect|TestHydratedPublic|TestRegistry|TestComputedRouting|TestCanonicalDescriptions|TestHydratedContractAdditive|TestHydratedOperationPreflight|TestFR20FR21Ownership|TestDisabledIntegratedConformance/ASSERT_PACKAGE_OWNERSHIP_ONLY|TestFocusedActualFR20Measurement|TestFocusedTypedJoin|TestFocusedUnknownDuplicate|TestFocusedCatalogNonSource|TestFocusedPrivacyRangesVersions|TestFocusedEndpointPolicy|TestFocusedTampering|TestFocusedSidecarStates' -count=1 -v
```
Exit 0, all eight packages passed; exact output in
`testdata/public-final-green.log`. The ownership-only integration subtest builds
but does not launch the fake executable. No other integration subtest runs.

Final numeric strengthening (reject `1.00000000000000001` before it can round):
```
go test -p 1 ./cmd/lsp-trace-mcp -run '^TestHydratedPublicControls$' -count=1 -v
```
Exit 0, `testdata/public-numeric-green.log`. Real-process controls include private,
ranges, whole, endpoints, sidecar and page parity; MCP malformed requests fail
before semantic work, CLI stale cursor rejects, complete MCP pages reassemble.

Bounded vet and hygiene:
```
go vet -p 1 ./internal/hydratedinspection ./internal/operation ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace ./cmd/lsp-trace-mcp
git diff --check
git diff 9f85e18 --name-only -- internal/retainedcalls sessionruntime internal/provider providers
```
All passed; last command produced no paths. `testdata/public-vet.log` is empty
(success). Required package ownership checks passed with the wrapper changes dirty.

## Counterfactuals, minimization and adequacy boundary

`testdata/public-mutations.log` records exact per-overlay `go test -p 1 -overlay
<file> <package> -run '^<test>$' -count=1 -v` commands, assertion-specific failures,
then the same unmodified guards passing. The disposable runner/overlays were
removed; the committed Go guards and observations persist.

- [2] Remove closed request schema check: PUBLIC_CLOSED_PREFLIGHT fails on null
  node_ids, then passes when restored.
- [3] Enable default bodies: PUBLIC_PRIVACY fails, then passes when restored.
- [1] Remove mandatory producer ValidateFocused: distinct manifest and core
  PUBLIC_PRODUCER_VALIDATION failures; explicit producer-error case still passes.
- [4] Remove manifest binding from public cursor: PUBLIC_CURSOR_BINDING fails on
  a different unknown ID while core request/bundle remain otherwise compatible.
- [1] Remove focused source metadata filter: PUBLIC_CORE_TEXT fails on unrelated
  bookkeeping; restored core=6032/focused=5480 passes.
- [5] Remove only focused-final-tests.log ownership: exactly that registration
  fails; other registered paths still pass; restoration passes.

These reject six attempted smaller implementations. They are bounded witnesses,
not a claim that every individual test assertion has a separate mutation pair or
that every resource/semantic distinction has exhaustive adequacy coverage.
Original process RED and current GREEN provide the public CLI/MCP boundary pair.
No full FR21 coverage/completeness sentinel or independent-review PASS is claimed.

## Ownership and shared integration conflicts

P3 fixed by exact root filenames only: focused-final-tests.log,
focused-mutation.log, focused-red.log, focused-hydration.claim.md, focused-vet.log.
No all-root-log rule. New unowned.log/focused-other.log/unowned.claim.md negatives
remain rejected. Public wrapper package and exact CLI/operation files are narrowly
registered; existing unowned-public-hydration negative paths remain unchanged.

Shared conflict files needing parent review:
- cmd/lsp-trace/inspect_command.go
- cmd/lsp-trace-mcp/main.go (one handler registration)
- internal/hydratedevidence/wire.go (same renderer; selected metadata filter)
- internal/mcp/{registry.go,transport.go,bounded_analysis.go}
- internal/mcpcontract/{contract.go,operation_validator.go}
- internal/integratedconformance/{fr20_fr21_ownership_test.go,harness_test.go}
- Runtime tool-count tests in cmd/lsp-trace-mcp and internal/mcp, including ONLY
  the numeric runtime-count assertion in normalized_provider_contract_test.go.

No retainedcalls implementation, runtime implementation, provider implementation,
provider fixtures, acquisition implementation, PRD or product repository changed.
Git worktree listing showed the main worktree on feature/normalized-relations-provider
at 9f85e18. The child did not inspect its concurrent uncommitted retainedcalls
optimization outside isolation; parent must verify/retain that active work during
integration. This is an explicit verification limit, not an assertion it is absent.

## Limits and deferred gates

Public combined artifact/sidecars cap 1 MiB, request JSON cap 4 MiB, inline response
including newline cap 1 MiB, human text cap 1 MiB, complete public page sequence cap
64 MiB, 64 sidecars; core limits also apply. Repeated manifest/request metadata
must fit each response. MaxPageBytes governs the core page, not the wrapper total.
No source filesystem fallback, streaming/cancellation/wall-clock/RSS guarantee,
authentication, automatic parser/provider boundaries, or publication mechanism.
The existing scoped input opener rejects FIFO/nonregular handles and escapes;
pre-open final symlinks are rejected. No stronger filesystem snapshot promise.

Immutable publication: NOT_IMPLEMENTED initial gap; output_selector rejected.
Advanced catalog: core-only; no public flag claimed. Generic validate/schema_get
families are not silently broadened to pretend standalone focused validation.
D01: pending, original artifact unavailable. Named gopls run: unqualified.
Full repository, race, provider-heavy/native-process, cross-platform and deployed
qualification gates are deliberately left to the parent. No network, installs,
product access, deployment, or subagent delegation occurred.
