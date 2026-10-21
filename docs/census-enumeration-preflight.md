# Census enumeration preflight and pruning

`lsp-trace census --enumeration-preflight` performs only workspace enumeration. It runs after CLI parsing and validation but before resume handling, profile loading, runtime initialization, provider startup, document preparation, call hierarchy, acquisition, or publication. It therefore requires no `--server`, `--profile`, or `--publication-root`, and it cannot be combined with those controls or with `--resume`.

`--prune-dir DIR` is repeatable in preflight and normal Census runs. Each value must be a canonical workspace-relative directory: non-empty, clean, non-absolute, non-dot, non-escaping, slash-separated, and without a trailing slash. Values are sorted, deduplicated, and ancestor-minimized. A pruned directory is rejected before admission and does not consume enumeration work; overlapping source roots cannot re-enter it. Includes and excludes retain their existing semantics for paths outside pruned subtrees, and excludes still win.

The production enumeration limits are fixed: 100,000 discovered entries, 200,000 walker callbacks, 4,096 path bytes, depth 64, and the existing selected-file limit. Limit accounting is pre-admission and path-safe: exact diagnostics contain only a category, observed count, and limit, never a root or path.

Successful preflight writes one canonical `lsp-trace.census-enumeration-preflight-result.v1` JSON value containing normalized roots, includes, excludes, pruned directories, exact counts, a domain-separated digest of lexically sorted selected paths, limits, and a `COMPLETE` terminal. Its epistemic `completeness` is exactly `UNKNOWN`: terminal completion means only that the bounded walk finished, not that the evidence is source-complete. A typed enumeration-limit failure writes `lsp-trace.census-enumeration-diagnostic.v1`. The result has authority 0 and does not establish source completeness, provider completeness, runtime execution, feature identity, production authority, or absence from paths that were pruned, excluded, unavailable, generated, dynamic, reflective, or otherwise outside the bounded walk.

Normal Census requests with no pruned directories retain the byte-exact `lsp-trace.census-request-receipt.v1` representation. A request with pruned directories uses additive `lsp-trace.census-request-receipt.v2`, whose semantic identity includes the canonical minimized prune set. The v1 schema and historical artifacts are unchanged.

These schemas are available through the offline schema registry; this change adds no MCP operation and changes no public provider surface.
