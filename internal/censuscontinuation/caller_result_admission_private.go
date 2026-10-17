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
	privateCallerResultAdmissionSchema         = "lsp-trace.census-caller-result-mechanical-admission.private.v1"
	privateCallerResultAdmissionStatus         = "CALLER_RESULT_MECHANICALLY_ADMITTED"
	privateCallerResultAdmissionIdentityDomain = "lsp-trace:census-caller-result-mechanical-admission:private:v1"
	privateCallerResultAdmissionMaxBytes       = 64 << 20
)

type privateCallerResultAdmissionWire struct {
	SchemaVersion string `json:"schema_version"`
	AdmissionID   string `json:"admission_id"`
	Status        string `json:"status"`

	Authority                   int    `json:"authority"`
	Accepted                    bool   `json:"accepted"`
	Completeness                string `json:"completeness"`
	SemanticQualification       string `json:"semantic_qualification"`
	CandidateGenerationAllowed  bool   `json:"candidate_generation_allowed"`
	AutomaticProgressionAllowed bool   `json:"automatic_progression_allowed"`
	Custody                     string `json:"custody"`
	Provenance                  string `json:"provenance"`

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
	RequestsCount      int    `json:"requests_count"`

	Rows []privateCallerResultAdmissionRow `json:"rows"`
}

type privateCallerResultAdmissionRow struct {
	Ordinal         int    `json:"ordinal"`
	RecordID        string `json:"record_id"`
	MessageID       string `json:"message_id"`
	LineageIdentity string `json:"lineage_identity"`
}

type privateCallerResultAdmission struct {
	wire privateCallerResultAdmissionWire
}

func (a privateCallerResultAdmission) ID() string { return a.wire.AdmissionID }

