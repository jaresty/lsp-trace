// Package searchqualificationv11 defines a private, non-dispatching, harness-neutral
// prospective Search qualification contract. It never invokes a model or Search.
package searchqualificationv11

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	Version              = "lsp-trace.adr0007.search-qualification.private.v11"
	Generation           = "search-v11-design-generation-1"
	MemberCount          = 24
	DescribeRecords      = 48
	MaxQueryBytes        = 1024
	MaxDepth             = 1
	MaxFrontier          = 24
	MaxEvaluations       = 24
	WorkPerMember        = 10
	MaxWork              = 240
	MaxResults           = 5
	MaxResponseBytes     = 65536
	MaxCustodyIDBytes    = 256
	MaxReviewReasonBytes = 1024
	ScoreThreshold       = "0.600000"
	CustodyCeiling       = "PROCEDURAL_INDEPENDENCE_ONLY_NO_CRYPTO_ORG_AUTHORITY_OR_PRIVILEGED_ACTOR_PROTECTION"
)

var (
	HostDispositions  = [...]string{"RESULT", "REFUSE", "UNAVAILABLE", "CANCELLED", "TIMEOUT", "FAILED"}
	OperationOutcomes = [...]string{"COMPLETE", "INVALID_QUERY", "INDEX_UNAVAILABLE", "INDEX_MISMATCH", "CANCELLED", "TIMEOUT", "RESOURCE_LIMIT", "BACKEND_FAILURE", "POLICY_MISMATCH"}
	MemberOutcomes    = [...]string{"RETURNED", "BELOW_THRESHOLD", "FILTERED_BY_POLICY", "DUPLICATE_MEMBER", "INVALID_MEMBER"}
	AttemptStates     = [...]string{"RECEIVED", "ADAPTED", "COMMITTED", "TERMINAL_INVALID"}
	Precedence        = [...]string{"REQUEST", "ASSIGNMENT", "RAW_SIZE", "UTF8", "NDJSON_FRAME", "STRICT_JSON", "DIGEST", "CANCELLATION", "DEADLINE", "FRONTIER", "EVALUATION", "WORK_PRECHARGE", "RESULT_COUNT", "RESPONSE_BYTES"}
	ReviewChecks      = [...]string{"request_bound", "raw_bound", "result_bound", "pair_bound", "evidence_bound", "limits_bound", "semantic_complete", "ledger_balanced", "custody_procedural", "no_feature_identity"}
)

type Identity struct {
	RequestID, AttemptID, ReceiptID, RecordID string
	RawDigest, EvidenceDigest, SourceDigest   string
}

type MemberBinding struct {
	Ordinal        int
	MemberID       string
	Representation string
	Primary        Identity
	Replay         Identity
}

type DescribeBinding struct {
	GenerationID, DescribeDigest, PolicyDigest, EvaluationDigest string
	Members                                                      []MemberBinding
}

type Limits struct {
	QueryBytes, Depth, Frontier, Evaluations, WorkPrecharge, WorkMax, ResultCount, ResponseBytes int
	CancellationCheckpoints                                                                      []string
	DeadlineSemantics                                                                            string
}

func FixedLimits() Limits {
	return Limits{MaxQueryBytes, MaxDepth, MaxFrontier, MaxEvaluations, WorkPerMember, MaxWork, MaxResults, MaxResponseBytes, []string{"BEFORE_ADAPT", "BEFORE_MEMBER", "BEFORE_COMMIT"}, "deadline is expired when now >= deadline; cancellation precedes deadline at every checkpoint"}
}

type AdmissionState struct {
	RequestValid, AssignmentValid, DigestValid                   bool
	RawBytes, Frontier, Evaluations, Work, NextWork, ResultCount int
	UTF8, Frame, StrictJSON, Cancelled, DeadlineExpired          bool
}

