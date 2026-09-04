# lsp-trace

A language-neutral CLI for recursively tracing incoming LSP Call Hierarchy relations.

## Build and test

```sh
go test ./...
go vet ./...
go build ./...
```

## Usage

```sh
go run ./cmd/lsp-trace incoming \
  --workspace /path/to/workspace \
  --server typescript-language-server \
  --server-arg --stdio \
  --at src/example.ts:12:8 \
  --pretty
```

Line and column values are one-based. Graph JSON is written to stdout (or `--output`), while diagnostics are written to stderr. Exit code `0` means complete traversal, `2` means a structured but incomplete traversal, `1` means invocation or unrecoverable server failure, and `130` means interruption.

## Bounded call-graph slices

Use `slice` to enumerate document symbols in one starting file, prepare every server-accepted callable, walk outgoing calls to an exact depth, and reuse the incoming traversal from the deterministic union of that frontier plus server-reported outgoing leaves reached before it:

```sh
lsp-trace slice \
  --workspace /path/to/workspace \
  --server language-server \
  --from-file src/example.ts \
  --down-depth 2 \
  --up-depth 8
```

Choose exactly one starting mode: `--from-file FILE` enumerates every server-preparable document symbol in one file; repeatable `--at PATH:LINE:COLUMN` uses explicit positions with automatic `seed-1`, `seed-2`, … labels; `--seed-file FILE` uses the existing labeled seed JSON format. Explicit starts are prepared once, deduplicated by native node identity for traversal, and retain each caller-supplied seed occurrence in `invocation.seeds` and `seeds`.

`--down-depth` and `--up-depth` count call edges. Zero means no traversal in that direction. Slice output is v3-only and includes `slice.start_mode` plus metadata whose layers, frontier, outgoing terminals, upward starts, and outgoing relations reference the native graph's canonical node and relation IDs. `frontier_node_ids` is only the exact-depth layer. `outgoing_terminal_node_ids` contains successful empty `outgoingCalls` responses reached before that layer. `upward_start_node_ids` is their native-ID-sorted union. Each upward start is depth zero for its own incoming walk, so `--up-depth` is measured independently from every node in that union; it is not measured from the original seed or from every outgoing-discovered node. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not a server-reported leaf. Hitting the node budget while an outgoing expansion remains unresolved is a conservative hard boundary: it makes traversal incomplete even if the request that was not completed might have returned no calls. Retained advisory range or normalization warnings may coexist with `traversal_complete: true`; that flag means the requested walks completed within their declared bounds, not that warnings are absent or that the graph proves runtime execution, complete feature coverage, source truth, or unreported dynamic, reflective, generated, configuration, template, or framework relationships.

## Inspect retained seeds

Inspect an existing v3 `artifact.json` or publication selector without constructing a replacement graph. Choose exactly one mode:

```sh
lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL --json
lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --json
```

Single-seed output is a `SEED_INSPECTION`; aggregate output is `ALL_SEEDS`. Both identify `lsp-trace.inspect.v1` and authority `NON_AUTHORITATIVE_DERIVED_VIEW`. The aggregate preserves stored invocation-seed order and failures, copies admitted native records once, and emits normalized per-seed reference collections plus mechanical accounting.

Selector inspection verifies complete-generation exact-byte custody before structural and semantic artifact validation. Direct artifacts receive structural and semantic validation, but their computed exact-byte digest makes no custody claim. Unknown labels, missing or duplicate same-label aggregate results, and invalid inputs fail without inspection JSON.

Inspection is deterministic and read-only. Global boundaries and diagnostics remain global; reached-node diagnostic indexes are correlations rather than per-seed custody or causation. Record and reference counts do not establish feature coverage, evidence sufficiency, runtime execution, domain meaning, or acceptance. Standalone `verify` remains an explicit selector custody audit.

## Compare retained seed evidence

After producing an `ALL_SEEDS` inspection document, compare exactly two stored seed labels without starting a language server or changing the input:

```sh
lsp-trace filter evidence-inspection.json --compare-seeds LEFT_LABEL --compare-seeds RIGHT_LABEL --json
```

The input must be a path to a structurally and semantically valid `lsp-trace.inspect.v1` `ALL_SEEDS` document. Exactly two distinct, known `--compare-seeds` values are required. The output is a `lsp-trace.filter.v1` `SEED_EVIDENCE_COMPARISON` containing typed references partitioned into `shared`, `left_only`, and `right_only`, plus selected-seed state, global completeness boundaries, and mechanical accounting. It does not copy native records; join references back to the admitted inspection input.

