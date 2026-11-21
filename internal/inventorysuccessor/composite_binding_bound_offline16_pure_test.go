package inventorysuccessor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"lsp-trace/sessionruntime"
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
// two new tiny temp files provide actual ordinary source-capture receipts.
// Adapted narrowly from referencegrouping/nativeCodecFixture. Duplicate raw
// ranges are intentionally retained; no deduplication or new admission API.
// Characterization only: synthetic controlled metadata, no runtime qualification.
func TestCompositeBindingBoundOffline16Pure(t *testing.T) {
	for _, metadataBytes := range []int{0, 8192, 32768, 65536} {
		t.Run(fmt.Sprintf("metadata_%d", metadataBytes), func(t *testing.T) {
			root := t.TempDir()
			contents := []string{"package fixture\nfunc A() { B() }\n", "package fixture\nfunc B() {}\n"}
			names := []string{"a.go", "b.go"}
			total := len(contents[0]) + len(contents[1])
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
			item := lsp.CallHierarchyItem{Name: "A", Kind: 12, URI: uri("a.go"), Range: rangeAt(0, 16), SelectionRange: rangeAt(5, 6)}
			other := lsp.CallHierarchyItem{Name: "B", Kind: 12, URI: uri("b.go"), Range: rangeAt(0, 11), SelectionRange: rangeAt(5, 6)}
			line, character := uint32(1), uint32(5)
			parent := acquisition.PrivateDirectionalParentV2{OperationID: "offline-duplicate-no-custody", SessionID: "session", Generation: 1, URI: item.URI, LanguageID: "go", Line: &line, Character: &character, MaxMessages: 1, PositionEncoding: "utf-16", SupplyURI: item.URI, SupplyObservation: json.RawMessage("null")}
			params := [][]byte{marshal(map[string]any{"textDocument": map[string]string{"uri": item.URI}, "position": lsp.Position{Line: line, Character: character}}), marshal(map[string]any{"item": item}), marshal(map[string]any{"item": item})}
			results := [][]byte{marshal([]lsp.CallHierarchyItem{item}), marshal([]lsp.CallHierarchyIncomingCall{{From: other, FromRanges: []lsp.Range{rangeAt(5, 6), rangeAt(5, 6)}}}), marshal([]lsp.CallHierarchyOutgoingCall{{To: other, FromRanges: []lsp.Range{rangeAt(11, 12), rangeAt(11, 12)}}})}
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
			seed := graph.InvocationSeed{Label: "root", At: "a.go:2:6", ResolvedURI: item.URI, ContentSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(contents[0]))), LanguageID: "go"}
			proof.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "offline-duplicate-no-custody", SourceRevision: "commit", ServerVersion: "gopls@1"}}
			if metadataBytes > 0 {
				proof.Invocation.Server.Arguments = []string{"OFFLINE_SYNTHETIC=" + strings.Repeat("x", metadataBytes)}
			}
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
					if receipt.Status != "READABLE" || (receipt.URI != uri("a.go") && receipt.URI != uri("b.go")) {
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
				if len(edges) != 1 || len(edges[0].CallSites) != 1 {
					t.Fatal("SETUP_ONE_CANONICAL_EDGE_SITE_PER_VIEW", view)
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
			// Deliberately exclude the independently bounded proof codec. This
			// isolates valid Compose -> Admit -> typed projection -> binding.
			// Original TestDuplicateCallsAssemblyPure covers the proof-backed baseline.
			// No native proof is encoded or submitted.
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
			bindingRaw := marshal(projection.CompositeSourceBinding())
			bound, boundOK := sessionruntime.PrivateGroupingCheckedEncodedSize(16<<10, uint64(len(composite.Bytes)), 6, 0)
			t.Logf("MEASURE synthetic_metadata=%d retained=%d binding_json=%d binding_with_newline=%d conservative_unchecked=%d checked=%d bound_ok=%t", metadataBytes, len(composite.Bytes), len(bindingRaw), len(bindingRaw)+1, 16384+6*len(composite.Bytes), bound, boundOK)
			for i, c := range projection.CompositeSourceBinding().Constituents {
				t.Logf("CONSTITUENT index=%d envelope_bytes=%d base64_bytes=%d invocation_json=%d supplies=%d captures=%d bindings=%d", i, c.ByteLength, len(c.BytesBase64), len(c.Invocation), len(c.Supplies), len(c.Captures), len(c.Bindings))
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
			t.Logf("SETUP_VALID; OFFLINE_SYNTHETIC_NO_PRODUCER_CUSTODY; REQUESTS=3; VIEWS=2; SOURCE_CONSTRUCTOR_CALLS=2; TEMP_FILES=2; TEMP_BYTES=%d; RAW_JOINS=4; CANONICAL_OCCURRENCES=2; TWO_RAW_RANGE_ORDINALS_PER_VIEW_MAP_TO_ONE_CANONICAL_SITE_LOCAL0; ORIGINAL_ORDINALS=[[-1,0,1],[-1,0,2]]; COMPOSE_ADMISSION_TYPED_PROJECTION_PASSED_NO_NATIVE_PROOF", total)
			// Exact seam only; do not claim native proof or whole-assembly admission.
			if boundOK {
				got, err := privateGroupingJSON(projection.CompositeSourceBinding(), bound)
				if err != nil || !bytes.Equal(got, append(bindingRaw, '\n')) {
					t.Fatal("CHARACTERIZATION_SEAM_SERIALIZER", err)
				}
				t.Log("CHARACTERIZATION bound_admitted=true serializer_exact=true")
			} else {
				t.Logf("CHARACTERIZATION bound_admitted=false actual_binding_below_ceiling=%t NOT_EXPECTED_SUCCESS_RED_NOT_REPAIR", len(bindingRaw)+1 < 1500000)
			}
		})
	}
}
