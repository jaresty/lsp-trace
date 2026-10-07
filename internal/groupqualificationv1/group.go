// Package groupqualificationv1 implements the private, non-dispatching ADR 0007
// Group design contract. It adapts retained structural evidence and never runs
// Leiden, assigns feature semantics, or raises authority.
package groupqualificationv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	Version                   = "lsp-trace.adr0007.group.private.v1"
	SearchSchemaVersion       = "lsp-trace.adr0007.search-qualification.private.v11.account"
	SearchFreeze              = "sha256:b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301"
	SearchExecutionManifest   = "sha256:bda8404feca6bad41b971dc8b951a8f19352002eed5a9e7f784d6af7c31236bf"
	SearchFinalAudit          = "sha256:9bb9c9d3b5dfac0ad780a23fa6e7d3810a2516ab68f91e2b2eb61b58605d5b24"
	SearchFinalSeal           = "sha256:d43305ff9e0a57ac3613bf5f2d988f0c31b82b831345d2a8cbe9d4a197eadce2"
	SearchTerminalReport      = "sha256:cefa9168920ba4da91bc8756cb1186efee28c9f47052249b1fede27b8d4c58cd"
	SearchCase01Request       = "sha256:3dd64f018b72600c0a4bd7caaed59f079ec0f1bb62401ea3e024f39850d7494d"
	SearchCase01Result        = "sha256:2ddf53ed7b7dbc7d228f96f7949a8110d5e77f6b26f4138197fa6c2d6b3ff9b6"
	SearchCase01Producer      = "sha256:1ee199562b2390fd720075ef8756ab28fa1109cf5ddaea43b063d18b69570aa6"
	SearchCase01ReviewRequest = "sha256:debf6776e99003122f567dbddb194562c9b5bd0673591787e106e7d27bc25aff"
	SearchCase01ReviewAttempt = "sha256:40738b784d62395f3f11370a3364662531c4b65e915cde17174e12fe47442472"
	SearchCase01Account       = "sha256:27eee48bf832ba8172c8ce25698eb24457a286ff700fc4bf9c92d6049de1dbfd"
	SearchCase01Custody       = "sha256:36286111ea77812b7d3fa37d41830ad30d0eebf9f480c6c31362001c40b7cc6e"
	GroupPolicyDigest         = "sha256:4304452d1109e1021c05427552460978afcc60045ad52ff5116c4b4a1b52beec"
	GroupLimitsDigest         = "sha256:4da1b9054a687296a7c17648f03a83577f4824bc3078be1fcc29a73d438e38e3"
	MemberCount               = 24
	MaxRequestBytes           = 262144
	MaxResponseBytes          = 65536
	MaxIdentityBytes          = 256
	MaxReviewReasonBytes      = 1024
	MaxRelations              = 512
	MaxCandidates             = 24
	MaxMembersPerCandidate    = 24
	MaxCandidateOccurrences   = 24
	MaxEvidenceRefsPerMember  = 32
	MaxSourceObjects          = 256
	MaxSourceRanges           = 256
	MaxUniqueSourceBytes      = 2097152
	MaxWork                   = 4096
	WorkImport                = 1
	WorkMember                = 1
	WorkRelation              = 1
	WorkCandidate             = 1
	WorkCommit                = 1
)

var (
	OperationOutcomes = [...]string{"COMPLETE", "INDEX_UNAVAILABLE", "INDEX_MISMATCH", "CANCELLED", "TIMEOUT", "RESOURCE_LIMIT", "BACKEND_FAILURE", "POLICY_MISMATCH"}
	MemberOutcomes    = [...]string{"GROUPED", "UNMATCHED", "FILTERED_BY_POLICY", "DUPLICATE_MEMBER", "INVALID_MEMBER"}
	SearchOutcomes    = [...]string{"RETURNED", "BELOW_THRESHOLD", "FILTERED_BY_POLICY", "DUPLICATE_MEMBER", "INVALID_MEMBER"}
	AttemptStates     = [...]string{"RECEIVED", "ADAPTED", "COMMITTED", "TERMINAL_INVALID"}
	Checkpoints       = [...]string{"BEFORE_IMPORT", "BEFORE_MEMBER", "BEFORE_RELATION", "BEFORE_CANDIDATE", "BEFORE_COMMIT"}
	ReviewChecks      = [...]string{"request_bound", "raw_bound", "result_bound", "search_bound", "evidence_bound", "partition_balanced", "candidate_nonsemantic", "authority_ceiling", "custody_procedural"}
	CauseCodes        = [...]string{"MISSING_SEARCH", "INVALID_SEARCH", "SEARCH_FREEZE", "SEARCH_SCHEMA", "SEARCH_UNBALANCED", "SEARCH_OUTCOMES", "SEARCH_DIGEST", "SEARCH_IDENTITY", "SEARCH_ATTEMPT", "EVIDENCE_DIGEST", "INVALID_SOURCE", "INVALID_CALLS", "INVALID_COMMUNITY", "DUPLICATE_ORDINAL", "DUPLICATE_MEMBER", "INVALID_MEMBER", "POLICY", "CANCELLATION", "DEADLINE_EXPIRED", "WORK_PRECHARGE", "BACKEND", "RAW_SIZE", "UTF8", "STRICT_JSON", "DIGEST", "ASSIGNMENT", "SECOND_ATTEMPT", "RELATION_LIMIT", "CANDIDATE_LIMIT", "OCCURRENCE_LIMIT", "EVIDENCE_LIMIT", "SOURCE_OBJECT_LIMIT", "SOURCE_RANGE_LIMIT", "SOURCE_BYTES_LIMIT", "RESPONSE_LIMIT"}
)

type SearchMember struct {
	Ordinal           int
	MemberID, Outcome string
	Authority         int
	Accepted          bool
	FeatureIdentity   string
}

type SearchAccount struct {
	SchemaVersion, FreezeIdentity                                                      string
	RequestID, RequestDigest, ResultID, ResultDigest, ReviewID, ReviewDigest           string
	ProducerAttemptID, ProducerAttemptDigest, ReviewerAttemptID, ReviewerAttemptDigest string
	ReviewRequestDigest, AccountDigest, CustodyDigest                                  string
	ExecutionManifestDigest, FinalAuditDigest, FinalSealDigest, TerminalReportDigest   string
	ProducerStates, ReviewerStates                                                     []string
	Denominator                                                                        int
	Completeness, ReviewerVerdict                                                      string
	Balanced                                                                           bool
	OutcomeCounts                                                                      map[string]int
	Authority                                                                          int
	Accepted                                                                           bool
	FeatureIdentity                                                                    string
	Members                                                                            []SearchMember
}

