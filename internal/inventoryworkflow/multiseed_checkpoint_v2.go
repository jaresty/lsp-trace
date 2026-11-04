package inventoryworkflow

import (
	"context"
	"errors"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
)

type MultiSeedCheckpointRequestV2 struct {
	Workflow                     Request
	Operation                    inventorysuccessor.MultiSeedOperationKindV2
	Envelope                     programcrepresentative.ArtifactBindingDTO
	ImplementationSource         programcrepresentative.ImplementationSourceReaderV2
	MaxImplementationSourceBytes uint64
}

// Artifact-only resolver: qualification provenance and reconstruction belong to Census.
type rootQualificationArtifactsV2 struct {
	objects *inventorysuccessor.MultiSeedObjectsV2
}

func (s rootQualificationArtifactsV2) ResolveQualificationArtifactV2(ctx context.Context, b programcrepresentative.ArtifactBindingDTO) ([]byte, error) {
	return s.objects.Resolve(ctx, b)
}

// RunMultiSeedCheckpointV2 executes exactly one explicitly requested operation.
// Each call opens/closes its own root and independently reconstructs frozen
// custody, subject, review, envelope and terminal generation. It never catches
// a conflict/unknown commit to retry or invoke another checkpoint operation.
func RunMultiSeedCheckpointV2(ctx context.Context, request MultiSeedCheckpointRequestV2) (inventorysuccessor.MultiSeedOperationResultV2, error) {
	return runMultiSeedCheckpointV2(ctx, request, nil)
}

// The optional private observer reports completed operation counters only. It
// cannot supply decisions, override a store, change a result, or grant authority.
func runMultiSeedCheckpointV2(ctx context.Context, request MultiSeedCheckpointRequestV2, observe func(inventorysuccessor.MultiSeedOperationObservationsV2)) (result inventorysuccessor.MultiSeedOperationResultV2, err error) {
	w := request.Workflow
	if ctx == nil || w.ProgramC.OpenState == nil || w.SuccessorWorkflowKey == "" || w.SuccessorHeadKey == "" {
		return result, inventorysuccessor.ErrInvalid
	}
	if request.Operation != inventorysuccessor.MultiSeedDenominatorOperationV2 && request.Operation != inventorysuccessor.MultiSeedLineageOperationV2 && request.Operation != inventorysuccessor.MultiSeedSuccessorOperationV2 {
		return result, inventorysuccessor.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	root, err := w.ProgramC.OpenState()
	if err != nil {
		return result, err
	}
	if root == nil {
		return result, inventorysuccessor.ErrInvalid
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			result = inventorysuccessor.MultiSeedOperationResultV2{}
			err = errors.Join(err, closeErr)
		}
	}()
	heads, err := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		return result, err
	}
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		return result, err
	}
	identity, err := inventorysuccessor.VerifyMultiSeedWorkflowIdentityV2(ctx, w.SuccessorWorkflowKey, w.SuccessorHeadKey, w.Identity, heads, objects)
	if err != nil {
		return result, err
	}
	verification, err := programctransitionhost.NewCompletionVerificationRequest(ctx, CompletionRequest{StateRoot: root, Receipt: w.ProgramC.Receipt, Ledger: w.ProgramC.Ledger, WorkflowKey: w.ProgramC.WorkflowKey, HeadKey: w.ProgramC.HeadKey, Bindings: w.ProgramC.Bindings})
	if err != nil {
		return result, err
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(ctx, verification)
	if err != nil {
		return result, err
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		return result, err
	}
	expected.WorkflowIdentity = qualificationWorkflowIdentity(identity)
	expected.SourcePinSHA256 = identity.SourcePinSHA256
	expected.ImplementationSource = request.ImplementationSource
	expected.MaxImplementationSourceBytes = request.MaxImplementationSourceBytes
	expected.Stage6Implementation, err = programcrepresentative.BuildStage6ImplementationV2(request.MaxImplementationSourceBytes)
	if err != nil {
		return result, err
	}
	if request.Envelope.SchemaVersion != programcrepresentative.QualificationEnvelopeSchemaVersionV2 {
		return result, inventorysuccessor.ErrInvalid
	}
	raw, err := objects.Resolve(ctx, request.Envelope)
	if err != nil {
		return result, err
	}
	source := rootQualificationArtifactsV2{objects: objects}
	qualification, err := evidence.VerifyQualificationEnvelopeV2(ctx, expected, raw, source)
	if err != nil {
		return result, err
	}
	binding, err := qualification.EnvelopeBindingV2()
	if err != nil || binding != request.Envelope {
		return result, inventorysuccessor.ErrInvalid
	}
	operation, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(root, request.Operation, w.SuccessorWorkflowKey, w.SuccessorHeadKey, identity, qualification)
	if err != nil {
		return result, err
	}
	result, err = operation.Execute(ctx)
	if observe != nil {
		observe(operation.ObservationsV2())
	}
	return result, err
}
