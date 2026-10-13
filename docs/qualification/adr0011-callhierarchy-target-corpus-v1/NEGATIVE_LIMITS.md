# Negative limits and non-claims

- No gopls request was made.
- No incoming or outgoing call relation was acquired.
- No document-symbol response was acquired; `DOCUMENT_SYMBOL_BOUND` is absent.
- No provider success, failure, coverage, or completeness conclusion is encoded.
- No target is removed because of a future provider outcome.
- No feature, runtime, reachability, ownership, purpose, architecture, or repository-completeness claim is made.
- Tracked tests, testdata, docs, tooling evidence, vendor paths, and generated files are accounted exclusions, not evidence that their declarations are absent or irrelevant.
- Build constraints are intentionally not evaluated; admission is a source-byte census, not a build graph.
- Source-path uniqueness is not substituted for the frozen identity fields; duplicate IDs fail generation.
- Dirty workspaces, wrong revision/tree, parser errors, malformed UTF-8 position prefixes, and unaccounted tracked Go files refuse output.
- Both directional manifests remain authority 0, `accepted=false`, completeness `UNKNOWN`, and production admission `REFUSED_MISSING_EVIDENCE`.
