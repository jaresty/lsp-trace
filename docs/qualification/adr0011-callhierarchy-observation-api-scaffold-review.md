# ADR 0011 Call Hierarchy observation API scaffold design review

- **Repository:** `/Users/schwa/dev/lsp-trace-adr0011-c17`
- **Reviewed HEAD:** `d6190bf2d7f12fd3d818c72308d6c9ac2de74d89`
- **Reviewed design:** `docs/qualification/adr0011-callhierarchy-observation-api-scaffold-design.md`
- **Design SHA-256:** `fef15423af489675ad673714d69c2a60c5837995f9104a67c67f014b33fbc959`
- **Review authority:** design review only; no implementation, source, test, behavior, evidence, qualification, production, or commit authority
- **Verdict:** `ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_BLOCKED`

## Decision

The proposal is correctly bounded as an unexported, default-disabled, fail-closed, behaviorally inert scaffold. It does not authorize observations, production wiring, directional semantics, selection, normalization, occurrence construction, qualification, or any public/package expansion. Its proposed five-file implementation write-set is a strict subset of the accepted seven-source-file `sessionruntime` reuse boundary and existing focused tests.

The design cannot receive GO because its exact typed identity and predecessor API drops an immutable identity required by the accepted predecessor assessment, and it permits workspace identity to enter through a caller-constructed aggregate instead of an exact manager-owned workspace/session/generation join. Those defects prevent the proposed signatures from being the stable causal-RED scaffold they claim to be.

## Exact blocking defects

### B1 — immutable transaction and predecessor commitments omit the exact request key

The accepted reuse assessment and GO retain exact request-key/JSON-RPC-ID linkage as shared transaction custody. The corrected cross-transaction requirement explicitly requires predecessor identity, key, and ordinal. Current source also already exposes the transaction key as `RoundTripResult.Key lspwire.RequestKey`.

The proposed typed identity set has no request-key identity. `callHierarchyObservationIdentity` contains workspace, session, generation, profile, binding, transaction, and method, while `callHierarchyPredecessorCommitment` contains predecessor identity, transaction identity, method, request ordinal, and digest. Neither value can retain the exact request key/JSON-RPC ID, and no constructor signature accepts it.

A manager-local transaction identity is not declared or proven to be the exact wire request key and cannot silently replace it. A digest also cannot replace the omitted identity. As written, two separately keyed transactions can satisfy the same proposed commitment shape, so the scaffold cannot support the already accepted immutable predecessor identity/key/ordinal join.

Required correction: add a distinct private typed request-key identity, or an equally strict typed wrapper around the exact existing `lspwire.RequestKey`, to the transaction identity and every predecessor commitment that must preserve it. Define its zero/validity and defensive-copy rules and require it in the exact constructors used by the future causal RED. Do not collapse it into transaction identity, predecessor identity, method, ordinal, or digest.

### B2 — workspace custody is caller-constructible rather than manager-owned

The accepted predecessor boundary permits workspace identity only through the exact manager-owned workspace/session/generation custody that owns the transaction. Caller assertion, session identity alone, payload inference, profile identity, and adapter inference are explicitly insufficient.

The proposed `newCallHierarchyObservationRequest` accepts a fully populated `callHierarchyObservationIdentity` from its caller. The design supplies no manager-owned identity constructor, no unforgeable manager custody value, and no constructor signature that joins the workspace to the exact managed session and generation. Because all relevant types and fields are package-private rather than opaque across a package boundary, another `sessionruntime` caller can construct the aggregate directly or convert raw strings into each defined string type. The statement that workspace is manager-owned is therefore commentary, not an enforced API property.

The scaffold is inert, so this cannot create an observation today, but the stated purpose is to freeze signatures for a later causal RED. Freezing a caller-supplied workspace identity now would embed the prohibited provenance path into that API and force the later behavior phase either to trust it or to replace the scaffold signature.

