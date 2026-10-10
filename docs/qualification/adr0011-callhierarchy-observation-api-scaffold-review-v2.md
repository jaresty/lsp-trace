# ADR 0011 Call Hierarchy observation API scaffold design review v2

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `d6190bf2d7f12fd3d818c72308d6c9ac2de74d89`
- **Reviewed design:** `docs/qualification/adr0011-callhierarchy-observation-api-scaffold-design.md`
- **Corrected design SHA-256:** `5831da2ac8d1baff903b9d1592326643cb11f3f024a0feb0653fe3bc8294af63`
- **Preserved blocked review:** `docs/qualification/adr0011-callhierarchy-observation-api-scaffold-review.md`
- **Preserved blocked review SHA-256:** `6c50cc19706fc5e7c71b62e27ef8599daacc21df4ddfa205d5e48d43f4a41aad`
- **Review authority:** design review only; no implementation, behavior, evidence, qualification, production, or commit authority
- **Evidence boundary:** degraded filesystem-only review because exact-workspace managed-session lookup returned `APPLICABLE_SESSION_NOT_FOUND`
- **Verdict:** `ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO`

## Decision

The corrected design resolves both blockers in the preserved first review and retains every previously passing scaffold boundary. The proposed API is a compileable private shape for a later causal RED, but remains deliberately inert: supplying it cannot produce an owner, lease, observation, callback, terminal result, available accounting, diagnostic, reservation, or behavior change. Therefore it cannot make the later nonzero-observation RED pass.

This GO authorizes only a separately executed inert scaffold implementation within the frozen five-file write-set. It authorizes no implementation in this review, no behavior test, no observation behavior, no evidence or qualification change, no production admission, and no commit.

## Resolution of the prior blockers

### B1 resolved — exact typed JSON-RPC request key is preserved

The design now defines `callHierarchyJSONRPCRequestKey` as a closed kind plus kind-specific storage. String and number keys remain distinct: a string key stores the exact decoded string value, while a number key stores a defensive copy of the exact JSON number token bytes. The design prohibits display formatting, numeric coercion, normalized JSON, decode/remarshal substitution, and use of a lossy `lspwire.RequestKey`.

The complete request identity contains manager-owned workspace/session/generation custody and the exact typed request key. That request identity is carried by transaction identity and predecessor commitment and is required in the length-delimited preimages for predecessor, selected-item, handoff, request-frame, response-frame, aggregate transaction, and any future owner/lease commitment. Thus predecessor identity, exact key, and ordinal can be retained without collapsing the key into a transaction label or digest.

### B2 resolved — custody is manager-owned and cannot be caller-minted

Workspace, request, and transaction identities can be issued only by a private `Manager` method reading the exact managed session record under manager custody. The public shape proposed within the package has no constructor accepting raw workspace, session, generation, request-key components, strings, or numbers. Callers may pass only an already-issued lease or immutable commitment.

Construction refuses zero custody, stale generation, workspace/session/generation mismatch, foreign-manager tokens, malformed typed keys, and over-limit input before any request, predecessor, selected-item, handoff, frame observation, lease, commitment, or hash preimage can be formed. Mutable number-token bytes and all other nested byte sequences are defensively copied, and no accessor exposes the manager nonce or mutable identity storage. Package-private field visibility therefore does not become caller minting through the authorized constructor seam.

## Re-run scaffold design checks

### Compileable RED remains inert

The future causal RED can compile across typed request construction, the real manager round trip, result lease retrieval, and observation callback API. In the scaffold phase, `Manager.RoundTrip` ignores the private request, the result owner remains nil, lease acquisition returns false, and execution fails at the expected `observations=0, want observations>0` boundary. The scaffold forbids adding the causal behavior test, creating a placeholder or known-zero observation, or special-casing the test. A semantic RED therefore cannot pass.

### Generic and non-directional

The API contains no incoming/outgoing, caller/callee, `fromRanges`, occurrence, selector, normalizer, classifier, assembler, qualification, or relation-builder semantics. Profile, binding, predecessor, selected ordinal, selected bytes, and handoff bytes remain opaque. The method allowlist is supplied by the caller, copied immutably, exact-equality-only, and contains no built-in Call Hierarchy methods or sequence rule.

### Immutable and defensive

All slices and nested byte sequences are copied on construction and copied again at accessor or callback-view boundaries. Predecessor order is preserved without append mutation; selected-item and handoff commitments preserve exact bounded byte spans without semantic interpretation. Pointer inputs are copied into private owned values, and callback aliases cannot retain mutable owner storage after return.

### Accounting availability and finite limits

Bytes, objects, work, and diagnostics independently distinguish unavailable `{available:false,value:0}` from known zero `{available:true,value:0}`. Nonzero unavailable measures are invalid. Limits are positive, finite, independently hard-capped, and checked for overflow; there is no unlimited sentinel or cross-dimension borrowing. The scaffold returns unavailable accounting and performs no reservation, charge, release, or observation allocation during round trips.

### Closed terminals and null handling

Terminal values are a closed private enum with a closed validity switch. No arbitrary string conversion is accepted, and the scaffold returns no observation terminal because it returns no observation. Exact `null`, exact `[]`, unavailable payload, and other exact bytes remain distinct; the generic scaffold does not decide whether a later semantic owner maps either successful wire form to `SuccessfulEmpty`.

### Allowlist

The allowlist rejects empty methods, duplicates, non-finite or over-limit counts, preserves exact caller order and strings, and exposes no mutation. `permits` is exact equality only and has no directional or profile knowledge. Production behavior does not invoke it during this scaffold phase.

### Production unreachability

All proposed symbols and attachments are unexported. Existing request/result attachments are private and `json:"-"`. The design adds no public API, constructor, option, interface, CLI, MCP, schema, config, environment variable, build tag, registry, capability, profile catalog, `init`, production host wiring, or production constructor call. No owner is constructed and no observation path is registered. Production admission remains `REFUSED_MISSING_EVIDENCE` with authority unavailable, authority zero, accepted false, and completeness unknown.

### Exact write-set

The authorized implementation write-set is exactly these existing files:

1. `sessionruntime/owned_method_pair.go`
2. `sessionruntime/b4_owned_method_pair_lease.go`
3. `sessionruntime/sessionruntime.go`
4. `sessionruntime/owned_method_pair_test.go`
5. `sessionruntime/b4_owned_method_pair_lease_test.go`

No new source file, test file, fixture, package, generated artifact, evidence artifact, or other document is authorized. In particular, the four later behavior files named in the design remain untouched. Any need to modify another file invalidates this GO and requires rereview.

## Verification boundary

The repository HEAD exactly matched `d6190bf2d7f12fd3d818c72308d6c9ac2de74d89` and remained descended from the accepted assessment HEAD `8210109f3f71fd969fe3201935014d4226c90c13`. Before this review was created there was no tracked worktree diff; only the corrected design and preserved blocked review were untracked. Their SHA-256 values exactly matched those recorded above. A tracked-source symbol search found no scaffold implementation.

No Go tests were run because there is no implementation to compile or behavior to accept. This review does not claim source completeness, runtime execution, provider authentication, server-reported CALLS truth, profile authority, or production suitability.

## Gate

The exact disposition is:

```text
ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO
```

This token applies only to the corrected design at SHA-256 `5831da2ac8d1baff903b9d1592326643cb11f3f024a0feb0653fe3bc8294af63` on reviewed HEAD `d6190bf2d7f12fd3d818c72308d6c9ac2de74d89`. Any design change, expanded write-set, production reachability, behavior wiring, causal RED addition, evidence change, or commit requires a new gate.
