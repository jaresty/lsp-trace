package groupqualificationv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type FreezeFile struct {
	Path   string
	Digest string
	Bytes  int64
}
type DesignFreeze struct {
	SchemaVersion       string
	Status              string
	FreezeID            string
	Files               []FreezeFile
	Sources             []FreezeFile
	DispatchAllowed     bool
	GroupExecuted       bool
	GroupDesignGO       bool
	BlockedPredecessors []string
}
type freezeCase struct{ ID, Kind string }
type freezeCondition struct {
	SchemaVersion, CaseID, Kind string
	Control                     Control
}
type freezeExpected struct {
	SchemaVersion, CaseID, Cause, SecondaryCause, OperationOutcome, MechanicalQualification, NonqualificationReason string
	Success, Committed, ProducerReplay, ProducerConflict, ProducerSecondAttempt                                     bool
	ReviewerReplay, ReviewerConflict, ReviewerSecondAttempt, ForgeryRejected                                        bool
	OutcomeCounts                                                                                                   map[string]int
	Counters                                                                                                        Counters
	Candidates                                                                                                      int
}

var freezeCases = []freezeCase{
	{"01-complete-two-candidates", "two"}, {"02-complete-all-unmatched", "unmatched"}, {"03-index-unavailable", "missing-search"}, {"04-index-mismatch-freeze", "search-freeze"},
	{"05-incomplete-search", "search-incomplete"}, {"06-unbalanced-search", "search-unbalanced"}, {"07-search-authority-violation", "search-authority"}, {"08-search-accepted-violation", "search-accepted"},
	{"09-search-feature-violation", "search-feature"}, {"10-source-unavailable", "source-unavailable"}, {"11-source-collision", "source-collision"}, {"12-source-digest-invalid", "source-digest"},
	{"13-text-calls-substitution", "text-calls"}, {"14-cross-capture-calls", "cross-capture"}, {"15-community-unavailable", "community-unavailable"}, {"16-community-non-leiden", "community-non-leiden"},
	{"17-feature-semantic-candidate-rejection", "feature-semantic"}, {"18-duplicate-member-accounted", "duplicate"}, {"19-invalid-member-accounted", "invalid"}, {"20-overlap-rejection", "overlap"},
	{"21-policy-filtered", "filtered"}, {"22-exact-work-boundary", "work-boundary"}, {"23-work-plus-one-precharge", "work-plus-one"}, {"24-cancellation-and-deadline-before-commit", "custody"},
}

