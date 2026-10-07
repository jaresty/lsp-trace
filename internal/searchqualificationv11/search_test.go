package searchqualificationv11

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func fixtureDescribe() DescribeBinding {
	d := DescribeBinding{GenerationID: Generation, DescribeDigest: Digest([]byte("describe")), PolicyDigest: Digest([]byte("policy")), EvaluationDigest: Digest([]byte("evaluation"))}
	for i := 1; i <= MemberCount; i++ {
		mk := func(l string) Identity {
			return Identity{fmt.Sprintf("%s-request-%02d", l, i), fmt.Sprintf("%s-attempt-%02d", l, i), fmt.Sprintf("%s-receipt-%02d", l, i), fmt.Sprintf("%s-record-%02d", l, i), Digest([]byte(fmt.Sprintf("%s-raw-%02d", l, i))), Digest([]byte(fmt.Sprintf("%s-evidence-%02d", l, i))), Digest([]byte(fmt.Sprintf("%s-source-%02d", l, i)))}
		}
		d.Members = append(d.Members, MemberBinding{i, fmt.Sprintf("member-%02d", i), "primary+replay", mk("primary"), mk("replay")})
	}
	return d
}
func fixtureRequest(t *testing.T) SearchRequest {
	t.Helper()
	r, e := Prepare(fixtureDescribe(), "bounded search", Digest([]byte("index")))
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func hostRaw(r SearchRequest, disp string, n int) []byte {
	h := HostResult{Version + ".host-result", r.GenerationID, r.RequestID, r.AttemptID, "assignment-1", disp, nil}
	for i := 0; i < n; i++ {
		h.Evaluations = append(h.Evaluations, MemberEvaluation{r.OrderedMembers[i], fmt.Sprintf("0.%06d", 900000-i*10000), "semantic rationale"})
	}
	b, _ := json.Marshal(h)
	return append(b, '\n')
}
func mustIngest(t *testing.T, w *Writer, r SearchRequest, raw []byte) (Attempt, SearchResult) {
	t.Helper()
	a, x, e := w.Ingest(r, "assignment-1", raw, Digest(raw))
	if e != nil {
		t.Fatal(e)
	}
	return a, x
}

func TestExactEnumSets(t *testing.T) {
	if got, want := fmt.Sprint(OperationOutcomes), "[COMPLETE INVALID_QUERY INDEX_UNAVAILABLE INDEX_MISMATCH CANCELLED TIMEOUT RESOURCE_LIMIT BACKEND_FAILURE POLICY_MISMATCH]"; got != want {
		t.Fatalf("operation enums %s", got)
	}
	if got, want := fmt.Sprint(MemberOutcomes), "[RETURNED BELOW_THRESHOLD FILTERED_BY_POLICY DUPLICATE_MEMBER INVALID_MEMBER]"; got != want {
		t.Fatalf("member enums %s", got)
	}
	if got, want := fmt.Sprint(AttemptStates), "[RECEIVED ADAPTED COMMITTED TERMINAL_INVALID]"; got != want {
		t.Fatalf("attempt enums %s", got)
	}
	if got, want := fmt.Sprint(ReviewChecks), "[request_bound raw_bound result_bound pair_bound evidence_bound limits_bound semantic_complete ledger_balanced custody_procedural no_feature_identity]"; got != want {
		t.Fatalf("review checks %s", got)
	}
}

func TestPrepareExactBinding(t *testing.T) {
	d := fixtureDescribe()
	r, e := Prepare(d, "q", Digest([]byte("index")))
	if e != nil || len(r.OrderedMembers) != 24 || r.DescribeDigest != d.DescribeDigest {
		t.Fatalf("prepare: %v", e)
	}
	for _, mut := range []func(*DescribeBinding){func(x *DescribeBinding) { x.Members = x.Members[:23] }, func(x *DescribeBinding) { x.Members[0].Replay.RequestID = x.Members[0].Primary.RequestID }, func(x *DescribeBinding) { x.Members[0].Primary.RawDigest = "bad" }} {
		x := fixtureDescribe()
		mut(&x)
		if _, e = Prepare(x, "q", Digest([]byte("index"))); e == nil {
			t.Fatal("invalid Describe accepted")
		}
	}
}
func TestLimitsAndPrecedence(t *testing.T) {
	l := FixedLimits()
	if l.Depth != 1 || l.Frontier != 24 || l.Evaluations != 24 || l.WorkMax != 240 || l.ResultCount != 5 || l.ResponseBytes != 65536 {
		t.Fatalf("limits: %+v", l)
	}
	want := []string{"REQUEST", "ASSIGNMENT", "RAW_SIZE", "UTF8", "NDJSON_FRAME", "STRICT_JSON", "DIGEST", "CANCELLATION", "DEADLINE", "FRONTIER", "EVALUATION", "WORK_PRECHARGE", "RESULT_COUNT", "RESPONSE_BYTES"}
	if fmt.Sprint(Precedence) != fmt.Sprint(want) {
		t.Fatal("precedence")
	}
	good := AdmissionState{true, true, true, 65536, 24, 24, 230, 10, 5, true, true, true, false, false}
	if got := ClassifyAdmission(good); got != "ADMIT" {
		t.Fatal(got)
	}
	mutations := []struct {
		name, want string
		apply      func(*AdmissionState)
	}{
		{"request", "REQUEST", func(s *AdmissionState) { s.RequestValid = false }}, {"assignment", "ASSIGNMENT", func(s *AdmissionState) { s.AssignmentValid = false }},
		{"bytes+1", "RAW_SIZE", func(s *AdmissionState) { s.RawBytes = 65537 }}, {"utf8", "UTF8", func(s *AdmissionState) { s.UTF8 = false }},
		{"frame", "NDJSON_FRAME", func(s *AdmissionState) { s.Frame = false }}, {"json", "STRICT_JSON", func(s *AdmissionState) { s.StrictJSON = false }},
		{"digest", "DIGEST", func(s *AdmissionState) { s.DigestValid = false }}, {"cancel-before-deadline", "CANCELLED", func(s *AdmissionState) { s.Cancelled = true; s.DeadlineExpired = true }},
		{"deadline", "DEADLINE", func(s *AdmissionState) { s.DeadlineExpired = true }}, {"frontier", "FRONTIER", func(s *AdmissionState) { s.Frontier = 25 }},
		{"evaluation", "EVALUATION", func(s *AdmissionState) { s.Evaluations = 25 }}, {"work+1", "WORK_PRECHARGE", func(s *AdmissionState) { s.Work = 231 }},
		{"sixth", "RESULT_COUNT", func(s *AdmissionState) { s.ResultCount = 6 }},
	}
	for _, tc := range mutations {
		s := good
		tc.apply(&s)
		if got := ClassifyAdmission(s); got != tc.want {
			t.Errorf("%s got %s want %s", tc.name, got, tc.want)
		}
	}
}
func TestHostSemanticShape(t *testing.T) {
	r := fixtureRequest(t)
	for _, d := range HostDispositions {
		n := 0
		if d == "RESULT" {
			n = 24
		}
		if _, e := ParseHostResult(hostRaw(r, d, n), r, "assignment-1", ""); e != nil {
			t.Fatalf("%s: %v", d, e)
		}
	}
	raw := hostRaw(r, "RESULT", 24)
	var h HostResult
	json.Unmarshal(raw[:len(raw)-1], &h)
	h.Evaluations[0].Score = ".900000"
	b, _ := json.Marshal(h)
	if _, e := ParseHostResult(append(b, '\n'), r, "assignment-1", ""); e == nil {
		t.Fatal("malformed score accepted")
	}
}
func TestResponseByteBoundary(t *testing.T) {
	r := fixtureRequest(t)
	h := HostResult{Version + ".host-result", r.GenerationID, r.RequestID, r.AttemptID, "assignment-1", "UNAVAILABLE", nil}
	base, _ := json.Marshal(h)
	// Use one rationale-bearing partial evaluation to tune canonical bytes exactly.
	h.Disposition = "FAILED"
	h.Evaluations = []MemberEvaluation{{r.OrderedMembers[0], "0.700000", "x"}}
	base, _ = json.Marshal(h)
	pad := MaxResponseBytes - 1 - len(base)
	h.Evaluations[0].Rationale = strings.Repeat("x", pad+1)
	b, _ := json.Marshal(h)
	raw := append(b, '\n')
	if len(raw) != MaxResponseBytes {
		t.Fatalf("boundary=%d", len(raw))
	}
	if _, e := ParseHostResult(raw, r, "assignment-1", Digest(raw)); e != nil {
		t.Fatal(e)
	}
	if _, e := ParseHostResult(append(raw, 'x'), r, "assignment-1", ""); e == nil || e.Error() != "RAW_SIZE" {
		t.Fatalf("+1: %v", e)
	}
}

func TestStrictIngestAdversarial(t *testing.T) {
	r := fixtureRequest(t)
	valid := hostRaw(r, "RESULT", 24)
	cases := map[string][]byte{"invalid_utf8": {0xff, '\n'}, "partial": valid[:len(valid)-1], "trailing": append(valid, 'x', '\n'), "unknown": []byte(`{"schemaVersion":"` + Version + `.host-result","requestID":"` + r.RequestID + `","assignmentID":"assignment-1","disposition":"UNAVAILABLE","evaluations":[],"x":1}` + "\n"), "duplicate_key": []byte(`{"schemaVersion":"` + Version + `.host-result","schemaVersion":"` + Version + `.host-result","requestID":"` + r.RequestID + `","assignmentID":"assignment-1","disposition":"UNAVAILABLE","evaluations":[]}` + "\n")}
	for n, b := range cases {
		if _, e := ParseHostResult(b, r, "assignment-1", ""); e == nil {
			t.Errorf("%s accepted", n)
		}
	}
	h := HostResult{Version + ".host-result", r.GenerationID, r.RequestID, r.AttemptID, "assignment-1", "RESULT", nil}
	for i := 0; i < 24; i++ {
		m := r.OrderedMembers[i]
		if i == 23 {
			m = r.OrderedMembers[0]
		}
		h.Evaluations = append(h.Evaluations, MemberEvaluation{m, "0.700000", "r"})
	}
	b, _ := json.Marshal(h)
	if _, e := ParseHostResult(append(b, '\n'), r, "assignment-1", ""); e == nil {
		t.Fatal("duplicate member")
	}
	h.Evaluations[23].MemberID = "unknown"
	b, _ = json.Marshal(h)
	if _, e := ParseHostResult(append(b, '\n'), r, "assignment-1", ""); e == nil {
		t.Fatal("unknown member")
	}
	if _, e := ParseHostResult(valid, r, "assignment-1", Digest([]byte("wrong"))); e == nil {
		t.Fatal("digest mismatch")
	}
	big := append(bytes.Repeat([]byte{' '}, MaxResponseBytes), '\n')
	if _, e := ParseHostResult(big, r, "assignment-1", ""); e == nil {
		t.Fatal("size +1")
	}
}
func TestAdapterLedgerTopKAndPartial(t *testing.T) {
	r := fixtureRequest(t)
	w := NewWriter()
	_, x := mustIngest(t, w, r, hostRaw(r, "RESULT", 24))
	if x.Completeness != "COMPLETE" || len(x.Ledger) != 24 || len(x.Selected) != 5 || x.Unevaluated != 0 {
		t.Fatalf("complete %+v", x)
	}
	r2 := r
	r2.RequestID += "-partial"
	_, p := mustIngest(t, NewWriter(), r2, hostRaw(r2, "REFUSE", 4))
	if p.Completeness != "INCOMPLETE" || len(p.Ledger) != 4 || p.Unevaluated != 20 || p.Denominator != 24 {
		t.Fatalf("partial %+v", p)
	}
	r3 := r
	r3.RequestID += "-empty"
	raw := hostRaw(r3, "RESULT", 24)
	var h HostResult
	json.Unmarshal(raw[:len(raw)-1], &h)
	for i := range h.Evaluations {
		h.Evaluations[i].Score = "0.590000"
	}
	b, _ := json.Marshal(h)
	_, empty := mustIngest(t, NewWriter(), r3, append(b, '\n'))
	if len(empty.Selected) != 0 {
		t.Fatal("empty not empty")
	}
}
func TestDeliveryReplayConflict(t *testing.T) {
	r := fixtureRequest(t)
	w := NewWriter()
	raw := hostRaw(r, "RESULT", 24)
	a, x := mustIngest(t, w, r, raw)
	a2, x2, e := w.Ingest(r, "assignment-1", raw, Digest(raw))
	if e != nil || a2.Key != a.Key || x2.ResultID != x.ResultID {
		t.Fatal("identical delivery not idempotent")
	}
	bad := append([]byte(nil), raw...)
	bad[len(bad)-2] = ' '
	a3, _, e := w.Ingest(r, "assignment-1", bad, Digest(bad))
	if e == nil || a3.ResultID != x.ResultID || fmt.Sprint(a3.States) != "[RECEIVED ADAPTED COMMITTED]" || len(w.conflicts) != 1 {
		t.Fatalf("conflict mutated history %+v conflicts=%+v err=%v", a3, w.conflicts, e)
	}
}
func TestSingleWriterOneAttempt(t *testing.T) {
	r := fixtureRequest(t)
	w := NewWriter()
	raw := hostRaw(r, "RESULT", 24)
	const n = 20
	ch := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { _, _, e := w.Ingest(r, "assignment-1", raw, Digest(raw)); ch <- e }()
	}
	for i := 0; i < n; i++ {
		if e := <-ch; e != nil {
			t.Fatal(e)
		}
	}
	if len(w.attempts) != 1 || len(w.results) != 1 {
		t.Fatalf("attempts=%d results=%d", len(w.attempts), len(w.results))
	}
}
func fixtureCustody(r SearchRequest, a Attempt, x SearchResult) ParentCustodyEvidence {
	b := canon(x)
	return ParentCustodyEvidence{SchemaVersion: Version + ".parent-custody-evidence", GenerationID: r.GenerationID, ProducerAssignmentID: "assignment-1", ProducerWorkerID: "producer-worker", ProducerTaskID: "producer-task", ProducerRole: "producer", ReviewerAssignmentID: "review-assignment", ReviewerWorkerID: "reviewer-worker", ReviewerTaskID: "reviewer-task", ReviewerRole: "reviewer", OutputSelector: "result:" + x.ResultID, OutputDigest: Digest(b), OutputLength: len(b), EventSelector: "committed:" + a.Key, EventDigest: a.RawDigest, EventLength: len(a.RawBytes)}
}
func reviewerRaw(checks map[string]bool, reasons map[string]string) []byte {
	b, _ := json.Marshal(ReviewerRaw{checks, reasons})
	return append(b, '\n')
}
func allReviewChecks(value bool) map[string]bool {
	m := map[string]bool{}
	for _, k := range ReviewChecks {
		m[k] = value
	}
	return m
}
func fixtureReviewed(t *testing.T) (SearchRequest, Attempt, SearchResult, ReviewRequest, ReviewerAttempt, Review) {
	t.Helper()
	r := fixtureRequest(t)
	a, x := mustIngest(t, NewWriter(), r, hostRaw(r, "RESULT", 24))
	q, err := NewReviewRequest(r, a, x, fixtureCustody(r, a, x))
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewReviewerAttemptKey(q, "review-assignment", "review-attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	raw := reviewerRaw(allReviewChecks(true), map[string]string{})
	ra, rv, err := NewReviewWriter().Ingest(q, key, raw, Digest(raw))
	if err != nil {
		t.Fatal(err)
	}
	return r, a, x, q, ra, rv
}
func outcomeCounts(x SearchResult) map[string]int {
	m := map[string]int{}
	for _, k := range MemberOutcomes {
		m[k] = 0
	}
	for _, e := range x.Ledger {
		m[e.Outcome]++
	}
	return m
}

func TestParentCustodyEvidenceRoleAndDigest(t *testing.T) {
	r := fixtureRequest(t)
	a, x := mustIngest(t, NewWriter(), r, hostRaw(r, "RESULT", 24))
	e := fixtureCustody(r, a, x)
	if err := ValidateParentCustodyEvidence(e); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ParentCustodyEvidence){
		"role_overlap":     func(e *ParentCustodyEvidence) { e.ReviewerRole = e.ProducerRole },
		"worker_overlap":   func(e *ParentCustodyEvidence) { e.ReviewerWorkerID = e.ProducerWorkerID },
		"digest":           func(e *ParentCustodyEvidence) { e.OutputDigest = "SHA256:bad" },
		"identity_ceiling": func(e *ParentCustodyEvidence) { e.ReviewerTaskID = strings.Repeat("x", MaxCustodyIDBytes+1) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := e
			mutate(&bad)
			if ValidateParentCustodyEvidence(bad) == nil {
				t.Fatal("accepted")
			}
		})
	}
	e.ProducerAssignmentID = "other"
	if _, err := NewReviewRequest(r, a, x, e); err == nil {
		t.Fatal("assignment mismatch accepted")
	}
}
func TestReviewRequestExactIdentityBindings(t *testing.T) {
	r := fixtureRequest(t)
	a, x := mustIngest(t, NewWriter(), r, hostRaw(r, "RESULT", 24))
	e := fixtureCustody(r, a, x)
	if _, err := NewReviewRequest(r, a, x, e); err != nil {
		t.Fatal(err)
	}
	badA := a
	badA.AttemptID += "-substitute"
	if _, err := NewReviewRequest(r, badA, x, e); err == nil {
		t.Fatal("attempt substitution accepted")
	}
	badX := x
	badX.GenerationID += "-other"
	if _, err := NewReviewRequest(r, a, badX, e); err == nil {
		t.Fatal("generation substitution accepted")
	}
	e.OutputLength++
	if _, err := NewReviewRequest(r, a, x, e); err == nil {
		t.Fatal("length mismatch accepted")
	}
}
func TestReviewerRawStrictAdversarial(t *testing.T) {
	valid := reviewerRaw(allReviewChecks(true), map[string]string{})
	if _, err := ParseReviewerRaw(valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"utf8": {0xff, '\n'}, "partial": valid[:len(valid)-1], "trailing": append(valid, '\n'),
		"duplicate":        []byte(`{"checks":{},"checks":{},"reasons":{}}` + "\n"),
		"nested_duplicate": []byte(`{"checks":{"request_bound":true,"request_bound":false},"reasons":{}}` + "\n"),
		"unknown":          []byte(`{"checks":{},"reasons":{},"verdict":"ACCEPT"}` + "\n"),
		"noncanonical":     []byte("{ \"checks\":{},\"reasons\":{}}\n"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseReviewerRaw(raw); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	if _, err := ParseReviewerRaw(append(bytes.Repeat([]byte{'x'}, MaxResponseBytes), '\n')); err == nil || err.Error() != "RAW_SIZE" {
		t.Fatalf("size: %v", err)
	}
}
func TestReviewFalseAcceptImpossible(t *testing.T) {
	r, a, x, q, _, _ := fixtureReviewed(t)
	_ = r
	_ = a
	_ = x
	checks := allReviewChecks(true)
	checks[ReviewChecks[0]] = false
	if _, err := DeriveReview(q, ReviewerRaw{checks, map[string]string{}}); err == nil {
		t.Fatal("false without reason accepted")
	}
	rv, err := DeriveReview(q, ReviewerRaw{checks, map[string]string{ReviewChecks[0]: "request mismatch"}})
	if err != nil || rv.Verdict != "REJECT" {
		t.Fatalf("%v %+v", err, rv)
	}
	if _, err := DeriveReview(q, ReviewerRaw{allReviewChecks(true), map[string]string{ReviewChecks[0]: "static verdict surrogate"}}); err == nil {
		t.Fatal("reason on true accepted")
	}
}
func TestReviewAttemptConflictAndSecondAttempt(t *testing.T) {
	r := fixtureRequest(t)
	a, x := mustIngest(t, NewWriter(), r, hostRaw(r, "RESULT", 24))
	q, _ := NewReviewRequest(r, a, x, fixtureCustody(r, a, x))
	key, _ := NewReviewerAttemptKey(q, "review-assignment", "review-attempt-1")
	w := NewReviewWriter()
	raw := reviewerRaw(allReviewChecks(true), map[string]string{})
	committed, rv, err := w.Ingest(q, key, raw, Digest(raw))
	if err != nil {
		t.Fatal(err)
	}
	replay, rv2, err := w.Ingest(q, key, raw, Digest(raw))
	if err != nil || replay.ReviewID != rv.ReviewID || rv2.ReviewID != rv.ReviewID {
		t.Fatal("idempotency")
	}
	conflict := reviewerRaw(allReviewChecks(false), map[string]string{"request_bound": "x", "raw_bound": "x", "result_bound": "x", "pair_bound": "x", "evidence_bound": "x", "limits_bound": "x", "semantic_complete": "x", "ledger_balanced": "x", "custody_procedural": "x", "no_feature_identity": "x"})
	preserved, _, err := w.Ingest(q, key, conflict, Digest(conflict))
	if err == nil || preserved.ReviewID != committed.ReviewID || len(w.conflicts) != 1 {
		t.Fatal("conflict did not preserve commit")
	}
	key2, _ := NewReviewerAttemptKey(q, "review-assignment", "review-attempt-2")
	if _, _, err := w.Ingest(q, key2, raw, Digest(raw)); err == nil || err.Error() != "SECOND_REVIEW_ATTEMPT" {
		t.Fatalf("second: %v", err)
	}
}
func TestMakeAccountExactOutcomesAndBindings(t *testing.T) {
	r, a, x, q, ra, rv := fixtureReviewed(t)
	counts := outcomeCounts(x)
	sel := AccountSelection{r.GenerationID, r.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
	acct, err := MakeAccount(sel, r, a, x, q, ra, rv, counts)
	if err != nil || !acct.Balanced || acct.MechanicalPercent != 100 || acct.CustodyPercent != 100 || acct.AccountingPercent != 100 || acct.CriticalReviewPercent != 100 || acct.SemanticUsefulnessQualified || acct.FeatureIdentity != "UNRESOLVED" {
		t.Fatalf("%v %+v", err, acct)
	}
	for name, mutate := range map[string]func(map[string]int){
		"missing":  func(m map[string]int) { delete(m, "INVALID_MEMBER") },
		"unknown":  func(m map[string]int) { m["UNKNOWN"] = 0 },
		"mismatch": func(m map[string]int) { m["RETURNED"]++ },
	} {
		t.Run(name, func(t *testing.T) {
			m := outcomeCounts(x)
			mutate(m)
			if _, err := MakeAccount(sel, r, a, x, q, ra, rv, m); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	badSel := sel
	badSel.GenerationID += "-other"
	if _, err := MakeAccount(badSel, r, a, x, q, ra, rv, counts); err == nil {
		t.Fatal("generation substitution")
	}
	badSel = sel
	badSel.ReviewerAttemptKey.AttemptID += "-other"
	if _, err := MakeAccount(badSel, r, a, x, q, ra, rv, counts); err == nil {
		t.Fatal("review key substitution")
	}
	partial := x
	partial.Completed--
	if _, err := MakeAccount(sel, r, a, partial, q, ra, rv, counts); err == nil {
		t.Fatal("unbalanced partial accounting accepted")
	}

	partialReq := r
	partialReq.RequestID += "-partial-account"
	partialA, partialResult := mustIngest(t, NewWriter(), partialReq, hostRaw(partialReq, "REFUSE", 4))
	partialQ, err := NewReviewRequest(partialReq, partialA, partialResult, fixtureCustody(partialReq, partialA, partialResult))
	if err != nil {
		t.Fatal(err)
	}
	partialKey, _ := NewReviewerAttemptKey(partialQ, "review-assignment", "review-attempt-1")
	partialRaw := reviewerRaw(allReviewChecks(true), map[string]string{})
	partialRA, partialReview, err := NewReviewWriter().Ingest(partialQ, partialKey, partialRaw, Digest(partialRaw))
	if err != nil {
		t.Fatal(err)
	}
	partialSel := AccountSelection{partialReq.GenerationID, partialReq.RequestID, partialA.Key, partialResult.ResultID, partialRA.Key, partialReview.ReviewID}
	if acct, err := MakeAccount(partialSel, partialReq, partialA, partialResult, partialQ, partialRA, partialReview, outcomeCounts(partialResult)); err != nil || !acct.Balanced {
		t.Fatalf("valid partial: %v %+v", err, acct)
	}
}
func TestCanonicalRequestDeterministic(t *testing.T) {
	a := fixtureRequest(t)
	b := fixtureRequest(t)
	if !bytes.Equal(canon(a), canon(b)) || a.RequestID != b.RequestID {
		t.Fatal("nondeterministic")
	}
	if !strings.Contains(string(canon(a)), "bounded search") {
		t.Fatal("query absent")
	}
}
