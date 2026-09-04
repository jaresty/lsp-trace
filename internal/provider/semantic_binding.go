package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
)

// ObservationSemanticAdapter binds strict transport receipts to the provider-neutral
// observation adapter and projects its result into the incoming/slice receipt contract.
type ObservationSemanticAdapter struct {
	declarations map[string]Declaration
	adapter      observationadapter.Identity
	adapterID    string
}

func NewObservationSemanticAdapter(provisioned Provisioned, adapter observationadapter.Identity) (*ObservationSemanticAdapter, error) {
	if provisioned.Registry == nil || adapter.Name == "" || adapter.Version == "" {
		return nil, errors.New("provider semantic adapter requires provisioning and adapter identity")
	}
	declarations := make(map[string]Declaration, len(provisioned.Declarations))
	for _, declaration := range provisioned.Declarations {
		declarations[declaration.Identity] = cloneDeclaration(declaration)
	}
	return &ObservationSemanticAdapter{declarations: declarations, adapter: adapter, adapterID: adapter.Name + "@" + adapter.Version}, nil
}

func (a *ObservationSemanticAdapter) Adapt(_ context.Context, request StrictCollectorRequest, receipt Receipt) (json.RawMessage, error) {
	if a == nil || request.SchemaVersion != CollectorRequestSchema {
		return nil, errors.New("provider semantic adapter unavailable or request schema mismatch")
	}
	if receipt.Failure != nil {
		return nil, fmt.Errorf("provider transport failed: %s", receipt.Failure.Kind)
	}
	if receipt.ProviderID != request.ProviderID {
		return nil, errors.New("provider receipt identity does not match request")
	}
	declaration, ok := a.declarations[request.ProviderID]
	if !ok {
		return nil, errors.New("request provider is not host provisioned")
	}
	if request.AdapterID != a.adapterID {
		return nil, errors.New("request adapter identity mismatch")
	}
	var envelope observationadapter.Envelope
	if err := decodeObservationEnvelope(receipt.Response, &envelope); err != nil {
		return nil, fmt.Errorf("strict observation envelope: %w", err)
	}
	if envelope.Provider.Name+"@"+envelope.Provider.Version != declaration.Identity || envelope.Provider.Version != declaration.Version {
		return nil, errors.New("observation provider identity/version mismatch")
	}
	if envelope.Protocol.Name != declaration.Protocol.Name || envelope.Protocol.Version != declaration.Protocol.Version {
		return nil, errors.New("observation protocol identity/version mismatch")
	}
	if envelope.Adapter != a.adapter {
		return nil, errors.New("observation adapter identity/version mismatch")
	}
	if envelope.RequestID != semanticRequestID(request) {
		return nil, errors.New("observation request identity mismatch")
	}
	if err := validateSemanticCustody(request, envelope); err != nil {
		return nil, err
	}
	selected := make(map[string]struct{}, len(request.Relations))
	for _, relation := range request.Relations {
		selected[relation] = struct{}{}
	}
	for _, observation := range envelope.Observations {
		if _, ok := selected[string(observation.Kind)]; !ok {
			return nil, fmt.Errorf("observation relation %q was not selected", observation.Kind)
		}
	}
	adapted, err := observationadapter.Adapt(envelope)
	if err != nil {
		return nil, err
	}
	type relation struct {
		RelationID string `json:"relation_id"`
		Kind       string `json:"kind"`
	}
	relations := make([]relation, len(adapted.GraphV4.Relations))
	for i, item := range adapted.GraphV4.Relations {
		relations[i] = relation{RelationID: item.RelationID, Kind: item.Kind}
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
	bounds, err := json.Marshal(request.Limits)
	if err != nil {
		return nil, err
	}
	complete := string(adapted.Coverage.Status) == "COMPLETE_WITHIN_BOUNDS" && adapted.Failure == ""
	payload := struct {
		ProviderID string          `json:"provider_id"`
		Terminal   string          `json:"terminal"`
		Complete   bool            `json:"complete"`
		Truncated  bool            `json:"truncated"`
		Bounds     json.RawMessage `json:"bounds"`
		Relations  []relation      `json:"relations"`
	}{request.ProviderID, string(adapted.Coverage.Status), complete, string(adapted.Failure) == "BOUNDED_TRUNCATION", bounds, relations}
	return json.Marshal(payload)
}

func decodeObservationEnvelope(raw []byte, target *observationadapter.Envelope) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}

func semanticRequestID(request StrictCollectorRequest) string {
	return fmt.Sprintf("%s:%d:%s", request.Session.SessionID, request.Session.Generation, request.Seed.URI)
}

func validateSemanticCustody(request StrictCollectorRequest, envelope observationadapter.Envelope) error {
	if len(envelope.Documents) == 0 {
		return errors.New("observation envelope omitted document custody")
	}
	var workspace graph.RevisionIdentity
	if len(request.Documents.WorkspaceRevision) > 0 {
		if err := json.Unmarshal(request.Documents.WorkspaceRevision, &workspace); err != nil {
			return fmt.Errorf("request workspace revision: %w", err)
		}
	}
	matched := false
	for _, document := range envelope.Documents {
		if document.OriginalURI != request.Documents.OriginalURI {
			continue
		}
		matched = true
		if workspace.Kind != "" && (document.Revision.Kind != workspace.Kind || document.Revision.Value != workspace.Value) {
			return errors.New("observation document revision does not match request custody")
		}
	}
	if !matched {
		return errors.New("observation original URI does not match request custody")
	}
	return nil
}
