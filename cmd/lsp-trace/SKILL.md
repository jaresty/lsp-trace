---
name: lsp-trace
description: Trace authoritative incoming callers through an LSP server and optionally expose dispatch-family or top-level sibling discovery relationships.
---

# lsp-trace

Use `lsp-trace` when you need an evidence-preserving upward caller trace from one or more exact source positions.

## Retrieve this skill

```bash
lsp-trace skill get
```

The command prints this complete embedded document to stdout. Static retrieval is built in; dynamic skill discovery, listing, and installation are not currently provided.

## Trace incoming callers

```bash
lsp-trace incoming \
  --workspace /path/to/workspace \
  --server language-server \
  --at path/to/file.ext:LINE:COLUMN
```

`--at` is repeatable. `--seed-file` accepts labeled seeds. Lines and columns are one-based.

Important options:

- `--expand-dispatch-family` asks the server's Type Hierarchy for implementation-family members and emits `dispatch_relationships` separately from call edges.
- `--expand-topmost-siblings` asks for document symbols and emits top-level callable candidates in `sibling_candidates`; candidates do not imply calls, visibility, or equivalence.
- `--max-depth`, `--max-nodes`, `--timeout`, and `--request-timeout` bound traversal.
- `--output` writes graph JSON to a file; otherwise JSON goes to stdout. Diagnostics go to stderr.

## Interpret results honestly

- `edges` contain only server-reported Call Hierarchy caller relationships.
- `dispatch_relationships` are association evidence, not caller evidence.
- `sibling_candidates` are discovery candidates, not usage evidence.
- `traversal_complete` is scoped to server-reported Call Hierarchy under the requested limits.
- `source_graph_complete` remains `UNKNOWN`.
- Unresolved evidence marks traversal incomplete; dynamic-call evidence is advisory and never fabricates edges.

Exit code `0` means traversal completed within this server-relative scope. Exit code `2` means structured but incomplete output. Exit code `1` means invocation or unrecoverable server failure. Exit code `130` means interruption.
