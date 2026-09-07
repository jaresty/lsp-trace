package retainedcalls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
)

func fixture(t *testing.T, sites []graph.Range, repeats int) []byte {
	t.Helper()
	root := t.TempDir()
	seed := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	callee := (&url.URL{Scheme: "file", Path: filepath.Join(root, "b.go")}).String()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package p\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: seed, Range: graph.Range{End: graph.Position{Line: 9}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	b := graph.NewNode(graph.Item{Name: "B", Kind: 12, URI: callee, Range: graph.Range{End: graph.Position{Line: 9}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: []graph.Node{a, b}, Targets: []string{a.ID}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i := 0; i < repeats; i++ {
		r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: sites})
	}
	r.Invocation.Target = graph.Target{URI: seed}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: seed + ":0:0", ResolvedURI: seed}}
	ids := []string{a.ID, b.ID}
	rels := []string{r.Edges[0].RelationID}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: ids, ReachedRelationIDs: rels, ReachedEdges: r.Edges}}
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: seed, DownDepth: 1, UpDepth: 1, StartingNodeIDs: []string{a.ID}, Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: []string{a.ID}}, {Depth: 1, NodeIDs: []string{b.ID}}}, FrontierNodeIDs: []string{b.ID}, UpwardStartNodeIDs: []string{b.ID}, OutgoingRelationIDs: rels}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := graphprovenance.Capture(context.Background(), raw, root, seed, "session", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	return input
}
func twoSites() []graph.Range {
	return []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, {Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}}
}
func exported(t *testing.T, input []byte) Evidence {
	t.Helper()
	raw, err := Export(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(raw, Family, "v1"); err != nil {
		t.Fatal(err)
	}
	var e Evidence
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
func TestDistinctCallsitesGroupSupportAndTablesOnly(t *testing.T) {
	input := fixture(t, twoSites(), 7)
	e := exported(t, input)
	if len(e.Tables.Groups) != 1 || len(e.Tables.Occurrences) != 2 || e.Tables.SupportTotal != 1 || e.Tables.Groups[0].Receipt.SupportContribution != 1 {
		t.Fatal("ASSERT_TWO_OCCURRENCES_ONE_SUPPORT_GROUP")
	}
	if !bytes.Equal(e.InputBytes, input) || e.Ceilings.RepeatedReports != "UNAVAILABLE" {
		t.Fatal("ASSERT_MERGED_REPEAT_CEILING")
	}
	e.InputBytes = nil // decoder cannot access embedded input or graph bytes
	p, err := Reconstruct(e.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Edges) != 1 || len(p.Edges[0].CallSites) != 2 || p.Receipt.SupportTotal != 1 || p.Edges[0].RelationID != e.Tables.Groups[0].RelationID {
		t.Fatal("ASSERT_TABLES_ONLY_HISTORICAL_ROUNDTRIP")
	}
}
func TestUnreportedDoesNotInventZeroRange(t *testing.T) {
	for _, sites := range [][]graph.Range{nil, {{}}} {
		e := exported(t, fixture(t, sites, 2))
		p, err := Reconstruct(e.Tables)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Edges[0].CallSites) != len(sites) || len(e.Tables.Occurrences) != len(sites) {
			t.Fatal("ASSERT_NO_INVENTED_ZERO_RANGE")
		}
		if len(sites) == 0 && e.Tables.Groups[0].CallsiteState != "UNREPORTED" {
			t.Fatal("ASSERT_UNREPORTED_GROUP")
		}
	}
}
func TestContextIdentityOuterFormattingAndPerturbations(t *testing.T) {
	input := fixture(t, twoSites(), 2)
	base := exported(t, input)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, input, "", "  "); err != nil {
		t.Fatal(err)
	}
	formatted := exported(t, pretty.Bytes())
	if base.InputDigest == formatted.InputDigest || base.Tables.Contexts[0].ID != formatted.Tables.Contexts[0].ID || base.Tables.Occurrences[0].ID != formatted.Tables.Occurrences[0].ID {
		t.Fatal("ASSERT_OUTER_FORMAT_INVARIANCE")
	}
	for _, kind := range []string{"session", "generation", "graph_bytes", "capture"} {
		t.Run(kind, func(t *testing.T) {
			var e graphprovenance.Evidence
			_ = json.Unmarshal(input, &e)
			switch kind {
			case "session":
				e.SessionID = "another"
			case "generation":
				e.Generation = 9007199254740993
			case "graph_bytes":
				e.GraphBytes = append(e.GraphBytes, '\n')
				e.GraphDigest = digest(graphprovenance.Version+":graph", e.GraphBytes)
			case "capture":
				u, _ := url.Parse(e.WorkspaceURI)
				later, err := graphprovenance.Capture(context.Background(), e.GraphBytes, u.Path, e.SeedURI, e.SessionID, e.Generation, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(later, &e); err != nil {
					t.Fatal(err)
				}
			}
			raw, _ := json.Marshal(e)
			changed := exported(t, raw)
			if changed.Tables.Occurrences[0].ID == base.Tables.Occurrences[0].ID {
				t.Fatal("ASSERT_CONTEXT_PERTURBATION")
			}
			if kind == "capture" && (changed.Tables.Captures[0].Content != nil || changed.Tables.Captures[0].Status != "UNREADABLE") {
				t.Fatal("ASSERT_ABSENT_BYTES_HONEST")
			}
			if kind == "generation" && !bytes.Contains(changed.Tables.Contexts[0].Envelope, []byte(`9007199254740993`)) {
				t.Fatal("ASSERT_EXACT_INTEGER_CONTEXT")
			}
			if changed.Tables.Groups[0].RelationID != base.Tables.Groups[0].RelationID {
				t.Fatal("ASSERT_HISTORICAL_GROUP_STABILITY")
			}
		})
	}
}
func TestCoherentResealingCannotHideTableTampering(t *testing.T) {
	input := fixture(t, twoSites(), 2)
	mutations := map[string]func(*Evidence){
		"drop": func(e *Evidence) {
			e.Tables.Occurrences = e.Tables.Occurrences[:1]
			e.Tables.Groups[0].OccurrenceIDs = e.Tables.Groups[0].OccurrenceIDs[:1]
		},
		"extra": func(e *Evidence) {
			o := e.Tables.Occurrences[0]
			o.Range.End.Character++
			o.ID = occurrenceID(o)
			e.Tables.Occurrences = append(e.Tables.Occurrences, o)
			e.Tables.Groups[0].OccurrenceIDs = append(e.Tables.Groups[0].OccurrenceIDs, o.ID)
		},
		"duplicate": func(e *Evidence) { e.Tables.Occurrences = append(e.Tables.Occurrences, e.Tables.Occurrences[0]) },
		"range_resealed": func(e *Evidence) {
			e.Tables.Occurrences[0].Range.End.Character++
			e.Tables.Occurrences[0].ID = occurrenceID(e.Tables.Occurrences[0])
			e.Tables.Groups[0].OccurrenceIDs[0] = e.Tables.Occurrences[0].ID
		},
		"reclassified":        func(e *Evidence) { e.Tables.Groups[0].CallsiteState = "UNREPORTED" },
		"source_binding":      func(e *Evidence) { e.Tables.Occurrences[0].Binding.ReceiptIDs = nil },
		"source_substitution": func(e *Evidence) { e.Tables.Occurrences[0].Binding = e.Tables.Occurrences[1].Binding },
		"receipt":             func(e *Evidence) { e.Tables.Groups[0].Receipt.EvidenceClass = "AUTHENTICATED" },
		"source_receipt":      func(e *Evidence) { e.Tables.Captures = e.Tables.Captures[1:] },
		"endpoint":            func(e *Evidence) { e.Tables.Endpoints = e.Tables.Endpoints[1:] },
		"membership":          func(e *Evidence) { e.Tables.Groups[0].Memberships = nil },
		"support":             func(e *Evidence) { e.Tables.SupportTotal = 2; e.Tables.Groups[0].Receipt.SupportContribution = 2 },
		"outgoing":            func(e *Evidence) { e.Tables.Groups[0].OutgoingParticipation = false },
		"ceiling":             func(e *Evidence) { e.Ceilings.SourceAuthentication = "VERIFIED" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			e := exported(t, input)
			mutate(&e)
			e.InputDigest = digest(Version+":input", e.InputBytes)
			raw, _ := json.Marshal(e)
			if _, err := ValidateFor(raw, Family, "v1"); err == nil {
				t.Fatal("ASSERT_RESEALED_TAMPERING_REJECTED")
			}
		})
	}
}
func TestStrictInputAndFamilyBounds(t *testing.T) {
	input := fixture(t, twoSites(), 1)
	e := exported(t, input)
	raw, _ := json.Marshal(e)
	for name, bad := range map[string][]byte{"duplicate": bytes.Replace(raw, []byte(`"schema_version":`), []byte(`"schema_version":"x","schema_version":`), 1), "nested_case": bytes.Replace(raw, []byte(`"caller_uri":`), []byte(`"Caller_URI":`), 1), "trailing": append(append([]byte{}, raw...), []byte(` {}`)...)} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateFor(bad, Family, "v1"); err == nil {
				t.Fatal("ASSERT_STRICT_RECURSIVE_INPUT")
			}
		})
	}
	if _, err := schema.ValidateFor(raw, Family, "v1"); err == nil {
		t.Fatal("ASSERT_SCHEMA_ONLY_FAILS_CLOSED")
	}
	if _, err := ValidateFor(raw, Family, "v2"); err == nil {
		t.Fatal("ASSERT_UNSUPPORTED_VERSION")
	}
	if _, err := Export([]byte(`{}`)); err == nil {
		t.Fatal("ASSERT_INPUT_ADMISSION")
	}
	if _, err := Export(make([]byte, graphprovenance.MaxEnvelopeBytes+1)); err == nil {
		t.Fatal("ASSERT_INPUT_BUDGET")
	}
	if _, err := ValidateFor(make([]byte, MaxBytes+1), Family, "v1"); err == nil {
		t.Fatal("ASSERT_EXPORT_BUDGET")
	}
	e.Tables.Occurrences = make([]Occurrence, MaxRows+1)
	if _, err := Reconstruct(e.Tables); err == nil {
		t.Fatal("ASSERT_TABLE_ROW_BUDGET")
	}
}
func TestIdentityFixedVectors(t *testing.T) {
	vector := graphprovenance.Evidence{SchemaVersion: graphprovenance.Version, GraphBytes: []byte("{}"), GraphDigest: "sha256:graph", WorkspaceURI: "file:///repo", SeedURI: "file:///repo/a.go", SessionID: "s", Generation: 1, AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", SupplyStatus: "NO_NOTIFICATION_OBSERVATION", Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.Binding{}}
	if got := digest(Version+":context", canonical(vector)); got != "sha256:85f09899c3b2c4b147d056577cfe9189e6e58caeef68859e0214752d58f6114d" {
		t.Fatalf("ASSERT_FIXED_CONTEXT_VECTOR: %s", got)
	}
	got := digest(Version+":input", []byte("{}\n"))
	const want = "sha256:b01926e6d77b4b6a6f4627e7ecd4a854fcc2c8a607106303554f2ae1c281e7a0"
	if got != want {
		t.Fatalf("ASSERT_FIXED_INPUT_VECTOR: got %s want %s", got, want)
	}
	o := Occurrence{ContextID: "context", RelationID: "historical", CallerNodeID: "caller", CalleeNodeID: "callee", CallerURI: "file:///a.go", Range: twoSites()[0]}
	if got := occurrenceID(o); got != "sha256:7508335ee386648f67b703ec64e04ec83183036b9a9496395765514d8c1a744f" {
		t.Fatalf("ASSERT_FIXED_OCCURRENCE_VECTOR: %s preimage=%s", got, fmt.Sprint(o))
	}
}
