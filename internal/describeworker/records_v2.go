package describeworker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

const ResponseSchemaV2 = "lsp-trace.describe-response.v2"

const (
	ConsumerResolvedV2   = "RESOLVED"
	ConsumerUnresolvedV2 = "UNRESOLVED"
)

type ConsumerNeedV2 struct {
	Status string `json:"status"`
	Value  string `json:"value"`
}
type ProvidedBehaviorV2 struct {
	Value            string `json:"value"`
	ConsumerRelative bool   `json:"consumer_relative"`
}
type CitationSuggestionsV2 struct {
	TargetRole           []string `json:"target_role"`
	ConsumerNeed         []string `json:"consumer_need"`
	ProvidedBehavior     []string `json:"provided_behavior"`
	BoundaryContribution []string `json:"boundary_contribution"`
	Limitations          []string `json:"limitations"`
}
type AttributedCitationSuggestionsV2 struct {
	Label       string                `json:"label"`
	Suggestions CitationSuggestionsV2 `json:"suggestions"`
}
type SemanticResponseV2 struct {
	Verdict              string                 `json:"verdict"`
	TargetRole           string                 `json:"target_role"`
	ConsumerNeed         ConsumerNeedV2         `json:"consumer_need"`
	ProvidedBehavior     ProvidedBehaviorV2     `json:"provided_behavior"`
	BoundaryContribution string                 `json:"boundary_contribution"`
	Limitations          []string               `json:"limitations"`
	CitationSuggestions  *CitationSuggestionsV2 `json:"citation_suggestions,omitempty"`
}
type ConsumerIdentityV2 struct {
	Resolution         string `json:"resolution"`
	AlternativeID      string `json:"alternative_id"`
	AlternativeOrdinal int    `json:"alternative_ordinal"`
}
type HostPinsV2 struct {
	WorkerSHA256  string `json:"worker_sha256"`
	ModelSHA256   string `json:"model_sha256"`
	GrammarSHA256 string `json:"grammar_sha256"`
	PromptSHA256  string `json:"prompt_sha256"`
}
type HostProvenanceV2 struct {
	PacketID               string `json:"packet_id"`
	RequestLineageIdentity string `json:"request_lineage_identity"`
}
type HostCustodyV2 struct {
	GraphDigest string `json:"graph_digest"`
	CaptureID   string `json:"capture_id"`
}
type ResponseHostV2 struct {
	RequestRecordID string             `json:"request_record_id"`
	MessageID       string             `json:"message_id"`
	AttemptID       string             `json:"attempt_id"`
	Consumer        ConsumerIdentityV2 `json:"consumer"`
	Pins            HostPinsV2         `json:"pins"`
	Provenance      HostProvenanceV2   `json:"provenance"`
	Custody         HostCustodyV2      `json:"custody"`
}
type EvidenceBasisV2 struct {
	Label                string   `json:"label"`
	TargetRole           []string `json:"target_role"`
	ConsumerNeed         []string `json:"consumer_need"`
	ProvidedBehavior     []string `json:"provided_behavior"`
	BoundaryContribution []string `json:"boundary_contribution"`
	Limitations          []string `json:"limitations"`
}
type responseWireV2 struct {
	SchemaVersion           string                           `json:"schema_version"`
	ResponseID              string                           `json:"response_id"`
	Authority               int                              `json:"authority"`
	Accepted                bool                             `json:"accepted"`
	Completeness            string                           `json:"completeness"`
	RawStdoutSHA256         string                           `json:"raw_stdout_sha256"`
	RawStdoutLength         int                              `json:"raw_stdout_length"`
	CanonicalSemanticSHA256 string                           `json:"canonical_semantic_sha256"`
	CanonicalSemantic       json.RawMessage                  `json:"canonical_semantic"`
	Host                    ResponseHostV2                   `json:"host"`
	AdmissibleEvidenceBasis EvidenceBasisV2                  `json:"admissible_evidence_basis"`
	CitationSuggestions     *AttributedCitationSuggestionsV2 `json:"citation_suggestions,omitempty"`
}
type ResponseRecordV2 struct{ wire responseWireV2 }

var (
	citationIDV2          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
	errSemanticSyntaxV2   = errors.New("semantic response v2 syntax")
	errSemanticSchemaV2   = errors.New("semantic response v2 schema mismatch")
	errSemanticValueV2    = errors.New("semantic response v2 value invalid")
	errInternalValidateV2 = errors.New("semantic response v2 internal validation")
)

