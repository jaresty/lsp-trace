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
- `--output` privately publishes graph JSON; v3 atomically selects one immutable generation containing graph and receipt, while explicit v1/v2 publish only their historical graph projection. POSIX uses owner-only modes; Windows relies on native account access controls without a POSIX mode claim. Otherwise pure graph JSON goes to stdout. Diagnostics go to stderr.
- `--schema v1|v2|v3` selects compatibility; v3 is default and explicit v1/v2 retain historical projections.
- `lsp-trace verify PATH` validates the exact-byte sidecar and embedded semantic receipt offline; success prints no graph.
- `--provenance-invocation-id`, `--provenance-caller`, `--provenance-source`, `--provenance-source-revision`, `--provenance-server-version`, `--provenance-timestamp`, and `--provenance-tool-version` add caller-supplied receipt metadata. Omitted values remain `UNKNOWN`; the tool never infers them from Git, the clock, the environment, or the server.

## Interpret results honestly

- `edges` contain only server-reported Call Hierarchy caller relationships.
- `dispatch_relationships` are association evidence, not caller evidence.
- `sibling_candidates` are discovery candidates, not usage evidence.
- `evidence_semantics` is the machine-readable claim ceiling. Call edges support only a server-reported caller-to-callee relation; they do not establish runtime execution, feature identity, whole-source completeness, or independent source confirmation.
- Discovery records use `evidence_class: "DISCOVERY_NOMINATION"` and `support_contribution: 0`. They nominate separate investigation and contribute no caller/callee support.
- `evidence_receipt` assigns domain-separated canonical `sha256:` identities to call, sibling, and dispatch relations using the caller-supplied source revision, direction, locator, evidence class, relation kind, and semantic endpoints. Separate memberships join each seed occurrence to its reached call relations and discovery evidence; discovery still contributes zero call support.
- In v3, `trace_receipt.content_digest` commits to canonical semantic content with the receipt omitted; the receipt in the atomically selected generation separately commits to exact serialized bytes. These establish integrity/custody only, not authenticity, signature, source truth, or runtime behavior.
- V3 identity labels caller provenance `CALLER_ASSERTED` and derives resolved seed URI/content digests plus an aggregate scoped `RESOLVED_SEED_CONTENTS`. Failed seeds are not source identities.
- V3 embeds no environment values or working-directory path. `process_context` records tool-derived, domain-separated identities for the effective inherited-plus-override environment and cwd, the environment-variable count, and explicit redaction state. Offline verification validates their bound structure but cannot independently confirm those execution facts without the original process context. Hashes may confirm guesses; custodians must still control access to arguments, explicit environment names, paths, opaque data, diagnostics, server stderr, and trace transcripts.
- `traversal_complete` is scoped to server-reported Call Hierarchy under the requested limits.
- `source_graph_complete` remains `UNKNOWN`.
- Unresolved evidence marks traversal incomplete; dynamic-call evidence is advisory and never fabricates edges.
- `UNKNOWN` provenance is an explicit evidence boundary, not permission to substitute the current timestamp, executable version, repository revision, or server version.

Exit code `0` means traversal completed within this server-relative scope. Exit code `2` means structured but incomplete output. Exit code `1` means invocation or unrecoverable server failure. Exit code `130` means interruption.
