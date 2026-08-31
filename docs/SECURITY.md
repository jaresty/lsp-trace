# Security and trust boundaries

`lsp-trace` starts a caller-selected language server with the invoking user's permissions. A server may execute project build or restore logic, read workspace files, access the network, and emit secrets through stderr or opaque `CallHierarchyItem.data`. Use only trusted server binaries and workspaces; sandboxing is the deployer's responsibility.

The client passes arguments directly without a shell, performs no automatic network access, and does not invoke Git or repository hooks. V3 records command arguments and values explicitly supplied through `--server-env`; it never snapshots the ambient process environment. Explicit server environment declarations may contain secrets.

`--trace-lsp PATH` writes an opt-in JSON Lines protocol transcript with deterministic sequence numbers and directions. Transcripts contain raw sent and received JSON-RPC payloads and may expose source text, paths, identifiers, opaque `CallHierarchyItem.data`, or secrets; protect and redact them before sharing. The CLI does not include environment values in the transcript. Graph output and captured server stderr may also be sensitive. Output and transcript files are created with mode `0600`; stdout remains subject to the caller's redirection and logging environment. Review retained qualification evidence before sharing it.

V3 sensitivity policy covers invocation arguments, explicit environment values, workspace/source/output paths, opaque node data, diagnostics, captured server stderr, and trace transcripts. Automatic redaction is false. Bundle custodians are responsible for access control, retention, and any redaction before sharing. Artifact and sidecar mode `0600` protect local files only; stdout, backups, and copied files require separate controls. Receipts provide integrity/custody checking, not authenticity or signatures.

Report vulnerabilities privately to the repository maintainers rather than publishing exploit details in an issue.