func ParseSemanticResponseV2(raw []byte) (SemanticResponseV2, []byte, error) {
	var semantic SemanticResponseV2
	if len(raw) == 0 || len(raw) > 64*1024 || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) || !json.Valid(raw) || rejectDuplicateJSON(raw) != nil {
		return semantic, nil, errSemanticSyntaxV2
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return semantic, nil, errSemanticSyntaxV2
	}
	allowed := map[string]bool{"verdict": true, "target_role": true, "consumer_need": true, "provided_behavior": true, "boundary_contribution": true, "limitations": true, "citation_suggestions": true}
	for key := range members {
		if !allowed[key] {
			return semantic, nil, errSemanticSchemaV2
		}
	}
	for _, key := range []string{"verdict", "target_role", "consumer_need", "provided_behavior", "boundary_contribution", "limitations"} {
		if _, ok := members[key]; !ok {
			return semantic, nil, errSemanticSchemaV2
		}
	}
	if suggestion, present := members["citation_suggestions"]; present && bytes.Equal(bytes.TrimSpace(suggestion), []byte("null")) {
		return semantic, nil, errSemanticValueV2
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&semantic); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return semantic, nil, errSemanticValueV2
		}
		return semantic, nil, errSemanticSchemaV2
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return semantic, nil, errSemanticSyntaxV2
	}
	if err := validateSemanticV2(semantic); err != nil {
		return semantic, nil, errSemanticValueV2
	}
	canonical, err := json.Marshal(semantic)
	if err != nil {
		return semantic, nil, errInternalValidateV2
	}
	return cloneSemanticV2(semantic), canonical, nil
}

func validateSemanticV2(s SemanticResponseV2) error {
	if s.Verdict != "COMPLETE" && s.Verdict != "ABSTAINED" {
		return errors.New("invalid semantic response v2 verdict")
	}
	if !boundedV2(s.TargetRole, false) || !boundedV2(s.ProvidedBehavior.Value, false) || !boundedV2(s.BoundaryContribution, false) || s.Limitations == nil || len(s.Limitations) > 8 {
		return errors.New("invalid semantic response v2 bounds")
	}
	if s.ConsumerNeed.Status != ConsumerResolvedV2 && s.ConsumerNeed.Status != ConsumerUnresolvedV2 {
		return errors.New("invalid semantic response v2 consumer status")
	}
	if s.ConsumerNeed.Status == ConsumerResolvedV2 && !boundedV2(s.ConsumerNeed.Value, false) || s.ConsumerNeed.Status == ConsumerUnresolvedV2 && s.ConsumerNeed.Value != "" {
		return errors.New("invalid semantic response v2 consumer need")
	}
	for _, limitation := range s.Limitations {
		if !boundedV2(limitation, false) {
			return errors.New("invalid semantic response v2 limitation")
		}
	}
	if s.CitationSuggestions != nil {
		for _, ids := range [][]string{s.CitationSuggestions.TargetRole, s.CitationSuggestions.ConsumerNeed, s.CitationSuggestions.ProvidedBehavior, s.CitationSuggestions.BoundaryContribution, s.CitationSuggestions.Limitations} {
			if len(ids) > 8 {
				return errors.New("invalid semantic response v2 citation suggestions")
			}
			seen := map[string]bool{}
			for _, id := range ids {
				if !knownCitationSuggestionV2(id) || seen[id] {
					return errors.New("invalid semantic response v2 citation suggestions")
				}
				seen[id] = true
			}
		}
	}
	return nil
}

func boundedV2(s string, empty bool) bool {
	return (empty || s != "") && len(s) <= 4096
}

func knownCitationSuggestionV2(id string) bool {
	switch id {
	case "C1", "C2", "PACKET_SCOPE", "UNRESOLVED_CUSTODY":
		return true
	}
	return false
}

func NewResponseRecordV2(raw []byte, host ResponseHostV2) (ResponseRecordV2, error) {
	semantic, _, err := ParseSemanticResponseV2(raw)
	if err != nil {
		return ResponseRecordV2{}, err
	}
	var suggestions *AttributedCitationSuggestionsV2
	if semantic.CitationSuggestions != nil {
		copy := cloneSuggestionsV2(*semantic.CitationSuggestions)
		suggestions = &AttributedCitationSuggestionsV2{Label: "MODEL_ATTRIBUTED_NON_AUTHORITATIVE", Suggestions: copy}
		semantic.CitationSuggestions = nil
	}
	canonical, _ := json.Marshal(semantic)
	rawSum := sha256.Sum256(raw)
	canonicalSum := sha256.Sum256(canonical)
	wire := responseWireV2{
		SchemaVersion: ResponseSchemaV2, Completeness: CompletenessUnknown,
		RawStdoutSHA256: "sha256:" + hex.EncodeToString(rawSum[:]), RawStdoutLength: len(raw),
		CanonicalSemanticSHA256: "sha256:" + hex.EncodeToString(canonicalSum[:]), CanonicalSemantic: append(json.RawMessage(nil), canonical...),
		Host: cloneHostV2(host), AdmissibleEvidenceBasis: evidenceBasisV2(host, canonical), CitationSuggestions: suggestions,
	}
	wire.ResponseID = responseIdentityV2(wire)
	record := ResponseRecordV2{wire: wire}
	return record, record.Validate()
}