type SourceObject struct {
	ObjectID, Digest, SourceDigest, CaptureID string
	Complete                                  bool
	Authority                                 int
	RangeCount, UniqueBytes                   int
}
type SourceRef struct {
	Digest, CaptureID, ObjectID, ObjectDigest string
	Complete                                  bool
	Authority                                 int
	RangeCount, UniqueBytes                   int
	Objects                                   []SourceObject
}
type RelationRef struct {
	Digest, CaptureID, FromMemberID, ToMemberID, Kind, Provenance string
	ServerReported                                                bool
}
type CommunityRef struct {
	Digest, CaptureID, ObservationID, CommunityID, MemberID string
	Algorithm                                               string
	StructuralOnly, Retained, Nominated                     bool
	Seed                                                    int64
	Resolution                                              string
}
type MemberEvidence struct {
	Ordinal   int
	MemberID  string
	Filtered  bool
	Source    *SourceRef
	Calls     []RelationRef
	Community *CommunityRef
}
type Limits struct {
	MaxRequestBytes, MaxResponseBytes, MaxIdentityBytes, MaxMembers, MaxRelations              int
	MaxCandidates, MaxMembersPerCandidate, MaxCandidateOccurrences                             int
	MaxEvidenceRefsPerMember, MaxSourceObjects, MaxSourceRanges, MaxUniqueSourceBytes, MaxWork int
}

func FixedLimits() Limits {
	return Limits{262144, 65536, 256, 24, 512, 24, 24, 24, 32, 256, 256, 2097152, 4096}
}

type GroupRequest struct {
	SchemaVersion, RequestID, GenerationID, AttemptID, AssignmentID string
	SearchFreezeIdentity                                            string
	Search                                                          SearchAccount
	OrderedMembers                                                  []SearchMember
	PolicyDigest, LimitsDigest, EvidenceDigest                      string
	PartitionCaptureID, PartitionIdentity                           string
	PartitionSeed                                                   int64
	PartitionResolution                                             string
	Authority                                                       int
	Accepted                                                        bool
	Completeness, FeatureIdentity                                   string
	Limits                                                          Limits
	Evidence                                                        []MemberEvidence
}
type Candidate struct {
	CandidateID, WorkingLabel, CommunityID string
	MemberOrdinals                         []int
	MemberIDs                              []string
	Authority                              int
	Accepted                               bool
	FeatureIdentity                        string
}
type LedgerEntry struct {
	Ordinal                                   int
	MemberID, Outcome, CauseCode, CandidateID string
}
type Counters struct{ Begun, Completed, Relations, Candidates, WorkCharged int }
type Result struct {
	SchemaVersion, ResultID, RequestID, GenerationID, AttemptKey, Outcome, CauseCode           string
	Completeness                                                                               string
	Denominator                                                                                int
	Ledger                                                                                     []LedgerEntry
	Candidates                                                                                 []Candidate
	OutcomeCounts                                                                              map[string]int
	Counters                                                                                   Counters
	RequestDigest, RawDigest, PolicyDigest, LimitsDigest, EvidenceDigest, SearchFreezeIdentity string
	Authority                                                                                  int
	Accepted                                                                                   bool
	FeatureIdentity                                                                            string
}

// Control provides deterministic checkpoint injection. TriggerOccurrence is
// one-based among checkpoints with TriggerCheckpoint's name; zero means one.
type Control struct {
	Cancelled, DeadlineExpired, BackendFailed                      bool
	WorkLimit                                                      int
	TriggerCheckpoint                                              string
	TriggerOccurrence                                              int
	TriggerCancelled, TriggerDeadlineExpired, TriggerBackendFailed bool
	TriggerWorkLimit                                               int
}
type ProducerRaw struct {
	Evidence []MemberEvidence `json:"evidence"`
}
type Attempt struct {
	Key, GenerationID, RequestID, AttemptID, AssignmentID, RawDigest string
	RawBytes                                                         []byte
	States                                                           []string
	TerminalReason, ResultID                                         string
	Counters                                                         Counters
}
type DeliveryConflict struct{ ConflictID, AttemptKey, PriorDigest, ConflictingDigest string }

