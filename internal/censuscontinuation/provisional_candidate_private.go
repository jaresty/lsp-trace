package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"lsp-trace/internal/describerequest"
)

const (
	privateProvisionalCandidateSchema         = "lsp-trace.census-provisional-candidate.private.v1"
	privateProvisionalCandidateStatus         = "PROVISIONAL_CANDIDATES_PREPARED"
	privateProvisionalCandidateIdentityDomain = "lsp-trace:census-provisional-candidate-artifact:private:v1"
	privateProvisionalDescriptionIDDomain     = "lsp-trace:census-provisional-description:private:v1"
	privateProvisionalCandidateIDDomain       = "lsp-trace:census-provisional-candidate:private:v1"
	privateProvisionalCandidateMaxBytes       = 64 << 20
)

const (
	privateDispositionCandidate  = "PROVISIONAL_CANDIDATE"
	privateDispositionPartial    = "PARTIAL_DESCRIPTION"
	privateDispositionFailed     = "FAILED_DESCRIPTION"
	privateDispositionRefused    = "REFUSED_DESCRIPTION"
	privateDispositionUnresolved = "UNRESOLVED_DESCRIPTION"
)

type privateProvisionalCandidateWire struct {
	SchemaVersion string `json:"schema_version"`
	ArtifactID    string `json:"artifact_id"`
	Status        string `json:"status"`

	Authority                   int    `json:"authority"`
	Accepted                    bool   `json:"accepted"`
	Completeness                string `json:"completeness"`
	FeatureIdentity             string `json:"feature_identity"`
	SemanticQualification       string `json:"semantic_qualification"`
	AutomaticProgressionAllowed bool   `json:"automatic_progression_allowed"`

	AdmissionSelector   string `json:"admission_selector"`
	AdmissionDigest     string `json:"admission_digest"`
	AdmissionByteLength uint64 `json:"admission_byte_length"`
	AdmissionID         string `json:"admission_id"`

	ResultSelector   string `json:"result_selector"`
	ResultDigest     string `json:"result_digest"`
	ResultByteLength uint64 `json:"result_byte_length"`
	ResultID         string `json:"result_id"`

	PreparedSelector   string `json:"prepared_selector"`
	PreparedDigest     string `json:"prepared_digest"`
	PreparedByteLength uint64 `json:"prepared_byte_length"`
	PreparedID         string `json:"prepared_id"`

	CheckpointSelector   string `json:"checkpoint_selector"`
	CheckpointDigest     string `json:"checkpoint_digest"`
	CheckpointByteLength uint64 `json:"checkpoint_byte_length"`
	CheckpointID         string `json:"checkpoint_id"`
	ChainIdentity        string `json:"chain_identity"`

	RequestsSelector   string `json:"requests_selector"`
	RequestsDigest     string `json:"requests_digest"`
	RequestsByteLength uint64 `json:"requests_byte_length"`
	AdmittedRowsCount  int    `json:"admitted_rows_count"`

	OriginalAccounting privateProvisionalOriginalAccounting `json:"original_accounting"`
	Descriptions       []privateProvisionalDescription      `json:"descriptions"`
	Candidates         []privateProvisionalCandidate        `json:"candidates"`
}

type privateProvisionalOriginalAccounting struct {
	Status        string `json:"status"`
	RequestsCount *int   `json:"requests_count"`
	PreparedCount *int   `json:"prepared_count"`
}

type privateProvisionalDescription struct {
	DescriptionID   string          `json:"description_id"`
	Ordinal         int             `json:"ordinal"`
	RecordID        string          `json:"record_id"`
	MessageID       string          `json:"message_id"`
	LineageIdentity string          `json:"lineage_identity"`
	Disposition     string          `json:"disposition"`
	OutcomeKind     string          `json:"outcome_kind"`
	OutcomeDigest   string          `json:"outcome_digest"`
	OutcomeLength   uint64          `json:"outcome_byte_length"`
	Outcome         json.RawMessage `json:"outcome"`
}