func evidenceBasisV2(host ResponseHostV2, canonical []byte) EvidenceBasisV2 {
	var semantic SemanticResponseV2
	_ = json.Unmarshal(canonical, &semantic)
	consumer := []string{"C2"}
	boundary := []string{"C1", "C2"}
	if host.Consumer.Resolution == ConsumerUnresolvedV2 {
		consumer = []string{"UNRESOLVED_CUSTODY"}
		boundary = []string{"C1"}
	}
	provided := []string{"C1"}
	if semantic.ProvidedBehavior.ConsumerRelative && host.Consumer.Resolution == ConsumerResolvedV2 {
		provided = append(provided, "C2")
	}
	return EvidenceBasisV2{Label: "admissible_evidence_basis", TargetRole: []string{"C1"}, ConsumerNeed: consumer, ProvidedBehavior: provided, BoundaryContribution: boundary, Limitations: []string{"PACKET_SCOPE"}}
}

func ParseResponseRecordV2(raw []byte) (ResponseRecordV2, error) {
	var wire responseWireV2
	if len(raw) == 0 || rejectDuplicateJSON(raw) != nil {
		return ResponseRecordV2{}, errors.New("invalid response record v2")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return ResponseRecordV2{}, errors.New("invalid response record v2")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ResponseRecordV2{}, errors.New("trailing response record v2")
	}
	canonical, _ := json.Marshal(wire)
	if !bytes.Equal(raw, canonical) {
		return ResponseRecordV2{}, errors.New("noncanonical response record v2")
	}
	record := ResponseRecordV2{wire: wire}
	return record, record.Validate()
}

func (r ResponseRecordV2) Validate() error {
	wire := r.wire
	if wire.SchemaVersion != ResponseSchemaV2 || wire.ResponseID != responseIdentityV2(wire) || wire.Authority != 0 || wire.Accepted || wire.Completeness != CompletenessUnknown || !digestPattern.MatchString(wire.RawStdoutSHA256) || wire.RawStdoutLength <= 0 || !digestPattern.MatchString(wire.CanonicalSemanticSHA256) {
		return errors.New("response record v2 integrity mismatch")
	}
	_, canonical, err := ParseSemanticResponseV2(wire.CanonicalSemantic)
	if err != nil {
		return errors.New("response record v2 canonical semantic invalid: " + err.Error())
	}
	if !bytes.Equal(canonical, wire.CanonicalSemantic) {
		return errors.New("response record v2 canonical semantic mismatch")
	}
	if digestOfBytesV2(canonical) != wire.CanonicalSemanticSHA256 {
		return errors.New("response record v2 canonical digest mismatch")
	}
	if err := validateHostV2(wire.Host); err != nil {
		return errors.New("response record v2 host mismatch: " + err.Error())
	}
	expectedBasis := evidenceBasisV2(wire.Host, canonical)
	basisBytes, _ := json.Marshal(wire.AdmissibleEvidenceBasis)
	expectedBytes, _ := json.Marshal(expectedBasis)
	if !bytes.Equal(basisBytes, expectedBytes) {
		return errors.New("response record v2 evidence basis mismatch")
	}
	if wire.CitationSuggestions != nil {
		if wire.CitationSuggestions.Label != "MODEL_ATTRIBUTED_NON_AUTHORITATIVE" {
			return errors.New("response record v2 citation attribution mismatch")
		}
		candidate := SemanticResponseV2{Verdict: "COMPLETE", TargetRole: "x", ConsumerNeed: ConsumerNeedV2{Status: ConsumerUnresolvedV2}, ProvidedBehavior: ProvidedBehaviorV2{Value: "x"}, BoundaryContribution: "x", Limitations: []string{}, CitationSuggestions: &wire.CitationSuggestions.Suggestions}
		if err := validateSemanticV2(candidate); err != nil {
			return err
		}
	}
	return nil
}

