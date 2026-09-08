# FR20 public acquisition v2 (experimental source implementation)

This additive adapter exposes the existing acquisition coordinator and graph-provenance v2 capturer. It is not an analytics/export adapter, a deployment claim, FR20 completion approval, or qualification against a real product repository. Omitted acquisition version remains v1, including historical artifact bytes, errors and tool names.

## Public boundary

| Surface | Producer | Input | Output |
|---|---|---|---|
| CLI | `slice --acquisition-version v2` | bounded regular `--seed-manifest` file plus managed server/profile options | graph-provenance/v2 |
| CLI | `incoming --acquisition-version v2` | same manifest; per-target up depth | graph-provenance/v2 |
| MCP | `lsp_trace_v2_slice` | structured `seed_manifest`, session ID/alias, exact generation | graph-provenance/v2 |
| MCP | `lsp_trace_v2_incoming` | same structured contract | graph-provenance/v2 |

Both producers invoke `acquisitionops.Executor`, which resolves the exact host-managed session, obtains its workspace and position encoding, uses the runtime's document-supply interface, runs **one** `acquisition.Acquire`, then calls `graphprovenance.CaptureV2`. Neither loops over legacy per-seed producers. Context IDs identify normalized effective requests, not authenticated acquisition events.

The MCP registry advertises these operations, their closed input/output schemas and an `acquisition_v2` compatibility record. `source_implementation=EXPERIMENTAL` is separate from `deployed_availability=UNKNOWN`. The record describes this public-acquisition subset, not a complete FR22 project-wide matrix. Public v2 export and analytics remain `NOT_IMPLEMENTED`; no graph-v3 wrapper or implied v1 consumer compatibility is advertised.

## Manifest

```json
{
  "schema_version": "lsp-trace.seed-manifest.v2",
  "coordinate_convention": "zero-based-session",
  "root": {
    "id": "root",
    "locator": {"uri": "file:///workspace/main.go", "line": 1, "character": 5},
    "down_depth": 2, "up_depth": 2
  },
  "required_targets": [
    {"id": "callee", "locator": {"uri": "file:///workspace/leaf.go", "symbol": "Leaf"}},
    {"id": "other", "locator": {"uri": "file:///workspace/other.go", "line": 1, "character": 5}}
  ],
  "limits": {"max_nodes": 100, "max_requests": 1000, "max_path_work": 100000, "max_evidence_bytes": 4194304, "timeout_ms": 5000, "request_timeout_ms": 1000, "max_response_bytes": 4194304, "max_messages": 64}
}
```

Rules are checked before CLI server launch or MCP runtime acquisition:

* Maximum input: 262144 bytes. Duplicate/unknown keys, nulls, missing version/convention/root/target-array, empty explicit symbol/language IDs, unsupported versions and conflicting selectors are rejected.
* At most 64 root plus required-target records, ordered as supplied, with unique nonempty bounded IDs; the primary ID must be `root`. Required targets may be an explicit empty array.
* Every locator requires an exact canonical absolute URI (local capture is restricted to workspace file URIs) and either a nonempty symbol or both nonnegative line and character. Symbol and position cannot coexist. No suffix/name-derived file selection, one-based conversion or arbitrary MCP manifest path exists. Repeated locator identities are permitted and retain separate requested records.
* Coordinates are zero-based in the selected session's negotiated encoding. Optional locator `language_id` overrides the managed runtime's default. CLI `--language-id` is that runtime default, not an inline locator.
* Every target, including the root, has `down_depth` and `up_depth` (default 2 each, range 0..64). In incoming mode only up depth governs traversal; down depth remains recorded. Explicit zero is preserved. There are no top-level depth fields.
* Shared defaults: nodes 100, requests 1000, path work 100000, evidence 4 MiB, timeout 5000 ms. Maxima are nodes 10000, requests 100000, path work 100000000, evidence 64 MiB and timeout 60000 ms. Explicit zero is preserved for node/request/path-work/evidence budgets; timeout must be positive. Wire limits default to request timeout 1000 ms, response 4 MiB and 64 messages (maxima 60000 ms, 16 MiB and 4096; all positive). Budgets do not multiply by target count. Capture has its own existing bounded envelope admission, not a claim that all source bytes fit inside the graph-evidence budget.
* CLI legacy `--at`, `--seed`, symbol/URI selectors and inline traversal/budget flags conflict with v2 manifest input. Unsupported acquisition versions fail before launch. Version-looking server argument **values** are not interpreted as CLI version options. Omitted and explicit `v1` use the historical command path.
* CLI regular input uses the existing scoped nonblocking reader: FIFO/nonregular, oversized and escaping final symlink inputs fail closed. The containing directory is host-selected CLI input; MCP accepts only the structured manifest.

Example (positional locators are more portable across servers):