type privateProvisionalCandidate struct {
	CandidateID         string   `json:"candidate_id"`
	DescriptionID       string   `json:"description_id"`
	Revision            int      `json:"revision"`
	Status              string   `json:"status"`
	Label               string   `json:"label"`
	Purpose             string   `json:"purpose"`
	Value               string   `json:"value"`
	Alternatives        []string `json:"alternatives"`
	Evidence            []string `json:"evidence"`
	ContraryEvidence    []string `json:"contrary_evidence"`
	RejectionConditions []string `json:"rejection_conditions"`
	InventoryDelta      []string `json:"inventory_delta"`
	SourceRefs          []string `json:"source_refs"`
	Authority           int      `json:"authority"`
	Accepted            bool     `json:"accepted"`
	Completeness        string   `json:"completeness"`
	FeatureIdentity     string   `json:"feature_identity"`
}

type privateProvisionalArtifact struct {
	wire privateProvisionalCandidateWire
}

func (a privateProvisionalArtifact) ID() string { return a.wire.ArtifactID }

func (a privateProvisionalArtifact) Bytes() ([]byte, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a.wire)
	if err != nil || len(raw) > privateProvisionalCandidateMaxBytes {
		return nil, errors.New("private provisional candidate byte limit")
	}
	return raw, nil
}

func parsePrivateProvisionalCandidate(raw []byte) (privateProvisionalArtifact, error) {
	if len(raw) > privateProvisionalCandidateMaxBytes {
		return privateProvisionalArtifact{}, errors.New("private provisional candidate byte limit")
	}
	var wire privateProvisionalCandidateWire
	if err := strictCanonical(raw, &wire); err != nil {
		return privateProvisionalArtifact{}, err
	}
	artifact := privateProvisionalArtifact{wire: wire}
	return artifact, artifact.validate()
}

func (a privateProvisionalArtifact) validate() error {
	w := a.wire
	if w.SchemaVersion != privateProvisionalCandidateSchema || w.Status != privateProvisionalCandidateStatus ||
		w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || w.FeatureIdentity != "UNRESOLVED" ||
		w.SemanticQualification != "NOT_PERFORMED" || w.AutomaticProgressionAllowed || !validDigest(w.ArtifactID) ||
		!validDigest(w.AdmissionSelector) || w.AdmissionDigest != w.AdmissionSelector || w.AdmissionByteLength == 0 || !validDigest(w.AdmissionID) ||
		!validDigest(w.ResultSelector) || w.ResultDigest != w.ResultSelector || w.ResultByteLength == 0 || !validDigest(w.ResultID) ||
		!validDigest(w.PreparedSelector) || w.PreparedDigest != w.PreparedSelector || w.PreparedByteLength == 0 || !validDigest(w.PreparedID) ||
		!validDigest(w.CheckpointSelector) || w.CheckpointDigest != w.CheckpointSelector || w.CheckpointByteLength == 0 || !validDigest(w.CheckpointID) || !validDigest(w.ChainIdentity) ||
		!validDigest(w.RequestsSelector) || w.RequestsDigest != w.RequestsSelector || w.RequestsByteLength == 0 || w.AdmittedRowsCount < 0 ||
		w.OriginalAccounting.Status != "UNAVAILABLE" || w.OriginalAccounting.RequestsCount != nil || w.OriginalAccounting.PreparedCount != nil ||
		w.Descriptions == nil || w.Candidates == nil || len(w.Descriptions) != w.AdmittedRowsCount {
		return errors.New("private provisional candidate integrity mismatch")
	}
	candidateByDescription := make(map[string]bool, len(w.Candidates))
	for i, d := range w.Descriptions {
		if d.Ordinal != i || !validDigest(d.DescriptionID) || !validUnprefixedIdentity(d.RecordID) || !validUnprefixedIdentity(d.MessageID) || !validUnprefixedIdentity(d.LineageIdentity) ||
			!validDigest(d.OutcomeDigest) || d.OutcomeLength == 0 || len(d.Outcome) == 0 || digestOf(d.Outcome) != d.OutcomeDigest || uint64(len(d.Outcome)) != d.OutcomeLength || validateRetainedOutcome(d.Outcome) != nil ||
			privateDispositionForOutcome(d.OutcomeKind) != d.Disposition {
			return errors.New("private provisional description mismatch")
		}
		var kind privateCallerOutcomeKind
		if json.Unmarshal(d.Outcome, &kind) != nil || kind.Kind != d.OutcomeKind {
			return errors.New("private provisional description outcome mismatch")
		}
		want := identityJSON(privateProvisionalDescriptionIDDomain, d, func(v *privateProvisionalDescription) { v.DescriptionID = "" })
		if want != d.DescriptionID {
			return errors.New("private provisional description identity mismatch")
		}
	}
	for _, c := range w.Candidates {
		if !validDigest(c.CandidateID) || !validDigest(c.DescriptionID) || c.Revision != 1 || c.Status != "PROVISIONAL" ||
			c.Label != "" || c.Purpose != "" || c.Value != "" || c.Alternatives == nil || c.Evidence == nil || c.ContraryEvidence == nil || c.RejectionConditions == nil || c.InventoryDelta == nil || c.SourceRefs == nil ||
			len(c.Alternatives)+len(c.Evidence)+len(c.ContraryEvidence)+len(c.RejectionConditions)+len(c.InventoryDelta)+len(c.SourceRefs) != 0 ||
			c.Authority != 0 || c.Accepted || c.Completeness != ContinuationCompleteness || c.FeatureIdentity != "UNRESOLVED" || candidateByDescription[c.DescriptionID] {
			return errors.New("private provisional candidate row mismatch")
		}
		found := false
		for _, d := range w.Descriptions {
			if d.DescriptionID == c.DescriptionID && d.Disposition == privateDispositionCandidate {
				found = true
				break
			}
		}
		if !found || identityJSON(privateProvisionalCandidateIDDomain, c, func(v *privateProvisionalCandidate) { v.CandidateID = "" }) != c.CandidateID {
			return errors.New("private provisional candidate identity mismatch")
		}
		candidateByDescription[c.DescriptionID] = true
	}
	for _, d := range w.Descriptions {
		if candidateByDescription[d.DescriptionID] != (d.Disposition == privateDispositionCandidate) {
			return errors.New("private provisional candidate closure mismatch")
		}
	}
	if identityJSON(privateProvisionalCandidateIdentityDomain, w, func(v *privateProvisionalCandidateWire) { v.ArtifactID = "" }) != w.ArtifactID {
		return errors.New("private provisional artifact identity mismatch")
	}
	return nil
}

