package inventoryworkflow

import (
	"context"
	"errors"
	"fmt"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
)

const predecessorRetainedCallerAsserted = "PREDECESSOR_RETAINED_CALLER_ASSERTED"

// BuildProgramCRepresentativeQualificationSubject is the private read-only
// composition boundary. It opens one state root, strictly verifies the existing
// WAITING_EVIDENCE predecessor head, obtains verified Program C evidence, and
// constructs the trusted subject from those two independently verified sides.
func BuildProgramCRepresentativeQualificationSubject(ctx context.Context, request Request) (subject programcrepresentative.ProgramCRepresentativeQualificationSubject, err error) {
	if ctx == nil || request.ProgramC.OpenState == nil || request.ProgramC.WorkflowKey == "" || request.ProgramC.HeadKey == "" || request.SuccessorWorkflowKey == "" || request.SuccessorHeadKey == "" {
		return subject, inventorysuccessor.ErrInvalid
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return subject, contextErr
	}
	root, err := request.ProgramC.OpenState()
	if err != nil {
		return subject, err
	}
	if root == nil {
		return subject, inventorysuccessor.ErrInvalid
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			subject = programcrepresentative.ProgramCRepresentativeQualificationSubject{}
			err = errors.Join(err, closeErr)
		}
	}()

	heads, err := inventorysuccessor.NewHeads(root)
	if err != nil {
		return subject, err
	}
	identity, _, err := inventorysuccessor.VerifyWaitingEvidenceHead(ctx, request.SuccessorWorkflowKey, request.SuccessorHeadKey, request.Identity, heads)
	if err != nil {
		return subject, err
	}
	programC, err := programctransitionhost.NewCompletionVerificationRequest(ctx, CompletionRequest{
		StateRoot: root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger,
		WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings,
	})
	if err != nil {
		return subject, err
	}
	evidence, err := censuscontinuation.BuildProgramCRepresentativeQualificationEvidence(ctx, programC)
	if err != nil {
		return subject, err
	}
	expected := evidence.QualificationSubjectExpectedContext()
	expected.WorkflowIdentity = qualificationWorkflowIdentity(identity)
	expected.SourcePinSHA256 = identity.SourcePinSHA256
	subject, err = programcrepresentative.NewProgramCRepresentativeQualificationSubject(expected)
	if err != nil {
		return programcrepresentative.ProgramCRepresentativeQualificationSubject{}, fmt.Errorf("%w: qualification subject: %v", censuscontinuation.ErrInvalidProgramCTransition, err)
	}
	return subject, nil
}

func qualificationWorkflowIdentity(in inventorysuccessor.WorkflowIdentity) programcrepresentative.QualificationWorkflowIdentityDTO {
	return programcrepresentative.QualificationWorkflowIdentityDTO{
		Profile: in.Profile, Contract: in.Contract,
		CensusReceiptSHA256: in.CensusReceiptSHA256, AvailabilityLedgerSHA256: in.AvailabilityLedgerSHA256,
		SourcePinSHA256: in.SourcePinSHA256, ProgramCPlanSHA256: in.ProgramCPlanSHA256,
		ProgramCBindingsSHA256: in.ProgramCBindingsSHA256, PrivacyPolicySHA256: in.PrivacyPolicySHA256,
		RetentionPolicySHA256: in.RetentionPolicySHA256, ResourcePolicySHA256: in.ResourcePolicySHA256,
		ReplayPolicySHA256: in.ReplayPolicySHA256, QualificationPolicySHA256: in.QualificationPolicySHA256,
		Provenance: predecessorRetainedCallerAsserted,
	}
}