func freezeCanonical(v any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }
func freezeDigest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func freezeWrite(path string, v any) error { return os.WriteFile(path, freezeCanonical(v), 0644) }
func fstr() map[string]any                 { return map[string]any{"type": "string", "minLength": 1} }
func fid() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": MaxIdentityBytes, "pattern": `^\S(?:.*\S)?$`, "x-maxUtf8Bytes": MaxIdentityBytes, "x-noSurroundingWhitespace": true}
}
func fdigest() map[string]any {
	return map[string]any{"type": "string", "pattern": `^sha256:[0-9a-f]{64}$`}
}
func fenum(v ...string) map[string]any { return map[string]any{"type": "string", "enum": v} }
func fint(a, b int) map[string]any {
	return map[string]any{"type": "integer", "minimum": a, "maximum": b}
}
func fbool() map[string]any { return map[string]any{"type": "boolean"} }
func farr(i map[string]any, a, b int) map[string]any {
	return map[string]any{"type": "array", "items": i, "minItems": a, "maxItems": b}
}
func fobj(req []string, p map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": req, "properties": p}
}
func fpub(n string, s map[string]any) map[string]any {
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = Version + ".schema." + n
	return s
}
func fraw(max int) map[string]any {
	return map[string]any{"type": "string", "minLength": 4, "maxLength": ((max + 2) / 3) * 4, "pattern": `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`, "x-maxDecodedBytes": max}
}
func legacyFreezeSchemas() map[string]map[string]any {
	searchMember := fobj([]string{"Ordinal", "MemberID", "Outcome", "Authority", "Accepted", "FeatureIdentity"}, map[string]any{"Ordinal": fint(1, MemberCount), "MemberID": fid(), "Outcome": fenum(SearchOutcomes[:]...), "Authority": fint(0, 0), "Accepted": map[string]any{"type": "boolean", "const": false}, "FeatureIdentity": map[string]any{"type": "string", "const": "UNRESOLVED"}})
	searchAccount := fobj([]string{"SchemaVersion", "FreezeIdentity", "RequestID", "RequestDigest", "ResultID", "ResultDigest", "ReviewID", "ReviewDigest", "ProducerAttemptID", "ProducerAttemptDigest", "ReviewerAttemptID", "ReviewerAttemptDigest", "ReviewRequestDigest", "AccountDigest", "CustodyDigest", "ExecutionManifestDigest", "FinalAuditDigest", "FinalSealDigest", "TerminalReportDigest", "ProducerStates", "ReviewerStates", "Denominator", "Completeness", "ReviewerVerdict", "Balanced", "OutcomeCounts", "Authority", "Accepted", "FeatureIdentity", "Members"}, map[string]any{"SchemaVersion": map[string]any{"const": SearchSchemaVersion}, "FreezeIdentity": map[string]any{"const": SearchFreeze}, "RequestID": fid(), "RequestDigest": fdigest(), "ResultID": fid(), "ResultDigest": fdigest(), "ReviewID": fid(), "ReviewDigest": fdigest(), "ProducerAttemptID": fid(), "ProducerAttemptDigest": fdigest(), "ReviewerAttemptID": fid(), "ReviewerAttemptDigest": fdigest(), "ReviewRequestDigest": fdigest(), "AccountDigest": fdigest(), "CustodyDigest": fdigest(), "ExecutionManifestDigest": map[string]any{"const": SearchExecutionManifest}, "FinalAuditDigest": map[string]any{"const": SearchFinalAudit}, "FinalSealDigest": map[string]any{"const": SearchFinalSeal}, "TerminalReportDigest": map[string]any{"const": SearchTerminalReport}, "ProducerStates": farr(fenum(AttemptStates[:]...), 3, 3), "ReviewerStates": farr(fenum(AttemptStates[:]...), 3, 3), "Denominator": fint(MemberCount, MemberCount), "Completeness": map[string]any{"const": "COMPLETE"}, "ReviewerVerdict": map[string]any{"const": "ACCEPT_MECHANICAL"}, "Balanced": map[string]any{"const": true}, "OutcomeCounts": fobj(SearchOutcomes[:], map[string]any{}), "Authority": fint(0, 0), "Accepted": map[string]any{"const": false}, "FeatureIdentity": map[string]any{"const": "UNRESOLVED"}, "Members": farr(searchMember, MemberCount, MemberCount)})
	for _, k := range SearchOutcomes {
		searchAccount["properties"].(map[string]any)["OutcomeCounts"].(map[string]any)["properties"].(map[string]any)[k] = fint(0, MemberCount)
	}
	sourceObject := fobj([]string{"ObjectID", "Digest", "SourceDigest", "CaptureID", "Complete", "Authority", "RangeCount", "UniqueBytes"}, map[string]any{"ObjectID": fid(), "Digest": fdigest(), "SourceDigest": fdigest(), "CaptureID": fid(), "Complete": fbool(), "Authority": fint(0, 0), "RangeCount": fint(0, MaxSourceRanges), "UniqueBytes": fint(0, MaxUniqueSourceBytes)})
	source := fobj([]string{"Digest", "CaptureID", "ObjectID", "ObjectDigest", "Complete", "Authority", "RangeCount", "UniqueBytes", "Objects"}, map[string]any{"Digest": fdigest(), "CaptureID": fid(), "ObjectID": fid(), "ObjectDigest": fdigest(), "Complete": fbool(), "Authority": fint(0, 0), "RangeCount": fint(0, MaxSourceRanges), "UniqueBytes": fint(0, MaxUniqueSourceBytes), "Objects": farr(sourceObject, 0, MaxSourceObjects)})
	calls := fobj([]string{"Digest", "CaptureID", "FromMemberID", "ToMemberID", "Kind", "Provenance", "ServerReported"}, map[string]any{"Digest": fdigest(), "CaptureID": fid(), "FromMemberID": fid(), "ToMemberID": fid(), "Kind": map[string]any{"const": "CALLS"}, "Provenance": map[string]any{"const": "SERVER_REPORTED"}, "ServerReported": map[string]any{"const": true}})
	community := fobj([]string{"Digest", "CaptureID", "ObservationID", "CommunityID", "MemberID", "Algorithm", "StructuralOnly", "Retained", "Nominated", "Seed", "Resolution"}, map[string]any{"Digest": fdigest(), "CaptureID": fid(), "ObservationID": fid(), "CommunityID": fid(), "MemberID": fid(), "Algorithm": map[string]any{"const": "LEIDEN"}, "StructuralOnly": fbool(), "Retained": fbool(), "Nominated": fbool(), "Seed": map[string]any{"type": "integer"}, "Resolution": fid()})
	evidence := fobj([]string{"Ordinal", "MemberID", "Filtered", "Source", "Calls", "Community"}, map[string]any{"Ordinal": fint(1, MemberCount), "MemberID": fid(), "Filtered": fbool(), "Source": map[string]any{"anyOf": []any{source, map[string]any{"type": "null"}}}, "Calls": farr(calls, 0, MaxRelations), "Community": map[string]any{"anyOf": []any{community, map[string]any{"type": "null"}}}})
	limits := fobj([]string{"MaxRequestBytes", "MaxResponseBytes", "MaxIdentityBytes", "MaxMembers", "MaxRelations", "MaxCandidates", "MaxMembersPerCandidate", "MaxCandidateOccurrences", "MaxEvidenceRefsPerMember", "MaxSourceObjects", "MaxSourceRanges", "MaxUniqueSourceBytes", "MaxWork"}, map[string]any{})
	lv := FixedLimits()
	vals := map[string]int{"MaxRequestBytes": lv.MaxRequestBytes, "MaxResponseBytes": lv.MaxResponseBytes, "MaxIdentityBytes": lv.MaxIdentityBytes, "MaxMembers": lv.MaxMembers, "MaxRelations": lv.MaxRelations, "MaxCandidates": lv.MaxCandidates, "MaxMembersPerCandidate": lv.MaxMembersPerCandidate, "MaxCandidateOccurrences": lv.MaxCandidateOccurrences, "MaxEvidenceRefsPerMember": lv.MaxEvidenceRefsPerMember, "MaxSourceObjects": lv.MaxSourceObjects, "MaxSourceRanges": lv.MaxSourceRanges, "MaxUniqueSourceBytes": lv.MaxUniqueSourceBytes, "MaxWork": lv.MaxWork}
	for k, v := range vals {
		limits["properties"].(map[string]any)[k] = fint(v, v)
	}
	request := fobj([]string{"SchemaVersion", "RequestID", "GenerationID", "AttemptID", "AssignmentID", "SearchFreezeIdentity", "Search", "OrderedMembers", "PolicyDigest", "LimitsDigest", "EvidenceDigest", "PartitionCaptureID", "PartitionIdentity", "PartitionSeed", "PartitionResolution", "Authority", "Accepted", "Completeness", "FeatureIdentity", "Limits", "Evidence"}, map[string]any{"SchemaVersion": map[string]any{"const": Version + ".request"}, "RequestID": fid(), "GenerationID": fid(), "AttemptID": fid(), "AssignmentID": fid(), "SearchFreezeIdentity": map[string]any{"const": SearchFreeze}, "Search": searchAccount, "OrderedMembers": farr(searchMember, MemberCount, MemberCount), "PolicyDigest": map[string]any{"const": GroupPolicyDigest}, "LimitsDigest": map[string]any{"const": GroupLimitsDigest}, "EvidenceDigest": fdigest(), "PartitionCaptureID": fid(), "PartitionIdentity": fid(), "PartitionSeed": map[string]any{"type": "integer"}, "PartitionResolution": fid(), "Authority": fint(0, 0), "Accepted": map[string]any{"const": false}, "Completeness": map[string]any{"const": "UNKNOWN"}, "FeatureIdentity": map[string]any{"const": "UNRESOLVED"}, "Limits": limits, "Evidence": farr(evidence, MemberCount, MemberCount)})
	counts := fobj(MemberOutcomes[:], map[string]any{})
	for _, k := range MemberOutcomes {
		counts["properties"].(map[string]any)[k] = fint(0, MemberCount)
	}
	attempt := fobj([]string{"Key", "GenerationID", "RequestID", "AttemptID", "AssignmentID", "RawDigest", "RawBytes", "States", "TerminalReason", "ResultID", "Counters"}, map[string]any{"Key": fstr(), "GenerationID": fid(), "RequestID": fid(), "AttemptID": fid(), "AssignmentID": fid(), "RawDigest": fdigest(), "RawBytes": fraw(MaxResponseBytes), "States": farr(fenum(AttemptStates[:]...), 1, 3), "TerminalReason": map[string]any{"type": "string"}, "ResultID": map[string]any{"type": "string"}, "Counters": fobj([]string{"Begun", "Completed", "Relations", "Candidates", "WorkCharged"}, map[string]any{"Begun": fint(0, MemberCount), "Completed": fint(0, MemberCount), "Relations": fint(0, MaxRelations), "Candidates": fint(0, MaxCandidates), "WorkCharged": fint(0, MaxWork)})})
	checks := map[string]any{}
	reasons := map[string]any{}
	for _, k := range ReviewChecks {
		checks[k] = fbool()
		reasons[k] = map[string]any{"type": "string", "maxLength": MaxReviewReasonBytes, "x-maxUtf8Bytes": MaxReviewReasonBytes}
	}
	simple := func(fields ...string) map[string]any {
		p := map[string]any{}
		for _, k := range fields {
			p[k] = fstr()
		}
		return fobj(fields, p)
	}
	return map[string]map[string]any{
		"GroupRequest": fpub("GroupRequest", request), "ProducerRaw": fpub("ProducerRaw", fobj([]string{"evidence"}, map[string]any{"evidence": farr(evidence, MemberCount, MemberCount)})), "ProducerAttempt": fpub("ProducerAttempt", attempt),
		"Result":        fpub("Result", simple("SchemaVersion", "ResultID", "RequestID", "GenerationID", "AttemptKey", "Outcome", "CauseCode", "Completeness", "RequestDigest", "RawDigest", "PolicyDigest", "LimitsDigest", "EvidenceDigest", "SearchFreezeIdentity")),
		"ReviewRequest": fpub("ReviewRequest", simple("SchemaVersion", "ReviewRequestID", "GenerationID", "RequestID", "RequestDigest", "ProducerAttemptKey", "ProducerAttemptID", "ProducerAssignmentID", "ProducerAttemptDigest", "RawDigest", "ResultID", "ResultDigest", "SearchDigest", "EvidenceDigest", "PolicyDigest", "LimitsDigest", "ProducerWorkerID", "ProducerTaskID", "ProducerRole", "ReviewerAssignmentID", "ReviewerWorkerID", "ReviewerTaskID", "ReviewerRole", "AdapterChecksDigest")),
		"ReviewerRaw":   fpub("ReviewerRaw", fobj([]string{"checks", "reasons"}, map[string]any{"checks": fobj(ReviewChecks[:], checks), "reasons": fobj([]string{}, reasons)})), "ReviewerAttempt": fpub("ReviewerAttempt", simple("RawDigest", "RawBytes", "TerminalReason", "ReviewID")), "Review": fpub("Review", simple("ReviewID", "ReviewRequestID", "Verdict")), "Account": fpub("Account", fobj([]string{"OutcomeCounts", "Balanced", "SemanticAccepted", "Completeness", "FeatureIdentity", "Authority", "Accepted"}, map[string]any{"OutcomeCounts": counts, "Balanced": fbool(), "SemanticAccepted": map[string]any{"const": false}, "Completeness": map[string]any{"const": "UNKNOWN"}, "FeatureIdentity": map[string]any{"const": "UNRESOLVED"}, "Authority": fint(0, 0), "Accepted": map[string]any{"const": false}})),
		"SearchAccount": fpub("SearchAccount", searchAccount), "SourceRef": fpub("SourceRef", source), "RelationRef": fpub("RelationRef", calls), "CommunityRef": fpub("CommunityRef", community), "MemberEvidence": fpub("MemberEvidence", evidence),
	}
}

