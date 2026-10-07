package groupqualificationv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func digest(label string) string { return Digest([]byte(label)) }
func committedSearch() SearchAccount {
	counts := map[string]int{"RETURNED": 5, "BELOW_THRESHOLD": 19, "FILTERED_BY_POLICY": 0, "DUPLICATE_MEMBER": 0, "INVALID_MEMBER": 0}
	s := SearchAccount{SchemaVersion: SearchSchemaVersion, FreezeIdentity: SearchFreeze, RequestID: "search-request", RequestDigest: SearchCase01Request, ResultID: "search-result", ResultDigest: SearchCase01Result, ReviewID: "search-review", ReviewDigest: digest("search-review"), ProducerAttemptID: "search-producer-attempt", ProducerAttemptDigest: SearchCase01Producer, ReviewerAttemptID: "search-reviewer-attempt", ReviewerAttemptDigest: SearchCase01ReviewAttempt, ReviewRequestDigest: SearchCase01ReviewRequest, AccountDigest: SearchCase01Account, CustodyDigest: SearchCase01Custody, ExecutionManifestDigest: SearchExecutionManifest, FinalAuditDigest: SearchFinalAudit, FinalSealDigest: SearchFinalSeal, TerminalReportDigest: SearchTerminalReport, ProducerStates: []string{"RECEIVED", "ADAPTED", "COMMITTED"}, ReviewerStates: []string{"RECEIVED", "ADAPTED", "COMMITTED"}, Denominator: 24, Completeness: "COMPLETE", ReviewerVerdict: "ACCEPT_MECHANICAL", Balanced: true, OutcomeCounts: counts, FeatureIdentity: "UNRESOLVED"}
	out := []string{}
	for _, item := range []struct {
		k string
		n int
	}{{"RETURNED", 5}, {"BELOW_THRESHOLD", 19}} {
		for i := 0; i < item.n; i++ {
			out = append(out, item.k)
		}
	}
	for i := 1; i <= 24; i++ {
		s.Members = append(s.Members, SearchMember{Ordinal: i, MemberID: fmt.Sprintf("member-%02d", i), Outcome: out[i-1], FeatureIdentity: "UNRESOLVED"})
	}
	return s
}
func fixture(t *testing.T) GroupRequest {
	t.Helper()
	s := committedSearch()
	r := GroupRequest{SchemaVersion: Version + ".request", RequestID: "group-request", GenerationID: "generation", AttemptID: "attempt-1", AssignmentID: "producer-assignment", SearchFreezeIdentity: SearchFreeze, Search: s, OrderedMembers: append([]SearchMember(nil), s.Members...), PolicyDigest: GroupPolicyDigest, LimitsDigest: GroupLimitsDigest, PartitionCaptureID: "capture", PartitionIdentity: "observation", PartitionSeed: 7, PartitionResolution: "1.0", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Limits: FixedLimits()}
	for i, m := range s.Members {
		e := MemberEvidence{Ordinal: i + 1, MemberID: m.MemberID, Source: &SourceRef{Digest: digest(fmt.Sprintf("source-%d", i)), CaptureID: "capture", ObjectID: fmt.Sprintf("object-%02d", i), ObjectDigest: digest(fmt.Sprintf("object-%d", i)), Complete: true, RangeCount: 1, UniqueBytes: 10}, Community: &CommunityRef{Digest: digest(fmt.Sprintf("community-%d", i)), CaptureID: "capture", ObservationID: "observation", CommunityID: fmt.Sprintf("community-%02d", i/4), MemberID: m.MemberID, Algorithm: "LEIDEN", StructuralOnly: true, Retained: true, Nominated: true, Seed: 7, Resolution: "1.0"}}
		if i < 23 {
			e.Calls = []RelationRef{{Digest: digest(fmt.Sprintf("call-%d", i)), CaptureID: "capture", FromMemberID: m.MemberID, ToMemberID: s.Members[i+1].MemberID, Kind: "CALLS", Provenance: "SERVER_REPORTED", ServerReported: true}}
		}
		r.Evidence = append(r.Evidence, e)
	}
	r.EvidenceDigest = Digest(canon(r.Evidence))
	return r
}
func producerRaw(r GroupRequest) []byte {
	b, _ := json.Marshal(ProducerRaw{Evidence: r.Evidence})
	return append(b, '\n')
}
func requestRaw(r GroupRequest) []byte { b, _ := json.Marshal(r); return append(b, '\n') }
func ingest(t *testing.T, r GroupRequest) (Attempt, Result, *Writer) {
	t.Helper()
	raw := producerRaw(r)
	w := NewWriter()
	a, x, e := w.Ingest(r, raw, Digest(raw), Control{})
	if e != nil {
		t.Fatal(e)
	}
	return a, x, w
}
func allChecks(v bool) map[string]bool {
	m := map[string]bool{}
	for _, k := range ReviewChecks {
		m[k] = v
	}
	return m
}
func reviewed(t *testing.T) (GroupRequest, Attempt, Result, ReviewRequest, ReviewerAttempt, Review) {
	t.Helper()
	r := fixture(t)
	a, x, _ := ingest(t, r)
	q, e := NewReviewRequest(r, a, x, "producer-worker", "producer-task", "producer", "review-assignment", "review-worker", "review-task", "reviewer")
	if e != nil {
		t.Fatal(e)
	}
	k := ReviewerAttemptKey{r.GenerationID, "review-assignment", q.ReviewRequestID, "review-attempt"}
	b, _ := json.Marshal(ReviewerRaw{allChecks(true), map[string]string{}})
	raw := append(b, '\n')
	ra, rv, e := NewReviewWriter().Ingest(r, a, x, q, k, raw, Digest(raw))
	if e != nil {
		t.Fatal(e)
	}
	return r, a, x, q, ra, rv
}

