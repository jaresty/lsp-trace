# Final Stage 3 claim

Stage 3 is implemented on `pi-agent-138bf7fa-b8e2-497` rebased onto `dfbf3a4`, preserving the bounded local analytics history.

The request-lifecycle projector now requires exact nonempty finalized graph-provenance V3 bytes and binds their schema, exact length, and SHA-256 digest, with optional publication digest support. The V3 acquisition CLI retains only manager-issued diagnostic handles, obtains an exact certified snapshot set, and publishes the lifecycle document only after public bytes are fixed and successfully emitted or published. Private failure remains secondary and generic.

The private root/relative selector remains V3-only; V1/V2 reject it. Publication reuses the hardened root-descriptor-relative atomic 0600 no-replace sink. Startup diagnostics remain available. No public MCP operation, public schema-registry entry, collector input, public opt-in member, or separate artifact source was added.

The offline private validator accepts a lifecycle path plus its exact public V3 path and verifies the binding before lifecycle semantics. Its historical one-path behavior remains compatible.

Focused retained evidence:

- normal: `go test ./internal/requestlifecycle ./sessionruntime ./cmd/lsp-trace -run 'RuntimeProjection|PrivateRequest|FR23|RoundTrip' -count=1` — 20 passed in 3 packages;
- race: `go test -race ./internal/requestlifecycle ./sessionruntime -run 'RuntimeProjection|RoundTrip' -count=1` — 16 passed in 2 packages;
- real CLI/fake-LSP process: `go test ./cmd/lsp-trace -run 'TestFR23BuiltCLIPrivateRequestFailureDiagnostic' -count=1` — 1 passed;
- vet: `go vet ./internal/requestlifecycle ./sessionruntime ./cmd/lsp-trace` — no diagnostics;
- build/compile: `go test ./internal/requestlifecycle ./sessionruntime ./cmd/lsp-trace -run '^$' -count=1` — completed.

The process guard verifies initialize/documentSymbol readiness, a matched prepare deadline path, exact private/public binding, correlation, generic stderr, unchanged valid public V3 stdout, no raw URI, and no private file without the selector pair. Existing focused runtime guards cover unmatched-before-matched handling, late responses, malformed framing, process exit, write failure, bounds, and omission semantics.

No full suite, full race, network access, installation, deployment, product action, public MCP invocation, schema-registry publication, or live D01 run was performed or claimed.
