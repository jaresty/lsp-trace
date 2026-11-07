package sourcecontinuation

import (
	"context"
	"encoding/json"
	"errors"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/targetpacket"
	"strings"
	"testing"
	"time"
)

type responseCountingStore struct{ gets int }

func (s *responseCountingStore) Get(context.Context, string) ([]byte, error) {
	s.gets++
	return nil, errors.New("unexpected Get")
}
func (s *responseCountingStore) Put(context.Context, []byte) (string, error) {
	return "", errors.New("unexpected Put")
}
func (s *responseCountingStore) MaxObjectBytes() int64 { return 8 << 20 }

type responseWorker struct{ calls int }

func (w *responseWorker) Run(context.Context, describerequest.Record, string) (describeworker.RunResult, error) {
	w.calls++
	return describeworker.RunResult{}, errors.New("prohibited")
}

type responseWorkerV2 struct{ calls int }

func (w *responseWorkerV2) RunV2(context.Context, describerequest.Record, targetpacket.Packet, string) (describeworker.RunResultV2, error) {
	w.calls++
	return describeworker.RunResultV2{}, errors.New("prohibited")
}

func TestSuppliedSourceResponsePreflight(t *testing.T) {
	store := &responseCountingStore{}
	q := SuppliedSourceResponseRequest{Resume: ResumeRequest{Store: store, PrivateSources: &PrivateAllModeSourcesResume{}}, Attempt: SuppliedSourceAttempt{ID: "controlled"}, AdapterLine: []byte("{}\n"), Limits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 * 1024, MaxSemanticBytes: 64 * 1024}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, done := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer done()
	worker := &responseWorker{}
	worker2 := &responseWorkerV2{}
	cases := []struct {
		name   string
		ctx    context.Context
		mutate func(*SuppliedSourceResponseRequest)
	}{
		{"zero-expected", context.Background(), func(*SuppliedSourceResponseRequest) {}},
		{"cancelled", cancelled, func(*SuppliedSourceResponseRequest) {}},
		{"expired-context", expired, func(*SuppliedSourceResponseRequest) {}},
		{"nil-context", nil, func(*SuppliedSourceResponseRequest) {}},
		{"oversized", context.Background(), func(q *SuppliedSourceResponseRequest) { q.AdapterLine = []byte(strings.Repeat("x", 512*1024+1)) }},
		{"attempt-bound", context.Background(), func(q *SuppliedSourceResponseRequest) { q.Attempt.ID = strings.Repeat("x", 257) }},
		{"worker", context.Background(), func(q *SuppliedSourceResponseRequest) { q.Resume.Worker = worker }},
		{"worker-v2", context.Background(), func(q *SuppliedSourceResponseRequest) { q.Resume.WorkerV2 = worker2 }},
		{"capture", context.Background(), func(q *SuppliedSourceResponseRequest) { q.Resume.FreshCapture = &cc.FreshCaptureDependencies{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := q
			tc.mutate(&v)
			b, e := BindSuppliedSourceResponse(tc.ctx, v)
			if e == nil || b.ID() != "" || store.gets != 0 || worker.calls != 0 || worker2.calls != 0 {
				t.Fatal("ASSERT_BIND_PREFLIGHT_NO_IO", tc.name, e)
			}
			t.Log("ASSERT_BIND_PREFLIGHT_NO_IO: PASS")
		})
	}
	var b BoundSuppliedSourceResponse
	if b.ID() != "" {
		t.Fatal("ASSERT_BIND_OPAQUE_ZERO")
	}
	if _, e := b.Bytes(); e == nil {
		t.Fatal("ASSERT_BIND_OPAQUE_ZERO")
	}
	if json.Unmarshal([]byte(`{"binding_id":"public","state":{}}`), &b) != nil || b.ID() != "" {
		t.Fatal("ASSERT_BIND_PUBLIC_NONADMITTING")
	}
	// A valid lower candidate cannot be passed to this entry: its signature only
	// accepts bytes plus the exact private Resume context, never a candidate/DTO.
	t.Log("ASSERT_BIND_OPAQUE_ZERO: PASS; ASSERT_BIND_PUBLIC_NONADMITTING: PASS")
}

func TestSuppliedResponseAggregateBoundaries(t *testing.T) {
	const max = ^uint64(0)
	b, e := sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, max, max)
	if e != nil {
		t.Fatal("ASSERT_SUPPLIED_BOUND_SETUP", e)
	}
	if b.wire <= 8192 || b.wire <= b.semantic {
		t.Fatal("ASSERT_SUPPLIED_COMPONENTS_INDIVIDUALLY_FIT")
	}
	just, e := sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, b.wire, b.peak)
	if e != nil || just != bWithCaps(b, b.wire, b.peak) || !just.allows(just.wire) {
		t.Fatal("ASSERT_SUPPLIED_BOUND_JUST_FIT", e)
	}
	if _, e = sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, b.wire-1, b.peak); e == nil || !strings.Contains(e.Error(), "whole-wire ceiling") {
		t.Fatal("ASSERT_SUPPLIED_WHOLE_WIRE_CAP", e)
	}
	if _, e = sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, b.wire, b.peak-1); e == nil || !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
		t.Fatal("ASSERT_SUPPLIED_PEAK_CAP", e)
	}
	if _, e = sourceResponseAggregateBound(b.peak, 8192, 128, 1024, 1024, b.wire, b.peak); e == nil {
		t.Fatal("ASSERT_SUPPLIED_RETAINED_NOT_FREE")
	}
	t.Log("ASSERT_SUPPLIED_BOUND_JUST_FIT: PASS; ASSERT_SUPPLIED_WHOLE_WIRE_CAP: PASS; ASSERT_SUPPLIED_PEAK_CAP: PASS; ASSERT_SUPPLIED_RETAINED_NOT_FREE: PASS")
	for name, values := range map[string][5]uint64{
		"metadata": {1, 1, max, 1, 1}, "wire-sum": {1, max, 1, 1, 1}, "raw-product": {1, 1, 1, max, 1}, "semantic-product": {1, 1, 1, max, max}, "wire-product": {1, max / 2, 1, 1, 1}, "peak-sum": {max, 1, 1, 1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := sourceResponseAggregateBound(values[0], values[1], values[2], values[3], values[4], max, max); e == nil {
				t.Fatal("ASSERT_SUPPLIED_CHECKED_OVERFLOW", name)
			}
			t.Log("ASSERT_SUPPLIED_CHECKED_OVERFLOW: PASS")
		})
	}
	// Escaping must affect the bound even when the raw metadata length fits.
	if _, e = sourceResponseAggregateBound(4096, 8192, 4096, 1024, 1024, b.wire, b.peak); e == nil {
		t.Fatal("ASSERT_SUPPLIED_ESCAPED_METADATA_BOUND")
	}
	if _, e = sourceResponseAggregateBound(4096, 8192, 128, 8192, 8192, b.wire, b.peak); e == nil {
		t.Fatal("ASSERT_SUPPLIED_SEMANTIC_EXPANSION_BOUND")
	}
	t.Log("ASSERT_SUPPLIED_ESCAPED_METADATA_BOUND: PASS; ASSERT_SUPPLIED_SEMANTIC_EXPANSION_BOUND: PASS")
}
func bWithCaps(b sourceResponseBound, w, p uint64) sourceResponseBound {
	b.wireCap = w
	b.parentCap = p
	return b
}