// ClassifyAdmission applies only static ceilings and control state. It performs no
// Search inference or semantic qualification. Its order is the frozen precedence.
func ClassifyAdmission(s AdmissionState) string {
	if !s.RequestValid {
		return "REQUEST"
	}
	if !s.AssignmentValid {
		return "ASSIGNMENT"
	}
	if s.RawBytes > MaxResponseBytes {
		return "RAW_SIZE"
	}
	if !s.UTF8 {
		return "UTF8"
	}
	if !s.Frame {
		return "NDJSON_FRAME"
	}
	if !s.StrictJSON {
		return "STRICT_JSON"
	}
	if !s.DigestValid {
		return "DIGEST"
	}
	if s.Cancelled {
		return "CANCELLED"
	}
	if s.DeadlineExpired {
		return "DEADLINE"
	}
	if s.Frontier > MaxFrontier {
		return "FRONTIER"
	}
	if s.Evaluations > MaxEvaluations {
		return "EVALUATION"
	}
	if s.NextWork < 0 || s.Work > MaxWork-s.NextWork {
		return "WORK_PRECHARGE"
	}
	if s.ResultCount > MaxResults {
		return "RESULT_COUNT"
	}
	return "ADMIT"
}

type SearchRequest struct {
	SchemaVersion, RequestID, GenerationID, AttemptID, Query                   string
	OrderedMembers                                                             []string
	Denominator, IndexIdentity, DescribeDigest, PolicyDigest, EvaluationDigest string
	ProtocolIdentity, CeilingIdentity                                          string
	Limits                                                                     Limits
}

type MemberEvaluation struct{ MemberID, Score, Rationale string }
type HostResult struct {
	SchemaVersion, GenerationID, RequestID, AttemptID, AssignmentID, Disposition string
	Evaluations                                                                  []MemberEvaluation
}

type LedgerEntry struct {
	Ordinal                             int
	MemberID, Outcome, Score, Rationale string
	Selected                            bool
}
type SearchResult struct {
	SchemaVersion, ResultID, RequestID, GenerationID, AttemptKey             string
	Disposition, Outcome, Completeness                                       string
	Denominator, Begun, Completed, Unevaluated, WorkCharged                  int
	Ledger                                                                   []LedgerEntry
	Selected                                                                 []LedgerEntry
	RequestDigest, RawDigest, DescribeDigest, PolicyDigest, EvaluationDigest string
	Authority                                                                int
	Accepted                                                                 bool
	FeatureIdentity                                                          string
}

type Attempt struct {
	Key, GenerationID, RequestID, AttemptID, AssignmentID, RawDigest string
	RawBytes                                                         []byte
	States                                                           []string
	TerminalReason                                                   string
	ResultID                                                         string
}

type ParentCustodyEvidence struct {
	SchemaVersion                                                        string
	GenerationID                                                         string
	ProducerAssignmentID, ProducerWorkerID, ProducerTaskID, ProducerRole string
	ReviewerAssignmentID, ReviewerWorkerID, ReviewerTaskID, ReviewerRole string
	OutputSelector, OutputDigest                                         string
	OutputLength                                                         int
	EventSelector, EventDigest                                           string
	EventLength                                                          int
}

type ReviewRequest struct {
	SchemaVersion, ReviewRequestID, GenerationID                string
	RequestID, RequestDigest                                    string
	ProducerAttemptKey, ProducerAttemptID, ProducerAssignmentID string
	RawDigest, ResultID, ResultDigest                           string
	Evidence                                                    ParentCustodyEvidence
	PairDigest, EvidenceDigest, LimitsDigest, CustodyCeiling    string
}

type ReviewerRaw struct {
	Checks  map[string]bool   `json:"checks"`
	Reasons map[string]string `json:"reasons"`
}
type Review struct {
	ReviewID, ReviewRequestID, Verdict string
	Checks                             map[string]bool
	Reasons                            map[string]string
}
type ReviewerAttemptKey struct{ GenerationID, AssignmentID, ReviewRequestID, AttemptID string }
type ReviewerAttempt struct {
	Key                      ReviewerAttemptKey
	RawDigest                string
	RawBytes                 []byte
	States                   []string
	TerminalReason, ReviewID string
}
type ReviewDeliveryConflict struct {
	ConflictID                     string
	Key                            ReviewerAttemptKey
	PriorDigest, ConflictingDigest string
}
type AccountSelection struct {
	GenerationID, RequestID, ProducerAttemptKey, ResultID string
	ReviewerAttemptKey                                    ReviewerAttemptKey
	ReviewID                                              string
}
type Account struct {
	Selection                                                                   AccountSelection
	OutcomeCounts                                                               map[string]int
	Balanced                                                                    bool
	MechanicalPercent, CustodyPercent, AccountingPercent, CriticalReviewPercent int
	SemanticUsefulnessQualified                                                 bool
	FeatureIdentity                                                             string
}

func Digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func canon(v any) []byte     { b, _ := json.Marshal(v); return b }
func id(prefix string, v any) string {
	return prefix + "-" + strings.TrimPrefix(Digest(canon(v)), "sha256:")
}
func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil && strings.ToLower(s) == s
}
func validScore(s string) bool {
	if len(s) != 8 || s[1] != '.' {
		return false
	}
	if s[0] < '0' || s[0] > '1' {
		return false
	}
	if s[0] == '1' && s[2:] != "000000" {
		return false
	}
	for _, r := range s[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func score(s string) int { n, _ := strconv.Atoi(s[:1] + s[2:]); return n }
func memberSet(r SearchRequest) map[string]int {
	m := map[string]int{}
	for i, x := range r.OrderedMembers {
		m[x] = i + 1
	}
	return m
}

func ValidateDescribe(d DescribeBinding) error {
	if d.GenerationID == "" || !validDigest(d.DescribeDigest) || !validDigest(d.PolicyDigest) || !validDigest(d.EvaluationDigest) || len(d.Members) != MemberCount {
		return errors.New("DESCRIBE_BINDING")
	}
	seenM, seenIDs := map[string]bool{}, map[string]bool{}
	for i, m := range d.Members {
		if m.Ordinal != i+1 || m.MemberID == "" || seenM[m.MemberID] || m.Representation == "" {
			return errors.New("DESCRIBE_MEMBER")
		}
		seenM[m.MemberID] = true
		for _, x := range []Identity{m.Primary, m.Replay} {
			vals := []string{x.RequestID, x.AttemptID, x.ReceiptID, x.RecordID, x.RawDigest, x.EvidenceDigest, x.SourceDigest}
			for j, v := range vals {
				if v == "" || seenIDs[v] || (j >= 4 && !validDigest(v)) {
					return errors.New("DESCRIBE_IDENTITY")
				}
				seenIDs[v] = true
			}
		}
	}
	if len(seenIDs) != DescribeRecords*7 {
		return errors.New("DESCRIBE_EXACT_48")
	}
	return nil
}

func Prepare(d DescribeBinding, query, indexIdentity string) (SearchRequest, error) {
	if err := ValidateDescribe(d); err != nil {
		return SearchRequest{}, err
	}
	if query == "" || !utf8.ValidString(query) || len([]byte(query)) > MaxQueryBytes {
		return SearchRequest{}, errors.New("INVALID_QUERY")
	}
	if !validDigest(indexIdentity) {
		return SearchRequest{}, errors.New("INDEX_IDENTITY")
	}
	r := SearchRequest{SchemaVersion: Version + ".request", GenerationID: d.GenerationID, Query: query, Denominator: fmt.Sprint(MemberCount), IndexIdentity: indexIdentity, DescribeDigest: d.DescribeDigest, PolicyDigest: d.PolicyDigest, EvaluationDigest: d.EvaluationDigest, ProtocolIdentity: Version, CeilingIdentity: CustodyCeiling, Limits: FixedLimits()}
	for _, m := range d.Members {
		r.OrderedMembers = append(r.OrderedMembers, m.MemberID)
	}
	r.RequestID = id("search-request", r)
	r.AttemptID = id("search-attempt", struct{ GenerationID, RequestID string }{r.GenerationID, r.RequestID})
	return r, nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return errors.New("STRICT_JSON")
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return errors.New("STRICT_JSON")
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] {
					return errors.New("DUPLICATE_KEY")
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("STRICT_JSON")
			}
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("STRICT_JSON")
			}
		default:
			return errors.New("STRICT_JSON")
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("TRAILING")
	}
	return nil
}

