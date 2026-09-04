# Unified cross-surface symbol identity claim

## Claim

The direct CLI slice composer and the managed slice executor must reconcile clangd's outgoing and incoming aliases through one narrow internal helper before merging their independently built graphs. The helper will live in `internal/graph/reconcile.go`, with assertion-first policy tests in `internal/graph/reconcile_test.go`; the direct consumer and its regression coverage are `cmd/lsp-trace/slice_command.go` and `cmd/lsp-trace/main_test.go`; the managed consumer and its regression coverage are `sliceops/executor.go` and `sliceops/executor_test.go`.

The governing goal is one deterministic native symbol and no duplicate relation when clangd presents the same semantic location differently across outgoing and incoming call-hierarchy responses, without globally redefining graph identity.

Reconciliation identity is exactly name, kind, URI, and selection range. Range, detail, and opaque data are not identity dimensions. A candidate is rewritten only when that semantic-location key selects exactly one canonical outgoing node; zero or multiple outgoing matches preserve the incoming identity as an ambiguity fallback. Opaque data is never authoritative for reconciliation or disambiguation.

Before merge, preserve incoming presentation in diagnostics when it differs, rewrite matched incoming node and edge IDs to the canonical outgoing presentation, then run the existing canonical merge. Preserve malformed outgoing `FromRanges` behavior and valid-range warnings; distinct selection ranges remain distinct. Existing depth behavior remains incomplete at 2 and 3 and complete at 4. Graphs, receipts, all twelve tools, and trusted-local contract wording remain deterministic and unchanged outside this narrow seam.

## Enforcement sequence

1. Exercise the clean `3a9b5f3` direct CLI and managed slice surfaces to capture the real alias shape.
2. Run `bar build make witness ground gate falsify atomic` literally and apply its instruction before implementation mutation.
3. Add assertion-first failing regressions for direct CLI and managed slice composition, plus focused helper guards for ambiguity, opaque data, and distinct selection ranges.
4. Perturb each assertion independently to prove it can fail for the intended reason.
5. Implement only the shared reconciliation helper and wire both consumers immediately before `graph.MergeResults`.
6. Re-run focused, repeated, race, full, full-race, vet, build, CI, docs, direct release, Linux compile-only, twelve-tool, trusted-local, and clean-tree gates.
7. Re-run real clangd direct CLI at depths 2, 3, and 4 and prove one root with no duplicate edges after correction; add a managed real-process check only if the bounded existing harness supports it.
8. Commit one clean change directly atop `3a9b5f3`, update this artifact with exact evidence and a `## Derivation` section, and do not push.

## Derivation

The clean baseline was `3a9b5f37658103db417b23958113b4201e0d92cd`. Apple clangd 21 reproduced the direct composition defect at depths 2, 3, and 4: each artifact contained two `root` nodes and a duplicate `root → first` relation. The managed assertion `ASSERT_MANAGED_SLICE_UNIFIED_SYMBOL_IDENTITY` independently failed with `nodes=3 edges=2`.

The narrow implementation adds `internal/graph/reconcile.go` and `internal/graph/reconcile_test.go`, invokes it immediately before merge in `cmd/lsp-trace/slice_command.go` and `sliceops/executor.go`, and adds managed consumer coverage in `sliceops/executor_test.go`. Matching uses exactly name, kind, URI, and selection range; only a unique outgoing match rewrites incoming node/edge references. The outgoing node presentation is retained while the displaced incoming item is serialized in an `INCOMING_SYMBOL_ALIAS_RECONCILED` diagnostic. Ambiguous and distinct-selection-range candidates remain unchanged, and opaque data differs in the passing semantic-location test.

Exact observed evidence before the final commit:

- Baseline packages: `Go test: 209 passed in 2 packages`.
- Managed red: `ASSERT_MANAGED_SLICE_UNIFIED_SYMBOL_IDENTITY: nodes=3 edges=2`.
- Focused green: `Go test: 4 passed in 2 packages`.
- Affected packages: `Go test: 311 passed in 5 packages`.
- Repeated: `Go test: 2840 passed in 3 packages`.
- Focused race: `Go test: 284 passed in 3 packages`.
- Corrected real clangd: depths 2, 3, and 4 each reported `roots=1 duplicate_edges=0`.
- The initial full-suite attempt reached the clean-tree ownership guard and failed only because the implementation was intentionally uncommitted: `unowned path "cmd/lsp-trace/slice_command.go"`.
- Clean committed full suite: `Go test: 2842 passed in 32 packages`.
- Clean committed full race: `Go test: 2842 passed in 32 packages`.
- `go vet ./...` and `go build ./...` passed.
- `scripts/check-ci.sh` passed format, test, vet, build, Python, shell, release, clean, and release-dry-run gates.
- `scripts/check-docs.sh` ended `DOCUMENTATION CHECK PASS` without modifying documentation.
- `scripts/release-check.sh` ended `RELEASE CHECK PASS`, including `PASS R-MCP-CONTRACT`, `PASS DOC-MCP-TWELVE-TOOLS`, trusted-local security wording checks, and direct release binaries.
- Linux compile-only passed with `GOOS=linux GOARCH=amd64 go build ./...`. An earlier `GOOS=linux go test -run '^$'` attempt was invalid because Go attempted to execute Linux binaries on Darwin and reported `exec format error`; it was replaced by the compile-only build.
