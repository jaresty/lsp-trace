package censuscontinuation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
)

const (
	EndpointOutcomeSchema              = "lsp-trace.census-endpoint-capture-outcome.v1"
	PreparationFailureSchema           = "lsp-trace.census-preparation-failure.v1"
	EndpointRoleTarget                 = "TARGET"
	EndpointRoleCaller                 = "CALLER"
	EndpointStatusCaptured             = "CAPTURED"
	EndpointStatusSourceUnavailable    = "SOURCE_UNAVAILABLE"
	SourceCodeExactEndpointUnavailable = "EXACT_ENDPOINT_SOURCE_UNAVAILABLE"
)

type EndpointIdentity struct {
	NominationID        string `json:"nomination_id"`
	Role                string `json:"role"`
	ConstituentIdentity string `json:"constituent_identity"`
	ConstituentOrdinal  int    `json:"constituent_ordinal"`
	GraphSubjectID      string `json:"graph_subject_id"`
	NodeID              string `json:"node_id"`
	LogicalSourceDigest string `json:"logical_source_digest"`
	LogicalSourceID     string `json:"-"`
}

type EndpointCaptureOutcome struct {
	SchemaVersion string           `json:"schema_version"`
	OutcomeID     string           `json:"outcome_id"`
	Endpoint      EndpointIdentity `json:"endpoint"`
	Status        string           `json:"status"`
	Code          string           `json:"code,omitempty"`
	Authority     int              `json:"authority"`
	Accepted      bool             `json:"accepted"`
	Completeness  string           `json:"completeness"`
}

type PreparationFailure struct {
	SchemaVersion       string `json:"schema_version"`
	FailureID           string `json:"failure_id"`
	NominationID        string `json:"nomination_id"`
	PacketIntentID      string `json:"packet_intent_id"`
	Role                string `json:"role"`
	ConstituentIdentity string `json:"constituent_identity"`
	ConstituentOrdinal  int    `json:"constituent_ordinal"`
	GraphSubjectID      string `json:"graph_subject_id"`
	NodeID              string `json:"node_id"`
	LogicalSourceDigest string `json:"logical_source_digest"`
	Code                string `json:"code"`
	ResponseID          string `json:"response_id"`
	Authority           int    `json:"authority"`
	Accepted            bool   `json:"accepted"`
	Completeness        string `json:"completeness"`
}

func NewEndpointCaptureOutcome(endpoint EndpointIdentity, status, code string) (EndpointCaptureOutcome, error) {
	endpoint = canonicalEndpoint(endpoint)
	o := EndpointCaptureOutcome{SchemaVersion: EndpointOutcomeSchema, Endpoint: endpoint, Status: status, Code: code, Completeness: ContinuationCompleteness}
	o.OutcomeID = identityJSON("lsp-trace:census-endpoint-capture-outcome:v1", o, func(v *EndpointCaptureOutcome) { v.OutcomeID = "" })
	return o, o.Validate()
}

func (o EndpointCaptureOutcome) Validate() error {
	if o.SchemaVersion != EndpointOutcomeSchema || !validDigest(o.OutcomeID) || o.Authority != 0 || o.Accepted || o.Completeness != ContinuationCompleteness || validateEndpoint(o.Endpoint) != nil {
		return errors.New("endpoint outcome integrity mismatch")
	}
	if (o.Status == EndpointStatusCaptured && o.Code != "") || (o.Status == EndpointStatusSourceUnavailable && o.Code != SourceCodeExactEndpointUnavailable) || (o.Status != EndpointStatusCaptured && o.Status != EndpointStatusSourceUnavailable) {
		return errors.New("endpoint outcome status mismatch")
	}
	if identityJSON("lsp-trace:census-endpoint-capture-outcome:v1", o, func(v *EndpointCaptureOutcome) { v.OutcomeID = "" }) != o.OutcomeID {
		return errors.New("endpoint outcome identity mismatch")
	}
	return nil
}

func ParseEndpointCaptureOutcome(raw []byte) (EndpointCaptureOutcome, error) {
	var o EndpointCaptureOutcome
	if err := strictRecord(raw, &o); err != nil {
		return o, err
	}
	return o, o.Validate()
}

