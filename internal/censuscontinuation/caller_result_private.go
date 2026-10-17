package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
)

const (
	privateCallerResultSubmissionSchema = "lsp-trace.census-caller-result-submission.private.v1"
	privateCallerResultSchema           = "lsp-trace.census-caller-result.private.v1"
	privateCallerResultStatus           = "CALLER_RESULT_RETAINED"
	privateCallerResultCustody          = "CALLER_CONTROLLED_INFERENCE"
	privateCallerResultObserved         = "CALLER_OBSERVED"
	privateCallerResultSynthetic        = "CALLER_SYNTHETIC"
	privateCallerResultMaxBytes         = 64 << 20
	privateCallerResultMaxTextBytes     = 4096
	privateCallerResultMaxAmbiguities   = 16
	privateCallerResultMaxOptions       = 16
	privateCallerResultIdentityDomain   = "lsp-trace:census-caller-result:private:v1"
)

const (
	privateCallerOutcomeSuccess = "SUCCESS"
	privateCallerOutcomePartial = "PARTIAL"
	privateCallerOutcomeFailed  = "FAILED"
	privateCallerOutcomeRefused = "REFUSED"
	privateCallerOutcomeUnknown = "UNKNOWN"
)

type privateCallerResultSubmissionWire struct {
	SchemaVersion string                             `json:"schema_version"`
	Provenance    string                             `json:"provenance"`
	Results       []privateCallerResultSubmissionRow `json:"results"`
}

type privateCallerResultSubmissionRow struct {
	RequestOrdinal  int             `json:"request_ordinal"`
	RecordID        string          `json:"record_id"`
	MessageID       string          `json:"message_id"`
	LineageIdentity string          `json:"lineage_identity"`
	Outcome         json.RawMessage `json:"outcome"`
}

type privateCallerResultWire struct {
	SchemaVersion string `json:"schema_version"`
	ResultID      string `json:"result_id"`
	Status        string `json:"status"`
	Authority     int    `json:"authority"`
	Accepted      bool   `json:"accepted"`
	Completeness  string `json:"completeness"`
	Custody       string `json:"custody"`
	Provenance    string `json:"provenance"`

	PreparedSelector   string `json:"prepared_selector"`
	PreparedID         string `json:"prepared_id"`
	PreparedDigest     string `json:"prepared_digest"`
	PreparedByteLength uint64 `json:"prepared_byte_length"`

	CheckpointSelector string `json:"checkpoint_selector"`
	CheckpointID       string `json:"checkpoint_id"`
	ChainIdentity      string `json:"chain_identity"`

	RequestsSelector   string `json:"requests_selector"`
	RequestsDigest     string `json:"requests_digest"`
	RequestsByteLength uint64 `json:"requests_byte_length"`
	RequestsCount      int    `json:"requests_count"`

	Results []privateCallerResultRow `json:"results"`
}

type privateCallerResultRow struct {
	RequestOrdinal  int             `json:"request_ordinal"`
	RecordID        string          `json:"record_id"`
	MessageID       string          `json:"message_id"`
	LineageIdentity string          `json:"lineage_identity"`
	Outcome         json.RawMessage `json:"outcome"`
}

type privateCallerSemanticSubmission struct {
	Kind     string          `json:"kind"`
	Semantic json.RawMessage `json:"semantic"`
}

type privateCallerSemanticOutcome struct {
	Kind                  string          `json:"kind"`
	RawSemanticSHA256     string          `json:"raw_semantic_sha256"`
	RawSemanticByteLength uint64          `json:"raw_semantic_byte_length"`
	CanonicalSemantic     json.RawMessage `json:"canonical_semantic"`
}

