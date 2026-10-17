package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
)

func unavailableDiagnostics() manageddiagnostic.QueryResult {
	return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}
}

func omissionBearingResult(t *testing.T) acquisition.Result {
	t.Helper()
	result, _ := coordinatorV2Fixture(t, func(r *acquisition.Request) { r.TopmostSiblings = true })
	if len(result.Omissions) != 1 || result.Omissions[0].Reason != acquisition.OmissionNoMatch {
		t.Fatalf("ASSERT_V4_FIXTURE_HAS_REAL_OMISSION: %+v", result.Omissions)
	}
	if err := acquisition.ValidateResult(result); err != nil {
		t.Fatalf("ASSERT_V4_FIXTURE_VALID: %v", err)
	}
	return result
}

func cloneAcquisitionV4(t *testing.T, result acquisition.Result) acquisition.Result {
	t.Helper()
	raw, err := CaptureV4(result, unavailableDiagnostics())
	if err != nil {
		t.Fatal(err)
	}
	var evidence EvidenceV4
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	return evidence.Acquisition
}

func omissionResultV4(t *testing.T, reason acquisition.OmissionReason) acquisition.Result {
	return omissionResultsV4(t, reason, 1)
}

func omissionResultsV4(t *testing.T, reason acquisition.OmissionReason, count int) acquisition.Result {
	return omissionResultsWithPaddingV4(t, reason, count, 0)
}

func omissionResultsWithPaddingV4(t *testing.T, reason acquisition.OmissionReason, count, errorPadding int) acquisition.Result {
	t.Helper()
	root := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	line, character := uint32(0), uint32(0)
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &line, Character: &character}, DownDepth: 1, UpDepth: 1}
	request := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 10000, MaxRequests: 100000, MaxEvidenceBytes: 64 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 16 << 20, MaxMessages: 64}, TopmostSiblings: true}
	if reason == acquisition.OmissionNodeAdmissionBlocked {
		request.Limits.MaxNodes = 1
	}
	if reason == acquisition.OmissionRequestBudgetBlocked {
		request.Limits.MaxRequests = 2
	}
	seed := lsp.CallHierarchyItem{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Character: 10}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}
	peer := lsp.CallHierarchyItem{Name: "Peer", Kind: 12, URI: uri, Range: lsp.Range{Start: lsp.Position{Character: 2}, End: lsp.Position{Character: 9}}, SelectionRange: lsp.Range{Start: lsp.Position{Character: 2}, End: lsp.Position{Character: 3}}}
	client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
		switch wire.Method {
		case "textDocument/documentSymbol":
			if reason == acquisition.OmissionRequestFailed {
				return nil, errors.New("document symbol failed" + strings.Repeat("x", errorPadding))
			}
			children := []lsp.DocumentSymbol{{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange}}
			for i := 0; i < count; i++ {
				declaration := lsp.DocumentSymbol{Name: fmt.Sprintf("Peer%06d", i), Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange}
				if reason == acquisition.OmissionInvalidDeclaration {
					line := uint32(i + 1)
					declaration.Range = lsp.Range{Start: lsp.Position{Line: line, Character: 2}, End: lsp.Position{Line: line, Character: 1}}
					declaration.SelectionRange = lsp.Range{Start: lsp.Position{Line: line, Character: 2}, End: lsp.Position{Line: line, Character: 3}}
				}
				children = append(children, declaration)
			}
			return json.Marshal([]lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: uint32(count + 2)}}, Children: children}})
		case "textDocument/prepareCallHierarchy":
			var params lsp.PrepareCallHierarchyParams
			if err := json.Unmarshal(wire.Params, &params); err != nil {
				return nil, err
			}
			if params.Position == seed.SelectionRange.Start {
				return json.Marshal([]lsp.CallHierarchyItem{seed})
			}
			candidate := peer
			candidate.Name = "Peer000000"
			switch reason {
			case acquisition.OmissionNoMatch:
				return json.RawMessage(`[]`), nil
			case acquisition.OmissionAmbiguousMatch:
				return json.Marshal([]lsp.CallHierarchyItem{candidate, candidate})
			case acquisition.OmissionPreparationFailed:
				return nil, errors.New("sibling preparation failed")
			default:
				return json.Marshal([]lsp.CallHierarchyItem{candidate})
			}
		default:
			return json.RawMessage(`[]`), nil
		}
	})
	result, err := acquisition.Acquire(context.Background(), client, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Omissions) != count {
		t.Fatalf("ASSERT_V4_%s_OMISSION_COUNT: got=%d want=%d", reason, len(result.Omissions), count)
	}
	for _, omission := range result.Omissions {
		if omission.Reason != reason {
			t.Fatalf("ASSERT_V4_%s_REASON: %+v", reason, omission)
		}
	}
	if err := acquisition.ValidateResult(result); err != nil {
		t.Fatalf("ASSERT_V4_%s_FIXTURE_VALID: %v", reason, err)
	}
	return result
}