type ReviewRequest struct {
	SchemaVersion, ReviewRequestID, GenerationID, RequestID, RequestDigest                      string
	ProducerAttemptKey, ProducerAttemptID, ProducerAssignmentID, ProducerAttemptDigest          string
	RawDigest, ResultID, ResultDigest, SearchDigest, EvidenceDigest, PolicyDigest, LimitsDigest string
	ProducerWorkerID, ProducerTaskID, ProducerRole                                              string
	ReviewerAssignmentID, ReviewerWorkerID, ReviewerTaskID, ReviewerRole                        string
	AdapterChecks                                                                               map[string]bool
	AdapterChecksDigest                                                                         string
}
type ReviewerRaw struct {
	Checks  map[string]bool   `json:"checks"`
	Reasons map[string]string `json:"reasons"`
}
type Review struct {
	ReviewID, ReviewRequestID, Verdict string
	Checks, AdapterChecks              map[string]bool
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
type PercentageBreakdown struct{ Numerator, Denominator, Percent int }
type Account struct {
	Selection                                                                               AccountSelection
	RequestDigest, ResultDigest, ReviewDigest, ProducerAttemptDigest, ReviewerAttemptDigest string
	OutcomeCounts                                                                           map[string]int
	Balanced                                                                                bool
	Mechanical, Custody, Accounting, Evidence, Ceiling                                      PercentageBreakdown
	SemanticAccepted                                                                        bool
	Completeness, FeatureIdentity                                                           string
	Authority                                                                               int
	Accepted                                                                                bool
}

func Digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func canon(v any) []byte     { b, _ := json.Marshal(v); return b }
func domainID(domain string, v any) string {
	h := sha256.New()
	h.Write([]byte(Version + "\x00" + domain + "\x00"))
	h.Write(canon(v))
	return domain + "-" + hex.EncodeToString(h.Sum(nil))
}
func producerAttemptKey(r GroupRequest) string {
	return domainID("producer-attempt-key", struct{ GenerationID, AssignmentID, RequestID, AttemptID string }{r.GenerationID, r.AssignmentID, r.RequestID, r.AttemptID})
}
func canonicalReviewerKey(q ReviewRequest, attemptID string) ReviewerAttemptKey {
	return ReviewerAttemptKey{q.GenerationID, q.ReviewerAssignmentID, q.ReviewRequestID, attemptID}
}
func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || strings.ToLower(s) != s {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}
func validID(s string) bool {
	return s != "" && utf8.ValidString(s) && len([]byte(s)) <= MaxIdentityBytes && strings.TrimSpace(s) == s
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func exactStates(x []string) bool {
	return len(x) == 3 && x[0] == "RECEIVED" && x[1] == "ADAPTED" && x[2] == "COMMITTED"
}
func exactCounts(keys []string, m map[string]int, total int) bool {
	if len(m) != len(keys) {
		return false
	}
	n := 0
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v < 0 {
			return false
		}
		n += v
	}
	return n == total
}
func operationForCause(c string) string {
	switch c {
	case "CANCELLATION":
		return "CANCELLED"
	case "DEADLINE_EXPIRED":
		return "TIMEOUT"
	case "WORK_PRECHARGE", "RAW_SIZE", "RELATION_LIMIT", "CANDIDATE_LIMIT", "OCCURRENCE_LIMIT", "EVIDENCE_LIMIT", "SOURCE_OBJECT_LIMIT", "SOURCE_RANGE_LIMIT", "SOURCE_BYTES_LIMIT", "RESPONSE_LIMIT":
		return "RESOURCE_LIMIT"
	case "MISSING_SEARCH":
		return "INDEX_UNAVAILABLE"
	case "SEARCH_FREEZE", "SEARCH_SCHEMA", "SEARCH_UNBALANCED", "SEARCH_OUTCOMES", "SEARCH_DIGEST", "SEARCH_IDENTITY", "SEARCH_ATTEMPT", "INVALID_SEARCH":
		return "INDEX_MISMATCH"
	case "POLICY", "EVIDENCE_DIGEST":
		return "POLICY_MISMATCH"
	default:
		return "BACKEND_FAILURE"
	}
}

func ValidateSearch(s SearchAccount) error {
	if s.SchemaVersion == "" {
		return errors.New("MISSING_SEARCH")
	}
	if s.SchemaVersion != SearchSchemaVersion {
		return errors.New("SEARCH_SCHEMA")
	}
	if s.FreezeIdentity != SearchFreeze || s.ExecutionManifestDigest != SearchExecutionManifest || s.FinalAuditDigest != SearchFinalAudit || s.FinalSealDigest != SearchFinalSeal || s.TerminalReportDigest != SearchTerminalReport {
		return errors.New("SEARCH_FREEZE")
	}
	digests := []string{s.RequestDigest, s.ResultDigest, s.ReviewDigest, s.ProducerAttemptDigest, s.ReviewerAttemptDigest, s.ReviewRequestDigest, s.AccountDigest, s.CustodyDigest, s.ExecutionManifestDigest, s.FinalAuditDigest, s.FinalSealDigest, s.TerminalReportDigest}
	for _, d := range digests {
		if !validDigest(d) {
			return errors.New("SEARCH_DIGEST")
		}
	}
	if s.RequestDigest != SearchCase01Request || s.ResultDigest != SearchCase01Result || s.ProducerAttemptDigest != SearchCase01Producer || s.ReviewRequestDigest != SearchCase01ReviewRequest || s.ReviewerAttemptDigest != SearchCase01ReviewAttempt || s.AccountDigest != SearchCase01Account || s.CustodyDigest != SearchCase01Custody {
		return errors.New("SEARCH_DIGEST")
	}
	ids := []string{s.RequestID, s.ResultID, s.ReviewID, s.ProducerAttemptID, s.ReviewerAttemptID}
	for _, id := range ids {
		if !validID(id) {
			return errors.New("SEARCH_IDENTITY")
		}
	}
	if s.RequestID == s.ResultID || s.ResultID == s.ReviewID || s.ProducerAttemptID == s.ReviewerAttemptID {
		return errors.New("SEARCH_IDENTITY")
	}
	if !exactStates(s.ProducerStates) || !exactStates(s.ReviewerStates) {
		return errors.New("SEARCH_ATTEMPT")
	}
	if s.Denominator != MemberCount || len(s.Members) != MemberCount || s.Completeness != "COMPLETE" || !s.Balanced {
		return errors.New("SEARCH_UNBALANCED")
	}
	if s.Authority != 0 || s.Accepted || s.FeatureIdentity != "UNRESOLVED" || s.ReviewerVerdict != "ACCEPT_MECHANICAL" {
		return errors.New("INVALID_SEARCH")
	}
	if !exactCounts(SearchOutcomes[:], s.OutcomeCounts, MemberCount) || s.OutcomeCounts["RETURNED"] != 5 || s.OutcomeCounts["BELOW_THRESHOLD"] != 19 || s.OutcomeCounts["FILTERED_BY_POLICY"] != 0 || s.OutcomeCounts["DUPLICATE_MEMBER"] != 0 || s.OutcomeCounts["INVALID_MEMBER"] != 0 {
		return errors.New("SEARCH_OUTCOMES")
	}
	actual := map[string]int{}
	for _, o := range SearchOutcomes {
		actual[o] = 0
	}
	seen := map[string]bool{}
	for i, m := range s.Members {
		if m.Ordinal != i+1 || !validID(m.MemberID) || seen[m.MemberID] || !contains(SearchOutcomes[:], m.Outcome) || m.Authority != 0 || m.Accepted || m.FeatureIdentity != "UNRESOLVED" {
			return errors.New("SEARCH_UNBALANCED")
		}
		seen[m.MemberID] = true
		actual[m.Outcome]++
	}
	if !equalIntMap(actual, s.OutcomeCounts) {
		return errors.New("SEARCH_OUTCOMES")
	}
	return nil
}

func ValidateRequest(r GroupRequest) error {
	if len(canon(r)) > MaxRequestBytes {
		return errors.New("RAW_SIZE")
	}
	for _, x := range []string{r.RequestID, r.GenerationID, r.AttemptID, r.AssignmentID, r.PartitionCaptureID, r.PartitionIdentity, r.PartitionResolution} {
		if !validID(x) {
			return errors.New("ASSIGNMENT")
		}
	}
	if r.SchemaVersion != Version+".request" || r.SearchFreezeIdentity != SearchFreeze || r.PolicyDigest != GroupPolicyDigest || r.LimitsDigest != GroupLimitsDigest || !validDigest(r.EvidenceDigest) || r.Authority != 0 || r.Accepted || r.Completeness != "UNKNOWN" || r.FeatureIdentity != "UNRESOLVED" || r.Limits != FixedLimits() {
		return errors.New("POLICY")
	}
	if err := ValidateSearch(r.Search); err != nil {
		return err
	}
	if len(r.OrderedMembers) != MemberCount || len(r.Evidence) != MemberCount {
		return errors.New("SEARCH_UNBALANCED")
	}
	for i, m := range r.OrderedMembers {
		if m != r.Search.Members[i] {
			return errors.New("SEARCH_UNBALANCED")
		}
	}
	if Digest(canon(r.Evidence)) != r.EvidenceDigest {
		return errors.New("EVIDENCE_DIGEST")
	}
	return nil
}

func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, e := dec.Token()
		if e != nil {
			return errors.New("STRICT_JSON")
		}
		d, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				k, e := dec.Token()
				if e != nil {
					return errors.New("STRICT_JSON")
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return errors.New("STRICT_JSON")
				}
				seen[s] = true
				if e = walk(); e != nil {
					return e
				}
			}
			end, e := dec.Token()
			if e != nil || end != json.Delim('}') {
				return errors.New("STRICT_JSON")
			}
		case '[':
			for dec.More() {
				if e := walk(); e != nil {
					return e
				}
			}
			end, e := dec.Token()
			if e != nil || end != json.Delim(']') {
				return errors.New("STRICT_JSON")
			}
		default:
			return errors.New("STRICT_JSON")
		}
		return nil
	}
	if e := walk(); e != nil {
		return e
	}
	if _, e := dec.Token(); e != io.EOF {
		return errors.New("STRICT_JSON")
	}
	return nil
}
func strictRaw(raw []byte, max int, dst any) error {
	if len(raw) > max {
		return errors.New("RAW_SIZE")
	}
	if !utf8.Valid(raw) {
		return errors.New("UTF8")
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 {
		return errors.New("STRICT_JSON")
	}
	line := raw[:len(raw)-1]
	if e := rejectDuplicateKeys(line); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return errors.New("STRICT_JSON")
	}
	if !bytes.Equal(line, canon(dst)) {
		return errors.New("STRICT_JSON")
	}
	return nil
}
func ParseGroupRequest(raw []byte) (GroupRequest, error) {
	var r GroupRequest
	if e := strictRaw(raw, MaxRequestBytes, &r); e != nil {
		return r, e
	}
	return r, ValidateRequest(r)
}
func ParseProducerRaw(raw []byte) (ProducerRaw, error) {
	var p ProducerRaw
	e := strictRaw(raw, MaxResponseBytes, &p)
	return p, e
}
func ParseReviewerRaw(raw []byte) (ReviewerRaw, error) {
	var x ReviewerRaw
	if e := strictRaw(raw, MaxResponseBytes, &x); e != nil {
		return x, e
	}
	return x, validateReviewerRaw(x)
}

