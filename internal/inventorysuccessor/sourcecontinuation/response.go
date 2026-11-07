package sourcecontinuation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/targetpacket"
)

// SuppliedSourceAttempt labels this binding only. It is not a producer nonce,
// invocation record, host attestation or claim of a started/reaped process.
type SuppliedSourceAttempt struct {
	ID              string
	CheckpointID    string
	SourceUnitID    string
	RequestRecordID string
}

type SuppliedSourceResponseRequest struct {
	Resume      ResumeRequest
	Record      describerequest.Record
	Attempt     SuppliedSourceAttempt
	AdapterLine []byte
	Limits      describeworker.SuppliedResponseLimits
}

// BoundSuppliedSourceResponse cannot be reconstructed from public JSON or a
// lower parse candidate. Eligibility exists only in the private state minted
// after exact source Resume and the corresponding opaque admission join.
type BoundSuppliedSourceResponse struct{ state *boundSourceResponseState }
type boundSourceResponseState struct {
	id        string
	canonical []byte
	candidate describeworker.SuppliedTerminalCandidate
	bounds    sourceResponseBound
}

type suppliedSourceResponseWire struct {
	Version                 string                 `json:"version"`
	BindingID               string                 `json:"binding_id"`
	Origin                  string                 `json:"origin"`
	Execution               string                 `json:"execution"`
	Authority               int                    `json:"authority"`
	Accepted                bool                   `json:"accepted"`
	Identity                string                 `json:"identity"`
	Completeness            string                 `json:"completeness"`
	Absence                 string                 `json:"absence"`
	HandoffID               string                 `json:"handoff_id"`
	CheckpointID            string                 `json:"checkpoint_id"`
	SourceUnitID            string                 `json:"source_unit_id"`
	SourceDigest            string                 `json:"source_digest"`
	Workspace               string                 `json:"workspace"`
	Request                 describerequest.Record `json:"request"`
	AttemptID               string                 `json:"attempt_id"`
	RawAdapterDigest        string                 `json:"raw_adapter_digest"`
	CanonicalSemanticDigest string                 `json:"canonical_semantic_digest"`
	Semantic                json.RawMessage        `json:"semantic"`
}