Required correction: separate caller-supplied profile/binding/method policy from manager-derived transaction custody. Define the exact private constructor or manager-owned opaque custody input that supplies workspace, session, generation, transaction, and request key from the real manager transaction. The future causal RED must cross that exact manager-owned identity seam rather than provide an `exactIdentity` aggregate assembled by the test caller.

## Requirements that pass review

Subject to B1 and B2, the remaining design boundaries are acceptable:

- Workspace, session, generation, profile, binding, transaction, predecessor, method, and selected ordinal use distinct defined private types; none is an alias of another.
- The generic allowlist is caller-supplied, immutable, exact-equality-only, defensively copied, and contains no built-in incoming/outgoing methods or directional branch.
- Generic type and field names do not encode incoming, outgoing, caller, callee, `fromRanges`, or occurrence semantics.
- Request and response original-frame observations are separate types and preserve copied exact framed bytes, frame-relative spans, lengths, and SHA-256 values without remarshal substitution.
- Unavailable, exact `null`, exact `[]`, and other exact token bytes are distinct.
- Selected-item and handoff commitments are bounded byte-custody values only; they do not select, scan, decode, normalize, qualify, or construct occurrences.
- Slice and byte ownership is specified as defensive copying at construction and view/access boundaries.
- Terminal values are closed by a private enum and closed validity switch; the scaffold returns no observation terminal.
- Accounting independently distinguishes unavailable from known zero for bytes, objects, work, and diagnostics.
- Limits are positive, finite, independently hard-capped, and checked for overflow without an unlimited sentinel.
- Default and supplied requests remain inert: `Manager.RoundTrip` ignores the private request, no owner is built, the result pointer remains nil, lease acquisition returns false, no callback runs, and no observation or available accounting is produced.
- No production constructor, registration, CLI, MCP, schema, config, environment, build-tag, registry, capability, profile catalog, or public/exported route is proposed.
- No selection, normalization, occurrence construction, qualification, evidence role, admission, graph, grouping, Leiden, or semantic-vector behavior is proposed.
- The later causal RED shape would fail at lease acquisition with `observations=0, want observations>0`; the scaffold expressly forbids adding that behavior test or producing a placeholder observation.
- Scaffold tests are limited to API/value conformance and inert zero-result behavior. They do not run a manager transaction, assert produced observations, or claim enabled/disabled behavior equivalence.
- The proposed implementation write-set is exactly three existing source files and two existing test files: `sessionruntime/owned_method_pair.go`, `sessionruntime/b4_owned_method_pair_lease.go`, `sessionruntime/sessionruntime.go`, `sessionruntime/owned_method_pair_test.go`, and `sessionruntime/b4_owned_method_pair_lease_test.go`. This is a strict subset of the accepted seven-source-file boundary and adds no package, public surface, source file, test file, fixture, or generated file.

These passing properties do not cure the missing exact request key or caller-constructible workspace provenance.

## Verification boundary

The repository HEAD exactly matches the recorded full commit and descends from the accepted assessment HEAD `8210109f3f71fd969fe3201935014d4226c90c13`. The reviewed design hash exactly matches the value above. Before this review file was created, the design was the sole untracked file and there was no tracked worktree diff.

No Go tests were run: there is no scaffold implementation to compile or behavior to accept. This review checked design signatures and current source-visible type seams only. It does not claim source completeness, runtime execution, provider authentication, CALLS truth, exact profile authority, or production suitability.

## Gate

`ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_GO` is not issued.

The exact disposition is:

```text
ADR0011_CALLHIERARCHY_OBSERVATION_API_SCAFFOLD_DESIGN_BLOCKED
```

Unblock only by revising the design to preserve the exact typed request key in transaction and predecessor commitments and to make workspace/session/generation transaction custody originate from an exact manager-owned seam rather than a caller-constructed aggregate. A corrected design requires a new exact-hash independent review.

This review grants no implementation authority beyond design and authorizes no source, test, behavior, evidence, qualification, production, or commit action.
