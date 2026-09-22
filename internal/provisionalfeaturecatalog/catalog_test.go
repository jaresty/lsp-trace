package provisionalfeaturecatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
)

type fixtureLookup struct{ body []byte }

func (l fixtureLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l.body...)}, nil
}

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	digestD = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	digestE = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	digestF = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

func inputs(t *testing.T) (targetpacket.Result, []describerequest.Record, []describeworker.InvocationRecord, []describeworker.ResponseRecord) {
	t.Helper()
	fx := retainedprojectiontestfixture.ValidV2Artifact(t)
	n := censusprogramc.Representative{Status: censusprogramc.CandidateStatus, Authority: 0, SourceGraphComplete: "UNKNOWN", CensusID: "census", ConstituentIdentity: "constituent", ConstituentOrdinal: 0, SelectionState: "SELECTED", SelectedNode: fx.NodeID, ClaimCeiling: "STRUCTURAL", BatchID: "batch", CommunityIdentity: "community", ExecutionBundleID: "bundle", SeedLabel: "seed", SeedAt: "at", Members: []string{"member"}, SCCMembers: []string{"member"}}
	c := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: fx.GraphV5Digest, GraphByteLength: int(fx.GraphV5ByteLen)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{n}}}
	packets, err := targetpacket.Build(targetpacket.Request{Census: c, Snapshots: []targetpacket.Snapshot{{ConstituentIdentity: "constituent", Raw: fx.Raw}}, Lookup: fixtureLookup{fx.Content}, Policy: sourceprojection.Policy{PolicyID: "p", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 10, MaxObjects: 10, MaxWork: 100, EnforceLimits: true}, ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: uint64(len(fx.Content)), MaxLogicalSelections: 1}, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	requests, err := describerequest.Build(packets, 90000)
	if err != nil {
		t.Fatal(err)
	}
	invs := make([]describeworker.InvocationRecord, len(requests))
	responses := make([]describeworker.ResponseRecord, len(requests))
	for i, r := range requests {
		b := describeworker.IdentityBinding{RequestRecordID: r.RecordID, MessageID: r.Envelope.MessageID, AttemptID: "attempt", WorkerSHA256: digestA, ModelSHA256: digestB, LibrarySHA256: digestC, SandboxExecutableSHA256: digestD, SandboxProfileSHA256: digestE, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: "sha256:" + r.Envelope.InputSHA256, GrammarSHA256: digestF, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 4096, StderrBytes: 1024, WorkBytes: 8192, TempBytes: 8192}}
		s := describeworker.SemanticResponse{Verdict: "SUPPORTED", TargetRole: "model target role", NearestOutwardConsumer: "unresolved", ConsumerNeed: "model need", ProvidedBehavior: "model behavior", BoundaryContribution: "model boundary", Limitations: []string{"bounded"}, Citations: []string{"C1"}}
		responses[i], err = describeworker.NewResponseRecord(b, s)
		if err != nil {
			t.Fatal(err)
		}
		invs[i], err = describeworker.NewInvocationRecord(b, describeworker.StatusSucceeded, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, responses[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	return packets, requests, invs, responses
}

func TestBuildZeroAndCompleteAccounting(t *testing.T) {
	c, err := Build(targetpacket.Result{State: targetpacket.StateEmpty}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Outcome() != OutcomeComplete || c.Accounting().Total != 0 || len(c.Entries()) != 0 {
		t.Fatalf("ASSERT_ZERO_BALANCED: %+v", c.Accounting())
	}
	packets, requests, invs, responses := inputs(t)
	c, err = Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	if c.Outcome() != OutcomeComplete || c.Accounting().Complete != 1 || c.Accounting().Total != 1 {
		t.Fatalf("ASSERT_COMPLETE_BALANCED: %+v", c.Accounting())
	}
	e := c.Entries()[0]
	if e.Authority != 0 || e.Accepted || e.Completeness != CompletenessUnknown || e.Status != StatusProvisional || e.TerminalOutcome != TerminalComplete {
		t.Fatalf("ASSERT_FIXED_ENVELOPE: %+v", e)
	}
	if e.Lineage.CommunityIdentity != "community" || e.Lineage.PacketID != packets.Packets[0].PacketID || e.Lineage.RequestRecordID != requests[0].RecordID || e.Lineage.InvocationID != invs[0].ID() || e.Lineage.ResponseID != responses[0].ID() {
		t.Fatalf("ASSERT_DISTINCT_LINEAGE: %+v", e.Lineage)
	}
	if e.Semantic.TargetRole != "model target role" || e.Semantic.TargetRoleIsModelText != true {
		t.Fatalf("ASSERT_MODEL_TEXT_NOT_IDENTITY: %+v", e.Semantic)
	}
}

func TestTerminalMappingAndDegradedRetention(t *testing.T) {
	packets, requests, _, responses := inputs(t)
	statuses := []struct {
		status describeworker.TerminalStatus
		want   TerminalOutcome
	}{{describeworker.StatusCancelled, TerminalCancelled}, {describeworker.StatusTimeout, TerminalTimeout}, {describeworker.StatusResourceLimit, TerminalResourceLimit}, {describeworker.StatusBackendFailure, TerminalBackendFailure}, {describeworker.StatusOutputInvalid, TerminalOutputInvalid}, {describeworker.StatusModelUnavailable, TerminalModelUnavailable}, {describeworker.StatusPolicyMismatch, TerminalPolicyMismatch}}
	for _, tc := range statuses {
		t.Run(string(tc.status), func(t *testing.T) {
			b := responses[0].Binding()
			b.AttemptID = "failure"
			inv, err := describeworker.NewInvocationRecord(b, tc.status, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, describeworker.ResponseRecord{})
			if err != nil {
				t.Fatalf("ASSERT_RESPONSE_FREE_FAILURE: %v", err)
			}
			c, err := Build(packets, requests, []describeworker.InvocationRecord{inv}, nil)
			if err != nil {
				t.Fatal(err)
			}
			entry := c.Entries()[0]
			if c.Outcome() != OutcomeDegraded || entry.TerminalOutcome != tc.want || entry.Lineage.ResponseID != "" || c.Accounting().Total != 1 || c.Accounting().sum() != 1 {
				t.Fatalf("ASSERT_DEGRADED_BALANCED_RETENTION: outcome=%s entry=%+v accounting=%+v", c.Outcome(), entry, c.Accounting())
			}
		})
	}
}

func TestJoinRejectsMissingDuplicateForeignAndUnused(t *testing.T) {
	packets, requests, invs, responses := inputs(t)
	cases := []struct {
		name string
		req  []describerequest.Record
		inv  []describeworker.InvocationRecord
		resp []describeworker.ResponseRecord
	}{
		{"missing-request", nil, invs, responses}, {"missing-invocation", requests, nil, responses}, {"missing-response", requests, invs, nil},
		{"duplicate-request", append(append([]describerequest.Record{}, requests...), requests...), invs, responses}, {"duplicate-invocation", requests, append(append([]describeworker.InvocationRecord{}, invs...), invs...), responses}, {"duplicate-response", requests, invs, append(append([]describeworker.ResponseRecord{}, responses...), responses...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build(packets, tc.req, tc.inv, tc.resp); err == nil {
				t.Fatal("ASSERT_JOIN_REJECTION")
			}
		})
	}
	foreign := requests[0]
	foreign.Lineage.PacketID = "foreign"
	if _, err := Build(packets, []describerequest.Record{foreign}, invs, responses); err == nil {
		t.Fatal("ASSERT_FOREIGN_REJECTED")
	}
}

func TestShuffleDeterminismAndDefensiveOwnership(t *testing.T) {
	packets, requests, invs, responses := inputs(t)
	c1, err := Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := c1.Bytes()
	r1, _ := RenderReview(c1)
	rand.New(rand.NewSource(7)).Shuffle(len(requests), func(i, j int) { requests[i], requests[j] = requests[j], requests[i] })
	c2, err := Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := c2.Bytes()
	r2, _ := RenderReview(c2)
	if !bytes.Equal(b1, b2) || r1 != r2 {
		t.Fatal("ASSERT_PERMUTATION_IDENTICAL")
	}
	entries := c1.Entries()
	entries[0].Lineage.Members[0] = "mutated"
	entries[0].Semantic.Limitations[0].Text = "mutated"
	b3, _ := c1.Bytes()
	if !bytes.Equal(b1, b3) {
		t.Fatal("ASSERT_DEFENSIVE_OWNERSHIP")
	}
}

func TestStrictParseTamperAndCoordinatedSubstitution(t *testing.T) {
	packets, requests, invs, responses := inputs(t)
	c, err := Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := c.Bytes()
	if _, err = Parse(raw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"unknown":0,"schema_version":`), 1), bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"x","schema_version":`), 1)} {
		if _, err := Parse(bad); err == nil {
			t.Fatal("ASSERT_STRICT_PARSE_REJECTION")
		}
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	entries := wire["entries"].([]any)
	entry := entries[0].(map[string]any)
	lineage := entry["lineage"].(map[string]any)
	lineage["community_identity"] = "substituted"
	entry["entry_id"] = recomputeMapID("lsp-trace.provisional-feature-entry.v1", entry, "entry_id")
	wire["catalog_id"] = recomputeMapID("lsp-trace.provisional-feature-catalog.v1", wire, "catalog_id")
	mutated, _ := json.Marshal(wire)
	if _, err := Parse(mutated); err == nil {
		t.Fatal("ASSERT_COORDINATED_SUBSTITUTION_REJECTED")
	}
}

func recomputeMapID(domain string, v map[string]any, field string) string {
	old := v[field]
	v[field] = ""
	b, _ := json.Marshal(v)
	v[field] = old
	s := sha256.Sum256(append([]byte(domain+"\x00"), b...))
	return "sha256:" + hex.EncodeToString(s[:])
}

func TestCorrectionAppendSupersedeAndTamper(t *testing.T) {
	packets, requests, invs, responses := inputs(t)
	prior, err := Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := prior.Bytes()
	corr := CorrectionInput{ActorID: "reviewer", ActorAuthority: AuthorityStakeholder, Reason: "correct bounded review state", InventoryStateDeltas: []InventoryStateDelta{{Kind: DeltaReviewState, From: "UNREVIEWED", To: "REVIEWED"}}, AffectedEntryIDs: []string{prior.Entries()[0].EntryID}, RebuildDisposition: RebuildRequired}
	next, err := BuildSuccessor(prior, []CorrectionInput{corr})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := prior.Bytes()
	if !bytes.Equal(before, after) || next.ID() == prior.ID() || next.PredecessorCatalogID() != prior.ID() || len(next.Corrections()) != 1 || next.Corrections()[0].SupersedesEntryIDs[0] != prior.Entries()[0].EntryID {
		t.Fatal("ASSERT_APPEND_ONLY_CORRECTION_IDENTITY")
	}
	foreign := corr
	foreign.SupersedesEntryIDs = []string{"sha256:foreign-entry"}
	if _, err := BuildSuccessor(prior, []CorrectionInput{foreign}); err == nil {
		t.Fatal("ASSERT_FOREIGN_SUPERSESSION_REJECTED")
	}
	bad := corr
	bad.InventoryStateDeltas = []InventoryStateDelta{{Kind: DeltaCommunityRename, From: "a", To: "b"}}
	if _, err := BuildSuccessor(prior, []CorrectionInput{bad}); err == nil {
		t.Fatal("ASSERT_COMMUNITY_RENAME_REJECTED")
	}
}

func TestNoSemanticMergeAndReviewRedaction(t *testing.T) {
	packets, requests, invs, responses := inputs(t)
	c, err := Build(packets, requests, invs, responses)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries()) != len(requests) {
		t.Fatal("ASSERT_NO_SEMANTIC_MERGE")
	}
	review, err := RenderReview(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PROVISIONAL — NOT ACCEPTED · Authority 0 · Completeness UNKNOWN", "Mechanical community community", "model target role", "Terminal accounting", "Limitations", "Citations"} {
		if !strings.Contains(review, want) {
			t.Fatalf("ASSERT_REVIEW_CONTENT missing %q", want)
		}
	}
	for _, secret := range []string{requests[0].Envelope.Prompt, requests[0].Envelope.SourceRevision, "runtime", "adapter", "model-v1", "/private/secret/source.go"} {
		if secret != "" && strings.Contains(review, secret) {
			t.Fatalf("ASSERT_REVIEW_REDACTION leaked %q", secret)
		}
	}
}

func TestSchemaHasNoDynamicPersistedFieldsAndDependencyBoundary(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(CatalogWire{}), reflect.TypeOf(Entry{}), reflect.TypeOf(Accounting{}), reflect.TypeOf(Correction{}), reflect.TypeOf(Limitation{}), reflect.TypeOf(Citation{})} {
		for i := 0; i < typ.NumField(); i++ {
			k := typ.Field(i).Type.Kind()
			if k == reflect.Map || k == reflect.Interface {
				t.Fatalf("ASSERT_CLOSED_SCHEMA: %s.%s", typ.Name(), typ.Field(i).Name)
			}
		}
	}
	if strings.Contains(reflect.TypeOf(Build).String(), "censusprogramc") || strings.Contains(reflect.TypeOf(Build).String(), "retainedprojection") {
		t.Fatal("ASSERT_DEPENDENCY_BOUNDARY")
	}
}