Comparison is deterministic, read-only, and limited to explicit per-seed references. Shared references do not prove shared purpose, and exclusive references do not prove distinct purpose. Failed or successful-empty seeds remain valid operands, so an empty partition is not a feature or workflow conclusion. The projection does not recover selector custody, authenticate its producer or source, establish runtime behavior, rank evidence, measure coverage, or decide identity, merge, split, confidence, or acceptance. See [ADR 0002](docs/adr/0002-deterministic-seed-evidence-filtering.md) for the versioned contract and deferred operations.

## MCP offline evidence server

`lsp-trace-mcp` is a local-development-only stdio MCP server. Configure an MCP client to start the executable directly; it accepts MCP JSON-RPC on stdin and writes MCP JSON-RPC to stdout. Its default surface has twelve canonical tools: six deterministic offline evidence tools, four local session lifecycle tools, and two bounded traversal tools. Launch it without filesystem publication, or pin selector publication beneath one private root:

> **WARNING:** Local LSP child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted. Run only developer-configured commands you trust.

```sh
lsp-trace-mcp
lsp-trace-mcp --publication-root /absolute/private/root
lsp-trace-mcp --bootstrap-config /absolute/path/bootstrap.json
```

Traversal requires a READY session. The host—not the MCP caller—provisions trusted sessions before stdio serving with a strict bootstrap file; MCP callers cannot select executable commands. The config path, executable path, workspace, and execution directory are absolute. Unknown fields, trailing JSON values, duplicate session identities, spawn failures, and readiness failures fail closed before MCP stdio is served; partial startup is rolled back within bounded shutdown.

```json
{
  "version": 1,
  "processes": [{
    "profile": {
      "trust_domain": "local-development",
      "workspace": "/absolute/path/to/workspace",
      "profile": "gopls",
      "environment_reference": "developer"
    },
    "execution": {
      "path": "/absolute/path/to/gopls",
      "directory": "/absolute/path/to/workspace"
    }
  }]
}
```

After startup, call `lsp_session_v1_list` to discover the exact READY `session_id` and `generation`; pass that selector to `lsp_trace_v1_incoming` or `lsp_trace_v1_slice`. Process configuration never enters MCP input schemas or session identity.

Stage 1 advertises six canonical tools and accepts one unadvertised compatibility alias for each:

| Canonical tool | Alias |
|---|---|
| `lsp_trace_v1_capabilities` | `lsp_trace_capabilities` |
| `lsp_trace_v1_schema_get` | `lsp_trace_schema_get` |
| `lsp_trace_v1_validate` | `lsp_trace_validate` |
| `lsp_trace_v1_verify` | `lsp_trace_verify` |
| `lsp_trace_v1_inspect` | `lsp_trace_inspect` |
| `lsp_trace_v1_filter` | `lsp_trace_filter` |

Call `lsp_trace_v1_capabilities` with `{}` to discover the process-lifetime tool availability, canonical names and aliases, immutable input/envelope/artifact schema IDs, publication support, and effective limits. The embedded `internal/mcpcontract/testdata/stage1-manifest.v1.json` plus its registered schemas are authoritative for tool/schema compatibility; each operation returns exactly one versioned envelope selected by `envelope_schema_id` in `structuredContent`, with empty non-mirroring MCP `content`.

Artifact-producing tools return exact bytes inline when they fit the 1,048,576-byte limit. A request whose result may exceed that limit should provide an `output_selector`; selector publication is available only when the process was started with `--publication-root`, and the selector must resolve beneath that pinned root. Publication uses exclusive no-replace owner-only files and returns a path-free receipt. The server never chooses or returns a private path. `list_page_max` is 100.

The default surface publishes exactly twelve canonical tools. MCP transport, envelopes, inline delivery, publication receipts, validation, inspection, filtering, and slice traversal do not upgrade graph authority, custody, authenticity, source truth, execution proof, feature identity, coverage, or acceptance. Stage 2 lifecycle tools (`lsp_session_v1_list`, `lsp_session_v1_status`, `lsp_session_v1_stop`, and `lsp_session_v1_restart`) plus bounded `lsp_trace_v1_incoming` and `lsp_trace_v1_slice` traversal are enabled by default and route to one process-local runtime. Both traversal tools require an exact READY session generation with retained call-hierarchy and position-encoding evidence. Slice performs exact-depth outgoing discovery, starts incoming traversal from the sorted deduplicated union of exact-depth frontier nodes and genuine successful empty outgoing leaves, and reports failed/null outgoing responses as incomplete rather than leaves. Its `max_messages` and `max_bytes` inputs bound each individual prepare, outgoing, and incoming LSP wire request; they are not aggregate budgets across the whole slice. Darwin uses local process-group supervision; unsupported platforms retain the twelve-tool surface but process-start-dependent behavior fails explicitly without starting a child. Child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted. This design makes no hostile-code safety, native containment, remote execution, or privileged-isolation claim. See [ADR 0003](docs/adr/0003-always-local-stage2.md).