func captureAndValidateV4(t *testing.T, result acquisition.Result) []byte {
	t.Helper()
	raw, err := CaptureV4(result, unavailableDiagnostics())
	if err != nil {
		t.Fatalf("ASSERT_V4_CAPTURE: %v", err)
	}
	if got, err := ValidateFor(raw, Family, "v4"); err != nil || got != VersionV4 {
		t.Fatalf("ASSERT_V4_VALIDATE: got=%q err=%v", got, err)
	}
	return raw
}

func TestV4EveryAcquisitionOmissionReasonRoundTripsExactly(t *testing.T) {
	reasons := []acquisition.OmissionReason{
		acquisition.OmissionNoMatch,
		acquisition.OmissionAmbiguousMatch,
		acquisition.OmissionNodeAdmissionBlocked,
		acquisition.OmissionInvalidDeclaration,
		acquisition.OmissionRequestFailed,
		acquisition.OmissionPreparationFailed,
		acquisition.OmissionRequestBudgetBlocked,
	}
	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			result := omissionResultV4(t, reason)
			raw := captureAndValidateV4(t, result)
			var evidence EvidenceV4
			if err := json.Unmarshal(raw, &evidence); err != nil {
				t.Fatal(err)
			}
			if len(evidence.Acquisition.Omissions) != 1 || evidence.Acquisition.Omissions[0] != result.Omissions[0] {
				t.Fatalf("ASSERT_V4_EXACT_OMISSION_%s: got=%+v want=%+v", reason, evidence.Acquisition.Omissions, result.Omissions)
			}
			reencoded, err := json.Marshal(evidence)
			if err != nil || !bytes.Equal(bytes.TrimSuffix(raw, []byte{'\n'}), reencoded) {
				t.Fatalf("ASSERT_V4_CANONICAL_OMISSION_%s: %v", reason, err)
			}
		})
	}
}

func TestV4OmissionSchemaAndCardinalityBoundary(t *testing.T) {
	schemaBytes, err := schema.BytesFor(Family, "v4")
	if err != nil {
		t.Fatal(err)
	}
	needle := []byte(fmt.Sprintf(`"omissions":{"type":"array","maxItems":%d`, MaxOmissionsV4))
	compact := new(bytes.Buffer)
	if err := json.Compact(compact, schemaBytes); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(compact.Bytes(), needle) {
		t.Fatalf("ASSERT_V4_OMISSION_SCHEMA_BOUND_TO_CONSTANT: missing %s", needle)
	}
	result := omissionResultV4(t, acquisition.OmissionNoMatch)
	template := result.Omissions[0]
	result.Omissions = make([]acquisition.Omission, MaxOmissionsV4)
	for i := range result.Omissions {
		result.Omissions[i] = template
		result.Omissions[i].NodeID = fmt.Sprintf("omitted-%06d", i)
	}
	if _, err := CaptureV4(result, unavailableDiagnostics()); err == nil || err.Error() == "V4 omission count exceeds bound" {
		t.Fatalf("ASSERT_V4_OMISSION_EXACT_MAX_REACHES_SEMANTIC_VALIDATION: %v", err)
	}
	result.Omissions = append(result.Omissions, acquisition.Omission{Reason: template.Reason, NodeID: "omitted-plus-one", RequestID: template.RequestID})
	if _, err := CaptureV4(result, unavailableDiagnostics()); err == nil || err.Error() != "V4 omission count exceeds bound" {
		t.Fatalf("ASSERT_V4_OMISSION_PLUS_ONE_REFUSED: %v", err)
	}
}

func diagnosticQueryV4(sessionID string, generation uint64, n int) manageddiagnostic.QueryResult {
	records := make([]manageddiagnostic.Record, n)
	for i := range records {
		records[i] = manageddiagnostic.RecordCapabilityCheck(manageddiagnostic.CapabilityObservation{SessionID: sessionID, Generation: generation, Sequence: uint64(i + 1), Supported: true})
		records[i].Read.State, records[i].Write.State = manageddiagnostic.IOUnavailable, manageddiagnostic.IOUnavailable
	}
	return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: records}
}

func TestV4DiagnosticCollectionExactAndPlusOne(t *testing.T) {
	result := omissionBearingResult(t)
	query := diagnosticQueryV4(result.Request.Context.SessionID, result.Request.Context.Generation, MaxDiagnosticRecords)
	raw, err := CaptureV4(result, query)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(raw, Family, "v4"); err != nil {
		t.Fatalf("ASSERT_V4_DIAGNOSTICS_EXACT_MAX: %v", err)
	}
	var evidence EvidenceV4
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	extra := evidence.Diagnostics.Records[len(evidence.Diagnostics.Records)-1]
	extra.Sequence++
	evidence.Diagnostics.Records = append(evidence.Diagnostics.Records, extra)
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(append(encoded, '\n'), Family, "v4"); err == nil {
		t.Fatal("ASSERT_V4_DIAGNOSTICS_PLUS_ONE_REFUSED")
	}
}

