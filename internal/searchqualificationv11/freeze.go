package searchqualificationv11

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
	"sort"
	"strings"
)

type FileIdentity struct {
	Path, Digest string
	Bytes        int64
}
type Freeze struct {
	SchemaVersion, Status, FreezeID                 string
	Files, Sources                                  []FileIdentity
	DispatchAllowed, SearchExecuted, SearchDesignGO bool
	BlockedPredecessors                             []string
}

type sourceRequest struct {
	SchemaVersion   string `json:"schema_version"`
	RequestID       string `json:"request_id"`
	PreparationID   string `json:"preparation_id"`
	CaseID          string `json:"case_id"`
	LaneID          string `json:"lane_id"`
	AdmissionID     string `json:"admission_id"`
	SourceRevision  string `json:"source_revision"`
	EvidenceDigest  string `json:"evidence_digest"`
	Prompt          string `json:"prompt"`
	Authority       int    `json:"authority"`
	Accepted        bool   `json:"accepted"`
	Completeness    string `json:"completeness"`
	FeatureIdentity string `json:"feature_identity"`
}
type sourceAttempt struct {
	Key                                                                 struct{ GenerationID, AssignmentID, Role, AttemptID string }
	RequestDigest, RawResponseDigest, RawResultDigest, State, ReceiptID string
	History                                                             []string
	RawResult, Record                                                   json.RawMessage
}
type describeRecord struct {
	Generation, Case, Lane, Preparation, Request, RequestDigest, Attempt, RawResultDigest, RawResponseDigest string
	Receipt, Evidence, Source, State                                                                         string
	History                                                                                                  []string
	Record, RawResult                                                                                        json.RawMessage
	SemanticPayloadDigest                                                                                    string
}
type describeArtifact struct {
	SchemaVersion, AttemptsSourceDigest, RequestsSourceDigest, PolicyDigest, EvaluationDigest string
	Cases, Attempts                                                                           int
	Records                                                                                   []describeRecord
}
type caseSpec struct{ ID, Kind string }
type caseCondition struct {
	SchemaVersion, CaseID, Action, Query, IndexIdentity, DescribeMutation string
	Admission                                                             *AdmissionState
}
type expectedFixture struct {
	SchemaVersion, CaseID, Cause, OperationOutcome, Completeness, Nonqualification, ConflictCause, ConflictOutcome string
	Denominator, Begun, Completed, Unevaluated, Work, Selected                                                     int
	OutcomeCounts                                                                                                  map[string]int
	Ledger                                                                                                         []LedgerEntry
	ReviewVerdict                                                                                                  string
}

var matrix = []caseSpec{
	{"01-complete-top5", "complete"}, {"02-complete-empty", "empty"}, {"03-invalid-query", "invalid-query"}, {"04-unavailable", "unavailable"},
	{"05-mismatch", "mismatch"}, {"06-refusal-zero", "refusal-0"}, {"07-refusal-partial", "refusal-4"}, {"08-cancel-zero", "cancel-0"},
	{"09-cancel-partial", "cancel-7"}, {"10-timeout-partial", "timeout-11"}, {"11-work-boundary", "work-boundary"}, {"12-work-limit-before-24th", "work-limit"},
	{"13-response-boundary-65536", "response-boundary"}, {"14-response-plus-one", "response-plus-one"}, {"15-backend-zero", "backend-0"}, {"16-backend-partial", "backend-3"},
	{"17-malformed-score", "malformed-score"}, {"18-unknown-member", "unknown-member"}, {"19-duplicate-delivery-conflict", "conflict"}, {"20-describe-mutation", "describe-mutation"},
	{"21-sixth-qualifying", "sixth"}, {"22-invalid-utf8", "invalid-utf8"}, {"23-partial-frame", "partial-frame"}, {"24-duplicate-member", "duplicate-member"},
}

