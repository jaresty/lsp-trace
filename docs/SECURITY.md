# Security and trust boundaries

`lsp-trace` starts a caller-selected language server with the invoking user's permissions. A server may execute project build or restore logic, read workspace files, access the network, and emit secrets through stderr or opaque `CallHierarchyItem.data`. Use only trusted server binaries and workspaces; sandboxing is the deployer's responsibility.

The client passes arguments directly without a shell, performs no automatic network access, does not invoke Git or repository hooks, and does not log environment values by default. Values supplied through `--server-env` are inherited by the server process and may influence server behavior; the graph records the command and arguments but not those environment values.

`--trace-lsp` is not implemented, and the current CLI does not write a protocol transcript. Graph output, server stderr, and opaque `CallHierarchyItem.data` can still contain source text, paths, identifiers, or secrets and must be treated as potentially sensitive. Output files are created with mode `0600`; stdout remains subject to the caller's redirection and logging environment. Review retained qualification evidence before sharing it.

Report vulnerabilities privately to the repository maintainers rather than publishing exploit details in an issue.