## Pi direct tools with pi-mcp-adapter

Use the standard adapter rather than maintaining a repository-specific tool bridge:

```sh
pi install npm:pi-mcp-adapter
```

Restart Pi after installation. Preferred project config: `.mcp.json`. The host writes this file and the referenced bootstrap file; they are trusted configuration, not MCP call arguments.

```json
{
  "mcpServers": {
    "lsp-trace": {
      "command": "/absolute/path/to/lsp-trace-mcp",
      "args": [
        "--bootstrap-config",
        "/absolute/path/to/bootstrap.json"
      ],
      "directTools": [
        "lsp_trace_v1_capabilities",
        "lsp_trace_v1_schema_get",
        "lsp_trace_v1_validate",
        "lsp_trace_v1_verify",
        "lsp_trace_v1_inspect",
        "lsp_trace_v1_filter",
        "lsp_session_v1_list",
        "lsp_session_v1_restart",
        "lsp_session_v1_status",
        "lsp_session_v1_stop",
        "lsp_trace_v1_incoming",
        "lsp_trace_v1_slice"
      ],
      "toolPrefix": "none"
    }
  }
}
```

The list contains exactly the twelve canonical MCP names. `toolPrefix: "none"` keeps those names unchanged in Pi. Do not add a repository-local Pi extension or a second MCP bridge. Only the host-authored `.mcp.json` command, arguments, and bootstrap file choose executable, environment, or working directory. MCP callers receive the existing twelve tools and cannot override process configuration.

The warning above still applies: the adapter starts `lsp-trace-mcp`, which starts trusted local language-server children with the developer's permissions. They are not sandboxed and may access local files and network.

### Pi self-check

From the repository root, reconnect once so the adapter refreshes cached metadata, then inspect its direct tool list:

```text
/mcp reconnect lsp-trace
/mcp tools
```

Confirm that `lsp-trace` is connected and that the twelve names in the configuration appear once each. On the first run the adapter may initially expose only its proxy while metadata is cached; reconnecting refreshes and hot-loads the configured direct tools. A missing or extra name is compatibility drift: stop and run the repository checks before using the integration.

```sh
./scripts/check-docs.sh
go test ./internal/mcp -run TestLifecycleExecutorFamilyIsEnabledAndAdvertisedByDefault -count=1
```

### Pi lifecycle examples

First discover the host-provisioned selector; do not invent or assume a generation:

```text
lsp_session_v1_list {}
lsp_session_v1_status {"session_id":"SESSION_FROM_LIST","generation":1}
lsp_trace_v1_incoming {"session_id":"SESSION_FROM_LIST","generation":1,"uri":"file:///absolute/workspace/file.go","line":0,"character":0,"max_depth":8,"max_nodes":1000,"request_timeout_ms":30000,"timeout_ms":60000}
```

Use the exact current generation returned by list/status. A stale-generation diagnostic names the authoritative current generation; query status with that exact selector before retrying. Stop and restart are host-managed lifecycle intents:

```text
lsp_session_v1_stop {"session_id":"SESSION_FROM_LIST","generation":1,"caller_id":"pi-session"}
lsp_session_v1_restart {"session_id":"SESSION_FROM_LIST","generation":1,"caller_id":"pi-session"}
```

Cancellation stops the caller observation; an already accepted lifecycle intent may continue. Query current-generation status before retrying. A reap failure requires host-operator inspection and reaping rather than caller-supplied executable, environment, or directory controls. Partial or truncated traversal results remain honest bounded outcomes. Increase only the documented bounded parameter responsible for a reported frontier; never reinterpret partial evidence as complete.

## Named server profiles

Profiles are selected explicitly with `--profile NAME` on both `incoming` and `slice`; `language_ids` never selects a profile automatically. Without `--profile`, configuration files are not read and all legacy flags retain their existing behavior. `--server` may be combined with `--profile` and overrides the profile command.

