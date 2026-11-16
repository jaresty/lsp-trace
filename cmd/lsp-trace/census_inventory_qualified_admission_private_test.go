package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	continuation "lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
)

func qualifiedAdmissionFixture(t *testing.T) (censusQualifiedCLIEnvelopePrivate, censussourceplan.QualifiedInputV1, map[string]any) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	pin := "sha256:" + strings.Repeat("a", 64)
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "offline", Includes: []string{"a.go"}, Excludes: []string{"b.go"}, ApplicationRoots: []string{}, SourceID: "offline-list", ConfigDigest: pin, PolicyID: inventoryscope.PolicyConsensusV1}, Candidates: []inventoryscope.CandidateV1{}, Observations: []inventoryscope.ObservationV1{}, Resolutions: []inventoryscope.ResolutionV1{}}
	files := []censusacquisition.SourceFile{{Path: "a.go", URI: "file://" + root + "/a.go", LanguageID: "go"}, {Path: "b.go", URI: "file://" + root + "/b.go", LanguageID: "go"}}
	for i, f := range files {
		id, e := inventoryscope.CandidateIDV1(in.Declaration.SourceID, f.Path)
		if e != nil {
			t.Fatal(e)
		}
		in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: f.Path, CandidateID: id, LanguageID: f.LanguageID, DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "explicit-list", Digest: pin}})
	}
	m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	policy := pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 2, MaxContextFiles: 1, MaxMembers: 3, FileBytes: 4096, TotalBytes: 8192, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 2, MaxSymbols: 8, MaxDepth: 8, MaxStringBytes: 4096}, MaxTargets: 4, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 4}, MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 4096, MaxPathWork: 1000, TimeoutMS: 30000, RequestTimeoutMS: 1000, MaxResponseBytes: 4096, MaxMessages: 8}
	env := censusQualifiedCLIEnvelopePrivate{Family: censusQualifiedCLIFamilyPrivate, Universe: censusQualifiedUniversePrivate{Complete: true, Claim: "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED", ListMethod: "explicit offline list", Provenance: inventoryscope.ArtifactRefV1{ArtifactID: "list", Digest: pin}, OriginalDenominatorCount: m.OriginalDenominatorCount, OriginalDenominatorDigest: m.OriginalDenominatorDigest}, Originals: files, Qualification: in, Selection: inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "offline-selection"}, Workspace: root, PartitionSize: 16, Preparation: censusApplicationQualifiedOptionsPrivate{RevisionAssertion: censusInventorySourceRevision, Policy: policy, MaxArtifactBytes: 1 << 20, MaxMetadataBytes: 48000}, TimeoutMS: 30000, OutputDirectory: root, OutputName: "not-published"}
	q, e := validateCensusQualifiedCLIPrivate(env)
	if e != nil {
		t.Fatal("fixture validation", e)
	}
	_, identity, e := censusQualifiedCLIIdentityPrivate(env, q)
	if e != nil {
		t.Fatal(e)
	}
	h := map[string]any{"Schema": "private-connected-inventory-qualified-header.v1", "SourceRevision": censusInventorySourceRevision, "Workspace": root, "Workflow": "qualified-offline", "PlanIdentity": q.QualifiedDigest, "Scope": q.Scope.ScopeDigest, "Provenance": env.Universe.Claim, "Plan": q.AnalysisPlan, "UntouchedGaps": []string{"excluded original remains in qualified envelope"}, "Exclusions": []string{}, "QualifiedAdmission": map[string]any{"Qualified": q, "Envelope": env, "PreparationIdentity": identity}}
	return env, q, h
}
func qualifiedAdmissionDecode(t *testing.T, h map[string]any) (censusInventoryBridgeConfig, error) {
	t.Helper()
	// Independent wire projection also compiles against the pre-extension header.
	var wire struct {
		Schema, SourceRevision, Workspace, PlanIdentity, Workflow, Scope, Provenance string
		Plan                                                                         censussourceplan.Plan
		UntouchedGaps, Exclusions                                                    []string
		QualifiedAdmission                                                           *struct {
			Qualified           censussourceplan.QualifiedInputV1
			Envelope            censusQualifiedCLIEnvelopePrivate
			PreparationIdentity string
		} `json:",omitempty"`
	}
	raw, e := json.Marshal(h)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &wire); e != nil {
		t.Fatal(e)
	}
	headerRaw, e := json.Marshal(wire)
	if e != nil {
		t.Fatal(e)
	}
	config := censusInventoryBridgeConfig{Schema: "private-connected-inventory-bridge.v1", Action: "setup", Root: h["Workspace"].(string)}
	raw, e = json.Marshal(config)
	if e != nil {
		t.Fatal(e)
	}
	empty, _ := json.Marshal(config.Header)
	raw = bytes.Replace(raw, append([]byte(`"Header":`), empty...), append([]byte(`"Header":`), headerRaw...), 1)
	return decodeCensusInventoryBridge(raw)
}
func TestQualifiedInventoryHeaderAdmissionV1(t *testing.T) {
	env, q, h := qualifiedAdmissionFixture(t)
	c, e := qualifiedAdmissionDecode(t, h)
	if e != nil {
		t.Fatalf("ASSERT qualified full-envelope header admitted: FAIL %v", e)
	}
	store, _, heads, closeRoots, e := openCensusInventoryCursor(env.Workspace, c.Header.Workflow)
	if e != nil {
		t.Fatal(e)
	}
	defer closeRoots()
	cursor, e := store.initialize(context.Background(), c.Header)
	if e != nil {
		t.Fatalf("ASSERT qualified full-envelope header admitted: FAIL %v", e)
	}
	t.Log("ASSERT qualified full-envelope header admitted: PASS")
	raw, e := store.objects.Get(context.Background(), cursor.Header)
	if e != nil {
		t.Fatal(e)
	}
	var retained map[string]json.RawMessage
	if json.Unmarshal(raw, &retained) != nil || len(retained["QualifiedAdmission"]) == 0 {
		t.Fatal("ASSERT complete original envelope retained: FAIL")
	}
	t.Log("ASSERT complete original envelope retained: PASS")
	loaded, e := store.load(context.Background())
	if e != nil || loaded.Header != cursor.Header {
		t.Fatal("ASSERT qualified cursor load canonical identity: FAIL", e)
	}
	t.Log("ASSERT qualified cursor load canonical identity: PASS")
	request := continuation.Request{Plan: q.AnalysisPlan, PartitionSize: 16, WorkflowKey: c.Header.Workflow, HeadKey: c.Header.Workflow, Objects: store.objects, Heads: heads}
	c.ExpectedHead = censusInventoryDigest([]byte("synthetic-head"))
	c.Attempt = censusInventoryDigest([]byte("synthetic-attempt"))
	c.CanonicalJob = []byte("offline accounting declaration only")
	c.Deadline = time.Now().Add(10 * time.Second)
	cursor, permit, e := store.claimPartition(context.Background(), cursor, c, request)
	if e != nil {
		t.Fatal("ASSERT qualified durable claim identity: FAIL", e)
	}
	claimRaw, e := store.objects.Get(context.Background(), cursor.Partitions[0].Claim)
	var claim censusInventoryPartitionClaim
	if e != nil || json.Unmarshal(claimRaw, &claim) != nil || claim.Header != cursor.Header || !claim.Deadline.Equal(c.Deadline) {
		t.Fatal("ASSERT qualified durable claim identity: FAIL")
	}
	t.Log("ASSERT qualified durable claim identity: PASS")
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if permit.take(context.Background(), 0, q.AnalysisPlan.Partitions[0].Members) == nil {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Fatal("ASSERT qualified durable permit once: FAIL", won)
	}
	t.Log("ASSERT qualified durable permit once: PASS")
	if _, _, e = store.claimPartition(context.Background(), cursor, c, request); e == nil {
		t.Fatal("ASSERT qualified receiptless no replay: FAIL")
	}
	t.Log("ASSERT qualified receiptless no replay: PASS")
	h["QualifiedAdmission"].(map[string]any)["Envelope"] = func() censusQualifiedCLIEnvelopePrivate { env.OutputName = "different-preparation"; return env }()
	changed, e := qualifiedAdmissionDecode(t, h)
	if e == nil {
		if _, e = store.initialize(context.Background(), changed.Header); e == nil {
			t.Fatal("ASSERT changed preparation cannot reuse cursor: FAIL")
		}
	}
	t.Log("ASSERT changed preparation cannot reuse cursor: PASS")
}
func TestQualifiedInventoryHeaderRefusalsV1(t *testing.T) {
	for _, kind := range []string{"old-family", "old-plan-identity", "incomplete", "wrong-claim", "wrong-qualified", "wrong-scope", "wrong-plan", "wrong-preparation", "wrong-workspace", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			env, q, h := qualifiedAdmissionFixture(t)
			ext := h["QualifiedAdmission"].(map[string]any)
			switch kind {
			case "old-family":
				h["Schema"] = "private-connected-inventory-header.v1"
			case "old-plan-identity":
				h["PlanIdentity"] = q.AnalysisPlan.Identity
			case "incomplete":
				env.Universe.Complete = false
				ext["Envelope"] = env
			case "wrong-claim":
				env.Universe.Claim = "FILESYSTEM_VERIFIED"
				ext["Envelope"] = env
			case "wrong-qualified":
				q.QualifiedDigest = censusInventoryDigest([]byte("wrong"))
				ext["Qualified"] = q
			case "wrong-scope":
				h["Scope"] = "other"
			case "wrong-plan":
				h["Plan"] = censussourceplan.Plan{}
			case "wrong-preparation":
				ext["PreparationIdentity"] = censusInventoryDigest([]byte("wrong"))
			case "wrong-workspace":
				h["Workspace"] = "/other"
			case "oversize":
				h["UntouchedGaps"] = []string{strings.Repeat("x", 4<<20)}
			}
			if _, e := qualifiedAdmissionDecode(t, h); e == nil {
				t.Fatal("ASSERT qualified invalid header refused: FAIL", kind)
			}
			t.Log("ASSERT qualified invalid header refused: PASS", kind)
		})
	}
}
func rebuildQualifiedAdmissionHeader(t *testing.T, env censusQualifiedCLIEnvelopePrivate) (censusInventoryBridgeConfig, censussourceplan.QualifiedInputV1) {
	t.Helper()
	m, e := inventoryscope.QualifyV1(env.Qualification, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	env.Universe.OriginalDenominatorCount = m.OriginalDenominatorCount
	env.Universe.OriginalDenominatorDigest = m.OriginalDenominatorDigest
	env.Selection.ManifestDigest = m.ScopeDigest
	q, e := validateCensusQualifiedCLIPrivate(env)
	if e != nil {
		t.Fatal(e)
	}
	_, identity, e := censusQualifiedCLIIdentityPrivate(env, q)
	if e != nil {
		t.Fatal(e)
	}
	h := map[string]any{"Schema": "private-connected-inventory-qualified-header.v1", "SourceRevision": censusInventorySourceRevision, "Workspace": env.Workspace, "Workflow": "qualified-offline", "PlanIdentity": q.QualifiedDigest, "Scope": q.Scope.ScopeDigest, "Provenance": env.Universe.Claim, "Plan": q.AnalysisPlan, "UntouchedGaps": []string{}, "Exclusions": []string{}, "QualifiedAdmission": map[string]any{"Qualified": q, "Envelope": env, "PreparationIdentity": identity}}
	c, e := qualifiedAdmissionDecode(t, h)
	if e != nil {
		t.Fatal(e)
	}
	return c, q
}
func TestQualifiedInventoryNonExecutingStatesV1(t *testing.T) {
	for _, kind := range []string{"no-work", "included-unavailable", "expired-deadline"} {
		t.Run(kind, func(t *testing.T) {
			env, _, _ := qualifiedAdmissionFixture(t)
			if kind == "no-work" {
				env.Qualification.Declaration.Includes = []string{}
				env.Qualification.Declaration.Excludes = []string{"a.go", "b.go"}
			}
			if kind == "included-unavailable" {
				env.Originals[0].Disposition = census.FileUnreadable
			}
			cfg, q := rebuildQualifiedAdmissionHeader(t, env)
			s, objects, heads, closeRoots, e := openCensusInventoryCursor(env.Workspace, cfg.Header.Workflow)
			if e != nil {
				t.Fatal(e)
			}
			defer closeRoots()
			c, e := s.initialize(context.Background(), cfg.Header)
			if e != nil {
				t.Fatal(e)
			}
			before := c.Selector
			raw, e := s.objects.Get(context.Background(), c.Header)
			var retained struct {
				QualifiedAdmission struct {
					Qualified censussourceplan.QualifiedInputV1
				}
			}
			if e != nil || json.Unmarshal(raw, &retained) != nil || retained.QualifiedAdmission.Qualified.Scope.OriginalDenominatorCount != 2 || len(retained.QualifiedAdmission.Qualified.Originals) != 2 {
				t.Fatal("ASSERT qualified original denominator retained: FAIL")
			}
			if kind == "no-work" && (!q.NoWork || len(q.AnalysisPlan.Files) != 0 || len(q.AnalysisPlan.Partitions) != 0) {
				t.Fatal("ASSERT no-work not reminted as execution: FAIL")
			}
			if kind == "included-unavailable" && (q.NoWork || len(q.AnalysisPlan.Files) != 1 || q.AnalysisPlan.Files[0].Disposition != census.FileUnreadable || q.OriginalOrdinals[0] != 0) {
				t.Fatal("ASSERT included unavailable not pruned: FAIL")
			}
			cfg.ExpectedHead = censusInventoryDigest([]byte("head"))
			cfg.Attempt = censusInventoryDigest([]byte("attempt"))
			cfg.CanonicalJob = []byte("no authority")
			cfg.Deadline = time.Now().Add(5 * time.Second)
			if kind == "expired-deadline" {
				cfg.Deadline = time.Now().Add(-time.Second)
			}
			r := continuation.Request{Plan: q.AnalysisPlan, PartitionSize: 16, WorkflowKey: cfg.Header.Workflow, HeadKey: cfg.Header.Workflow, Objects: objects, Heads: heads}
			if _, _, e = s.claimPartition(context.Background(), c, cfg, r); e == nil {
				t.Fatal("ASSERT non-executing qualified state refused before reservation: FAIL", kind)
			}
			after, e := s.load(context.Background())
			if e != nil || before != after.Selector {
				t.Fatal("ASSERT non-executing refusal preserves custody: FAIL")
			}
			t.Log("ASSERT non-executing qualified state retained and refused before reservation: PASS", kind)
		})
	}
}
func TestQualifiedInventoryPreparationIdentityV1(t *testing.T) {
	for _, kind := range []string{"policy", "configuration", "method", "provenance", "excluded-original"} {
		t.Run(kind, func(t *testing.T) {
			env, _, _ := qualifiedAdmissionFixture(t)
			first, _ := rebuildQualifiedAdmissionHeader(t, env)
			s, _, _, closeRoots, e := openCensusInventoryCursor(env.Workspace, first.Header.Workflow)
			if e != nil {
				t.Fatal(e)
			}
			defer closeRoots()
			c, e := s.initialize(context.Background(), first.Header)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "policy":
				env.Preparation.Policy.MaxNodes++
			case "configuration":
				env.Preparation.MaxMetadataBytes--
			case "method":
				env.Universe.ListMethod += "-changed"
			case "provenance":
				env.Universe.Provenance.ArtifactID += "-changed"
			case "excluded-original":
				env.Originals[1].Disposition = census.FileUnreadable
			}
			changed, _ := rebuildQualifiedAdmissionHeader(t, env)
			raw, e := censusInventoryMarshal(changed.Header, 4<<20)
			if e != nil || censusInventoryDigest(raw) == c.Header {
				t.Fatal("ASSERT complete preparation changes canonical header: FAIL", kind)
			}
			if _, e = s.initialize(context.Background(), changed.Header); e == nil {
				t.Fatal("ASSERT changed valid preparation cannot reuse cursor: FAIL", kind)
			}
			t.Log("ASSERT complete valid preparation is bound to header and cursor: PASS", kind)
		})
	}
}
func TestQualifiedInventoryLegacyBytesV1(t *testing.T) {
	env, q, _ := qualifiedAdmissionFixture(t)
	h := censusInventoryHeader{Schema: "private-connected-inventory-header.v1", SourceRevision: censusInventorySourceRevision, Workspace: env.Workspace, Workflow: "legacy-offline", PlanIdentity: q.AnalysisPlan.Identity, Plan: q.AnalysisPlan, Scope: "legacy", Provenance: "legacy"}
	// Independent pre-extension projection preserves field order, nulls and LF.
	legacy := struct {
		Schema, SourceRevision, Workspace, PlanIdentity, Workflow, Scope, Provenance string
		Plan                                                                         censussourceplan.Plan
		UntouchedGaps, Exclusions                                                    []string
	}{h.Schema, h.SourceRevision, h.Workspace, h.PlanIdentity, h.Workflow, h.Scope, h.Provenance, h.Plan, h.UntouchedGaps, h.Exclusions}
	want, e := censusInventoryMarshal(legacy, 4<<20)
	if e != nil {
		t.Fatal(e)
	}
	store, _, _, closeRoots, e := openCensusInventoryCursor(env.Workspace, h.Workflow)
	if e != nil {
		t.Fatal(e)
	}
	defer closeRoots()
	c, e := store.initialize(context.Background(), h)
	if e != nil {
		t.Fatal(e)
	}
	got, e := store.objects.Get(context.Background(), c.Header)
	if e != nil || !bytes.Equal(got, want) {
		t.Fatal("ASSERT absent extension exact legacy bytes: FAIL")
	}
	t.Log("ASSERT absent extension exact legacy bytes: PASS")
}
