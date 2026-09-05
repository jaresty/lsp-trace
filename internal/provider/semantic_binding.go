package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
	"lsp-trace/internal/relations"
)

// Result is the lossless provider-neutral result accepted by managed operations.
type Result struct {
	ProviderID    string                           `json:"provider_id"`
	Provider      observationadapter.Identity      `json:"provider"`
	Protocol      observationadapter.Identity      `json:"protocol"`
	Adapter       observationadapter.Identity      `json:"adapter"`
	Terminal      string                           `json:"terminal"`
	Complete      bool                             `json:"complete"`
	Truncated     bool                             `json:"truncated"`
	Bounds        CollectorLimits                  `json:"bounds"`
	Coverage      relations.Coverage               `json:"coverage"`
	Custody       graph.DocumentCustodyReceipt     `json:"custody"`
	Observations  []observationadapter.Observation `json:"observations"`
	GraphV4       graph.NormalizedRelations        `json:"graph_v4"`
	LogicalDigest string                           `json:"logical_digest"`
	Receipt       ExecutionReceipt                 `json:"receipt"`
}

type ExecutionReceipt struct {
	Messages        int    `json:"messages"`
	Stderr          string `json:"stderr,omitempty"`
	StderrBytes     int64  `json:"stderr_bytes"`
	StderrTruncated bool   `json:"stderr_truncated"`
	ExitCode        int    `json:"exit_code"`
	Terminated      bool   `json:"terminated"`
	Reaped          bool   `json:"reaped"`
}

// ObservationSemanticAdapter binds strict transport receipts to the provider-neutral
// observation adapter without interpreting framework semantics.
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
	canonical, err := json.Marshal(adapted.GraphV4)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canonical)
	logicalDigest := "sha256:" + hex.EncodeToString(sum[:])
	executionReceipt := ExecutionReceipt{Messages: receipt.Messages, Stderr: string(receipt.Stderr.Bytes), StderrBytes: receipt.Stderr.TotalBytes, StderrTruncated: receipt.Stderr.Truncated, ExitCode: receipt.ExitCode, Terminated: receipt.Terminated, Reaped: receipt.Reaped}
	observationIDs := make([]string, len(adapted.Observations))
	for i := range adapted.Observations {
		observationIDs[i] = adapted.Observations[i].ObservationID
	}
	marshal := func(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
	adapted.GraphV4.Provenance = &graph.NormalizedRelationsProvenance{
		ProviderID: request.ProviderID, Provider: marshal(envelope.Provider), Protocol: marshal(envelope.Protocol), Adapter: marshal(envelope.Adapter),
		Coverage: marshal(adapted.Coverage), Bounds: marshal(request.Limits), Custody: marshal(adapted.Custody), ObservationIDs: observationIDs,
		LogicalDigest: logicalDigest, Receipt: marshal(executionReceipt),
	}
	complete := string(adapted.Coverage.Status) == "COMPLETE_WITHIN_BOUNDS" && adapted.Failure == ""
	return json.Marshal(Result{
		ProviderID: request.ProviderID, Provider: envelope.Provider, Protocol: envelope.Protocol, Adapter: envelope.Adapter,
		Terminal: string(adapted.Coverage.Status), Complete: complete, Truncated: string(adapted.Failure) == "BOUNDED_TRUNCATION",
		Bounds: request.Limits, Coverage: adapted.Coverage, Custody: adapted.Custody,
		Observations: adapted.Observations, GraphV4: adapted.GraphV4, LogicalDigest: logicalDigest, Receipt: executionReceipt,
	})
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