func (a privateCallerResultAdmission) Bytes() ([]byte, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a.wire)
	if err != nil {
		return nil, err
	}
	if err := validatePrivateCallerResultAdmissionSize(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validatePrivateCallerResultAdmissionSize(raw []byte) error {
	if len(raw) > privateCallerResultAdmissionMaxBytes {
		return errors.New("private caller result admission byte limit")
	}
	return nil
}

func parsePrivateCallerResultAdmission(raw []byte) (privateCallerResultAdmission, error) {
	if err := validatePrivateCallerResultAdmissionSize(raw); err != nil {
		return privateCallerResultAdmission{}, err
	}
	var wire privateCallerResultAdmissionWire
	if err := strictCanonical(raw, &wire); err != nil {
		return privateCallerResultAdmission{}, err
	}
	admission := privateCallerResultAdmission{wire: wire}
	return admission, admission.validate()
}

func (a privateCallerResultAdmission) validate() error {
	w := a.wire
	if w.SchemaVersion != privateCallerResultAdmissionSchema || w.Status != privateCallerResultAdmissionStatus ||
		w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness ||
		w.SemanticQualification != "NOT_PERFORMED" || w.CandidateGenerationAllowed || w.AutomaticProgressionAllowed ||
		w.Custody != privateCallerResultCustody || !validCallerProvenance(w.Provenance) ||
		!validDigest(w.AdmissionID) || !validDigest(w.ResultSelector) || w.ResultDigest != w.ResultSelector || w.ResultByteLength == 0 || !validDigest(w.ResultID) ||
		!validDigest(w.PreparedSelector) || w.PreparedDigest != w.PreparedSelector || w.PreparedByteLength == 0 || !validDigest(w.PreparedID) ||
		!validDigest(w.CheckpointSelector) || w.CheckpointDigest != w.CheckpointSelector || w.CheckpointByteLength == 0 || !validDigest(w.CheckpointID) || !validDigest(w.ChainIdentity) ||
		!validDigest(w.RequestsSelector) || w.RequestsDigest != w.RequestsSelector || w.RequestsByteLength == 0 || w.RequestsCount < 0 ||
		w.Rows == nil || len(w.Rows) != w.RequestsCount {
		return errors.New("private caller result admission integrity mismatch")
	}
	for i, row := range w.Rows {
		if row.Ordinal != i || !validUnprefixedIdentity(row.RecordID) || !validUnprefixedIdentity(row.MessageID) || !validUnprefixedIdentity(row.LineageIdentity) {
			return errors.New("private caller result admission row mismatch")
		}
	}
	want := identityJSON(privateCallerResultAdmissionIdentityDomain, w, func(v *privateCallerResultAdmissionWire) { v.AdmissionID = "" })
	if want != w.AdmissionID {
		return errors.New("private caller result admission identity mismatch")
	}
	return nil
}

func privateCallerResultAdmissionChainIdentity(chain []Checkpoint) string {
	ids := make([]string, len(chain))
	for i := range chain {
		ids[i] = chain[i].ID()
	}
	return identityJSON("lsp-trace:census-handoff-prepared-chain:private:v1", ids, func(*[]string) {})
}

// AdmitPrivateCallerResultSelector mechanically rebinds an immutable retained
// caller result. Store immutability and content addressability are preconditions;
// the bounded rereads below detect only inconsistencies observed by this call.
func AdmitPrivateCallerResultSelector(ctx context.Context, store Store, resultSelector string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if store == nil || !validDigest(resultSelector) {
		return "", errors.New("private caller result admission input invalid")
	}

	resultRaw, err := store.Get(ctx, resultSelector)
	if err != nil {
		return "", err
	}
	if digestOf(resultRaw) != resultSelector {
		return "", errors.New("private caller result admission result selector mismatch")
	}
	result, err := parsePrivateCallerResult(resultRaw)
	if err != nil {
		return "", err
	}

	preparedRaw, err := store.Get(ctx, result.wire.PreparedSelector)
	if err != nil {
		return "", err
	}
	if digestOf(preparedRaw) != result.wire.PreparedSelector || uint64(len(preparedRaw)) != result.wire.PreparedByteLength {
		return "", errors.New("private caller result admission prepared bytes mismatch")
	}
	prepared, err := parsePrivateHandoffPrepared(preparedRaw)
	if err != nil {
		return "", err
	}
	if prepared.ID() != result.wire.PreparedID || result.wire.PreparedDigest != result.wire.PreparedSelector {
		return "", errors.New("private caller result admission prepared binding mismatch")
	}

	chain, err := VerifyChain(ctx, store, prepared.wire.CheckpointSelector)
	if err != nil || len(chain) == 0 {
		return "", errors.New("private caller result admission checkpoint chain invalid")
	}
	terminal := chain[len(chain)-1]
	if terminal.Stage() != StageRequestsRendered || terminal.Status() != StatusRunning ||
		terminal.ID() != prepared.wire.CheckpointID || terminal.ID() != result.wire.CheckpointID ||
		prepared.wire.CheckpointSelector != result.wire.CheckpointSelector || prepared.wire.ChainIdentity != result.wire.ChainIdentity ||
		privateCallerResultAdmissionChainIdentity(chain) != prepared.wire.ChainIdentity {
		return "", errors.New("private caller result admission terminal binding mismatch")
	}

	checkpointRaw, err := store.Get(ctx, prepared.wire.CheckpointSelector)
	if err != nil {
		return "", err
	}
	if digestOf(checkpointRaw) != prepared.wire.CheckpointSelector {
		return "", errors.New("private caller result admission checkpoint selector mismatch")
	}
	checkpoint, err := ParseCheckpoint(checkpointRaw)
	if err != nil || !reflect.DeepEqual(checkpoint.wire, terminal.wire) || checkpoint.ID() != terminal.ID() || checkpoint.PriorID() != terminal.PriorID() || checkpoint.Stage() != terminal.Stage() || checkpoint.Status() != terminal.Status() || !reflect.DeepEqual(checkpoint.Artifacts(), terminal.Artifacts()) {
		return "", errors.New("private caller result admission checkpoint terminal mismatch")
	}

	var requestsRef ArtifactRef
	for _, ref := range checkpoint.Artifacts() {
		if ref.Kind == "requests" {
			if requestsRef.Kind != "" {
				return "", errors.New("private caller result admission requests ambiguous")
			}
			requestsRef = ref
		}
	}
	if requestsRef.Kind == "" || requestsRef.ID != requestsRef.Digest || requestsRef.ID != prepared.wire.RequestsSelector || requestsRef.Digest != prepared.wire.RequestsDigest || requestsRef.ByteLength != prepared.wire.RequestsByteLength ||
		requestsRef.ID != result.wire.RequestsSelector || requestsRef.Digest != result.wire.RequestsDigest || requestsRef.ByteLength != result.wire.RequestsByteLength {
		return "", errors.New("private caller result admission requests binding mismatch")
	}
	requestsRaw, err := store.Get(ctx, requestsRef.ID)
	if err != nil {
		return "", err
	}
	if digestOf(requestsRaw) != requestsRef.Digest || uint64(len(requestsRaw)) != requestsRef.ByteLength {
		return "", errors.New("private caller result admission requests bytes mismatch")
	}
	requests, err := describerequest.Parse(requestsRaw)
	if err != nil || len(requests) != prepared.wire.RequestsCount || len(requests) != result.wire.RequestsCount || len(requests) != len(result.wire.Results) {
		return "", errors.New("private caller result admission requests count mismatch")
	}

	rows := make([]privateCallerResultAdmissionRow, len(requests))
	for i, request := range requests {
		retained := result.wire.Results[i]
		if retained.RequestOrdinal != i || retained.RecordID != request.RecordID || retained.MessageID != request.Envelope.MessageID || retained.LineageIdentity != request.Lineage.LineageIdentity {
			return "", errors.New("private caller result admission result row mismatch")
		}
		rows[i] = privateCallerResultAdmissionRow{Ordinal: i, RecordID: request.RecordID, MessageID: request.Envelope.MessageID, LineageIdentity: request.Lineage.LineageIdentity}
	}

	for _, observed := range []struct {
		selector string
		raw      []byte
	}{{resultSelector, resultRaw}, {result.wire.PreparedSelector, preparedRaw}, {result.wire.CheckpointSelector, checkpointRaw}, {result.wire.RequestsSelector, requestsRaw}} {
		again, getErr := store.Get(ctx, observed.selector)
		if getErr != nil {
			return "", getErr
		}
		if !bytes.Equal(again, observed.raw) {
			return "", errors.New("private caller result admission observed inconsistent predecessor")
		}
	}

	wire := privateCallerResultAdmissionWire{
		SchemaVersion: privateCallerResultAdmissionSchema, Status: privateCallerResultAdmissionStatus,
		Completeness: ContinuationCompleteness, SemanticQualification: "NOT_PERFORMED",
		Custody: result.wire.Custody, Provenance: result.wire.Provenance,
		ResultSelector: resultSelector, ResultDigest: digestOf(resultRaw), ResultByteLength: uint64(len(resultRaw)), ResultID: result.ID(),
		PreparedSelector: result.wire.PreparedSelector, PreparedDigest: digestOf(preparedRaw), PreparedByteLength: uint64(len(preparedRaw)), PreparedID: prepared.ID(),
		CheckpointSelector: result.wire.CheckpointSelector, CheckpointDigest: digestOf(checkpointRaw), CheckpointByteLength: uint64(len(checkpointRaw)), CheckpointID: checkpoint.ID(), ChainIdentity: result.wire.ChainIdentity,
		RequestsSelector: result.wire.RequestsSelector, RequestsDigest: digestOf(requestsRaw), RequestsByteLength: uint64(len(requestsRaw)), RequestsCount: len(requests), Rows: rows,
	}
	wire.AdmissionID = identityJSON(privateCallerResultAdmissionIdentityDomain, wire, func(v *privateCallerResultAdmissionWire) { v.AdmissionID = "" })
	admission := privateCallerResultAdmission{wire: wire}
	raw, err := admission.Bytes()
	if err != nil {
		return "", err
	}
	parsed, err := parsePrivateCallerResultAdmission(raw)
	if err != nil || parsed.ID() != admission.ID() {
		return "", errors.New("private caller result admission self validation failed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector, err := store.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	if selector != digestOf(raw) {
		return "", errors.New("private caller result admission store selector mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return "", err
	}
	readbackAdmission, err := parsePrivateCallerResultAdmission(readback)
	if err != nil || !bytes.Equal(readback, raw) || digestOf(readback) != selector || readbackAdmission.ID() != admission.ID() {
		return "", errors.New("private caller result admission readback mismatch")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return selector, nil
}