func freezeSearch() SearchAccount {
	counts := map[string]int{"RETURNED": 5, "BELOW_THRESHOLD": 19, "FILTERED_BY_POLICY": 0, "DUPLICATE_MEMBER": 0, "INVALID_MEMBER": 0}
	s := SearchAccount{SchemaVersion: SearchSchemaVersion, FreezeIdentity: SearchFreeze, RequestID: "search-request", RequestDigest: SearchCase01Request, ResultID: "search-result", ResultDigest: SearchCase01Result, ReviewID: "search-review", ReviewDigest: Digest([]byte("search-review")), ProducerAttemptID: "search-producer-attempt", ProducerAttemptDigest: SearchCase01Producer, ReviewerAttemptID: "search-reviewer-attempt", ReviewerAttemptDigest: SearchCase01ReviewAttempt, ReviewRequestDigest: SearchCase01ReviewRequest, AccountDigest: SearchCase01Account, CustodyDigest: SearchCase01Custody, ExecutionManifestDigest: SearchExecutionManifest, FinalAuditDigest: SearchFinalAudit, FinalSealDigest: SearchFinalSeal, TerminalReportDigest: SearchTerminalReport, ProducerStates: []string{"RECEIVED", "ADAPTED", "COMMITTED"}, ReviewerStates: []string{"RECEIVED", "ADAPTED", "COMMITTED"}, Denominator: 24, Completeness: "COMPLETE", ReviewerVerdict: "ACCEPT_MECHANICAL", Balanced: true, OutcomeCounts: counts, FeatureIdentity: "UNRESOLVED"}
	for i := 1; i <= 24; i++ {
		o := "BELOW_THRESHOLD"
		if i <= 5 {
			o = "RETURNED"
		}
		s.Members = append(s.Members, SearchMember{Ordinal: i, MemberID: fmt.Sprintf("member-%02d", i), Outcome: o, FeatureIdentity: "UNRESOLVED"})
	}
	return s
}
func freezeRequest() GroupRequest {
	s := freezeSearch()
	r := GroupRequest{SchemaVersion: Version + ".request", RequestID: "group-request", GenerationID: "generation", AttemptID: "attempt-1", AssignmentID: "producer-assignment", SearchFreezeIdentity: SearchFreeze, Search: s, OrderedMembers: append([]SearchMember(nil), s.Members...), PolicyDigest: GroupPolicyDigest, LimitsDigest: GroupLimitsDigest, PartitionCaptureID: "capture", PartitionIdentity: "observation", PartitionSeed: 7, PartitionResolution: "1.0", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Limits: FixedLimits()}
	for i, m := range s.Members {
		c := "community-b"
		if i < 12 {
			c = "community-a"
		}
		e := MemberEvidence{Ordinal: i + 1, MemberID: m.MemberID, Source: &SourceRef{Digest: Digest([]byte(fmt.Sprintf("source-%d", i))), CaptureID: "capture", ObjectID: fmt.Sprintf("object-%02d", i), ObjectDigest: Digest([]byte(fmt.Sprintf("object-%d", i))), Complete: true, RangeCount: 1, UniqueBytes: 10}, Community: &CommunityRef{Digest: Digest([]byte(fmt.Sprintf("community-%d", i))), CaptureID: "capture", ObservationID: "observation", CommunityID: c, MemberID: m.MemberID, Algorithm: "LEIDEN", StructuralOnly: true, Retained: true, Nominated: true, Seed: 7, Resolution: "1.0"}}
		if i < 23 {
			e.Calls = []RelationRef{{Digest: Digest([]byte(fmt.Sprintf("call-%d", i))), CaptureID: "capture", FromMemberID: m.MemberID, ToMemberID: s.Members[i+1].MemberID, Kind: "CALLS", Provenance: "SERVER_REPORTED", ServerReported: true}}
		}
		r.Evidence = append(r.Evidence, e)
	}
	r.EvidenceDigest = Digest(canon(r.Evidence))
	return r
}
func applyFreezeCase(c freezeCase, r *GroupRequest, ctl *Control) {
	switch c.Kind {
	case "unmatched":
		for i := range r.Evidence {
			r.Evidence[i].Community = nil
		}
	case "missing-search":
		r.Search = SearchAccount{}
	case "search-freeze":
		r.Search.FreezeIdentity = Digest([]byte("other"))
	case "search-incomplete":
		r.Search.Completeness = "INCOMPLETE"
	case "search-unbalanced":
		r.Search.Balanced = false
	case "search-authority":
		r.Search.Authority = 1
	case "search-accepted":
		r.Search.Accepted = true
	case "search-feature":
		r.Search.FeatureIdentity = "FEATURE"
	case "source-unavailable":
		r.Evidence[0].Source = nil
	case "source-collision":
		r.Evidence[1].Source.Objects = []SourceObject{{ObjectID: r.Evidence[0].Source.ObjectID, Digest: Digest([]byte("other")), SourceDigest: r.Evidence[0].Source.Digest, CaptureID: "capture", Complete: true}}
	case "source-digest":
		r.Evidence[0].Source.Digest = "bad"
	case "text-calls":
		r.Evidence[0].Calls[0].ServerReported = false
		r.Evidence[0].Calls[0].Provenance = "TEXT"
	case "cross-capture":
		r.Evidence[0].Calls[0].CaptureID = "other"
	case "community-unavailable":
		r.Evidence[0].Community = nil
	case "community-non-leiden":
		r.Evidence[0].Community.Algorithm = "LOUVAIN"
	case "feature-semantic":
		r.Evidence[0].Community.CommunityID = "Feature semantics"
	case "duplicate":
		r.Evidence[1].MemberID = r.Evidence[0].MemberID
	case "invalid":
		r.Evidence[1].Source = nil
	case "overlap":
		r.Evidence[1].Community.CommunityID = r.Evidence[0].Community.CommunityID
	case "filtered":
		r.Evidence[0].Filtered = true
	case "work-boundary":
		base, _ := Adapt(*r, "k", Digest([]byte("raw")), Control{})
		ctl.WorkLimit = base.Counters.WorkCharged
	case "work-plus-one":
		base, _ := Adapt(*r, "k", Digest([]byte("raw")), Control{})
		ctl.WorkLimit = base.Counters.WorkCharged - 1
	case "custody":
		ctl.TriggerCheckpoint = "BEFORE_COMMIT"
		ctl.TriggerCancelled = true
		ctl.TriggerDeadlineExpired = true
	}
	r.EvidenceDigest = Digest(canon(r.Evidence))
}

