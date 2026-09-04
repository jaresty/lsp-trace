package provider

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/observationadapter"
)

// ObservationSemanticAdapter binds provider transport receipts to observation adaptation.
type ObservationSemanticAdapter struct{}

func NewObservationSemanticAdapter(Provisioned, observationadapter.Identity) (*ObservationSemanticAdapter, error) {
	return &ObservationSemanticAdapter{}, nil
}

func (*ObservationSemanticAdapter) Adapt(context.Context, StrictCollectorRequest, Receipt) (json.RawMessage, error) {
	return nil, errors.New("provider semantic binding not implemented")
}
