# Security and trust boundaries

`lsp-trace` starts a caller-selected language server with the invoking user's permissions. A server may execute project build or restore logic, read workspace files, access the network, and emit secrets through stderr or opaque `CallHierarchyItem.data`. Use only trusted server binaries and workspaces; sandboxing is the deployer's responsibility.

The client passes arguments directly without a shell, performs no automatic network access, does not invoke Git or repository hooks, and does not log environment values by default. Protocol transcripts are opt-in and output, stderr, opaque data, and transcripts must be treated as potentially sensitive. Review retained qualification evidence before sharing it.

Report vulnerabilities privately to the repository maintainers rather than publishing exploit details in an issue.
