// Package describeworker runs the pinned ADR-0007 semantic worker and retains
// immutable, non-authoritative invocation and response records.
package describeworker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const (
	ResponseSchema      = "lsp-trace.describe-response.v1"
	InvocationSchema    = "lsp-trace.describe-invocation.v1"
	CompletenessUnknown = "UNKNOWN"
)

type TerminalStatus string

const (
	StatusSucceeded        TerminalStatus = "SUCCEEDED"
	StatusCancelled        TerminalStatus = "CANCELLED"
	StatusTimeout          TerminalStatus = "TIMEOUT"
	StatusResourceLimit    TerminalStatus = "RESOURCE_LIMIT"
	StatusBackendFailure   TerminalStatus = "BACKEND_FAILURE"
	StatusOutputInvalid    TerminalStatus = "OUTPUT_INVALID"
	StatusModelUnavailable TerminalStatus = "MODEL_UNAVAILABLE"
	StatusPolicyMismatch   TerminalStatus = "POLICY_MISMATCH"
)

type SemanticResponse struct {
	Verdict                string   `json:"verdict"`
	TargetRole             string   `json:"target_role"`
	NearestOutwardConsumer string   `json:"nearest_outward_consumer"`
	ConsumerNeed           string   `json:"consumer_need"`
	ProvidedBehavior       string   `json:"provided_behavior"`
	BoundaryContribution   string   `json:"boundary_contribution"`
	Limitations            []string `json:"limitations"`
	Citations              []string `json:"citations"`
}
type Limits struct {
	TimeoutMS     int `json:"timeout_ms"`
	MaxTokens     int `json:"max_tokens"`
	ContextTokens int `json:"context_tokens"`
	StdoutBytes   int `json:"stdout_bytes"`
	StderrBytes   int `json:"stderr_bytes"`
	WorkBytes     int `json:"work_bytes"`
	TempBytes     int `json:"temp_bytes"`
}
type IdentityBinding struct {
	RequestRecordID         string `json:"request_record_id"`
	MessageID               string `json:"message_id"`
	AttemptID               string `json:"attempt_id"`
	WorkerSHA256            string `json:"worker_sha256"`
	ModelSHA256             string `json:"model_sha256"`
	LibrarySHA256           string `json:"library_sha256"`
	SandboxExecutableSHA256 string `json:"sandbox_executable_sha256"`
	SandboxProfileSHA256    string `json:"sandbox_profile_sha256"`
	RuntimeIdentity         string `json:"runtime_identity"`
	AdapterIdentity         string `json:"adapter_identity"`
	ModelIdentity           string `json:"model_identity"`
	PromptSHA256            string `json:"prompt_sha256"`
	GrammarSHA256           string `json:"grammar_sha256"`
	Limits                  Limits `json:"limits"`
}
type ResourceReceipt struct {
	Started          bool `json:"started"`
	TerminalOutcomes int  `json:"terminal_outcomes"`
	Reaped           bool `json:"reaped"`
	Teardown         bool `json:"teardown"`
	StdoutBytes      int  `json:"stdout_bytes"`
	StderrBytes      int  `json:"stderr_bytes"`
	StdoutTruncated  bool `json:"stdout_truncated"`
	StderrTruncated  bool `json:"stderr_truncated"`
}

type responseWire struct {
	SchemaVersion string           `json:"schema_version"`
	ResponseID    string           `json:"response_id"`
	Binding       IdentityBinding  `json:"binding"`
	Authority     int              `json:"authority"`
	Accepted      bool             `json:"accepted"`
	Completeness  string           `json:"completeness"`
	Response      SemanticResponse `json:"response"`
}
type ResponseRecord struct{ wire responseWire }
type invocationWire struct {
	SchemaVersion  string          `json:"schema_version"`
	InvocationID   string          `json:"invocation_id"`
	Binding        IdentityBinding `json:"binding"`
	TerminalStatus TerminalStatus  `json:"terminal_status"`
	Receipt        ResourceReceipt `json:"receipt"`
	Response       *responseWire   `json:"response"`
	Authority      int             `json:"authority"`
	Accepted       bool            `json:"accepted"`
	Completeness   string          `json:"completeness"`
}
type InvocationRecord struct{ wire invocationWire }