func GenerateArtifacts(root, packageDir string) (string, error) {
	preserved := map[string][]byte{}
	for _, name := range []string{"DESIGN.md", "SEARCH_BINDING.json", "POLICY.json", "LIMITS.json"} {
		if b, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			preserved[name] = b
		}
	}
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(root, "schemas"), 0755); err != nil {
		return "", err
	}
	for n, s := range freezeSchemas() {
		if err := freezeWrite(filepath.Join(root, "schemas", n+".schema.json"), s); err != nil {
			return "", err
		}
	}
	roots := map[string]any{"REVIEW_POLICY.json": map[string]any{"schemaVersion": Version + ".review-policy", "checks": ReviewChecks, "acceptIffAllChecksTrue": true}, "AUTHORIZATION_CANDIDATE.json": map[string]any{"schemaVersion": Version + ".authorization-candidate", "status": "DESIGN_FROZEN_NON_DISPATCHING", "dispatchAllowed": false, "groupExecuted": false, "groupDesignGO": false}, "PRE_EXECUTION_AUDIT.json": map[string]any{"schemaVersion": Version + ".pre-execution-audit", "status": "BLOCKED", "go": false, "execution": false}, "PREDECESSORS.json": map[string]any{"schemaVersion": Version + ".predecessors", "blocked": []string{"Group execution", "public CLI/MCP", "ADR0011", "candidate-group"}}, "SOURCE_BINDINGS.json": FreezeSourceBindings()}
	for n, v := range roots {
		if err := freezeWrite(filepath.Join(root, n), v); err != nil {
			return "", err
		}
	}
	for name, b := range preserved {
		if err := os.WriteFile(filepath.Join(root, name), b, 0644); err != nil {
			return "", err
		}
	}
	for _, c := range freezeCases {
		if err := generateFreezeCase(root, c); err != nil {
			return "", fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	return GenerateFreeze(root, packageDir)
}
func generateFreezeCase(root string, c freezeCase) error {
	d := filepath.Join(root, "cases", c.ID)
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	baseline := freezeRequest()
	ctl := Control{}
	r := cloneGroupRequest(baseline)
	applyFreezeCase(c, &r, &ctl)
	raw := freezeCanonical(ProducerRaw{Evidence: r.Evidence})
	cond := freezeCondition{Version + ".case-condition", c.ID, c.Kind, ctl}
	exp := freezeExpected{SchemaVersion: Version + ".expected", CaseID: c.ID, MechanicalQualification: "NO_COMMITTED_RESULT", OutcomeCounts: zeroCounts(MemberOutcomes[:])}
	if err := freezeWrite(filepath.Join(d, "CONDITION.json"), cond); err != nil {
		return err
	}
	if err := freezeWrite(filepath.Join(d, "REQUEST.json"), baseline); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "RAW"), freezeCanonical(ProducerRaw{Evidence: baseline.Evidence}), 0644); err != nil {
		return err
	}
	parsed, perr := ParseGroupRequest(freezeCanonical(r))
	if perr != nil {
		exp.Cause = perr.Error()
		exp.NonqualificationReason = exp.Cause
		exp.OperationOutcome = operationForCause(exp.Cause)
		return freezeWrite(filepath.Join(d, "EXPECTED.json"), exp)
	}
	w := NewWriter()
	a, x, err := w.Ingest(parsed, raw, Digest(raw), ctl)
	if e := freezeWrite(filepath.Join(d, "PRODUCER_ATTEMPT.json"), a); e != nil {
		return e
	}
	exp.Counters = a.Counters
	if err != nil {
		exp.Cause = err.Error()
		exp.NonqualificationReason = exp.Cause
		exp.OperationOutcome = operationForCause(exp.Cause)
		if c.Kind == "custody" {
			replay, _, re := w.Ingest(parsed, raw, Digest(raw), ctl)
			exp.ProducerReplay = re == nil && reflect.DeepEqual(replay, a)
			conflict := append([]byte{}, raw...)
			conflict[len(conflict)-2] = ' '
			_, _, ce := w.Ingest(parsed, conflict, Digest(conflict), ctl)
			exp.ProducerConflict = ce != nil && ce.Error() == "CONFLICTING_DELIVERY" && len(w.Conflicts()) == 1
			r2 := parsed
			r2.AttemptID = "attempt-2"
			_, _, se := w.Ingest(r2, raw, Digest(raw), ctl)
			exp.ProducerSecondAttempt = se != nil && se.Error() == "SECOND_ATTEMPT"
			deadlineControl := Control{TriggerCheckpoint: "BEFORE_COMMIT", TriggerDeadlineExpired: true}
			da, _, de := NewWriter().Ingest(parsed, raw, Digest(raw), deadlineControl)
			if de == nil || de.Error() != "DEADLINE_EXPIRED" {
				return errors.New("deadline probe")
			}
			exp.SecondaryCause = de.Error()
			_ = da
			if !exp.ProducerReplay || !exp.ProducerConflict || !exp.ProducerSecondAttempt {
				return errors.New("producer custody")
			}
		}
		return freezeWrite(filepath.Join(d, "EXPECTED.json"), exp)
	}
	exp.Committed = true
	exp.MechanicalQualification = "RESULT_ONLY_ACCOUNT_PERCENT"
	exp.NonqualificationReason = "ACCOUNT_PERCENT"
	exp.OperationOutcome = x.Outcome
	exp.OutcomeCounts = copyInt(x.OutcomeCounts)
	exp.Counters = x.Counters
	exp.Candidates = len(x.Candidates)
	if err := freezeWrite(filepath.Join(d, "RESULT.json"), x); err != nil {
		return err
	}
	q, e := NewReviewRequest(parsed, a, x, "producer-worker", "producer-task", "producer", "review-assignment", "review-worker", "review-task", "reviewer")
	if e != nil {
		return e
	}
	rr := ReviewerRaw{Checks: map[string]bool{}, Reasons: map[string]string{}}
	for _, k := range ReviewChecks {
		rr.Checks[k] = true
	}
	rraw := freezeCanonical(rr)
	key := ReviewerAttemptKey{parsed.GenerationID, q.ReviewerAssignmentID, q.ReviewRequestID, "review-attempt"}
	rw := NewReviewWriter()
	ra, rv, e := rw.Ingest(parsed, a, x, q, key, rraw, Digest(rraw))
	if e != nil {
		return e
	}
	sel := AccountSelection{parsed.GenerationID, parsed.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
	acct, accountErr := MakeAccount(sel, parsed, a, x, q, ra, rv, x.OutcomeCounts)
	reviewArtifacts := map[string]any{"REVIEW_REQUEST.json": q, "REVIEW_RAW.json": rr, "REVIEW_ATTEMPT.json": ra, "REVIEW.json": rv}
	if accountErr == nil {
		exp.Success = true
		exp.MechanicalQualification = "QUALIFIED"
		exp.NonqualificationReason = ""
		reviewArtifacts["ACCOUNT.json"] = acct
	} else if accountErr.Error() != "ACCOUNT_PERCENT" {
		return accountErr
	}
	if c.ID == "01-complete-two-candidates" {
		pa, px, pe := w.Ingest(parsed, raw, Digest(raw), Control{})
		exp.ProducerReplay = pe == nil && reflect.DeepEqual(pa, a) && reflect.DeepEqual(px, x)
		conflict := append([]byte{}, raw...)
		conflict[len(conflict)-2] = ' '
		_, _, pe = w.Ingest(parsed, conflict, Digest(conflict), Control{})
		exp.ProducerConflict = pe != nil && pe.Error() == "CONFLICTING_DELIVERY" && len(w.Conflicts()) == 1
		if exp.ProducerConflict {
			reviewArtifacts["DELIVERY_CONFLICT.json"] = w.Conflicts()[0]
		}
		r2 := parsed
		r2.AttemptID = "attempt-2"
		_, _, pe = w.Ingest(r2, raw, Digest(raw), Control{})
		exp.ProducerSecondAttempt = pe != nil && pe.Error() == "SECOND_ATTEMPT"
		rpa, rpr, re := rw.Ingest(parsed, a, x, q, key, rraw, Digest(rraw))
		exp.ReviewerReplay = re == nil && reflect.DeepEqual(rpa, ra) && reflect.DeepEqual(rpr, rv)
		rconf := append([]byte{}, rraw...)
		rconf[len(rconf)-2] = ' '
		_, _, re = rw.Ingest(parsed, a, x, q, key, rconf, Digest(rconf))
		exp.ReviewerConflict = re != nil && re.Error() == "CONFLICTING_DELIVERY" && len(rw.Conflicts()) == 1
		if exp.ReviewerConflict {
			reviewArtifacts["REVIEWER_CONFLICT.json"] = rw.Conflicts()[0]
		}
		key2 := key
		key2.AttemptID = "review-attempt-2"
		_, _, re = rw.Ingest(parsed, a, x, q, key2, rraw, Digest(rraw))
		exp.ReviewerSecondAttempt = re != nil && re.Error() == "SECOND_ATTEMPT"
		if !exp.ProducerReplay || !exp.ProducerConflict || !exp.ProducerSecondAttempt || !exp.ReviewerReplay || !exp.ReviewerConflict || !exp.ReviewerSecondAttempt {
			return errors.New("custody probes")
		}
	}
	if c.Kind == "feature-semantic" {
		forged := cloneResult(x)
		forged.Candidates[0].WorkingLabel = "Feature semantics"
		forged.ResultID = ""
		forged.ResultID = domainID("group-result", forged)
		fa := cloneAttempt(a)
		fa.ResultID = forged.ResultID
		exp.ForgeryRejected = ValidateResult(parsed, fa, forged) != nil
		if !exp.ForgeryRejected {
			return errors.New("semantic forgery")
		}
	}
	if c.Kind == "overlap" {
		forged := cloneResult(x)
		if len(forged.Candidates) < 2 {
			return errors.New("overlap fixture")
		}
		forged.Candidates[1].MemberOrdinals = append(forged.Candidates[1].MemberOrdinals, forged.Candidates[0].MemberOrdinals[0])
		forged.Candidates[1].MemberIDs = append(forged.Candidates[1].MemberIDs, forged.Candidates[0].MemberIDs[0])
		forged.ResultID = ""
		forged.ResultID = domainID("group-result", forged)
		fa := cloneAttempt(a)
		fa.ResultID = forged.ResultID
		exp.ForgeryRejected = ValidateResult(parsed, fa, forged) != nil
		if !exp.ForgeryRejected {
			return errors.New("overlap forgery")
		}
	}
	for n, v := range reviewArtifacts {
		if e = freezeWrite(filepath.Join(d, n), v); e != nil {
			return e
		}
	}
	return freezeWrite(filepath.Join(d, "EXPECTED.json"), exp)
}
func cloneGroupRequest(r GroupRequest) GroupRequest {
	var out GroupRequest
	_ = json.Unmarshal(canon(r), &out)
	return out
}

