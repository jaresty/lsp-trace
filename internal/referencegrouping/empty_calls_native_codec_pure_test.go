package referencegrouping

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
)

// EXPECTED-SUCCESS REGRESSION: valid empty C/CR must build and fully readmit.
// OFFLINE CODEC CONSISTENCY ONLY. Synthetic RPC frames are not manager-observed
// originals, producer custody, native execution, funding or qualified behavior.
// Only the two newly written tiny temp files supply actual filesystem receipts.
func TestEmptyCallsNativeCodecPure(t *testing.T) {
	root := t.TempDir()
	contents := map[string]string{"a.go": "package fixture\nfunc A() { B() }\n", "b.go": "package fixture\nfunc B() {}\n"}
	total := 0
	for _, name := range []string{"a.go", "b.go"} {
		total += len(contents[name])
		if total > 8192 {
			t.Fatal("ASSERT_CODEC_TEMP_BYTES")
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents[name]), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("OFFLINE_CODEC_ONLY; TEMP_SOURCE_FILES=2; TEMP_SOURCE_BYTES=%d; NO_PRODUCER_CUSTODY", total)
	empty := nativeCodecFixture(t, root, false)
	for _, mixed := range []bool{false, true} {
		name := "C-empty"
		if mixed {
			name = "CR-C-empty"
		}
		t.Run(name, func(t *testing.T) {
			req := nativeCodecInput(t, empty, mixed)
			projection, err := programcadmission.PreparePrivateLeidenProjection(req.TypedInput)
			if err != nil {
				t.Fatal("ASSERT_CODEC_TYPED_PROJECTION", err)
			}
			calls := 0
			selected := false
			for _, kind := range projection.SelectedKinds() {
				selected = selected || kind == programcadmission.TypedCalls
			}
			for _, occurrence := range projection.Occurrences() {
				if occurrence.Kind == programcadmission.TypedCalls {
					calls++
				}
			}
			if !selected || calls != 0 || len(projection.RetainedCallsCanonicalBytes()) == 0 || req.TypedInput.Authority != 0 || req.TypedInput.SourceGraphComplete != "UNKNOWN" {
				t.Fatal("ASSERT_CODEC_EMPTY_NOT_ABSENT")
			}
			ledger, err := canonicalOccurrenceLedger(projection.Occurrences())
			if err != nil || !bytes.Equal(ledger, req.OccurrenceLedgerBytes) {
				t.Fatal("ASSERT_CODEC_LEDGER", err)
			}
			if err := validateRequest(req, projection.Occurrences()); err != nil {
				t.Fatal("ASSERT_CODEC_REQUEST", err)
			}
			if err := validateBoundPredecessors(req.Predecessors, projection.MethodEvidence(), projection.MethodCanonicalBytes(), projection.CompositeSourceBinding(), projection.RetainedCallsCanonicalBytes()); err != nil {
				t.Fatal("ASSERT_CODEC_BOUND_PREDECESSORS", err)
			}
			if err := ReplayPrivateNativeEvidenceV2(empty.proof, projection.RetainedCallsCanonicalBytes()); err != nil {
				t.Fatal("ASSERT_CODEC_EMPTY_VALID_REPLAY", err)
			}
			artifact, err := BuildInput(req)
			if err != nil {
				t.Fatalf("ASSERT_CODEC_EMPTY_BUILD_EXPECTED_SUCCESS: %v", err)
			}
			admitted, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits())
			if err != nil {
				t.Fatalf("ASSERT_CODEC_EMPTY_READMISSION_EXPECTED_SUCCESS: %v", err)
			}
			if !admitted.MatchesTypedInput(req.TypedInput) || !bytes.Equal(admitted.CanonicalBytes(), artifact.CanonicalBytes()) || admitted.Artifact().Authority() != 0 || admitted.Artifact().Accepted() || admitted.Artifact().Completeness() != "UNKNOWN" {
				t.Fatal("ASSERT_CODEC_EMPTY_READMISSION_BINDING_AND_CEILING")
			}
			t.Logf("OFFLINE_EMPTY_CODEC_EXPECTED_SUCCESS; BUILD_PARSE_ADMIT_PASSED; METHOD_ROWS=%d; AUTHORITY0_ACCEPTED_FALSE_COMPLETENESS_UNKNOWN", len(projection.MethodEvidence()))
		})
	}

	t.Run("empty-proof-negatives", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*PrivateNativeEvidenceV2)
		}{
			{"missing-request", func(p *PrivateNativeEvidenceV2) { p.Requests = p.Requests[:2] }},
			{"tampered-wire-commitment", func(p *PrivateNativeEvidenceV2) { p.Requests[1].Wire.ResponseLengthPrivate++ }},
			{"tampered-ledger", func(p *PrivateNativeEvidenceV2) { p.Requests[1].Observations[0].Length++ }},
			{"wrong-empty-status", func(p *PrivateNativeEvidenceV2) { p.Requests[1].Status = "OBSERVED_NONEMPTY" }},
			{"unavailable-is-not-empty", func(p *PrivateNativeEvidenceV2) { p.Requests[1].Status = "UNAVAILABLE" }},
			{"missing-source", func(p *PrivateNativeEvidenceV2) { p.Sources[0] = graphprovenance.EvidenceV2{} }},
			{"tampered-view-ordinals", func(p *PrivateNativeEvidenceV2) { p.OriginalOrdinals[0][0] = 99 }},
			{"wrong-native-semantics", func(p *PrivateNativeEvidenceV2) { p.Inputs[0].ExactMetadata.AcquisitionSemantics = "managed-lsp-v1" }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				var p PrivateNativeEvidenceV2
				if err := json.Unmarshal(empty.proof, &p); err != nil {
					t.Fatal(err)
				}
				mutation.edit(&p)
				raw, marshalErr := json.Marshal(p)
				if marshalErr != nil {
					if mutation.name != "missing-source" || !strings.Contains(marshalErr.Error(), "V2 typed graph differs from captured bytes") {
						t.Fatalf("ASSERT_CODEC_UNEXPECTED_MARSHAL_ERROR mutation=%s: %v", mutation.name, marshalErr)
					}
					t.Logf("OFFLINE_NEGATIVE %s: expected encoding refusal: %v", mutation.name, marshalErr)
					return
				}
				raw = append(raw, '\n')
				if err := ReplayPrivateNativeEvidenceV2(raw, empty.composite); err == nil {
					t.Fatal("ASSERT_CODEC_NEGATIVE_REFUSED", mutation.name)
				} else {
					t.Logf("OFFLINE_NEGATIVE %s: %v", mutation.name, err)
				}
			})
		}
		if ReplayPrivateNativeEvidenceV2(nil, empty.composite) == nil || ReplayPrivateNativeEvidenceV2(empty.proof, append(append([]byte(nil), empty.composite...), ' ')) == nil {
			t.Fatal("ASSERT_CODEC_MISSING_OR_TAMPERED_COMPOSITE_REFUSED")
		}
	})

	t.Run("empty-carrier-admission-controls", func(t *testing.T) {
		withoutRole := func(req InputRequest, role string) InputRequest {
			var kept []PredecessorRecord
			for _, predecessor := range req.Predecessors {
				if predecessor.Role != role {
					kept = append(kept, predecessor)
				}
			}
			req.Predecessors = kept
			return req
		}
		for _, mixed := range []bool{false, true} {
			name := "C"
			if mixed {
				name = "CR"
			}
			t.Run(name+"-missing-native-proof", func(t *testing.T) {
				req := withoutRole(nativeCodecInput(t, empty, mixed), "NATIVE_EVIDENCE")
				artifact, err := BuildInput(req)
				if err != nil {
					t.Fatal("ASSERT_CODEC_EMPTY_MISSING_PROOF_BUILD", err)
				}
				if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || err.Error() != "native proof path mismatch" {
					t.Fatal("ASSERT_CODEC_EMPTY_MISSING_PROOF_NOT_ADMITTED", err)
				}
			})
			t.Run(name+"-ordinary-with-native-proof", func(t *testing.T) {
				req := emptyCallsPredecessorsFixture(t, mixed).request()
				req.Predecessors = append(req.Predecessors, PredecessorRecord{Role: "NATIVE_EVIDENCE", Selector: "calls/native-evidence.json", Schema: PrivateNativeEvidenceSchemaV2, Bytes: empty.proof})
				artifact, err := BuildInput(req)
				if err != nil {
					t.Fatal("ASSERT_CODEC_EMPTY_ORDINARY_WITH_PROOF_BUILD", err)
				}
				if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || err.Error() != "native proof path mismatch" {
					t.Fatal("ASSERT_CODEC_EMPTY_ORDINARY_WITH_PROOF_NOT_ADMITTED", err)
				}
			})
		}
		for _, mutation := range []struct{ role, want string }{
			{"ACQUISITION_RECEIPT", "acquisition source binding mismatch"},
			{"RETAINED_COMPOSITE", "retained composite bytes mismatch"},
		} {
			t.Run("missing-"+mutation.role, func(t *testing.T) {
				req := withoutRole(nativeCodecInput(t, empty, false), mutation.role)
				emptyCallsPredecessorsWantError(t, req, mutation.want)
			})
		}
		for _, mutation := range []struct{ role, want string }{
			{"ACQUISITION_RECEIPT", "acquisition source binding mismatch"},
			{"RETAINED_COMPOSITE", "retained composite bytes mismatch"},
			{"OCCURRENCE_LEDGER", "invalid occurrence ledger"},
		} {
			t.Run("tampered-"+mutation.role, func(t *testing.T) {
				req := nativeCodecInput(t, empty, false)
				for i := range req.Predecessors {
					if req.Predecessors[i].Role == mutation.role {
						req.Predecessors[i].Bytes = append(req.Predecessors[i].Bytes, ' ')
					}
				}
				emptyCallsPredecessorsWantError(t, req, mutation.want)
			})
		}
		t.Run("extra-role", func(t *testing.T) {
			req := nativeCodecInput(t, empty, false)
			req.Predecessors = append(req.Predecessors, PredecessorRecord{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: []byte("{\"query\":true}\n")})
			emptyCallsPredecessorsWantError(t, req, "predecessor set mismatch")
		})
		t.Run("duplicate-role", func(t *testing.T) {
			req := nativeCodecInput(t, empty, false)
			req.Predecessors = append(req.Predecessors, req.Predecessors[0])
			emptyCallsPredecessorsWantError(t, req, "duplicate predecessor identity")
		})
		for _, callerRoles := range []bool{false, true} {
			t.Run(fmt.Sprintf("orphan-proof-unrequested-C-caller-roles-%t", callerRoles), func(t *testing.T) {
				req := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}).request()
				for _, predecessor := range nativeCodecInput(t, empty, false).Predecessors {
					if predecessor.Role == "NATIVE_EVIDENCE" || (callerRoles && (predecessor.Role == "ACQUISITION_RECEIPT" || predecessor.Role == "RETAINED_COMPOSITE")) {
						req.Predecessors = append(req.Predecessors, predecessor)
					}
				}
				emptyCallsPredecessorsWantError(t, req, "native proof without calls")
			})
		}
		t.Run("tampered-typed-seal", func(t *testing.T) {
			req := nativeCodecInput(t, empty, false)
			req.TypedInput.Authority = 1
			if _, err := BuildInput(req); err == nil {
				t.Fatal("ASSERT_CODEC_EMPTY_TAMPERED_SEAL_REFUSED")
			}
		})
		for _, kind := range []string{"canonical", "source", "ledger"} {
			t.Run("readmission-tampered-native-"+kind, func(t *testing.T) {
				req := nativeCodecInput(t, empty, false)
				var proof PrivateNativeEvidenceV2
				if err := json.Unmarshal(empty.proof, &proof); err != nil {
					t.Fatal(err)
				}
				bad := append([]byte(nil), empty.proof...)
				switch kind {
				case "canonical":
					bad = append(bad, ' ')
				case "source":
					proof.Sources[0].Captures[0].Status = "MISSING"
					bad = nativeCodecJSON(t, proof)
				case "ledger":
					proof.Requests[1].Observations[0].Length++
					bad = nativeCodecJSON(t, proof)
				}
				for i := range req.Predecessors {
					if req.Predecessors[i].Role == "NATIVE_EVIDENCE" {
						req.Predecessors[i].Bytes = bad
					}
				}
				artifact, err := BuildInput(req)
				if err != nil {
					t.Fatal("ASSERT_CODEC_EMPTY_TAMPERED_PROOF_BUILD", err)
				}
				if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || !strings.Contains(err.Error(), ErrPrivateNativeEvidenceV2.Error()) {
					t.Fatal("ASSERT_CODEC_EMPTY_TAMPERED_PROOF_NOT_ADMITTED", err)
				}
			})
		}
	})

	nonempty := nativeCodecFixture(t, root, true)
	t.Run("nonempty-public-full-readmission", func(t *testing.T) {
		req := nativeCodecInput(t, nonempty, false)
		if len(req.TypedInput.Occurrences) == 0 {
			t.Fatal("ASSERT_CODEC_NONEMPTY_CONTROL")
		}
		artifact, err := BuildInput(req)
		if err != nil {
			t.Fatal("ASSERT_CODEC_NONEMPTY_BUILD", err)
		}
		admitted, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits())
		if err != nil || !admitted.MatchesTypedInput(req.TypedInput) || !bytes.Equal(admitted.CanonicalBytes(), artifact.CanonicalBytes()) || admitted.Artifact().Authority() != 0 || admitted.Artifact().Accepted() || admitted.Artifact().Completeness() != "UNKNOWN" {
			t.Fatal("ASSERT_CODEC_NONEMPTY_FULL_READMISSION", err)
		}
		t.Logf("OFFLINE_NONEMPTY_CODEC; BUILD_PARSE_ADMIT_PASSED; CALLS_ROWS=%d; AUTHORITY0_ACCEPTED_FALSE_COMPLETENESS_UNKNOWN", len(req.TypedInput.Occurrences))
	})
	t.Run("readmission-tampered-native-range", func(t *testing.T) {
		req := nativeCodecInput(t, nonempty, false)
		var proof PrivateNativeEvidenceV2
		if err := json.Unmarshal(nonempty.proof, &proof); err != nil || len(proof.RawJoins) == 0 {
			t.Fatal("ASSERT_CODEC_RANGE_CONTROL_PRESENT", err)
		}
		proof.RawJoins[0].Range.End.Character++
		for i := range req.Predecessors {
			if req.Predecessors[i].Role == "NATIVE_EVIDENCE" {
				req.Predecessors[i].Bytes = nativeCodecJSON(t, proof)
			}
		}
		artifact, err := BuildInput(req)
		if err != nil {
			t.Fatal("ASSERT_CODEC_RANGE_TAMPER_BUILD", err)
		}
		if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || !strings.Contains(err.Error(), "raw ordinal join") {
			t.Fatal("ASSERT_CODEC_RANGE_TAMPER_NOT_ADMITTED", err)
		}
	})
	t.Run("readmission-missing-native-proof", func(t *testing.T) {
		req := nativeCodecInput(t, nonempty, false)
		var predecessors []PredecessorRecord
		for _, predecessor := range req.Predecessors {
			if predecessor.Role != "NATIVE_EVIDENCE" {
				predecessors = append(predecessors, predecessor)
			}
		}
		req.Predecessors = predecessors
		artifact, err := BuildInput(req)
		if err != nil {
			t.Fatal("ASSERT_CODEC_CURRENT_BUILD_WITHOUT_PROOF", err)
		}
		if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || err.Error() != "native proof path mismatch" {
			t.Fatal("ASSERT_CODEC_READMISSION_MISSING_PROOF", err)
		}
	})
	t.Run("readmission-ordinary-semantics-with-proof", func(t *testing.T) {
		req := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedCalls}).request()
		req.Predecessors = append(req.Predecessors, PredecessorRecord{Role: "NATIVE_EVIDENCE", Selector: "calls/native-evidence.json", Schema: PrivateNativeEvidenceSchemaV2, Bytes: nonempty.proof})
		artifact, err := BuildInput(req)
		if err != nil {
			t.Fatal("ASSERT_CODEC_ORDINARY_BUILD_WITH_ORPHAN_PROOF", err)
		}
		if _, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits()); err == nil || err.Error() != "native proof path mismatch" {
			t.Fatal("ASSERT_CODEC_READMISSION_SEMANTIC_PARITY", err)
		}
	})
}

