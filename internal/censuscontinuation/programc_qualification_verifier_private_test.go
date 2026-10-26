package censuscontinuation

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
)

func TestProgramCStage6PinnedSourceCustody(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_PROGRAM_C_STAGE6_PINNED_SOURCE_CALLER")
	}
	tests := []struct {
		path       string
		gitBlob    string
		sha256     string
		byteLength int
	}{
		{path: "programc_transition.go", gitBlob: "eb308697b4e8c968478180a8b775a314a43d43e4", sha256: "c67659e1df91a821a2152d6cb70973238a646fbca3d5c2d3fc0d5421f147d29e", byteLength: 38854},
		{path: "programc_stage6_adapter_private.go", gitBlob: "4144ff5a3980de6c56305d94eeb844a4b7f74cd9", sha256: "eaba97734c84b0c06da51a6b0d0e5d94c41dee81feded7634b7acb91c9439dfa", byteLength: 8699},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), tc.path))
			if err != nil {
				t.Fatal(err)
			}
			gitHash := sha1.New()
			_, _ = fmt.Fprintf(gitHash, "blob %d%c", len(raw), byte(0))
			_, _ = gitHash.Write(raw)
			digest := sha256.Sum256(raw)
			if got := hex.EncodeToString(gitHash.Sum(nil)); got != tc.gitBlob || hex.EncodeToString(digest[:]) != tc.sha256 || len(raw) != tc.byteLength {
				t.Fatalf("ASSERT_PROGRAM_C_STAGE6_PINNED_SOURCE_CUSTODY git_blob=%s sha256=%s bytes=%d", got, hex.EncodeToString(digest[:]), len(raw))
			}
		})
	}
}

func TestProgramCQualificationAdapterReportsCanonicalFiveSeedCount(t *testing.T) {
	entry := programCQualificationMultiSeedEntryFixture(t, 5)
	view, err := graph.NewNativeV5RepresentativeView(entry.outcomes[0].Source.GraphV5Bytes())
	if err != nil || len(view.InvocationSeeds()) != 5 {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_VALID_FIVE_SEED_NATIVE_V5 count=%d err=%v", len(view.InvocationSeeds()), err)
	}
	adapted, err := adaptProgramCQualificationStage6Entry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if !errors.Is(err, ErrInvalidProgramCTransition) || !strings.Contains(err.Error(), "invariant=invocation-seed-count observed=5 expected=1") || adapted.classification != "" || adapted.mechanics != nil {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_MULTI_SEED_PRECISE_REFUSAL adapted=%+v err=%v", adapted, err)
	}

	entry.nativeV5 = []byte("not-the-canonical-source")
	_, err = adaptProgramCQualificationStage6Entry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if !errors.Is(err, ErrInvalidProgramCTransition) || strings.Contains(err.Error(), "observed=5") {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_NO_UNBOUND_SEED_COUNT err=%v", err)
	}
}

func TestProgramCQualificationVerifierCancellationIsOneLoadAndZeroExposure(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || first.Status != ProgramCTransitionCommitted {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_CANCEL_SETUP_FIRST result=%+v err=%v", first, err)
	}
	request.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || second.Status != ProgramCTransitionCommitted {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_CANCEL_SETUP_SECOND result=%+v err=%v", second, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	heads.loads = 0
	puts, swaps := objects.puts, heads.cas
	authority, adaptations, err := verifyProgramCQualificationAuthority(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner)
	if !errors.Is(err, context.Canceled) || heads.loads != 1 || objects.puts != puts || heads.cas != swaps || authority.result != (ProgramCTransitionResult{}) || authority.terminalRaw != nil || authority.inventory != nil || adaptations != nil {
		t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_CANCEL_ZERO_EXPOSURE authority=%+v adaptations=%v loads=%d puts=%d/%d cas=%d/%d err=%v", authority, adaptations, heads.loads, objects.puts, puts, heads.cas, swaps, err)
	}
}

func programCQualificationMultiSeedEntryFixture(t *testing.T, invocationSeedCount int) programCTransitionReceiptInventoryEntry {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///fixture/input.go", Range: r, SelectionRange: r})
	}
	a, b, c := node("alpha", 0), node("beta", 2), node("gamma", 4)
	edges := []graph.Edge{
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 0, Character: 1}, End: graph.Position{Line: 0, Character: 2}}}},
		{CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 2, Character: 2}}}},
	}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{c, a, b}, Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	invocationSeeds := make([]graph.InvocationSeed, invocationSeedCount)
	seedResults := make([]graph.SeedResult, invocationSeedCount)
	for i := range invocationSeeds {
		label := fmt.Sprintf("seed-%d", i)
		invocationSeeds[i] = graph.InvocationSeed{Label: label, At: fmt.Sprintf("input.go:%d:1", i+1), ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
		seedResults[i] = graph.SeedResult{Label: label, PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID, b.ID, c.ID}, ReachedRelationIDs: []string{edges[0].RelationID, edges[1].RelationID}}
	}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation: graph.Invocation{
			Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: invocationSeeds,
			Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"},
			Expansion:  graph.ExpansionConfig{TopmostSiblings: true},
		},
		Nodes:        []graph.Node{c, a, b},
		Edges:        edges,
		Seeds:        seedResults,
		Summary:      graph.Summary{Complete: true},
		Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatalf("build exact multi-seed V5 fixture: %v", err)
	}
	outcomes := make([]programc.Outcome, 0, 6)
	for _, seedValue := range []uint64{1, 1, 1, 2, 2, 2} {
		outcome, failure := programc.Compute(retained, seedValue)
		if failure != nil || outcome.Outcome != "COMPLETE" {
			t.Fatalf("compute multi-seed fixture outcome=%+v failure=%v", outcome, failure)
		}
		outcomes = append(outcomes, outcome)
	}
	bindings := transitionBindings(transitionPlan(t, records2()[:1]))
	instability, err := (ProgramCProductionRunner{}).Instability(outcomes, bindings)
	if err != nil {
		t.Fatal(err)
	}
	receipt := programCTransitionReceipt{
		SchemaVersion: programCTransitionReceiptVersion,
		WorkflowKey:   "workflow", HeadKey: "program-c", Authority: 0, Bindings: bindings,
		ConstituentCount: 1, Ordinal: 0, ReceiptOrdinal: 1, PublicationByteLength: 1,
		PublicationSelector: "sha256:" + strings.Repeat("b", 64), ConstituentSelector: "sha256:" + strings.Repeat("c", 64),
		Outcome: "SUCCEEDED", ProgramCOutcome: "COMPLETE", ResultSHA256: transitionSHA(instability), ResultByteLength: len(instability), ResultBytes: instability,
		InstabilitySchema: programc.InstabilityVersionV2, InstabilityClaimCeiling: programc.InstabilityClaimCeilingV2,
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return programCTransitionReceiptInventoryEntry{ordinal: 0, selector: transitionSHA(receiptRaw), canonicalReceipt: receiptRaw, value: receipt, nativeV5: retained, outcomes: outcomes}
}