func strictObject(line []byte, dst any) error {
	if err := rejectDuplicateJSONKeys(line); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return errors.New("STRICT_JSON")
	}
	seen := map[string]bool{}
	for dec.More() {
		t, e := dec.Token()
		if e != nil {
			return errors.New("STRICT_JSON")
		}
		k, ok := t.(string)
		if !ok || seen[k] {
			return errors.New("DUPLICATE_KEY")
		}
		seen[k] = true
		var raw json.RawMessage
		if e = dec.Decode(&raw); e != nil {
			return errors.New("STRICT_JSON")
		}
	}
	if _, err = dec.Token(); err != nil {
		return errors.New("STRICT_JSON")
	}
	if _, err = dec.Token(); err != io.EOF {
		return errors.New("TRAILING")
	}
	dec = json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err = dec.Decode(dst); err != nil {
		return fmt.Errorf("STRICT_JSON: %w", err)
	}
	if !bytes.Equal(line, canon(dst)) {
		return errors.New("NONCANONICAL")
	}
	return nil
}

func ParseHostResult(raw []byte, req SearchRequest, assignmentID, expectedDigest string) (HostResult, error) {
	if len(raw) > MaxResponseBytes {
		return HostResult{}, errors.New("RAW_SIZE")
	}
	if !utf8.Valid(raw) {
		return HostResult{}, errors.New("UTF8")
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return HostResult{}, errors.New("PARTIAL_FRAME")
	}
	if bytes.Count(raw, []byte{'\n'}) != 1 {
		return HostResult{}, errors.New("TRAILING")
	}
	line := raw[:len(raw)-1]
	if expectedDigest != "" && Digest(raw) != expectedDigest {
		return HostResult{}, errors.New("DIGEST")
	}
	var h HostResult
	if err := strictObject(line, &h); err != nil {
		return HostResult{}, err
	}
	if h.SchemaVersion != Version+".host-result" || h.GenerationID != req.GenerationID || h.RequestID != req.RequestID || h.AttemptID != req.AttemptID || h.AssignmentID != assignmentID {
		return HostResult{}, errors.New("REQUEST_ASSIGNMENT")
	}
	ok := false
	for _, d := range HostDispositions {
		ok = ok || h.Disposition == d
	}
	if !ok {
		return HostResult{}, errors.New("DISPOSITION")
	}
	members, seen := memberSet(req), map[string]bool{}
	for _, e := range h.Evaluations {
		if members[e.MemberID] == 0 {
			return HostResult{}, errors.New("UNKNOWN_MEMBER")
		}
		if seen[e.MemberID] {
			return HostResult{}, errors.New("DUPLICATE_MEMBER")
		}
		seen[e.MemberID] = true
		if !validScore(e.Score) || strings.TrimSpace(e.Rationale) == "" {
			return HostResult{}, errors.New("EVALUATION")
		}
	}
	if h.Disposition == "RESULT" && len(h.Evaluations) != MemberCount {
		return HostResult{}, errors.New("INCOMPLETE_RESULT")
	}
	return h, nil
}