func GenerateFreeze(root, packageDir string) (string, error) {
	files, e := freezeTree(root)
	if e != nil {
		return "", e
	}
	sources, e := freezeSources(packageDir)
	if e != nil {
		return "", e
	}
	f := DesignFreeze{Version + ".freeze", "DESIGN_FROZEN_NON_DISPATCHING", "", files, sources, false, false, false, []string{"Group execution", "public CLI/MCP", "ADR0011", "candidate-group"}}
	f.FreezeID = domainFreezeID(f)
	if e = freezeWrite(filepath.Join(root, "FREEZE.json"), f); e != nil {
		return "", e
	}
	return f.FreezeID, nil
}
func domainFreezeID(f DesignFreeze) string {
	h := sha256.New()
	h.Write([]byte(Version + "\x00design-freeze\x00"))
	h.Write(canon(f))
	return "group-freeze-" + hex.EncodeToString(h.Sum(nil))
}
func freezeSources(dir string) ([]FreezeFile, error) {
	names := []string{"freeze.go", "freeze_test.go", "group.go", "group_test.go"}
	out := []FreezeFile{}
	for _, n := range names {
		b, e := os.ReadFile(filepath.Join(dir, n))
		if e != nil {
			return nil, e
		}
		out = append(out, FreezeFile{n, freezeDigest(b), int64(len(b))})
	}
	return out, nil
}
func freezeTree(root string) ([]FreezeFile, error) {
	out := []FreezeFile{}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "FREEZE.json" {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out = append(out, FreezeFile{rel, freezeDigest(b), int64(len(b))})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, e
}
func VerifyFreeze(root string) (string, error) {
	b, e := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if e != nil {
		return "", e
	}
	var f DesignFreeze
	if e = freezeStrict(b, &f); e != nil {
		return "", e
	}
	files, e := freezeTree(root)
	if e != nil {
		return "", e
	}
	x := f
	x.FreezeID = ""
	if f.Status != "DESIGN_FROZEN_NON_DISPATCHING" || f.DispatchAllowed || f.GroupExecuted || f.GroupDesignGO || !reflect.DeepEqual(files, f.Files) || domainFreezeID(x) != f.FreezeID {
		return "", errors.New("freeze mismatch")
	}
	return f.FreezeID, nil
}
func VerifyArtifacts(root string) error {
	if _, e := VerifyFreeze(root); e != nil {
		return e
	}
	if e := verifyPublishedSchemaSet(root); e != nil {
		return e
	}
	if _, e := ValidateAllSchemaArtifacts(root); e != nil {
		return e
	}
	entries, e := os.ReadDir(filepath.Join(root, "cases"))
	if e != nil || len(entries) != 24 {
		return errors.New("case count")
	}
	for _, entry := range entries {
		if e = verifyFreezeCase(filepath.Join(root, "cases", entry.Name())); e != nil {
			return fmt.Errorf("%s: %w", entry.Name(), e)
		}
	}
	return nil
}
func verifyFreezeCase(d string) error {
	var c freezeCondition
	var r GroupRequest
	var exp freezeExpected
	if freezeRead(filepath.Join(d, "CONDITION.json"), &c) != nil || freezeRead(filepath.Join(d, "REQUEST.json"), &r) != nil || freezeRead(filepath.Join(d, "EXPECTED.json"), &exp) != nil {
		return errors.New("parse")
	}
	baselineRaw, e := os.ReadFile(filepath.Join(d, "RAW"))
	if e != nil {
		return e
	}
	if _, e = ParseGroupRequest(freezeCanonical(r)); e != nil {
		return errors.New("invalid persisted baseline")
	}
	var baselineProducer ProducerRaw
	if e = freezeStrict(baselineRaw, &baselineProducer); e != nil || !reflect.DeepEqual(baselineProducer.Evidence, r.Evidence) {
		return errors.New("baseline raw")
	}
	runtime := cloneGroupRequest(r)
	ctl := Control{}
	applyFreezeCase(freezeCase{c.CaseID, c.Kind}, &runtime, &ctl)
	if !reflect.DeepEqual(ctl, c.Control) {
		return errors.New("control mismatch")
	}
	raw := freezeCanonical(ProducerRaw{Evidence: runtime.Evidence})
	parsed, pe := ParseGroupRequest(freezeCanonical(runtime))
	if pe != nil {
		if exp.Committed || exp.Success || exp.Cause != pe.Error() || exp.OperationOutcome != operationForCause(pe.Error()) || exp.MechanicalQualification != "NO_COMMITTED_RESULT" || exp.NonqualificationReason != pe.Error() {
			return errors.New("request expectation")
		}
		return nil
	}
	a, x, ie := NewWriter().Ingest(parsed, raw, Digest(raw), c.Control)
	var fa Attempt
	if freezeRead(filepath.Join(d, "PRODUCER_ATTEMPT.json"), &fa) != nil || !reflect.DeepEqual(a, fa) {
		return errors.New("attempt")
	}
	if ie != nil {
		if exp.Committed || exp.Success || exp.Cause != ie.Error() || exp.OperationOutcome != operationForCause(ie.Error()) || x.ResultID != "" || exp.Counters != a.Counters || exp.MechanicalQualification != "NO_COMMITTED_RESULT" || exp.NonqualificationReason != ie.Error() {
			return errors.New("terminal expectation")
		}
		if _, e = os.Stat(filepath.Join(d, "RESULT.json")); !errors.Is(e, os.ErrNotExist) {
			return errors.New("terminal result")
		}
		if c.Kind == "custody" {
			if exp.SecondaryCause != "DEADLINE_EXPIRED" || !exp.ProducerReplay || !exp.ProducerConflict || !exp.ProducerSecondAttempt {
				return errors.New("custody expectation")
			}
			deadline := Control{TriggerCheckpoint: "BEFORE_COMMIT", TriggerDeadlineExpired: true}
			_, _, de := NewWriter().Ingest(parsed, raw, Digest(raw), deadline)
			if de == nil || de.Error() != exp.SecondaryCause {
				return errors.New("deadline replay")
			}
		}
		return nil
	}
	var fx Result
	if freezeRead(filepath.Join(d, "RESULT.json"), &fx) != nil || !reflect.DeepEqual(x, fx) || ValidateResult(parsed, a, x) != nil {
		return errors.New("result")
	}
	if !exp.Committed || exp.OperationOutcome != x.Outcome || !reflect.DeepEqual(exp.OutcomeCounts, x.OutcomeCounts) || exp.Counters != x.Counters || exp.Candidates != len(x.Candidates) {
		return errors.New("result expectation")
	}
	var q ReviewRequest
	var rr ReviewerRaw
	var fra ReviewerAttempt
	var fr Review
	var acct Account
	if freezeRead(filepath.Join(d, "REVIEW_REQUEST.json"), &q) != nil || freezeRead(filepath.Join(d, "REVIEW_RAW.json"), &rr) != nil || freezeRead(filepath.Join(d, "REVIEW_ATTEMPT.json"), &fra) != nil || freezeRead(filepath.Join(d, "REVIEW.json"), &fr) != nil {
		return errors.New("review parse")
	}
	rraw := freezeCanonical(rr)
	rw := NewReviewWriter()
	ra, rv, e := rw.Ingest(parsed, a, x, q, fra.Key, rraw, Digest(rraw))
	if e != nil || !reflect.DeepEqual(ra, fra) || !reflect.DeepEqual(rv, fr) {
		return errors.New("review")
	}
	accountPath := filepath.Join(d, "ACCOUNT.json")
	if exp.Success {
		if exp.MechanicalQualification != "QUALIFIED" || freezeRead(accountPath, &acct) != nil {
			return errors.New("account parse")
		}
		got, e := MakeAccount(acct.Selection, parsed, a, x, q, ra, rv, x.OutcomeCounts)
		if e != nil || !reflect.DeepEqual(got, acct) {
			return errors.New("account")
		}
	} else {
		if exp.MechanicalQualification != "RESULT_ONLY_ACCOUNT_PERCENT" || exp.NonqualificationReason != "ACCOUNT_PERCENT" {
			return errors.New("nonqualification label")
		}
		if _, e := os.Stat(accountPath); !errors.Is(e, os.ErrNotExist) {
			return errors.New("unqualified account")
		}
		sel := AccountSelection{parsed.GenerationID, parsed.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
		if _, e := MakeAccount(sel, parsed, a, x, q, ra, rv, x.OutcomeCounts); e == nil || e.Error() != "ACCOUNT_PERCENT" {
			return errors.New("nonqualification reason")
		}
	}
	if c.CaseID == "01-complete-two-candidates" {
		if !exp.ProducerReplay || !exp.ProducerConflict || !exp.ProducerSecondAttempt || !exp.ReviewerReplay || !exp.ReviewerConflict || !exp.ReviewerSecondAttempt {
			return errors.New("custody expected flags")
		}
		var pc DeliveryConflict
		var rc ReviewDeliveryConflict
		if freezeRead(filepath.Join(d, "DELIVERY_CONFLICT.json"), &pc) != nil || freezeRead(filepath.Join(d, "REVIEWER_CONFLICT.json"), &rc) != nil {
			return errors.New("conflict records")
		}
		if pc != NewWriterConflictProbe(parsed, raw) || rc != NewReviewerConflictProbe(parsed, a, x, q, fra.Key, rraw) {
			return errors.New("conflict custody")
		}
	}
	if (c.Kind == "feature-semantic" || c.Kind == "overlap") && !exp.ForgeryRejected {
		return errors.New("forgery expectation")
	}
	return nil
}
func NewWriterConflictProbe(r GroupRequest, raw []byte) DeliveryConflict {
	w := NewWriter()
	_, _, _ = w.Ingest(r, raw, Digest(raw), Control{})
	conflict := append([]byte{}, raw...)
	conflict[len(conflict)-2] = ' '
	_, _, _ = w.Ingest(r, conflict, Digest(conflict), Control{})
	return w.Conflicts()[0]
}
func NewReviewerConflictProbe(r GroupRequest, a Attempt, x Result, q ReviewRequest, key ReviewerAttemptKey, raw []byte) ReviewDeliveryConflict {
	w := NewReviewWriter()
	_, _, _ = w.Ingest(r, a, x, q, key, raw, Digest(raw))
	conflict := append([]byte{}, raw...)
	conflict[len(conflict)-2] = ' '
	_, _, _ = w.Ingest(r, a, x, q, key, conflict, Digest(conflict))
	return w.Conflicts()[0]
}

func freezeRead(path string, dst any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return freezeStrict(b, dst)
}
func freezeStrict(b []byte, dst any) error {
	if !json.Valid(bytes.TrimSuffix(b, []byte{'\n'})) || bytes.Count(b, []byte{'\n'}) != 1 || len(b) == 0 || b[len(b)-1] != '\n' {
		return errors.New("strict")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e := dec.Decode(dst); e != nil {
		return e
	}
	var x any
	if dec.Decode(&x) == nil {
		return errors.New("trailing")
	}
	return nil
}
func VerifyRawBase64(encoded string, max int) error {
	if len(encoded) > ((max+2)/3)*4 {
		return errors.New("encoded size")
	}
	b, e := base64.StdEncoding.Strict().DecodeString(encoded)
	if e != nil || len(b) > max || base64.StdEncoding.EncodeToString(b) != encoded {
		return errors.New("base64")
	}
	return nil
}
func MakeReadOnly(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return os.Chmod(p, 0555)
		}
		return os.Chmod(p, 0444)
	})
}
func FreezeSourceBindings() map[string]string {
	return map[string]string{"integratedMainTree": "e1407f77353b0ccfff13e693570f0e3fc472e579", "describeIntegrationCommit": "404404127053229c8bd154d7e0fa15cf66485186", "searchIntegrationCommit": "4bc75e97d845401bd0090535c1f9cb3d1bf97229", "searchFreezeIdentity": SearchFreeze, "searchFreezeCommit": "b1207bdd52463dcc81fd0c47eccae0452630cf0e96d2c4d72991d99a4c4fe301", "groupDesignCommit": "c31ae0fc", "groupCoreCommit": "4c674da5c1cf212d38d37b4a18df03bedb7a2de4", "searchCase01Request": SearchCase01Request, "searchCase01Result": SearchCase01Result, "searchCase01Producer": SearchCase01Producer, "searchCase01ReviewRequest": SearchCase01ReviewRequest, "searchCase01ReviewAttempt": SearchCase01ReviewAttempt, "searchCase01Account": SearchCase01Account, "searchCase01Custody": SearchCase01Custody}
}
func FreezeSchemas() map[string]map[string]any { return freezeSchemas() }

