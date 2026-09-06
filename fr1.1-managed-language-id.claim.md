# FR1.1 generic managed effective language identity

## Claim

Managed incoming and slice operations establish one effective document language before any document-symbol or call-hierarchy request. Explicit operation `language_id` takes precedence over configured bootstrap `language_id`, which takes precedence over the shared `internal/source.LanguageID` extension policy used by CLI traversal. Plain `.js` therefore resolves to `javascript`; unknown extensions resolve to no language and fail closed with `LANGUAGE_ID_UNAVAILABLE`/MCP `INPUT_INVALID` before any LSP document or hierarchy request.

The managed runtime validates file URIs against the session workspace, reads the synchronized bytes, and retains generation-local document state. First access sends version-1 `textDocument/didOpen`; unchanged reuse sends no synchronization; changed bytes send one full-content `textDocument/didChange` with the next version; an attempted language-identity change conflicts. A successful restart replaces document state with the new generation, so the first access emits a fresh version-1 `didOpen` and stale explicit/inferred conflicts cannot cross generations. A failed restart does not create a generation and retains the poisoned current generation's document state for lifecycle diagnosis. Incoming and slice preparation, document-symbol resolution, graph invocation evidence, and real server traffic use the same effective language. Static `callHierarchyProvider` admission and FR1 unsupported distinctions remain unchanged; no Angular-specific behavior exists.

## RED evidence

Baseline production revision: `1eb09048df850f59a50cd389a91edfb67e45e5b7`.

Before production mutation, `incomingops/managed_language_test.go` exercised a real `sessionruntime.Manager`, managed initialization, and managed incoming execution. Exact command:

```text
go test -json ./incomingops -run TestManagedPlainJavaScriptOpensWithEffectiveLanguageBeforePreparation -count=1
```

It failed only the named assertion:

```text
ASSERT_FR11_MANAGED_JS_EFFECTIVE_LANGUAGE_DIDOPEN: first_document_frame=textDocument/prepareCallHierarchy params={"textDocument":{"uri":"file:///workspace/calls.js"},"position":{"line":0,"character":0}}
```

The exact JSON event stream and command are retained in `qualification/retained/fr1.1-managed-language-id/red.json` and `red.command.txt`. Independent grep confirmed the assertion, first frame, and package `Action:"fail"` before production mutation.

Before the restart production edit, `TestManagedRestartOpensFreshDocumentWithGenerationLanguage` opened generation 1 explicitly as `typescript`, restarted successfully, and required generation 2 to infer `javascript` and send fresh version-1 `didOpen`. Exact command:

```text
go test -json ./incomingops -run TestManagedRestartOpensFreshDocumentWithGenerationLanguage -count=1
```

The retained `restart-red.json` failed only its named assertion with `generation_two_prepare={URI: LanguageID: Version:0 Failure:LIFECYCLE_CONFLICT}` and package `Action:"fail"`; `restart-red.command.txt` retains the command.

## GREEN evidence

- The identical focused managed assertion passed: `go test ./incomingops -run TestManagedPlainJavaScriptOpensWithEffectiveLanguageBeforePreparation -count=1` — 1/1.
- The identical focused restart assertion passed: `go test ./incomingops -run TestManagedRestartOpensFreshDocumentWithGenerationLanguage -count=1` — 1/1.
- Managed language/lifecycle plus slice tests passed: `go test ./sessionruntime ./incomingops ./sliceops -count=1` — 105/105.
- Successful restart tests prove fresh version-1 `didOpen` and stale explicit/inferred conflict removal; failed successor spawn proves generation 1 remains poisoned and retains its document map because no generation 2 exists.
- Contract, MCP/process, CLI, and managed parity passed: `go test ./internal/mcpcontract ./internal/mcp ./cmd/lsp-trace-mcp ./cmd/lsp-trace ./incomingops ./sliceops ./sessionruntime -count=1` — 525/525.
- Full and race suites passed: `go test ./... -count=1` and `go test -race ./... -count=1` — 3334/3334 each.
- CI, retained qualification, schema, custody, parity, omission, release, and GoReleaser snapshot/archive guards passed through the repository scripts and GoReleaser v2.
- Assertions cover inferred `.js`, unknown extension/no request, explicit-over-configured precedence, configured-over-inference precedence, lease reuse, full-content change/version increment, and language conflict.

## Real TypeScript language server evidence

`typescript-language-server --version` reported exactly `6.0.0`. Global TypeScript 7.0.2 lacks the tsserver entrypoint required by TLS 6.0.0, so TypeScript 5.9.3 was installed only in the disposable `/tmp/lsp-trace-fr11-managed-js` qualification workspace. No source repository was modified.

The real managed MCP request selected symbol `callee` in plain `calls.js`. Its exact refreshed retained response is `qualification/retained/fr1.1-managed-language-id/tls-6.0.0-green.jsonl`. A transparent disposable stdin tee inspected the exact wire request: version-1 `textDocument/didOpen` carried `languageId:"javascript"` before `textDocument/prepareCallHierarchy`. The response reports:

- invocation `language_id:"javascript"`;
- `call_hierarchy_provider:true` and `prepare_succeeded:true`;
- server-reported `caller` and `callee` nodes;
- `CALLER_TO_CALLEE` evidence and the caller call site `(4,9)-(4,15)`;
- `incoming_edges:2` (including the fixture's top-level call to `caller`), complete traversal, and no synthetic source-derived edge.

## Project replay availability

No environment-provided immutable source path or managed configuration was available for DASL, Survey Center, or Market View. Each replay is `UNAVAILABLE`. No capability, preparation, document-symbol, CALLS, Angular, or non-Angular project outcome is inferred.

## Derivation

1. FR1 established static capability admission, but capability support does not establish a usable managed document.
2. The real managed RED showed preparation was sent before any `didOpen`, proving the defect at the shared managed lifecycle boundary rather than in Angular or a specific language server.
3. CLI traversal already centralizes extension policy in `internal/source.LanguageID`; managed resolution therefore reuses that policy instead of duplicating extension tables.
4. Effective identity is resolved once at document preparation, stored with document generation state, sent in `didOpen`, and copied to invocation evidence consumed by incoming/slice paths.
5. Document synchronization state belongs to the process generation: successful restart must replace it alongside pending/request state, while failed restart must not fabricate a successor generation or erase the poisoned generation's diagnostic state.
6. Unknown policy results cannot safely become an empty LSP language ID or a guessed plaintext identity, so they terminate before any unsupported request.
7. Readable workspace bytes, canonical URI containment, deterministic reuse/versioning, language conflict, and session serialization preserve the managed lifecycle boundary across repeat requests, restart, and shutdown.
8. Real TLS 6.0.0 evidence establishes only server-reported static call hierarchy for the disposable plain-JavaScript fixture; it does not establish runtime execution, source-wide completeness, or any unavailable product replay.
