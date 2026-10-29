package graphprovenance

import "encoding/json"

func (e SourceWithholdingPrivateV2) MarshalJSON() ([]byte, error) {
	// Reuse the neutral exact-graph codec, despite its historical V1 name.
	// This is codec reuse, not V1 carrier conversion or schema admission.
	evidence, err := marshalSourceEvidencePrivateV1(e.Evidence)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		SchemaVersion      string                `json:"schema_version"`
		Policy             SourcePolicyPrivateV2 `json:"source_policy"`
		Evidence           json.RawMessage       `json:"evidence"`
		SourceCompleteness string                `json:"source_completeness"`
		Accepted           bool                  `json:"accepted"`
		Authority          int                   `json:"authority"`
		ReplayAdmitted     bool                  `json:"replay_admitted"`
		Partial            bool                  `json:"partial"`
		WithheldCount      int                   `json:"withheld_count"`
		FrozenQueryScope   bool                  `json:"frozen_query_scope"`
	}{e.SchemaVersion, e.Policy, evidence, e.SourceCompleteness, e.Accepted, e.Authority, e.ReplayAdmitted, e.Partial, e.WithheldCount, e.FrozenQueryScope})
}
