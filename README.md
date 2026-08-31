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
- `--output PATH`: write JSON to a mode-`0600` file instead of stdout.
- `--pretty`: indent JSON output; default is compact JSON.

Language servers run with the invoking user's permissions and may execute project build logic, restore dependencies, access the network, or emit sensitive data. Use only trusted servers and workspaces.

The current implementation includes deterministic graph normalization, sequential reverse-BFS traversal, explicit terminals/frontiers, stdio JSON-RPC framing, LSP lifecycle handling, and the `incoming` CLI.

## Qualification and release

TypeScript, C#, and Elixir fixtures and the PASS/BLOCKED/FAIL evidence contract are documented in [qualification/README.md](qualification/README.md). Server support is claimed only from retained PASS evidence containing the server version, capability result, command, graph, stderr, and assertions. A BLOCKED or FAIL external-server run remains a release blocker; fixture presence alone is not interoperability evidence.

- [Graph semantics](docs/SEMANTICS.md)
- [Security and trust boundaries](docs/SECURITY.md)
- [Schema compatibility policy](docs/SCHEMA_POLICY.md)
- [Release checklist](docs/RELEASING.md)

Run `./scripts/release-check.sh` for hermetic release-asset checks. Retained TypeScript and C# qualification runs pass. The strengthened ElixirLS cross-module qualification is retained as BLOCKED because the required aliased and fully qualified caller edges were not reported. The hermetic real-stdio fake-server suite runs in normal Go tests.
