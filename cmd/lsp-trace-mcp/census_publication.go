package main

import (
	"context"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/operation"
)

type censusProjectionPublisher interface {
	PublishCaptureSet(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult
}

// publishCensusProjection is package-main publication authority. It accepts no
// path and publishes only through the host-owned root attached to the request.
func publishCensusProjection(ctx context.Context, request operation.Request, projection censusacquisition.Projection) (*captureset.PublicationReceipt, *censusRuntimeFailure) {
	if request.PublicationRoot == nil {
		return nil, censusPublicationFailure()
	}
	return publishCensusProjectionWith(ctx, projection, captureset.NewPublisher(request.PublicationRoot))
}

// Once a receipt exists, cancellation or degraded receipt fields cannot turn
// the committed namespace entry into a retryable failure.
func publishCensusProjectionWith(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher) (*captureset.PublicationReceipt, *censusRuntimeFailure) {
	if ctx.Err() != nil || publisher == nil {
		return nil, censusPublicationFailure()
	}
	exact := make([][]byte, len(projection.Constituents))
	for i := range projection.Constituents {
		exact[i] = append([]byte(nil), projection.Constituents[i].Raw...)
	}
	result := publisher.PublishCaptureSet(projection.Manifest, exact, captureset.NativeV5Authority())
	if result.Receipt == nil {
		return nil, censusPublicationFailure()
	}
	return result.Receipt, nil
}
