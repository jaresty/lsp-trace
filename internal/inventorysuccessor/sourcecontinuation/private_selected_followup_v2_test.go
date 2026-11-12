package sourcecontinuation

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	pc "lsp-trace/internal/programc"
	pa "lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
)

// Adapted from programc/grouping_case_bundle_fixture_private_test.go's METHOD
// branch only. Untrusted serialized DATA, no provider, job, grant, qualification,
// PrivateProducerOriginals, private-field forgery or native source admission.
func followupReadyRD(t *testing.T) (cc.PrivateAllModeHandoff, cc.ContinuationContract) {
	t.Helper()
	marshal := func(v any) []byte {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(b, '\n')
	}
	kinds := []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}
	var occurrences []pa.MethodOccurrenceEvidence
	for n, k := range kinds {
		method, sr, tr := "textDocument/references", "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		if k == pa.TypedResolvesToDefinition {
			method, sr, tr = "textDocument/definition", "QUERY_SYMBOL", "DEFINITION_TARGET"
		}
		q, e := []byte("{\"query\":true}\n"), []byte("{\"evidence\":true}\n")
		occurrences = append(occurrences, pa.MethodOccurrenceEvidence{Identity: fmt.Sprintf("pure-%d", n), Kind: k, Ordinal: 0, From: "pure-a", To: "pure-b", SourceRole: sr, TargetRole: tr, Method: method, QuerySelector: "methods/query.json", QueryDigest: digest(q), QueryLength: len(q), EvidenceSelector: "methods/evidence.json", EvidenceDigest: digest(e), EvidenceLength: len(e), Provider: "controlled-data-only", ProviderVersion: "v1", SessionID: "not-a-live-session", Generation: 1, Revision: "pure-revision", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16"})
	}
	methods, e := pa.AdmitUntrustedMethodEvidenceEnvelope(pa.UntrustedMethodEvidenceEnvelope{Family: pa.MethodEvidenceFamily, Version: pa.MethodEvidenceFamily, ClaimCeiling: pa.MethodEvidenceClaimCeiling, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Provider: "controlled-data-only", ProviderVersion: "v1", SessionID: "not-a-live-session", Generation: 1, Revision: "pure-revision", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16", Occurrences: occurrences})
	if e != nil {
		t.Fatal(e)
	}
	typed, e := pa.PrepareTypedInput(pa.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: kinds})
	if e != nil {
		t.Fatal(e)
	}
	projection, e := pa.PreparePrivateLeidenProjection(typed)
	if e != nil {
		t.Fatal(e)
	}
	type row struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	type mapping struct {
		Identity         string `json:"identity"`
		Kind             string `json:"kind"`
		From             string `json:"from"`
		To               string `json:"to"`
		Ordinal          int    `json:"ordinal"`
		QuerySelector    string `json:"query_selector"`
		EvidenceSelector string `json:"evidence_selector"`
	}
	var rows []row
	var mappings []mapping
	var outcomes []rg.CandidateOutcome
	for _, o := range typed.Occurrences {
		rows = append(rows, row{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID})
		outcomes = append(outcomes, rg.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"})
	}
	for _, o := range projection.MethodEvidence() {
		mappings = append(mappings, mapping{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.QuerySelector, o.EvidenceSelector})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Identity < mappings[j].Identity })
	ledger := marshal(struct {
		Domain      string `json:"domain"`
		Version     string `json:"version"`
		Claim       string `json:"claim"`
		Occurrences []row  `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	receipt := marshal(struct {
		Family   string    `json:"family"`
		Version  string    `json:"version"`
		Mappings []mapping `json:"mappings"`
	}{"METHOD_MAPPING_RECEIPT", "v1", mappings})
	predecessors := []rg.PredecessorRecord{
		{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger},
		{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: []byte("{\"query\":true}\n")},
		{Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: pa.MethodEvidenceFamily, Bytes: projection.MethodCanonicalBytes()},
		{Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: receipt},
	}
	artifact, e := rg.BuildInput(rg.InputRequest{TypedInput: typed, Predecessors: predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: []rg.EndpointCustody{{Endpoint: "pure-a", PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"}, {Endpoint: "pure-b", PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"}}, OccurrenceAdmissionPolicyID: "pure-occurrence", CompositionPolicyID: "pure-composition", PrivacyPolicy: "pure-private", EligibleKinds: []pa.RelationKind{pa.TypedCalls, pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}, Limits: rg.ResourceLimits{MaxArtifacts: 16, MaxNodes: 8, MaxOccurrences: 8, MaxContributions: 8, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: rg.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}})
	if e != nil {
		t.Fatal(e)
	}
	input, e := rg.AdmitInput(artifact, predecessors)
	if e != nil {
		t.Fatal(e)
	}
	var rules []rg.GroupingPolicyRule
	for _, k := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
		r := rg.GroupingPolicyRule{RelationKind: k, Action: "INCLUDE", RationaleCode: "PURE_RD_ONLY", Weight: rg.Rational{Numerator: 1, Denominator: 1}}
		if k == "CALLS" {
			r.Action = "EXCLUDE"
			r.Weight.Numerator = 0
		}
		rules = append(rules, r)
	}
	policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "PURE_RD_ONLY", Epoch: 1, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: "pure-case", RunID: "pure-ready-RD", SelectorID: "pure-selector", SelectorBytes: []byte("DATA_ONLY_NOT_QUERY_EXECUTION"), Input: input, Policy: policy, Seed: 3, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxCanonicalBytes: 1 << 20, MaxOccurrences: 8, MaxPairContributions: 8, MaxPairs: 8, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 30}})
	if e != nil {
		t.Fatal(e)
	}
	result, e := pc.EvaluateGroupingCasePrivate(frozen)
	if e != nil {
		t.Fatal(e)
	}
	var slots []cc.PrivateAllModeSlot
	var entries []cc.PrivateAllModeEntry
	for i, m := range []string{"RD", "C", "CRD", "D", "CD", "R", "CR"} {
		s := cc.PrivateAllModeSlot{Ordinal: i, Mode: m, RunID: "pure-ready-" + m, SeedNamespace: "pure-selector", Seed: 3}
		slots = append(slots, s)
		entry := cc.PrivateAllModeEntry{Slot: s, Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: "pure-lineage", MissingReason: "CONTROLLED_MISSING_DATA"}
		if m == "RD" {
			entry = cc.PrivateAllModeEntry{Slot: s, Disposition: cc.PrivateAllModeReady, Lineage: "pure-lineage", Input: input, Policy: policy, Case: frozen, Result: result}
		}
		entries = append(entries, entry)
	}
	h, e := cc.NewPrivateAllModeHandoff("pure-workspace", "pure-revision", slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	pin := "sha256:" + strings.Repeat("0", 64)
	c, e := cc.NewContinuationContract(cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "PURE_NO_ORIGINALS"}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
	if e != nil {
		t.Fatal(e)
	}
	return h, c
}

// Store observations only. No production hook or native operation is installed.
type followupStore struct {
	*selectedStoreV2
	bounds         int
	putIDs, getIDs []string
	corrupt        bool
}

func (s *followupStore) MaxObjectBytes() int64 { s.bounds++; return s.selectedStoreV2.MaxObjectBytes() }
func (s *followupStore) Put(c context.Context, b []byte) (string, error) {
	id, e := s.selectedStoreV2.Put(c, b)
	if e == nil {
		s.putIDs = append(s.putIDs, id)
	}
	return id, e
}
func (s *followupStore) Get(c context.Context, id string) ([]byte, error) {
	b, e := s.selectedStoreV2.Get(c, id)
	s.getIDs = append(s.getIDs, id)
	if e == nil && s.corrupt && len(b) > 0 {
		b[0] ^= 1
	}
	return b, e
}
func followupMemory(t *testing.T) *followupStore {
	t.Helper()
	s, e := newSelectedStoreV2(8<<20, 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.close() })
	return &followupStore{selectedStoreV2: s}
}
func followupSource(deadline time.Time) PrivateAllModeSourcesRequest {
	return PrivateAllModeSourcesRequest{PolicyID: "PURE_NO_ORIGINALS", Deadline: deadline, Limits: tp.PrivateSourceLimits{MaxDocumentBytes: 1024, MaxTotalBytes: 4096, MaxWork: 10000, MaxObjects: 1000}}
}

func TestSelectedV2FollowupActualAggregateWork(t *testing.T) {
	h, c := followupReadyRD(t)
	deadline := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	lower := cc.Run(ctx, cc.Request{PrivateAllMode: &h, Workspace: "pure-workspace", Contract: c, Store: followupMemory(t), StopAfter: cc.StopAfterSourceIntents})
	if lower.Err != nil || lower.PrivateAllMode == nil {
		t.Fatal("valid lower READY fixture", lower.Err)
	}
	base := *lower.PrivateAllMode
	if base.Counts.Ready != 1 || base.Counts.Missing != 6 || len(base.Nominations) == 0 || len(base.Intents) == 0 {
		t.Fatal("not a real READY lower fixture", base.Counts)
	}
	q := followupSource(deadline)
	body := []byte("x")
	q.Objects = []sourceobject.Object{{Identity: sourceobject.Identity{Digest: digest(body), ByteLength: 1}, Bytes: body}}
	required := len(base.Intents) + len(base.Nominations) + len(q.Objects)
	if required-1 < len(base.Intents) {
		t.Fatal("below case would hit intent-count guard")
	}
	for _, budget := range []int{required - 1, required, required + 1} {
		t.Run(fmt.Sprintf("budget_%d", budget), func(t *testing.T) {
			s := followupMemory(t)
			q := q
			q.Limits.MaxWork = budget
			if e := preflight(ctx, q, c, s, "pure-workspace", StopAfterPrivateSourceRequests); e != nil {
				t.Fatal("legal preflight", e)
			}
			s.bounds = 0 // observation counter only; no native operation exists.
			out, mats, admissions, e := prepare(ctx, base, h, c, q, s, "pure-workspace", StopAfterPrivateSourceRequests)
			if budget < required {
				if e == nil || e.Error() != "private source aggregate work bound" || len(mats) != 0 || len(admissions) != 0 || s.bounds != 1 {
					t.Fatal("FOLLOWUP_WORK: exact aggregate rejection before post-guard processing", e, s.bounds)
				}
				t.Log("FOLLOWUP_WORK_AVAILABLE: A-fail: private source aggregate work bound")
			} else {
				if e != nil || len(mats) != 3 || len(admissions) != 0 || s.bounds != 2 || out.Counts.Intents != len(base.Intents) || out.Counts.Unresolved != len(base.Intents) {
					t.Fatal("FOLLOWUP_WORK: valid subsequent preparation", e, out.Counts, s.bounds)
				}
				t.Log("FOLLOWUP_WORK_AVAILABLE: A-pass: valid subsequent preparation")
			}
			if s.puts != 0 || s.gets != 0 {
				t.Fatal("prepare performed store effects")
			}
		})
	}
	t.Logf("FOLLOWUP_WORK: U=0 I=%d N=%d objects=1 K=%d; U>0/293 NOT EXERCISED", len(base.Intents), len(base.Nominations), required)
}

func TestSelectedV2FollowupReadyRDWholeResume(t *testing.T) {
	h, c := followupReadyRD(t)
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprintf("corrupt_%t", corrupt), func(t *testing.T) {
			deadline := time.Now().Add(10 * time.Second)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			s := followupMemory(t)
			q := followupSource(deadline)
			run := Run(ctx, Request{PrivateSources: &q, PrivateAllMode: &h, Workspace: "pure-workspace", Contract: c, Store: s, StopAfter: StopAfterPrivateSourceRequests})
			if run.Err != nil || run.PrivateSources == nil {
				t.Fatal("READY metadata Run", run.Err)
			}
			var base cc.PrivateAllModeResult
			if json.Unmarshal(s.lower, &base) != nil || base.Counts.Ready != 1 || base.Counts.Missing != 6 || len(base.Nominations) == 0 || len(base.Intents) == 0 {
				t.Fatal("actual emitted lower READY metadata")
			}
			out := run.PrivateSources
			if len(out.Outcomes) != len(base.Intents) {
				t.Fatal("every endpoint row must be preserved", len(out.Outcomes), len(base.Intents))
			}
			typed, missing := 0, 0
			roles := map[string]bool{}
			for row, intent := range base.Intents {
				o := out.Outcomes[row]
				if o.Intent != row || o.Status != "UNRESOLVED" || o.AdmissionID != "" || o.RequestID != "" {
					t.Fatal("every endpoint row remains unresolved", row, o)
				}
				if intent.Endpoint.Kind != "" {
					typed++
					roles[intent.Endpoint.Role] = true
					if intent.Mode != "RD" || intent.Endpoint.Disposition != "INCLUDED" || intent.BindingState != "UNRESOLVED" || o.Gap != "NO_OPAQUE_PRODUCER_ORIGINALS" {
						t.Fatal("typed missing-original gap", intent, o)
					}
				} else {
					missing++
					if intent.Gap != cc.PrivateAllModeDeclaredMissing || o.Gap != intent.Gap {
						t.Fatal("declared missing gap", intent, o)
					}
				}
			}
			if typed == 0 || missing != 6 || !roles["DEFINITION_TARGET"] || !roles["REFERENCED_SYMBOL"] || out.Counts.Intents != len(base.Intents) || out.Counts.Unresolved != len(base.Intents) || out.Counts.Admitted != 0 || len(out.Units) != 0 || len(out.Requests) != 0 || out.Counts.Requests != 0 || len(out.Expected.state.admissions) != 0 || len(q.Originals) != 0 || s.puts != 7 {
				t.Fatal("numerical READY is not native admission", out.Counts, roles)
			}
			s.corrupt = corrupt
			resumed := Resume(ctx, ResumeRequest{PrivateSources: &PrivateAllModeSourcesResume{Version: PrivateAllModeSourcesVersion, Expected: out.Expected, ExpectedHandoff: h}, Workspace: "pure-workspace", Store: s, StopAfter: StopAfterPrivateSourceRequests, Selector: out.CheckpointID})
			if corrupt {
				if resumed.Err == nil || resumed.PrivateSources != nil || s.gets == 0 {
					t.Fatal("FOLLOWUP_READY_READBACK: corrupted material accepted")
				}
				t.Log("FOLLOWUP_READY_READBACK: A-fail: corrupted material refused")
				return
			}
			if resumed.Err != nil || resumed.PrivateSources == nil || !sameSelectedReadbackV2(*out, *resumed.PrivateSources) || s.puts != 7 || s.gets != 7 {
				t.Fatal("FOLLOWUP_READY_READBACK: exact whole Resume", resumed.Err)
			}
			sort.Strings(s.putIDs)
			sort.Strings(s.getIDs)
			if !reflect.DeepEqual(s.putIDs, s.getIDs) {
				t.Fatal("not the same seven material identities")
			}
			t.Logf("FOLLOWUP_READY_READBACK: A-pass: exact seven materials; typed=%d missing=%d ZERO_NATIVE_ADMISSIONS", typed, missing)
		})
	}
}
