package sourcecontinuation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	tp "lsp-trace/internal/targetpacket"
)

var ErrSelectedConsumerV3 = errors.New("selected consumer v3 refused")

// Shared caller cost lives below censuscontinuation to avoid a reverse import.
type CallerBudgetV3 = cat.CallerBudgetV3

func NewCallerBudgetV3(deadline time.Time, bytes uint64) (*CallerBudgetV3, error) {
	return cat.NewCallerBudgetV3(deadline, bytes)
}

type CallerDescribeBindingV3 struct {
	AttemptID, Interpreter string
	Pins                   dw.HostPinsV2 // caller-declared, unauthenticated; never execution attestation
	Deadline               time.Time     // fresh caller interpretation only, never a native extension
	Limits                 dw.SuppliedResponseLimits
	Budget                 *CallerBudgetV3
}
type SelectedDescribeV3 struct{ state *selectedDescribeStateV3 }
type selectedDescribeStateV3 struct {
	mu                  sync.Mutex
	used                bool
	selected            PrivateSelectedEndpointV2
	unit                tp.PrivateSourceUnit
	request             describerequest.Record
	handoff, checkpoint string
	binding             CallerDescribeBindingV3
	host                dw.ResponseHostV2
}
type BoundSelectedDescriptionV3 struct{ state *boundSelectedStateV3 }
type boundSelectedStateV3 struct {
	evidence SelectedEvidenceV3
	raw      []byte
	budget   *CallerBudgetV3
}

// SelectedEvidenceV3 is data-only. It cannot be passed as opaque admission.
type SelectedEvidenceV3 = cat.SelectedEvidenceV3

func liveSelectedV3(ctx context.Context, d time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !d.IsZero() && time.Now().Before(d)
}
func smallSelectedV3(s string) bool { return len(s) > 0 && len(s) <= 256 }

// CallerRetentionUpperV3 is charged length-only metadata, not admission, native
// credit, or source acquisition. It accounts existing state held across the tail.
func (r PrivateSelectedResultV2) CallerRetentionUpperV3(ctx context.Context, b *CallerBudgetV3) (uint64, error) {
	if r.state == nil || b.Reserve(ctx, 1<<20) != nil {
		return 0, ErrSelectedConsumerV3
	}
	s := r.state
	if len(s.sources.Outcomes) > 1000 || len(s.sources.Units) > 1000 || len(s.sources.Requests) > 1000 || s.sources.Expected.state == nil {
		return 0, ErrSelectedConsumerV3
	}
	expected := s.sources.Expected.state
	if len(expected.request.Originals) > 1000 || len(expected.request.Objects) > 1000 || len(expected.request.Definitions) > 1000 || len(expected.admissions) > 1000 {
		return 0, ErrSelectedConsumerV3
	}
	n, ok := sourceResponseRetainedUpper(expected, &s.sources)
	if !ok || n > 64<<20 {
		return 0, ErrSelectedConsumerV3
	}
	return n, nil
}