type privateCallerFailure struct {
	Category  string `json:"category"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	Retryable bool   `json:"retryable"`
}

type privateCallerFailureOutcome struct {
	Kind    string               `json:"kind"`
	Failure privateCallerFailure `json:"failure"`
}

type privateCallerAmbiguity struct {
	Code    string   `json:"code"`
	Subject string   `json:"subject"`
	Options []string `json:"options"`
}

type privateCallerUnknownOutcome struct {
	Kind        string                   `json:"kind"`
	Ambiguities []privateCallerAmbiguity `json:"ambiguities"`
}

type privateCallerOutcomeKind struct {
	Kind string `json:"kind"`
}

type privateCallerResult struct{ wire privateCallerResultWire }

func (r privateCallerResult) ID() string { return r.wire.ResultID }

func (r privateCallerResult) Bytes() ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r.wire)
	if err != nil || len(raw) > privateCallerResultMaxBytes {
		return nil, errors.New("private caller result byte limit")
	}
	return raw, nil
}

func parsePrivateCallerResult(raw []byte) (privateCallerResult, error) {
	if len(raw) > privateCallerResultMaxBytes {
		return privateCallerResult{}, errors.New("private caller result byte limit")
	}
	var wire privateCallerResultWire
	if err := strictCanonical(raw, &wire); err != nil {
		return privateCallerResult{}, err
	}
	result := privateCallerResult{wire: wire}
	return result, result.validate()
}

func (r privateCallerResult) validate() error {
	w := r.wire
	if w.SchemaVersion != privateCallerResultSchema || w.Status != privateCallerResultStatus ||
		w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness ||
		w.Custody != privateCallerResultCustody || !validCallerProvenance(w.Provenance) ||
		!validDigest(w.ResultID) || !validDigest(w.PreparedSelector) || !validDigest(w.PreparedID) ||
		w.PreparedDigest != w.PreparedSelector || w.PreparedByteLength == 0 ||
		!validDigest(w.CheckpointSelector) || !validDigest(w.CheckpointID) || !validDigest(w.ChainIdentity) ||
		!validDigest(w.RequestsSelector) || w.RequestsDigest != w.RequestsSelector ||
		w.RequestsByteLength == 0 || w.RequestsCount < 0 || len(w.Results) != w.RequestsCount {
		return errors.New("private caller result integrity mismatch")
	}
	seenRecords := make(map[string]bool, len(w.Results))
	seenMessages := make(map[string]bool, len(w.Results))
	seenLineages := make(map[string]bool, len(w.Results))
	for i, row := range w.Results {
		if row.RequestOrdinal != i || !validUnprefixedIdentity(row.RecordID) ||
			!validUnprefixedIdentity(row.MessageID) || !validUnprefixedIdentity(row.LineageIdentity) ||
			seenRecords[row.RecordID] || seenMessages[row.MessageID] || seenLineages[row.LineageIdentity] {
			return errors.New("private caller result request binding mismatch")
		}
		seenRecords[row.RecordID], seenMessages[row.MessageID], seenLineages[row.LineageIdentity] = true, true, true
		if err := validateRetainedOutcome(row.Outcome); err != nil {
			return err
		}
	}
	want := identityJSON(privateCallerResultIdentityDomain, w, func(v *privateCallerResultWire) { v.ResultID = "" })
	if want != w.ResultID {
		return errors.New("private caller result identity mismatch")
	}
	return nil
}

var privateUnprefixedIdentity = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validUnprefixedIdentity(value string) bool { return privateUnprefixedIdentity.MatchString(value) }
func validCallerProvenance(value string) bool {
	return value == privateCallerResultObserved || value == privateCallerResultSynthetic
}

// RetainPrivateCallerResultSelector retains caller-controlled inference output as
// an immutable, authority-zero sidecar. It does not admit, qualify, or consume it.
func RetainPrivateCallerResultSelector(ctx context.Context, store Store, preparedSelector string, submissionRaw []byte) (string, error) {
	_, selector, err := retainPrivateCallerResult(ctx, store, preparedSelector, submissionRaw)
	return selector, err
}

func retainPrivateCallerResult(ctx context.Context, store Store, preparedSelector string, submissionRaw []byte) (privateCallerResult, string, error) {
	if err := ctx.Err(); err != nil {
		return privateCallerResult{}, "", err
	}
	if store == nil || !validDigest(preparedSelector) || len(submissionRaw) == 0 || len(submissionRaw) > privateCallerResultMaxBytes {
		return privateCallerResult{}, "", errors.New("private caller result input invalid")
	}

	var submission privateCallerResultSubmissionWire
	if err := strictCanonical(submissionRaw, &submission); err != nil {
		return privateCallerResult{}, "", err
	}
	if submission.SchemaVersion != privateCallerResultSubmissionSchema || !validCallerProvenance(submission.Provenance) || submission.Results == nil {
		return privateCallerResult{}, "", errors.New("private caller result submission invalid")
	}

	preparedRaw, err := store.Get(ctx, preparedSelector)
	if err != nil {
		return privateCallerResult{}, "", err
	}
	if digestOf(preparedRaw) != preparedSelector {
		return privateCallerResult{}, "", errors.New("private caller result prepared selector mismatch")
	}
	prepared, err := parsePrivateHandoffPrepared(preparedRaw)
	if err != nil {
		return privateCallerResult{}, "", err
	}

	chain, err := VerifyChain(ctx, store, prepared.wire.CheckpointSelector)
	if err != nil || len(chain) == 0 {
		return privateCallerResult{}, "", errors.New("private caller result checkpoint chain invalid")
	}
	checkpoint := chain[len(chain)-1]
	checkpointIDs := make([]string, len(chain))
	for i := range chain {
		checkpointIDs[i] = chain[i].ID()
	}
	chainIdentity := identityJSON("lsp-trace:census-handoff-prepared-chain:private:v1", checkpointIDs, func(*[]string) {})
	if checkpoint.Stage() != StageRequestsRendered || checkpoint.Status() != StatusRunning ||
		checkpoint.ID() != prepared.wire.CheckpointID || chainIdentity != prepared.wire.ChainIdentity {
		return privateCallerResult{}, "", errors.New("private caller result prepared checkpoint mismatch")
	}

	var requestsRef ArtifactRef
	for _, ref := range checkpoint.Artifacts() {
		if ref.Kind == "requests" {
			if requestsRef.Kind != "" {
				return privateCallerResult{}, "", errors.New("private caller result requests ambiguous")
			}
			requestsRef = ref
		}
	}
	if requestsRef.Kind == "" || requestsRef.ID != prepared.wire.RequestsSelector ||
		requestsRef.Digest != prepared.wire.RequestsDigest || requestsRef.ByteLength != prepared.wire.RequestsByteLength {
		return privateCallerResult{}, "", errors.New("private caller result requests binding mismatch")
	}
	requestsRaw, err := store.Get(ctx, requestsRef.ID)
	if err != nil {
		return privateCallerResult{}, "", err
	}
	if digestOf(requestsRaw) != requestsRef.Digest || uint64(len(requestsRaw)) != requestsRef.ByteLength {
		return privateCallerResult{}, "", errors.New("private caller result requests identity mismatch")
	}
	requests, err := describerequest.Parse(requestsRaw)
	if err != nil || len(requests) != prepared.wire.RequestsCount || len(submission.Results) != len(requests) {
		return privateCallerResult{}, "", errors.New("private caller result request count mismatch")
	}

	rows := make([]privateCallerResultRow, len(requests))
	for i, request := range requests {
		supplied := submission.Results[i]
		if supplied.RequestOrdinal != i || supplied.RecordID != request.RecordID ||
			supplied.MessageID != request.Envelope.MessageID || supplied.LineageIdentity != request.Lineage.LineageIdentity {
			return privateCallerResult{}, "", errors.New("private caller result request substitution")
		}
		outcome, err := retainOutcome(supplied.Outcome)
		if err != nil {
			return privateCallerResult{}, "", err
		}
		rows[i] = privateCallerResultRow{RequestOrdinal: i, RecordID: request.RecordID, MessageID: request.Envelope.MessageID, LineageIdentity: request.Lineage.LineageIdentity, Outcome: outcome}
	}

	wire := privateCallerResultWire{
		SchemaVersion: privateCallerResultSchema, Status: privateCallerResultStatus,
		Completeness: ContinuationCompleteness, Custody: privateCallerResultCustody, Provenance: submission.Provenance,
		PreparedSelector: preparedSelector, PreparedID: prepared.ID(), PreparedDigest: digestOf(preparedRaw), PreparedByteLength: uint64(len(preparedRaw)),
		CheckpointSelector: prepared.wire.CheckpointSelector, CheckpointID: prepared.wire.CheckpointID, ChainIdentity: prepared.wire.ChainIdentity,
		RequestsSelector: prepared.wire.RequestsSelector, RequestsDigest: prepared.wire.RequestsDigest,
		RequestsByteLength: prepared.wire.RequestsByteLength, RequestsCount: prepared.wire.RequestsCount, Results: rows,
	}
	wire.ResultID = identityJSON(privateCallerResultIdentityDomain, wire, func(v *privateCallerResultWire) { v.ResultID = "" })
	result := privateCallerResult{wire: wire}
	raw, err := result.Bytes()
	if err != nil {
		return privateCallerResult{}, "", err
	}
	parsed, err := parsePrivateCallerResult(raw)
	if err != nil || parsed.ID() != result.ID() {
		return privateCallerResult{}, "", errors.New("private caller result self validation failed")
	}
	if err := ctx.Err(); err != nil {
		return privateCallerResult{}, "", err
	}
	selector, err := store.Put(ctx, raw)
	if err != nil {
		return privateCallerResult{}, "", err
	}
	if selector != digestOf(raw) {
		return privateCallerResult{}, "", errors.New("private caller result store selector mismatch")
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return privateCallerResult{}, "", err
	}
	readbackResult, err := parsePrivateCallerResult(readback)
	if err != nil || !bytes.Equal(readback, raw) || readbackResult.ID() != result.ID() {
		return privateCallerResult{}, "", errors.New("private caller result readback mismatch")
	}
	return result, selector, nil
}

func retainOutcome(raw json.RawMessage) (json.RawMessage, error) {
	var kind privateCallerOutcomeKind
	if err := strictCanonical(raw, &kind); err != nil {
		// The discriminator-only decode intentionally rejects arm fields, so use a
		// minimal map to select the strict arm decoder.
		var members map[string]json.RawMessage
		if json.Unmarshal(raw, &members) != nil || len(members["kind"]) == 0 || json.Unmarshal(members["kind"], &kind.Kind) != nil {
			return nil, errors.New("private caller result outcome invalid")
		}
	}
	switch kind.Kind {
	case privateCallerOutcomeSuccess, privateCallerOutcomePartial:
		var arm privateCallerSemanticSubmission
		if err := strictCanonical(raw, &arm); err != nil || arm.Kind != kind.Kind || len(arm.Semantic) == 0 {
			return nil, errors.New("private caller semantic outcome invalid")
		}
		semantic, canonical, err := describeworker.ParseSemanticResponseV2(arm.Semantic)
		if err != nil || !bytes.Equal(arm.Semantic, canonical) {
			return nil, errors.New("private caller semantic outcome invalid")
		}
		if kind.Kind == privateCallerOutcomeSuccess && semantic.Verdict != "COMPLETE" {
			return nil, errors.New("private caller success verdict invalid")
		}
		if kind.Kind == privateCallerOutcomePartial && (semantic.Verdict != "ABSTAINED" || len(semantic.Limitations) == 0) {
			return nil, errors.New("private caller partial verdict invalid")
		}
		out := privateCallerSemanticOutcome{Kind: kind.Kind, RawSemanticSHA256: digestOf(arm.Semantic), RawSemanticByteLength: uint64(len(arm.Semantic)), CanonicalSemantic: append(json.RawMessage(nil), canonical...)}
		return json.Marshal(out)
	case privateCallerOutcomeFailed, privateCallerOutcomeRefused:
		var arm privateCallerFailureOutcome
		if err := strictCanonical(raw, &arm); err != nil || arm.Kind != kind.Kind || validateFailure(kind.Kind, arm.Failure) != nil {
			return nil, errors.New("private caller failure outcome invalid")
		}
		return json.Marshal(arm)
	case privateCallerOutcomeUnknown:
		var arm privateCallerUnknownOutcome
		if err := strictCanonical(raw, &arm); err != nil || arm.Kind != kind.Kind || validateAmbiguities(arm.Ambiguities) != nil {
			return nil, errors.New("private caller unknown outcome invalid")
		}
		return json.Marshal(arm)
	default:
		return nil, fmt.Errorf("private caller result outcome kind invalid: %q", kind.Kind)
	}
}

func validateRetainedOutcome(raw json.RawMessage) error {
	var members map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &members) != nil {
		return errors.New("private caller retained outcome invalid")
	}
	var kind string
	if json.Unmarshal(members["kind"], &kind) != nil {
		return errors.New("private caller retained outcome invalid")
	}
	switch kind {
	case privateCallerOutcomeSuccess, privateCallerOutcomePartial:
		var arm privateCallerSemanticOutcome
		if err := strictCanonical(raw, &arm); err != nil || arm.Kind != kind || !validDigest(arm.RawSemanticSHA256) || arm.RawSemanticByteLength == 0 {
			return errors.New("private caller retained semantic invalid")
		}
		semantic, canonical, err := describeworker.ParseSemanticResponseV2(arm.CanonicalSemantic)
		if err != nil || !bytes.Equal(canonical, arm.CanonicalSemantic) || arm.RawSemanticSHA256 != digestOf(canonical) || arm.RawSemanticByteLength != uint64(len(canonical)) {
			return errors.New("private caller retained semantic invalid")
		}
		if kind == privateCallerOutcomeSuccess && semantic.Verdict != "COMPLETE" || kind == privateCallerOutcomePartial && (semantic.Verdict != "ABSTAINED" || len(semantic.Limitations) == 0) {
			return errors.New("private caller retained semantic verdict invalid")
		}
	case privateCallerOutcomeFailed, privateCallerOutcomeRefused:
		var arm privateCallerFailureOutcome
		if err := strictCanonical(raw, &arm); err != nil || validateFailure(kind, arm.Failure) != nil {
			return errors.New("private caller retained failure invalid")
		}
	case privateCallerOutcomeUnknown:
		var arm privateCallerUnknownOutcome
		if err := strictCanonical(raw, &arm); err != nil || validateAmbiguities(arm.Ambiguities) != nil {
			return errors.New("private caller retained unknown invalid")
		}
	default:
		return errors.New("private caller retained outcome kind invalid")
	}
	return nil
}

func validateFailure(kind string, failure privateCallerFailure) error {
	if !boundedPrivateText(failure.Category) || !boundedPrivateText(failure.Code) || !boundedPrivateText(failure.Detail) {
		return errors.New("private caller failure bounds")
	}
	allowed := false
	if kind == privateCallerOutcomeFailed {
		switch failure.Category {
		case "CALLER_FAILURE", "INPUT_UNAVAILABLE", "RESOURCE_LIMIT", "CANCELLED":
			allowed = true
		}
	} else if kind == privateCallerOutcomeRefused && !failure.Retryable {
		switch failure.Category {
		case "POLICY_REFUSAL", "CUSTODY_REFUSAL", "INTEGRITY_REFUSAL":
			allowed = true
		}
	}
	if !allowed {
		return errors.New("private caller failure category")
	}
	return nil
}

func validateAmbiguities(ambiguities []privateCallerAmbiguity) error {
	if ambiguities == nil || len(ambiguities) == 0 || len(ambiguities) > privateCallerResultMaxAmbiguities {
		return errors.New("private caller ambiguity cardinality")
	}
	seenAmbiguities := map[string]bool{}
	for _, ambiguity := range ambiguities {
		if !boundedPrivateText(ambiguity.Code) || !boundedPrivateText(ambiguity.Subject) || ambiguity.Options == nil || len(ambiguity.Options) == 0 || len(ambiguity.Options) > privateCallerResultMaxOptions {
			return errors.New("private caller ambiguity invalid")
		}
		seenOptions := map[string]bool{}
		for _, option := range ambiguity.Options {
			if !boundedPrivateText(option) || seenOptions[option] {
				return errors.New("private caller ambiguity option invalid")
			}
			seenOptions[option] = true
		}
		keyBytes, _ := json.Marshal(ambiguity)
		key := string(keyBytes)
		if seenAmbiguities[key] {
			return errors.New("private caller ambiguity duplicate")
		}
		seenAmbiguities[key] = true
	}
	return nil
}

func boundedPrivateText(value string) bool {
	return value != "" && len([]byte(value)) <= privateCallerResultMaxTextBytes
}