func Adapt(req SearchRequest, attemptKey, rawDigest string, h HostResult) SearchResult {
	members := memberSet(req)
	evals := append([]MemberEvaluation(nil), h.Evaluations...)
	sort.Slice(evals, func(i, j int) bool { return members[evals[i].MemberID] < members[evals[j].MemberID] })
	r := SearchResult{SchemaVersion: Version + ".result", RequestID: req.RequestID, GenerationID: req.GenerationID, AttemptKey: attemptKey, Disposition: h.Disposition, Denominator: MemberCount, Begun: len(evals), Completed: len(evals), Unevaluated: MemberCount - len(evals), WorkCharged: len(evals) * WorkPerMember, RequestDigest: Digest(canon(req)), RawDigest: rawDigest, DescribeDigest: req.DescribeDigest, PolicyDigest: req.PolicyDigest, EvaluationDigest: req.EvaluationDigest, Authority: 0, Accepted: false, FeatureIdentity: "UNRESOLVED"}
	for _, e := range evals {
		r.Ledger = append(r.Ledger, LedgerEntry{members[e.MemberID], e.MemberID, "BELOW_THRESHOLD", e.Score, e.Rationale, false})
	}
	candidates := append([]LedgerEntry(nil), r.Ledger...)
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := score(candidates[i].Score), score(candidates[j].Score)
		if a != b {
			return a > b
		}
		return candidates[i].Ordinal < candidates[j].Ordinal
	})
	for _, e := range candidates {
		if score(e.Score) >= score(ScoreThreshold) && len(r.Selected) < MaxResults {
			e.Outcome, e.Selected = "RETURNED", true
			r.Selected = append(r.Selected, e)
			for i := range r.Ledger {
				if r.Ledger[i].MemberID == e.MemberID {
					r.Ledger[i].Outcome, r.Ledger[i].Selected = "RETURNED", true
				}
			}
		}
	}
	r.Completeness = "INCOMPLETE"
	if h.Disposition == "RESULT" && len(evals) == MemberCount && r.WorkCharged <= MaxWork {
		r.Completeness = "COMPLETE"
	}
	r.Outcome = map[string]string{"RESULT": "COMPLETE", "REFUSE": "POLICY_MISMATCH", "UNAVAILABLE": "INDEX_UNAVAILABLE", "CANCELLED": "CANCELLED", "TIMEOUT": "TIMEOUT", "FAILED": "BACKEND_FAILURE"}[h.Disposition]
	r.ResultID = id("search-result", r)
	return r
}

type DeliveryConflict struct{ DeliveryID, AttemptKey, PriorDigest, ConflictingDigest string }
type Writer struct {
	mu        sync.Mutex
	attempts  map[string]Attempt
	results   map[string]SearchResult
	conflicts []DeliveryConflict
}

func NewWriter() *Writer {
	return &Writer{attempts: map[string]Attempt{}, results: map[string]SearchResult{}}
}
func (w *Writer) Ingest(req SearchRequest, assignment string, raw []byte, expectedDigest string) (Attempt, SearchResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := req.GenerationID + ":" + req.RequestID + ":" + req.AttemptID + ":" + assignment
	digest := Digest(raw)
	if old, ok := w.attempts[key]; ok {
		if old.RawDigest == digest {
			return old, w.results[old.ResultID], nil
		}
		w.conflicts = append(w.conflicts, DeliveryConflict{id("delivery-conflict", struct{ Key, Prior, Conflict string }{key, old.RawDigest, digest}), key, old.RawDigest, digest})
		return old, SearchResult{}, errors.New("CONFLICTING_DELIVERY")
	}
	a := Attempt{Key: key, GenerationID: req.GenerationID, RequestID: req.RequestID, AttemptID: req.AttemptID, AssignmentID: assignment, RawDigest: digest, RawBytes: append([]byte(nil), raw...), States: []string{"RECEIVED"}}
	h, err := ParseHostResult(raw, req, assignment, expectedDigest)
	if err != nil {
		a.States = append(a.States, "TERMINAL_INVALID")
		a.TerminalReason = err.Error()
		w.attempts[key] = a
		return a, SearchResult{}, err
	}
	a.States = append(a.States, "ADAPTED")
	r := Adapt(req, key, digest, h)
	a.States = append(a.States, "COMMITTED")
	a.ResultID = r.ResultID
	w.attempts[key] = a
	w.results[r.ResultID] = r
	return a, r, nil
}

