//go:build darwin || linux

package inventorysuccessor_test

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
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/referencegrouping"
)

// Expected success, OFFLINE CODEC CONSISTENCY ONLY. Synthetic RPC frames are
// not producer originals, native execution, funding, or source custody. Only
// one new tiny temp file provides ordinary source-capture receipts.
// Adapted narrowly from referencegrouping/nativeCodecFixture. Duplicate raw
// ranges are intentionally retained; no deduplication or new admission API.
func TestConfigurationV3DuplicateLiteralPure(t *testing.T) {
	// Source-only drift pin, not native behavioral evidence. The behavioral
	// qualification below independently encodes/replays these literal shapes.
	responder, err := os.ReadFile("application_inventory_v3_configuration_responder_external_test.go")
	if err != nil {
		t.Fatal("SETUP_RESPONDER_SHAPE_PIN", err)
	}
	for _, literal := range []string{
		`filepath.Join(root, "root.go")`,
		`Range: pos(line, 0, 22), SelectionRange: pos(line, 5, end)`,
		`query, other, third := item("Root", 1, 9), item("Other", 2, 10), item("Third", 3, 10)`,
		`result = []lsp.CallHierarchyItem{query}`,
		`sites := []lsp.Range{pos(2, 14, 18)}`,
		`lsp.CallHierarchyIncomingCall{From: other, FromRanges: sites}`,
		`sites := []lsp.Range{pos(1, 13, 18)}`,
		`lsp.CallHierarchyOutgoingCall{To: other, FromRanges: sites}`,
	} {
		if !strings.Contains(string(responder), literal) {
			t.Fatal("SETUP_RESPONDER_SHAPE_PIN", literal)
		}
	}
	if strings.Count(string(responder), "sites = append(sites, sites[0])") != 2 {
		t.Fatal("SETUP_RESPONDER_DUPLICATE_SHAPE_PIN")
	}
	root := t.TempDir()
	contents := []string{"package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\nfunc Third(){ Root() }\n"}
	names := []string{"root.go"}
	total := len(contents[0])
	if total > 8192 {
		t.Fatal("SETUP_TEMP_BYTE_BOUND")
	}
	for i, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents[i]), 0600); err != nil {
			t.Fatal("SETUP_TEMP", err)
		}
	}
	marshal := func(v any) []byte {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal("SETUP_JSON", err)
		}
		return b
	}
	uri := func(name string) string { return (&url.URL{Scheme: "file", Path: filepath.Join(root, name)}).String() }
	rangeAt := func(start, end uint32) lsp.Range {
		return lsp.Range{Start: lsp.Position{Line: 1, Character: start}, End: lsp.Position{Line: 1, Character: end}}
	}
	graphRange := func(r lsp.Range) graph.Range {
		return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
	}
	item := lsp.CallHierarchyItem{Name: "Root", Kind: 12, URI: uri("root.go"), Range: rangeAt(0, 22), SelectionRange: rangeAt(5, 9)}
	otherRange := func(start, end uint32) lsp.Range { r := rangeAt(start, end); r.Start.Line, r.End.Line = 2, 2; return r }
	other := lsp.CallHierarchyItem{Name: "Other", Kind: 12, URI: uri("root.go"), Range: otherRange(0, 22), SelectionRange: otherRange(5, 10)}
	line, character := uint32(1), uint32(5)
	parent := acquisition.PrivateDirectionalParentV2{OperationID: "offline-duplicate-no-custody", SessionID: "session", Generation: 1, URI: item.URI, LanguageID: "go", Line: &line, Character: &character, MaxMessages: 1, PositionEncoding: "utf-16", SupplyURI: item.URI, SupplyObservation: json.RawMessage("null")}
	params := [][]byte{marshal(map[string]any{"textDocument": map[string]string{"uri": item.URI}, "position": lsp.Position{Line: line, Character: character}}), marshal(map[string]any{"item": item}), marshal(map[string]any{"item": item})}
	results := [][]byte{marshal([]lsp.CallHierarchyItem{item}), marshal([]lsp.CallHierarchyIncomingCall{{From: other, FromRanges: []lsp.Range{otherRange(14, 18), otherRange(14, 18)}}}), marshal([]lsp.CallHierarchyOutgoingCall{{To: other, FromRanges: []lsp.Range{rangeAt(13, 18), rangeAt(13, 18)}}})}
	methods := []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"}
	proof := referencegrouping.PrivateNativeEvidenceV2{Schema: referencegrouping.PrivateNativeEvidenceSchemaV2, Parent: parent}
	frame := func(body []byte) []byte {
		return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	}
	for i, method := range methods {
		id := uint64(i + 1)
		request := frame([]byte(fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"method\":%q,\"params\":%s}", id, method, params[i])))
		response := frame([]byte(fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}", id, results[i])))
		r := referencegrouping.PrivateNativeRequestV2{Ordinal: i, Method: method, RequestID: id, Generation: 1, Params: params[i], Result: results[i], RequestFrame: request, ResponseFrames: [][]byte{response}, Status: "OBSERVED_NONEMPTY"}
		r.Wire = directionalraw.CompletionMetadataPrivate{RequestIDPrivate: id, GenerationPrivate: 1, RequestLengthPrivate: uint64(len(request)), RequestSHA256Private: sha256.Sum256(request), ResponseLengthPrivate: uint64(len(response)), ResponseSHA256Private: sha256.Sum256(response), ResultLengthPrivate: uint64(len(results[i])), ResultSHA256Private: sha256.Sum256(results[i])}
		observe := func(component string, raw []byte) directionalraw.Observation {
			return directionalraw.Observation{Method: method, RequestID: id, Generation: 1, Component: component, Length: uint64(len(raw)), SHA256: sha256.Sum256(raw)}
		}
		r.Observations = []directionalraw.Observation{observe("request-frame", request), observe("response-frame", response)}
		if i == 0 {
			o := observe("element", marshal(item))
			o.Selected = true
			r.Observations = append(r.Observations, o)
		}
		proof.Requests = append(proof.Requests, r)
		parent.Requests = append(parent.Requests, acquisition.PrivateDirectionalObservationV2{Ordinal: i, Method: method, RequestID: id, Generation: 1, Params: params[i], Result: results[i]})
	}
	views, err := acquisition.BuildDirectionalViewsPrivateV2(parent)
	if err != nil {
		t.Fatal("SETUP_DIRECTIONAL_VIEWS", err)
	}
	workspaceURI := (&url.URL{Scheme: "file", Path: root}).String()
	seed := graph.InvocationSeed{Label: "root", At: "root.go:2:6", ResolvedURI: item.URI, ContentSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(contents[0]))), LanguageID: "go"}
	proof.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "offline-duplicate-no-custody", SourceRevision: "commit", ServerVersion: "gopls@1"}}
	for i, view := range views {
		proof.OriginalOrdinals[i] = view.OriginalOrdinals
		proof.Sources[i], err = graphprovenance.CaptureNativeConstructorSourcePrivateV2(context.Background(), view.Result, root)
		if err != nil {
			t.Fatal("SETUP_ORDINARY_SOURCE_CAPTURE", err)
		}
		if len(proof.Sources[i].Captures) == 0 {
			t.Fatal("SETUP_SOURCE_RECEIPTS_MISSING")
		}
		for _, receipt := range proof.Sources[i].Captures {
			if receipt.Status != "READABLE" || receipt.URI != uri("root.go") {
				t.Fatal("SETUP_SOURCE_SCOPE")
			}
		}
		if !bytes.Equal(marshal(view.Result), marshal(proof.Sources[i].Acquisition)) {
			t.Fatal("SETUP_SOURCE_ROUNDTRIP")
		}
		native := view.Result.Graph
		native.SchemaVersion, native.Invocation = graph.SchemaVersionV5, proof.Invocation
		native.Capabilities.CallHierarchyProvider = true
		native.Canonicalize()
		envelope, e := graphprovenance.CaptureV5(marshal(native), parent.SessionID, parent.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &proof.Sources[i])
		if e != nil {
			t.Fatal("SETUP_CAPTURE_V5", e)
		}
		proof.Inputs = append(proof.Inputs, programccompose.Input{Bytes: envelope, Identity: fmt.Sprintf("%s:view-%d", proof.Invocation.Provenance.InvocationID, i), ByteLength: len(envelope), SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(envelope)), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: workspaceURI, RevisionCustody: "CALLER_ASSERTED", PositionEncoding: parent.PositionEncoding, AcquisitionSemantics: "ONE_ACTUAL_DIRECTIONAL_INVOCATION_TWO_VALIDATED_VIEWS", PrivacyPolicy: "private-test-v1"}})
		joins, e := referencegrouping.PrivateNativeRangeJoinV2(parent, i, native)
		if e != nil {
			t.Fatal("SETUP_RAW_RANGE_JOIN", e)
		}
		proof.RawJoins = append(proof.RawJoins, joins...)
	}
	if !bytes.Equal(marshal(proof.OriginalOrdinals), []byte("[[-1,0,1],[-1,0,2]]")) || len(proof.RawJoins) != 4 {
		t.Fatal("SETUP_DUPLICATE_ORDINALS_JOINS", proof.OriginalOrdinals, len(proof.RawJoins))
	}
	// MergeEdge/mergeRanges retains one canonical site per identical range;
	// PrivateNativeRangeJoinV2 separately retains both raw range ordinals.
	for view := 0; view < 2; view++ {
		edges := views[view].Result.Graph.Edges
		expectedSite := otherRange(14, 18)
		if view == 1 {
			expectedSite = rangeAt(13, 18)
		}
		nodeIDs := map[string]string{}
		for _, node := range views[view].Result.Graph.Nodes {
			if node.URI != uri("root.go") {
				t.Fatal("SETUP_LITERAL_NODE_URI")
			}
			nodeIDs[node.Name] = node.ID
		}
		caller, callee := nodeIDs["Other"], nodeIDs["Root"]
		if view == 1 {
			caller, callee = nodeIDs["Root"], nodeIDs["Other"]
		}
		if caller == "" || callee == "" || len(edges) != 1 || edges[0].CallerNodeID != caller || edges[0].CalleeNodeID != callee {
			t.Fatal("SETUP_LITERAL_ENDPOINT_DIRECTION", view)
		}
		if len(edges) != 1 || len(edges[0].CallSites) != 1 || edges[0].CallSites[0] != graphRange(expectedSite) {
			t.Fatal("SETUP_ONE_LITERAL_CANONICAL_EDGE_SITE_PER_VIEW", view)
		}
		direction := "INCOMING"
		if view == 1 {
			direction = "OUTGOING"
		}
		for ordinal := 0; ordinal < 2; ordinal++ {
			matches := 0
			for _, join := range proof.RawJoins {
				if join.ViewIndex == view && join.RangeOrdinal == ordinal {
					if join.OriginalRequestOrdinal != view+1 || join.ElementOrdinal != 0 || join.EdgeLocalOrdinal != 0 || join.Direction != direction || join.RelationID != edges[0].RelationID || join.Range != edges[0].CallSites[0] {
						t.Fatal("SETUP_EXACT_RAW_CANONICAL_MAPPING", view, ordinal)
					}
					matches++
				}
			}
			if matches != 1 {
				t.Fatal("SETUP_EXACT_RANGE_MULTIPLICITY", view, ordinal, matches)
			}
		}
	}
	composite, err := programccompose.Compose(proof.Inputs)
	if err != nil {
		t.Fatal("SETUP_COMPOSE", err)
	}
	encoded, err := referencegrouping.EncodePrivateNativeEvidenceV2(proof, composite.Bytes)
	if err != nil {
		t.Fatal("SETUP_ENCODE_REPLAY", err)
	}
	if err = referencegrouping.ReplayPrivateNativeEvidenceV2(encoded, composite.Bytes); err != nil {
		t.Fatal("SETUP_EXPLICIT_REPLAY", err)
	}
	var decoded referencegrouping.PrivateNativeEvidenceV2
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal("SETUP_SERIALIZED_PROOF_DECODE", err)
	}
	if len(decoded.RawJoins) != 4 || !bytes.Equal(marshal(decoded.RawJoins), marshal(proof.RawJoins)) || !bytes.Equal(marshal(decoded.OriginalOrdinals), marshal(proof.OriginalOrdinals)) {
		t.Fatal("SETUP_SERIALIZED_RAW_JOINS_AND_ORDINALS")
	}
	admitted, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal("SETUP_OPAQUE_COMPOSITE_ADMISSION", err)
	}
	typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil {
		t.Fatal("SETUP_TYPED_SEAL", err)
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(typed)
	if err != nil {
		t.Fatal("SETUP_PROJECTION_SEAL", err)
	}
	if len(typed.Occurrences) != 2 || !bytes.Equal(projection.RetainedCallsCanonicalBytes(), composite.Bytes) {
		t.Fatalf("SETUP_MULTIPLICITY_CANONICAL occurrences=%d want=2 canonical_equal=%t retained_bytes=%d composite_bytes=%d raw_joins=%d", len(typed.Occurrences), bytes.Equal(projection.RetainedCallsCanonicalBytes(), composite.Bytes), len(projection.RetainedCallsCanonicalBytes()), len(composite.Bytes), len(proof.RawJoins))
	}
	for view := 0; view < 2; view++ {
		edge := views[view].Result.Graph.Edges[0]
		matches := 0
		for _, occurrence := range typed.Occurrences {
			if occurrence.RelationID == edge.RelationID && occurrence.Ordinal == 0 && occurrence.From == edge.CallerNodeID && occurrence.To == edge.CalleeNodeID {
				matches++
			}
		}
		if matches != 1 {
			t.Fatal("SETUP_CANONICAL_OCCURRENCE_MAPPING", view, matches)
		}
	}
	seen := map[string]bool{}
	for _, o := range typed.Occurrences {
		if o.Kind != programcadmission.TypedCalls || seen[o.Identity] {
			t.Fatal("SETUP_OCCURRENCE_IDENTITY")
		}
		seen[o.Identity] = true
	}
	// The checker is also exercised on single-field perturbations. Its expected
	// sites are literal responder ranges, not values obtained from the raw joins.
	checkMapping := func(p referencegrouping.PrivateNativeEvidenceV2) error {
		if len(p.RawJoins) != 4 {
			return fmt.Errorf("RAW_MAPPING: count")
		}
		seen := map[[2]int]bool{}
		for _, j := range p.RawJoins {
			if j.ViewIndex < 0 || j.ViewIndex > 1 || j.RangeOrdinal < 0 || j.RangeOrdinal > 1 {
				return fmt.Errorf("RAW_MAPPING: ordinal")
			}
			key := [2]int{j.ViewIndex, j.RangeOrdinal}
			if seen[key] {
				return fmt.Errorf("RAW_MAPPING: repeated ordinal")
			}
			seen[key] = true
			direction, site := "INCOMING", otherRange(14, 18)
			if j.ViewIndex == 1 {
				direction, site = "OUTGOING", rangeAt(13, 18)
			}
			edge := views[j.ViewIndex].Result.Graph.Edges[0]
			if j.OriginalRequestOrdinal != j.ViewIndex+1 || j.ElementOrdinal != 0 || j.EdgeLocalOrdinal != 0 || j.Direction != direction || j.RelationID != edge.RelationID || j.Range != graphRange(site) {
				return fmt.Errorf("RAW_MAPPING: exact fields")
			}
			matches := 0
			for _, o := range typed.Occurrences {
				if o.RelationID == j.RelationID && o.Ordinal == j.EdgeLocalOrdinal && o.Ordinal >= 0 && o.Ordinal < len(edge.CallSites) && edge.CallSites[o.Ordinal] == j.Range && o.From == edge.CallerNodeID && o.To == edge.CalleeNodeID {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("RAW_MAPPING: canonical occurrence")
			}
		}
		return nil
	}
	if err := checkMapping(decoded); err != nil {
		t.Fatal(err)
	}
	t.Log("RAW_MAPPING: PASS exact four original joins to two canonical occurrences")
	for _, join := range decoded.RawJoins {
		for _, occurrence := range typed.Occurrences {
			if occurrence.RelationID == join.RelationID && occurrence.Ordinal == join.EdgeLocalOrdinal {
				t.Logf("OFFLINE_EXACT_MAPPING view=%d request=%d element=%d range_ordinal=%d direction=%s range=%+v relation=%s canonical_ordinal=%d identity=%s from=%s to=%s", join.ViewIndex, join.OriginalRequestOrdinal, join.ElementOrdinal, join.RangeOrdinal, join.Direction, join.Range, join.RelationID, occurrence.Ordinal, occurrence.Identity, occurrence.From, occurrence.To)
			}
		}
	}
	mutations := []struct {
		name   string
		mutate func(*referencegrouping.PrivateNativeEvidenceV2)
	}{
		{"request", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].OriginalRequestOrdinal++ }},
		{"element", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].ElementOrdinal++ }},
		{"range-ordinal", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].RangeOrdinal = 1 }},
		{"view", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].ViewIndex = 1 }},
		{"edge-local", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].EdgeLocalOrdinal++ }},
		{"direction", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].Direction = "OUTGOING" }},
		{"relation", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].RelationID += "-changed" }},
		{"range", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins[0].Range.Start.Character++ }},
		{"lost-duplicate", func(p *referencegrouping.PrivateNativeEvidenceV2) { p.RawJoins = p.RawJoins[:3] }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			var perturbed referencegrouping.PrivateNativeEvidenceV2
			if err := json.Unmarshal(encoded, &perturbed); err != nil {
				t.Fatal(err)
			}
			mutation.mutate(&perturbed)
			if err := checkMapping(perturbed); err == nil {
				t.Fatal("RAW_MAPPING: perturbation accepted")
			} else {
				t.Log("RAW_MAPPING: FAIL (expected)", err)
			}
		})
	}
	if !bytes.Equal(marshal(decoded.RawJoins), marshal(proof.RawJoins)) {
		t.Fatal("RAW_MAPPING: originals changed")
	}
	t.Logf("SETUP_VALID; OFFLINE_SYNTHETIC_NO_PRODUCER_CUSTODY; REQUESTS=3; VIEWS=2; SOURCE_CONSTRUCTOR_CALLS=2; TEMP_FILES=1; TEMP_BYTES=%d; RAW_JOINS=4; CANONICAL_OCCURRENCES=2; TWO_RAW_RANGE_ORDINALS_PER_VIEW_MAP_TO_ONE_CANONICAL_SITE_LOCAL0; ORIGINAL_ORDINALS=[[-1,0,1],[-1,0,2]]; REPLAY_ADMISSION_TYPED_PROJECTION_PASSED", total)
	// The external campaign checks canonical identities only; this independent
	// offline fixture supplies the separate exact raw-join qualification.
	if err := configurationDuplicateCanonicalV3(typed); err != nil {
		t.Fatal("ASSERT_CONFIGURATION_V3_DUPLICATE_CANONICAL", err)
	}
	t.Log("ASSERT_CONFIGURATION_V3_DUPLICATE_CANONICAL: PASS")
	for _, name := range []string{"four-unique", "duplicate-identity", "empty-identity"} {
		t.Run(name, func(t *testing.T) {
			perturbed := typed
			perturbed.Occurrences = append([]programcadmission.TypedOccurrence(nil), typed.Occurrences...)
			switch name {
			case "four-unique":
				for i := 0; i < 2; i++ {
					extra := typed.Occurrences[i]
					extra.Identity += "-extra"
					perturbed.Occurrences = append(perturbed.Occurrences, extra)
				}
			case "duplicate-identity":
				perturbed.Occurrences[1].Identity = perturbed.Occurrences[0].Identity
			case "empty-identity":
				perturbed.Occurrences[0].Identity = ""
			}
			if err := configurationDuplicateCanonicalV3(perturbed); err == nil {
				t.Fatal("ASSERT_CONFIGURATION_V3_DUPLICATE_CANONICAL: perturbation accepted")
			} else {
				t.Log("ASSERT_CONFIGURATION_V3_DUPLICATE_CANONICAL: FAIL (expected)", err)
			}
		})
	}
}

// Canonical-only campaign guard. Raw multiplicity is qualified separately by
// TestConfigurationV3DuplicateLiteralPure, never inferred from native DTO counts.
func configurationDuplicateCanonicalV3(typed programcadmission.TypedInput) error {
	count := 0
	seen := map[string]bool{}
	for _, o := range typed.Occurrences {
		if o.Kind != programcadmission.TypedCalls {
			continue
		}
		if o.Identity == "" || seen[o.Identity] {
			return fmt.Errorf("duplicate or empty CALLS identity")
		}
		seen[o.Identity] = true
		count++
	}
	if count != 2 {
		return fmt.Errorf("canonical CALLS=%d want=2", count)
	}
	return nil
}
