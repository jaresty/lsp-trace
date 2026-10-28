//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/filter"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/seedformat"
)

func TestScopedPartialInspectionPreflightPrivate(t *testing.T) {
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		selection, authority, binding, req := closedInputsPrivate(t, ctx, f)
		for _, kind := range []string{"query-set", "session", "budget", "context-nomination", "copied-guard", "foreign-manager"} {
			request := req
			manager := f.Manager
			original := selection.state.guard
			switch kind {
			case "query-set":
				request.FrozenQueryURIsPrivate[0] = "file:///arbitrary.go"
			case "session":
				request.Context.SessionID = "foreign"
			case "budget":
				request.Limits.MaxRequests++
			case "context-nomination":
				request.Root.Locator.URI = f.ContextURI
			case "copied-guard":
				copyGuard := *original
				selection.state.guard = &copyGuard
				selection.state.adapter.guard = &copyGuard
			case "foreign-manager":
				manager = f.ForeignManager(t)
			}
			before, prepBefore := f.Traffic()
			out, seeds, err := AcquireScopedPartialInspectionPrivate(ctx, manager, selection, authority, binding, request)
			after, prepAfter := f.Traffic()
			selection.state.guard = original
			selection.state.adapter.guard = original
			if err == nil || len(out) != 0 || len(seeds) != 0 || selection.state.spent || before != after || prepBefore != prepAfter {
				t.Fatalf("ASSERT_PARTIAL_PREFLIGHT_NO_ACQUIRE: %s err=%v", kind, err)
			}
		}
	})
}

func TestScopedPartialInspectionConnectedPrivate(t *testing.T) {
	for _, scenario := range []string{"outside-only", "inside-context", "alias-file-equal", "alias-file-different", "alias-directory-equal", "alias-directory-different"} {
		t.Run(scenario, func(t *testing.T) {
			testScopedPartialInspectionConnectedPrivate(t, scenario)
		})
	}
}