func NewResponseRecord(binding IdentityBinding, semantic SemanticResponse) (ResponseRecord, error) {
	binding.AttemptID = "" // semantic identity deliberately excludes retry nonce
	w := responseWire{SchemaVersion: ResponseSchema, Binding: cloneBinding(binding), Completeness: CompletenessUnknown, Response: cloneSemantic(semantic)}
	w.ResponseID = identity("lsp-trace.describe-response.identity.v1", w, func(v *responseWire) { v.ResponseID = "" })
	r := ResponseRecord{wire: w}
	return r, r.Validate()
}
func NewInvocationRecord(binding IdentityBinding, status TerminalStatus, receipt ResourceReceipt, response ResponseRecord) (InvocationRecord, error) {
	w := invocationWire{SchemaVersion: InvocationSchema, Binding: cloneBinding(binding), TerminalStatus: status, Receipt: receipt, Completeness: CompletenessUnknown}
	if response.ID() != "" {
		responseWire := response.wire
		w.Response = &responseWire
	}
	w.InvocationID = identity("lsp-trace.describe-invocation.identity.v1", w, func(v *invocationWire) { v.InvocationID = "" })
	r := InvocationRecord{wire: w}
	return r, r.Validate()
}
func (r ResponseRecord) ID() string                 { return r.wire.ResponseID }
func (r ResponseRecord) Authority() int             { return r.wire.Authority }
func (r ResponseRecord) Accepted() bool             { return r.wire.Accepted }
func (r ResponseRecord) Completeness() string       { return r.wire.Completeness }
func (r ResponseRecord) Response() SemanticResponse { return cloneSemantic(r.wire.Response) }
func (r ResponseRecord) Binding() IdentityBinding   { return cloneBinding(r.wire.Binding) }
func (r InvocationRecord) ID() string               { return r.wire.InvocationID }
func (r InvocationRecord) Status() TerminalStatus   { return r.wire.TerminalStatus }
func (r InvocationRecord) Receipt() ResourceReceipt { return r.wire.Receipt }
func (r InvocationRecord) Response() ResponseRecord {
	if r.wire.Response == nil {
		return ResponseRecord{}
	}
	return ResponseRecord{wire: *r.wire.Response}
}
func (r InvocationRecord) Binding() IdentityBinding { return cloneBinding(r.wire.Binding) }
func (r ResponseRecord) Bytes() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.wire)
}
func (r InvocationRecord) Bytes() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.wire)
}
func ParseResponseRecord(raw []byte) (ResponseRecord, error) {
	var w responseWire
	if err := strictCanonical(raw, &w); err != nil {
		return ResponseRecord{}, err
	}
	r := ResponseRecord{wire: w}
	return r, r.Validate()
}
func ParseInvocationRecord(raw []byte) (InvocationRecord, error) {
	var w invocationWire
	if err := strictCanonical(raw, &w); err != nil {
		return InvocationRecord{}, err
	}
	r := InvocationRecord{wire: w}
	return r, r.Validate()
}
func (r ResponseRecord) Validate() error {
	w := r.wire
	id := identity("lsp-trace.describe-response.identity.v1", w, func(v *responseWire) { v.ResponseID = "" })
	if w.SchemaVersion != ResponseSchema || w.ResponseID != id || w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown {
		return errors.New("describe response integrity mismatch")
	}
	if err := validateBinding(w.Binding, false); err != nil {
		return err
	}
	return validateSemantic(w.Response)
}
func (r InvocationRecord) Validate() error {
	w := r.wire
	id := identity("lsp-trace.describe-invocation.identity.v1", w, func(v *invocationWire) { v.InvocationID = "" })
	if w.SchemaVersion != InvocationSchema || w.InvocationID != id || w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || !validStatus(w.TerminalStatus) || w.Receipt.TerminalOutcomes != 1 {
		return errors.New("describe invocation integrity mismatch")
	}
	if err := validateBinding(w.Binding, true); err != nil {
		return err
	}
	if w.TerminalStatus == StatusSucceeded {
		if w.Response == nil {
			return errors.New("successful invocation response missing")
		}
		return ResponseRecord{wire: *w.Response}.Validate()
	}
	if w.Response != nil {
		return errors.New("failed invocation must not contain semantic response")
	}
	return nil
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateBinding(b IdentityBinding, attempt bool) error {
	if b.RequestRecordID == "" || b.MessageID == "" || (attempt && b.AttemptID == "") || b.RuntimeIdentity == "" || b.AdapterIdentity == "" || b.ModelIdentity == "" {
		return errors.New("describe binding missing")
	}
	for _, d := range []string{b.WorkerSHA256, b.ModelSHA256, b.LibrarySHA256, b.SandboxExecutableSHA256, b.SandboxProfileSHA256, b.PromptSHA256, b.GrammarSHA256} {
		if !digestPattern.MatchString(d) {
			return errors.New("describe binding digest invalid")
		}
	}
	return validateLimits(b.Limits)
}
func validateSemantic(s SemanticResponse) error {
	if s.Verdict == "" || s.TargetRole == "" || s.NearestOutwardConsumer == "" || s.ConsumerNeed == "" || s.ProvidedBehavior == "" || s.BoundaryContribution == "" || s.Limitations == nil || s.Citations == nil {
		return errors.New("describe semantic response invalid")
	}
	for _, citation := range s.Citations {
		if citation == "" {
			return errors.New("describe citation invalid")
		}
	}
	return nil
}
func validateLimits(l Limits) error {
	if l.TimeoutMS <= 0 || l.MaxTokens <= 0 || l.ContextTokens <= 0 || l.StdoutBytes <= 0 || l.StderrBytes <= 0 || l.WorkBytes <= 0 || l.TempBytes <= 0 {
		return errors.New("describe limits invalid")
	}
	return nil
}
func validStatus(s TerminalStatus) bool {
	switch s {
	case StatusSucceeded, StatusCancelled, StatusTimeout, StatusResourceLimit, StatusBackendFailure, StatusOutputInvalid, StatusModelUnavailable, StatusPolicyMismatch:
		return true
	}
	return false
}
func cloneSemantic(s SemanticResponse) SemanticResponse {
	s.Limitations = append([]string(nil), s.Limitations...)
	s.Citations = append([]string(nil), s.Citations...)
	return s
}
func cloneBinding(b IdentityBinding) IdentityBinding { return b }
func identity[T any](domain string, v T, clear func(*T)) string {
	clear(&v)
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(append([]byte(domain+"\x00"), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func strictCanonical(raw []byte, out any) error {
	if len(raw) == 0 || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return errors.New("invalid canonical json")
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid canonical json")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing json")
	}
	canonical, _ := json.Marshal(out)
	if !bytes.Equal(raw, canonical) {
		return errors.New("noncanonical json")
	}
	return nil
}
func rejectDuplicateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	return scanValue(d)
}
func scanValue(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := k.(string)
			if !ok {
				return errors.New("invalid json key")
			}
			if _, ok := seen[key]; ok {
				return errors.New("duplicate json key")
			}
			seen[key] = struct{}{}
			if e = scanValue(d); e != nil {
				return e
			}
		}
		_, err = d.Token()
		return err
	case '[':
		for d.More() {
			if err = scanValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	default:
		return fmt.Errorf("invalid json delimiter")
	}
}
