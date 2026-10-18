package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func privateProvisionalFixture(t *testing.T, kinds []string) (*MemoryStore, string, privateCallerResultAdmission) {
	t.Helper()
	store, _, _, preparedSelector, requests := privateCallerPreparedFixture(t)
	if len(kinds) == 0 {
		t.Fatal("fixture requires at least one outcome kind")
	}
	rows := make([]privateCallerResultSubmissionRow, len(requests))
	for i, request := range requests {
		rows[i] = privateCallerResultSubmissionRow{RequestOrdinal: i, RecordID: request.RecordID, MessageID: request.Envelope.MessageID, LineageIdentity: request.Lineage.LineageIdentity, Outcome: privateCallerOutcome(t, kinds[i%len(kinds)])}
	}
	submission, err := json.Marshal(privateCallerResultSubmissionWire{SchemaVersion: privateCallerResultSubmissionSchema, Provenance: privateCallerResultObserved, Results: rows})
	if err != nil {
		t.Fatal(err)
	}
	resultSelector, err := RetainPrivateCallerResultSelector(context.Background(), store, preparedSelector, submission)
	if err != nil {
		t.Fatal(err)
	}
	admissionSelector, err := AdmitPrivateCallerResultSelector(context.Background(), store, resultSelector)
	if err != nil {
		t.Fatal(err)
	}
	admissionRaw, _ := store.Get(context.Background(), admissionSelector)
	admission, err := parsePrivateCallerResultAdmission(admissionRaw)
	if err != nil {
		t.Fatal(err)
	}
	return store, admissionSelector, admission
}

func TestPrivateProvisionalCandidateAllArmsMixedOrderAndCandidateClosure(t *testing.T) {
	allKinds := []string{privateCallerOutcomeSuccess, privateCallerOutcomePartial, privateCallerOutcomeFailed, privateCallerOutcomeRefused, privateCallerOutcomeUnknown}
	for _, kind := range allKinds {
		t.Run(kind, func(t *testing.T) {
			store, admissionSelector, _ := privateProvisionalFixture(t, []string{kind})
			selector, err := PreparePrivateProvisionalCandidateSelector(context.Background(), store, admissionSelector)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := store.Get(context.Background(), selector)
			artifact, err := parsePrivateProvisionalCandidate(raw)
			if err != nil || len(artifact.wire.Descriptions) != 2 {
				t.Fatalf("parse=%v descriptions=%d", err, len(artifact.wire.Descriptions))
			}
			wantCandidates := 0
			if kind == privateCallerOutcomeSuccess {
				wantCandidates = 2
			}
			if len(artifact.wire.Candidates) != wantCandidates {
				t.Fatalf("kind=%s candidates=%d want=%d", kind, len(artifact.wire.Candidates), wantCandidates)
			}
		})
	}

	kinds := []string{privateCallerOutcomeUnknown, privateCallerOutcomeSuccess}
	store, admissionSelector, admission := privateProvisionalFixture(t, kinds)
	selector, err := PreparePrivateProvisionalCandidateSelector(context.Background(), store, admissionSelector)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateProvisionalCandidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	w := artifact.wire
	if w.SchemaVersion != privateProvisionalCandidateSchema || w.Status != privateProvisionalCandidateStatus || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || w.FeatureIdentity != "UNRESOLVED" || w.SemanticQualification != "NOT_PERFORMED" || w.AutomaticProgressionAllowed {
		t.Fatalf("fixed fields: %+v", w)
	}
	if w.AdmissionSelector != admissionSelector || w.AdmissionID != admission.ID() || w.OriginalAccounting.Status != "UNAVAILABLE" || w.OriginalAccounting.RequestsCount != nil || w.OriginalAccounting.PreparedCount != nil {
		t.Fatalf("bindings/accounting: %+v", w)
	}
	wantDispositions := []string{privateDispositionUnresolved, privateDispositionCandidate}
	if len(w.Descriptions) != len(kinds) || len(w.Candidates) != 1 {
		t.Fatalf("descriptions=%d candidates=%d", len(w.Descriptions), len(w.Candidates))
	}
	for i, description := range w.Descriptions {
		if description.Ordinal != i || description.OutcomeKind != kinds[i] || description.Disposition != wantDispositions[i] || description.Outcome == nil || len(description.Outcome) == 0 || digestOf(description.Outcome) != description.OutcomeDigest || uint64(len(description.Outcome)) != description.OutcomeLength {
			t.Fatalf("description %d: %+v", i, description)
		}
		if bytes.Contains(description.Outcome, []byte("SOURCE_UNAVAILABLE")) {
			t.Fatalf("invented source unavailable: %s", description.Outcome)
		}
	}
	candidate := w.Candidates[0]
	if candidate.DescriptionID != w.Descriptions[1].DescriptionID || candidate.Revision != 1 || candidate.Status != "PROVISIONAL" || candidate.Label != "" || candidate.Purpose != "" || candidate.Value != "" || candidate.Accepted || candidate.Authority != 0 || candidate.FeatureIdentity != "UNRESOLVED" || candidate.Completeness != ContinuationCompleteness {
		t.Fatalf("candidate: %+v", candidate)
	}
	if candidate.Alternatives == nil || candidate.Evidence == nil || candidate.ContraryEvidence == nil || candidate.RejectionConditions == nil || candidate.InventoryDelta == nil || candidate.SourceRefs == nil {
		t.Fatalf("null candidate collection: %+v", candidate)
	}
	if !bytes.Contains(w.Descriptions[1].Outcome, []byte(`"canonical_semantic"`)) {
		t.Fatalf("success content unavailable to challenge: %s", w.Descriptions[1].Outcome)
	}
}