func testScopedPartialInspectionConnectedPrivate(t *testing.T, scenario string) {
	insideContext := scenario != "outside-only"
	alias := strings.HasPrefix(scenario, "alias-")
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		one := 1
		zero := uint32(0)
		files := f.adapter.files
		outside := (&url.URL{Scheme: "file", Path: filepath.Join(f.Workspace, "outside.go")}).String()
		if err := os.WriteFile(filepath.Join(f.Workspace, "outside.go"), []byte("forbidden outside source"), 0600); err != nil {
			t.Fatal(err)
		}
		var calls []string
		f.child.mu.Lock()
		f.child.response = func(m lspwire.Message) json.RawMessage {
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				Item lsp.CallHierarchyItem `json:"item"`
			}
			if err := json.Unmarshal(m.Params, &p); err != nil {
				t.Fatal(err)
			}
			calls = append(calls, m.Method+":"+p.TextDocument.URI+p.Item.URI)
			var v any = []any{}
			switch m.Method {
			case "textDocument/prepareCallHierarchy":
				v = []lsp.CallHierarchyItem{scopeAdapterItem(p.TextDocument.URI)}
			case "callHierarchy/outgoingCalls":
				if p.Item.URI == files[0].URI {
					if alias {
						// The closed producer already bound its source capability before
						// issuing this real Manager query. Replace only owned fixture paths.
						allowed := filepath.Join(f.Workspace, fixtureScopePaths[5])
						forbiddenDir := filepath.Join(f.Workspace, "outside-six")
						if err := os.Mkdir(forbiddenDir, 0700); err != nil {
							t.Fatal(err)
						}
						forbidden := filepath.Join(forbiddenDir, filepath.Base(allowed))
						content := []byte("nonce outside-six different bytes")
						if strings.HasSuffix(scenario, "-equal") {
							content = []byte("package fixture // " + fixtureScopePaths[5] + "\n")
						}
						if err := os.WriteFile(forbidden, content, 0600); err != nil {
							t.Fatal(err)
						}
						target := forbidden
						if strings.Contains(scenario, "-directory-") {
							allowed, target = filepath.Dir(allowed), forbiddenDir
						}
						if err := os.Rename(allowed, allowed+"-saved"); err != nil {
							t.Fatal(err)
						}
						relative, err := filepath.Rel(filepath.Dir(allowed), target)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(relative, allowed); err != nil {
							t.Fatal(err)
						}
					}
					row := lsp.CallHierarchyOutgoingCall{To: scopeAdapterItem(outside), FromRanges: []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}}
					v = []lsp.CallHierarchyOutgoingCall{row, row} // preserve actual response duplicates
					if insideContext {
						v = []lsp.CallHierarchyOutgoingCall{row, row, {To: scopeAdapterItem(f.ContextURI), FromRanges: row.FromRanges}}
					}
				}
			case "callHierarchy/incomingCalls":
				if insideContext && p.Item.URI == f.ContextURI {
					v = []lsp.CallHierarchyIncomingCall{{From: scopeAdapterItem(f.ContextURI), FromRanges: []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}}}
				}
			}
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
		f.child.mu.Unlock()
		targets := []acquisitionmanifest.Target{}
		seeds := []seedformat.Seed{}
		// All four nomination paths are exercised; this is four fake targets, not the
		// frozen 25-target real workload. No context-only file is nominated.
		for i, file := range files {
			if i >= 4 || file.Path != fixtureScopePaths[i] {
				t.Fatalf("ASSERT_PARTIAL_EXACT_FOUR_NOMINATION_PATHS: %d %s", i, file.Path)
			}
			id := fmt.Sprintf("peer%d", i)
			if i == 0 {
				id = "root"
			}
			targets = append(targets, acquisitionmanifest.Target{ID: id, Locator: acquisition.Locator{URI: file.URI, Line: &zero, Character: &zero, LanguageID: "go"}, DownDepth: &one, UpDepth: &one})
			seeds = append(seeds, seedformat.Seed{Type: seedformat.PositionType, Position: &seedformat.Position{Label: id, Path: file.Path, Line: 1, Column: 1}})
		}
		manifest := acquisitionmanifest.Manifest{SchemaVersion: acquisitionmanifest.ManifestVersion, CoordinateConvention: "zero-based-session", Root: targets[0], RequiredTargets: targets[1:]}
		seedBytes, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &one, UpDepth: &one}, Seeds: seeds}, f.Workspace)
		if err != nil {
			t.Fatal(err)
		}
		selection, binding, err := f.Mint(ctx, "partial-inspection-fixture", manifest, seedBytes)
		if err != nil {
			t.Fatal(err)
		}
		authority, err := acquisitionauthority.MintSeedAuthority(binding, seedBytes, seedbinding.CallerAssertedLocal, false, "", true)
		if err != nil {
			t.Fatal(err)
		}
		req, err := manifest.Request(acquisitionmanifest.SliceV3)
		if err != nil {
			t.Fatal(err)
		}
		req.Context = acquisition.AcquisitionContext{SessionID: f.Identity.SessionID, Generation: f.Identity.Generation, PositionEncoding: "utf-16"}
		identity, _ := json.Marshal(req)
		req.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(identity))
		raw, retainedSeeds, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(retainedSeeds, seedBytes) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_CANONICAL_SEEDS")
		}
		envelope, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil {
			t.Fatal(err)
		}
		r := envelope.Evidence.Acquisition
		for i, target := range r.Targets {
			if target.Requested.ID != targets[i].ID || target.Requested.Locator.URI != files[i].URI || target.Requested.Locator.URI == f.ContextURI || target.Resolution.Status != acquisition.Resolved {
				t.Fatalf("ASSERT_PARTIAL_CONNECTED_TARGET_ORDER_AND_NOMINATION: %d %+v", i, target)
			}
		}
		wantEdges, wantRows := 1, 2
		if insideContext {
			wantEdges, wantRows = 3, 3
		}
		if len(r.EdgeObservations) != wantEdges {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_REQUEST_RELATION_OBSERVATION")
		}
		duplicateRows := false
		for _, record := range r.Requests {
			if record.Method == "callHierarchy/outgoingCalls" && record.NodeID == r.Targets[0].Resolution.Identity.ID {
				var rows []lsp.CallHierarchyOutgoingCall
				if json.Unmarshal(record.Response, &rows) != nil || len(rows) != wantRows || rows[0].To.URI != outside || rows[1].To.URI != outside || !bytes.Equal(rows[0].To.Data, rows[1].To.Data) {
					t.Fatal("ASSERT_PARTIAL_CONNECTED_RESPONSE_ORDER_DUPLICATES")
				}
				duplicateRows = true
			}
		}
		if !duplicateRows {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ACTUAL_RESPONSE")
		}
		secondElementBound := false
		for _, binding := range envelope.Evidence.Bindings {
			if binding.URI == outside && strings.Contains(binding.Pointer, "/response/1/to/") && len(binding.ReceiptIDs) == 1 {
				secondElementBound = true
			}
		}
		if !secondElementBound {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_LEXICAL_DUPLICATE_BINDING")
		}
		if !envelope.FrozenQueryScope || !envelope.Partial || envelope.Authority != 0 || envelope.Accepted || envelope.ReplayAdmitted || envelope.SourceCompleteness != "UNKNOWN" || r.Graph.Summary.Complete || len(r.Graph.Edges) != wantEdges || len(r.Graph.Edges[0].CallSites) != 1 || len(r.Targets) != 4 {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_RETAINED_CEILING")
		}
		wantStarts := 1
		if insideContext {
			wantStarts = 2
		}
		if len(r.Targets[0].Incoming.StartIDs) != wantStarts || len(r.Graph.Frontier) != 2 {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_INCOMING_START: %+v frontier=%+v", r.Targets[0].Incoming, r.Graph.Frontier)
		}
		startURIs := map[string]bool{}
		for _, startID := range r.Targets[0].Incoming.StartIDs {
			for _, node := range r.Graph.Nodes {
				if node.ID == startID {
					startURIs[node.URI] = true
				}
			}
		}
		if len(startURIs) != wantStarts || !startURIs[outside] || (insideContext && !startURIs[f.ContextURI]) {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_EXACT_INCOMING_START_SET: %v", startURIs)
		}
		if insideContext {
			incomingC := 0
			for _, call := range calls {
				if call == "callHierarchy/incomingCalls:"+f.ContextURI {
					incomingC++
				}
			}
			if incomingC != 1 {
				t.Fatalf("ASSERT_PARTIAL_CONNECTED_CONTEXT_METHOD_URI: calls=%v", calls)
			}
			capturedC, retainedC := false, false
			for _, row := range envelope.Evidence.Captures {
				if row.URI == f.ContextURI {
					expected := []byte("package fixture // " + fixtureScopePaths[5] + "\n")
					if alias {
						// Actual no-follow capture refusal, not an FD-level no-open spy.
						wantStatus := "NONREGULAR"
						if strings.Contains(scenario, "-directory-") {
							wantStatus = "UNREADABLE"
						}
						if row.Status != wantStatus || len(row.Content) != 0 {
							t.Fatalf("ASSERT_PARTIAL_CONNECTED_ALIAS_NOFOLLOW: status=%s content=%q", row.Status, row.Content)
						}
					} else if row.Status != "READABLE" || !bytes.Equal(row.Content, expected) {
						t.Fatalf("ASSERT_PARTIAL_CONNECTED_CONTEXT_BYTES: status=%s content=%q", row.Status, row.Content)
					}
					capturedC = true
				}
			}
			for _, edge := range r.Graph.Edges {
				for _, node := range r.Graph.Nodes {
					if node.URI == f.ContextURI && edge.CallerNodeID == node.ID && edge.CalleeNodeID == node.ID && len(edge.CallSites) == 1 {
						retainedC = true
					}
				}
			}
			if !capturedC || !retainedC {
				t.Fatal("ASSERT_PARTIAL_CONNECTED_CONTEXT_CAPTURE_AND_OCCURRENCE")
			}
		}
		for _, c := range calls {
			if strings.Contains(c, outside) || c == "callHierarchy/incomingCalls:"+files[0].URI {
				t.Fatalf("ASSERT_PARTIAL_CONNECTED_NO_EXTERNAL_OR_RESEED: %s", c)
			}
		}
		withheld := 0
		for _, row := range envelope.Evidence.Captures {
			if row.URI == outside {
				if row.Status != graphprovenance.PolicyWithheldPrivateV1 || len(row.Content) != 0 {
					t.Fatal("ASSERT_PARTIAL_CONNECTED_WITHHELD")
				}
				withheld++
			}
		}
		if withheld != 1 {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_WITHHELD_COUNT")
		}
		projection, err := inspection.ProjectAllSeeds(envelope.Evidence.GraphBytes)
		if err != nil {
			t.Fatal(err)
		}
		projectionBytes, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		comparison, err := filter.ProjectPairwise(projectionBytes, "root", "peer1")
		if err != nil {
			t.Fatal(err)
		}
		for _, edge := range r.Graph.Edges {
			found := false
			for _, reference := range comparison.Partitions.CallRelations.LeftOnly {
				found = found || reference == edge.RelationID
			}
			if !found {
				t.Fatalf("ASSERT_PARTIAL_CONNECTED_EVERY_RELATION_CLASSIFIED: %s", edge.RelationID)
			}
		}
		out, _ := json.MarshalIndent(comparison, "", "  ")
		if len(projection.Records.CallRelations) != wantEdges || len(projection.Records.Frontier) != 2 || !strings.Contains(string(out), r.Graph.Edges[0].RelationID) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_EXISTING_ENGINES")
		}
		// Neither projection engine acquires source. The original private source
		// carrier remains intact alongside its read-only graph-derived views.
		afterViews, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil || afterViews.WithheldCount != 1 || !bytes.Equal(afterViews.Evidence.GraphBytes, envelope.Evidence.GraphBytes) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_CARRIER_SURVIVES_VIEWS")
		}
		if err := inspection.ValidateAllSeedAccounting(projection); err != nil {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_INSPECTION_ACCOUNTING", err)
		}
		if projection.Accounting.Truncated || projection.Accounting.TraversalComplete || projection.Accounting.SourceGraphComplete != "UNKNOWN" || comparison.GlobalBoundary.Truncated || comparison.GlobalBoundary.TraversalComplete || comparison.GlobalBoundary.SourceGraphComplete != "UNKNOWN" {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_COVERAGE_CEILING: inspection=%+v filter=%+v", projection.Accounting, comparison.GlobalBoundary)
		}
		if comparison.Accounting.RequestedSeedCount != 4 || comparison.Accounting.SuccessfulSeedCount != 4 || comparison.Accounting.FailedSeedCount != 0 || len(comparison.Partitions.CallRelations.LeftOnly) != wantEdges || len(comparison.Partitions.CallRelations.Shared) != 0 || len(comparison.Partitions.CallRelations.RightOnly) != 0 {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_REFERENCE_CLASSIFICATION: %s", out)
		}
		t.Logf("PRODUCT_CHECKPOINT: four deterministic nominated targets; scenario=%s retained CALLS=%d (root-only references), frontier=2; incoming(B) policy-stopped, never incoming(A); outside source POLICY_WITHHELD; existing inspect/filter reference comparison:\n%s", scenario, wantEdges, out)
		assertPartialInspectionReleasedPrivate(t, f)
		before, prepBefore := f.Traffic()
		if _, _, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req); err == nil {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ONE_SHOT")
		}
		after, prepAfter := f.Traffic()
		if before != after || prepBefore != prepAfter {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ONE_SHOT_NO_TRAFFIC")
		}
	})
}