func projectionSamples() (map[string]any, error) {
	r := freezeRequest()
	raw := freezeCanonical(ProducerRaw{Evidence: r.Evidence})
	w := NewWriter()
	a, x, e := w.Ingest(r, raw, Digest(raw), Control{})
	if e != nil {
		return nil, e
	}
	conflicting := append([]byte{}, raw...)
	conflicting[len(conflicting)-2] = ' '
	if _, _, e = w.Ingest(r, conflicting, Digest(conflicting), Control{}); e == nil {
		return nil, errors.New("producer conflict sample")
	}
	pc := w.Conflicts()[0]
	q, e := NewReviewRequest(r, a, x, "producer-worker", "producer-task", "producer", "review-assignment", "review-worker", "review-task", "reviewer")
	if e != nil {
		return nil, e
	}
	rr := ReviewerRaw{Checks: map[string]bool{}, Reasons: map[string]string{}}
	for _, k := range ReviewChecks {
		rr.Checks[k] = true
	}
	rraw := freezeCanonical(rr)
	key := ReviewerAttemptKey{r.GenerationID, q.ReviewerAssignmentID, q.ReviewRequestID, "review-attempt"}
	rw := NewReviewWriter()
	ra, rv, e := rw.Ingest(r, a, x, q, key, rraw, Digest(rraw))
	if e != nil {
		return nil, e
	}
	rconf := append([]byte{}, rraw...)
	rconf[len(rconf)-2] = ' '
	if _, _, e = rw.Ingest(r, a, x, q, key, rconf, Digest(rconf)); e == nil {
		return nil, errors.New("review conflict sample")
	}
	rc := rw.Conflicts()[0]
	sel := AccountSelection{r.GenerationID, r.RequestID, a.Key, x.ResultID, ra.Key, rv.ReviewID}
	acct, e := MakeAccount(sel, r, a, x, q, ra, rv, x.OutcomeCounts)
	if e != nil {
		return nil, e
	}
	return map[string]any{"GroupRequest": r, "SearchAccount": r.Search, "SearchMember": r.Search.Members[0], "Limits": r.Limits, "SourceObject": SourceObject{ObjectID: "sample-object", Digest: Digest([]byte("object")), SourceDigest: Digest([]byte("source")), CaptureID: r.PartitionCaptureID, Complete: true}, "SourceRef": *r.Evidence[0].Source, "RelationRef": r.Evidence[0].Calls[0], "CommunityRef": *r.Evidence[0].Community, "MemberEvidence": r.Evidence[0], "ProducerRaw": ProducerRaw{Evidence: r.Evidence}, "ProducerAttempt": a, "Result": x, "LedgerEntry": x.Ledger[0], "Candidate": x.Candidates[0], "Counters": x.Counters, "DeliveryConflict": pc, "ReviewRequest": q, "ReviewerRaw": rr, "ReviewerAttempt": ra, "ReviewerAttemptKey": ra.Key, "Review": rv, "ReviewerConflict": rc, "Account": acct, "AccountSelection": sel, "PercentageBreakdown": acct.Mechanical}, nil
}

var schemaOwnedFiles = map[string]string{"REQUEST.json": "GroupRequest", "RAW": "ProducerRaw", "PRODUCER_ATTEMPT.json": "ProducerAttempt", "RESULT.json": "Result", "DELIVERY_CONFLICT.json": "DeliveryConflict", "REVIEW_REQUEST.json": "ReviewRequest", "REVIEW_RAW.json": "ReviewerRaw", "REVIEW_ATTEMPT.json": "ReviewerAttempt", "REVIEW.json": "Review", "REVIEWER_CONFLICT.json": "ReviewerConflict", "ACCOUNT.json": "Account"}

func verifyPublishedSchemaSet(root string) error {
	schemas := freezeSchemas()
	for name, want := range schemas {
		var got map[string]any
		if e := freezeRead(filepath.Join(root, "schemas", name+".schema.json"), &got); e != nil {
			return fmt.Errorf("schema %s: %w", name, e)
		}
		var normalized map[string]any
		b, _ := json.Marshal(want)
		_ = json.Unmarshal(b, &normalized)
		if !reflect.DeepEqual(got, normalized) {
			return fmt.Errorf("schema %s mismatch", name)
		}
		if e := schemaClosed(name, got); e != nil {
			return e
		}
	}
	samples, e := projectionSamples()
	if e != nil {
		return e
	}
	for name, sample := range samples {
		if e = validateProjection(schemas[name], sample); e != nil {
			return fmt.Errorf("schema %s rejects runtime projection: %w", name, e)
		}
	}
	return nil
}
func schemaClosed(path string, s map[string]any) error {
	if s["type"] == "object" {
		if s["additionalProperties"] != false {
			return fmt.Errorf("open schema %s", path)
		}
		props, _ := s["properties"].(map[string]any)
		for n, v := range props {
			if e := schemaClosed(path+"."+n, v.(map[string]any)); e != nil {
				return e
			}
		}
	}
	if s["type"] == "array" {
		return schemaClosed(path+"[]", s["items"].(map[string]any))
	}
	for _, key := range []string{"anyOf", "allOf"} {
		if xs, ok := s[key].([]any); ok {
			for i, v := range xs {
				if e := schemaClosed(fmt.Sprintf("%s.%s[%d]", path, key, i), v.(map[string]any)); e != nil {
					return e
				}
			}
		}
	}
	for _, key := range []string{"if", "then", "else"} {
		if v, ok := s[key].(map[string]any); ok {
			if e := schemaClosed(path+"."+key, v); e != nil {
				return e
			}
		}
	}
	return nil
}

func ValidateAllSchemaArtifacts(root string) (map[string]int, error) {
	schemas := freezeSchemas()
	counts := map[string]int{}
	entries, e := os.ReadDir(filepath.Join(root, "cases"))
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		dir := filepath.Join(root, "cases", entry.Name())
		for file, name := range schemaOwnedFiles {
			path := filepath.Join(dir, file)
			b, e := os.ReadFile(path)
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return nil, e
			}
			value, e := decodeCanonicalValue(b)
			if e != nil {
				return nil, fmt.Errorf("%s/%s: %w", entry.Name(), file, e)
			}
			if e = schemaValidateValue(schemas[name], value); e != nil {
				return nil, fmt.Errorf("%s/%s schema %s: %w", entry.Name(), file, name, e)
			}
			counts[name]++
		}
	}
	return counts, nil
}
func decodeCanonicalValue(b []byte) (any, error) {
	if len(b) == 0 || b[len(b)-1] != '\n' || bytes.Count(b, []byte{'\n'}) != 1 || !utf8.Valid(b) {
		return nil, errors.New("canonical frame")
	}
	line := b[:len(b)-1]
	if e := rejectDuplicateKeys(line); e != nil {
		return nil, e
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(line))
	if e := dec.Decode(&v); e != nil {
		return nil, e
	}
	return v, nil
}

var persistedProjectionTypes = map[string]reflect.Type{
	"GroupRequest": reflect.TypeOf(GroupRequest{}), "SearchAccount": reflect.TypeOf(SearchAccount{}),
	"SearchMember": reflect.TypeOf(SearchMember{}), "Limits": reflect.TypeOf(Limits{}),
	"SourceObject": reflect.TypeOf(SourceObject{}), "SourceRef": reflect.TypeOf(SourceRef{}),
	"RelationRef": reflect.TypeOf(RelationRef{}), "CommunityRef": reflect.TypeOf(CommunityRef{}),
	"MemberEvidence": reflect.TypeOf(MemberEvidence{}), "ProducerRaw": reflect.TypeOf(ProducerRaw{}),
	"ProducerAttempt": reflect.TypeOf(Attempt{}), "Result": reflect.TypeOf(Result{}),
	"LedgerEntry": reflect.TypeOf(LedgerEntry{}), "Candidate": reflect.TypeOf(Candidate{}),
	"Counters": reflect.TypeOf(Counters{}), "DeliveryConflict": reflect.TypeOf(DeliveryConflict{}),
	"ReviewRequest": reflect.TypeOf(ReviewRequest{}), "ReviewerRaw": reflect.TypeOf(ReviewerRaw{}),
	"ReviewerAttempt": reflect.TypeOf(ReviewerAttempt{}), "ReviewerAttemptKey": reflect.TypeOf(ReviewerAttemptKey{}),
	"Review": reflect.TypeOf(Review{}), "ReviewerConflict": reflect.TypeOf(ReviewDeliveryConflict{}),
	"Account": reflect.TypeOf(Account{}), "AccountSelection": reflect.TypeOf(AccountSelection{}),
	"PercentageBreakdown": reflect.TypeOf(PercentageBreakdown{}),
}

