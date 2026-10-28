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
					row := lsp.CallHierarchyOutgoingCall{To: scopeAdapterItem(outside), FromRanges: []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}}
					v = []lsp.CallHierarchyOutgoingCall{row, row} // preserve actual response duplicates
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
		if len(r.EdgeObservations) != 1 {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_REQUEST_RELATION_OBSERVATION")
		}
		duplicateRows := false
		for _, record := range r.Requests {
			if record.Method == "callHierarchy/outgoingCalls" && record.NodeID == r.Targets[0].Resolution.Identity.ID {
				var rows []lsp.CallHierarchyOutgoingCall
				if json.Unmarshal(record.Response, &rows) != nil || len(rows) != 2 || rows[0].To.URI != outside || rows[1].To.URI != outside || !bytes.Equal(rows[0].To.Data, rows[1].To.Data) {
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
		if !envelope.FrozenQueryScope || !envelope.Partial || envelope.Authority != 0 || envelope.Accepted || envelope.ReplayAdmitted || envelope.SourceCompleteness != "UNKNOWN" || r.Graph.Summary.Complete || len(r.Graph.Edges) != 1 || len(r.Graph.Edges[0].CallSites) != 1 || len(r.Targets) != 4 {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_RETAINED_CEILING")
		}
		if len(r.Targets[0].Incoming.StartIDs) != 1 || r.Targets[0].Incoming.StartIDs[0] != r.Graph.Edges[0].CalleeNodeID || len(r.Graph.Frontier) != 2 {
			t.Fatalf("ASSERT_PARTIAL_CONNECTED_INCOMING_START: %+v", r.Targets[0].Incoming)
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
		out, _ := json.MarshalIndent(comparison, "", "  ")
		if len(projection.Records.CallRelations) != 1 || len(projection.Records.Frontier) != 2 || !strings.Contains(string(out), r.Graph.Edges[0].RelationID) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_EXISTING_ENGINES")
		}
		// Neither projection engine acquires source. The original private source
		// carrier remains intact alongside its read-only graph-derived views.
		afterViews, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil || afterViews.WithheldCount != 1 || !bytes.Equal(afterViews.Evidence.GraphBytes, envelope.Evidence.GraphBytes) {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_CARRIER_SURVIVES_VIEWS")
		}
		t.Logf("PRODUCT_CHECKPOINT: four deterministic nominated targets; retained A->outsideB; incoming(B) policy-stopped, never incoming(A); outside source POLICY_WITHHELD; existing inspect/filter reference comparison:\n%s", out)
		if _, _, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req); err == nil {
			t.Fatal("ASSERT_PARTIAL_CONNECTED_ONE_SHOT")
		}
	})
}