// BindSuppliedSourceResponse is nonexecuting. It preserves the original source
// deadline and caps, rejects all execution dependencies, then delegates source
// checkpoint/proof validation to the existing Resume (no alternate admission).
func BindSuppliedSourceResponse(ctx context.Context, q SuppliedSourceResponseRequest) (BoundSuppliedSourceResponse, error) {
	bad := func(s string) (BoundSuppliedSourceResponse, error) {
		return BoundSuppliedSourceResponse{}, errors.New(s)
	}
	r := q.Resume
	if ctx == nil || ctx.Err() != nil || r.PrivateSources == nil || r.Worker != nil || r.WorkerV2 != nil || r.FreshCapture != nil || !q.Limits.Valid() || len(q.AdapterLine) < 2 || len(q.AdapterLine) > q.Limits.MaxAdapterBytes || len(q.Attempt.ID) == 0 || len(q.Attempt.ID) > 256 || strings.TrimSpace(q.Attempt.ID) != q.Attempt.ID {
		return bad("supplied source response preflight")
	}
	s := r.PrivateSources.Expected.state
	if s == nil || s.stop != StopAfterPrivateSourceRequests || q.Attempt.CheckpointID != s.checkpoint || q.Limits.MaxAdapterBytes > s.contract.PacketMaxResponseBytes() || q.Limits.MaxSemanticBytes > s.contract.PacketMaxResponseBytes() {
		return bad("supplied source response expected/attempt/limits")
	}
	if err := preflight(ctx, s.request, s.contract, r.Store, r.Workspace, s.stop); err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	resumed := Resume(ctx, r)
	if resumed.Err != nil {
		return BoundSuppliedSourceResponse{}, resumed.Err
	}
	if resumed.PrivateSources == nil {
		return bad("supplied source response missing regenerated sources")
	}
	if ctx.Err() != nil || !time.Now().Before(s.request.Deadline) {
		return bad("supplied source response cancelled or deadline after Resume")
	}
	out := resumed.PrivateSources
	// Record consists entirely of comparable value fields. Compare every field
	// before marshaling any caller-controlled Record; a public reseal is not proof.
	found := false
	for _, record := range out.Requests {
		if record == q.Record && record.RecordID == q.Attempt.RequestRecordID && record.Envelope.AdmissionID == q.Attempt.SourceUnitID && record.Lineage.PacketID == q.Attempt.SourceUnitID {
			found = true
			break
		}
	}
	if !found {
		return bad("supplied source response exact request mismatch")
	}
	var sourceDigest string
	for _, a := range s.admissions {
		unit, err := a.Unit()
		if err != nil || unit.ID != q.Attempt.SourceUnitID {
			continue
		}
		for _, regenerated := range out.Units {
			if reflect.DeepEqual(unit, regenerated) {
				sourceDigest = unit.SourceDigest
				break
			}
		}
		if sourceDigest != "" {
			break
		}
	}
	if sourceDigest == "" {
		return bad("supplied source response opaque admission mismatch")
	}
	// No new candidate canonical copies, response digest, JSON serialization or
	// identity input is constructed before both existing aggregate ceilings fit.
	retained, ok := sourceResponseRetainedUpper(s, out)
	if !ok {
		return bad("supplied source response retained bound")
	}
	recordUpper, ok := sourceResponseRecordUpper(q.Record)
	if !ok {
		return bad("supplied source response record bound")
	}
	metadata, ok := sourceResponseStringBytes(out.HandoffID, out.CheckpointID, q.Attempt.SourceUnitID, sourceDigest, r.Workspace, q.Attempt.ID)
	if !ok {
		return bad("supplied source response metadata bound")
	}
	capture := s.contract.CaptureLimits()
	wireCap := s.contract.PacketMaxResponseBytes()
	if capture.MaxArtifactBytes < wireCap {
		wireCap = capture.MaxArtifactBytes
	}
	if wireCap <= 0 || capture.MaxParentBytes <= 0 {
		return bad("supplied source response existing ceilings")
	}
	bounds, err := sourceResponseAggregateBound(retained, recordUpper, metadata, uint64(len(q.AdapterLine)), uint64(q.Limits.MaxSemanticBytes), uint64(wireCap), uint64(capture.MaxParentBytes))
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	candidate, err := describeworker.ParseSuppliedTerminalResponse(q.AdapterLine, q.Limits)
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	if ctx.Err() != nil || !time.Now().Before(s.request.Deadline) {
		return bad("supplied source response cancelled or deadline")
	}
	wire := suppliedSourceResponseWire{
		Version: "lsp-trace.private-supplied-source-response.v1",
		Origin:  describeworker.SuppliedTerminalResponseOnly, Execution: describeworker.NoProcessOrHostExecutionVerified,
		Identity: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED",
		HandoffID: out.HandoffID, CheckpointID: out.CheckpointID, SourceUnitID: q.Attempt.SourceUnitID,
		SourceDigest: sourceDigest, Workspace: r.Workspace, Request: q.Record, AttemptID: q.Attempt.ID,
		RawAdapterDigest: candidate.RawAdapterDigest(), CanonicalSemanticDigest: candidate.CanonicalSemanticDigest(), Semantic: candidate.CanonicalSemantic(),
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	wire.BindingID = digest(append([]byte("lsp-trace.private-supplied-source-response.identity.v1\x00"), raw...))
	raw, err = json.Marshal(wire)
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	if !bounds.allows(uint64(len(raw))) {
		return bad("supplied source response serialized bound mismatch")
	}
	// Last finite body operation before constructing/publishing opaque state:
	// serialization, canonical copies and identity hashing are already complete.
	if !sourceResponsePublicationLive(ctx, s.request.Deadline) {
		return bad("supplied source response cancelled or expired at publication")
	}
	return BoundSuppliedSourceResponse{&boundSourceResponseState{wire.BindingID, raw, candidate, bounds}}, nil
}

func (b BoundSuppliedSourceResponse) ID() string {
	if b.state == nil {
		return ""
	}
	return b.state.id
}
func (b BoundSuppliedSourceResponse) Bytes() ([]byte, error) {
	if b.state == nil || !b.state.bounds.allows(uint64(len(b.state.canonical))) {
		return nil, errors.New("unbound or unbudgeted supplied source response")
	}
	return append([]byte(nil), b.state.canonical...), nil
}
func (b BoundSuppliedSourceResponse) Semantic() describeworker.SemanticResponseV2 {
	if b.state == nil || !b.state.bounds.allows(uint64(len(b.state.canonical))) {
		return describeworker.SemanticResponseV2{}
	}
	return b.state.candidate.Semantic()
}
func (b BoundSuppliedSourceResponse) MarshalJSON() ([]byte, error) { return b.Bytes() }

// This is the original context/deadline, checked only after final serialization.
// The finite check is logical publication liveness, not atomic OS-clock custody.
func sourceResponsePublicationLive(ctx context.Context, deadline time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !deadline.IsZero() && time.Now().Before(deadline)
}

// sourceResponseBound is a finite logical payload/copy allowance under the
// already admitted contract. It is NOT sizeof, a physical Go heap bound, a
// cumulative allowance for unlimited caller copies, or a fresh resource grant.
// Accessors retain these original ceilings and reserve one call's defensive
// copies (including a caller json.Marshal wrapper) in the same peak calculation.
type sourceResponseBound struct {
	retained, wire, semantic, peak, wireCap, parentCap uint64
}

func (b sourceResponseBound) allows(actual uint64) bool {
	return b.retained > 0 && b.wire > 0 && b.semantic > 0 && actual <= b.wire && b.wire <= b.wireCap && b.peak >= b.wire && b.peak <= b.parentCap
}
func sourceResponseAdd(values ...uint64) (uint64, bool) {
	var sum uint64
	for _, v := range values {
		if v > ^uint64(0)-sum {
			return 0, false
		}
		sum += v
	}
	return sum, true
}
func sourceResponseMul(a, b uint64) (uint64, bool) {
	if a != 0 && b > ^uint64(0)/a {
		return 0, false
	}
	return a * b, true
}
func sourceResponseStringBytes(values ...string) (uint64, bool) {
	var total uint64
	for _, v := range values {
		var ok bool
		total, ok = sourceResponseAdd(total, uint64(len(v)))
		if !ok {
			return 0, false
		}
	}
	return total, true
}

// Record is a fixed shape: enumerate ALL variable strings, including metadata,
// rather than charge only Prompt. 2048 covers the fixed keys, JSON punctuation,
// five signed integer fields (at most 20 bytes each) and boolean representation.
// Six bytes per input byte covers JSON control/HTML escaping and invalid UTF-8.
func sourceResponseRecordUpper(r describerequest.Record) (uint64, bool) {
	n, ok := sourceResponseStringBytes(r.RecordID,
		r.Envelope.Protocol, r.Envelope.MessageType, r.Envelope.MessageID, r.Envelope.CorrelationID,
		r.Envelope.AdmissionID, r.Envelope.Corpus, r.Envelope.ItemType, r.Envelope.AcquisitionMode,
		r.Envelope.InputSHA256, r.Envelope.SourceRevision, r.Envelope.TargetMessageID, r.Envelope.Prompt,
		r.Lineage.PacketID, r.Lineage.CensusID, r.Lineage.LineageIdentity, r.Lineage.ConsumerResolution,
		r.Lineage.AlternativeID, r.Lineage.Completeness, r.Lineage.SourceGraphComplete)
	if !ok {
		return 0, false
	}
	n, ok = sourceResponseMul(n, 6)
	if !ok {
		return 0, false
	}
	return sourceResponseAdd(n, 2048)
}

func sourceResponseUnitUpper(u targetpacket.PrivateSourceUnit) (uint64, bool) {
	n, ok := sourceResponseStringBytes(u.Version, u.ID, u.InputID, u.URI, u.SourceDigest, u.Revision, u.Privacy, u.Workspace, u.SessionID, u.DocumentVersion, u.PositionEncoding, u.SymbolID, u.Body, u.ClaimCeiling)
	if !ok {
		return 0, false
	}
	n, ok = sourceResponseMul(n, 6)
	if !ok {
		return 0, false
	}
	// Fixed keys, generation and both complete display/selection ranges.
	return sourceResponseAdd(n, 2048)
}

// Bound the state still live across Resume, not an assumed empty parent budget.
// The complete opaque handoff is charged even for READY inputs with no supplied
// source original; projection metadata/intents alone cannot bound those payloads.
// This intentionally overcounts sharing: original proofs*4 (preflight/prepare),
// the sourceBytes*12 + intents*16384 preparation term, every definition/source
// projection per intent (at least every distinct endpoint), every opaque
// admission's existing Bound(), and two complete regenerated output uppers.
// The source-only pause contains the same admitted handoff; these are the
// existing logical source-preparation terms, not a new whole-heap profile.
func sourceResponseRetainedUpper(s *expected, out *PrivateAllModeSourcesResult) (uint64, bool) {
	if s == nil || out == nil {
		return 0, false
	}
	handoff, ok := s.handoff.RetainedPayloadUpper()
	if !ok {
		return 0, false
	}
	total, ok := sourceResponseAdd(32768, handoff)
	if !ok {
		return 0, false
	}
	addProduct := func(a, b uint64) bool {
		n, ok := sourceResponseMul(a, b)
		if !ok {
			return false
		}
		total, ok = sourceResponseAdd(total, n)
		return ok
	}
	for _, o := range s.request.Originals {
		b := o.Bound()
		if b == 0 || !addProduct(b, 4) {
			return 0, false
		}
	}
	var source, definitions uint64
	for _, o := range s.request.Objects {
		var ok bool
		source, ok = sourceResponseAdd(source, uint64(len(o.Bytes)))
		if !ok {
			return 0, false
		}
	}
	for _, d := range s.request.Definitions {
		b, ok := d.EncodedBound()
		if !ok {
			return 0, false
		}
		definitions, ok = sourceResponseAdd(definitions, b)
		if !ok {
			return 0, false
		}
	}
	intents := uint64(len(out.Outcomes))
	if !addProduct(source, 12) || !addProduct(intents, 16384) || !addProduct(definitions, intents) {
		return 0, false
	}
	projection, ok := sourceResponseMul(source, 4)
	if !ok || !addProduct(projection, intents) {
		return 0, false
	}
	for _, a := range s.admissions {
		b := a.Bound()
		if b <= 0 || !addProduct(uint64(b), 1) {
			return 0, false
		}
	}
	output, ok := sourceResponseMul(intents, 2048)
	if !ok {
		return 0, false
	}
	output, ok = sourceResponseAdd(output, 16384)
	if !ok {
		return 0, false
	}
	for _, u := range out.Units {
		b, ok := sourceResponseMul(uint64(len(u.Body)), 6)
		if !ok {
			return 0, false
		}
		b, ok = sourceResponseAdd(b, 32768)
		if !ok {
			return 0, false
		}
		full, ok := sourceResponseUnitUpper(u)
		if !ok {
			return 0, false
		}
		if full > b {
			b = full
		}
		output, ok = sourceResponseAdd(output, b)
		if !ok {
			return 0, false
		}
	}
	for _, r := range out.Requests {
		b, ok := sourceResponseRecordUpper(r)
		if !ok {
			return 0, false
		}
		legacy, ok := sourceResponseMul(uint64(len(r.Envelope.Prompt)), 6)
		if !ok {
			return 0, false
		}
		legacy, ok = sourceResponseAdd(legacy, 32768)
		if !ok {
			return 0, false
		}
		if b < legacy {
			b = legacy
		}
		output, ok = sourceResponseAdd(output, b)
		if !ok {
			return 0, false
		}
	}
	if !addProduct(output, 2) {
		return 0, false
	}
	return total, true
}

// All arithmetic precedes candidate canonical copies/hash/marshal. The actual
// bounded adapter length further bounds decoded semantic text, so min(actual
// adapter bytes, declared semantic bytes) is sound without decoding the payload.
// 2048 semantic bytes covers fixed semantic keys/default encodings; 4096 wire
// bytes covers fixed keys/constants, three SHA256 strings and numeric/boolean
// fields. Every variable Record and response metadata string is counted at 6x.
// Peak payload allowance: retained source state + 10 wires (two serializations,
// two encoder buffers, namespace/hash inputs, defensive accessor/caller wrapper
// copies) + 8 raw adapter payloads (bounded parse/validation copies) + 6 canonical
// semantic payloads (parser/model/wire/accessor copies) + 32768 fixed scratch.
func sourceResponseAggregateBound(retained, record, metadata, raw, semanticLimit, wireCap, parentCap uint64) (sourceResponseBound, error) {
	bad := func(s string) (sourceResponseBound, error) { return sourceResponseBound{}, errors.New(s) }
	if retained == 0 || record == 0 || raw == 0 || semanticLimit == 0 || wireCap == 0 || parentCap == 0 {
		return bad("supplied source response invalid aggregate inputs")
	}
	if semanticLimit > raw {
		semanticLimit = raw
	}
	semantic, ok := sourceResponseMul(semanticLimit, 6)
	if !ok {
		return bad("supplied source response semantic overflow")
	}
	semantic, ok = sourceResponseAdd(semantic, 2048)
	if !ok {
		return bad("supplied source response semantic overflow")
	}
	metadata, ok = sourceResponseMul(metadata, 6)
	if !ok {
		return bad("supplied source response metadata overflow")
	}
	wire, ok := sourceResponseAdd(4096, record, metadata, semantic)
	if !ok {
		return bad("supplied source response wire overflow")
	}
	if wire > wireCap {
		return bad("supplied source response whole-wire ceiling")
	}
	wires, ok := sourceResponseMul(wire, 10)
	if !ok {
		return bad("supplied source response peak overflow")
	}
	raws, ok := sourceResponseMul(raw, 8)
	if !ok {
		return bad("supplied source response peak overflow")
	}
	semantics, ok := sourceResponseMul(semantic, 6)
	if !ok {
		return bad("supplied source response peak overflow")
	}
	peak, ok := sourceResponseAdd(retained, wires, raws, semantics, 32768)
	if !ok {
		return bad("supplied source response peak overflow")
	}
	if peak > parentCap {
		return bad("supplied source response retained-plus-peak ceiling")
	}
	return sourceResponseBound{retained, wire, semantic, peak, wireCap, parentCap}, nil
}