Default discovery reads the user file at `$XDG_CONFIG_HOME/lsp-trace/config.toml` (or `~/.config/lsp-trace/config.toml`) and then the project file at `--workspace/.lsp-trace.toml`. `--config PATH` replaces both default files and requires `--profile`. Malformed TOML and unknown fields fail closed. Values merge by field: command-related CLI flags override the project profile field, which overrides the user profile field.

```toml
[profiles.typescript]
command = "typescript-language-server"
args = ["--stdio"]
env = ["NPM_TOKEN", "AUTH_TOKEN=${AUTH_TOKEN}"]
language_ids = ["typescript", "typescriptreact"]
```

Profile `env` entries are variable names or `KEY=${VAR}` exact references only. Values are read from the lsp-trace process environment at runtime; missing variables fail before server startup. Plaintext values are rejected. Graph invocation output records environment names/references, never values. For an explicitly selected profile, the first `language_ids` entry is the default document language ID; `--language-id` overrides it.

## Flags

Required flags:

- `--workspace PATH`: workspace root used for the LSP workspace URI and relative target resolution.
- `--server COMMAND`: language-server executable, started directly without a shell.
- `--at PATH:LINE:COLUMN`: target source position; line and column are one-based.

Optional flags:

- `--server-arg VALUE`: append one server argument; repeat for multiple arguments.
- `--server-env KEY=VALUE`: append or replace one server environment entry; repeat for multiple entries.
- `--language-id VALUE`: override the language ID inferred from the target extension.
- `--max-depth N`: maximum caller depth (default `100`); `0` is unlimited and negative values are rejected.
- `--max-nodes N`: maximum graph nodes (default `10000`); `0` is unlimited and negative values are rejected.
- `--timeout DURATION`: global command deadline as a Go duration (default `5m`); `0` is unlimited and negative values are rejected.
- `--request-timeout DURATION`: per-request deadline (default `30s`); it must be greater than zero.
- `--concurrency N`: request concurrency; the sequential MVP accepts only `1`.
- `--log-level LEVEL`: human diagnostic level: `error`, `warn`, `info`, or `debug` (default `warn`).
- `--trace-lsp PATH`: opt-in JSON Lines transcript of sent and received JSON-RPC messages with deterministic sequence numbers.
- `--expand-dispatch-family`: use LSP Type Hierarchy to emit implementation-family relationships separately from call edges.
- `--expand-topmost-siblings`: use hierarchical document symbols to emit top-level sibling candidates without call, visibility, or equivalence claims.
- `--provenance-invocation-id`, `--provenance-caller`, `--provenance-source`, `--provenance-source-revision`, `--provenance-server-version`, `--provenance-timestamp`, and `--provenance-tool-version`: attach caller-supplied receipt metadata; omitted values remain `UNKNOWN` and are never inferred.
- `--output PATH`: publish through owner-only custody files. On POSIX, the selector and generation files use mode `0600` and v3 generation directories use `0700`; on Windows, privacy follows native account access controls and no POSIX mode-bit guarantee is asserted. V3 makes `PATH` a replaceable atomic selector containing a generation basename, not the graph bytes; the referenced immutable private generation directory contains the graph and receipt. Pass `PATH` itself to selector-aware `verify` or `inspect`, which resolves the referenced generation. The selector and filenames remain visible to principals that can list the destination directory.
- `--schema v1|v2|v3`: select output schema; v3 is the default, while explicit v1/v2 retain their historical projections.
- `--pretty`: indent JSON output; default is compact JSON.

In v3, every original `invocation.seeds` entry has exactly one `seeds` result with the same label, including failures. Each result's `reached_node_ids` and `reached_relation_ids` are the deterministic causal closure for that exact seed occurrence. For slices, that closure follows bounded outgoing paths from the seed and bounded incoming paths only from upward-start nodes reached by those outgoing paths. Shared nodes and relations may belong to multiple seeds; failed seeds have empty membership; the global graph remains deduplicated and equals the union of successful memberships.

Retrieve the committed Draft 2020-12 contracts with `lsp-trace schema get --schema v1|v2|v3`. The command writes the exact schema bytes embedded in the binary. Validate a graph file or stdin with `lsp-trace validate [--schema v1|v2|v3] PATH|-`; the version is auto-detected from `schema_version` unless an explicit matching version is required. Structural schema validation runs first. V3 then runs the existing deeper semantic checks for graph references, digests, receipts, and evidence rules. Validation does not rewrite or canonicalize input.