func freezeSchemas() map[string]map[string]any {
	out := make(map[string]map[string]any, len(persistedProjectionTypes))
	for name, typ := range persistedProjectionTypes {
		out[name] = fpub(name, projectionSchema(typ, name))
	}
	producer := out["ProducerAttempt"]
	producer["allOf"] = []any{
		attemptStateConditional(producer, "ResultID"),
		map[string]any{"if": closedConditional(producer, []string{"TerminalReason"}, map[string]any{"TerminalReason": map[string]any{"const": "RAW_SIZE"}}), "then": closedConditional(producer, nil, map[string]any{"RawBytes": rawExact(MaxResponseBytes + 1)}), "else": closedConditional(producer, nil, map[string]any{"RawBytes": fraw(MaxResponseBytes)})},
	}
	reviewer := out["ReviewerAttempt"]
	reviewer["allOf"] = []any{attemptStateConditional(reviewer, "ReviewID")}
	resultProps := out["Result"]["properties"].(map[string]any)
	resultProps["Outcome"].(map[string]any)["const"] = "COMPLETE"
	resultProps["CauseCode"].(map[string]any)["const"] = ""
	delete(resultProps["CauseCode"].(map[string]any), "minLength")
	return out
}

func attemptStateConditional(base map[string]any, idField string) map[string]any {
	return map[string]any{"if": closedConditional(base, []string{"TerminalReason"}, map[string]any{"TerminalReason": map[string]any{"const": ""}}), "then": closedConditional(base, nil, map[string]any{"States": map[string]any{"const": []any{"RECEIVED", "ADAPTED", "COMMITTED"}}, idField: fid()}), "else": closedConditional(base, nil, map[string]any{"States": map[string]any{"anyOf": []any{map[string]any{"const": []any{"RECEIVED", "TERMINAL_INVALID"}}, map[string]any{"const": []any{"RECEIVED", "ADAPTED", "TERMINAL_INVALID"}}}}, idField: map[string]any{"const": ""}})}
}

func closedConditional(base map[string]any, required []string, overrides map[string]any) map[string]any {
	props := map[string]any{}
	for name, schema := range base["properties"].(map[string]any) {
		props[name] = cloneSchema(schema.(map[string]any))
	}
	for name, schema := range overrides {
		props[name] = schema
	}
	return fobj(required, props)
}

func projectionSchema(t reflect.Type, path string) map[string]any {
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{projectionSchema(t.Elem(), path), map[string]any{"type": "null"}}}
	}
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		max := MaxResponseBytes
		if strings.HasPrefix(path, "ProducerAttempt.") {
			max++
		}
		return fraw(max)
	}
	var s map[string]any
	switch t.Kind() {
	case reflect.Struct:
		required, props := make([]string, 0, t.NumField()), map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := f.Name
			if tag := f.Tag.Get("json"); tag != "" {
				if n := strings.Split(tag, ",")[0]; n != "" && n != "-" {
					name = n
				}
			}
			required = append(required, name)
			props[name] = projectionSchema(f.Type, path+"."+name)
		}
		s = fobj(required, props)
	case reflect.Slice:
		item := projectionSchema(t.Elem(), path+"[]")
		if strings.HasSuffix(path, "States") {
			item = fenum(AttemptStates[:]...)
		}
		s = farr(item, 0, freezeArrayMax(path))
	case reflect.Map:
		s = projectionMapSchema(path)
	case reflect.String:
		s = map[string]any{"type": "string"}
	case reflect.Bool:
		s = fbool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s = map[string]any{"type": "integer"}
	default:
		panic("unsupported projection type " + t.String())
	}
	applyProjectionConstraints(s, path)
	if s["type"] == "array" {
		if min, ok := schemaInt(s["minItems"]); ok && min == 0 {
			return map[string]any{"anyOf": []any{s, map[string]any{"type": "null"}}}
		}
	}
	return s
}

func projectionMapSchema(path string) map[string]any {
	keys := []string{}
	value := fint(0, MemberCount)
	switch {
	case strings.HasSuffix(path, ".OutcomeCounts") && (strings.HasPrefix(path, "SearchAccount") || strings.Contains(path, ".Search.OutcomeCounts")):
		keys = append(keys, SearchOutcomes[:]...)
	case strings.HasSuffix(path, ".OutcomeCounts"):
		keys = append(keys, MemberOutcomes[:]...)
	case strings.HasSuffix(path, ".AdapterChecks"), strings.HasSuffix(path, ".Checks"), strings.HasSuffix(path, ".checks"):
		keys = append(keys, ReviewChecks[:]...)
		value = fbool()
	case strings.HasSuffix(path, ".Reasons"), strings.HasSuffix(path, ".reasons"):
		props := map[string]any{}
		for _, k := range ReviewChecks {
			props[k] = map[string]any{"type": "string", "maxLength": MaxReviewReasonBytes, "x-maxUtf8Bytes": MaxReviewReasonBytes}
		}
		return fobj([]string{}, props)
	default:
		panic("unowned persisted map " + path)
	}
	props := map[string]any{}
	for _, k := range keys {
		props[k] = cloneSchema(value)
	}
	return fobj(keys, props)
}

func cloneSchema(s map[string]any) map[string]any {
	b, _ := json.Marshal(s)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func freezeArrayMax(path string) int {
	switch {
	case strings.HasSuffix(path, ".Members"), strings.HasSuffix(path, ".OrderedMembers"), strings.HasSuffix(path, ".Evidence"), strings.HasSuffix(path, ".Ledger"):
		return MemberCount
	case strings.HasSuffix(path, ".Candidates"):
		return MaxCandidates
	case strings.HasSuffix(path, ".MemberOrdinals"), strings.HasSuffix(path, ".MemberIDs"):
		return MaxMembersPerCandidate
	case strings.HasSuffix(path, ".Calls"):
		return MaxRelations
	case strings.HasSuffix(path, ".Objects"):
		return MaxSourceObjects
	case strings.HasSuffix(path, ".States"), strings.HasSuffix(path, "ProducerStates"), strings.HasSuffix(path, "ReviewerStates"):
		return 3
	default:
		return 4096
	}
}

func applyProjectionConstraints(s map[string]any, path string) {
	field := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		field = path[i+1:]
	}
	if s["type"] == "string" {
		s["minLength"] = 1
		if strings.Contains(field, "Digest") || field == "FreezeIdentity" {
			replaceSchema(s, fdigest())
		}
		if isIdentityField(field) {
			replaceSchema(s, fid())
		}
		switch field {
		case "SchemaVersion":
			if strings.Contains(path, ".Search.SchemaVersion") {
				s["const"] = SearchSchemaVersion
				break
			}
			switch strings.Split(path, ".")[0] {
			case "GroupRequest":
				s["const"] = Version + ".request"
			case "SearchAccount":
				s["const"] = SearchSchemaVersion
			case "Result":
				s["const"] = Version + ".result"
			case "ReviewRequest":
				s["const"] = Version + ".review-request"
			}
		case "FreezeIdentity", "SearchFreezeIdentity":
			s["const"] = SearchFreeze
		case "PolicyDigest":
			s["const"] = GroupPolicyDigest
		case "LimitsDigest":
			s["const"] = GroupLimitsDigest
		case "ExecutionManifestDigest":
			s["const"] = SearchExecutionManifest
		case "FinalAuditDigest":
			s["const"] = SearchFinalAudit
		case "FinalSealDigest":
			s["const"] = SearchFinalSeal
		case "TerminalReportDigest":
			s["const"] = SearchTerminalReport
		case "Outcome":
			if strings.HasPrefix(path, "SearchMember.") || strings.Contains(path, ".Members[].Outcome") || strings.Contains(path, ".OrderedMembers[].Outcome") {
				s["enum"] = SearchOutcomes[:]
			} else if strings.HasPrefix(path, "LedgerEntry.") || strings.Contains(path, ".Ledger[].Outcome") {
				s["enum"] = MemberOutcomes[:]
			} else {
				s["enum"] = OperationOutcomes[:]
			}
		case "CauseCode":
			s["enum"] = append([]string{""}, CauseCodes[:]...)
			s["minLength"] = 0
		case "Completeness":
			if strings.HasPrefix(path, "SearchAccount.") || strings.Contains(path, ".Search.Completeness") {
				s["const"] = "COMPLETE"
			} else {
				s["const"] = "UNKNOWN"
			}
		case "FeatureIdentity":
			s["const"] = "UNRESOLVED"
		case "ReviewerVerdict":
			s["const"] = "ACCEPT_MECHANICAL"
		case "Verdict":
			s["enum"] = []string{"ACCEPT_MECHANICAL_DESIGN", "REJECT"}
		case "Kind":
			s["const"] = "CALLS"
		case "Provenance":
			s["const"] = "SERVER_REPORTED"
		case "Algorithm":
			s["const"] = "LEIDEN"
		case "States":
			s["enum"] = AttemptStates[:]
		case "TerminalReason":
			s["enum"] = append([]string{""}, CauseCodes[:]...)
			s["minLength"] = 0
		case "ResultID":
			if strings.HasPrefix(path, "ProducerAttempt.") {
				allowEmptyIdentity(s)
			}
		case "ReviewID":
			if strings.HasPrefix(path, "ReviewerAttempt.") {
				allowEmptyIdentity(s)
			}
		case "CandidateID":
			if strings.Contains(path, ".Ledger[]") || strings.HasPrefix(path, "LedgerEntry.") {
				allowEmptyIdentity(s)
			}
		case "RawBytes": // handled as []byte before this function
		}
	}
	if s["type"] == "integer" {
		min, max := 0, MaxWork
		switch field {
		case "Ordinal":
			min, max = 1, MemberCount
		case "Denominator":
			if strings.HasPrefix(path, "PercentageBreakdown.") || isPercentagePath(path) {
				min, max = 1, 100
			} else {
				min, max = MemberCount, MemberCount
			}
		case "Authority":
			min, max = 0, 0
		case "Begun", "Completed":
			max = MemberCount
		case "Relations":
			max = MaxRelations
		case "Candidates":
			max = MaxCandidates
		case "WorkCharged":
			max = MaxWork
		case "RangeCount":
			max = MaxSourceRanges
		case "UniqueBytes":
			max = MaxUniqueSourceBytes
		case "Numerator", "Percent":
			max = 100
		case "Seed", "PartitionSeed":
			delete(s, "minimum")
			delete(s, "maximum")
			return
		case "MaxRequestBytes":
			min, max = MaxRequestBytes, MaxRequestBytes
		case "MaxResponseBytes":
			min, max = MaxResponseBytes, MaxResponseBytes
		case "MaxIdentityBytes":
			min, max = MaxIdentityBytes, MaxIdentityBytes
		case "MaxMembers":
			min, max = MemberCount, MemberCount
		case "MaxRelations":
			min, max = MaxRelations, MaxRelations
		case "MaxCandidates":
			min, max = MaxCandidates, MaxCandidates
		case "MaxMembersPerCandidate":
			min, max = MaxMembersPerCandidate, MaxMembersPerCandidate
		case "MaxCandidateOccurrences":
			min, max = MaxCandidateOccurrences, MaxCandidateOccurrences
		case "MaxEvidenceRefsPerMember":
			min, max = MaxEvidenceRefsPerMember, MaxEvidenceRefsPerMember
		case "MaxSourceObjects":
			min, max = MaxSourceObjects, MaxSourceObjects
		case "MaxSourceRanges":
			min, max = MaxSourceRanges, MaxSourceRanges
		case "MaxUniqueSourceBytes":
			min, max = MaxUniqueSourceBytes, MaxUniqueSourceBytes
		case "MaxWork":
			min, max = MaxWork, MaxWork
		}
		s["minimum"], s["maximum"] = min, max
	}
	if s["type"] == "boolean" {
		switch field {
		case "Accepted", "SemanticAccepted":
			s["const"] = false
		case "ServerReported", "Complete", "StructuralOnly", "Retained":
			s["const"] = true
		}
	}
	if s["type"] == "array" {
		switch field {
		case "Members", "OrderedMembers", "Evidence", "Ledger":
			s["minItems"], s["maxItems"] = MemberCount, MemberCount
		case "MemberOrdinals", "MemberIDs":
			s["minItems"], s["maxItems"] = 1, MaxMembersPerCandidate
		case "ProducerStates", "ReviewerStates":
			s["minItems"], s["maxItems"] = 3, 3
		case "States":
			s["minItems"], s["maxItems"] = 1, 3
		}
	}
}