func TestExactEnumClosure(t *testing.T) {
	if fmt.Sprint(OperationOutcomes) != "[COMPLETE INDEX_UNAVAILABLE INDEX_MISMATCH CANCELLED TIMEOUT RESOURCE_LIMIT BACKEND_FAILURE POLICY_MISMATCH]" {
		t.Fatal(OperationOutcomes)
	}
	if fmt.Sprint(MemberOutcomes) != "[GROUPED UNMATCHED FILTERED_BY_POLICY DUPLICATE_MEMBER INVALID_MEMBER]" {
		t.Fatal(MemberOutcomes)
	}
	for _, c := range CauseCodes {
		if contains(OperationOutcomes[:], c) {
			t.Fatalf("cause leaked: %s", c)
		}
	}
}
func TestExactCommittedSearchImport(t *testing.T) {
	s := committedSearch()
	if e := ValidateSearch(s); e != nil {
		t.Fatal(e)
	}
	mutations := map[string]func(*SearchAccount){"schema": func(x *SearchAccount) { x.SchemaVersion += "x" }, "freeze": func(x *SearchAccount) { x.FreezeIdentity = digest("other") }, "manifest": func(x *SearchAccount) { x.ExecutionManifestDigest = digest("other") }, "audit": func(x *SearchAccount) { x.FinalAuditDigest = digest("other") }, "seal": func(x *SearchAccount) { x.FinalSealDigest = digest("other") }, "terminal": func(x *SearchAccount) { x.TerminalReportDigest = digest("other") }, "digest syntax": func(x *SearchAccount) { x.ResultDigest = "bad" }, "states": func(x *SearchAccount) { x.ProducerStates = []string{"RECEIVED", "COMMITTED"} }, "review": func(x *SearchAccount) { x.ReviewerVerdict = "ACCEPT" }, "key missing": func(x *SearchAccount) { delete(x.OutcomeCounts, "INVALID_MEMBER") }, "count": func(x *SearchAccount) { x.OutcomeCounts["RETURNED"]-- }, "member outcome": func(x *SearchAccount) { x.Members[0].Outcome = "GROUPED" }, "identity": func(x *SearchAccount) { x.ProducerAttemptID = x.ReviewerAttemptID }}
	for n, f := range mutations {
		t.Run(n, func(t *testing.T) {
			x := committedSearch()
			f(&x)
			if ValidateSearch(x) == nil {
				t.Fatal("accepted")
			}
		})
	}
}
func TestExactCommittedDigestConstants(t *testing.T) {
	r := fixture(t)
	if r.Search.RequestDigest != SearchCase01Request || r.Search.ResultDigest != SearchCase01Result || r.Search.ProducerAttemptDigest != SearchCase01Producer || r.Search.ReviewRequestDigest != SearchCase01ReviewRequest || r.Search.ReviewerAttemptDigest != SearchCase01ReviewAttempt || r.Search.AccountDigest != SearchCase01Account || r.Search.CustodyDigest != SearchCase01Custody || r.PolicyDigest != GroupPolicyDigest || r.LimitsDigest != GroupLimitsDigest {
		t.Fatal("committed digest mismatch")
	}
	if !reflect.DeepEqual(r.Search.OutcomeCounts, map[string]int{"RETURNED": 5, "BELOW_THRESHOLD": 19, "FILTERED_BY_POLICY": 0, "DUPLICATE_MEMBER": 0, "INVALID_MEMBER": 0}) {
		t.Fatal(r.Search.OutcomeCounts)
	}
}
func TestEvidenceDigestAndStructuralBindings(t *testing.T) {
	r := fixture(t)
	if e := ValidateRequest(r); e != nil {
		t.Fatal(e)
	}
	r.Evidence[0].Source.ObjectDigest = digest("changed")
	if ValidateRequest(r) == nil {
		t.Fatal("stale evidence digest")
	}
	r = fixture(t)
	r.Evidence[0].Calls[0].ToMemberID = "outside"
	r.EvidenceDigest = Digest(canon(r.Evidence))
	_, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil {
		t.Fatal(e)
	}
	r = fixture(t)
	r.Evidence[0].Community.Algorithm = "LOUVAIN"
	r.EvidenceDigest = Digest(canon(r.Evidence))
	x, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil || x.Ledger[0].Outcome != "INVALID_MEMBER" || x.Ledger[0].CauseCode != "INVALID_COMMUNITY" {
		t.Fatalf("%v %+v", e, x.Ledger[0])
	}
}
func TestSourceObjectCollisionIdentity(t *testing.T) {
	r := fixture(t)
	base := r.Evidence[0].Source
	r.Evidence[1].Source.Objects = append(r.Evidence[1].Source.Objects, SourceObject{ObjectID: base.ObjectID, Digest: base.ObjectDigest, SourceDigest: base.Digest, CaptureID: base.CaptureID, Complete: base.Complete, Authority: base.Authority, RangeCount: base.RangeCount, UniqueBytes: base.UniqueBytes})
	r.EvidenceDigest = Digest(canon(r.Evidence))
	if _, e := Adapt(r, "k", digest("raw"), Control{}); e != nil {
		t.Fatalf("exact duplicate rejected: %v", e)
	}
	r.Evidence[1].Source.Objects[0].UniqueBytes++
	r.EvidenceDigest = Digest(canon(r.Evidence))
	if x, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "INVALID_SOURCE" || x.ResultID != "" {
		t.Fatalf("collision accepted: %v %+v", e, x)
	}
	r = fixture(t)
	addObjects(r.Evidence, MaxSourceObjects-MemberCount)
	r.Evidence[23].Source.Objects = append(r.Evidence[23].Source.Objects, SourceObject{ObjectID: r.Evidence[0].Source.ObjectID, Digest: digest("conflict"), SourceDigest: r.Evidence[0].Source.Digest, CaptureID: "capture", Complete: true})
	r.EvidenceDigest = Digest(canon(r.Evidence))
	if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "INVALID_SOURCE" {
		t.Fatalf("limit bypass collision: %v", e)
	}
}
func TestFiveOutcomeConservationDuplicateInvalid(t *testing.T) {
	r := fixture(t)
	r.Evidence[1].MemberID = r.Evidence[0].MemberID
	r.Evidence[2].Source = nil
	r.Evidence[3].Community.Nominated = false
	r.Evidence[4].Filtered = true
	r.EvidenceDigest = Digest(canon(r.Evidence))
	x, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"GROUPED", "DUPLICATE_MEMBER", "INVALID_MEMBER", "UNMATCHED", "FILTERED_BY_POLICY"}
	for i, w := range want {
		if x.Ledger[i].Outcome != w {
			t.Fatalf("slot %d got %s", i, x.Ledger[i].Outcome)
		}
	}
	if len(x.Ledger) != 24 || !exactCounts(MemberOutcomes[:], x.OutcomeCounts, 24) {
		t.Fatalf("conservation %+v", x.OutcomeCounts)
	}
}
func TestDuplicateOrdinalKeepsFirstCanonical(t *testing.T) {
	r := fixture(t)
	r.Evidence[1].Ordinal = 1
	r.EvidenceDigest = Digest(canon(r.Evidence))
	x, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil || x.Ledger[0].Outcome != "GROUPED" || x.Ledger[1].Outcome != "DUPLICATE_MEMBER" || x.Ledger[1].CauseCode != "DUPLICATE_ORDINAL" {
		t.Fatalf("%v %+v", e, x.Ledger[:2])
	}
}
func TestAllCheckpointsAndPrecedence(t *testing.T) {
	r := fixture(t)
	cases := []struct {
		name string
		occ  int
	}{{"BEFORE_IMPORT", 1}, {"BEFORE_MEMBER", 1}, {"BEFORE_RELATION", 1}, {"BEFORE_CANDIDATE", 1}, {"BEFORE_COMMIT", 1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, e := Adapt(r, "k", digest("raw"), Control{TriggerCheckpoint: tc.name, TriggerOccurrence: tc.occ, TriggerCancelled: true, TriggerDeadlineExpired: true, TriggerBackendFailed: true, TriggerWorkLimit: 1})
			if e == nil || e.Error() != "CANCELLATION" || x.Outcome != "CANCELLED" || x.ResultID != "" || len(x.Ledger) != 0 || len(x.Candidates) != 0 {
				t.Fatalf("%v %+v", e, x)
			}
		})
	}
	x, e := Adapt(r, "k", digest("raw"), Control{TriggerCheckpoint: "BEFORE_RELATION", TriggerDeadlineExpired: true, TriggerBackendFailed: true})
	if e == nil || e.Error() != "DEADLINE_EXPIRED" || x.Outcome != "TIMEOUT" {
		t.Fatalf("%v %+v", e, x)
	}
}
func TestWorkBoundaryPlusOne(t *testing.T) {
	r := fixture(t)
	base, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil {
		t.Fatal(e)
	}
	cost := base.Counters.WorkCharged
	if _, e = Adapt(r, "k", digest("raw"), Control{WorkLimit: cost}); e != nil {
		t.Fatal(e)
	}
	x, e := Adapt(r, "k", digest("raw"), Control{WorkLimit: cost - 1})
	if e == nil || x.Outcome != "RESOURCE_LIMIT" || len(x.Ledger) != 0 {
		t.Fatalf("%v %+v", e, x)
	}
}
func TestCommittedLimitBoundaries(t *testing.T) {
	t.Run("relations", func(t *testing.T) {
		r := fixture(t)
		setTotalRelations(&r, MaxRelations)
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e != nil {
			t.Fatal(e)
		}
		setTotalRelations(&r, MaxRelations+1)
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "RELATION_LIMIT" {
			t.Fatal(e)
		}
	})
	t.Run("evidence refs", func(t *testing.T) {
		r := fixture(t)
		r.Evidence[0].Calls = makeRelations(r, 0, 30)
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e != nil {
			t.Fatal(e)
		}
		r.Evidence[0].Calls = append(r.Evidence[0].Calls, r.Evidence[0].Calls[0])
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "EVIDENCE_LIMIT" {
			t.Fatal(e)
		}
	})
	t.Run("source ranges bytes", func(t *testing.T) {
		r := fixture(t)
		for i := range r.Evidence {
			r.Evidence[i].Source.RangeCount = 0
			r.Evidence[i].Source.UniqueBytes = 0
		}
		r.Evidence[0].Source.RangeCount = MaxSourceRanges
		r.Evidence[0].Source.UniqueBytes = MaxUniqueSourceBytes
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e != nil {
			t.Fatal(e)
		}
		r.Evidence[0].Source.RangeCount++
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "SOURCE_RANGE_LIMIT" {
			t.Fatal(e)
		}
		r = fixture(t)
		for i := range r.Evidence {
			r.Evidence[i].Source.UniqueBytes = 0
		}
		r.Evidence[0].Source.UniqueBytes = MaxUniqueSourceBytes + 1
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "SOURCE_BYTES_LIMIT" {
			t.Fatal(e)
		}
	})
	t.Run("source objects", func(t *testing.T) {
		r := fixture(t)
		addObjects(r.Evidence, MaxSourceObjects-MemberCount)
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e != nil {
			t.Fatal(e)
		}
		addObjects(r.Evidence, 1)
		r.EvidenceDigest = Digest(canon(r.Evidence))
		if _, e := Adapt(r, "k", digest("raw"), Control{}); e == nil || e.Error() != "SOURCE_OBJECT_LIMIT" {
			t.Fatal(e)
		}
	})
}
func makeRelations(r GroupRequest, owner, n int) []RelationRef {
	x := make([]RelationRef, n)
	for i := range x {
		x[i] = RelationRef{Digest: digest(fmt.Sprintf("extra-call-%d-%d", owner, i)), CaptureID: "capture", FromMemberID: r.Evidence[owner].MemberID, ToMemberID: r.OrderedMembers[(owner+1)%24].MemberID, Kind: "CALLS", Provenance: "SERVER_REPORTED", ServerReported: true}
	}
	return x
}
func setTotalRelations(r *GroupRequest, total int) {
	for i := range r.Evidence {
		r.Evidence[i].Calls = nil
	}
	for i := 0; total > 0; i++ {
		n := 30
		if total < n {
			n = total
		}
		r.Evidence[i].Calls = makeRelations(*r, i, n)
		total -= n
	}
}
func addObjects(es []MemberEvidence, n int) {
	start := 0
	for i := range es {
		start += len(es[i].Source.Objects)
	}
	for i := 0; i < n; i++ {
		ordinal := start + i
		slot := ordinal % len(es)
		es[slot].Source.Objects = append(es[slot].Source.Objects, SourceObject{ObjectID: fmt.Sprintf("extra-object-%03d", ordinal), Digest: digest(fmt.Sprintf("extra-object-%d", ordinal)), SourceDigest: digest(fmt.Sprintf("extra-source-%d", ordinal)), CaptureID: "capture", Complete: true})
	}
}
func TestRequestParserAdversarial(t *testing.T) {
	r := fixture(t)
	valid := requestRaw(r)
	if _, e := ParseGroupRequest(valid); e != nil {
		t.Fatal(e)
	}
	cases := map[string][]byte{"utf8": {0xff, '\n'}, "partial": valid[:len(valid)-1], "trailing": append(append([]byte{}, valid...), 'x'), "duplicate": []byte(`{"SchemaVersion":"x","SchemaVersion":"y"}` + "\n"), "unknown": []byte(`{"unknown":1}` + "\n"), "noncanonical": append([]byte{' '}, valid...)}
	for n, b := range cases {
		t.Run(n, func(t *testing.T) {
			if _, e := ParseGroupRequest(b); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	big := append(bytes.Repeat([]byte{'x'}, MaxRequestBytes), '\n')
	if _, e := ParseGroupRequest(big); e == nil || e.Error() != "RAW_SIZE" {
		t.Fatal(e)
	}
	r.RequestID = strings.Repeat("é", 129)
	r.EvidenceDigest = Digest(canon(r.Evidence))
	if _, e := ParseGroupRequest(requestRaw(r)); e == nil {
		t.Fatal("identity bytes")
	}
}
func TestCandidateLimitBoundaries(t *testing.T) {
	limits := FixedLimits()
	candidates := make([]Candidate, limits.MaxCandidates)
	for i := range candidates {
		candidates[i] = Candidate{MemberIDs: []string{fmt.Sprintf("m-%d", i)}, MemberOrdinals: []int{i + 1}}
	}
	if e := validateCandidateLimits(candidates, limits); e != nil {
		t.Fatal(e)
	}
	if e := validateCandidateLimits(append(candidates, Candidate{}), limits); e == nil || e.Error() != "CANDIDATE_LIMIT" {
		t.Fatal(e)
	}
	one := Candidate{MemberIDs: make([]string, limits.MaxMembersPerCandidate), MemberOrdinals: make([]int, limits.MaxMembersPerCandidate)}
	if e := validateCandidateLimits([]Candidate{one}, limits); e != nil {
		t.Fatal(e)
	}
	one.MemberIDs = append(one.MemberIDs, "plus-one")
	one.MemberOrdinals = append(one.MemberOrdinals, 25)
	if e := validateCandidateLimits([]Candidate{one}, limits); e == nil || e.Error() != "CANDIDATE_LIMIT" {
		t.Fatal(e)
	}
	occ := []Candidate{{MemberIDs: make([]string, 12), MemberOrdinals: make([]int, 12)}, {MemberIDs: make([]string, 12), MemberOrdinals: make([]int, 12)}}
	if e := validateCandidateLimits(occ, limits); e != nil {
		t.Fatal(e)
	}
	occ[1].MemberIDs = append(occ[1].MemberIDs, "plus-one")
	occ[1].MemberOrdinals = append(occ[1].MemberOrdinals, 25)
	if e := validateCandidateLimits(occ, limits); e == nil || e.Error() != "OCCURRENCE_LIMIT" {
		t.Fatal(e)
	}
}
func TestProducerEvidenceDigestBinding(t *testing.T) {
	r := fixture(t)
	r.EvidenceDigest = digest("wrong")
	raw := producerRaw(r)
	a, x, e := NewWriter().Ingest(r, raw, Digest(raw), Control{})
	if e == nil || e.Error() != "EVIDENCE_DIGEST" || a.ResultID != "" || x.ResultID != "" {
		t.Fatalf("%v %+v %+v", e, a, x)
	}
}
func TestIdentityAndReviewReasonBoundaries(t *testing.T) {
	if !validID(strings.Repeat("x", MaxIdentityBytes)) || validID(strings.Repeat("x", MaxIdentityBytes+1)) {
		t.Fatal("identity byte boundary")
	}
	x := ReviewerRaw{Checks: allChecks(true), Reasons: map[string]string{}}
	x.Checks[ReviewChecks[0]] = false
	x.Reasons[ReviewChecks[0]] = strings.Repeat("x", MaxReviewReasonBytes)
	if e := validateReviewerRaw(x); e != nil {
		t.Fatal(e)
	}
	x.Reasons[ReviewChecks[0]] += "x"
	if e := validateReviewerRaw(x); e == nil || e.Error() != "REVIEW_REASONS" {
		t.Fatal(e)
	}
}
func TestProducerResponseBoundary(t *testing.T) {
	var p ProducerRaw
	base, _ := json.Marshal(p)
	p.Evidence = []MemberEvidence{{MemberID: strings.Repeat("x", MaxResponseBytes-len(base)-30)}}
	b, _ := json.Marshal(p)
	if len(b) < MaxResponseBytes {
		p.Evidence[0].MemberID += strings.Repeat("x", MaxResponseBytes-len(b))
		b, _ = json.Marshal(p)
	}
	raw := append(b, '\n')
	if len(raw) == MaxResponseBytes {
		if _, e := ParseProducerRaw(raw); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := ParseProducerRaw(append(bytes.Repeat([]byte{'x'}, MaxResponseBytes), '\n')); e == nil || e.Error() != "RAW_SIZE" {
		t.Fatal(e)
	}
}
func TestWriterTerminalReplayConflictSecondAttempt(t *testing.T) {
	r := fixture(t)
	raw := producerRaw(r)
	w := NewWriter()
	a, x, e := w.Ingest(r, raw, Digest(raw), Control{})
	if e != nil {
		t.Fatal(e)
	}
	a.RawBytes[0] = 'x'
	x.Ledger[0].MemberID = "x"
	a2, x2, e := w.Ingest(r, raw, Digest(raw), Control{})
	if e != nil || a2.RawBytes[0] == 'x' || x2.Ledger[0].MemberID == "x" {
		t.Fatal("copy")
	}
	bad := append([]byte{}, raw...)
	bad[len(bad)-2] = ' '
	old, _, e := w.Ingest(r, bad, Digest(bad), Control{})
	if e == nil || old.ResultID != a2.ResultID || len(w.Conflicts()) != 1 {
		t.Fatal("conflict")
	}
	r.AttemptID = "attempt-2"
	if _, _, e = w.Ingest(r, raw, Digest(raw), Control{}); e == nil || e.Error() != "SECOND_ATTEMPT" {
		t.Fatal(e)
	}
}
func TestProducerAttemptExactCustody(t *testing.T) {
	r := fixture(t)
	a, x, _ := ingest(t, r)
	other := fixture(t)
	other.Evidence[0].Community.CommunityID = "other-community"
	other.EvidenceDigest = Digest(canon(other.Evidence))
	raw := producerRaw(other)
	forgedA := cloneAttempt(a)
	forgedA.RawBytes = raw
	forgedA.RawDigest = Digest(raw)
	forgedX, e := Adapt(r, producerAttemptKey(r), forgedA.RawDigest, Control{})
	if e != nil {
		t.Fatal(e)
	}
	forgedA.ResultID = forgedX.ResultID
	forgedA.Counters = forgedX.Counters
	if e = ValidateResult(r, forgedA, forgedX); e == nil {
		t.Fatal("unrelated canonical raw accepted")
	}
	if _, e = NewReviewRequest(r, forgedA, forgedX, "pw", "pt", "pr", "ra", "rw", "rt", "rr"); e == nil {
		t.Fatal("unrelated raw reached review")
	}
	wrongKey := domainID("producer-attempt-key", struct{ GenerationID, AssignmentID, RequestID, AttemptID string }{r.GenerationID, r.AssignmentID, r.RequestID, "wrong"})
	wrongA := cloneAttempt(a)
	wrongA.Key = wrongKey
	wrongX, e := Adapt(r, wrongKey, wrongA.RawDigest, Control{})
	if e != nil {
		t.Fatal(e)
	}
	wrongA.ResultID = wrongX.ResultID
	wrongA.Counters = wrongX.Counters
	if e = ValidateResult(r, wrongA, wrongX); e == nil {
		t.Fatal("recomputed wrong key accepted")
	}
	if _, e = NewReviewRequest(r, wrongA, wrongX, "pw", "pt", "pr", "ra", "rw", "rt", "rr"); e == nil {
		t.Fatal("wrong key reached review")
	}
	_ = x
}
func TestValidateResultRejectsRedigestedForgery(t *testing.T) {
	r := fixture(t)
	a, x, _ := ingest(t, r)
	forged := cloneResult(x)
	forged.Ledger[0].Outcome = "INVALID_MEMBER"
	forged.OutcomeCounts["GROUPED"]--
	forged.OutcomeCounts["INVALID_MEMBER"]++
	forged.ResultID = ""
	forged.ResultID = domainID("group-result", forged)
	a.ResultID = forged.ResultID
	if e := ValidateResult(r, a, forged); e == nil {
		t.Fatal("redigested ledger forgery accepted")
	}
	forged = cloneResult(x)
	forged.Candidates[0].WorkingLabel = "Feature semantics"
	forged.ResultID = ""
	forged.ResultID = domainID("group-result", forged)
	a.ResultID = forged.ResultID
	if e := ValidateResult(r, a, forged); e == nil {
		t.Fatal("redigested candidate forgery accepted")
	}
}
func TestReviewRejectsFabricatedAllTrueRequest(t *testing.T) {
	r, a, x, q, _, _ := reviewed(t)
	forged := cloneResult(x)
	forged.Ledger[0].Outcome = "INVALID_MEMBER"
	forged.OutcomeCounts["GROUPED"]--
	forged.OutcomeCounts["INVALID_MEMBER"]++
	forged.ResultID = domainID("group-result", forged)
	q.ResultID, q.ResultDigest = forged.ResultID, Digest(canon(forged))
	q.AdapterChecks = allChecks(true)
	q.AdapterChecksDigest = Digest(canon(q.AdapterChecks))
	q.ReviewRequestID = domainID("review-request", q)
	k := ReviewerAttemptKey{r.GenerationID, q.ReviewerAssignmentID, q.ReviewRequestID, "fabricated-attempt"}
	b, _ := json.Marshal(ReviewerRaw{allChecks(true), map[string]string{}})
	raw := append(b, '\n')
	if _, _, e := NewReviewWriter().Ingest(r, a, forged, q, k, raw, Digest(raw)); e == nil || e.Error() != "REVIEW_REQUEST_BINDING" {
		t.Fatalf("fabricated all-true request accepted: %v", e)
	}
}
func TestReviewerWriterDeepCopyIsolation(t *testing.T) {
	r, a, x, q, _, _ := reviewed(t)
	_ = a
	_ = x
	k := ReviewerAttemptKey{r.GenerationID, "review-assignment", q.ReviewRequestID, "copy-attempt"}
	b, _ := json.Marshal(ReviewerRaw{allChecks(true), map[string]string{}})
	raw := append(b, '\n')
	w := NewReviewWriter()
	ra, rv, e := w.Ingest(r, a, x, q, k, raw, Digest(raw))
	if e != nil {
		t.Fatal(e)
	}
	ra.RawBytes[0] = 'x'
	ra.States[0] = "x"
	rv.Checks[ReviewChecks[0]] = false
	rv.AdapterChecks[ReviewChecks[0]] = false
	replay, replayRV, e := w.Ingest(r, a, x, q, k, raw, Digest(raw))
	if e != nil || replay.RawBytes[0] == 'x' || replay.States[0] == "x" || !replayRV.Checks[ReviewChecks[0]] || !replayRV.AdapterChecks[ReviewChecks[0]] {
		t.Fatal("shared mutable state")
	}
	conflict := append([]byte{}, raw...)
	conflict[len(conflict)-2] = ' '
	old, _, e := w.Ingest(r, a, x, q, k, conflict, Digest(conflict))
	if e == nil || old.ReviewID != replay.ReviewID || len(w.Conflicts()) != 1 {
		t.Fatal("conflict")
	}
}
func TestReviewerAttemptExactCustody(t *testing.T) {
	r, a, x, q, ra, rv := reviewed(t)
	counts := copyInt(x.OutcomeCounts)
	selection := AccountSelection{r.GenerationID, r.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
	mutations := map[string]func(*ReviewerAttempt){"generation": func(v *ReviewerAttempt) { v.Key.GenerationID += "x" }, "assignment": func(v *ReviewerAttempt) { v.Key.AssignmentID += "x" }, "request": func(v *ReviewerAttempt) { v.Key.ReviewRequestID += "x" }, "empty attempt": func(v *ReviewerAttempt) { v.Key.AttemptID = "" }}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			bad := cloneReviewerAttempt(ra)
			mutate(&bad)
			sel := selection
			sel.ReviewerAttemptKey = bad.Key
			if acct, e := MakeAccount(sel, r, a, x, q, bad, rv, counts); e == nil || acct.Custody.Percent == 100 {
				t.Fatalf("accepted: %v %+v", e, acct)
			}
		})
	}
	claims := allChecks(true)
	claims[ReviewChecks[0]] = false
	reasons := map[string]string{ReviewChecks[0]: "synthetic mismatch"}
	b, _ := json.Marshal(ReviewerRaw{claims, reasons})
	badRA := cloneReviewerAttempt(ra)
	badRA.RawBytes = append(b, '\n')
	badRA.RawDigest = Digest(badRA.RawBytes)
	badRV, e := deriveReview(q, ReviewerRaw{claims, reasons}, adapterChecks(r, a, x))
	if e != nil {
		t.Fatal(e)
	}
	badRA.ReviewID = badRV.ReviewID
	sel := selection
	sel.ReviewerAttemptKey = badRA.Key
	sel.ReviewID = badRV.ReviewID
	if acct, e := MakeAccount(sel, r, a, x, q, badRA, badRV, counts); e == nil || acct.Custody.Percent == 100 {
		t.Fatalf("redigested reviewer raw accepted: %v %+v", e, acct)
	}
	mismatch := cloneReview(rv)
	mismatch.Checks[ReviewChecks[0]] = false
	if acct, e := MakeAccount(selection, r, a, x, q, ra, mismatch, counts); e == nil || acct.Custody.Percent == 100 {
		t.Fatalf("raw/check mismatch accepted: %v %+v", e, acct)
	}
}
func TestMakeAccountComputedPercentagesAndMutation(t *testing.T) {
	r, a, x, q, ra, rv := reviewed(t)
	counts := copyInt(x.OutcomeCounts)
	sel := AccountSelection{r.GenerationID, r.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
	acct, e := MakeAccount(sel, r, a, x, q, ra, rv, counts)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range []PercentageBreakdown{acct.Mechanical, acct.Custody, acct.Accounting, acct.Evidence, acct.Ceiling} {
		if b.Percent != 100 || b.Numerator != b.Denominator {
			t.Fatal(b)
		}
	}
	badRV := cloneReview(rv)
	badRV.Checks[ReviewChecks[0]] = false
	if _, e := MakeAccount(sel, r, a, x, q, ra, badRV, counts); e == nil {
		t.Fatal("derived percentage mutation accepted")
	}
	badX := cloneResult(x)
	badX.Authority = 1
	if _, e := MakeAccount(sel, r, a, badX, q, ra, rv, counts); e == nil {
		t.Fatal("ceiling mutation accepted")
	}
	badSel := sel
	badSel.ResultID += "x"
	if _, e := MakeAccount(badSel, r, a, x, q, ra, rv, counts); e == nil {
		t.Fatal("selection mutation")
	}
}
func TestDeterminismAndNoOverlap(t *testing.T) {
	r := fixture(t)
	x, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := Adapt(r, "k", digest("raw"), Control{})
	if e != nil || !reflect.DeepEqual(x, y) {
		t.Fatal("nondeterministic")
	}
	seen := map[int]bool{}
	for _, c := range x.Candidates {
		if !strings.HasPrefix(c.WorkingLabel, "Provisional group ") || c.Authority != 0 || c.Accepted || c.FeatureIdentity != "UNRESOLVED" {
			t.Fatal(c)
		}
		for _, o := range c.MemberOrdinals {
			if seen[o] {
				t.Fatal("overlap")
			}
			seen[o] = true
		}
	}
}