func validCustodyID(s string) bool {
	return s != "" && len(s) <= MaxCustodyIDBytes && utf8.ValidString(s) && strings.TrimSpace(s) == s
}
func ValidateParentCustodyEvidence(e ParentCustodyEvidence) error {
	ids := []string{e.GenerationID, e.ProducerAssignmentID, e.ProducerWorkerID, e.ProducerTaskID, e.ProducerRole, e.ReviewerAssignmentID, e.ReviewerWorkerID, e.ReviewerTaskID, e.ReviewerRole, e.OutputSelector, e.EventSelector}
	for _, value := range ids {
		if !validCustodyID(value) {
			return errors.New("CUSTODY_ID")
		}
	}
	if e.SchemaVersion != Version+".parent-custody-evidence" || !validDigest(e.OutputDigest) || e.OutputLength < 0 || !validDigest(e.EventDigest) || e.EventLength < 0 || e.ProducerAssignmentID == e.ReviewerAssignmentID || e.ProducerWorkerID == e.ReviewerWorkerID || e.ProducerTaskID == e.ReviewerTaskID || e.ProducerRole == e.ReviewerRole {
		return errors.New("CUSTODY_BINDING")
	}
	return nil
}
func NewReviewRequest(req SearchRequest, a Attempt, r SearchResult, e ParentCustodyEvidence) (ReviewRequest, error) {
	if ValidateParentCustodyEvidence(e) != nil || e.GenerationID != req.GenerationID || e.ProducerAssignmentID != a.AssignmentID || a.GenerationID != req.GenerationID || a.RequestID != req.RequestID || a.AttemptID != req.AttemptID || a.ResultID != r.ResultID || r.GenerationID != req.GenerationID || r.RequestID != req.RequestID || r.AttemptKey != a.Key || a.RawDigest != r.RawDigest || e.OutputDigest != Digest(canon(r)) || e.OutputLength != len(canon(r)) || e.EventDigest != a.RawDigest || e.EventLength != len(a.RawBytes) {
		return ReviewRequest{}, errors.New("REVIEW_BINDING")
	}
	pair := struct{ ProducerAssignmentID, ProducerWorkerID, ReviewerAssignmentID, ReviewerWorkerID string }{e.ProducerAssignmentID, e.ProducerWorkerID, e.ReviewerAssignmentID, e.ReviewerWorkerID}
	x := ReviewRequest{SchemaVersion: Version + ".review-request", GenerationID: req.GenerationID, RequestID: req.RequestID, RequestDigest: Digest(canon(req)), ProducerAttemptKey: a.Key, ProducerAttemptID: a.AttemptID, ProducerAssignmentID: a.AssignmentID, RawDigest: a.RawDigest, ResultID: r.ResultID, ResultDigest: Digest(canon(r)), Evidence: e, PairDigest: Digest(canon(pair)), EvidenceDigest: Digest(canon(e)), LimitsDigest: Digest(canon(req.Limits)), CustodyCeiling: CustodyCeiling}
	x.ReviewRequestID = id("review-request", x)
	return x, nil
}
func ParseReviewerRaw(raw []byte) (ReviewerRaw, error) {
	if len(raw) > MaxResponseBytes {
		return ReviewerRaw{}, errors.New("RAW_SIZE")
	}
	if !utf8.Valid(raw) {
		return ReviewerRaw{}, errors.New("UTF8")
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return ReviewerRaw{}, errors.New("PARTIAL_FRAME")
	}
	if bytes.Count(raw, []byte{'\n'}) != 1 {
		return ReviewerRaw{}, errors.New("TRAILING")
	}
	var h ReviewerRaw
	if err := strictObject(raw[:len(raw)-1], &h); err != nil {
		return ReviewerRaw{}, err
	}
	if err := validateReviewerRaw(h); err != nil {
		return ReviewerRaw{}, err
	}
	return h, nil
}
func validateReviewerRaw(h ReviewerRaw) error {
	if len(h.Checks) != len(ReviewChecks) {
		return errors.New("REVIEW_CHECKS")
	}
	known := map[string]bool{}
	for _, k := range ReviewChecks {
		known[k] = true
		if _, ok := h.Checks[k]; !ok {
			return errors.New("REVIEW_CHECKS")
		}
	}
	for k := range h.Checks {
		if !known[k] {
			return errors.New("REVIEW_CHECKS")
		}
	}
	for k, reason := range h.Reasons {
		if !known[k] || h.Checks[k] || strings.TrimSpace(reason) == "" || len([]byte(reason)) > MaxReviewReasonBytes {
			return errors.New("REVIEW_REASONS")
		}
	}
	for k, value := range h.Checks {
		if !value {
			if _, ok := h.Reasons[k]; !ok {
				return errors.New("REVIEW_REASON")
			}
		}
	}
	return nil
}
func DeriveReview(q ReviewRequest, h ReviewerRaw) (Review, error) {
	if err := validateReviewerRaw(h); err != nil {
		return Review{}, err
	}
	verdict := "ACCEPT"
	for _, k := range ReviewChecks {
		if !h.Checks[k] {
			verdict = "REJECT"
		}
	}
	if verdict == "ACCEPT" && len(h.Reasons) != 0 {
		return Review{}, errors.New("FALSE_ACCEPT")
	}
	r := Review{ReviewRequestID: q.ReviewRequestID, Verdict: verdict, Checks: h.Checks, Reasons: h.Reasons}
	r.ReviewID = id("review", struct {
		RequestID, Verdict string
		Checks             map[string]bool
		Reasons            map[string]string
	}{q.ReviewRequestID, verdict, h.Checks, h.Reasons})
	return r, nil
}