func validateHostV2(host ResponseHostV2) error {
	if host.RequestRecordID == "" || host.MessageID == "" || host.AttemptID == "" || host.Consumer.AlternativeID == "" || host.Consumer.AlternativeOrdinal < 0 || host.Provenance.PacketID == "" || host.Provenance.RequestLineageIdentity == "" || host.Custody.CaptureID == "" {
		return errors.New("response host v2 missing")
	}
	if host.Consumer.Resolution != ConsumerResolvedV2 && host.Consumer.Resolution != ConsumerUnresolvedV2 {
		return errors.New("response host v2 consumer invalid")
	}
	if host.Consumer.Resolution == ConsumerUnresolvedV2 && (host.Consumer.AlternativeID != "OUTWARD_CONSUMER_UNRESOLVED" || host.Consumer.AlternativeOrdinal != 0) {
		return errors.New("response host v2 unresolved mismatch")
	}
	for _, digest := range []string{host.Pins.WorkerSHA256, host.Pins.ModelSHA256, host.Pins.GrammarSHA256, host.Pins.PromptSHA256, host.Custody.GraphDigest} {
		if !digestPattern.MatchString(digest) {
			return errors.New("response host v2 digest invalid")
		}
	}
	return nil
}

func responseIdentityV2(wire responseWireV2) string {
	wire.ResponseID = ""
	encoded, _ := json.Marshal(wire)
	sum := sha256.Sum256(append([]byte("lsp-trace.describe-response.identity.v2\x00"), encoded...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func digestOfBytesV2(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func digestOfV2(s string) string { return digestOfBytesV2([]byte(s)) }
func cloneSuggestionsV2(s CitationSuggestionsV2) CitationSuggestionsV2 {
	s.TargetRole = append([]string(nil), s.TargetRole...)
	s.ConsumerNeed = append([]string(nil), s.ConsumerNeed...)
	s.ProvidedBehavior = append([]string(nil), s.ProvidedBehavior...)
	s.BoundaryContribution = append([]string(nil), s.BoundaryContribution...)
	s.Limitations = append([]string(nil), s.Limitations...)
	return s
}
func cloneSemanticV2(s SemanticResponseV2) SemanticResponseV2 {
	if s.Limitations != nil {
		cloned := make([]string, len(s.Limitations))
		copy(cloned, s.Limitations)
		s.Limitations = cloned
	}
	if s.CitationSuggestions != nil {
		copy := cloneSuggestionsV2(*s.CitationSuggestions)
		s.CitationSuggestions = &copy
	}
	return s
}
func cloneHostV2(h ResponseHostV2) ResponseHostV2 { return h }
func cloneBasisV2(b EvidenceBasisV2) EvidenceBasisV2 {
	b.TargetRole = append([]string(nil), b.TargetRole...)
	b.ConsumerNeed = append([]string(nil), b.ConsumerNeed...)
	b.ProvidedBehavior = append([]string(nil), b.ProvidedBehavior...)
	b.BoundaryContribution = append([]string(nil), b.BoundaryContribution...)
	b.Limitations = append([]string(nil), b.Limitations...)
	return b
}

func (r ResponseRecordV2) ID() string { return r.wire.ResponseID }
func (r ResponseRecordV2) Bytes() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.wire)
}
func (r ResponseRecordV2) CanonicalSemanticDigest() string { return r.wire.CanonicalSemanticSHA256 }
func (r ResponseRecordV2) RawStdoutDigest() string         { return r.wire.RawStdoutSHA256 }
func (r ResponseRecordV2) RawStdoutLength() int            { return r.wire.RawStdoutLength }
func (r ResponseRecordV2) Consumer() ConsumerIdentityV2    { return r.wire.Host.Consumer }
func (r ResponseRecordV2) Host() ResponseHostV2            { return cloneHostV2(r.wire.Host) }
func (r ResponseRecordV2) Semantic() SemanticResponseV2 {
	var s SemanticResponseV2
	_ = json.Unmarshal(r.wire.CanonicalSemantic, &s)
	return cloneSemanticV2(s)
}
func (r ResponseRecordV2) AdmissibleEvidenceBasis() EvidenceBasisV2 {
	return cloneBasisV2(r.wire.AdmissibleEvidenceBasis)
}
func (r ResponseRecordV2) CitationSuggestions() AttributedCitationSuggestionsV2 {
	if r.wire.CitationSuggestions == nil {
		return AttributedCitationSuggestionsV2{}
	}
	return AttributedCitationSuggestionsV2{Label: r.wire.CitationSuggestions.Label, Suggestions: cloneSuggestionsV2(r.wire.CitationSuggestions.Suggestions)}
}

var _ = citationIDV2