func classifyMember(r GroupRequest, index int, seenOrd map[int]bool, seenMember map[string]bool) (string, string) {
	e := r.Evidence[index]
	expected := r.OrderedMembers[index]
	if e.Ordinal < 1 || e.Ordinal > MemberCount || seenOrd[e.Ordinal] {
		return "DUPLICATE_MEMBER", "DUPLICATE_ORDINAL"
	}
	seenOrd[e.Ordinal] = true
	if seenMember[e.MemberID] {
		return "DUPLICATE_MEMBER", "DUPLICATE_MEMBER"
	}
	seenMember[e.MemberID] = true
	if e.Ordinal != index+1 || e.MemberID != expected.MemberID || !validID(e.MemberID) {
		return "INVALID_MEMBER", "INVALID_MEMBER"
	}
	if e.Filtered {
		return "FILTERED_BY_POLICY", "POLICY"
	}
	if e.Source == nil {
		return "INVALID_MEMBER", "INVALID_SOURCE"
	}
	s := e.Source
	if !s.Complete || !validDigest(s.Digest) || !validDigest(s.ObjectDigest) || !validID(s.ObjectID) || s.CaptureID != r.PartitionCaptureID || s.Authority != 0 || s.RangeCount < 0 || s.UniqueBytes < 0 {
		return "INVALID_MEMBER", "INVALID_SOURCE"
	}
	for _, o := range s.Objects {
		if !validID(o.ObjectID) || !validDigest(o.Digest) || !validDigest(o.SourceDigest) || o.CaptureID != r.PartitionCaptureID || !o.Complete || o.Authority != 0 || o.RangeCount < 0 || o.UniqueBytes < 0 {
			return "INVALID_MEMBER", "INVALID_SOURCE"
		}
	}
	for _, x := range e.Calls {
		if x.Kind != "CALLS" || !x.ServerReported || x.Provenance != "SERVER_REPORTED" || x.CaptureID != r.PartitionCaptureID || !validDigest(x.Digest) || x.FromMemberID != e.MemberID || !searchMemberExists(r, x.ToMemberID) {
			return "INVALID_MEMBER", "INVALID_CALLS"
		}
	}
	if e.Community == nil {
		return "UNMATCHED", ""
	}
	return classifyCommunity(r, e)
}
func classifyCommunity(r GroupRequest, e MemberEvidence) (string, string) {
	c := e.Community
	if c == nil {
		return "UNMATCHED", ""
	}
	if !c.Nominated {
		return "UNMATCHED", ""
	}
	if c.Algorithm != "LEIDEN" || !c.StructuralOnly || !c.Retained || !validDigest(c.Digest) || c.CaptureID != r.PartitionCaptureID || c.ObservationID != r.PartitionIdentity || c.MemberID != e.MemberID || !validID(c.CommunityID) || c.Seed != r.PartitionSeed || c.Resolution != r.PartitionResolution {
		return "INVALID_MEMBER", "INVALID_COMMUNITY"
	}
	return "GROUPED", ""
}
func searchMemberExists(r GroupRequest, id string) bool {
	for _, m := range r.OrderedMembers {
		if m.MemberID == id {
			return true
		}
	}
	return false
}