```sh
lsp-trace slice --acquisition-version v2 --workspace /workspace \
  --server /installed/path/gopls --language-id go --seed-manifest /inputs/seeds.json
```

MCP uses a host-configured session alias, not caller-selected workspace/server authority:

```json
{"name":"lsp_trace_v2_incoming","arguments":{"session_id":"fixture","generation":1,"seed_manifest":{"schema_version":"lsp-trace.seed-manifest.v2","coordinate_convention":"zero-based-session","root":{"id":"root","locator":{"uri":"file:///workspace/main.go","line":1,"character":5},"up_depth":0,"down_depth":0},"required_targets":[]}}}
```

## Accounting and authority

Root failure does not erase other targets. Resolution outcome, graph admission, aliases, connection/path support and overall partial status remain separate. One selected item appearing twice has two requested target rows, not two invented graph items. Disconnected or unresolved targets, unsupported and ambiguous preparation, zero-hop paths and directed reachability retain the coordinator/kernel's v2 meanings; they do not imply complete workspace coverage.

Acquisition records exact native returned items, directed evidence, session/generation/encoding and supplied-document versions. Capture confines referenced files to the host workspace and retains separate repeated URI/version supplies. It does not assert which source the server analyzed: analyzed version is `UNVERIFIED`; dependency completeness is `UNKNOWN_INCOMPLETE`. Native data and exact generation numbers survive the MCP number-preserving decoder. Inline v2 validation preserves object input bytes before generic transport normalization.

## Validation, immutable publication and verification

Producers validate the graph-provenance/v2 family, not graph-v3. CLI `--output` and MCP `output_selector` publish the same admitted bytes through existing immutable publication custody. Inline and selected publication are not distinct evidence semantics.

```sh
lsp-trace schema get --family graph-provenance --version v2
lsp-trace validate --family graph-provenance --version v2 evidence.json
lsp-trace verify --family graph-provenance --version v2 publication-selector.json
```

MCP schema retrieval/validation keep their existing canonical names with explicit `schema: {"family":"graph-provenance","version":"v2"}`. Publication verification is explicitly `lsp_trace_v2_verify`, requiring that family/version and the selected-publication input. It verifies immutable bytes/digest/length plus v2 admission, not producer authenticity or source authorization. V1 verification retains its historical default/error contract. Existing source-independent `ValidateV2` replays retained evidence after source deletion.

## Bounded test evidence

`TestFR20PublicProcess` checks launched CLI/MCP byte parity, target/alias/zero-hop accounting, exact host workspace/generation, schema validation and publication/verification. `TestFR20GraphMatrix` covers both modes over connected/reversed, ambiguous-root, request/node/path/evidence-budget scenarios with seven requested records, unsupported/missing/isolated targets, directionally correct paths and offline replay. Invalid-manifest tests reject before launching a server; a FIFO process test checks bounded failure. Direct executor tests check defaults, explicit zeros and invalid manifests before runtime access. Registry/decoder tests cover closed schemas, source-vs-deployment claims and exact large integer transport.

Historical-byte qualification builds **f9981ba76ab6737a43d9abc50133ebb5d90dc1dc** CLI/MCP binaries, then runs `TestFR20FrozenV1ProcessParity` with `LSP_TRACE_FR20_BASELINE_CLI` and `LSP_TRACE_FR20_BASELINE_MCP` pointing to them. It compares omitted-version CLI bytes/status/stderr, explicit-v1 CLI equivalence and MCP artifact bytes. It is opt-in because baseline binaries are not shipped.

`LSP_TRACE_FR20_GOPLS=/installed/path/gopls go test ./cmd/lsp-trace-mcp -run '^TestFR20InstalledGopls$' -count=1 -v` runs a disposable multi-file positional qualification in both modes. Installed gopls v0.23.0 passed connected/disconnected paths, four-file capture and source-gone validation/verification. A preceding symbol-name trial failed with `missing range` on flat SymbolInformation: **native name-selector qualification did not pass**. No new server support, dependency coverage, native full-volume or real-product qualification follows from the positional result.

RED evidence includes absent public/verification routes, empty selectors, version-looking server values, FIFO blocking and missing numeric-preservation routing. Adapter counterfactuals drop requested targets, enlarge shared requests and widen the canonical workspace; the ordered-accounting, global-bound and host-workspace assertions reject them respectively. Counterfactual edits are restored, not shipped. An accepted reduction hashes the normalized request directly instead of a second manifest-shaped context structure.

Full/race/vet/build/CI/release commands and exact run outcomes belong to the accompanying implementation claim. The dirty-worktree ownership guard now explicitly registers FR20/FR21 packages and named adapter/document/schema paths; unrelated paths remain rejected. `TestFR20HydrationIntegration` checks original public CLI v2 bytes through internal offline hydration with an explicitly synthetic caller-asserted sidecar; it adds no public hydration operation. Repository-root/symlink guards are likewise not weakened. This document is not independent-review approval.
