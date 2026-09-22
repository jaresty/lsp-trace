package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/programccompose"
)

type censusProjectionVerifier interface {
	Verify(string, captureset.ExactBytesAuthority) (captureset.Manifest, error)
	ResolveConstituent(string, string, captureset.ExactBytesAuthority) ([]byte, error)
}

type censusCompletionCustody struct {
	projection  censusacquisition.Projection
	result      censusresult.Result
	publication censusprogramc.VerifiedPublication
	metadata    programccompose.ExactMetadata
	workspace   censuscontinuation.WorkspaceIdentity
}

type censusCompletion struct {
	Result              *censusresult.Result
	Diagnostic          *censusresult.Diagnostic
	DiscoveryDiagnostic *censusresult.DiscoveryDiagnostic
	RequestReceipt      *censusrequest.Receipt
	continuation        *censusCompletionCustody
}

func (c censusCompletion) BuildCommittedHandoff() (censuscontinuation.CommittedHandoff, error) {
	if c.Result == nil || c.Diagnostic != nil || c.continuation == nil {
		return censuscontinuation.CommittedHandoff{}, errors.New("committed continuation custody incomplete")
	}
	custody, err := cloneCensusCompletionCustody(*c.continuation)
	if err != nil {
		return censuscontinuation.CommittedHandoff{}, errors.New("committed continuation custody incomplete")
	}
	return censuscontinuation.BuildHandoff(censuscontinuation.BuildInput{Result: custody.result, Projection: custody.projection, Publication: custody.publication, Metadata: custody.metadata, Workspace: custody.workspace})
}

func (r *censusRuntime) run(ctx context.Context, request operation.Request) censusCompletion {
	prepared, failure, reason := r.execute(ctx, request)
	if failure != nil {
		return censusFailureCompletion(failure, reason)
	}
	if !prepared.discovery.Complete {
		diagnostic, err := censusresult.NewDiscoveryDiagnosticFromAcquisition(prepared.discovery.Observations, prepared.admitted.requestReceipt.Semantic)
		if err == nil {
			receipt := prepared.admitted.requestReceipt
			return censusCompletion{DiscoveryDiagnostic: &diagnostic, RequestReceipt: &receipt}
		}
	}
	projection, failure, reason := r.acquire(ctx, prepared)
	if failure != nil {
		return censusFailureCompletion(failure, reason)
	}
	if request.PublicationRoot == nil {
		return censusFailureCompletion(censusPublicationFailure(), reasonPublicationRootRequired)
	}
	publisher := captureset.NewPublisher(request.PublicationRoot)
	completionCtx, cancel := context.WithDeadline(ctx, prepared.deadline)
	defer cancel()
	return completeCensusProjectionWithContinuation(completionCtx, projection, publisher, publisher, programccompose.ExactMetadata{
		RevisionCustody: "CALLER_ASSERTED", PositionEncoding: prepared.admitted.positionEncoding,
		AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1",
	})
}

func completeCensusProjectionWith(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher, verifier censusProjectionVerifier) censusCompletion {
	return completeCensusProjection(ctx, projection, publisher, verifier, nil)
}

func completeCensusProjectionWithContinuation(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher, verifier censusProjectionVerifier, metadata programccompose.ExactMetadata) censusCompletion {
	return completeCensusProjection(ctx, projection, publisher, verifier, &metadata)
}