type ReviewWriter struct {
	mu        sync.Mutex
	attempts  map[ReviewerAttemptKey]ReviewerAttempt
	reviews   map[string]Review
	conflicts []ReviewDeliveryConflict
}

func NewReviewWriter() *ReviewWriter {
	return &ReviewWriter{attempts: map[ReviewerAttemptKey]ReviewerAttempt{}, reviews: map[string]Review{}}
}
func NewReviewerAttemptKey(q ReviewRequest, assignmentID, attemptID string) (ReviewerAttemptKey, error) {
	k := ReviewerAttemptKey{q.GenerationID, assignmentID, q.ReviewRequestID, attemptID}
	if !validCustodyID(assignmentID) || !validCustodyID(attemptID) || assignmentID != q.Evidence.ReviewerAssignmentID {
		return ReviewerAttemptKey{}, errors.New("REVIEWER_ASSIGNMENT")
	}
	return k, nil
}
func (w *ReviewWriter) Ingest(q ReviewRequest, key ReviewerAttemptKey, raw []byte, expectedDigest string) (ReviewerAttempt, Review, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if key.GenerationID != q.GenerationID || key.ReviewRequestID != q.ReviewRequestID || key.AssignmentID != q.Evidence.ReviewerAssignmentID || !validCustodyID(key.AttemptID) {
		return ReviewerAttempt{}, Review{}, errors.New("REVIEWER_KEY")
	}
	for k := range w.attempts {
		if k.GenerationID == key.GenerationID && k.AssignmentID == key.AssignmentID && k.ReviewRequestID == key.ReviewRequestID && k.AttemptID != key.AttemptID {
			return ReviewerAttempt{}, Review{}, errors.New("SECOND_REVIEW_ATTEMPT")
		}
	}
	digest := Digest(raw)
	if old, ok := w.attempts[key]; ok {
		if old.RawDigest == digest {
			return old, w.reviews[old.ReviewID], nil
		}
		w.conflicts = append(w.conflicts, ReviewDeliveryConflict{id("review-conflict", struct {
			Key             ReviewerAttemptKey
			Prior, Conflict string
		}{key, old.RawDigest, digest}), key, old.RawDigest, digest})
		return old, Review{}, errors.New("CONFLICTING_REVIEW_DELIVERY")
	}
	a := ReviewerAttempt{Key: key, RawDigest: digest, RawBytes: append([]byte(nil), raw...), States: []string{"RECEIVED"}}
	if expectedDigest != "" && digest != expectedDigest {
		a.States = append(a.States, "TERMINAL_INVALID")
		a.TerminalReason = "DIGEST"
		w.attempts[key] = a
		return a, Review{}, errors.New("DIGEST")
	}
	h, err := ParseReviewerRaw(raw)
	if err != nil {
		a.States = append(a.States, "TERMINAL_INVALID")
		a.TerminalReason = err.Error()
		w.attempts[key] = a
		return a, Review{}, err
	}
	a.States = append(a.States, "ADAPTED")
	r, err := DeriveReview(q, h)
	if err != nil {
		a.States = []string{"RECEIVED", "TERMINAL_INVALID"}
		a.TerminalReason = err.Error()
		w.attempts[key] = a
		return a, Review{}, err
	}
	a.States = append(a.States, "COMMITTED")
	a.ReviewID = r.ReviewID
	w.attempts[key] = a
	w.reviews[r.ReviewID] = r
	return a, r, nil
}
func exactOutcomeCounts(r SearchResult, supplied map[string]int) error {
	if len(supplied) != len(MemberOutcomes) {
		return errors.New("OUTCOME_COUNTS")
	}
	actual := map[string]int{}
	for _, k := range MemberOutcomes {
		actual[k] = 0
		if _, ok := supplied[k]; !ok {
			return errors.New("OUTCOME_COUNTS")
		}
	}
	for _, e := range r.Ledger {
		if _, ok := actual[e.Outcome]; !ok {
			return errors.New("OUTCOME_UNKNOWN")
		}
		actual[e.Outcome]++
	}
	total := 0
	for k, n := range supplied {
		if n < 0 || actual[k] != n {
			return errors.New("OUTCOME_MISMATCH")
		}
		total += n
	}
	if total != r.Completed {
		return errors.New("OUTCOME_COMPLETED")
	}
	return nil
}
func MakeAccount(sel AccountSelection, req SearchRequest, a Attempt, r SearchResult, q ReviewRequest, ra ReviewerAttempt, rv Review, counts map[string]int) (Account, error) {
	pair := struct{ ProducerAssignmentID, ProducerWorkerID, ReviewerAssignmentID, ReviewerWorkerID string }{q.Evidence.ProducerAssignmentID, q.Evidence.ProducerWorkerID, q.Evidence.ReviewerAssignmentID, q.Evidence.ReviewerWorkerID}
	if sel.GenerationID != req.GenerationID || sel.RequestID != req.RequestID || sel.ProducerAttemptKey != a.Key || sel.ResultID != r.ResultID || sel.ReviewerAttemptKey != ra.Key || sel.ReviewID != rv.ReviewID || a.GenerationID != req.GenerationID || r.GenerationID != req.GenerationID || q.GenerationID != req.GenerationID || ra.Key.GenerationID != req.GenerationID || a.ResultID != r.ResultID || q.RequestID != req.RequestID || q.RequestDigest != Digest(canon(req)) || q.ProducerAttemptKey != a.Key || q.ProducerAttemptID != a.AttemptID || q.ProducerAssignmentID != a.AssignmentID || q.RawDigest != a.RawDigest || q.ResultID != r.ResultID || q.ResultDigest != Digest(canon(r)) || q.PairDigest != Digest(canon(pair)) || q.EvidenceDigest != Digest(canon(q.Evidence)) || q.LimitsDigest != Digest(canon(req.Limits)) || q.CustodyCeiling != CustodyCeiling || q.ReviewRequestID != ra.Key.ReviewRequestID || q.ReviewRequestID != rv.ReviewRequestID || fmt.Sprint(a.States) != "[RECEIVED ADAPTED COMMITTED]" || fmt.Sprint(ra.States) != "[RECEIVED ADAPTED COMMITTED]" || a.ResultID == "" || ra.ReviewID != rv.ReviewID || rv.Verdict != "ACCEPT" {
		return Account{}, errors.New("ACCOUNT_KEYS")
	}
	balanced := r.Denominator == r.Completed+r.Unevaluated && r.Completed == len(r.Ledger) && r.WorkCharged == r.Completed*WorkPerMember
	if !balanced {
		return Account{}, errors.New("ACCOUNT_UNBALANCED")
	}
	if err := exactOutcomeCounts(r, counts); err != nil {
		return Account{}, err
	}
	copyCounts := map[string]int{}
	for k, n := range counts {
		copyCounts[k] = n
	}
	return Account{sel, copyCounts, true, 100, 100, 100, 100, false, "UNRESOLVED"}, nil
}