func jsonDepthV4(raw []byte) int {
	depth, maxDepth := 0, 0
	inString, escaped := false, false
	for _, b := range raw {
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}', ']':
			depth--
		}
	}
	return maxDepth
}

func nestedJSONV4(depth int) json.RawMessage {
	return json.RawMessage(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
}

func setOpaqueNodeDataV4(t *testing.T, result *acquisition.Result, data json.RawMessage) {
	t.Helper()
	old := result.Graph.Nodes[0].Data
	result.Graph.Nodes[0].Data = append(json.RawMessage(nil), data...)
	for i := range result.Targets {
		if result.Targets[i].Resolution.Identity != nil && result.Targets[i].Resolution.Identity.ID == result.Graph.Nodes[0].ID {
			result.Targets[i].Resolution.Identity.Data = append(json.RawMessage(nil), data...)
		}
		if result.Targets[i].Resolution.Prepared != nil && bytes.Equal(result.Targets[i].Resolution.Prepared.Data, old) {
			result.Targets[i].Resolution.Prepared.Data = append(json.RawMessage(nil), data...)
		}
	}
	for i := range result.Requests {
		for _, carrier := range []*json.RawMessage{&result.Requests[i].Params, &result.Requests[i].Response} {
			if len(*carrier) == 0 || !bytes.Contains(*carrier, old) {
				continue
			}
			replaced := bytes.ReplaceAll(*carrier, old, data)
			result.Requests[i].EvidenceBytes += len(replaced) - len(*carrier)
			result.Usage.EvidenceBytes += len(replaced) - len(*carrier)
			*carrier = replaced
		}
	}
	attempts, probes, evidenceBytes := 0, 0, 0
	for i := range result.Requests {
		result.Requests[i].Before.Requests = attempts
		result.Requests[i].Before.PrepareProbes = probes
		result.Requests[i].Before.EvidenceBytes = evidenceBytes
		evidenceBytes += result.Requests[i].EvidenceBytes
		if result.Requests[i].Attempted {
			attempts++
			if result.Requests[i].Method == "textDocument/prepareCallHierarchy" {
				probes++
			}
		}
	}
	if err := acquisition.ValidateResult(*result); err != nil {
		t.Fatalf("ASSERT_V4_OPAQUE_DATA_FIXTURE_VALID: %v", err)
	}
}

func TestV4JSONDepthExactAndPlusOneThroughValidator(t *testing.T) {
	base := captureAndValidateV4(t, omissionBearingResult(t))
	for _, depth := range []int{MaxDepthV2, MaxDepthV2 + 1} {
		prefix := []byte(`{"unknown":`)
		bad := append(prefix, nestedJSONV4(depth-1)...)
		bad = append(bad, ',')
		bad = append(bad, base[1:]...)
		_, err := ValidateFor(bad, Family, "v4")
		if err == nil {
			t.Fatalf("ASSERT_V4_DEPTH_%d_REJECTED_LATER_OR_AT_BOUND", depth)
		}
		depthError := strings.Contains(err.Error(), "V2 known-carrier depth limit")
		if depth == MaxDepthV2 && depthError {
			t.Fatalf("ASSERT_V4_DEPTH_EXACT_REACHES_STRUCTURAL_VALIDATION: %v", err)
		}
		if depth == MaxDepthV2+1 && !depthError {
			t.Fatalf("ASSERT_V4_DEPTH_PLUS_ONE_PREFLIGHT_REFUSAL: %v", err)
		}
	}
}

func TestV4EnvelopeBytesExactAndPlusOneThroughValidator(t *testing.T) {
	base := captureAndValidateV4(t, omissionBearingResult(t))
	exact := append(append([]byte(nil), bytes.TrimSpace(base)...), bytes.Repeat([]byte{' '}, MaxEnvelopeBytesV4-len(bytes.TrimSpace(base)))...)
	if len(exact) != MaxEnvelopeBytesV4 {
		t.Fatalf("ASSERT_V4_BYTES_EXACT_MAX_LENGTH: got=%d want=%d", len(exact), MaxEnvelopeBytesV4)
	}
	if _, err := ValidateFor(exact, Family, "v4"); err == nil || err.Error() == "V4 envelope byte limit" {
		t.Fatalf("ASSERT_V4_BYTES_EXACT_MAX_REACHES_CANONICAL_VALIDATION: %v", err)
	}
	plusOne := append(exact, ' ')
	if _, err := ValidateFor(plusOne, Family, "v4"); err == nil || err.Error() != "V4 envelope byte limit" {
		t.Fatalf("ASSERT_V4_BYTES_PLUS_ONE_REFUSED: %v", err)
	}
}

func mutateCallerOwnedV4(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			mutateCallerOwnedV4(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			mutateCallerOwnedV4(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if v.Type().Elem().Kind() == reflect.Uint8 && v.Index(i).CanSet() {
				v.Index(i).SetUint(v.Index(i).Uint() ^ 0xff)
			} else {
				mutateCallerOwnedV4(v.Index(i))
			}
		}
		if v.CanSet() {
			v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(v.MapIndex(key))
			mutateCallerOwnedV4(value)
			v.SetMapIndex(key, value)
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("mutated-after-capture")
		}
	}
}

func TestV4CaptureSynchronizedOwnershipHandoffDetachesSameInput(t *testing.T) {
	result := omissionBearingResult(t)
	query := unavailableDiagnostics()
	captured := make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-captured
		mutateCallerOwnedV4(reflect.ValueOf(&result).Elem())
		mutateCallerOwnedV4(reflect.ValueOf(&query).Elem())
		close(done)
	}()
	raw, err := CaptureV4(result, query)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(raw)
	close(captured)
	<-done
	if !bytes.Equal(raw, before) {
		t.Fatal("ASSERT_V4_POST_RETURN_OWNERSHIP_HANDOFF_CANONICAL_BYTES_DETACHED")
	}
	if _, err := ValidateFor(raw, Family, "v4"); err != nil {
		t.Fatalf("ASSERT_V4_POST_RETURN_OWNERSHIP_HANDOFF_VALID: %v", err)
	}
	var evidence EvidenceV4
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.AcquisitionSHA256 == "" || evidence.GraphSHA256 == "" {
		t.Fatal("ASSERT_V4_POST_RETURN_OWNERSHIP_HANDOFF_DIGESTS_PRESERVED")
	}
}