func ValidateEndpointCaptureClosure(derived []EndpointIdentity, outcomes []EndpointCaptureOutcome) ([]EndpointCaptureOutcome, error) {
	want := map[string]EndpointIdentity{}
	for _, endpoint := range derived {
		endpoint = canonicalEndpoint(endpoint)
		if err := validateEndpoint(endpoint); err != nil {
			return nil, err
		}
		key := endpointKey(endpoint)
		if _, duplicate := want[key]; duplicate {
			return nil, errors.New("duplicate derived endpoint")
		}
		want[key] = endpoint
	}
	got := make(map[string]bool, len(outcomes))
	canonical := append([]EndpointCaptureOutcome(nil), outcomes...)
	for i := range canonical {
		canonical[i].Endpoint = canonicalEndpoint(canonical[i].Endpoint)
		if err := canonical[i].Validate(); err != nil {
			return nil, err
		}
		key := endpointKey(canonical[i].Endpoint)
		expected, known := want[key]
		if !known || got[key] || canonical[i].Endpoint != expected {
			return nil, errors.New("unknown, duplicate, or substituted endpoint outcome")
		}
		got[key] = true
	}
	if len(got) != len(want) {
		return nil, errors.New("missing endpoint outcome")
	}
	sort.Slice(canonical, func(i, j int) bool { return endpointKey(canonical[i].Endpoint) < endpointKey(canonical[j].Endpoint) })
	return canonical, nil
}

func NewPreparationFailure(packetIntentID string, endpoint EndpointIdentity, code string) (PreparationFailure, error) {
	endpoint = canonicalEndpoint(endpoint)
	f := PreparationFailure{SchemaVersion: PreparationFailureSchema, NominationID: endpoint.NominationID, PacketIntentID: packetIntentID, Role: endpoint.Role, ConstituentIdentity: endpoint.ConstituentIdentity, ConstituentOrdinal: endpoint.ConstituentOrdinal, GraphSubjectID: endpoint.GraphSubjectID, NodeID: endpoint.NodeID, LogicalSourceDigest: endpoint.LogicalSourceDigest, Code: code, Completeness: ContinuationCompleteness}
	f.FailureID = identityJSON("lsp-trace:census-preparation-failure:v1", f, func(v *PreparationFailure) { v.FailureID = "" })
	return f, f.Validate()
}

func (f PreparationFailure) Validate() error {
	if f.SchemaVersion != PreparationFailureSchema || !validDigest(f.FailureID) || f.NominationID == "" || f.PacketIntentID == "" || (f.Role != EndpointRoleTarget && f.Role != EndpointRoleCaller) || f.ConstituentIdentity == "" || f.ConstituentOrdinal < 0 || f.GraphSubjectID == "" || f.NodeID == "" || !validDigest(f.LogicalSourceDigest) || f.Code != SourceCodeExactEndpointUnavailable || f.ResponseID != "" || f.Authority != 0 || f.Accepted || f.Completeness != ContinuationCompleteness {
		return errors.New("preparation failure integrity mismatch")
	}
	if identityJSON("lsp-trace:census-preparation-failure:v1", f, func(v *PreparationFailure) { v.FailureID = "" }) != f.FailureID {
		return errors.New("preparation failure identity mismatch")
	}
	return nil
}

func canonicalEndpoint(e EndpointIdentity) EndpointIdentity {
	if e.LogicalSourceDigest == "" && e.LogicalSourceID != "" {
		e.LogicalSourceDigest = digestString(e.LogicalSourceID)
	}
	e.LogicalSourceID = ""
	return e
}
func validateEndpoint(e EndpointIdentity) error {
	if e.NominationID == "" || (e.Role != EndpointRoleTarget && e.Role != EndpointRoleCaller) || e.ConstituentIdentity == "" || e.ConstituentOrdinal < 0 || e.GraphSubjectID == "" || e.NodeID == "" || !validDigest(e.LogicalSourceDigest) || e.LogicalSourceID != "" {
		return errors.New("endpoint identity invalid")
	}
	return nil
}
func endpointKey(e EndpointIdentity) string {
	b, _ := json.Marshal(canonicalEndpoint(e))
	return string(b)
}
func digestString(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}
func strictRecord(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing json")
	}
	canonical, err := json.Marshal(out)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("noncanonical json")
	}
	return nil
}