// Adapt consumes retained communities only. All checkpoint calls occur before
// their corresponding mutation and precharge the complete mutation unit.
func Adapt(r GroupRequest, attemptKey, rawDigest string, control Control) (Result, error) {
	res := Result{SchemaVersion: Version + ".result", RequestID: r.RequestID, GenerationID: r.GenerationID, AttemptKey: attemptKey, Completeness: "UNKNOWN", Denominator: MemberCount, OutcomeCounts: zeroCounts(MemberOutcomes[:]), RequestDigest: Digest(canon(r)), RawDigest: rawDigest, PolicyDigest: r.PolicyDigest, LimitsDigest: r.LimitsDigest, EvidenceDigest: r.EvidenceDigest, SearchFreezeIdentity: SearchFreeze, FeatureIdentity: "UNRESOLVED"}
	occ := map[string]int{}
	limit := control.WorkLimit
	if limit == 0 {
		limit = r.Limits.MaxWork
	}
	terminal := func(c string) (Result, error) {
		res.Outcome = operationForCause(c)
		res.CauseCode = c
		res.ResultID = ""
		res.Ledger = nil
		res.Candidates = nil
		res.OutcomeCounts = zeroCounts(MemberOutcomes[:])
		return res, errors.New(c)
	}
	checkpoint := func(name string, cost int) error {
		occ[name]++
		cancel, deadline, backend, wl := control.Cancelled, control.DeadlineExpired, control.BackendFailed, limit
		want := control.TriggerOccurrence
		if want == 0 {
			want = 1
		}
		if control.TriggerCheckpoint == name && occ[name] == want {
			cancel = cancel || control.TriggerCancelled
			deadline = deadline || control.TriggerDeadlineExpired
			backend = backend || control.TriggerBackendFailed
			if control.TriggerWorkLimit != 0 {
				wl = control.TriggerWorkLimit
			}
		}
		if cancel {
			return errors.New("CANCELLATION")
		}
		if deadline {
			return errors.New("DEADLINE_EXPIRED")
		}
		if backend {
			return errors.New("BACKEND")
		}
		if cost < 0 || res.Counters.WorkCharged > wl-cost {
			return errors.New("WORK_PRECHARGE")
		}
		res.Counters.WorkCharged += cost
		return nil
	}
	if e := checkpoint("BEFORE_IMPORT", WorkImport); e != nil {
		return terminal(e.Error())
	}
	if e := ValidateRequest(r); e != nil {
		return terminal(e.Error())
	}
	if e := validateAggregateLimits(r); e != nil {
		return terminal(e.Error())
	}
	seenOrd, seenMember := map[int]bool{}, map[string]bool{}
	groups := map[string][]MemberEvidence{}
	for i, evidence := range r.Evidence {
		if e := checkpoint("BEFORE_MEMBER", WorkMember); e != nil {
			return terminal(e.Error())
		}
		res.Counters.Begun++
		relationInvalid := false
		for range evidence.Calls {
			if e := checkpoint("BEFORE_RELATION", WorkRelation); e != nil {
				return terminal(e.Error())
			}
			res.Counters.Relations++
		}
		outcome, cause := classifyMember(r, i, seenOrd, seenMember)
		if relationInvalid {
			outcome, cause = "INVALID_MEMBER", "INVALID_CALLS"
		}
		entry := LedgerEntry{Ordinal: i + 1, MemberID: evidence.MemberID, Outcome: outcome, CauseCode: cause}
		res.Ledger = append(res.Ledger, entry)
		res.OutcomeCounts[outcome]++
		res.Counters.Completed++
		if outcome == "GROUPED" {
			groups[evidence.Community.CommunityID] = append(groups[evidence.Community.CommunityID], evidence)
		}
	}
	type group struct {
		id      string
		members []MemberEvidence
	}
	gs := make([]group, 0, len(groups))
	for id, m := range groups {
		sort.Slice(m, func(i, j int) bool { return m[i].Ordinal < m[j].Ordinal })
		gs = append(gs, group{id, m})
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].members[0].Ordinal != gs[j].members[0].Ordinal {
			return gs[i].members[0].Ordinal < gs[j].members[0].Ordinal
		}
		return memberVector(gs[i].members) < memberVector(gs[j].members)
	})
	if len(gs) > r.Limits.MaxCandidates {
		return terminal("CANDIDATE_LIMIT")
	}
	occurrences := 0
	for i, g := range gs {
		if len(g.members) > r.Limits.MaxMembersPerCandidate {
			return terminal("CANDIDATE_LIMIT")
		}
		if occurrences > r.Limits.MaxCandidateOccurrences-len(g.members) {
			return terminal("OCCURRENCE_LIMIT")
		}
		if e := checkpoint("BEFORE_CANDIDATE", WorkCandidate); e != nil {
			return terminal(e.Error())
		}
		ids := make([]string, len(g.members))
		ords := make([]int, len(g.members))
		for j, m := range g.members {
			ids[j], ords[j] = m.MemberID, m.Ordinal
		}
		occurrences += len(g.members)
		candidate := Candidate{CandidateID: domainID("group-candidate", struct {
			Capture, Observation, Community string
			Seed                            int64
			Resolution                      string
			Members                         []string
		}{r.PartitionCaptureID, r.PartitionIdentity, g.id, r.PartitionSeed, r.PartitionResolution, ids}), WorkingLabel: fmt.Sprintf("Provisional group %02d", i+1), CommunityID: g.id, MemberIDs: ids, MemberOrdinals: ords, FeatureIdentity: "UNRESOLVED"}
		res.Candidates = append(res.Candidates, candidate)
		res.Counters.Candidates++
		for li := range res.Ledger {
			for _, o := range ords {
				if res.Ledger[li].Ordinal == o {
					res.Ledger[li].CandidateID = candidate.CandidateID
				}
			}
		}
	}
	if e := checkpoint("BEFORE_COMMIT", WorkCommit); e != nil {
		return terminal(e.Error())
	}
	if len(res.Ledger) != MemberCount || !exactCounts(MemberOutcomes[:], res.OutcomeCounts, MemberCount) {
		return terminal("BACKEND")
	}
	if e := validateCandidateLimits(res.Candidates, r.Limits); e != nil {
		return terminal(e.Error())
	}
	res.Outcome = "COMPLETE"
	res.ResultID = domainID("group-result", res)
	if len(canon(res)) > r.Limits.MaxResponseBytes {
		return terminal("RESPONSE_LIMIT")
	}
	return res, nil
}

