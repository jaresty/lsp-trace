package sessionruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unsafe"

	"lsp-trace/internal/lspwire"
)

// A deterministic partial CLI projection, never the complete retained archive.
// Captured-error references take priority, then the latest prepare records, then
// latest other records. Original client ordinals survive the projection.
type PrivateDiscoveryResponseSummary struct {
	Available          bool                              `json:"available"`
	Total              uint16                            `json:"retained_total"`
	Returned           uint8                             `json:"returned"`
	Omitted            uint16                            `json:"omitted_retained"`
	RetentionTruncated bool                              `json:"retention_truncated"`
	Truncated          bool                              `json:"truncated"`
	Context            PrivateDiscoverySessionContext    `json:"context"`
	Records            [2]PrivateDiscoveryResponseRecord `json:"-"`
}

func PrivateDiscoverySummarize(d PrivateDiscoveryResponseFacts) (PrivateDiscoveryResponseSummary, error) {
	if !d.Valid() {
		return PrivateDiscoveryResponseSummary{}, errors.New("invalid private discovery summary")
	}
	s := PrivateDiscoveryResponseSummary{Available: d.Available, Total: d.Count, Context: d.Context}
	add := func(r PrivateDiscoveryResponseRecord) {
		if s.Returned == 2 {
			return
		}
		for i := 0; i < int(s.Returned); i++ {
			if s.Records[i].Ordinal == r.Ordinal {
				return
			}
		}
		s.Records[s.Returned] = r
		s.Returned++
	}
	for i := 0; i < int(d.Count); i++ {
		if d.Records[i].Capture.Artifact != 0 {
			add(d.Records[i])
		}
	}
	for i := int(d.Count) - 1; i >= 0; i-- {
		if d.Records[i].Correlation.Method == PrivateMethodPrepareCallHierarchy {
			add(d.Records[i])
		}
	}
	for i := int(d.Count) - 1; i >= 0; i-- {
		add(d.Records[i])
	}
	s.Omitted = s.Total - uint16(s.Returned)
	s.RetentionTruncated = d.Truncated
	s.Truncated = s.RetentionTruncated || s.Omitted != 0
	return s, nil
}
func (s PrivateDiscoveryResponseSummary) Valid() bool {
	if s.Returned > 2 || s.Total > 128 || uint16(s.Returned) > s.Total || s.Omitted != s.Total-uint16(s.Returned) || s.Truncated != (s.RetentionTruncated || s.Omitted != 0) || !s.Available && (s.Total != 0 || s.Returned != 0 || s.Truncated) {
		return false
	}
	d := PrivateDiscoveryResponseFacts{Available: s.Available, Context: s.Context, Count: uint16(s.Returned)}
	for i, r := range s.Records {
		if i >= int(s.Returned) {
			if r != (PrivateDiscoveryResponseRecord{}) {
				return false
			}
			continue
		}
		if r.Ordinal == 0 || r.Ordinal > s.Total || i == 1 && r.Ordinal == s.Records[0].Ordinal {
			return false
		}
		r.Ordinal = uint16(i + 1)
		d.Records[i] = r
	}
	return d.Valid()
}
func (s *PrivateDiscoveryResponseSummary) UnmarshalJSON(raw []byte) error {
	fail := func() error { return errors.New("invalid private discovery summary") }
	if len(raw) > 47000 {
		return fail()
	}
	var wire struct {
		Available          bool                           `json:"available"`
		Total              uint16                         `json:"retained_total"`
		Returned           uint8                          `json:"returned"`
		Omitted            uint16                         `json:"omitted_retained"`
		RetentionTruncated bool                           `json:"retention_truncated"`
		Truncated          bool                           `json:"truncated"`
		Context            PrivateDiscoverySessionContext `json:"context"`
		Records            json.RawMessage                `json:"records"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&wire) != nil || d.Decode(&struct{}{}) != io.EOF || wire.Returned > 2 {
		return fail()
	}
	value := PrivateDiscoveryResponseSummary{Available: wire.Available, Total: wire.Total, Returned: wire.Returned, Omitted: wire.Omitted, RetentionTruncated: wire.RetentionTruncated, Truncated: wire.Truncated, Context: wire.Context}
	records := json.NewDecoder(bytes.NewReader(wire.Records))
	records.DisallowUnknownFields()
	tok, err := records.Token()
	if err != nil || tok != json.Delim('[') {
		return fail()
	}
	count := 0
	for records.More() {
		if count == 2 {
			return fail()
		}
		if records.Decode(&value.Records[count]) != nil {
			return fail()
		}
		count++
	}
	tok, err = records.Token()
	if err != nil || tok != json.Delim(']') || records.Decode(&struct{}{}) != io.EOF || count != int(wire.Returned) || !value.Valid() {
		return fail()
	}
	*s = value
	return nil
}

func (s PrivateDiscoveryResponseSummary) MarshalJSON() ([]byte, error) {
	if !s.Valid() {
		return nil, errors.New("invalid private discovery summary")
	}
	raw, err := json.Marshal(struct {
		Available          bool                             `json:"available"`
		Total              uint16                           `json:"retained_total"`
		Returned           uint8                            `json:"returned"`
		Omitted            uint16                           `json:"omitted_retained"`
		RetentionTruncated bool                             `json:"retention_truncated"`
		Truncated          bool                             `json:"truncated"`
		Context            PrivateDiscoverySessionContext   `json:"context"`
		Records            []PrivateDiscoveryResponseRecord `json:"records"`
	}{s.Available, s.Total, s.Returned, s.Omitted, s.RetentionTruncated, s.Truncated, s.Context, s.Records[:s.Returned]})
	if err != nil || len(raw) > 47000 {
		return nil, errors.New("private discovery summary bound")
	}
	return raw, nil
}

// These are observations, not provider-failure receipts or authority. Selected
// is set only after pending.Accept. LastFrame may be a decode failure, an
// unrelated frame, or a frame joined during cancellation; ID equality alone
// never promotes it to a selected response. Ordinals are local slot numbers.
// LastFrame deliberately retains only the latest read attempt, not an archive.
// ReadAttempts includes incomplete acquisition; NotObserved facts stay unknown.
const PrivateDiscoveryResponseCapacity = 128

type PrivateDiscoveryResponseDisposition uint8

const (
	PrivateResponseNotObserved PrivateDiscoveryResponseDisposition = iota
	PrivateResponseBoundary
	PrivateResponseDecodeRejected
	PrivateResponseUnmatched
	PrivateResponseLate
	PrivateResponseSelected
	PrivateResponseJoined
)

type PrivateDiscoveryResponseRecord struct {
	Correlation     PrivateDiscoveryCorrelation         `json:"correlation"`
	Sent            bool                                `json:"sent"`
	SentAt          PrivateDiscoveryEventTime           `json:"sent_at"`
	LastRead        PrivateDiscoveryReadStages          `json:"last_read"`
	SelectedRead    PrivateDiscoveryReadStages          `json:"selected_read"`
	Classification  PrivateDiscoveryClassification      `json:"classification"`
	Ordinal         uint16                              `json:"ordinal"`
	ReadAttempts    uint16                              `json:"read_attempts"`
	LastDisposition PrivateDiscoveryResponseDisposition `json:"last_disposition"`
	LastFrame       lspwire.ResponseFacts               `json:"last_frame"`
	Selected        lspwire.ResponseFacts               `json:"selected"`
	Capture         PrivateCaptureRecord                `json:"capture"`
}
type PrivateDiscoveryResponseFacts struct {
	Context   PrivateDiscoverySessionContext                                   `json:"context"`
	Available bool                                                             `json:"available"`
	Truncated bool                                                             `json:"truncated"`
	Count     uint16                                                           `json:"count"`
	Records   [PrivateDiscoveryResponseCapacity]PrivateDiscoveryResponseRecord `json:"-"`
}

func (d PrivateDiscoveryResponseFacts) Valid() bool {
	if !d.Context.valid() || !d.Available && d.Context != (PrivateDiscoverySessionContext{}) || int(d.Count) > len(d.Records) {
		return false
	}
	if !d.Available && (d.Truncated || d.Count != 0) {
		return false
	}
	var captureIDs [2]bool
	for i, r := range d.Records {
		if i >= int(d.Count) {
			if r != (PrivateDiscoveryResponseRecord{}) {
				return false
			}
			continue
		}
		if !r.Capture.Valid() || !r.Correlation.valid() || !r.SentAt.valid() || !r.Sent && r.SentAt != (PrivateDiscoveryEventTime{}) || !r.LastRead.valid() || !r.SelectedRead.valid() || !r.Classification.valid() {
			return false
		}
		if r.Capture.Artifact != 0 {
			if r.Selected.Association != lspwire.ResponseIDMatched || r.Selected.Error != lspwire.ResponseObject || captureIDs[r.Capture.Artifact-1] {
				return false
			}
			captureIDs[r.Capture.Artifact-1] = true
		}
		if r.Selected.State == lspwire.ResponseNotObserved && r.SelectedRead != (PrivateDiscoveryReadStages{}) {
			return false
		}
		if r.Ordinal != uint16(i+1) || r.LastDisposition > PrivateResponseJoined || !r.LastFrame.Valid() || !r.Selected.Valid() {
			return false
		}
		if r.ReadAttempts > 4096 || (r.ReadAttempts == 0) != (r.LastDisposition == PrivateResponseNotObserved) {
			return false
		}
		if r.ReadAttempts == 0 && (r.LastFrame != (lspwire.ResponseFacts{}) || r.Selected != (lspwire.ResponseFacts{}) || r.LastRead != (PrivateDiscoveryReadStages{})) {
			return false
		}
		if r.LastDisposition == PrivateResponseSelected && (r.Selected != r.LastFrame || r.SelectedRead != r.LastRead) {
			return false
		}
		if r.Selected.State != lspwire.ResponseNotObserved && (r.Selected.State != lspwire.ResponseValidJSON || r.Selected.Association != lspwire.ResponseIDMatched) {
			return false
		}
	}
	return true
}
func (d PrivateDiscoveryResponseFacts) MarshalJSON() ([]byte, error) {
	if !d.Valid() {
		return nil, errors.New("invalid private response facts")
	}
	return json.Marshal(struct {
		Context   PrivateDiscoverySessionContext   `json:"context"`
		Available bool                             `json:"available"`
		Truncated bool                             `json:"truncated"`
		Records   []PrivateDiscoveryResponseRecord `json:"records"`
	}{d.Context, d.Available, d.Truncated, d.Records[:d.Count]})
}

// Charge all fixed stores and eight bounded value-copy handoffs before Starter
// construction. This is logical backing/copy accounting, not physical heap or
// stack containment. No ceiling, currency, refund, or kernel policy is changed.
const privateResponseFactsCharge = 8*uint64(unsafe.Sizeof(PrivateDiscoveryResponseFacts{})) + privateDiscoveryContextCharge

func (w *privateDiscoveryWindow) prepayResponseFacts() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts != nil {
		return false
	}
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, privateResponseFactsCharge}, privateDiscoveryCharge{&w.remaining.Work, privateResponseFactsCharge}) {
		return false
	}
	w.responseFacts = &PrivateDiscoveryResponseFacts{Available: true}
	return true
}
func (w *privateDiscoveryWindow) beginResponseFacts(context ...PrivateDiscoveryCorrelation) (int, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts == nil || w.failed || w.ctx.Err() != nil {
		return -1, false
	}
	d := w.responseFacts
	if int(d.Count) == len(d.Records) {
		d.Truncated = true
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return -1, false
	}
	i := int(d.Count)
	d.Count++
	d.Records[i].Ordinal = uint16(i + 1)
	if w.capture != nil {
		d.Records[i].Capture = PrivateCaptureRecord{Status: CaptureNotCaptured, Reason: CaptureNotSelected}
	}
	if len(context) == 1 && context[0].valid() {
		d.Records[i].Correlation = context[0]
	}
	return i, true
}
func (w *privateDiscoveryWindow) observeResponseFacts(i int, f *privateDiscoveryFrame, disposition PrivateDiscoveryResponseDisposition) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts == nil || i < 0 || i >= int(w.responseFacts.Count) || !f.owned(w) {
		return
	}
	r := &w.responseFacts.Records[i]
	if disposition == PrivateResponseSelected {
		r.Selected = f.facts
		r.SelectedRead = f.stages
		if w.capture != nil {
			r.Capture = PrivateCaptureRecord{Status: CaptureNotCaptured, Reason: CaptureNotErrorObject}
			if f.facts.Error == lspwire.ResponseAbsent || f.facts.Error == lspwire.ResponseNull {
				r.Capture = PrivateCaptureRecord{Status: CaptureAbsent}
			} else if f.facts.State == lspwire.ResponseValidJSON && f.facts.Association == lspwire.ResponseIDMatched && f.facts.Error == lspwire.ResponseObject {
				r.Capture = w.capture.capture(w.ctx, f.raw, f.errorSpan.Start, f.errorSpan.End)
			}
		}
	}
	// One increment per frame; disposition refinements do not count it twice.
	if disposition == PrivateResponseBoundary || disposition == PrivateResponseDecodeRejected || disposition == PrivateResponseJoined {
		if r.ReadAttempts < 4096 {
			r.ReadAttempts++
		}
	}
	r.LastRead = f.stages
	r.LastFrame = f.facts
	r.LastDisposition = disposition
}
func (w *privateDiscoveryWindow) responseFactsSnapshot() PrivateDiscoveryResponseFacts {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts == nil {
		return PrivateDiscoveryResponseFacts{}
	}
	return *w.responseFacts
}

// ResponseFacts returns an independent fixed value, including after finalization.
// Repeated caller copies/serialization are caller-owned work, not funded effects.
func (s *PrivateDiscoverySession) ResponseFacts() PrivateDiscoveryResponseFacts {
	if s == nil || s.client.w == nil {
		return PrivateDiscoveryResponseFacts{}
	}
	return s.client.w.responseFactsSnapshot()
}