func TestSuppliedResponseMetadataAndAccessorBounds(t *testing.T) {
	s := strings.Repeat("<&\x00", 4096)
	r := describerequest.Record{RecordID: s, Envelope: describerequest.Envelope{Protocol: s, MessageType: s, MessageID: s, CorrelationID: s, AdmissionID: s, Corpus: s, ItemType: s, AcquisitionMode: s, InputSHA256: s, SourceRevision: s, TargetMessageID: s, Prompt: s}, Lineage: describerequest.Lineage{PacketID: s, CensusID: s, LineageIdentity: s, ConsumerResolution: s, AlternativeID: s, Completeness: s, SourceGraphComplete: s}}
	n, ok := sourceResponseRecordUpper(r)
	raw, e := json.Marshal(r)
	if !ok || e != nil || uint64(len(raw)) > n || n < 6*uint64(len(s))*20 {
		t.Fatal("ASSERT_SUPPLIED_ALL_RECORD_METADATA", len(raw), n, e)
	}
	recordUpper := n
	metadata, ok := sourceResponseStringBytes(s, s, s, s, s, s)
	if !ok {
		t.Fatal("ASSERT_SUPPLIED_WHOLE_WIRE_ESCAPING")
	}
	wireBound, e := sourceResponseAggregateBound(4096, recordUpper, metadata, 4096, 4096, ^uint64(0), ^uint64(0))
	if e != nil {
		t.Fatal(e)
	}
	wire := suppliedSourceResponseWire{Version: "lsp-trace.private-supplied-source-response.v1", BindingID: "sha256:" + strings.Repeat("1", 64), Origin: describeworker.SuppliedTerminalResponseOnly, Execution: describeworker.NoProcessOrHostExecutionVerified, Identity: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", HandoffID: s, CheckpointID: s, SourceUnitID: s, SourceDigest: s, Workspace: s, Request: r, AttemptID: s, RawAdapterDigest: "sha256:" + strings.Repeat("2", 64), CanonicalSemanticDigest: "sha256:" + strings.Repeat("3", 64), Semantic: json.RawMessage(`{}`)}
	encoded, e := json.Marshal(wire)
	if e != nil || uint64(len(encoded)) > wireBound.wire {
		t.Fatal("ASSERT_SUPPLIED_WHOLE_WIRE_ESCAPING", len(encoded), wireBound.wire, e)
	}
	t.Log("ASSERT_SUPPLIED_WHOLE_WIRE_ESCAPING: PASS")
	u := targetpacket.PrivateSourceUnit{Version: s, ID: s, InputID: s, URI: s, SourceDigest: s, Revision: s, Privacy: s, Workspace: s, SessionID: s, DocumentVersion: s, PositionEncoding: s, SymbolID: s, Body: s, ClaimCeiling: s}
	n, ok = sourceResponseUnitUpper(u)
	raw, e = json.Marshal(u)
	if !ok || e != nil || uint64(len(raw)) > n {
		t.Fatal("ASSERT_SUPPLIED_ALL_UNIT_METADATA", len(raw), n, e)
	}
	semantic := `{"verdict":"ABSTAINED","target_role":"x","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"x","consumer_relative":false},"boundary_contribution":"x","limitations":[]}`
	candidate, e := describeworker.ParseSuppliedTerminalResponse(describeworker.EncodeAdapterResultV2(semantic), describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4096, MaxSemanticBytes: 4096})
	if e != nil {
		t.Fatal(e)
	}
	budget, e := sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, 1<<20, 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	// Package-private corruption guard only; this is not a public admission mint.
	b := BoundSuppliedSourceResponse{&boundSourceResponseState{id: "private-test", canonical: []byte(`{}`), candidate: candidate, bounds: budget}}
	if _, e = b.Bytes(); e != nil || b.Semantic().Verdict != "ABSTAINED" {
		t.Fatal("ASSERT_SUPPLIED_ORIGINAL_ACCESSOR_BOUND", e)
	}
	b.state.bounds.parentCap = budget.peak - 1
	if _, e = b.Bytes(); e == nil {
		t.Fatal("ASSERT_SUPPLIED_ACCESSOR_CAP_REFUSAL")
	}
	if _, e = b.MarshalJSON(); e == nil || b.Semantic().Verdict != "" {
		t.Fatal("ASSERT_SUPPLIED_ACCESSOR_CAP_REFUSAL")
	}
	t.Log("ASSERT_SUPPLIED_ALL_RECORD_METADATA: PASS; ASSERT_SUPPLIED_ALL_UNIT_METADATA: PASS; ASSERT_SUPPLIED_ORIGINAL_ACCESSOR_BOUND: PASS; ASSERT_SUPPLIED_ACCESSOR_CAP_REFUSAL: PASS")
}