func TestPrivateProvisionalCandidateStrictCanonicalParserAndLimits(t *testing.T) {
	store, admissionSelector, _ := privateProvisionalFixture(t, []string{privateCallerOutcomeSuccess, privateCallerOutcomeSuccess, privateCallerOutcomeSuccess, privateCallerOutcomeSuccess, privateCallerOutcomeSuccess})
	selector, err := PreparePrivateProvisionalCandidateSelector(context.Background(), store, admissionSelector)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, _ := parsePrivateProvisionalCandidate(raw)
	for name, candidate := range map[string][]byte{
		"unknown":           bytes.Replace(raw, []byte(`"status":`), []byte(`"unknown":0,"status":`), 1),
		"duplicate":         bytes.Replace(raw, []byte(`"status":`), []byte(`"status":"PROVISIONAL_CANDIDATES_PREPARED","status":`), 1),
		"trailing":          append(append([]byte(nil), raw...), []byte(`{}`)...),
		"whitespace":        append([]byte(" "), raw...),
		"null-descriptions": bytes.Replace(raw, mustJSON(artifact.wire.Descriptions), []byte("null"), 1),
		"null-candidates":   bytes.Replace(raw, mustJSON(artifact.wire.Candidates), []byte("null"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePrivateProvisionalCandidate(candidate); err == nil {
				t.Fatalf("accepted %s", name)
			}
		})
	}
	if _, err := parsePrivateProvisionalCandidate(make([]byte, privateProvisionalCandidateMaxBytes+1)); err == nil {
		t.Fatal("accepted oversized artifact")
	}
}

type privateProvisionalFaultStore struct {
	Store
	mu             sync.Mutex
	puts           int
	last           string
	wrongPut       bool
	putErr         bool
	readbackErr    bool
	mutateReadback bool
	cancel         context.CancelFunc
	gets           map[string]int
	changeSelector string
	changeOnRead   int
}

func (s *privateProvisionalFaultStore) Put(ctx context.Context, raw []byte) (string, error) {
	s.mu.Lock()
	s.puts++
	s.mu.Unlock()
	if s.putErr {
		return "", errors.New("put failed")
	}
	selector, err := s.Store.Put(ctx, raw)
	s.last = selector
	if s.cancel != nil {
		s.cancel()
	}
	if s.wrongPut {
		return digestOf([]byte("wrong")), err
	}
	return selector, err
}

func (s *privateProvisionalFaultStore) Get(ctx context.Context, selector string) ([]byte, error) {
	s.mu.Lock()
	if s.gets == nil {
		s.gets = map[string]int{}
	}
	s.gets[selector]++
	n := s.gets[selector]
	s.mu.Unlock()
	if selector == s.last && s.readbackErr {
		return nil, errors.New("readback failed")
	}
	raw, err := s.Store.Get(ctx, selector)
	if err != nil {
		return nil, err
	}
	if selector == s.last && s.mutateReadback {
		raw[len(raw)-1] ^= 1
	}
	if selector == s.changeSelector && n == s.changeOnRead {
		raw[len(raw)-1] ^= 1
	}
	return raw, nil
}