func canonical(v any) []byte                  { b, _ := json.Marshal(v); return append(b, '\n') }
func fileDigest(b []byte) string              { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func writeCanonical(path string, v any) error { return os.WriteFile(path, canonical(v), 0644) }
func schemaString() map[string]any            { return map[string]any{"type": "string", "minLength": 1} }
func schemaBoundedUTF8(max int) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": max, "x-maxUtf8Bytes": max}
}
func schemaRawBytesMax(maxDecoded int) map[string]any {
	return map[string]any{
		"type":              "string",
		"minLength":         4,
		"maxLength":         ((maxDecoded + 2) / 3) * 4,
		"pattern":           `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`,
		"x-maxDecodedBytes": maxDecoded,
	}
}
func schemaRawBytes() map[string]any { return schemaRawBytesMax(MaxResponseBytes) }
func schemaRawBytesExact(decodedBytes int) map[string]any {
	encodedBytes := ((decodedBytes + 2) / 3) * 4
	return map[string]any{
		"type":                "string",
		"minLength":           encodedBytes,
		"maxLength":           encodedBytes,
		"pattern":             `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`,
		"x-decodedBytesConst": decodedBytes,
	}
}
func schemaDigest() map[string]any {
	return map[string]any{"type": "string", "pattern": `^sha256:[0-9a-f]{64}$`}
}
func schemaEnum(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func schemaConst(value string) map[string]any {
	return map[string]any{"type": "string", "const": value}
}
func schemaInt(min, max int) map[string]any {
	return map[string]any{"type": "integer", "minimum": min, "maximum": max}
}
func schemaBool() map[string]any { return map[string]any{"type": "boolean"} }
func schemaArray(items map[string]any, min, max int) map[string]any {
	return map[string]any{"type": "array", "items": items, "minItems": min, "maxItems": max}
}
func schemaObject(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
}
func publishedSchema(name string, body map[string]any) map[string]any {
	body["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	body["$id"] = Version + ".schema." + name
	return body
}
func schemaMap() map[string]map[string]any {
	str, digest, boolean := schemaString, schemaDigest, schemaBool
	custodyID := func() map[string]any { return schemaBoundedUTF8(MaxCustodyIDBytes) }
	limits := schemaObject([]string{"QueryBytes", "Depth", "Frontier", "Evaluations", "WorkPrecharge", "WorkMax", "ResultCount", "ResponseBytes", "CancellationCheckpoints", "DeadlineSemantics"}, map[string]any{"QueryBytes": schemaInt(0, MaxQueryBytes), "Depth": schemaInt(0, MaxDepth), "Frontier": schemaInt(0, MaxFrontier), "Evaluations": schemaInt(0, MaxEvaluations), "WorkPrecharge": schemaInt(WorkPerMember, WorkPerMember), "WorkMax": schemaInt(MaxWork, MaxWork), "ResultCount": schemaInt(0, MaxResults), "ResponseBytes": schemaInt(0, MaxResponseBytes), "CancellationCheckpoints": schemaArray(schemaEnum("BEFORE_ADAPT", "BEFORE_MEMBER", "BEFORE_COMMIT"), 3, 3), "DeadlineSemantics": schemaConst("deadline is expired when now >= deadline; cancellation precedes deadline at every checkpoint")})
	ledger := schemaObject([]string{"Ordinal", "MemberID", "Outcome", "Score", "Rationale", "Selected"}, map[string]any{"Ordinal": schemaInt(1, MemberCount), "MemberID": str(), "Outcome": schemaEnum(MemberOutcomes[:]...), "Score": map[string]any{"type": "string", "pattern": `^(0\.[0-9]{6}|1\.000000)$`}, "Rationale": str(), "Selected": boolean()})
	evaluation := schemaObject([]string{"MemberID", "Score", "Rationale"}, map[string]any{"MemberID": str(), "Score": map[string]any{"type": "string", "pattern": `^(0\.[0-9]{6}|1\.000000)$`}, "Rationale": str()})
	attemptKey := schemaObject([]string{"GenerationID", "AssignmentID", "ReviewRequestID", "AttemptID"}, map[string]any{"GenerationID": custodyID(), "AssignmentID": custodyID(), "ReviewRequestID": str(), "AttemptID": custodyID()})
	custody := schemaObject([]string{"SchemaVersion", "GenerationID", "ProducerAssignmentID", "ProducerWorkerID", "ProducerTaskID", "ProducerRole", "ReviewerAssignmentID", "ReviewerWorkerID", "ReviewerTaskID", "ReviewerRole", "OutputSelector", "OutputDigest", "OutputLength", "EventSelector", "EventDigest", "EventLength"}, map[string]any{"SchemaVersion": schemaConst(Version + ".parent-custody-evidence"), "GenerationID": custodyID(), "ProducerAssignmentID": custodyID(), "ProducerWorkerID": custodyID(), "ProducerTaskID": custodyID(), "ProducerRole": custodyID(), "ReviewerAssignmentID": custodyID(), "ReviewerWorkerID": custodyID(), "ReviewerTaskID": custodyID(), "ReviewerRole": custodyID(), "OutputSelector": custodyID(), "OutputDigest": digest(), "OutputLength": schemaInt(1, MaxResponseBytes), "EventSelector": custodyID(), "EventDigest": digest(), "EventLength": schemaInt(1, MaxResponseBytes)})
	selection := schemaObject([]string{"GenerationID", "RequestID", "ProducerAttemptKey", "ResultID", "ReviewerAttemptKey", "ReviewID"}, map[string]any{"GenerationID": str(), "RequestID": str(), "ProducerAttemptKey": str(), "ResultID": str(), "ReviewerAttemptKey": attemptKey, "ReviewID": str()})
	outcomeCounts := map[string]any{"type": "object", "additionalProperties": false, "required": MemberOutcomes[:], "properties": map[string]any{}}
	for _, outcome := range MemberOutcomes {
		outcomeCounts["properties"].(map[string]any)[outcome] = schemaInt(0, MemberCount)
	}
	checks, reasons := map[string]any{}, map[string]any{}
	for _, check := range ReviewChecks {
		checks[check] = boolean()
		reasons[check] = map[string]any{"type": "string", "maxLength": MaxReviewReasonBytes}
	}
	producerRawBytes := schemaRawBytesMax(MaxResponseBytes + 1)
	producerRawBytes["description"] = "Canonical base64; decoded length is at most 65536 except the conditional exact 65537-byte RAW_SIZE terminal-invalid branch."
	producerAttempt := schemaObject([]string{"Key", "GenerationID", "RequestID", "AttemptID", "AssignmentID", "RawDigest", "RawBytes", "States", "TerminalReason", "ResultID"}, map[string]any{"Key": str(), "GenerationID": str(), "RequestID": str(), "AttemptID": str(), "AssignmentID": str(), "RawDigest": digest(), "RawBytes": producerRawBytes, "States": schemaArray(schemaEnum(AttemptStates[:]...), 1, 3), "TerminalReason": map[string]any{"type": "string"}, "ResultID": map[string]any{"type": "string"}})
	producerAttempt["allOf"] = []any{map[string]any{
		"if": map[string]any{"type": "object", "required": []string{"TerminalReason"}, "properties": map[string]any{"TerminalReason": schemaConst("RAW_SIZE")}},
		"then": map[string]any{"type": "object", "properties": map[string]any{
			"RawBytes": schemaRawBytesExact(MaxResponseBytes + 1),
			"States":   map[string]any{"const": []any{"RECEIVED", "TERMINAL_INVALID"}},
			"ResultID": schemaConst(""),
		}},
		"else": map[string]any{"type": "object", "properties": map[string]any{"RawBytes": schemaRawBytes()}},
	}}
	return map[string]map[string]any{
		"SearchRequest":         publishedSchema("SearchRequest", schemaObject([]string{"SchemaVersion", "RequestID", "GenerationID", "AttemptID", "Query", "OrderedMembers", "Denominator", "IndexIdentity", "DescribeDigest", "PolicyDigest", "EvaluationDigest", "ProtocolIdentity", "CeilingIdentity", "Limits"}, map[string]any{"SchemaVersion": schemaConst(Version + ".request"), "RequestID": str(), "GenerationID": str(), "AttemptID": str(), "Query": map[string]any{"type": "string", "minLength": 1, "maxLength": MaxQueryBytes}, "OrderedMembers": map[string]any{"type": "array", "items": str(), "minItems": MemberCount, "maxItems": MemberCount, "uniqueItems": true}, "Denominator": schemaConst("24"), "IndexIdentity": digest(), "DescribeDigest": digest(), "PolicyDigest": digest(), "EvaluationDigest": digest(), "ProtocolIdentity": schemaConst(Version), "CeilingIdentity": schemaConst(CustodyCeiling), "Limits": limits})),
		"HostResult":            publishedSchema("HostResult", schemaObject([]string{"SchemaVersion", "GenerationID", "RequestID", "AttemptID", "AssignmentID", "Disposition", "Evaluations"}, map[string]any{"SchemaVersion": schemaConst(Version + ".host-result"), "GenerationID": str(), "RequestID": str(), "AttemptID": str(), "AssignmentID": str(), "Disposition": schemaEnum(HostDispositions[:]...), "Evaluations": schemaArray(evaluation, 0, MemberCount)})),
		"SearchResult":          publishedSchema("SearchResult", schemaObject([]string{"SchemaVersion", "ResultID", "RequestID", "GenerationID", "AttemptKey", "Disposition", "Outcome", "Completeness", "Denominator", "Begun", "Completed", "Unevaluated", "WorkCharged", "Ledger", "Selected", "RequestDigest", "RawDigest", "DescribeDigest", "PolicyDigest", "EvaluationDigest", "Authority", "Accepted", "FeatureIdentity"}, map[string]any{"SchemaVersion": schemaConst(Version + ".result"), "ResultID": str(), "RequestID": str(), "GenerationID": str(), "AttemptKey": str(), "Disposition": schemaEnum(HostDispositions[:]...), "Outcome": schemaEnum(OperationOutcomes[:]...), "Completeness": schemaEnum("COMPLETE", "INCOMPLETE"), "Denominator": schemaInt(MemberCount, MemberCount), "Begun": schemaInt(0, MemberCount), "Completed": schemaInt(0, MemberCount), "Unevaluated": schemaInt(0, MemberCount), "WorkCharged": schemaInt(0, MaxWork), "Ledger": schemaArray(ledger, 0, MemberCount), "Selected": schemaArray(ledger, 0, MaxResults), "RequestDigest": digest(), "RawDigest": digest(), "DescribeDigest": digest(), "PolicyDigest": digest(), "EvaluationDigest": digest(), "Authority": schemaInt(0, 0), "Accepted": map[string]any{"type": "boolean", "const": false}, "FeatureIdentity": schemaConst("UNRESOLVED")})),
		"ProducerAttempt":       publishedSchema("ProducerAttempt", producerAttempt),
		"ProducerConflict":      publishedSchema("ProducerConflict", schemaObject([]string{"DeliveryID", "AttemptKey", "PriorDigest", "ConflictingDigest"}, map[string]any{"DeliveryID": str(), "AttemptKey": str(), "PriorDigest": digest(), "ConflictingDigest": digest()})),
		"ReviewRequest":         publishedSchema("ReviewRequest", schemaObject([]string{"SchemaVersion", "ReviewRequestID", "GenerationID", "RequestID", "RequestDigest", "ProducerAttemptKey", "ProducerAttemptID", "ProducerAssignmentID", "RawDigest", "ResultID", "ResultDigest", "Evidence", "PairDigest", "EvidenceDigest", "LimitsDigest", "CustodyCeiling"}, map[string]any{"SchemaVersion": schemaConst(Version + ".review-request"), "ReviewRequestID": str(), "GenerationID": custodyID(), "RequestID": str(), "RequestDigest": digest(), "ProducerAttemptKey": str(), "ProducerAttemptID": str(), "ProducerAssignmentID": custodyID(), "RawDigest": digest(), "ResultID": str(), "ResultDigest": digest(), "Evidence": custody, "PairDigest": digest(), "EvidenceDigest": digest(), "LimitsDigest": digest(), "CustodyCeiling": schemaConst(CustodyCeiling)})),
		"ReviewerRaw":           publishedSchema("ReviewerRaw", schemaObject([]string{"checks", "reasons"}, map[string]any{"checks": map[string]any{"type": "object", "additionalProperties": false, "required": ReviewChecks[:], "properties": checks}, "reasons": map[string]any{"type": "object", "additionalProperties": false, "required": []string{}, "properties": reasons}})),
		"ReviewerAttempt":       publishedSchema("ReviewerAttempt", schemaObject([]string{"Key", "RawDigest", "RawBytes", "States", "TerminalReason", "ReviewID"}, map[string]any{"Key": attemptKey, "RawDigest": digest(), "RawBytes": schemaRawBytes(), "States": schemaArray(schemaEnum(AttemptStates[:]...), 1, 3), "TerminalReason": map[string]any{"type": "string"}, "ReviewID": map[string]any{"type": "string"}})),
		"ReviewerConflict":      publishedSchema("ReviewerConflict", schemaObject([]string{"ConflictID", "Key", "PriorDigest", "ConflictingDigest"}, map[string]any{"ConflictID": str(), "Key": attemptKey, "PriorDigest": digest(), "ConflictingDigest": digest()})),
		"Account":               publishedSchema("Account", schemaObject([]string{"Selection", "OutcomeCounts", "Balanced", "MechanicalPercent", "CustodyPercent", "AccountingPercent", "CriticalReviewPercent", "SemanticUsefulnessQualified", "FeatureIdentity"}, map[string]any{"Selection": selection, "OutcomeCounts": outcomeCounts, "Balanced": boolean(), "MechanicalPercent": schemaInt(0, 100), "CustodyPercent": schemaInt(0, 100), "AccountingPercent": schemaInt(0, 100), "CriticalReviewPercent": schemaInt(0, 100), "SemanticUsefulnessQualified": map[string]any{"type": "boolean", "const": false}, "FeatureIdentity": schemaConst("UNRESOLVED")})),
		"ParentCustodyEvidence": publishedSchema("ParentCustodyEvidence", custody),
	}
}
func operationForCause(cause string) string {
	switch cause {
	case "INVALID_QUERY":
		return "INVALID_QUERY"
	case "DESCRIBE_BINDING", "DESCRIBE_MEMBER", "DESCRIBE_IDENTITY", "DESCRIBE_EXACT_48", "INDEX_IDENTITY", "REQUEST_ASSIGNMENT":
		return "INDEX_MISMATCH"
	case "RAW_SIZE", "WORK_PRECHARGE":
		return "RESOURCE_LIMIT"
	default:
		return "BACKEND_FAILURE"
	}
}
func validOperationOutcome(outcome string) bool {
	for _, candidate := range OperationOutcomes {
		if outcome == candidate {
			return true
		}
	}
	return false
}

// GenerateArtifacts replaces root with a deterministic, private, non-dispatching fixture tree.
func GenerateArtifacts(root, describeSource, packageDir string) (string, error) {
	attemptPath := filepath.Join(describeSource, "producer-committed-attempts.json")
	requestPath := filepath.Join(describeSource, "producer-request-records.json")
	ab, err := os.ReadFile(attemptPath)
	if err != nil {
		return "", err
	}
	rb, err := os.ReadFile(requestPath)
	if err != nil {
		return "", err
	}
	policyBytes, err := os.ReadFile(filepath.Join(describeSource, "QUALIFICATION_POLICY_AMENDMENT.json"))
	if err != nil {
		return "", err
	}
	evaluationBytes, err := os.ReadFile(filepath.Join(describeSource, "THRESHOLD_EVALUATION.json"))
	if err != nil {
		return "", err
	}
	var attempts []sourceAttempt
	var requests []sourceRequest
	if json.Unmarshal(ab, &attempts) != nil || json.Unmarshal(rb, &requests) != nil {
		return "", errors.New("describe source parse")
	}
	binding, err := importDescribe(attempts, requests, fileDigest(ab), fileDigest(rb), fileDigest(policyBytes), fileDigest(evaluationBytes))
	if err != nil {
		return "", err
	}
	if err = os.RemoveAll(root); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Join(root, "schemas"), 0755); err != nil {
		return "", err
	}
	if err = writeCanonical(filepath.Join(root, "DESCRIBE_BINDING.json"), binding); err != nil {
		return "", err
	}
	roots := map[string]any{
		"POLICY.json":                  map[string]any{"schemaVersion": Version + ".policy", "scoreThreshold": ScoreThreshold, "operationOutcomes": OperationOutcomes, "memberOutcomes": MemberOutcomes, "attemptStates": AttemptStates, "precedence": Precedence, "authority": 0, "accepted": false, "featureIdentity": "UNRESOLVED"},
		"LIMITS.json":                  FixedLimits(),
		"REVIEW_POLICY.json":           map[string]any{"schemaVersion": Version + ".review-policy", "checks": ReviewChecks, "acceptIffAllChecksTrue": true, "custodyCeiling": CustodyCeiling},
		"AUTHORIZATION_CANDIDATE.json": map[string]any{"schemaVersion": Version + ".authorization-candidate", "state": "PENDING", "dispatchAllowed": false, "searchExecuted": false, "authority": 0, "featureIdentity": "UNRESOLVED"},
		"PRE_EXECUTION_AUDIT.json":     map[string]any{"schemaVersion": Version + ".pre-execution-audit", "status": "BLOCKED", "go": false, "execution": false, "reason": "DESIGN_ONLY_NON_DISPATCHING"},
		"PREDECESSORS.json":            map[string]any{"schemaVersion": Version + ".predecessors", "blocked": []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10"}},
	}
	for n, v := range roots {
		if err = writeCanonical(filepath.Join(root, n), v); err != nil {
			return "", err
		}
	}
	design := "# Bounded Search v11 design\n\nPrivate, prospective, non-dispatching qualification only. Exact operation outcomes: COMPLETE, INVALID_QUERY, INDEX_UNAVAILABLE, INDEX_MISMATCH, CANCELLED, TIMEOUT, RESOURCE_LIMIT, BACKEND_FAILURE, POLICY_MISMATCH. Exact member outcomes: RETURNED, BELOW_THRESHOLD, FILTERED_BY_POLICY, DUPLICATE_MEMBER, INVALID_MEMBER. Authority is zero; acceptance is false; feature identity is UNRESOLVED. Excludes execution, GO, public APIs, Group, ADR0011, push, and release.\n"
	if err = os.WriteFile(filepath.Join(root, "DESIGN.md"), []byte(design), 0644); err != nil {
		return "", err
	}
	for n, schema := range schemaMap() {
		if err = writeCanonical(filepath.Join(root, "schemas", n+".schema.json"), schema); err != nil {
			return "", err
		}
	}
	for i, c := range matrix {
		if err = generateCase(root, c, i, binding); err != nil {
			return "", fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	return GenerateFreezeWithSources(root, packageDir)
}

func importDescribe(attempts []sourceAttempt, requests []sourceRequest, ad, rd, policyDigest, evaluationDigest string) (describeArtifact, error) {
	if len(attempts) != DescribeRecords || len(requests) != DescribeRecords {
		return describeArtifact{}, errors.New("describe exact 48")
	}
	rm := map[string]sourceRequest{}
	cases := map[string]bool{}
	lanes := map[string]bool{}
	ids := map[string]bool{}
	for _, r := range requests {
		if r.RequestID == "" || rm[r.RequestID].RequestID != "" || !validDigest(r.SourceRevision) || !validDigest(r.EvidenceDigest) || r.Authority != 0 || r.Accepted || r.FeatureIdentity != "UNRESOLVED" {
			return describeArtifact{}, errors.New("describe request")
		}
		rm[r.RequestID] = r
		cases[r.CaseID] = true
		lanes[r.LaneID] = true
	}
	out := describeArtifact{Version + ".describe-binding", ad, rd, policyDigest, evaluationDigest, len(cases), len(attempts), nil}
	for _, a := range attempts {
		r, ok := rm[a.Key.AssignmentID]
		if !ok {
			return describeArtifact{}, errors.New("describe join")
		}
		vals := []string{a.Key.AttemptID, a.ReceiptID, a.RequestDigest, a.RawResultDigest, a.RawResponseDigest}
		for _, v := range vals {
			if v == "" || ids[v] {
				return describeArtifact{}, errors.New("describe uniqueness")
			}
			ids[v] = true
		}
		if a.State != "COMMITTED" || fmt.Sprint(a.History) != "[RECEIVED ADAPTED COMMITTED]" || a.RequestDigest == "" {
			return describeArtifact{}, errors.New("describe state")
		}
		var rec struct{ Semantic json.RawMessage }
		if json.Unmarshal(a.Record, &rec) != nil || len(rec.Semantic) == 0 {
			return describeArtifact{}, errors.New("describe semantic")
		}
		out.Records = append(out.Records, describeRecord{a.Key.GenerationID, r.CaseID, r.LaneID, r.PreparationID, r.RequestID, a.RequestDigest, a.Key.AttemptID, a.RawResultDigest, a.RawResponseDigest, a.ReceiptID, r.EvidenceDigest, r.SourceRevision, a.State, a.History, a.Record, a.RawResult, Digest(rec.Semantic)})
	}
	sort.Slice(out.Records, func(i, j int) bool {
		if out.Records[i].Case == out.Records[j].Case {
			return out.Records[i].Lane < out.Records[j].Lane
		}
		return out.Records[i].Case < out.Records[j].Case
	})
	if len(cases) != 24 || len(lanes) != 2 {
		return describeArtifact{}, errors.New("describe cardinality")
	}
	return out, nil
}

func fixtureDescribeFromBinding(b describeArtifact) DescribeBinding {
	d := DescribeBinding{GenerationID: Generation, DescribeDigest: Digest(canonical(b)), PolicyDigest: b.PolicyDigest, EvaluationDigest: b.EvaluationDigest}
	for i := 0; i < MemberCount; i++ {
		p, q := b.Records[i*2], b.Records[i*2+1]
		mk := func(x describeRecord) Identity {
			return Identity{x.Request, x.Attempt, x.Receipt, recordID(x.Record), x.RawResultDigest, Digest([]byte(x.Evidence + ":" + x.Request)), Digest([]byte(x.Source + ":" + x.Request))}
		}
		d.Members = append(d.Members, MemberBinding{i + 1, p.Case, "primary+replay", mk(p), mk(q)})
	}
	return d
}
func recordID(raw json.RawMessage) string {
	var v struct {
		RecordID string `json:"record_id"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.RecordID
}
func rawFor(req SearchRequest, kind string) []byte {
	n, disp := 24, "RESULT"
	switch kind {
	case "empty", "work-boundary", "sixth":
	case "unavailable":
		n = 0
		disp = "UNAVAILABLE"
	case "refusal-0":
		n = 0
		disp = "REFUSE"
	case "refusal-4":
		n = 4
		disp = "REFUSE"
	case "cancel-0":
		n = 0
		disp = "CANCELLED"
	case "cancel-7":
		n = 7
		disp = "CANCELLED"
	case "timeout-11":
		n = 11
		disp = "TIMEOUT"
	case "backend-0":
		n = 0
		disp = "FAILED"
	case "backend-3":
		n = 3
		disp = "FAILED"
	}
	h := HostResult{Version + ".host-result", req.GenerationID, req.RequestID, req.AttemptID, "assignment-1", disp, nil}
	for i := 0; i < n; i++ {
		s := "0.900000"
		if kind == "empty" {
			s = "0.590000"
		}
		h.Evaluations = append(h.Evaluations, MemberEvaluation{req.OrderedMembers[i], s, "bounded rationale"})
	}
	if kind == "malformed-score" {
		h.Disposition = "FAILED"
		h.Evaluations = []MemberEvaluation{{req.OrderedMembers[0], "0.7", "bad"}}
	}
	if kind == "unknown-member" {
		h.Disposition = "FAILED"
		h.Evaluations = []MemberEvaluation{{"unknown", "0.700000", "bad"}}
	}
	if kind == "duplicate-member" {
		h.Evaluations[23].MemberID = h.Evaluations[0].MemberID
	}
	b, _ := json.Marshal(h)
	return append(b, '\n')
}
func generateCase(root string, c caseSpec, index int, b describeArtifact) error {
	dir := filepath.Join(root, "cases", c.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	d := fixtureDescribeFromBinding(b)
	cond := caseCondition{SchemaVersion: Version + ".case-condition", CaseID: c.ID, Action: "INGEST", Query: "bounded search", IndexIdentity: Digest([]byte("index"))}
	if c.Kind == "invalid-query" {
		cond.Action, cond.Query = "PREPARE", ""
	}
	if c.Kind == "describe-mutation" {
		cond.Action, cond.DescribeMutation = "PREPARE", "DESCRIBE_DIGEST"
		d.DescribeDigest = "mutated"
	}
	if c.Kind == "work-limit" {
		cond.Action = "ADMISSION"
		cond.Admission = &AdmissionState{RequestValid: true, AssignmentValid: true, DigestValid: true, RawBytes: 1, UTF8: true, Frame: true, StrictJSON: true, Frontier: 24, Evaluations: 23, Work: 231, NextWork: 10}
	}
	req, prepErr := Prepare(d, cond.Query, cond.IndexIdentity)
	if prepErr != nil {
		req = SearchRequest{SchemaVersion: Version + ".request", GenerationID: Generation, Query: cond.Query}
	}
	var raw []byte
	if prepErr == nil {
		raw = rawFor(req, c.Kind)
	}
	if c.Kind == "mismatch" {
		var h HostResult
		_ = json.Unmarshal(bytes.TrimSuffix(raw, []byte{'\n'}), &h)
		h.RequestID += "-mismatch"
		raw = canonical(h)
	}
	if c.Kind == "invalid-utf8" {
		raw = []byte{0xff, '\n'}
	}
	if c.Kind == "partial-frame" {
		raw = []byte{'{'}
	}
	if c.Kind == "response-plus-one" {
		raw = bytes.Repeat([]byte{' '}, MaxResponseBytes+1)
	}
	if c.Kind == "response-boundary" {
		raw = bytes.Repeat([]byte{' '}, MaxResponseBytes)
	}
	exp := expectedFixture{SchemaVersion: Version + ".expected", CaseID: c.ID, Denominator: MemberCount, Unevaluated: MemberCount, OutcomeCounts: map[string]int{}, ReviewVerdict: "NOT_APPLICABLE", Nonqualification: "NO_COMMITTED_RESULT"}
	for _, o := range MemberOutcomes {
		exp.OutcomeCounts[o] = 0
	}
	if cond.Action == "PREPARE" {
		exp.Cause = prepErr.Error()
		exp.OperationOutcome = map[bool]string{true: "INVALID_QUERY", false: "INDEX_MISMATCH"}[c.Kind == "invalid-query"]
	} else if cond.Action == "ADMISSION" {
		exp.Cause = ClassifyAdmission(*cond.Admission)
		exp.OperationOutcome = "RESOURCE_LIMIT"
		exp.Begun, exp.Completed, exp.Unevaluated, exp.Work = 23, 23, 1, 231
	} else {
		w := NewWriter()
		a, res, ingestErr := w.Ingest(req, "assignment-1", raw, Digest(raw))
		if err := writeCanonical(filepath.Join(dir, "PRODUCER_ATTEMPT.json"), a); err != nil {
			return err
		}
		if ingestErr != nil {
			exp.Cause, exp.OperationOutcome = ingestErr.Error(), operationForCause(ingestErr.Error())
		} else {
			exp.OperationOutcome, exp.Completeness = res.Outcome, res.Completeness
			exp.Denominator, exp.Begun, exp.Completed, exp.Unevaluated, exp.Work = res.Denominator, res.Begun, res.Completed, res.Unevaluated, res.WorkCharged
			exp.Ledger, exp.Selected, exp.ReviewVerdict, exp.Nonqualification = res.Ledger, len(res.Selected), "ACCEPT", "QUALIFIED_MECHANICALLY_ONLY"
			for _, e := range res.Ledger {
				exp.OutcomeCounts[e.Outcome]++
			}
			if err := writeCanonical(filepath.Join(dir, "RESULT.json"), res); err != nil {
				return err
			}
			custody := ParentCustodyEvidence{Version + ".parent-custody-evidence", req.GenerationID, "assignment-1", "producer-worker", "producer-task", "producer", "review-assignment", "reviewer-worker", "reviewer-task", "reviewer", "result:" + res.ResultID, Digest(canon(res)), len(canon(res)), "committed:" + a.Key, a.RawDigest, len(a.RawBytes)}
			q, err := NewReviewRequest(req, a, res, custody)
			if err != nil {
				return err
			}
			reviewRaw := canonical(ReviewerRaw{allTrueChecks(), map[string]string{}})
			key, err := NewReviewerAttemptKey(q, custody.ReviewerAssignmentID, "review-attempt-1")
			if err != nil {
				return err
			}
			ra, rv, err := NewReviewWriter().Ingest(q, key, reviewRaw, Digest(reviewRaw))
			if err != nil {
				return err
			}
			sel := AccountSelection{req.GenerationID, req.RequestID, a.Key, res.ResultID, ra.Key, rv.ReviewID}
			acct, err := MakeAccount(sel, req, a, res, q, ra, rv, exp.OutcomeCounts)
			if err != nil {
				return err
			}
			for name, value := range map[string]any{"CUSTODY.json": custody, "REVIEW_REQUEST.json": q, "REVIEW_EXPECTED.json": rv, "REVIEW_ATTEMPT.json": ra, "ACCOUNT.json": acct} {
				if err = writeCanonical(filepath.Join(dir, name), value); err != nil {
					return err
				}
			}
			if err = os.WriteFile(filepath.Join(dir, "REVIEW_RAW"), reviewRaw, 0644); err != nil {
				return err
			}
		}
		if c.Kind == "conflict" {
			conflicting := rawFor(req, "unavailable")
			old, _, conflictErr := w.Ingest(req, "assignment-1", conflicting, Digest(conflicting))
			if conflictErr == nil || !reflect.DeepEqual(old, a) || len(w.conflicts) != 1 {
				return errors.New("conflict fixture")
			}
			cond.Action, exp.ConflictCause, exp.ConflictOutcome = "CONFLICT", conflictErr.Error(), operationForCause(conflictErr.Error())
			if err := os.WriteFile(filepath.Join(dir, "CONFLICT_RAW"), conflicting, 0644); err != nil {
				return err
			}
		}
	}
	if err := writeCanonical(filepath.Join(dir, "CONDITION.json"), cond); err != nil {
		return err
	}
	if err := writeCanonical(filepath.Join(dir, "REQUEST.json"), req); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "RAW"), raw, 0644); err != nil {
		return err
	}
	return writeCanonical(filepath.Join(dir, "EXPECTED.json"), exp)
}
func allTrueChecks() map[string]bool {
	m := map[string]bool{}
	for _, k := range ReviewChecks {
		m[k] = true
	}
	return m
}

func GenerateFreeze(root string) (string, error) { return GenerateFreezeWithSources(root, "") }
func GenerateFreezeWithSources(root, packageDir string) (string, error) {
	files, err := treeFiles(root)
	if err != nil {
		return "", err
	}
	var sources []FileIdentity
	if packageDir != "" {
		sources, err = sourceFiles(packageDir)
		if err != nil {
			return "", err
		}
	}
	f := Freeze{Version + ".freeze", "DESIGN_FROZEN_NON_DISPATCHING", "", files, sources, false, false, false, []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10"}}
	f.FreezeID = Digest(canon(f))
	if err = writeCanonical(filepath.Join(root, "FREEZE.json"), f); err != nil {
		return "", err
	}
	return f.FreezeID, nil
}
func VerifyFreeze(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if err != nil {
		return "", err
	}
	var f Freeze
	if strictJSONFile(b, &f) != nil {
		return "", errors.New("freeze parse")
	}
	files, err := treeFiles(root)
	if err != nil {
		return "", err
	}
	x := f
	x.FreezeID = ""
	if f.SchemaVersion != Version+".freeze" || f.Status != "DESIGN_FROZEN_NON_DISPATCHING" || f.DispatchAllowed || f.SearchExecuted || f.SearchDesignGO || fmt.Sprint(f.BlockedPredecessors) != "[v1 v2 v3 v4 v5 v6 v7 v8 v9 v10]" || Digest(canon(x)) != f.FreezeID {
		return "", errors.New("freeze metadata")
	}
	if fmt.Sprint(files) != fmt.Sprint(f.Files) {
		return "", errors.New("tree mismatch")
	}
	return f.FreezeID, nil
}
func VerifyArtifacts(root string) error {
	if _, err := VerifyFreeze(root); err != nil {
		return err
	}
	var b describeArtifact
	if err := readStrict(filepath.Join(root, "DESCRIBE_BINDING.json"), &b); err != nil {
		return err
	}
	if b.Cases != 24 || b.Attempts != 48 || len(b.Records) != 48 {
		return errors.New("describe counts")
	}
	baseDescribe := fixtureDescribeFromBinding(b)
	if err := ValidateDescribe(baseDescribe); err != nil {
		return fmt.Errorf("describe API: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "cases"))
	if err != nil || len(entries) != len(matrix) {
		return errors.New("case count")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return errors.New("case entry")
		}
		dir := filepath.Join(root, "cases", entry.Name())
		if err := verifyCaseArtifacts(dir, entry.Name(), baseDescribe); err != nil {
			return fmt.Errorf("%s: %w", entry.Name(), err)
		}
	}
	return nil
}

func verifyAttemptFileRawBytes(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var envelope struct {
		RawBytes       string
		States         []string
		TerminalReason string
		ResultID       string
		ReviewID       string
	}
	if err = json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(envelope.RawBytes)
	if err != nil || len(decoded) == 0 || base64.StdEncoding.EncodeToString(decoded) != envelope.RawBytes {
		return errors.New("published RawBytes")
	}
	switch filepath.Base(path) {
	case "PRODUCER_ATTEMPT.json":
		if envelope.TerminalReason == "RAW_SIZE" {
			if len(decoded) != MaxResponseBytes+1 || !reflect.DeepEqual(envelope.States, []string{"RECEIVED", "TERMINAL_INVALID"}) || envelope.ResultID != "" || envelope.ReviewID != "" {
				return errors.New("published RawBytes")
			}
			return nil
		}
		if len(decoded) > MaxResponseBytes {
			return errors.New("published RawBytes")
		}
	case "REVIEW_ATTEMPT.json":
		if len(decoded) > MaxResponseBytes {
			return errors.New("published RawBytes")
		}
	default:
		return errors.New("published attempt role")
	}
	return nil
}

func verifyCaseArtifacts(dir, caseID string, baseDescribe DescribeBinding) error {
	var cond caseCondition
	var frozenReq SearchRequest
	var exp expectedFixture
	if readStrict(filepath.Join(dir, "CONDITION.json"), &cond) != nil || readStrict(filepath.Join(dir, "REQUEST.json"), &frozenReq) != nil || readStrict(filepath.Join(dir, "EXPECTED.json"), &exp) != nil {
		return errors.New("fixture parse")
	}
	if cond.SchemaVersion != Version+".case-condition" || cond.CaseID != caseID || exp.CaseID != caseID || exp.SchemaVersion != Version+".expected" {
		return errors.New("fixture identity")
	}
	if !validOperationOutcome(exp.OperationOutcome) || (exp.Cause != "" && exp.OperationOutcome != operationForCause(exp.Cause)) {
		return errors.New("operation expectation")
	}
	if len(exp.OutcomeCounts) != len(MemberOutcomes) {
		return errors.New("outcome count keys")
	}
	for _, key := range MemberOutcomes {
		if _, ok := exp.OutcomeCounts[key]; !ok {
			return errors.New("outcome count keys")
		}
	}
	d := baseDescribe
	if cond.DescribeMutation == "DESCRIBE_DIGEST" {
		d.DescribeDigest = "mutated"
	} else if cond.DescribeMutation != "" {
		return errors.New("describe condition")
	}
	actualReq, prepErr := Prepare(d, cond.Query, cond.IndexIdentity)
	if prepErr != nil {
		actualReq = SearchRequest{SchemaVersion: Version + ".request", GenerationID: Generation, Query: cond.Query}
	}
	if !reflect.DeepEqual(actualReq, frozenReq) {
		return errors.New("request mismatch")
	}
	switch cond.Action {
	case "PREPARE":
		if prepErr == nil || prepErr.Error() != exp.Cause || exp.OperationOutcome != operationForCause(prepErr.Error()) || exp.Nonqualification != "NO_COMMITTED_RESULT" {
			return errors.New("prepare expectation")
		}
		return nil
	case "ADMISSION":
		if prepErr != nil || cond.Admission == nil || ClassifyAdmission(*cond.Admission) != exp.Cause || exp.Cause != "WORK_PRECHARGE" || exp.OperationOutcome != operationForCause(exp.Cause) || exp.Begun != 23 || exp.Completed != 23 || exp.Unevaluated != 1 || exp.Work != 231 || len(exp.Ledger) != 0 || exp.Nonqualification != "NO_COMMITTED_RESULT" {
			return errors.New("admission expectation")
		}
		return nil
	case "INGEST", "CONFLICT":
		if prepErr != nil {
			return prepErr
		}
	default:
		return errors.New("condition action")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "RAW"))
	if err != nil {
		return err
	}
	w := NewWriter()
	a, result, ingestErr := w.Ingest(frozenReq, "assignment-1", raw, Digest(raw))
	var frozenAttempt Attempt
	producerAttemptPath := filepath.Join(dir, "PRODUCER_ATTEMPT.json")
	if err = verifyAttemptFileRawBytes(producerAttemptPath); err != nil {
		return errors.New("producer attempt RawBytes")
	}
	if err = readStrict(producerAttemptPath, &frozenAttempt); err != nil || !reflect.DeepEqual(a, frozenAttempt) || a.RawDigest != Digest(raw) || !bytes.Equal(a.RawBytes, raw) {
		return errors.New("producer attempt")
	}
	if cond.Action == "CONFLICT" {
		if ingestErr != nil || a.ResultID == "" {
			return errors.New("conflict prior commit")
		}
		prior := a
		conflicting, readErr := os.ReadFile(filepath.Join(dir, "CONFLICT_RAW"))
		if readErr != nil {
			return readErr
		}
		got, _, conflictErr := w.Ingest(frozenReq, "assignment-1", conflicting, Digest(conflicting))
		if conflictErr == nil || conflictErr.Error() != exp.ConflictCause || !validOperationOutcome(exp.ConflictOutcome) || exp.ConflictOutcome != operationForCause(conflictErr.Error()) || !reflect.DeepEqual(got, prior) || !reflect.DeepEqual(w.attempts[prior.Key], prior) || len(w.conflicts) != 1 || w.conflicts[0].PriorDigest != prior.RawDigest || w.conflicts[0].ConflictingDigest != Digest(conflicting) {
			return errors.New("producer conflict")
		}
	} else if ingestErr != nil {
		if ingestErr.Error() != exp.Cause || exp.OperationOutcome != operationForCause(ingestErr.Error()) || fmt.Sprint(a.States) != "[RECEIVED TERMINAL_INVALID]" || exp.Nonqualification != "NO_COMMITTED_RESULT" {
			return errors.New("producer rejection")
		}
		replay, replayResult, replayErr := w.Ingest(frozenReq, "assignment-1", raw, Digest(raw))
		if replayErr != nil || !reflect.DeepEqual(replay, a) || !reflect.DeepEqual(replayResult, SearchResult{}) {
			return errors.New("producer invalid replay")
		}
		return nil
	}
	var frozenResult SearchResult
	if err = readStrict(filepath.Join(dir, "RESULT.json"), &frozenResult); err != nil || !reflect.DeepEqual(result, frozenResult) {
		return errors.New("result mismatch")
	}
	if err = verifyExpectedResult(exp, result); err != nil {
		return err
	}
	replayAttempt, replayResult, replayErr := w.Ingest(frozenReq, "assignment-1", raw, Digest(raw))
	if replayErr != nil || !reflect.DeepEqual(replayAttempt, a) || !reflect.DeepEqual(replayResult, result) {
		return errors.New("producer replay")
	}
	var custody ParentCustodyEvidence
	var frozenReviewRequest ReviewRequest
	if readStrict(filepath.Join(dir, "CUSTODY.json"), &custody) != nil || readStrict(filepath.Join(dir, "REVIEW_REQUEST.json"), &frozenReviewRequest) != nil {
		return errors.New("review fixture parse")
	}
	q, err := NewReviewRequest(frozenReq, a, result, custody)
	if err != nil || !reflect.DeepEqual(q, frozenReviewRequest) {
		return errors.New("review request")
	}
	reviewRaw, err := os.ReadFile(filepath.Join(dir, "REVIEW_RAW"))
	if err != nil {
		return err
	}
	key, err := NewReviewerAttemptKey(q, custody.ReviewerAssignmentID, "review-attempt-1")
	if err != nil {
		return err
	}
	rw := NewReviewWriter()
	ra, review, err := rw.Ingest(q, key, reviewRaw, Digest(reviewRaw))
	var frozenReview Review
	var frozenReviewAttempt ReviewerAttempt
	reviewerAttemptPath := filepath.Join(dir, "REVIEW_ATTEMPT.json")
	if verifyAttemptFileRawBytes(reviewerAttemptPath) != nil {
		return errors.New("review attempt RawBytes")
	}
	if err != nil || readStrict(filepath.Join(dir, "REVIEW_EXPECTED.json"), &frozenReview) != nil || readStrict(reviewerAttemptPath, &frozenReviewAttempt) != nil || !reflect.DeepEqual(review, frozenReview) || !reflect.DeepEqual(ra, frozenReviewAttempt) || exp.ReviewVerdict != review.Verdict {
		return errors.New("review result")
	}
	replayRA, replayReview, replayErr := rw.Ingest(q, key, reviewRaw, Digest(reviewRaw))
	if replayErr != nil || !reflect.DeepEqual(replayRA, ra) || !reflect.DeepEqual(replayReview, review) {
		return errors.New("review replay")
	}
	var frozenAccount Account
	if err = readStrict(filepath.Join(dir, "ACCOUNT.json"), &frozenAccount); err != nil {
		return err
	}
	account, err := MakeAccount(frozenAccount.Selection, frozenReq, a, result, q, ra, review, exp.OutcomeCounts)
	if err != nil || !reflect.DeepEqual(account, frozenAccount) || !account.Balanced || account.MechanicalPercent != 100 || account.CustodyPercent != 100 || account.AccountingPercent != 100 || account.CriticalReviewPercent != 100 || account.SemanticUsefulnessQualified || account.FeatureIdentity != "UNRESOLVED" || exp.Nonqualification != "QUALIFIED_MECHANICALLY_ONLY" {
		return errors.New("account mismatch")
	}
	return nil
}

func verifyExpectedResult(exp expectedFixture, result SearchResult) error {
	if exp.Cause != "" || exp.OperationOutcome != result.Outcome || exp.Completeness != result.Completeness || exp.Denominator != result.Denominator || exp.Begun != result.Begun || exp.Completed != result.Completed || exp.Unevaluated != result.Unevaluated || exp.Work != result.WorkCharged || exp.Selected != len(result.Selected) || !reflect.DeepEqual(exp.Ledger, result.Ledger) {
		return errors.New("expected result")
	}
	counts := map[string]int{}
	for _, key := range MemberOutcomes {
		counts[key] = 0
	}
	for _, item := range result.Ledger {
		counts[item.Outcome]++
	}
	if !reflect.DeepEqual(counts, exp.OutcomeCounts) || result.Completed != len(result.Ledger) || result.Denominator != result.Completed+result.Unevaluated || result.WorkCharged != result.Completed*WorkPerMember || (result.Completeness == "COMPLETE") != (len(result.Ledger) == MemberCount && result.Begun == MemberCount && result.Completed == MemberCount && result.Unevaluated == 0) {
		return errors.New("ledger accounting")
	}
	return nil
}
func mustRead(p string) []byte { b, _ := os.ReadFile(p); return b }
func readStrict(p string, dst any) error {
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	return strictJSONFile(b, dst)
}
func strictJSONFile(b []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e := dec.Decode(dst); e != nil {
		return e
	}
	var extra any
	if e := dec.Decode(&extra); e == nil {
		return errors.New("trailing")
	}
	return nil
}
func sourceFiles(dir string) ([]FileIdentity, error) {
	var out []FileIdentity
	for _, n := range []string{"freeze.go", "freeze_test.go", "search.go", "search_test.go"} {
		b, e := os.ReadFile(filepath.Join(dir, n))
		if e != nil {
			return nil, e
		}
		out = append(out, FileIdentity{n, fileDigest(b), int64(len(b))})
	}
	return out, nil
}
func treeFiles(root string) ([]FileIdentity, error) {
	var out []FileIdentity
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if rel == "FREEZE.json" || strings.HasPrefix(rel, "execution/") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out = append(out, FileIdentity{rel, fileDigest(b), int64(len(b))})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}