Verify a published v3 bundle offline with `lsp-trace verify PATH`. Success prints only `verified integrity and custody` and exits 0; malformed selectors or missing, malformed, semantically inconsistent, or byte-mismatched selected generations print a concise error to stderr and exit 1. Verification recomputes the role-named `semantic_commitment_digest` and `exact_serialized_bytes_digest`. Schema validation and verification establish only their documented structural, semantic, and exact-byte properties; neither establishes authenticity, a signature, producer identity, source truth, or independent confirmation of hidden process inputs.

Publication marshals the complete graph before writing and syncs private staged data in the destination directory. V3 validates the semantic bundle, writes artifact and receipt into one immutable generation, re-reads the artifact, opens and syncs the complete generation directory, and atomically replaces only `PATH`, the generation selector. Generation directory basenames are opaque publication coordinates, not content identities and not part of the determinism contract. It then opens and syncs the destination directory; an open or sync failure is returned even if the selector rename has already made the generation visible. Verification resolves the selector once and rejects malformed selectors plus incomplete, malformed, or mismatched selected generations. Explicit v1/v2 output remains a private atomic graph file without a v3 generation. On POSIX, published files are mode `0600` and generation directories are `0700`; on Windows, the tool relies on native account access controls and does not claim POSIX mode semantics. If publication fails after traversal, the complete marshaled graph is retained on stdout when writable, and the tool attempts a private strict-JSON failure record in the nearest existing target ancestor. It reports that record as retained only after syncing both the record and its containing directory. The record exposes the target path, error text, and `exact_serialized_bytes_digest`, which may be sensitive; its location and the publication error are reported on stderr, and the command fails.

Language servers run with the invoking user's permissions and may execute project build logic, restore dependencies, access the network, or emit sensitive data. V3 embeds no environment values or working-directory path, but records domain-separated identities for the effective inherited-plus-override environment and cwd; these hashes can confirm guesses and are not independently authenticated. The cwd digest uses the cleaned absolute process path and does not resolve symlink aliases, so lexical aliases of the same directory may produce different identities.

Field authority is bounded: provenance and requested invocation fields are caller-supplied; normalized identities, digests, memberships, counters, and receipts are tool-derived; capabilities, prepared targets, edges, discovery responses, diagnostics, and opaque data are server-reported; publication receipts establish only the documented integrity/custody commitments. Use only trusted servers and workspaces.

The current implementation includes deterministic graph normalization, bounded slice composition, read-only seed inspection, sequential reverse-BFS traversal, explicit terminals/frontiers, stdio JSON-RPC framing, LSP lifecycle handling, and the `incoming` CLI.

## Read-only presentation

Render a direct V3 artifact or an existing immutable-generation selector without changing the retained evidence or selector custody:

```bash
lsp-trace render evidence.selector.json --format summary --detail compact
lsp-trace render graph.v3.json --format tree --detail full
lsp-trace render graph.v3.json --format mermaid
```

Formats are `summary`, `tree`, and `mermaid`; detail is `compact` or `full`. Output is a deterministic, non-authoritative presentation view. File URIs beneath the recorded workspace are displayed relatively; external and non-file URIs remain unchanged. Retained terminal/frontier causes are shown, with limit guidance only when the cause maps to a caller-controlled option.

## Embedded skill

Retrieve the binary's static agent skill without network access or installation:

```bash
lsp-trace skill get
```

Dynamic skill discovery, listing, and installation are deferred. The embedded skill also documents labeled seed files and a bounded recipe for turning trace JSON into provisional feature-inventory candidates.

## Qualification and release

TypeScript, C#, and Elixir fixtures and the PASS/BLOCKED/FAIL evidence contract are documented in [qualification/README.md](qualification/README.md). Server support is claimed only from retained PASS evidence containing the server version, capability result, command, graph, stderr, and assertions. A BLOCKED or FAIL external-server run remains a release blocker; fixture presence alone is not interoperability evidence.

- [Graph semantics](docs/SEMANTICS.md)
- [Security and trust boundaries](docs/SECURITY.md)
- [Schema compatibility policy](docs/SCHEMA_POLICY.md)
- [Release checklist](docs/RELEASING.md)
- [Pi direct-tool setup](docs/PI.md)

Run `./scripts/release-check.sh` for hermetic release-asset checks. Retained TypeScript and C# qualification runs pass. The strengthened ElixirLS cross-module qualification is retained as BLOCKED because the required aliased and fully qualified caller edges were not reported. The hermetic real-stdio fake-server suite runs in normal Go tests.
