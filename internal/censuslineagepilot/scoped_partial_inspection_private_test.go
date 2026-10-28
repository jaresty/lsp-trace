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
			testScopedPartialInspectionConnectedPrivate(t, scenario, false)
		})
	}
}

func TestScopedPartialInspectionExact25ConnectedPrivate(t *testing.T) {
	testScopedPartialInspectionConnectedPrivate(t, "inside-context", true)
}

func testScopedPartialInspectionConnectedPrivate(t *testing.T, scenario string, exact25 bool) {
	var setup []func(string)
	expectedSource := map[string][]byte{}
	if exact25 {
		setup = append(setup, func(root string) {
			// Finalize six ordinary mock files before freeze, supply, or guard binding.
			// These deterministic bytes are not historical source.
			for _, path := range fixtureScopePaths {
				content := []byte("package fixture\n" + strings.Repeat("// deterministic owned mock padding ................................\n", 301))
				expectedSource[path] = content
				if err := os.WriteFile(filepath.Join(root, path), content, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
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
				Item     lsp.CallHierarchyItem `json:"item"`
				Position lsp.Position          `json:"position"`
			}
			if err := json.Unmarshal(m.Params, &p); err != nil {
				t.Fatal(err)
			}
			calls = append(calls, m.Method+":"+p.TextDocument.URI+p.Item.URI)
			var v any = []any{}
			switch m.Method {
			case "textDocument/prepareCallHierarchy":
				item := scopeAdapterItem(p.TextDocument.URI)
				if exact25 {
					matched := false
					for _, row := range partialInspectionExact25 {
						if p.TextDocument.URI == (&url.URL{Scheme: "file", Path: filepath.Join(f.Workspace, row.path)}).String() && p.Position.Line == row.line && p.Position.Character == row.character {
							item.Name = row.id
							item.Range = lsp.Range{Start: lsp.Position{Line: row.line}, End: lsp.Position{Line: row.line, Character: 64}}
							item.SelectionRange = lsp.Range{Start: p.Position, End: lsp.Position{Line: row.line, Character: row.character + 1}}
							matched = true
						}
					}
					if !matched {
						t.Errorf("ASSERT_EXACT25_PREPARE_POSITION: %s %+v", p.TextDocument.URI, p.Position)
					}
				}
				v = []lsp.CallHierarchyItem{item}
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
		if exact25 {
			targets, seeds = nil, nil
			for _, row := range partialInspectionExact25 {
				line, character := row.line, row.character
				uri := (&url.URL{Scheme: "file", Path: filepath.Join(f.Workspace, row.path)}).String()
				targets = append(targets, acquisitionmanifest.Target{ID: row.id, Locator: acquisition.Locator{URI: uri, Line: &line, Character: &character, LanguageID: "go"}, DownDepth: &one, UpDepth: &one})
				seeds = append(seeds, seedformat.Seed{Type: seedformat.PositionType, Position: &seedformat.Position{Label: row.id, Path: row.path, Line: uint64(row.line) + 1, Column: uint64(row.character) + 1}})
				lines := bytes.Split(expectedSource[row.path], []byte("\n"))
				if int(row.line) >= len(lines) || int(row.character)+1 > len(lines[row.line]) {
					t.Fatalf("ASSERT_EXACT25_MOCK_POSITION_AVAILABLE: %+v", row)
				}
			}
		}
		manifest := acquisitionmanifest.Manifest{SchemaVersion: acquisitionmanifest.ManifestVersion, CoordinateConvention: "zero-based-session", Root: targets[0], RequiredTargets: targets[1:]}
		if exact25 {
			// Test-only bounded headroom for race instrumentation across 75 requests.
			// Target coordinates, depths, expansion and canonical seed bytes stay exact.
			timeout, requestTimeout := 25000, 5000
			manifest.Limits.TimeoutMS, manifest.Limits.RequestTimeoutMS = &timeout, &requestTimeout
		}
		seedBytes, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &one, UpDepth: &one}, Seeds: seeds}, f.Workspace)
		if err != nil {
			t.Fatal(err)
		}
		if exact25 && fmt.Sprintf("%x", sha256.Sum256(seedBytes)) != partialInspectionExact25SeedSHA {
			t.Fatalf("ASSERT_EXACT25_CANONICAL_SEED_HASH: %x", sha256.Sum256(seedBytes))
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
		assertPartialInspectionReleasedPrivate(t, f) // Returned carrier is detached after genuine release.
		if !bytes.Equal(retainedSeeds, seedBytes) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_CANONICAL_SEEDS")
		}
		envelope, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil {
			t.Fatal(err)
		}
		r := envelope.Evidence.Acquisition
		for i, target := range r.Targets {
			if target.Requested.ID != targets[i].ID || target.Requested.Locator.URI != targets[i].Locator.URI || target.Requested.Locator.URI == f.ContextURI || target.Resolution.Status != acquisition.Resolved {
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
		if !envelope.FrozenQueryScope || !envelope.Partial || envelope.Authority != 0 || envelope.Accepted || envelope.ReplayAdmitted || envelope.SourceCompleteness != "UNKNOWN" || r.Graph.Summary.Complete || len(r.Graph.Edges) != wantEdges || len(r.Graph.Edges[0].CallSites) != 1 || len(r.Targets) != len(targets) {
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
					if exact25 {
						expected = expectedSource[fixtureScopePaths[5]]
					}
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
		comparison, err := filter.ProjectPairwise(projectionBytes, "root", targets[1].ID)
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
		if comparison.Accounting.RequestedSeedCount != len(targets) || comparison.Accounting.SuccessfulSeedCount != len(targets) || comparison.Accounting.FailedSeedCount != 0 || len(comparison.Partitions.CallRelations.LeftOnly) != wantEdges || len(comparison.Partitions.CallRelations.Shared) != 0 || len(comparison.Partitions.CallRelations.RightOnly) != 0 {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_REFERENCE_CLASSIFICATION: %s", out)
		}
		if exact25 {
			if len(r.Targets) != 25 {
				t.Fatalf("ASSERT_EXACT25_TARGET_COUNT: got=%d want=25", len(r.Targets))
			}
			identities := map[string]bool{}
			for i, target := range r.Targets {
				want, _ := json.Marshal(targets[i])
				got, _ := json.Marshal(target.Requested)
				if !bytes.Equal(got, want) || identities[target.Resolution.Identity.ID] {
					t.Fatalf("ASSERT_EXACT25_ORDER_COORDINATES_DISTINCT: index=%d got=%s want=%s", i, got, want)
				}
				identities[target.Resolution.Identity.ID] = true
			}
			traffic := map[string]int{}
			outgoingByLabel, incomingByLabel := map[string]int{}, map[string]int{}
			for _, msg := range f.RequestsPrivate() {
				traffic[msg.Method]++
				if strings.HasPrefix(msg.Method, "callHierarchy/") {
					var params struct {
						Item lsp.CallHierarchyItem `json:"item"`
					}
					if err := json.Unmarshal(msg.Params, &params); err != nil {
						t.Fatal(err)
					}
					if msg.Method == "callHierarchy/outgoingCalls" {
						outgoingByLabel[params.Item.Name]++
					} else {
						incomingByLabel[params.Item.Name]++
					}
				}
			}
			for i, row := range partialInspectionExact25 {
				wantIncoming := 1
				if i == 0 {
					wantIncoming = 0
				}
				if outgoingByLabel[row.id] != 1 || incomingByLabel[row.id] != wantIncoming {
					t.Fatalf("ASSERT_EXACT25_METHOD_START_IDENTITIES: %s outgoing=%d incoming=%d", row.id, outgoingByLabel[row.id], incomingByLabel[row.id])
				}
			}
			if len(outgoingByLabel) != 25 || len(incomingByLabel) != 25 || incomingByLabel["Query"] != 1 {
				t.Fatalf("ASSERT_EXACT25_CONTEXT_ONLY_EXTRA_START: outgoing=%v incoming=%v", outgoingByLabel, incomingByLabel)
			}
			opens := map[string]int{}
			for _, msg := range f.child.snapshot() {
				if msg.Method != "textDocument/didOpen" {
					continue
				}
				var params struct {
					TextDocument struct{ URI, Text string } `json:"textDocument"`
				}
				if err := json.Unmarshal(msg.Params, &params); err != nil {
					t.Fatal(err)
				}
				u, err := url.Parse(params.TextDocument.URI)
				if err != nil {
					t.Fatal(err)
				}
				rel, err := filepath.Rel(f.Workspace, u.Path)
				if err != nil || expectedSource[rel] == nil || !bytes.Equal([]byte(params.TextDocument.Text), expectedSource[rel]) {
					t.Fatalf("ASSERT_EXACT25_SIX_SUPPLIES_FROZEN_BYTES: %s", params.TextDocument.URI)
				}
				opens[rel]++
			}
			if len(opens) != 6 {
				t.Fatalf("ASSERT_EXACT25_SIX_SUPPLIES: %v", opens)
			}
			for path, count := range opens {
				if count != 1 {
					t.Fatalf("ASSERT_EXACT25_SUPPLY_ONCE: %s %d", path, count)
				}
			}
			if traffic["textDocument/prepareCallHierarchy"] != 25 || traffic["callHierarchy/outgoingCalls"] != 25 || traffic["callHierarchy/incomingCalls"] != 25 || traffic["textDocument/documentSymbol"] != 0 || len(calls) != 75 {
				t.Fatalf("ASSERT_EXACT25_FINITE_TRAFFIC: %v calls=%d", traffic, len(calls))
			}
			for _, target := range r.Targets[1:] {
				if len(target.Incoming.StartIDs) != 1 || target.Incoming.StartIDs[0] != target.Resolution.Identity.ID {
					t.Fatalf("ASSERT_EXACT25_EMPTY_OUTGOING_STARTS_SELF: %+v", target.Incoming)
				}
			}
			readable := 0
			for _, capture := range envelope.Evidence.Captures {
				if capture.URI == outside {
					continue
				}
				u, err := url.Parse(capture.URI)
				if err != nil {
					t.Fatal(err)
				}
				rel, err := filepath.Rel(f.Workspace, u.Path)
				if err != nil || expectedSource[rel] == nil || capture.Status != "READABLE" || !bytes.Equal(capture.Content, expectedSource[rel]) {
					t.Fatalf("ASSERT_EXACT25_FRESH_SOURCE_CAPTURE: %s status=%s", capture.URI, capture.Status)
				}
				readable++
			}
			if readable != 5 || len(envelope.Evidence.Captures) != 6 {
				t.Fatalf("ASSERT_EXACT25_SOURCE_COUNTS: readable=%d total=%d", readable, len(envelope.Evidence.Captures))
			}
			t.Logf("EXACT25_CHECKPOINT: targets=25 distinct=%d seed_sha256=%x prepare=25 outgoing=25 incoming=25 documentSymbol=0; readable=5 withheld=1; CALLS=3 frontier=2 root-only=3 shared=0 right-only=0", len(identities), sha256.Sum256(retainedSeeds))
		}
		t.Logf("PRODUCT_CHECKPOINT: deterministic nominated targets; scenario=%s retained CALLS=%d (root-only references), frontier=2; incoming(B) policy-stopped, never incoming(A); outside source POLICY_WITHHELD; existing inspect/filter reference comparison:\n%s", scenario, wantEdges, out)
		assertPartialInspectionReleasedPrivate(t, f)
		before, prepBefore := f.Traffic()
		if _, _, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req); err == nil {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ONE_SHOT")
		}
		after, prepAfter := f.Traffic()
		if before != after || prepBefore != prepAfter {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ONE_SHOT_NO_TRAFFIC")
		}
	}, setup...)
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
