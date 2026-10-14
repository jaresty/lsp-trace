# Causal evidence

## Wrapper behavior tests

`internal/adr0011adr0007composition/compose_test.go` contains these qualification tests:

- `TestWrapperBaselineNoOptionalInputExactADR0007Forwarding`
- `TestWrapperPinnedEvidenceChangesOnlySeparateCompositionCandidate`
- `TestWrapperIncomingOutgoingRemovalIsolation`
- `TestWrapperDirectionFailureNonPositivePreservesPeer`
- `TestWrapperUnorderedMarksStableAndProviderOrdinalsPreserved`
- `TestWrapperPinTamperAndIdentityEscalationRefused`
- `TestWrapperBoundsAndCancellation`
- `TestWrapperRepeatedIdenticalByteStable`

The tests verify absent-input exact ADR0007 forwarding, wrapper-only candidate effects, incoming/outgoing removal isolation, failure preservation, unordered normalization, artifact/profile/evidence/ordinal identity tamper refusal, MaxReferences boundary behavior, cancellation precedence, work bounds, and byte stability.

## Semantic mutation gates

`run_mutation_gates.py` applies disposable source mutations to `internal/adr0011adr0007composition/compose.go`, expects targeted tests to fail, restores the source after each mutation, and re-runs a baseline test. Captured results are in `logs/mutation-gates.log`.

Expected-fail gates recorded:

- `remove_incoming_membership_branch` -> membership test fails
- `remove_order_branch` -> order/action test fails
- `remove_failure_preservation` -> failed-direction preservation test fails
- `remove_pin_check` -> package/artifact/pin test fails
- `remove_ordinal_identity_check` -> mark-only ordinal identity adversary fails
- `remove_max_references_enforcement` -> MaxReferences boundary test fails
- `remove_identity_restraint` -> identity escalation test fails
- `remove_idempotence_normalization` -> reorder stability test fails
- `remove_cancel_precheck` -> pre-ADR0007 cancellation test fails

The final log ends with restored baseline success.

## Verification commands

- `go test ./internal/adr0011adr0007composition -run 'TestWrapper' -count=1`
- `go test ./internal/adr0011adr0007composition ./internal/adr0007sourcetextsearchv4private -count=1`
- `go test -race ./internal/adr0011adr0007composition -count=1`
- `go vet ./internal/adr0011adr0007composition ./internal/adr0007sourcetextsearchv4private`
- `git diff --check`

All final verification commands exited 0.
