package main

import (
	"context"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/operation"
)

type censusProjectionVerifier interface {
	Verify(string, captureset.ExactBytesAuthority) (captureset.Manifest, error)
	ResolveConstituent(string, string, captureset.ExactBytesAuthority) ([]byte, error)
}

type censusCompletion struct {
	Result     *censusresult.Result
	Diagnostic *censusresult.Diagnostic
}

func (r *censusRuntime) run(ctx context.Context, request operation.Request) censusCompletion {
	prepared, failure := r.execute(ctx, request)
	if failure != nil {
		return censusFailureCompletion(failure)
	}
	projection, failure := r.acquire(ctx, prepared)
	if failure != nil {
		return censusFailureCompletion(failure)
	}
	if request.PublicationRoot == nil {
		return censusFailureCompletion(censusPublicationFailure())
	}
	publisher := captureset.NewPublisher(request.PublicationRoot)
	completionCtx, cancel := context.WithDeadline(ctx, prepared.deadline)
	defer cancel()
	return completeCensusProjectionWith(completionCtx, projection, publisher, publisher)
}

func completeCensusProjectionWith(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher, verifier censusProjectionVerifier) censusCompletion {
	receipt, failure := publishCensusProjectionWith(ctx, projection, publisher)
	if failure != nil {
		return censusFailureCompletion(failure)
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
	if err == nil {
		for _, constituent := range verified.Constituents {
			if ctx.Err() != nil {
				err = ctx.Err()
				break
			}
			if _, resolveErr := verifier.ResolveConstituent(receipt.Selector, constituent.ImmutableSelector, authority); resolveErr != nil {
				err = resolveErr
				break
			}
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
	return censusCompletion{Result: &result}
}

func censusFailureCompletion(failure *censusRuntimeFailure) censusCompletion {
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
	return censusCompletion{Diagnostic: &diagnostic}
}