func privateDispositionForOutcome(kind string) string {
	switch kind {
	case privateCallerOutcomeSuccess:
		return privateDispositionCandidate
	case privateCallerOutcomePartial:
		return privateDispositionPartial
	case privateCallerOutcomeFailed:
		return privateDispositionFailed
	case privateCallerOutcomeRefused:
		return privateDispositionRefused
	case privateCallerOutcomeUnknown:
		return privateDispositionUnresolved
	default:
		return ""
	}
}

// PreparePrivateProvisionalCandidateSelector consumes exactly one mechanically
// admitted caller result and prepares authority-zero descriptions and candidates.
func PreparePrivateProvisionalCandidateSelector(ctx context.Context, store Store, admissionSelector string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if store == nil || !validDigest(admissionSelector) {
		return "", errors.New("private provisional candidate input invalid")
	}
	admissionRaw, err := store.Get(ctx, admissionSelector)
	if err != nil || digestOf(admissionRaw) != admissionSelector {
		return "", errors.New("private provisional candidate admission selector mismatch")
	}
	admission, err := parsePrivateCallerResultAdmission(admissionRaw)
	if err != nil {
		return "", err
	}
	resultRaw, err := store.Get(ctx, admission.wire.ResultSelector)
	if err != nil || digestOf(resultRaw) != admission.wire.ResultDigest || uint64(len(resultRaw)) != admission.wire.ResultByteLength {
		return "", errors.New("private provisional candidate result bytes mismatch")
	}
	result, err := parsePrivateCallerResult(resultRaw)
	if err != nil || result.ID() != admission.wire.ResultID {
		return "", errors.New("private provisional candidate result binding mismatch")
	}
	preparedRaw, err := store.Get(ctx, admission.wire.PreparedSelector)
	if err != nil || digestOf(preparedRaw) != admission.wire.PreparedDigest || uint64(len(preparedRaw)) != admission.wire.PreparedByteLength {
		return "", errors.New("private provisional candidate prepared bytes mismatch")
	}
	prepared, err := parsePrivateHandoffPrepared(preparedRaw)
	if err != nil || prepared.ID() != admission.wire.PreparedID {
		return "", errors.New("private provisional candidate prepared binding mismatch")
	}
	chain, err := VerifyChain(ctx, store, admission.wire.CheckpointSelector)
	if err != nil || len(chain) == 0 {
		return "", errors.New("private provisional candidate checkpoint chain invalid")
	}
	terminal := chain[len(chain)-1]
	if terminal.Stage() != StageRequestsRendered || terminal.Status() != StatusRunning || terminal.ID() != admission.wire.CheckpointID ||
		privateCallerResultAdmissionChainIdentity(chain) != admission.wire.ChainIdentity || prepared.wire.CheckpointSelector != admission.wire.CheckpointSelector {
		return "", errors.New("private provisional candidate checkpoint binding mismatch")
	}
	checkpointRaw, err := store.Get(ctx, admission.wire.CheckpointSelector)
	if err != nil || digestOf(checkpointRaw) != admission.wire.CheckpointDigest || uint64(len(checkpointRaw)) != admission.wire.CheckpointByteLength {
		return "", errors.New("private provisional candidate checkpoint bytes mismatch")
	}
	checkpoint, err := ParseCheckpoint(checkpointRaw)
	if err != nil || !reflect.DeepEqual(checkpoint.wire, terminal.wire) {
		return "", errors.New("private provisional candidate checkpoint mismatch")
	}
	var requestsRef ArtifactRef
	for _, ref := range checkpoint.Artifacts() {
		if ref.Kind == "requests" {
			if requestsRef.Kind != "" {
				return "", errors.New("private provisional candidate requests ambiguous")
			}
			requestsRef = ref
		}
	}
	if requestsRef.Kind == "" || requestsRef.ID != admission.wire.RequestsSelector || requestsRef.Digest != admission.wire.RequestsDigest || requestsRef.ByteLength != admission.wire.RequestsByteLength {
		return "", errors.New("private provisional candidate requests binding mismatch")
	}
	requestsRaw, err := store.Get(ctx, admission.wire.RequestsSelector)
	if err != nil || digestOf(requestsRaw) != admission.wire.RequestsDigest || uint64(len(requestsRaw)) != admission.wire.RequestsByteLength {
		return "", errors.New("private provisional candidate requests bytes mismatch")
	}
	requests, err := describerequest.Parse(requestsRaw)
	if err != nil || len(requests) != admission.wire.RequestsCount || len(result.wire.Results) != len(requests) || len(admission.wire.Rows) != len(requests) {
		return "", errors.New("private provisional candidate row count mismatch")
	}
	descriptions := make([]privateProvisionalDescription, len(requests))
	candidates := make([]privateProvisionalCandidate, 0, len(requests))
	for i, request := range requests {
		admitted, retained := admission.wire.Rows[i], result.wire.Results[i]
		if admitted.Ordinal != i || retained.RequestOrdinal != i || admitted.RecordID != request.RecordID || admitted.MessageID != request.Envelope.MessageID || admitted.LineageIdentity != request.Lineage.LineageIdentity ||
			retained.RecordID != admitted.RecordID || retained.MessageID != admitted.MessageID || retained.LineageIdentity != admitted.LineageIdentity {
			return "", errors.New("private provisional candidate row substitution")
		}
		var kind privateCallerOutcomeKind
		if json.Unmarshal(retained.Outcome, &kind) != nil || privateDispositionForOutcome(kind.Kind) == "" {
			return "", errors.New("private provisional candidate outcome invalid")
		}
		description := privateProvisionalDescription{Ordinal: i, RecordID: admitted.RecordID, MessageID: admitted.MessageID, LineageIdentity: admitted.LineageIdentity, Disposition: privateDispositionForOutcome(kind.Kind), OutcomeKind: kind.Kind, OutcomeDigest: digestOf(retained.Outcome), OutcomeLength: uint64(len(retained.Outcome)), Outcome: append(json.RawMessage(nil), retained.Outcome...)}
		description.DescriptionID = identityJSON(privateProvisionalDescriptionIDDomain, description, func(v *privateProvisionalDescription) { v.DescriptionID = "" })
		descriptions[i] = description
		if kind.Kind == privateCallerOutcomeSuccess {
			candidate := privateProvisionalCandidate{DescriptionID: description.DescriptionID, Revision: 1, Status: "PROVISIONAL", Alternatives: []string{}, Evidence: []string{}, ContraryEvidence: []string{}, RejectionConditions: []string{}, InventoryDelta: []string{}, SourceRefs: []string{}, Completeness: ContinuationCompleteness, FeatureIdentity: "UNRESOLVED"}
			candidate.CandidateID = identityJSON(privateProvisionalCandidateIDDomain, candidate, func(v *privateProvisionalCandidate) { v.CandidateID = "" })
			candidates = append(candidates, candidate)
		}
	}
	for _, observed := range []struct {
		selector string
		raw      []byte
	}{{admissionSelector, admissionRaw}, {admission.wire.ResultSelector, resultRaw}, {admission.wire.PreparedSelector, preparedRaw}, {admission.wire.CheckpointSelector, checkpointRaw}, {admission.wire.RequestsSelector, requestsRaw}} {
		again, getErr := store.Get(ctx, observed.selector)
		if getErr != nil || !bytes.Equal(again, observed.raw) {
			return "", errors.New("private provisional candidate observed inconsistent predecessor")
		}
	}
	wire := privateProvisionalCandidateWire{SchemaVersion: privateProvisionalCandidateSchema, Status: privateProvisionalCandidateStatus, Completeness: ContinuationCompleteness, FeatureIdentity: "UNRESOLVED", SemanticQualification: "NOT_PERFORMED", AdmissionSelector: admissionSelector, AdmissionDigest: admissionSelector, AdmissionByteLength: uint64(len(admissionRaw)), AdmissionID: admission.ID(), ResultSelector: admission.wire.ResultSelector, ResultDigest: digestOf(resultRaw), ResultByteLength: uint64(len(resultRaw)), ResultID: result.ID(), PreparedSelector: admission.wire.PreparedSelector, PreparedDigest: digestOf(preparedRaw), PreparedByteLength: uint64(len(preparedRaw)), PreparedID: prepared.ID(), CheckpointSelector: admission.wire.CheckpointSelector, CheckpointDigest: digestOf(checkpointRaw), CheckpointByteLength: uint64(len(checkpointRaw)), CheckpointID: checkpoint.ID(), ChainIdentity: admission.wire.ChainIdentity, RequestsSelector: admission.wire.RequestsSelector, RequestsDigest: digestOf(requestsRaw), RequestsByteLength: uint64(len(requestsRaw)), AdmittedRowsCount: len(requests), OriginalAccounting: privateProvisionalOriginalAccounting{Status: "UNAVAILABLE"}, Descriptions: descriptions, Candidates: candidates}
	wire.ArtifactID = identityJSON(privateProvisionalCandidateIdentityDomain, wire, func(v *privateProvisionalCandidateWire) { v.ArtifactID = "" })
	artifact := privateProvisionalArtifact{wire: wire}
	raw, err := artifact.Bytes()
	if err != nil {
		return "", err
	}
	parsed, err := parsePrivateProvisionalCandidate(raw)
	if err != nil || parsed.ID() != artifact.ID() {
		return "", errors.New("private provisional candidate self validation failed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector, err := store.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	if selector != digestOf(raw) {
		return "", errors.New("private provisional candidate store selector mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return "", err
	}
	readbackArtifact, err := parsePrivateProvisionalCandidate(readback)
	if err != nil || !bytes.Equal(readback, raw) || digestOf(readback) != selector || readbackArtifact.ID() != artifact.ID() {
		return "", errors.New("private provisional candidate readback mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return selector, nil
}