type offlineNativeCodecFixture struct{ proof, composite []byte }

func nativeCodecJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func nativeCodecFixture(t *testing.T, workspace string, nonempty bool) offlineNativeCodecFixture {
	t.Helper()
	uri := func(name string) string {
		return (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String()
	}
	rangeAt := func(start, end uint32) lsp.Range {
		return lsp.Range{Start: lsp.Position{Line: 1, Character: start}, End: lsp.Position{Line: 1, Character: end}}
	}
	item := lsp.CallHierarchyItem{Name: "A", Kind: 12, URI: uri("a.go"), Range: rangeAt(0, 16), SelectionRange: rangeAt(5, 6)}
	other := lsp.CallHierarchyItem{Name: "B", Kind: 12, URI: uri("b.go"), Range: rangeAt(0, 11), SelectionRange: rangeAt(5, 6)}
	line, character := uint32(1), uint32(5)
	parent := acquisition.PrivateDirectionalParentV2{OperationID: "offline-codec-no-custody", SessionID: "session", Generation: 1, URI: item.URI, LanguageID: "go", Line: &line, Character: &character, MaxMessages: 1, PositionEncoding: "utf-16", SupplyURI: item.URI, SupplyObservation: json.RawMessage("null")}
	marshal := func(v any) []byte { return bytes.TrimSuffix(nativeCodecJSON(t, v), []byte{'\n'}) }
	params := [][]byte{marshal(map[string]any{"textDocument": map[string]string{"uri": item.URI}, "position": lsp.Position{Line: line, Character: character}}), marshal(map[string]any{"item": item}), marshal(map[string]any{"item": item})}
	results := [][]byte{marshal([]lsp.CallHierarchyItem{item}), []byte("[]"), []byte("[]")}
	if nonempty {
		results[1] = marshal([]lsp.CallHierarchyIncomingCall{{From: other, FromRanges: []lsp.Range{rangeAt(5, 6)}}})
		results[2] = marshal([]lsp.CallHierarchyOutgoingCall{{To: other, FromRanges: []lsp.Range{rangeAt(11, 12)}}})
	}
	methods := []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"}
	p := PrivateNativeEvidenceV2{Schema: PrivateNativeEvidenceSchemaV2, Parent: parent}
	frame := func(body []byte) []byte {
		return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	}
	for i, method := range methods {
		id := uint64(i + 1)
		request := frame([]byte(fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"method\":%q,\"params\":%s}", id, method, params[i])))
		response := frame([]byte(fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}", id, results[i])))
		r := PrivateNativeRequestV2{Ordinal: i, Method: method, RequestID: id, Generation: 1, Params: params[i], Result: results[i], RequestFrame: request, ResponseFrames: [][]byte{response}, Status: "OBSERVED_NONEMPTY"}
		if i > 0 && !nonempty {
			r.Status = "OBSERVED_EMPTY"
		}
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
		p.Requests = append(p.Requests, r)
		parent.Requests = append(parent.Requests, acquisition.PrivateDirectionalObservationV2{Ordinal: i, Method: method, RequestID: id, Generation: 1, Params: params[i], Result: results[i]})
	}
	views, err := acquisition.BuildDirectionalViewsPrivateV2(parent)
	if err != nil {
		t.Fatal("FIRST_CODEC_VALIDATOR BuildDirectionalViewsPrivateV2", err)
	}
	workspaceURI := (&url.URL{Scheme: "file", Path: workspace}).String()
	seed := graph.InvocationSeed{Label: "root", At: "a.go:2:6", ResolvedURI: item.URI, ContentSHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("package fixture\nfunc A() { B() }\n"))), LanguageID: "go"}
	p.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: fmt.Sprintf("offline-codec-nonempty-%t", nonempty), SourceRevision: "commit", ServerVersion: "gopls@1"}}
	captures := 0
	for i, view := range views {
		p.OriginalOrdinals[i] = view.OriginalOrdinals
		captures++ // exactly one constructor invocation for each of two views
		p.Sources[i], err = graphprovenance.CaptureNativeConstructorSourcePrivateV2(context.Background(), view.Result, workspace)
		if err != nil {
			t.Fatalf("FIRST_CODEC_VALIDATOR CaptureNativeConstructorSourcePrivateV2 view=%d nodes=%d edges=%d requests=%d evidence=%d: %v", i, len(view.Result.Graph.Nodes), len(view.Result.Graph.Edges), len(view.Result.Requests), view.Result.Usage.EvidenceBytes, err)
		}
		for _, receipt := range p.Sources[i].Captures {
			if receipt.Status != "READABLE" || (receipt.URI != uri("a.go") && receipt.URI != uri("b.go")) {
				t.Fatal("ASSERT_CODEC_TEMP_RECEIPTS_ONLY")
			}
		}
		if len(p.Sources[i].Captures) == 0 {
			t.Fatal("ASSERT_CODEC_REAL_SOURCE_CAPTURE")
		}
		actual, captured := marshal(view.Result), marshal(p.Sources[i].Acquisition)
		if !bytes.Equal(actual, captured) {
			at := 0
			for at < len(actual) && at < len(captured) && actual[at] == captured[at] {
				at++
			}
			t.Fatalf("FIRST_CODEC_VALIDATOR source acquisition constructor-roundtrip view=%d offset=%d actual=%q captured=%q", i, at, actual[at:min(at+120, len(actual))], captured[at:min(at+120, len(captured))])
		}
		native := view.Result.Graph
		native.SchemaVersion, native.Invocation = graph.SchemaVersionV5, p.Invocation
		native.Capabilities.CallHierarchyProvider = true
		native.Canonicalize()
		graphBytes := marshal(native)
		envelope, err := graphprovenance.CaptureV5(graphBytes, parent.SessionID, parent.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &p.Sources[i])
		if err != nil {
			t.Fatal("FIRST_CODEC_VALIDATOR CaptureV5", err)
		}
		p.Inputs = append(p.Inputs, programccompose.Input{Bytes: envelope, Identity: fmt.Sprintf("%s:view-%d", p.Invocation.Provenance.InvocationID, i), ByteLength: len(envelope), SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(envelope)), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: workspaceURI, RevisionCustody: "CALLER_ASSERTED", PositionEncoding: parent.PositionEncoding, AcquisitionSemantics: "ONE_ACTUAL_DIRECTIONAL_INVOCATION_TWO_VALIDATED_VIEWS", PrivacyPolicy: "private-test-v1"}})
		joins, err := PrivateNativeRangeJoinV2(parent, i, native)
		if err != nil {
			t.Fatal("FIRST_CODEC_VALIDATOR PrivateNativeRangeJoinV2", err)
		}
		p.RawJoins = append(p.RawJoins, joins...)
	}
	if captures != 2 {
		t.Fatal("ASSERT_CODEC_SOURCE_CONSTRUCTOR_COUNT")
	}
	composite, err := programccompose.Compose(p.Inputs)
	if err != nil {
		t.Fatal("FIRST_CODEC_VALIDATOR Compose", err)
	}
	proof, err := EncodePrivateNativeEvidenceV2(p, composite.Bytes)
	if err != nil {
		t.Fatalf("FIRST_CODEC_VALIDATOR EncodeReplay requests=%d inputs=%d joins=%d sourcecaptures=%d/%d nonempty=%t: %v", len(p.Requests), len(p.Inputs), len(p.RawJoins), len(p.Sources[0].Captures), len(p.Sources[1].Captures), nonempty, err)
	}
	if err := ReplayPrivateNativeEvidenceV2(proof, composite.Bytes); err != nil {
		t.Fatal("ASSERT_CODEC_EXPLICIT_REPLAY", err)
	}
	t.Logf("OFFLINE_CODEC_REPLAY_ACCEPTED; NONEMPTY=%t; SOURCE_CONSTRUCTOR_CALLS=%d; SOURCE_CAPTURES=%d/%d; RAW_JOINS=%d; NO_PRODUCER_CUSTODY", nonempty, captures, len(p.Sources[0].Captures), len(p.Sources[1].Captures), len(p.RawJoins))
	return offlineNativeCodecFixture{proof, composite.Bytes}
}

