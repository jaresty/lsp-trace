package main

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/operation"
)

type censusProjectionPublisherFunc func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult

func (f censusProjectionPublisherFunc) PublishCaptureSet(m captureset.Manifest, raw [][]byte, authority captureset.ExactBytesAuthority) captureset.PublicationResult {
	return f(m, raw, authority)
}

func TestPublishCensusProjectionCopiesBytesAndPreservesCommittedReceipt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	projection := censusacquisition.Projection{Constituents: []censusacquisition.Constituent{{Raw: []byte("exact")}}}
	want := &captureset.PublicationReceipt{Selector: "capture-sets/v1/sha256/x.bundle", VerificationStatus: "COMMITTED_VERIFICATION_FAILED"}
	publisher := censusProjectionPublisherFunc(func(_ captureset.Manifest, raw [][]byte, authority captureset.ExactBytesAuthority) captureset.PublicationResult {
		if len(raw) != 1 || string(raw[0]) != "exact" || authority.AdmitGraphProvenanceV5 == nil {
			t.Fatalf("ASSERT_MCP_CENSUS_PUBLICATION_EXACT_INPUT: raw=%q", raw)
		}
		raw[0][0] = 'X'
		cancel()
		return captureset.PublicationResult{Receipt: want, Err: errors.New("ignored after receipt")}
	})
	got, failure := publishCensusProjectionWith(ctx, projection, publisher)
	if failure != nil || got != want || string(projection.Constituents[0].Raw) != "exact" || ctx.Err() == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_COMMITTED_RECEIPT_SURVIVES_DEGRADATION: receipt=%+v failure=%+v raw=%q err=%v", got, failure, projection.Constituents[0].Raw, ctx.Err())
	}
}

func TestPublishCensusProjectionFailsBeforeReceipt(t *testing.T) {
	publisher := censusProjectionPublisherFunc(func(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult {
		return captureset.PublicationResult{Err: errors.New("precommit")}
	})
	if receipt, failure := publishCensusProjectionWith(context.Background(), censusacquisition.Projection{}, publisher); receipt != nil || failure == nil || failure.stage != censusStagePublication || failure.code != censusCodePublicationFailed {
		t.Fatalf("ASSERT_MCP_CENSUS_PRECOMMIT_PUBLICATION_FAILURE: receipt=%+v failure=%+v", receipt, failure)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if receipt, failure := publishCensusProjectionWith(ctx, censusacquisition.Projection{}, publisher); receipt != nil || failure == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_CANCEL_BEFORE_PUBLICATION: receipt=%+v failure=%+v", receipt, failure)
	}
	if receipt, failure := publishCensusProjection(context.Background(), operation.Request{}, censusacquisition.Projection{}); receipt != nil || failure == nil {
		t.Fatalf("ASSERT_MCP_CENSUS_HOST_ROOT_REQUIRED: receipt=%+v failure=%+v", receipt, failure)
	}
}
