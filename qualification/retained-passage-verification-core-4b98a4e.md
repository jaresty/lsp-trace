# Retained passage verification core qualification

Semantic base revision: `1124b1ffa61b51d407d45aa36a82f89be8918acb`

Candidate provenance: `98dd71d88fb9711e60cabe45e0979aa5cac8f7c2`, originally based on `4b98a4e1d976775f43597f1c01e2fade68f95f5d`; repaired and requalified against the semantic base above.

Scope: `internal/passageverification` only. The core is transport-neutral and offline. It accepts exact artifact bytes or bytes plus a custody-admitted descriptor bound to the same digest and inspection ID. It invokes existing graph/schema, Graph Provenance, and hydrated-evidence admission and coordinate extraction. It admits ordinary graph.v3 for attribution-only checks, Graph Provenance V5, and validated `lsp-trace.graph-v5-source-snapshot.v1` carriers. It performs no filesystem reads, Git access, LSP calls, network access, live-source access, pinned-revision fallback, source reacquisition, publication, or authentication.

The result preserves separate artifact admission, artifact digest, selector custody, inspection, seed, membership, node identity, URI, range, source receipt, source digest, passage bytes, passage digest, and body-completeness dimensions. Body completeness is always `NOT_EVALUATED`. Overall status is fail-closed; graph.v3 can verify attribution but reports `SOURCE_BYTES_UNAVAILABLE`. Diagnostics are fixed non-content labels.

Bounds are 235 records, 192 MiB per artifact, and 192 MiB per batch. Limit failures return one result per submitted record; records are never truncated or reordered.

Qualification commands:

```text
go test ./internal/passageverification ./internal/hydratedevidence ./internal/hydratedinspection ./internal/inspection ./internal/graphprovenance ./internal/v5sourcesnapshot
go test -p 1 ./...
go vet ./...
./scripts/release-check.sh
GOOS=linux GOARCH=amd64 go test -c -o /tmp/passageverification-linux-amd64.test ./internal/passageverification
```

Deferred ownership: CLI and MCP adapters, Program C presentation/composition, and shared schema registration. No CLI, MCP, Program C, presentation, or shared registry wiring is included.