func PrepareSelectedDescribeV3(ctx context.Context, r PrivateSelectedResultV2, b CallerDescribeBindingV3) (SelectedDescribeV3, error) {
	bad := func() (SelectedDescribeV3, error) { return SelectedDescribeV3{}, ErrSelectedConsumerV3 }
	if !liveSelectedV3(ctx, b.Deadline) || time.Until(b.Deadline) > 30*time.Second || !smallSelectedV3(b.AttemptID) || !smallSelectedV3(b.Interpreter) || !b.Limits.Valid() || b.Budget.Reserve(ctx, 1<<20) != nil || r.state == nil {
		return bad()
	}
	s := r.state
	// Scalar lengths and collection counts precede iteration, snapshot and encoding.
	if len(s.sources.Outcomes) > 1000 || len(s.sources.Requests) > 1000 || len(s.sources.Units) > 1000 || len(s.metadata.Intents) > 1000 || len(s.unit.Body) > 8<<20 || len(s.request.Envelope.Prompt) > 8<<20 || !selectedSelectorBoundV2(s.selected) {
		return bad()
	}
	// Include the existing native state retained by this result, not just the
	// selected body. These established length-only bounds perform no Resume/Get,
	// source hash scan, or grant renewal. Counts are bounded above before sizing.
	if s.sources.Expected.state == nil || len(s.sources.Expected.state.request.Originals) > 1000 || len(s.sources.Expected.state.request.Objects) > 1000 || len(s.sources.Expected.state.request.Definitions) > 1000 || len(s.sources.Expected.state.admissions) > 1000 {
		return bad()
	}
	retained, ok := sourceResponseRetainedUpper(s.sources.Expected.state, &s.sources)
	if !ok {
		return bad()
	}
	unitUpper, ok := sourceResponseUnitUpper(s.unit)
	if !ok {
		return bad()
	}
	recordUpper, ok := sourceResponseRecordUpper(s.request)
	if !ok {
		return bad()
	}
	// Twenty identity/validation encodings, five prompt hashes, evidence encoding
	// and retained copies fit24 escaped record uppers (conservative overcharge).
	n, ok := sourceResponseAdd(retained, 2*unitUpper, 24*recordUpper, uint64(len(s.sources.Outcomes)+len(s.sources.Requests)+len(s.sources.Units)+len(s.metadata.Intents))*1024, 32768)
	if !ok || b.Budget.Reserve(ctx, n) != nil {
		return bad()
	}
	if !s.admission.Valid() || !s.admission.Matches(s.selected.Intent) {
		return bad()
	}
	u, e := s.admission.Unit()
	if e != nil || u != s.unit || u.InputID != "" || u.ID == "" || u.SourceDigest == "" {
		return bad()
	}
	row := s.selected.RowPosition
	if row < 0 || row >= len(s.metadata.Intents) || row >= len(s.sources.Outcomes) || s.metadata.Intents[row] != s.selected.Intent {
		return bad()
	}
	outcome := s.sources.Outcomes[row]
	if outcome.Intent != row || outcome.Status != "SOURCE_ADMITTED" || outcome.AdmissionID != u.ID || outcome.RequestID != s.request.RecordID || s.request.Envelope.AdmissionID != u.ID || s.request.Lineage.PacketID != u.ID || s.request.Lineage.ConsumerResolution != describerequest.PrivateSourceOnly {
		return bad()
	}
	requests, units := 0, 0
	for _, q := range s.sources.Requests {
		if q == s.request {
			requests++
		}
	}
	for _, v := range s.sources.Units {
		if v == u {
			units++
		}
	}
	if requests != 1 || units != 1 || s.sources.CheckpointID == "" || s.sources.HandoffID != s.metadata.HandoffID || describerequest.ValidateRecord(s.request) != nil {
		return bad()
	}
	// All retained payload fields are scalar strings/values; no mutable producer
	// slice escapes. The caller gets its own record, not producer collection headers.
	host := dw.ResponseHostV2{RequestRecordID: s.request.RecordID, MessageID: s.request.Envelope.MessageID, AttemptID: b.AttemptID,
		Consumer: dw.ConsumerIdentityV2{Resolution: dw.ConsumerUnresolvedV2, AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: b.Pins,
		Provenance: dw.HostProvenanceV2{PacketID: u.ID, RequestLineageIdentity: s.request.Lineage.LineageIdentity},
		Custody:    dw.HostCustodyV2{GraphDigest: s.selected.Intent.InputID, CaptureID: u.SessionID}}
	host.Pins.PromptSHA256 = "sha256:" + s.request.Envelope.InputSHA256
	if b.Pins.PromptSHA256 != host.Pins.PromptSHA256 {
		return bad()
	}
	for _, pin := range []string{host.Pins.WorkerSHA256, host.Pins.ModelSHA256, host.Pins.GrammarSHA256, host.Pins.PromptSHA256, host.Custody.GraphDigest} {
		if len(pin) != 71 || pin[:7] != "sha256:" {
			return bad()
		}
		for _, c := range pin[7:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return bad()
			}
		}
	}
	return SelectedDescribeV3{&selectedDescribeStateV3{selected: s.selected, unit: u, request: s.request, handoff: s.sources.HandoffID, checkpoint: s.sources.CheckpointID, binding: b, host: host}}, nil
}

