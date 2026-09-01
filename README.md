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

`--down-depth` and `--up-depth` count call edges. Zero means no traversal in that direction. Slice output is v3-only and includes `slice.start_mode` plus metadata whose layers, frontier, outgoing terminals, upward starts, and outgoing relations reference the native graph's canonical node and relation IDs. `frontier_node_ids` is only the exact-depth layer. `outgoing_terminal_node_ids` contains successful empty `outgoingCalls` responses reached before that layer. `upward_start_node_ids` is their native-ID-sorted union. Null, failed, timed-out, canceled, or node-budget-truncated outgoing expansion is not a server-reported leaf. The result is a deterministic bounded projection of call-hierarchy information reported by the selected language server; it is not proof of runtime execution, complete feature coverage, or unreported dynamic, reflective, generated, configuration, template, or framework relationships.

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
- `--output PATH`: publish through owner-only custody files. On POSIX, the selector and generation files use mode `0600` and v3 generation directories use `0700`; on Windows, privacy follows native account access controls and no POSIX mode-bit guarantee is asserted. V3 makes `PATH` an atomic selector containing a generation basename; the selected private generation contains the graph and receipt. The selector and filenames remain visible to principals that can list the destination directory.
- `--schema v1|v2|v3`: select output schema; v3 is the default, while explicit v1/v2 retain their historical projections.
- `--pretty`: indent JSON output; default is compact JSON.

In v3, every original `invocation.seeds` entry has exactly one `seeds` result with the same label, including failures. Each result's `reached_node_ids` and `reached_relation_ids` are the deterministic causal closure for that exact seed occurrence. For slices, that closure follows bounded outgoing paths from the seed and bounded incoming paths only from upward-start nodes reached by those outgoing paths. Shared nodes and relations may belong to multiple seeds; failed seeds have empty membership; the global graph remains deduplicated and equals the union of successful memberships.

Retrieve the committed Draft 2020-12 contracts with `lsp-trace schema get --schema v1|v2|v3`. The command writes the exact schema bytes embedded in the binary. Validate a graph file or stdin with `lsp-trace validate [--schema v1|v2|v3] PATH|-`; the version is auto-detected from `schema_version` unless an explicit matching version is required. Structural schema validation runs first. V3 then runs the existing deeper semantic checks for graph references, digests, receipts, and evidence rules. Validation does not rewrite or canonicalize input.

Verify a published v3 bundle offline with `lsp-trace verify PATH`. Success prints only `verified integrity and custody` and exits 0; malformed selectors or missing, malformed, semantically inconsistent, or byte-mismatched selected generations print a concise error to stderr and exit 1. Verification recomputes the role-named `semantic_commitment_digest` and `exact_serialized_bytes_digest`. Schema validation and verification establish only their documented structural, semantic, and exact-byte properties; neither establishes authenticity, a signature, producer identity, source truth, or independent confirmation of hidden process inputs.

Publication marshals the complete graph before writing and syncs private staged data in the destination directory. V3 validates the semantic bundle, writes artifact and receipt into one immutable generation, re-reads the artifact, opens and syncs the complete generation directory, and atomically replaces only `PATH`, the generation selector. Generation directory basenames are opaque publication coordinates, not content identities and not part of the determinism contract. It then opens and syncs the destination directory; an open or sync failure is returned even if the selector rename has already made the generation visible. Verification resolves the selector once and rejects malformed selectors plus incomplete, malformed, or mismatched selected generations. Explicit v1/v2 output remains a private atomic graph file without a v3 generation. On POSIX, published files are mode `0600` and generation directories are `0700`; on Windows, the tool relies on native account access controls and does not claim POSIX mode semantics. If publication fails after traversal, the complete marshaled graph is retained on stdout when writable, and the tool attempts a private strict-JSON failure record in the nearest existing target ancestor. It reports that record as retained only after syncing both the record and its containing directory. The record exposes the target path, error text, and `exact_serialized_bytes_digest`, which may be sensitive; its location and the publication error are reported on stderr, and the command fails.

Language servers run with the invoking user's permissions and may execute project build logic, restore dependencies, access the network, or emit sensitive data. V3 embeds no environment values or working-directory path, but records domain-separated identities for the effective inherited-plus-override environment and cwd; these hashes can confirm guesses and are not independently authenticated. The cwd digest uses the cleaned absolute process path and does not resolve symlink aliases, so lexical aliases of the same directory may produce different identities.

Field authority is bounded: provenance and requested invocation fields are caller-supplied; normalized identities, digests, memberships, counters, and receipts are tool-derived; capabilities, prepared targets, edges, discovery responses, diagnostics, and opaque data are server-reported; publication receipts establish only the documented integrity/custody commitments. Use only trusted servers and workspaces.

The current implementation includes deterministic graph normalization, sequential reverse-BFS traversal, explicit terminals/frontiers, stdio JSON-RPC framing, LSP lifecycle handling, and the `incoming` CLI.

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

Run `./scripts/release-check.sh` for hermetic release-asset checks. Retained TypeScript and C# qualification runs pass. The strengthened ElixirLS cross-module qualification is retained as BLOCKED because the required aliased and fully qualified caller edges were not reported. The hermetic real-stdio fake-server suite runs in normal Go tests.