func validateAggregateLimits(r GroupRequest) error {
	if len(r.Evidence) != MemberCount {
		return errors.New("SEARCH_UNBALANCED")
	}
	type objectIdentity struct {
		SourceDigest, ObjectDigest, CaptureID string
		Complete                              bool
		Authority, RangeCount, UniqueBytes    int
	}
	relations, objects, ranges, uniqueBytes := 0, map[string]objectIdentity{}, 0, 0
	addObject := func(id string, identity objectIdentity) error {
		if old, ok := objects[id]; ok {
			if old != identity {
				return errors.New("INVALID_SOURCE")
			}
			return nil
		}
		objects[id] = identity
		ranges += identity.RangeCount
		uniqueBytes += identity.UniqueBytes
		return nil
	}
	for _, e := range r.Evidence {
		refs := len(e.Calls)
		if e.Source != nil {
			refs++
			if err := addObject(e.Source.ObjectID, objectIdentity{e.Source.Digest, e.Source.ObjectDigest, e.Source.CaptureID, e.Source.Complete, e.Source.Authority, e.Source.RangeCount, e.Source.UniqueBytes}); err != nil {
				return err
			}
			for _, o := range e.Source.Objects {
				refs++
				if err := addObject(o.ObjectID, objectIdentity{o.SourceDigest, o.Digest, o.CaptureID, o.Complete, o.Authority, o.RangeCount, o.UniqueBytes}); err != nil {
					return err
				}
			}
		}
		if e.Community != nil {
			refs++
		}
		if refs > r.Limits.MaxEvidenceRefsPerMember {
			return errors.New("EVIDENCE_LIMIT")
		}
		relations += len(e.Calls)
	}
	if relations > r.Limits.MaxRelations {
		return errors.New("RELATION_LIMIT")
	}
	if len(objects) > r.Limits.MaxSourceObjects {
		return errors.New("SOURCE_OBJECT_LIMIT")
	}
	if ranges > r.Limits.MaxSourceRanges {
		return errors.New("SOURCE_RANGE_LIMIT")
	}
	if uniqueBytes > r.Limits.MaxUniqueSourceBytes {
		return errors.New("SOURCE_BYTES_LIMIT")
	}
	return nil
}
func validateCandidateLimits(candidates []Candidate, l Limits) error {
	if len(candidates) > l.MaxCandidates {
		return errors.New("CANDIDATE_LIMIT")
	}
	occurrences := 0
	for _, c := range candidates {
		if len(c.MemberIDs) != len(c.MemberOrdinals) || len(c.MemberIDs) > l.MaxMembersPerCandidate {
			return errors.New("CANDIDATE_LIMIT")
		}
		if occurrences > l.MaxCandidateOccurrences-len(c.MemberIDs) {
			return errors.New("OCCURRENCE_LIMIT")
		}
		occurrences += len(c.MemberIDs)
	}
	return nil
}
func memberVector(es []MemberEvidence) string {
	x := make([]string, len(es))
	for i, e := range es {
		x[i] = e.MemberID
	}
	return strings.Join(x, "\x00")
}
func zeroCounts(keys []string) map[string]int {
	m := map[string]int{}
	for _, k := range keys {
		m[k] = 0
	}
	return m
}
func equalIntMap(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func cloneAttempt(a Attempt) Attempt {
	a.RawBytes = append([]byte(nil), a.RawBytes...)
	a.States = append([]string(nil), a.States...)
	return a
}
func cloneResult(r Result) Result {
	r.Ledger = append([]LedgerEntry(nil), r.Ledger...)
	r.OutcomeCounts = copyInt(r.OutcomeCounts)
	r.Candidates = append([]Candidate(nil), r.Candidates...)
	for i := range r.Candidates {
		r.Candidates[i].MemberIDs = append([]string(nil), r.Candidates[i].MemberIDs...)
		r.Candidates[i].MemberOrdinals = append([]int(nil), r.Candidates[i].MemberOrdinals...)
	}
	return r
}

type Writer struct {
	mu        sync.Mutex
	attempts  map[string]Attempt
	results   map[string]Result
	conflicts []DeliveryConflict
}

func NewWriter() *Writer {
	return &Writer{attempts: map[string]Attempt{}, results: map[string]Result{}}
}
func (w *Writer) Ingest(r GroupRequest, raw []byte, expected string, control Control) (Attempt, Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := producerAttemptKey(r)
	dig := Digest(raw)
	if old, ok := w.attempts[key]; ok {
		if old.RawDigest == dig {
			return cloneAttempt(old), cloneResult(w.results[old.ResultID]), nil
		}
		w.conflicts = append(w.conflicts, DeliveryConflict{domainID("delivery-conflict", []string{key, old.RawDigest, dig}), key, old.RawDigest, dig})
		return cloneAttempt(old), Result{}, errors.New("CONFLICTING_DELIVERY")
	}
	for _, prior := range w.attempts {
		if prior.GenerationID == r.GenerationID && prior.AssignmentID == r.AssignmentID && prior.RequestID == r.RequestID && prior.AttemptID != r.AttemptID {
			return Attempt{}, Result{}, errors.New("SECOND_ATTEMPT")
		}
	}
	a := Attempt{Key: key, GenerationID: r.GenerationID, RequestID: r.RequestID, AttemptID: r.AttemptID, AssignmentID: r.AssignmentID, RawDigest: dig, RawBytes: append([]byte(nil), raw...), States: []string{"RECEIVED"}}
	fail := func(e error, x Result) (Attempt, Result, error) {
		a.States = append(a.States, "TERMINAL_INVALID")
		a.TerminalReason = e.Error()
		a.Counters = x.Counters
		w.attempts[key] = cloneAttempt(a)
		return cloneAttempt(a), Result{}, e
	}
	if expected != "" && expected != dig {
		return fail(errors.New("DIGEST"), Result{})
	}
	p, e := ParseProducerRaw(raw)
	if e != nil {
		return fail(e, Result{})
	}
	if Digest(canon(p.Evidence)) != r.EvidenceDigest {
		return fail(errors.New("EVIDENCE_DIGEST"), Result{})
	}
	r.Evidence = p.Evidence
	a.States = append(a.States, "ADAPTED")
	x, e := Adapt(r, key, dig, control)
	if e != nil {
		return fail(e, x)
	}
	a.States = append(a.States, "COMMITTED")
	a.ResultID = x.ResultID
	a.Counters = x.Counters
	w.attempts[key] = cloneAttempt(a)
	w.results[x.ResultID] = cloneResult(x)
	return cloneAttempt(a), cloneResult(x), nil
}
func (w *Writer) Conflicts() []DeliveryConflict {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]DeliveryConflict(nil), w.conflicts...)
}

