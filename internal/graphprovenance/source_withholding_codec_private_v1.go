package graphprovenance

import (
	"bytes"
	"encoding/json"
	"errors"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
)

// Private evidence uses the exact old wire shape/order, but not the ordinary
// EvidenceV2 MarshalJSON contract: its runtime decoder omits process metadata.
// Every other graph field is reprojected and compared before raw bytes are emitted.
func marshalSourceEvidencePrivateV1(e EvidenceV2) ([]byte, error) {
	if err := preflightGraphV2(e.GraphBytes); err != nil {
		return nil, err
	}
	native, err := graph.MarshalWithRetainedProcessContextPrivate(e.Acquisition.Graph, e.GraphBytes)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(native, e.GraphBytes) {
		return nil, errors.New("private typed graph differs from captured bytes")
	}
	a, err := json.Marshal(struct {
		*acquisition.Result
		Graph json.RawMessage `json:"graph"`
	}{&e.Acquisition, json.RawMessage(e.GraphBytes)})
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		evidenceV2Alias
		Acquisition json.RawMessage `json:"acquisition"`
	}{evidenceV2Alias(e), a})
}

func (e SourceWithholdingPrivateV1) MarshalJSON() ([]byte, error) {
	evidence, err := marshalSourceEvidencePrivateV1(e.Evidence)
	if err != nil {
		return nil, err
	}
	// Explicit typed DTO preserves the original evidence field's position.
	return json.Marshal(struct {
		SchemaVersion      string                `json:"schema_version"`
		Policy             SourcePolicyPrivateV1 `json:"source_policy"`
		Evidence           json.RawMessage       `json:"evidence"`
		SourceCompleteness string                `json:"source_completeness"`
		Accepted           bool                  `json:"accepted"`
		Authority          int                   `json:"authority"`
		ReplayAdmitted     bool                  `json:"replay_admitted"`
		Partial            bool                  `json:"partial"`
		WithheldCount      int                   `json:"withheld_count"`
		FrozenQueryScope   bool                  `json:"frozen_query_scope,omitempty"`
	}{e.SchemaVersion, e.Policy, evidence, e.SourceCompleteness, e.Accepted, e.Authority, e.ReplayAdmitted, e.Partial, e.WithheldCount, e.FrozenQueryScope})
}