// Request is a charged data-only snapshot. It cannot restore the attempt.
func (a SelectedDescribeV3) Request(ctx context.Context) (describerequest.Record, error) {
	if a.state == nil {
		return describerequest.Record{}, ErrSelectedConsumerV3
	}
	s := a.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used || !liveSelectedV3(ctx, s.binding.Deadline) || s.binding.Budget.Reserve(ctx, uint64(len(s.request.Envelope.Prompt))*6+4096) != nil {
		return describerequest.Record{}, ErrSelectedConsumerV3
	}
	return s.request, nil
}
func CompleteSelectedDescribeV3(ctx context.Context, a SelectedDescribeV3, adapter []byte) (BoundSelectedDescriptionV3, error) {
	bad := func() (BoundSelectedDescriptionV3, error) { return BoundSelectedDescriptionV3{}, ErrSelectedConsumerV3 }
	if a.state == nil {
		return bad()
	}
	s := a.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return bad()
	}
	s.used = true
	// Completion consumes even malformed/cancelled attempts. Parse and copies are
	// reserved before reading content. Adapter length rejection does not parse it.
	if !liveSelectedV3(ctx, s.binding.Deadline) || len(adapter) > s.binding.Limits.MaxAdapterBytes || s.binding.Budget.Reserve(ctx, uint64(s.binding.Limits.MaxAdapterBytes)*12+uint64(s.binding.Limits.MaxSemanticBytes)*24+32768) != nil {
		return bad()
	}
	candidate, e := dw.ParseSuppliedTerminalResponse(adapter, s.binding.Limits)
	if e != nil {
		return BoundSelectedDescriptionV3{}, e
	}
	semantic := candidate.Semantic()
	if semantic.Verdict != "COMPLETE" || semantic.ConsumerNeed.Status != dw.ConsumerUnresolvedV2 || semantic.ProvidedBehavior.ConsumerRelative {
		return bad()
	}
	record, e := dw.NewResponseRecordV2(candidate.CanonicalSemantic(), s.host)
	if e != nil {
		return BoundSelectedDescriptionV3{}, e
	}
	raw, e := record.Bytes()
	if e != nil {
		return BoundSelectedDescriptionV3{}, e
	}
	evidence := SelectedEvidenceV3{ID: record.ID(), HandoffID: s.handoff, CheckpointID: s.checkpoint, OriginalInputID: s.selected.Intent.InputID, Interpreter: s.binding.Interpreter, Origin: dw.SuppliedTerminalResponseOnly, Execution: dw.NoProcessOrHostExecutionVerified, Selected: cat.NativeSelectedRowV3{RowPosition: s.selected.RowPosition, SlotOrdinal: s.selected.Slot.Ordinal, Mode: s.selected.Slot.Mode, RunID: s.selected.Slot.RunID, SeedNamespace: s.selected.Slot.SeedNamespace, Seed: s.selected.Slot.Seed, Intent: s.selected.Intent}, Unit: s.unit, Request: s.request, Record: raw, RawAdapterDigest: candidate.RawAdapterDigest()}
	encoded, e := json.Marshal(evidence)
	if e != nil || len(encoded) > 8<<20 || !liveSelectedV3(ctx, s.binding.Deadline) {
		return bad()
	}
	return BoundSelectedDescriptionV3{&boundSelectedStateV3{evidence: evidence, raw: encoded, budget: s.binding.Budget}}, nil
}
func (b BoundSelectedDescriptionV3) Available() bool { return b.state != nil }
func (b BoundSelectedDescriptionV3) Evidence(ctx context.Context) (SelectedEvidenceV3, error) {
	if b.state == nil || b.state.budget.Reserve(ctx, uint64(len(b.state.raw))*2+4096) != nil {
		return SelectedEvidenceV3{}, ErrSelectedConsumerV3
	}
	e := b.state.evidence
	e.Record = append(json.RawMessage(nil), e.Record...)
	return e, nil
}
func (b BoundSelectedDescriptionV3) Bytes(ctx context.Context) ([]byte, error) {
	if b.state == nil || b.state.budget.Reserve(ctx, uint64(len(b.state.raw))+1) != nil {
		return nil, ErrSelectedConsumerV3
	}
	return append([]byte(nil), b.state.raw...), nil
}