func TestSuppliedHandoffRetainedBudgetBoundary(t *testing.T) {
	if _, ok := (cc.PrivateAllModeHandoff{}).RetainedPayloadUpper(); ok {
		t.Fatal("ASSERT_SUPPLIED_ZERO_HANDOFF_SIZE_REFUSED")
	}
	var slots []cc.PrivateAllModeSlot
	var entries []cc.PrivateAllModeEntry
	for i, mode := range []string{"RD", "C", "CRD", "D", "CD", "R", "CR"} {
		s := cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: strings.Repeat("r", i+1), SeedNamespace: "controlled-scalar", Seed: 7}
		slots = append(slots, s)
		entries = append(entries, cc.PrivateAllModeEntry{Slot: s, Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: strings.Repeat("l", i+1), MissingReason: "controlled missing input"})
	}
	h, e := cc.NewPrivateAllModeHandoff("controlled-scalar-workspace", "controlled-revision", slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	n, ok := h.RetainedPayloadUpper()
	if !ok || n == 0 {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_SCALAR_POSITIVE")
	}
	before := h.ID()
	again, ok := h.RetainedPayloadUpper()
	if !ok || again != n || h.ID() != before {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_SCALAR_STABLE")
	}
	// Sizing-only input, never source admission: the real READY integration
	// witness is in TestPrivateSuppliedSourceResponseConnectedRetainedHandoff.
	retained, ok := sourceResponseRetainedUpper(&expected{handoff: h}, &PrivateAllModeSourcesResult{})
	if !ok || retained != 65536+n {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_CHARGED", retained, n)
	}
	if _, ok := sourceResponseRetainedUpper(&expected{}, &PrivateAllModeSourcesResult{}); ok {
		t.Fatal("ASSERT_SUPPLIED_ZERO_HANDOFF_SIZE_REFUSED")
	}
	b, e := sourceResponseAggregateBound(retained, 8192, 128, 1024, 1024, ^uint64(0), ^uint64(0))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sourceResponseAggregateBound(retained, 8192, 128, 1024, 1024, b.wire, b.peak); e != nil {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_JUST_FIT", e)
	}
	if _, e = sourceResponseAggregateBound(retained+1, 8192, 128, 1024, 1024, b.wire, b.peak); e == nil || !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_ONE_BYTE_OVER", e)
	}
	if _, ok := sourceResponseAdd(n, ^uint64(0)); ok {
		t.Fatal("ASSERT_SUPPLIED_HANDOFF_CHECKED_ADD")
	}
	t.Log("ASSERT_SUPPLIED_ZERO_HANDOFF_SIZE_REFUSED: PASS; ASSERT_SUPPLIED_HANDOFF_SCALAR_POSITIVE: PASS; ASSERT_SUPPLIED_HANDOFF_SCALAR_STABLE: PASS; ASSERT_SUPPLIED_HANDOFF_CHARGED: PASS; ASSERT_SUPPLIED_HANDOFF_JUST_FIT: PASS; ASSERT_SUPPLIED_HANDOFF_ONE_BYTE_OVER: PASS; ASSERT_SUPPLIED_HANDOFF_CHECKED_ADD: PASS")
}

func TestSuppliedFinalPublicationDeadline(t *testing.T) {
	originalDeadline := time.Unix(1, 0)
	if sourceResponsePublicationLive(context.Background(), originalDeadline) {
		t.Fatal("ASSERT_SUPPLIED_EXPIRED_PUBLICATION")
	}
	originalDeadline = time.Now().Add(time.Second)
	if !sourceResponsePublicationLive(context.Background(), originalDeadline) {
		t.Fatal("ASSERT_SUPPLIED_LIVE_PUBLICATION")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if sourceResponsePublicationLive(cancelled, originalDeadline) {
		t.Fatal("ASSERT_SUPPLIED_CANCELLED_PUBLICATION")
	}
	t.Log("ASSERT_SUPPLIED_EXPIRED_PUBLICATION: PASS; ASSERT_SUPPLIED_LIVE_PUBLICATION: PASS; ASSERT_SUPPLIED_CANCELLED_PUBLICATION: PASS")
}