func completeCensusProjection(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher, verifier censusProjectionVerifier, metadata *programccompose.ExactMetadata) censusCompletion {
	receipt, failure, reason := publishCensusProjectionWith(ctx, projection, publisher)
	if failure != nil {
		return censusFailureCompletion(failure, reason)
	}
	verificationStatus := receipt.VerificationStatus
	authority := captureset.NativeV5Authority()
	var verified captureset.Manifest
	var err error
	if verifier == nil || ctx.Err() != nil {
		err = context.Canceled
	} else {
		verified, err = verifier.Verify(receipt.Selector, authority)
		if err == nil {
			err = ctx.Err()
		}
	}
	resolved := make([]censusprogramc.ResolvedConstituent, 0, len(verified.Constituents))
	if err == nil {
		for _, constituent := range verified.Constituents {
			if ctx.Err() != nil {
				err = ctx.Err()
				break
			}
			raw, resolveErr := verifier.ResolveConstituent(receipt.Selector, constituent.ImmutableSelector, authority)
			if resolveErr != nil {
				err = resolveErr
				break
			}
			resolved = append(resolved, censusprogramc.ResolvedConstituent{ImmutableSelector: constituent.ImmutableSelector, Bytes: append([]byte(nil), raw...)})
		}
	}
	if err != nil {
		verificationStatus = "COMMITTED_VERIFICATION_FAILED"
	}
	result, buildErr := censusresult.Build(projection, censusresult.PublicationEvidence{
		Selector: receipt.Selector, Digest: receipt.ArtifactSHA256, ByteLength: receipt.ByteLength,
		VerificationStatus: verificationStatus, DirectorySyncStatus: receipt.DirectorySyncStatus, CloseStatus: receipt.CloseStatus,
	})
	if buildErr != nil || verificationStatus != "VERIFIED" || receipt.DirectorySyncStatus == "COMMITTED_DIRECTORY_SYNC_FAILED" || receipt.CloseStatus == "COMMITTED_CLOSE_FAILED" {
		diagnostic, _ := censusresult.NewDiagnostic(censusresult.StageCommitted, nil)
		return censusCompletion{Diagnostic: &diagnostic}
	}
	completion := censusCompletion{Result: &result}
	if metadata != nil {
		workspaceURI := (&url.URL{Scheme: "file", Path: projection.Workspace}).String()
		workspaceSum := sha256.Sum256([]byte("lsp-trace:census-workspace:v1\x00" + workspaceURI))
		m := *metadata
		m.WorkspaceIdentity = workspaceURI
		custody := censusCompletionCustody{
			projection: projection, result: result,
			publication: censusprogramc.VerifiedPublication{Receipt: *receipt, Manifest: verified, Resolved: resolved}, metadata: m,
			workspace: censuscontinuation.WorkspaceIdentity{URI: workspaceURI, Digest: "sha256:" + hex.EncodeToString(workspaceSum[:])},
		}
		if cloned, cloneErr := cloneCensusCompletionCustody(custody); cloneErr == nil {
			completion.continuation = &cloned
		}
	}
	return completion
}

func cloneCensusCompletionCustody(in censusCompletionCustody) (censusCompletionCustody, error) {
	projection, err := censusacquisition.CloneProjection(in.projection)
	if err != nil {
		return censusCompletionCustody{}, err
	}
	out := in
	out.projection = projection
	out.publication.Resolved = append([]censusprogramc.ResolvedConstituent(nil), in.publication.Resolved...)
	for i := range out.publication.Resolved {
		out.publication.Resolved[i].Bytes = append([]byte(nil), in.publication.Resolved[i].Bytes...)
	}
	manifestBytes, err := json.Marshal(in.publication.Manifest)
	if err != nil {
		return censusCompletionCustody{}, err
	}
	if err := json.Unmarshal(manifestBytes, &out.publication.Manifest); err != nil {
		return censusCompletionCustody{}, err
	}
	return out, nil
}

func censusFailureCompletion(failure *censusRuntimeFailure, reason censusFailureReason) censusCompletion {
	stage := censusresult.StageConfig
	if failure != nil {
		switch failure.stage {
		case censusStageDiscovery:
			stage = censusresult.StageDiscovery
		case censusStageAcquisition:
			stage = censusresult.StageAcquisition
		case censusStagePublication:
			stage = censusresult.StagePublication
		}
	}
	diagnostic, _ := censusresult.NewDiagnostic(stage, nil)
	if detail := censusFailureDetail(reason); detail != "" {
		diagnostic = diagnostic.WithDetail(detail)
	}
	switch reason {
	case reasonPublicationRootOpen, reasonPublicationPrivateValidation, reasonPublicationEncode, reasonPublicationCanonicalize, reasonPublicationTempWrite, reasonPublicationFsync, reasonPublicationNoReplace, reasonPublicationVerify, reasonPublicationReceipt, reasonPublicationCleanup, reasonPublicationInternal:
		diagnostic.PublicationFailure = censusresult.PublicationFailureStage(reason)
		diagnostic.PublicationAccounting = &censusresult.PublicationAccounting{Category: censusresult.PublicationAccountingUnknown}
		var private *captureset.PublicationFailure
		if errors.As(failure.err, &private) && private.CandidateSHA256 != "" {
			observed := private.CandidateByteLength
			diagnostic.PublicationAccounting = &censusresult.PublicationAccounting{Category: censusresult.PublicationAccountingCandidateBytes, Observed: &observed}
			if private.CodecLimit > 0 {
				limit := private.CodecLimit
				diagnostic.PublicationAccounting.Limit = &limit
			}
		}
	}
	return censusCompletion{Diagnostic: &diagnostic}
}
