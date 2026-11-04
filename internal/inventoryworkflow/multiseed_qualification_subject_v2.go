package inventoryworkflow

import (
	"context"
	"errors"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
)

// BuildProgramCMultiSeedQualificationSubjectV2 composes only repository-owned
// reconstructed metadata. No caller supplies inventory selectors, seed fields,
// outcomes, counts, epochs, qualification booleans, or replacement manifests.
// This opens no new execution path: it is private read-only completion replay.
func BuildProgramCMultiSeedQualificationSubjectV2(ctx context.Context, request Request, implementationSource programcrepresentative.ImplementationSourceReaderV2, maxImplementationSourceBytes uint64) (subject programcrepresentative.ProgramCRepresentativeQualificationSubjectV2, err error) {
	if ctx == nil || request.ProgramC.OpenState == nil || request.ProgramC.WorkflowKey == "" || request.ProgramC.HeadKey == "" || request.SuccessorWorkflowKey == "" || request.SuccessorHeadKey == "" {
		return subject, inventorysuccessor.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return subject, err
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
			subject = programcrepresentative.ProgramCRepresentativeQualificationSubjectV2{}
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
	programC, err := programctransitionhost.NewCompletionVerificationRequest(ctx, CompletionRequest{StateRoot: root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger, WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings})
	if err != nil {
		return subject, err
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(ctx, programC)
	if err != nil {
		return subject, err
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		return subject, err
	}
	expected.WorkflowIdentity = qualificationWorkflowIdentity(identity)
	expected.SourcePinSHA256 = identity.SourcePinSHA256
	expected.Stage6Implementation, err = programcrepresentative.BuildStage6ImplementationV2(maxImplementationSourceBytes)
	if err != nil {
		return subject, err
	}
	expected.ImplementationSource = implementationSource
	expected.MaxImplementationSourceBytes = maxImplementationSourceBytes
	return programcrepresentative.NewProgramCRepresentativeQualificationSubjectV2(expected)
}
