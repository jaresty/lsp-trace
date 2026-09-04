# Provider Admission and Semantic Binding claim

Status: CLAIMED
Baseline: 4ce6e6d
Frame: Provider Admission and Semantic Binding

## Goal

Provide deterministic, host-authority-only admission against immutable `internal/provider.Provisioned` declarations and capabilities, plus a provider-neutral `provider.SemanticAdapter` that strictly decodes observation envelopes, verifies request/provider/protocol/adapter/custody consistency, delegates semantic interpretation to `internal/observationadapter`, and returns the exact payload consumed by incoming and slice collectors.

## Claims read before mutation

- `/tmp/bar-frame-work-lsp-trace-bootstrap-composition-4ce6e6d-v1/FRAMEWORK.md`
- `host-provider-provisioning.claim.md`
- `production-collector-wiring.claim.md`
- `real-process-conformance.claim.md`
- `observation-adaptation.claim.md`

## Exact ownership

New files only:

- `internal/provider/admission.go`
  - deterministic `AdmissionResolver` backed only by `Provisioned` declarations/capabilities
  - closed selection for `auto`, `none`, and registered stable identities
  - relation-capability admission and ambiguous-auto rejection
- `internal/provider/admission_test.go`
  - assertion-specific admission guards
- `internal/provider/semantic_binding.go`
  - production implementation of the existing `provider.SemanticAdapter` boundary
  - strict observation-envelope decoding
  - request/provider/protocol/adapter/custody consistency validation
  - delegation to `internal/observationadapter`
  - exact collector composition-payload projection
- `internal/provider/semantic_binding_test.go`
  - assertion-specific strict binding, mismatch, delegation, and payload guards
- `provider-admission-semantic-binding.claim.md`
  - this ownership and evidence record

No existing file or symbol is claimed. In particular, bootstrap parsing, MCP composition, incoming/slice collector composition, provider process transport, observation semantics, graph semantics, Ember/Glimmer parsing, and NAIS files remain unmodified and unclaimed.

## Behavioral dimensions

1. Admission is deterministic and depends only on host-provisioned declarations and canonical capabilities.
2. Selection accepts exactly `auto`, `none`, or a registered stable provider identity; caller paths, commands, arguments, directories, environment, and executable authority are never accepted.
3. `none` selects no provider; an explicit identity must support the requested relation; unsupported relations fail closed.
4. `auto` succeeds only when exactly one provisioned provider supports the requested relation; zero or multiple candidates fail closed, including ambiguous declaration ordering.
5. Semantic binding strictly decodes one observation envelope and rejects unknown fields, trailing values, malformed JSON, and unsupported envelope versions.
6. Binding verifies request identity, provider identity/version, protocol identity/version, adapter identity/version, relation, seed/session context, and original custody before semantic adaptation.
7. Binding delegates provider-reported observation semantics to `internal/observationadapter`; it does not parse or infer Ember/Glimmer semantics.
8. Successful binding returns the exact adapted composition payload expected by both incoming and slice collectors and preserves immutable custody and contributor identities.
9. Bootstrap parsing and MCP composition remain unchanged.

## Enforcement sequence

1. Commit this claim as the first repository mutation.
2. Add assertion-specific admission and semantic-binding guards with compile-valid present-but-wrong surfaces.
3. Capture focused red output naming each retained assertion.
4. Implement the smallest provider-neutral admission and binding files.
5. Rerun the identical focused guard green, then full tests, `go vet ./...`, and `git diff --check`.
6. Verify forbidden-path diffs are empty, update this claim with exact red/green evidence and commit IDs, and commit a clean worktree.

## Required assertion identities

- `ASSERT_PROVIDER_ADMISSION_CLOSED_SELECTION`
- `ASSERT_PROVIDER_ADMISSION_RELATION_CAPABILITY`
- `ASSERT_PROVIDER_ADMISSION_AUTO_UNAMBIGUOUS`
- `ASSERT_PROVIDER_ADMISSION_DETERMINISTIC`
- `ASSERT_PROVIDER_SEMANTIC_ENVELOPE_STRICT`
- `ASSERT_PROVIDER_SEMANTIC_IDENTITY_CONSISTENT`
- `ASSERT_PROVIDER_SEMANTIC_CUSTODY_CONSISTENT`
- `ASSERT_PROVIDER_SEMANTIC_DELEGATES_OBSERVATION_ADAPTER`
- `ASSERT_PROVIDER_SEMANTIC_COMPOSITION_PAYLOAD_EXACT`
- `ASSERT_PROVIDER_NEUTRAL_NO_EMBER_PARSING`

## Red evidence

The committed compile-valid permissive surfaces were exercised with:

`go test ./internal/provider -run 'Test(Admission|SemanticBinding)' -count=1 -v`

Result: `Go test: 0 passed, 9 failed in 1 packages`. The output independently named all nine behavioral assertion identities; `ASSERT_PROVIDER_NEUTRAL_NO_EMBER_PARSING` is structural and is enforced by owned-source and forbidden-path diff inspection.

## Implementation

- Added an immutable `ProvisionedAdmissionResolver` that canonicalizes copied host declarations, accepts only omitted/`auto`, `none`, or one registered stable identity, rejects caller-shaped authority and unsupported/duplicate relations, requires all requested capabilities, and rejects zero/multiple auto candidates.
- Added `ObservationSemanticAdapter`, a concrete `provider.SemanticAdapter`, with strict single-value JSON decoding; request, receipt, provider, protocol, adapter, relation, request-ID, and original custody checks; delegation to `internal/observationadapter`; and deterministic incoming/slice provider-receipt projection.
- The implementation imports no framework parser and contains no Ember/Glimmer vocabulary.
- Bootstrap parsing, MCP composition, incoming/slice composition, transport, graph semantics, and observation adaptation were not edited.

## Evidence

- Focused green after strengthened distinction coverage: `Go test: 18 passed in 1 packages`.
- Provider package: `Go test: 36 passed in 1 packages`.
- Provider race package: `Go test: 36 passed in 1 packages`.
- `go vet ./...`: pass, no diagnostics.
- `git diff --check`: pass, no diagnostics.
- Forbidden-path diff over bootstrap parsing, MCP composition, incoming, and slice: empty.
- First full pre-commit run: `Go test: 3192 passed, 7 failed, 2 skipped in 40 packages`; four failures are the frame-declared production acceptance reds, two are the dirty-worktree ownership guard, and the isolated slice managed-process test passed on immediate rerun.
- Clean-state full: `Go test: 3204 passed, 4 failed, 2 skipped in 40 packages`; only `ASSERT_PRODUCTION_MCP_INCOMING_NONCALLS_REAL_PROVIDER`, `ASSERT_PRODUCTION_MCP_SLICE_NONCALLS_REAL_PROVIDER`, `ASSERT_PRODUCTION_OMISSION_ZERO_PROVIDER_START_EXACT_GRAPH_V3`, and their parent test remain red, as assigned to later frames by `FRAMEWORK.md`.
- Post-commit focused: `Go test: 18 passed in 1 packages`.
- Final implementation commit: this commit — `feat: bind provider admission and observations`.
