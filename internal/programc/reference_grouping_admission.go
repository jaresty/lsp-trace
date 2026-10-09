package programc

import (
	"bytes"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
)

type qualifiedGroupingAdmission struct {
	policyBytes            []byte
	policyID, policyDigest string
	policyByteLength       int
	inputBytes             []byte
	inputID, inputDigest   string
	inputByteLength        int
	epoch                  uint64
	policy                 referencegrouping.GroupingPolicy
	input                  referencegrouping.AdmittedInput
	qualification          referencegrouping.VerifiedQualification
}

func newQualifiedGroupingAdmission(
	typedInput programcadmission.TypedInput,
	policy referencegrouping.GroupingPolicy,
	input referencegrouping.AdmittedInput,
	qualification referencegrouping.VerifiedQualification,
	epoch uint64,
) (qualifiedGroupingAdmission, bool) {
	if !input.MatchesTypedInput(typedInput) || !qualification.Matches(policy, input, epoch) {
		return qualifiedGroupingAdmission{}, false
	}
	return qualifiedGroupingAdmission{
		policyBytes: append([]byte(nil), policy.CanonicalBytes()...), policyID: policy.ID(),
		policyByteLength: policy.ByteLength(), policyDigest: policy.Digest(),
		inputBytes: append([]byte(nil), input.CanonicalBytes()...), inputID: input.ID(),
		inputByteLength: input.ByteLength(), inputDigest: input.Digest(), epoch: epoch,
		policy: policy, input: input, qualification: qualification,
	}, true
}

func (a qualifiedGroupingAdmission) valid() bool {
	return a.qualification.Matches(a.policy, a.input, a.epoch) &&
		bytes.Equal(a.policyBytes, a.policy.CanonicalBytes()) && a.policyID == a.policy.ID() &&
		a.policyByteLength == a.policy.ByteLength() && a.policyDigest == a.policy.Digest() &&
		bytes.Equal(a.inputBytes, a.input.CanonicalBytes()) && a.inputID == a.input.ID() &&
		a.inputByteLength == a.input.ByteLength() && a.inputDigest == a.input.Digest()
}

func cloneQualifiedGroupingAdmission(in qualifiedGroupingAdmission) qualifiedGroupingAdmission {
	out := in
	out.policyBytes = append([]byte(nil), in.policyBytes...)
	out.inputBytes = append([]byte(nil), in.inputBytes...)
	return out
}

func computeTypedQualified(
	in programcadmission.TypedInput,
	seed uint64,
	policy referencegrouping.GroupingPolicy,
	input referencegrouping.AdmittedInput,
	qualification referencegrouping.VerifiedQualification,
	epoch uint64,
) (TypedComputation, *Failure) {
	return computeTypedQualifiedWithProjection(in, seed, policy, input, qualification, epoch, prepareTypedProjection, computeTypedProjection)
}

func computeTypedQualifiedWithProjection(
	in programcadmission.TypedInput,
	seed uint64,
	policy referencegrouping.GroupingPolicy,
	input referencegrouping.AdmittedInput,
	qualification referencegrouping.VerifiedQualification,
	epoch uint64,
	prepare func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error),
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	admission, ok := newQualifiedGroupingAdmission(in, policy, input, qualification, epoch)
	if !ok {
		return TypedComputation{}, groupingPolicyUnqualifiedFailure()
	}
	out, failure := computeTypedWithGroupingAdmissionAndProjection(in, seed, admission, func(got qualifiedGroupingAdmission) bool {
		return got.valid()
	}, prepare, kernel)
	if failure != nil {
		return TypedComputation{}, failure
	}
	out.groupingAdmission = cloneQualifiedGroupingAdmission(admission)
	out.seal = typedComputationSeal(out)
	return out, nil
}

func recomputeTypedQualified(in TypedComputation) (TypedComputation, *Failure) {
	return recomputeTypedQualifiedWithProjection(in, prepareTypedProjection, computeTypedProjection)
}

func recomputeTypedQualifiedWithProjection(
	in TypedComputation,
	prepare func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error),
	kernel func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure),
) (TypedComputation, *Failure) {
	if !validTypedComputation(in) {
		return TypedComputation{}, typedComputationIntegrityFailure()
	}
	admission := cloneQualifiedGroupingAdmission(in.groupingAdmission)
	if !admission.valid() {
		return TypedComputation{}, groupingPolicyUnqualifiedFailure()
	}
	out, failure := recomputeTypedWithGroupingAdmissionAndProjection(in, admission, func(got qualifiedGroupingAdmission) bool {
		return got.valid()
	}, prepare, kernel)
	if failure != nil {
		return TypedComputation{}, failure
	}
	out.groupingAdmission = admission
	out.seal = typedComputationSeal(out)
	return out, nil
}
