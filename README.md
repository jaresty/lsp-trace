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

Language servers run with the invoking user's permissions and may execute project build logic, restore dependencies, access the network, or emit sensitive data. Use only trusted servers and workspaces.

The current implementation includes deterministic graph normalization, sequential reverse-BFS traversal, explicit terminals/frontiers, stdio JSON-RPC framing, LSP lifecycle handling, and the `incoming` CLI.

## Qualification and release

TypeScript, C#, and Elixir fixtures and the PASS/BLOCKED/FAIL evidence contract are documented in [qualification/README.md](qualification/README.md). A BLOCKED external-server run remains a release blocker; fixture presence alone is not interoperability evidence.

- [Graph semantics](docs/SEMANTICS.md)
- [Security and trust boundaries](docs/SECURITY.md)
- [Schema compatibility policy](docs/SCHEMA_POLICY.md)
- [Release checklist](docs/RELEASING.md)

Run `./scripts/release-check.sh` for hermetic release-asset checks. External TypeScript, C#, and ElixirLS qualification and the complete fake-server scenario suite remain release blockers described in [PRD.md](PRD.md).