func assertPartialInspectionReleasedPrivate(t *testing.T, f *ScopedBatchFixturePrivate) {
	t.Helper()
	live, _, _, _, general, occupied := f.Manager.DirectionalAccountingPrivate()
	active, closed, started, released := f.Manager.DirectionalGenericTerminalPrivate()
	prepLive, _, _, _, prepOccupied := f.Manager.DirectionalPreparationAccountingPrivate()
	if live != 0 || general != 0 || occupied || active != 0 || prepLive != 0 || prepOccupied || !closed || !started || !released {
		t.Fatalf("ASSERT_PARTIAL_CONNECTED_GENUINE_RELEASE: live=%d general=%d active=%d closed=%t started=%t released=%t", live, general, active, closed, started, released)
	}
}

func TestScopedPartialInspectionFailureReturnsNoArtifactPrivate(t *testing.T) {
	for _, fault := range []string{"malformed", "poison", "message-budget"} {
		t.Run(fault, func(t *testing.T) {
			WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
				selection, authority, binding, req := closedInputsPrivate(t, ctx, f)
				if fault == "message-budget" {
					f.child.mu.Lock()
					f.child.notifications = 65 // one over the existing per-class ceiling
					f.child.mu.Unlock()
				} else {
					f.Fault(fault)
				}
				out, seeds, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req)
				if err == nil || len(out) != 0 || len(seeds) != 0 || !selection.state.spent {
					t.Fatalf("ASSERT_PARTIAL_FAILURE_NO_SUCCESS_ARTIFACT: fault=%s err=%v bytes=%d seeds=%d spent=%t", fault, err, len(out), len(seeds), selection.state.spent)
				}
				assertPartialInspectionReleasedPrivate(t, f)
				before, prepBefore := f.Traffic()
				if _, _, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req); err == nil {
					t.Fatal("ASSERT_PARTIAL_FAILURE_ONE_SHOT")
				}
				after, prepAfter := f.Traffic()
				if before != after || prepBefore != prepAfter {
					t.Fatal("ASSERT_PARTIAL_FAILURE_NO_RETRY_TRAFFIC")
				}
				t.Logf("ASSERT_PARTIAL_FAILURE_NO_SUCCESS_ARTIFACT: %s refused: %v; genuine owner released", fault, err)
			})
		})
	}
}
