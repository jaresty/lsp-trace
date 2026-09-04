# MCP Transport Claim

## Assigned boundary

Expose the existing shared custody operation through the MCP contract and stdio invocation path. Do not alter immutable V2/V3 artifact bytes and do not implement sibling CLI, operation-core, lifecycle, traversal, or analysis scopes.

## Governing goal

MCP clients can discover and invoke the shared custody operation through registry-backed schemas with the same logical result digest and schema identity as the shared operation. Complete results are returned inline only within the declared response limit; oversized results require caller-selected immutable publication and return a complete path-free receipt. No result is silently truncated.

## Intended files and symbols

Expected implementation/test surface, refined only where repository structure requires:

- `internal/mcpcontract/*`: registered custody input/envelope/artifact schema IDs and registry/manifest entries.
- `internal/mcp/registry.go` and focused tests: registry-derived discovery, alias/dispatch composition, schema identity, result transport, and publication behavior.
- `cmd/lsp-trace-mcp/main.go`, `cmd/lsp-trace-mcp/custody.go`, and focused/integration tests: inject the command-owned custody loader and exercise canonical stdio invocation.
- `internal/mcpcontract/testdata/*` if the authoritative manifest/schema fixtures require custody registration.
- Documentation only where existing contract assertions require the new canonical tool count/name.

Key symbols expected to participate include the MCP registry/dispatcher construction, `operation.NewVerifyHandler` or the corresponding shared custody handler, `commandCustodyLoader`, registered tool descriptors, envelope encoding, and publication receipt construction.

## Behavioral dimensions

1. **Registry-backed discovery:** the custody tool name, status, input schema, result/envelope schema, aliases, and publication modes come from the authoritative registry rather than an ad hoc handler list.
2. **Shared invocation:** MCP dispatch invokes the existing shared custody operation with command-adapter-owned byte loading; it does not duplicate verification or custody semantics.
3. **Schema parity:** discovery schema IDs, accepted arguments, structured envelope, and artifact schema identity match the registered contract.
4. **Logical-digest parity:** the MCP result reports the same logical digest produced from the exact shared-operation result model/bytes; transport wrapping does not change it.
5. **Immutable-byte preservation:** V2/V3 bytes are passed through exactly and never normalized, reformatted, upgraded, or rewritten.
6. **Oversized publication:** results above the declared inline limit require a caller-provided selector and configured publication root, publish with existing immutable/no-replace owner-only semantics, and return byte length plus digest without exposing a private path.
7. **No silent truncation:** absence or invalidity of required publication fails explicitly; no prefix, summary substitution, or undisclosed omission is returned as the complete result.
8. **Compatibility:** existing canonical tools and aliases retain behavior outside this assigned custody addition.

## Enforcement sequence

1. Add focused red tests for registry discovery/schema identity and canonical/alias dispatch.
2. Add red tests for exact-byte/logical-digest parity and oversized no-selector failure.
3. Add red tests for immutable publication receipts, byte length/digest, no-replace behavior, and absence of truncation.
4. Implement the smallest registry/schema/adapter wiring that satisfies those tests through the shared custody operation.
5. Run focused red-to-green tests, affected-package tests, and `go test ./...`.
6. Record artifacts/assertions/test evidence, commit, and residual risks below.

## Exclusions

- No changes to V2/V3 serialization or semantic meaning.
- No new custody algorithm, trust claim, authentication claim, or verification semantics.
- No arbitrary filesystem path authority in the transport-neutral operation.
- No pagination design, live LSP behavior, session lifecycle changes, CLI redesign, or sibling analysis operation.
- No silent truncation or transport-only summary standing in for authoritative bytes.

## Inherited implementation assumption

Yes. The current architecture constrains this assignment to registry-derived MCP exposure, transport-neutral shared operation invocation, command-adapter-owned custody loading/publication, and exact preservation of immutable V2/V3 bytes. Existing MCP publication and envelope machinery must be reused rather than forked.

## Completion evidence

### Artifacts

- `internal/operation/types.go`: `operation.Result.LogicalDigest` transports shared logical identity without coupling the operation package to MCP.
- `internal/operation/verify.go`: `NewVerifyHandler` returns the already-validated V3 `trace_receipt.semantic_commitment_digest` while returning `material.Artifact` unchanged.
- `internal/mcp/transport.go`: successful inline and publication envelopes propagate `LogicalDigest`; the existing inline size gate, caller selector requirement, and immutable publisher remain authoritative.
- `internal/mcpcontract/testdata/schemas/envelope-artifact.v1.schema.json` and `envelope-publication.v1.schema.json`: registered envelope contracts permit a versioned SHA-256 logical digest.
- `internal/mcp/transport_test.go`: focused inline/publication parity guard.
- `cmd/lsp-trace-mcp/process_integration_test.go`: real stdio canonical/alias verify parity against the fixture semantic receipt.
- `internal/integratedconformance/harness_test.go`: exact ownership allowance for this assignment’s shared-operation and claim files.

### Assertions and observations

- `ASSERT_VERIFY_LOGICAL_DIGEST_PARITY` failed with `got=""` before the shared handler populated the digest and passed after implementation.
- `ASSERT_MCP_CUSTODY_LOGICAL_DIGEST_PARITY` failed for both inline and publication envelopes before transport/schema wiring and passed afterward.
- `ASSERT_REAL_PROCESS_CUSTODY_LOGICAL_DIGEST_PARITY` passed in the complete implementation, failed under an isolated one-line omission perturbation (`canonical=<nil>`), and passed after restoration.
- Existing `ASSERT_MCP_CLI_ARTIFACT_BYTE_PARITY`, `ASSERT_EXACT_ARTIFACT_SCHEMA_ID`, oversize `OUTPUT_REQUIRES_SELECTOR`, immutable publication/no-replace, receipt byte length/digest, and no-content-on-failure guards remained green.

### Tests

- Baseline: `go test ./cmd/lsp-trace-mcp ./internal/mcpcontract ./internal/operation/...` — 113 passed.
- Focused shared/transport/schema guards — 8 passed across 3 packages.
- Affected packages: `go test ./internal/operation ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace-mcp -count=1` — 148 passed across 4 packages.
- Full repository: `go test ./...` — 2987 passed across 36 packages.

### Commit

Implementation commit: `e897912` (`feat: expose custody logical digest through MCP`).

### Residual risks

- `LogicalDigest` is optional at the generic envelope-schema level because non-custody artifact operations do not all expose the same logical identity model; real-process tests require it specifically for verify.
- The shared verify handler remains V3-only by its pre-existing semantic verification contract. V2/V3 bytes were not modified or reserialized by this change.
- Logical digest is a semantic integrity identity, not authentication, producer identity, source truth, or runtime proof.