func TestPrivateProvisionalCandidatePredecessorSubstitutionAndNoSideEffects(t *testing.T) {
	store, admissionSelector, admission := privateProvisionalFixture(t, []string{privateCallerOutcomeSuccess, privateCallerOutcomePartial, privateCallerOutcomeFailed, privateCallerOutcomeRefused, privateCallerOutcomeUnknown})
	for name, selector := range map[string]string{"admission": admissionSelector, "result": admission.wire.ResultSelector, "prepared": admission.wire.PreparedSelector, "checkpoint": admission.wire.CheckpointSelector, "requests": admission.wire.RequestsSelector} {
		t.Run(name, func(t *testing.T) {
			fault := &privateProvisionalFaultStore{Store: store, changeSelector: selector, changeOnRead: 2}
			got, err := PreparePrivateProvisionalCandidateSelector(context.Background(), fault, admissionSelector)
			if err == nil || got != "" || fault.puts != 0 {
				t.Fatalf("selector=%q err=%v puts=%d", got, err, fault.puts)
			}
		})
	}
	admissionRaw, _ := store.Get(context.Background(), admissionSelector)
	parsed, _ := parsePrivateCallerResultAdmission(admissionRaw)
	parsed.wire.Rows[0].RecordID = parsed.wire.Rows[1].RecordID
	parsed.wire.AdmissionID = identityJSON(privateCallerResultAdmissionIdentityDomain, parsed.wire, func(v *privateCallerResultAdmissionWire) { v.AdmissionID = "" })
	substituted, _ := json.Marshal(parsed.wire)
	subSelector, _ := store.Put(context.Background(), substituted)
	fault := &privateProvisionalFaultStore{Store: store}
	if got, err := PreparePrivateProvisionalCandidateSelector(context.Background(), fault, subSelector); err == nil || got != "" || fault.puts != 0 {
		t.Fatalf("row substitution selector=%q err=%v puts=%d", got, err, fault.puts)
	}
}

func TestPrivateProvisionalCandidatePutReadbackCancellationReplayAndConcurrency(t *testing.T) {
	base, admissionSelector, _ := privateProvisionalFixture(t, []string{privateCallerOutcomeSuccess, privateCallerOutcomePartial, privateCallerOutcomeFailed, privateCallerOutcomeRefused, privateCallerOutcomeUnknown})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := PreparePrivateProvisionalCandidateSelector(cancelled, &privateProvisionalFaultStore{Store: base}, admissionSelector); err == nil || got != "" {
		t.Fatalf("pre-cancel selector=%q err=%v", got, err)
	}
	for name, configure := range map[string]func(*privateProvisionalFaultStore){
		"put-error":         func(s *privateProvisionalFaultStore) { s.putErr = true },
		"wrong-put":         func(s *privateProvisionalFaultStore) { s.wrongPut = true },
		"readback-error":    func(s *privateProvisionalFaultStore) { s.readbackErr = true },
		"readback-mutation": func(s *privateProvisionalFaultStore) { s.mutateReadback = true },
	} {
		t.Run(name, func(t *testing.T) {
			fault := &privateProvisionalFaultStore{Store: base}
			configure(fault)
			got, err := PreparePrivateProvisionalCandidateSelector(context.Background(), fault, admissionSelector)
			if err == nil || got != "" || fault.puts != 1 {
				t.Fatalf("selector=%q err=%v puts=%d", got, err, fault.puts)
			}
		})
	}
	ctx, cancelAfterPut := context.WithCancel(context.Background())
	fault := &privateProvisionalFaultStore{Store: base, cancel: cancelAfterPut}
	if got, err := PreparePrivateProvisionalCandidateSelector(ctx, fault, admissionSelector); err == nil || got != "" || fault.puts != 1 || fault.gets[fault.last] != 0 {
		t.Fatalf("post-put cancel selector=%q err=%v puts=%d readbacks=%d", got, err, fault.puts, fault.gets[fault.last])
	}
	successStore := &privateProvisionalFaultStore{Store: base}
	first, err := PreparePrivateProvisionalCandidateSelector(context.Background(), successStore, admissionSelector)
	if err != nil {
		t.Fatal(err)
	}
	if successStore.puts != 1 || successStore.gets[first] != 1 {
		t.Fatalf("successful persistence puts=%d readbacks=%d", successStore.puts, successStore.gets[first])
	}
	second, err := PreparePrivateProvisionalCandidateSelector(context.Background(), base, admissionSelector)
	if err != nil || second != first {
		t.Fatalf("replay first=%q second=%q err=%v", first, second, err)
	}
	const workers = 12
	selectors := make(chan string, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector, err := PreparePrivateProvisionalCandidateSelector(context.Background(), base, admissionSelector)
			selectors <- selector
			errs <- err
		}()
	}
	wg.Wait()
	close(selectors)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for selector := range selectors {
		if selector != first {
			t.Fatalf("concurrent selector=%q want=%q", selector, first)
		}
	}
}
