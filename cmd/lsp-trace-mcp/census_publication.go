package main

import (
	"context"
	"fmt"
	"os"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

const basePublicationFailureLedgerPathEnv = "LSP_TRACE_BASE_PUBLICATION_FAILURE_LEDGER_PATH"

type censusProjectionPublisher interface {
	PublishCaptureSet(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult
}

func validateBasePublicationFailureLedgerSink() error {
	path := os.Getenv(basePublicationFailureLedgerPathEnv)
	if path == "" {
		return nil
	}
	if _, err := captureset.OpenFailureLedger(captureset.FailureLedgerConfig{Path: path, MaxBytes: captureset.DefaultFailureLedgerMaxBytes, MaxRecords: captureset.DefaultFailureLedgerMaxRecords}); err != nil {
		return &diagnosticSinkUnavailable{Sink: "BASE_PUBLICATION_FAILURE_LEDGER", cause: err}
	}
	return nil
}

// publishCensusProjection is package-main publication authority. It accepts no
// path and publishes only through the host-owned root attached to the request.
func publishCensusProjection(ctx context.Context, request operation.Request, projection censusacquisition.Projection) (*captureset.PublicationReceipt, *censusRuntimeFailure, censusFailureReason) {
	if request.PublicationRoot == nil {
		return nil, censusPublicationFailure(), reasonPublicationRootRequired
	}
	var ledger *captureset.FailureLedger
	if ledgerPath := os.Getenv(basePublicationFailureLedgerPathEnv); ledgerPath != "" {
		var err error
		ledger, err = captureset.OpenFailureLedger(captureset.FailureLedgerConfig{Path: ledgerPath, MaxBytes: captureset.DefaultFailureLedgerMaxBytes, MaxRecords: captureset.DefaultFailureLedgerMaxRecords})
		if err != nil {
			ledger = captureset.FailedFailureLedger(err)
		}
	}
	return publishCensusProjectionWith(ctx, projection, captureset.NewPublisherWithTraceAndFailureLedger(request.PublicationRoot, func(event publication.BoundFileTraceEvent) {
		basePublicationTrace(ctx, event)
	}, ledger))
}

// Once a receipt exists, cancellation or degraded receipt fields cannot turn
// the committed namespace entry into a retryable failure.
func publishCensusProjectionWith(ctx context.Context, projection censusacquisition.Projection, publisher censusProjectionPublisher) (*captureset.PublicationReceipt, *censusRuntimeFailure, censusFailureReason) {
	if ctx.Err() != nil || publisher == nil {
		return nil, censusPublicationFailure(), reasonCancelled
	}
	exact := make([][]byte, len(projection.Constituents))
	for i := range projection.Constituents {
		exact[i] = append([]byte(nil), projection.Constituents[i].Raw...)
	}
	result := publisher.PublishCaptureSet(projection.Manifest, exact, captureset.NativeV5Authority())
	if result.Receipt == nil {
		_ = dumpRuntimeTrace(ctx, "base_publication")
		failure := censusPublicationFailure()
		if result.Err != nil {
			failure.err = fmt.Errorf("capture-set publisher: %w", result.Err)
			reason := reasonPublicationInternal
			if result.Failure != nil {
				candidate := censusFailureReason(result.Failure.Stage)
				switch candidate {
				case reasonPublicationRootOpen, reasonPublicationPrivateValidation, reasonPublicationEncode, reasonPublicationCanonicalize, reasonPublicationTempWrite, reasonPublicationFsync, reasonPublicationNoReplace, reasonPublicationVerify, reasonPublicationReceipt, reasonPublicationCleanup, reasonPublicationInternal:
					reason = candidate
				}
			}
			return nil, failure, reason
		}
		return nil, failure, reasonPublicationNoReceipt
	}
	return result.Receipt, nil, reasonNone
}