func TestV4CaptureDeepDetachesEveryReachableCallerOwnedInput(t *testing.T) {
	result := omissionBearingResult(t)
	query := unavailableDiagnostics()
	raw, err := CaptureV4(result, query)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(raw)
	mutateCallerOwnedV4(reflect.ValueOf(&result).Elem())
	mutateCallerOwnedV4(reflect.ValueOf(&query).Elem())
	if !bytes.Equal(raw, before) {
		t.Fatal("ASSERT_V4_DEEP_DETACHMENT_CANONICAL_BYTES_UNCHANGED")
	}
	if _, err := ValidateFor(raw, Family, "v4"); err != nil {
		t.Fatalf("ASSERT_V4_DEEP_DETACHMENT_VALID: %v", err)
	}
	var evidence EvidenceV4
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.AcquisitionSHA256 == "" || evidence.GraphSHA256 == "" {
		t.Fatal("ASSERT_V4_DEEP_DETACHMENT_DIGESTS_PRESERVED")
	}
}

func TestV4RejectsStrictAndSemanticMutations(t *testing.T) {
	result := omissionBearingResult(t)
	raw, err := CaptureV4(result, unavailableDiagnostics())
	if err != nil {
		t.Fatal(err)
	}
	var evidence EvidenceV4
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*EvidenceV4){
		"unknown request":  func(e *EvidenceV4) { e.Acquisition.Omissions[0].RequestID = "request-999999" },
		"unknown reason":   func(e *EvidenceV4) { e.Acquisition.Omissions[0].Reason = acquisition.OmissionReason("UNKNOWN") },
		"reason mismatch":  func(e *EvidenceV4) { e.Acquisition.Omissions[0].Reason = acquisition.OmissionRequestBudgetBlocked },
		"session mismatch": func(e *EvidenceV4) { e.SessionID = "different" },
		"graph digest":     func(e *EvidenceV4) { e.GraphSHA256 = "sha256:" + strings.Repeat("0", 64) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			clone := evidence
			encoded, _ := json.Marshal(evidence)
			_ = json.Unmarshal(encoded, &clone)
			mutate(&clone)
			bad, _ := json.Marshal(clone)
			if _, err := ValidateFor(append(bad, '\n'), Family, "v4"); err == nil {
				t.Fatalf("ASSERT_V4_MUTATION_REJECTED: %s", name)
			}
		})
	}
	unknown := bytes.Replace(raw, []byte(`"schema_version"`), []byte(`"unknown":0,"schema_version"`), 1)
	duplicate := bytes.Replace(raw, []byte(`"schema_version"`), []byte(`"schema_version":"lsp-trace.graph-provenance.v4","schema_version"`), 1)
	for name, bad := range map[string][]byte{"unknown": unknown, "duplicate": duplicate} {
		if _, err := ValidateFor(bad, Family, "v4"); err == nil {
			t.Fatalf("ASSERT_V4_STRICT_REJECTED: %s", name)
		}
	}
}