func isPercentagePath(path string) bool {
	for _, name := range []string{"Mechanical", "Custody", "Accounting", "Evidence", "Ceiling"} {
		if strings.Contains(path, "."+name+".") {
			return true
		}
	}
	return false
}

func isIdentityField(field string) bool {
	if strings.Contains(field, "Digest") || field == "FreezeIdentity" || field == "SearchFreezeIdentity" || field == "SchemaVersion" || field == "Outcome" || field == "CauseCode" || field == "Completeness" || field == "FeatureIdentity" || field == "TerminalReason" || field == "Verdict" || field == "Kind" || field == "Provenance" || field == "Algorithm" || field == "Resolution" {
		return false
	}
	return strings.HasSuffix(field, "ID") || strings.HasSuffix(field, "Key") || strings.HasSuffix(field, "Role") || field == "WorkingLabel"
}
func allowEmptyIdentity(s map[string]any) {
	s["minLength"] = 0
	delete(s, "pattern")
	delete(s, "x-noSurroundingWhitespace")
}

func rawExact(n int) map[string]any {
	s := fraw(n)
	s["minLength"] = ((n + 2) / 3) * 4
	s["x-decodedBytesConst"] = n
	delete(s, "x-maxDecodedBytes")
	return s
}

func replaceSchema(dst, src map[string]any) {
	for k := range dst {
		delete(dst, k)
	}
	for k, v := range src {
		dst[k] = v
	}
}

func schemaValidateValue(schema map[string]any, value any) error {
	if all, ok := schema["allOf"].([]any); ok {
		for _, item := range all {
			if e := schemaValidateValue(item.(map[string]any), value); e != nil {
				return e
			}
		}
	}
	if condition, ok := schema["if"].(map[string]any); ok {
		branch, _ := schema["else"].(map[string]any)
		if schemaValidateValue(condition, value) == nil {
			branch, _ = schema["then"].(map[string]any)
		}
		if branch != nil {
			if e := schemaValidateValue(branch, value); e != nil {
				return e
			}
		}
	}
	if variants, ok := schema["anyOf"].([]any); ok {
		reasons := make([]string, 0, len(variants))
		for _, v := range variants {
			if e := schemaValidateValue(v.(map[string]any), value); e == nil {
				return nil
			} else {
				reasons = append(reasons, e.Error())
			}
		}
		return fmt.Errorf("anyOf(%s)", strings.Join(reasons, " | "))
	}
	typ, _ := schema["type"].(string)
	switch typ {
	case "null":
		if value != nil {
			return errors.New("type")
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return errors.New("type")
		}
		props, _ := schema["properties"].(map[string]any)
		for _, n := range stringSlice(schema["required"]) {
			if _, ok := obj[n]; !ok {
				return fmt.Errorf("required %s", n)
			}
		}
		if schema["additionalProperties"] == false {
			for n := range obj {
				if _, ok := props[n]; !ok {
					return fmt.Errorf("unknown %s", n)
				}
			}
		}
		for n, v := range obj {
			if child, ok := props[n].(map[string]any); ok {
				if e := schemaValidateValue(child, v); e != nil {
					return fmt.Errorf("%s: %w", n, e)
				}
			}
		}
	case "array":
		a, ok := value.([]any)
		if !ok {
			return errors.New("type")
		}
		if n, ok := schemaInt(schema["minItems"]); ok && len(a) < n {
			return errors.New("minItems")
		}
		if n, ok := schemaInt(schema["maxItems"]); ok && len(a) > n {
			return errors.New("maxItems")
		}
		for _, v := range a {
			if e := schemaValidateValue(schema["items"].(map[string]any), v); e != nil {
				return e
			}
		}
	case "string":
		v, ok := value.(string)
		if !ok {
			return errors.New("type")
		}
		if n, ok := schemaInt(schema["minLength"]); ok && len([]rune(v)) < n {
			return errors.New("minLength")
		}
		if n, ok := schemaInt(schema["maxLength"]); ok && len([]rune(v)) > n {
			return errors.New("maxLength")
		}
		if n, ok := schemaInt(schema["x-maxUtf8Bytes"]); ok && len([]byte(v)) > n {
			return errors.New("utf8 bytes")
		}
		if p, ok := schema["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(v) {
			return errors.New("pattern")
		}
		if n, ok := schemaInt(schema["x-maxDecodedBytes"]); ok {
			if e := VerifyRawBase64(v, n); e != nil {
				return e
			}
		}
		if n, ok := schemaInt(schema["x-decodedBytesConst"]); ok {
			decoded, e := base64.StdEncoding.Strict().DecodeString(v)
			if e != nil || len(decoded) != n || base64.StdEncoding.EncodeToString(decoded) != v {
				return errors.New("decoded const")
			}
		}
		if exact, ok := schema["x-noSurroundingWhitespace"].(bool); ok && exact && strings.TrimSpace(v) != v {
			return errors.New("surrounding whitespace")
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != float64(int64(n)) {
			return errors.New("type")
		}
		if x, ok := schemaInt(schema["minimum"]); ok && n < float64(x) {
			return errors.New("minimum")
		}
		if x, ok := schemaInt(schema["maximum"]); ok && n > float64(x) {
			return errors.New("maximum")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("type")
		}
	}
	if c, ok := schema["const"]; ok && !reflect.DeepEqual(c, value) {
		return errors.New("const")
	}
	if raw, ok := schema["enum"]; ok {
		found := false
		switch es := raw.(type) {
		case []string:
			for _, x := range es {
				found = found || x == value
			}
		case []any:
			for _, x := range es {
				found = found || reflect.DeepEqual(x, value)
			}
		}
		if !found {
			return errors.New("enum")
		}
	}
	return nil
}
func schemaInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), n == float64(int(n))
	default:
		return 0, false
	}
}
func stringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		r := make([]string, len(x))
		for i, v := range x {
			r[i], _ = v.(string)
		}
		return r
	}
	return nil
}
func validateProjection(schema map[string]any, v any) error {
	var decoded any
	b := freezeCanonical(v)
	if e := json.Unmarshal(b, &decoded); e != nil {
		return e
	}
	return schemaValidateValue(schema, decoded)
}

var _ = strings.Builder{}
