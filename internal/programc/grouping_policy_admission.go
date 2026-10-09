package programc

import (
	"errors"
	"reflect"

	"lsp-trace/internal/programcadmission"
)

var errTypedSelection = errors.New("invalid typed selection")

func computeTypedWithGroupingGate[A any](
	in programcadmission.TypedInput,
	seed uint64,
	admission A,
	verify func(A) bool,
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	return computeTypedWithGroupingAdmissionAndProjection(in, seed, admission, verify, prepareTypedProjection, kernel)
}

func computeTypedWithGroupingAdmissionAndProjection[A any](
	in programcadmission.TypedInput,
	seed uint64,
	admission A,
	verify func(A) bool,
	prepare func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error),
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	if typedInputCallsOnly(in) {
		return computeTypedWithKernel(in, seed, kernel)
	}
	if verify == nil || !verify(admission) {
		return TypedComputation{}, groupingPolicyUnqualifiedFailure()
	}
	projection, selection, err := prepare(in)
	if err != nil {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	return formTypedComputation(in, seed, projection, selection, kernel)
}

func recomputeTypedWithGroupingGate[A any](
	in TypedComputation,
	admission A,
	verify func(A) bool,
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	return recomputeTypedWithGroupingAdmissionAndProjection(in, admission, verify, prepareTypedProjection, kernel)
}

func recomputeTypedWithGroupingAdmissionAndProjection[A any](
	in TypedComputation,
	admission A,
	verify func(A) bool,
	prepare func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error),
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	if !validTypedComputation(in) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	if typedInputCallsOnly(in.source) {
		return recomputeTypedWithKernel(in, kernel)
	}
	if verify == nil || !verify(admission) {
		return TypedComputation{}, groupingPolicyUnqualifiedFailure()
	}
	projection, selection, err := prepare(in.source)
	if err != nil || !typedProjectionMatchesCarrier(projection, in) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	return reformTypedComputation(in, projection, selection, kernel)
}

func prepareTypedProjection(in programcadmission.TypedInput) (typedProjectionInput, typedSelection, error) {
	projection, err := programcadmission.PreparePrivateLeidenProjection(in)
	if err != nil {
		return nil, 0, err
	}
	selection, ok := inferTypedSelection(projection.SelectedKinds())
	if !ok {
		return nil, 0, errTypedSelection
	}
	return projection, selection, nil
}

func formTypedComputation(
	in programcadmission.TypedInput,
	seed uint64,
	projection typedProjectionInput,
	selection typedSelection,
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	outcome, failure := kernel(projection, selection, seed)
	if failure != nil {
		return TypedComputation{}, failure
	}
	methodEvidence, methodCommitment := typedProjectionMethodEvidence(projection)
	out := TypedComputation{
		cachedOutcome:    CloneOutcome(outcome),
		source:           cloneTypedInputPublic(in),
		seed:             seed,
		occurrences:      projection.Occurrences(),
		methodEvidence:   methodEvidence,
		selectedKinds:    projection.SelectedKinds(),
		pairWeights:      projection.PairWeights(),
		methodCommitment: methodCommitment,
	}
	if len(out.methodEvidence) == 0 {
		out.methodCommitment = nil
	}
	out.seal = typedComputationSeal(out)
	return out, nil
}

func recomputeTypedWithKernel(
	in TypedComputation,
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	projection, selection, err := prepareTypedProjection(in.source)
	if err != nil || !typedProjectionMatchesCarrier(projection, in) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	return reformTypedComputation(in, projection, selection, kernel)
}

func reformTypedComputation(
	in TypedComputation,
	projection typedProjectionInput,
	selection typedSelection,
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	outcome, failure := kernel(projection, selection, in.seed)
	if failure != nil {
		return TypedComputation{}, failure
	}
	out := cloneTypedComputation(in)
	out.cachedOutcome = CloneOutcome(outcome)
	return out, nil
}

func typedProjectionMatchesCarrier(projection typedProjectionInput, in TypedComputation) bool {
	methodEvidence, methodCommitment := typedProjectionMethodEvidence(projection)
	return reflect.DeepEqual(projection.Occurrences(), in.occurrences) &&
		reflect.DeepEqual(methodEvidence, in.methodEvidence) &&
		reflect.DeepEqual(projection.SelectedKinds(), in.selectedKinds) &&
		reflect.DeepEqual(projection.PairWeights(), in.pairWeights) &&
		reflect.DeepEqual(normalizeMethodCommitment(methodEvidence, methodCommitment), in.methodCommitment)
}

func typedProjectionMethodEvidence(projection typedProjectionInput) ([]programcadmission.MethodOccurrenceEvidence, []byte) {
	withEvidence, ok := projection.(interface {
		MethodEvidence() []programcadmission.MethodOccurrenceEvidence
		MethodCommitment() []byte
	})
	if !ok {
		return nil, nil
	}
	return withEvidence.MethodEvidence(), withEvidence.MethodCommitment()
}

func typedInputCallsOnly(in programcadmission.TypedInput) bool {
	if len(in.Occurrences) == 0 {
		return false
	}
	for _, occurrence := range in.Occurrences {
		if occurrence.Kind != programcadmission.TypedCalls {
			return false
		}
	}
	return true
}