func ValidateResult(r GroupRequest, a Attempt, x Result) error {
	if err := ValidateRequest(r); err != nil {
		return err
	}
	if !exactStates(a.States) || a.GenerationID != r.GenerationID || a.RequestID != r.RequestID || a.AttemptID != r.AttemptID || a.AssignmentID != r.AssignmentID || a.ResultID != x.ResultID || a.RawDigest != x.RawDigest || Digest(a.RawBytes) != a.RawDigest || a.Counters != x.Counters || a.Key != producerAttemptKey(r) || a.Key != x.AttemptKey || a.TerminalReason != "" {
		return errors.New("RESULT_BINDING")
	}
	parsed, err := ParseProducerRaw(a.RawBytes)
	if err != nil || !reflect.DeepEqual(parsed.Evidence, r.Evidence) || Digest(canon(parsed.Evidence)) != r.EvidenceDigest {
		return errors.New("RESULT_RAW_CUSTODY")
	}
	expected, err := Adapt(r, producerAttemptKey(r), a.RawDigest, Control{})
	if err != nil {
		return errors.New("RESULT_RECOMPUTE")
	}
	if !reflect.DeepEqual(expected, x) {
		return errors.New("RESULT_MISMATCH")
	}
	return nil
}
func adapterChecks(r GroupRequest, a Attempt, x Result) map[string]bool {
	requestValid := ValidateRequest(r) == nil
	resultValid := ValidateResult(r, a, x) == nil
	return map[string]bool{"request_bound": requestValid, "raw_bound": resultValid && x.RawDigest == a.RawDigest, "result_bound": resultValid, "search_bound": requestValid && ValidateSearch(r.Search) == nil, "evidence_bound": requestValid && x.EvidenceDigest == r.EvidenceDigest && r.EvidenceDigest == Digest(canon(r.Evidence)), "partition_balanced": resultValid && len(x.Ledger) == MemberCount && exactCounts(MemberOutcomes[:], x.OutcomeCounts, MemberCount), "candidate_nonsemantic": resultValid && candidateCeilings(x), "authority_ceiling": resultValid && x.Authority == 0 && !x.Accepted && x.Completeness == "UNKNOWN" && x.FeatureIdentity == "UNRESOLVED", "custody_procedural": resultValid && exactStates(a.States)}
}
func candidateCeilings(x Result) bool {
	seen := map[int]bool{}
	if len(x.Candidates) > MaxCandidates {
		return false
	}
	occ := 0
	for _, c := range x.Candidates {
		if len(c.MemberIDs) > MaxMembersPerCandidate || c.Authority != 0 || c.Accepted || c.FeatureIdentity != "UNRESOLVED" {
			return false
		}
		for _, o := range c.MemberOrdinals {
			if seen[o] {
				return false
			}
			seen[o] = true
			occ++
		}
	}
	return occ <= MaxCandidateOccurrences
}
func NewReviewRequest(r GroupRequest, a Attempt, x Result, pw, pt, pr, ra, rw, rt, rr string) (ReviewRequest, error) {
	for _, s := range []string{pw, pt, pr, ra, rw, rt, rr} {
		if !validID(s) {
			return ReviewRequest{}, errors.New("ASSIGNMENT")
		}
	}
	if r.AssignmentID == ra || pw == rw || pt == rt || pr == rr || ValidateResult(r, a, x) != nil {
		return ReviewRequest{}, errors.New("ASSIGNMENT")
	}
	checks := adapterChecks(r, a, x)
	q := ReviewRequest{SchemaVersion: Version + ".review-request", GenerationID: r.GenerationID, RequestID: r.RequestID, RequestDigest: Digest(canon(r)), ProducerAttemptKey: a.Key, ProducerAttemptID: a.AttemptID, ProducerAssignmentID: a.AssignmentID, ProducerAttemptDigest: Digest(canon(a)), RawDigest: a.RawDigest, ResultID: x.ResultID, ResultDigest: Digest(canon(x)), SearchDigest: Digest(canon(r.Search)), EvidenceDigest: r.EvidenceDigest, PolicyDigest: r.PolicyDigest, LimitsDigest: r.LimitsDigest, ProducerWorkerID: pw, ProducerTaskID: pt, ProducerRole: pr, ReviewerAssignmentID: ra, ReviewerWorkerID: rw, ReviewerTaskID: rt, ReviewerRole: rr, AdapterChecks: checks, AdapterChecksDigest: Digest(canon(checks))}
	q.ReviewRequestID = domainID("review-request", q)
	return q, nil
}
func validateReviewerRaw(x ReviewerRaw) error {
	if len(x.Checks) != len(ReviewChecks) {
		return errors.New("REVIEW_CHECKS")
	}
	known := map[string]bool{}
	for _, k := range ReviewChecks {
		known[k] = true
		if _, ok := x.Checks[k]; !ok {
			return errors.New("REVIEW_CHECKS")
		}
	}
	for k := range x.Checks {
		if !known[k] {
			return errors.New("REVIEW_CHECKS")
		}
	}
	for k, v := range x.Checks {
		reason, ok := x.Reasons[k]
		if !v {
			if !ok || strings.TrimSpace(reason) == "" || len([]byte(reason)) > MaxReviewReasonBytes {
				return errors.New("REVIEW_REASONS")
			}
		} else if ok {
			return errors.New("FALSE_ACCEPT")
		}
	}
	return nil
}
func deriveReview(q ReviewRequest, x ReviewerRaw, recomputed map[string]bool) (Review, error) {
	if e := validateReviewerRaw(x); e != nil {
		return Review{}, e
	}
	if len(recomputed) != len(ReviewChecks) {
		return Review{}, errors.New("ADAPTER_CHECKS")
	}
	verdict := "ACCEPT_MECHANICAL_DESIGN"
	for _, k := range ReviewChecks {
		adapter, ok := recomputed[k]
		if !ok || !adapter || !x.Checks[k] {
			verdict = "REJECT"
		}
	}
	r := Review{ReviewRequestID: q.ReviewRequestID, Verdict: verdict, Checks: copyBool(x.Checks), AdapterChecks: copyBool(recomputed), Reasons: copyString(x.Reasons)}
	r.ReviewID = domainID("review", r)
	return r, nil
}
func cloneReview(r Review) Review {
	r.Checks = copyBool(r.Checks)
	r.AdapterChecks = copyBool(r.AdapterChecks)
	r.Reasons = copyString(r.Reasons)
	return r
}
func cloneReviewerAttempt(a ReviewerAttempt) ReviewerAttempt {
	a.RawBytes = append([]byte(nil), a.RawBytes...)
	a.States = append([]string(nil), a.States...)
	return a
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
func (w *ReviewWriter) Ingest(r GroupRequest, producer Attempt, result Result, q ReviewRequest, k ReviewerAttemptKey, raw []byte, expected string) (ReviewerAttempt, Review, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	recomputedQ, err := NewReviewRequest(r, producer, result, q.ProducerWorkerID, q.ProducerTaskID, q.ProducerRole, q.ReviewerAssignmentID, q.ReviewerWorkerID, q.ReviewerTaskID, q.ReviewerRole)
	if err != nil || !reflect.DeepEqual(recomputedQ, q) {
		return ReviewerAttempt{}, Review{}, errors.New("REVIEW_REQUEST_BINDING")
	}
	recomputedChecks := adapterChecks(r, producer, result)
	if k.GenerationID != q.GenerationID || k.AssignmentID != q.ReviewerAssignmentID || k.ReviewRequestID != q.ReviewRequestID || !validID(k.AttemptID) {
		return ReviewerAttempt{}, Review{}, errors.New("ASSIGNMENT")
	}
	for x := range w.attempts {
		if x.GenerationID == k.GenerationID && x.AssignmentID == k.AssignmentID && x.ReviewRequestID == k.ReviewRequestID && x.AttemptID != k.AttemptID {
			return ReviewerAttempt{}, Review{}, errors.New("SECOND_ATTEMPT")
		}
	}
	dig := Digest(raw)
	if old, ok := w.attempts[k]; ok {
		if old.RawDigest == dig {
			return cloneReviewerAttempt(old), cloneReview(w.reviews[old.ReviewID]), nil
		}
		w.conflicts = append(w.conflicts, ReviewDeliveryConflict{domainID("review-conflict", []string{old.RawDigest, dig}), k, old.RawDigest, dig})
		return cloneReviewerAttempt(old), Review{}, errors.New("CONFLICTING_DELIVERY")
	}
	a := ReviewerAttempt{Key: k, RawDigest: dig, RawBytes: append([]byte(nil), raw...), States: []string{"RECEIVED"}}
	fail := func(e error) (ReviewerAttempt, Review, error) {
		a.States = append(a.States, "TERMINAL_INVALID")
		a.TerminalReason = e.Error()
		w.attempts[k] = cloneReviewerAttempt(a)
		return cloneReviewerAttempt(a), Review{}, e
	}
	if expected != "" && expected != dig {
		return fail(errors.New("DIGEST"))
	}
	x, e := ParseReviewerRaw(raw)
	if e != nil {
		return fail(e)
	}
	a.States = append(a.States, "ADAPTED")
	rv, e := deriveReview(q, x, recomputedChecks)
	if e != nil {
		return fail(e)
	}
	a.States = append(a.States, "COMMITTED")
	a.ReviewID = rv.ReviewID
	w.attempts[k] = cloneReviewerAttempt(a)
	w.reviews[rv.ReviewID] = cloneReview(rv)
	return cloneReviewerAttempt(a), cloneReview(rv), nil
}
func (w *ReviewWriter) Conflicts() []ReviewDeliveryConflict {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ReviewDeliveryConflict(nil), w.conflicts...)
}