func nativeCodecInput(t *testing.T, fixture offlineNativeCodecFixture, mixed bool) InputRequest {
	t.Helper()
	calls, err := programcadmission.Admit(fixture.composite)
	if err != nil {
		t.Fatal("ASSERT_CODEC_COMPOSITE_ADMIT", err)
	}
	f := referenceTestFixture{}
	typedReq := programcadmission.TypedInputRequest{Calls: calls.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}}
	if mixed {
		f = referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
		projection, err := programcadmission.PreparePrivateLeidenProjection(f.typed)
		if err != nil {
			t.Fatal(err)
		}
		typedReq.Methods, err = admitCanonicalMethodEvidence(projection.MethodCanonicalBytes())
		if err != nil {
			t.Fatal(err)
		}
		typedReq.EnableMethods = true
		typedReq.Kinds = append(typedReq.Kinds, programcadmission.TypedReferencesSymbol)
	}
	f.typed, err = programcadmission.PrepareTypedInput(typedReq)
	if err != nil {
		t.Fatal("ASSERT_CODEC_SEALED_TYPED_INPUT", err)
	}
	f.ledger = canonicalOccurrenceLedgerLiteral(t, f.typed.Occurrences)
	if !mixed {
		f.predecessors = []PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: f.ledger}}
	} else {
		for i := range f.predecessors {
			if f.predecessors[i].Role == "OCCURRENCE_LEDGER" {
				f.predecessors[i].Bytes = f.ledger
			}
		}
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(f.typed)
	if err != nil {
		t.Fatal(err)
	}
	f.predecessors = append(f.predecessors,
		PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()},
		PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: canonicalCompositeSourceBindingLiteral(t, projection.CompositeSourceBinding())},
		PredecessorRecord{Role: "NATIVE_EVIDENCE", Selector: "calls/native-evidence.json", Schema: PrivateNativeEvidenceSchemaV2, Bytes: fixture.proof},
	)
	seen := map[string]bool{}
	for _, occurrence := range projection.Occurrences() {
		if occurrence.Kind != programcadmission.TypedCalls {
			continue
		}
		for _, endpoint := range []string{occurrence.From, occurrence.To} {
			if !seen[endpoint] {
				seen[endpoint] = true
				f.custody = append(f.custody, EndpointCustody{Endpoint: endpoint, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"})
			}
		}
	}
	return f.request()
}