func pct(n, d int) (PercentageBreakdown, error) {
	if d <= 0 || n < 0 || n > d {
		return PercentageBreakdown{}, errors.New("ACCOUNT_PERCENT")
	}
	return PercentageBreakdown{n, d, n * 100 / d}, nil
}
func MakeAccount(sel AccountSelection, r GroupRequest, a Attempt, x Result, q ReviewRequest, ra ReviewerAttempt, rv Review, counts map[string]int) (Account, error) {
	if ValidateRequest(r) != nil || ValidateResult(r, a, x) != nil {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	expectedQ, err := NewReviewRequest(r, a, x, q.ProducerWorkerID, q.ProducerTaskID, q.ProducerRole, q.ReviewerAssignmentID, q.ReviewerWorkerID, q.ReviewerTaskID, q.ReviewerRole)
	if err != nil || !reflect.DeepEqual(expectedQ, q) {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	if !validID(ra.Key.AttemptID) || ra.Key != canonicalReviewerKey(q, ra.Key.AttemptID) || Digest(ra.RawBytes) != ra.RawDigest || !exactStates(ra.States) || ra.TerminalReason != "" || ra.ReviewID != rv.ReviewID {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	parsedRaw, err := ParseReviewerRaw(ra.RawBytes)
	if err != nil || !reflect.DeepEqual(parsedRaw.Checks, rv.Checks) || !reflect.DeepEqual(parsedRaw.Reasons, rv.Reasons) {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	expectedChecks := adapterChecks(r, a, x)
	expectedRV, err := deriveReview(q, parsedRaw, expectedChecks)
	if err != nil || !reflect.DeepEqual(expectedRV, rv) {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	selectionFacts := []bool{sel.GenerationID == r.GenerationID, sel.RequestID == r.RequestID, sel.ProducerAttemptKey == a.Key, sel.ResultID == x.ResultID, sel.ReviewerAttemptKey == ra.Key, sel.ReviewID == rv.ReviewID, ra.Key == canonicalReviewerKey(q, ra.Key.AttemptID), Digest(ra.RawBytes) == ra.RawDigest, ra.ReviewID == rv.ReviewID, exactStates(ra.States), ra.TerminalReason == ""}
	if rv.Verdict != "ACCEPT_MECHANICAL_DESIGN" || !allTrue(rv.Checks) || !allTrue(rv.AdapterChecks) {
		return Account{}, errors.New("ACCOUNT_BINDING")
	}
	actual := zeroCounts(MemberOutcomes[:])
	for _, entry := range x.Ledger {
		if _, ok := actual[entry.Outcome]; !ok {
			return Account{}, errors.New("ACCOUNT_OUTCOME")
		}
		actual[entry.Outcome]++
	}
	accountingFacts := []bool{len(x.Ledger) == MemberCount, exactCounts(MemberOutcomes[:], actual, MemberCount), equalIntMap(actual, counts), x.Denominator == MemberCount}
	mechanical, _ := pct(countTrue(expectedChecks), len(ReviewChecks))
	custody, _ := pct(countBools(selectionFacts), len(selectionFacts))
	accounting, _ := pct(countBools(accountingFacts), len(accountingFacts))
	evidence, _ := pct(validEvidenceCount(r), MemberCount)
	ceilingFacts := []bool{r.Authority == 0, !r.Accepted, r.Completeness == "UNKNOWN", r.FeatureIdentity == "UNRESOLVED", x.Authority == 0, !x.Accepted, x.Completeness == "UNKNOWN", x.FeatureIdentity == "UNRESOLVED"}
	ceiling, _ := pct(countBools(ceilingFacts), len(ceilingFacts))
	for _, b := range []PercentageBreakdown{mechanical, custody, accounting, evidence, ceiling} {
		if b.Percent != 100 {
			return Account{}, errors.New("ACCOUNT_PERCENT")
		}
	}
	return Account{Selection: sel, RequestDigest: Digest(canon(r)), ResultDigest: Digest(canon(x)), ReviewDigest: Digest(canon(rv)), ProducerAttemptDigest: Digest(canon(a)), ReviewerAttemptDigest: Digest(canon(ra)), OutcomeCounts: copyInt(actual), Balanced: true, Mechanical: mechanical, Custody: custody, Accounting: accounting, Evidence: evidence, Ceiling: ceiling, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}, nil
}
func validEvidenceCount(r GroupRequest) int {
	if validateAggregateLimits(r) != nil {
		return 0
	}
	seenOrd, seenMember := map[int]bool{}, map[string]bool{}
	n := 0
	for i := range r.Evidence {
		outcome, _ := classifyMember(r, i, seenOrd, seenMember)
		if outcome == "GROUPED" || outcome == "UNMATCHED" || outcome == "FILTERED_BY_POLICY" {
			n++
		}
	}
	return n
}

func copyBool(m map[string]bool) map[string]bool {
	x := map[string]bool{}
	for k, v := range m {
		x[k] = v
	}
	return x
}
func copyString(m map[string]string) map[string]string {
	x := map[string]string{}
	for k, v := range m {
		x[k] = v
	}
	return x
}
func copyInt(m map[string]int) map[string]int {
	x := map[string]int{}
	for k, v := range m {
		x[k] = v
	}
	return x
}
func equalBoolMap(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
func allTrue(m map[string]bool) bool {
	if len(m) != len(ReviewChecks) {
		return false
	}
	for _, k := range ReviewChecks {
		if !m[k] {
			return false
		}
	}
	return true
}
func countTrue(m map[string]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}
func boolN(v bool) int {
	if v {
		return 1
	}
	return 0
}
func countBools(values []bool) int {
	n := 0
	for _, v := range values {
		if v {
			n++
		}
	}
	return n
}
