package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/schema"
)

func reviewEnvelope(t *testing.T) ([]byte, EvidenceV2) {
	t.Helper()
	r, root := suppliedV2Fixture(t, acquisition.Slice, true, false)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(raw, Family, "v2"); err != nil {
		t.Fatal("baseline", err)
	}
	return raw, e
}
func reviewAccounting(e *EvidenceV2) {
	delta := 0
	for i := range e.Acquisition.Requests {
		r := &e.Acquisition.Requests[i]
		r.Before.EvidenceBytes += delta
		if r.CaptureComplete {
			n := len(r.Params) + len(r.Response) + 2
			delta += n - r.EvidenceBytes
			r.EvidenceBytes = n
		}
	}
	e.Acquisition.Usage.EvidenceBytes += delta
}
func TestReviewResealedRejects(t *testing.T) {
	raw, _ := reviewEnvelope(t)
	cases := map[string]func(*EvidenceV2){
		"required_row":    func(e *EvidenceV2) { e.Acquisition.Targets = e.Acquisition.Targets[:1] },
		"request_row":     func(e *EvidenceV2) { e.Acquisition.Requests = e.Acquisition.Requests[1:] },
		"observation_row": func(e *EvidenceV2) { e.Acquisition.EdgeObservations = e.Acquisition.EdgeObservations[1:] },
		"supply_row":      func(e *EvidenceV2) { e.Supplies = e.Supplies[1:] },
		"capture_row":     func(e *EvidenceV2) { e.Captures = nil },
		"foreign_receipt": func(e *EvidenceV2) { e.Bindings[0].ReceiptIDs = []string{"sha256:" + strings.Repeat("a", 64)} },
		"wrong_source_uri_resealed": func(e *EvidenceV2) {
			r := e.Captures[0]
			r.URI += "wrong"
			r.Path += "wrong"
			e.Captures[0] = receiptV2(r.URI, r.Path, r.Classification, r.Status, r.Content, nil)
			_ = bindV2(e)
		},
		"changed_bytes_outer_resealed": func(e *EvidenceV2) {
			r := &e.Captures[0]
			r.Content = bytes.Repeat([]byte("X"), len(r.Content))
			*r = sealV2(*r)
			_ = bindV2(e)
		},
		"canonical_metadata_resealed": func(e *EvidenceV2) {
			r := &e.Captures[0]
			var m map[string]any
			_ = json.Unmarshal(r.CanonicalReceipt, &m)
			m["byte_count"] = 123
			r.CanonicalReceipt, _ = json.Marshal(m)
			*r = sealV2(*r)
			_ = bindV2(e)
		},
		"supply_version_resealed": func(e *EvidenceV2) { r := e.Supplies[1].Receipt; r.Supply.Version++; *r = sealV2(*r); _ = bindV2(e) },
		"supply_request_join":     func(e *EvidenceV2) { e.Supplies[0].RequestID = e.Supplies[1].RequestID },
		"source_version_auth":     func(e *EvidenceV2) { e.AnalyzedVersion = "VERIFIED" },
		"response_ranges_reaccounted": func(e *EvidenceV2) {
			for i := range e.Acquisition.Requests {
				r := &e.Acquisition.Requests[i]
				if r.Method == "callHierarchy/outgoingCalls" {
					var rows []lsp.CallHierarchyOutgoingCall
					_ = json.Unmarshal(r.Response, &rows)
					if len(rows) > 0 {
						rows[0].FromRanges[0].End.Character++
						r.Response, _ = json.Marshal(rows)
						break
					}
				}
			}
			reviewAccounting(e)
		},
		"wrong_query_uri_reaccounted": func(e *EvidenceV2) {
			for i := range e.Acquisition.Requests {
				r := &e.Acquisition.Requests[i]
				if r.Method == "callHierarchy/outgoingCalls" {
					r.Params = bytes.Replace(r.Params, []byte("a.go"), []byte("b.go"), 1)
					break
				}
			}
			reviewAccounting(e)
		},
		"substituted_valid_graph_rehashed": func(e *EvidenceV2) {
			r, _ := coordinatorV2Fixture(t, nil)
			e.Acquisition.Graph = r.Graph
			e.GraphBytes, _ = json.Marshal(r.Graph)
			e.GraphDigest = digest(VersionV2+":graph", e.GraphBytes)
		},
		"graph_hidden_seed_rehashed": func(e *EvidenceV2) {
			e.Acquisition.Graph.Seeds = e.Acquisition.Graph.Seeds[:1]
			e.GraphBytes, _ = json.Marshal(e.Acquisition.Graph)
			e.GraphDigest = digest(VersionV2+":graph", e.GraphBytes)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var e EvidenceV2
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			mutate(&e)
			bad, err := json.Marshal(e)
			if err == nil {
				_, err = ValidateFor(bad, Family, "v2")
			}
			if err == nil {
				t.Fatal("accepted contradictory records")
			}
			t.Log("REJECT", err)
		})
	}
}
func TestReviewEveryBindingDeletion(t *testing.T) {
	raw, base := reviewEnvelope(t)
	for i := range base.Bindings {
		var e EvidenceV2
		_ = json.Unmarshal(raw, &e)
		e.Bindings = append(e.Bindings[:i], e.Bindings[i+1:]...)
		if err := ValidateV2(e); err == nil {
			t.Fatalf("accepted omitted %s", base.Bindings[i].Pointer)
		}
	}
	t.Logf("all %d individual binding deletions rejected", len(base.Bindings))
}
func TestReviewSourceGoneRootUnavailableAndDepth(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.CaptureBudget.RootStatus != "UNAVAILABLE" || e.CaptureBudget.Attempts != 0 || e.Captures[0].Status != "UNREADABLE" {
		t.Fatal("root unavailable lost")
	}
	if err = ValidateV2(e); err != nil {
		t.Fatal(err)
	}
	for _, depth := range []int{64, 65} {
		err := preflightV2([]byte(strings.Repeat("[", depth)+"0"+strings.Repeat("]", depth)), MaxEnvelopeBytesV2)
		if (err == nil) != (depth == 64) {
			t.Fatalf("depth %d: %v", depth, err)
		}
	}
	if _, err = schema.BytesFor(Family, VersionV2); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{Family, "retained-calls", "graph"} {
		for _, v := range []string{"", "v1", "v2", VersionV2} {
			_, err := ValidateFor(raw, f, v)
			want := f == Family && (v == "v2" || v == VersionV2)
			if (err == nil) != want {
				t.Fatalf("dispatch %s/%s: %v", f, v, err)
			}
		}
	}
	t.Log("root unavailable/offline, 64 accepted/65 rejected, exact family dispatch PASS")
}
func TestReviewUnavailableCapturesAndCarrierBounds(t *testing.T) {
	for _, name := range []string{"FAILED", "CAPTURE_FAILED", "CAPTURE_INCOMPLETE"} {
		t.Run(name, func(t *testing.T) {
			base, root := coordinatorV2Fixture(t, nil)
			req := base.Request
			req.RequiredTargets = nil
			item := *base.Targets[0].Resolution.Prepared
			wire := acquisition.NewWireClient(func(context.Context, acquisition.WireRequest) (json.RawMessage, error) {
				return nil, fmt.Errorf("synthetic provider failure file:///not-source.go")
			})
			var client acquisition.Client = wire
			if name == "CAPTURE_FAILED" {
				item.Data = json.RawMessage(`{`)
				client = reviewPreparedClient{wire, item}
			}
			if name == "CAPTURE_INCOMPLETE" {
				req.Limits.MaxResponseBytes = 10
				client = reviewPreparedClient{wire, item}
			}
			r, err := acquisition.Acquire(context.Background(), client, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Requests) != 1 || r.Requests[0].Outcome != name || r.Targets[0].Resolution.Prepared != nil {
				t.Fatalf("unexpected fixture %+v", r.Requests)
			}
			raw, err := CaptureV2(context.Background(), r, root)
			if err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			_ = json.Unmarshal(raw, &e)
			for _, b := range e.Bindings {
				if strings.Contains(b.URI, "not-source.go") || strings.HasPrefix(b.Pointer, "/acquisition/requests/0/response/") {
					t.Fatalf("invented anchor %+v", b)
				}
			}
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if err := ValidateV2(e); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, depth := range []int{64, 65} {
		payload, _ := json.Marshal(map[string][]byte{"graph_bytes": []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))})
		err := preflightV2(payload, MaxEnvelopeBytesV2)
		if (err == nil) != (depth == 64) {
			t.Fatalf("encoded depth %d: %v", depth, err)
		}
	}
	for _, n := range []int{10000, 10001} {
		err := preflightGraphV2([]byte(`{"nodes":[` + strings.Repeat(`{},`, n-1) + `{}]}`))
		if (err == nil) != (n == 10000) {
			t.Fatalf("raw nodes %d: %v", n, err)
		}
	}
	r, root := coordinatorV2Fixture(t, nil)
	if err := os.WriteFile(root+"/a.go", nil, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	_ = json.Unmarshal(raw, &e)
	if e.Captures[0].Status != "READABLE" || e.CaptureBudget.ChargedBytes != 0 {
		t.Fatal("empty readable file")
	}
	t.Log("unavailable failures, empty file, encoded depth64/65, raw nodes10000/10001 PASS")
}
func TestReviewCoherentCaptureIsNotAuthentication(t *testing.T) {
	_, e := reviewEnvelope(t)
	r := e.Captures[0]
	r.Content = bytes.Repeat([]byte("Y"), len(r.Content))
	e.Captures[0] = receiptV2(r.URI, r.Path, r.Classification, r.Status, r.Content, nil)
	_ = bindV2(&e)
	if err := ValidateV2(e); err != nil {
		t.Fatal("fully coherent later capture is not authenticated", err)
	}
	t.Log("fully coherent substitute post-capture bytes accepted as expected, no authentication claim")
}
func TestReviewNativeReferenceCensus(t *testing.T) {
	_, e := reviewEnvelope(t)
	if len(e.Acquisition.Targets[1].Connection.Path.GroupIDs) == 0 {
		t.Fatal("fixture has no connection")
	}
	wanted := []string{"/acquisition/targets/1/connection/path/group_ids/0", "/acquisition/targets/1/connection/path/occurrence_ids/0/0", "/graph/seeds/0/reached_relation_ids/0"}
	by := map[string]BindingV2{}
	for _, b := range e.Bindings {
		by[b.Pointer] = b
	}
	for _, p := range wanted {
		if _, ok := by[p]; !ok {
			t.Errorf("missing native reference census %s", p)
		}
	}
}

type reviewPreparedClient struct {
	acquisition.Client
	item lsp.CallHierarchyItem
}

func (c reviewPreparedClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return []lsp.CallHierarchyItem{c.item}, nil
}
func TestReviewCanonicalGraphAndOpaqueNumbers(t *testing.T) {
	raw, e := reviewEnvelope(t)
	var formatted bytes.Buffer
	_ = json.Indent(&formatted, e.GraphBytes, "", "  ")
	e.GraphBytes = formatted.Bytes()
	e.GraphDigest = digest(VersionV2+":graph", e.GraphBytes)
	if err := ValidateV2(e); err == nil {
		t.Fatal("noncanonical native bytes accepted")
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	doc["foreign"] = json.RawMessage(`true`)
	bad, _ := json.Marshal(doc)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil {
		t.Fatal("unknown envelope field")
	}
	r, root := coordinatorV2Fixture(t, nil)
	item := *r.Targets[0].Resolution.Prepared
	item.Data = json.RawMessage(`{"n":18446744073709551615,"huge":1e1000000000,"deep":` + strings.Repeat("[", 100) + `0` + strings.Repeat("]", 100) + `}`)
	wire := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
		if q.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{item})
		}
		return json.RawMessage(`[]`), nil
	})
	r, err := acquisition.Acquire(context.Background(), reviewPreparedClient{wire, item}, r.Request)
	if err != nil {
		t.Fatal(err)
	}
	if r.Targets[0].Resolution.Status != acquisition.Resolved {
		t.Fatalf("numeric fixture unresolved: %+v", r.Targets[0].Resolution)
	}
	raw, err = CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var got EvidenceV2
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	restored, _ := json.Marshal(got.Acquisition)
	before, _ := json.Marshal(r)
	if !bytes.Equal(before, restored) || !bytes.Contains(restored, []byte("18446744073709551615")) || !bytes.Contains(restored, []byte("1e1000000000")) {
		t.Fatal("opaque number or descriptor changed")
	}
	for _, b := range got.Bindings {
		if strings.Contains(b.Pointer, "/data/") {
			t.Fatal("opaque census")
		}
	}
	t.Log("canonical graph strict, unknown envelope rejected, uint64-max/huge opaque exponent/depth100 preserved")
}
func TestReviewPositionSelectorCensus(t *testing.T) {
	r, _ := coordinatorV2Fixture(t, func(r *acquisition.Request) { r.Limits.MaxRequests = 0 })
	bs, err := CensusV2(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		if strings.HasPrefix(b.Pointer, "/acquisition/request/root/locator") {
			t.Logf("binding %+v", b)
			if strings.HasSuffix(b.Pointer, "/line") || strings.HasSuffix(b.Pointer, "/character") || b.Pointer == "/acquisition/request/root/locator" {
				return
			}
		}
	}
	t.Fatal("coordinate selector has no census anchor; only URI survives zero-request budget")
}
func TestReviewSupplyLanguageConsistency(t *testing.T) {
	_, e := reviewEnvelope(t)
	for i := range e.Acquisition.Supplies {
		e.Acquisition.Supplies[i].LanguageID = "typescript"
	}
	for i := range e.Acquisition.Requests {
		r := &e.Acquisition.Requests[i]
		if r.Method == "source/prepareDocument" {
			var s acquisition.Supply
			_ = json.Unmarshal(r.Response, &s)
			s.LanguageID = "typescript"
			r.Response, _ = json.Marshal(s)
		}
	}
	reviewAccounting(&e)
	var err error
	e.Supplies, err = suppliesV2(e.Acquisition, e.WorkspaceURI)
	if err != nil {
		t.Log("rejected", err)
		return
	}
	_ = bindV2(&e)
	raw, err := json.Marshal(e)
	if err == nil {
		_, err = ValidateFor(raw, Family, "v2")
	}
	if err == nil {
		t.Fatal("accepted coordinator language typescript with didOpen languageId go")
	}
}
func TestReviewMalformedResponseAnchor(t *testing.T) {
	base, root := coordinatorV2Fixture(t, nil)
	req := base.Request
	req.RequiredTargets = nil
	a := *base.Targets[0].Resolution.Prepared
	backwards := lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 1}}
	wire := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
		switch q.Method {
		case "textDocument/prepareCallHierarchy":
			return json.Marshal([]lsp.CallHierarchyItem{a})
		case "callHierarchy/outgoingCalls":
			return json.Marshal([]lsp.CallHierarchyOutgoingCall{{To: a, FromRanges: []lsp.Range{backwards}}})
		}
		return json.RawMessage(`[]`), nil
	})
	r, err := acquisition.Acquire(context.Background(), wire, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Graph.Edges) != 0 || r.Targets[0].Outgoing.Status != acquisition.Partial {
		t.Fatal("malformed fixture not partial")
	}
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	_ = json.Unmarshal(raw, &e)
	for i, q := range r.Requests {
		if q.Method == "callHierarchy/outgoingCalls" {
			p := fmt.Sprintf("/acquisition/requests/%d/response/0/fromRanges/0", i)
			for _, b := range e.Bindings {
				if b.Pointer == p {
					t.Logf("malformed backwards range bound %+v", b)
					encoded, _ := json.Marshal(b)
					var fields map[string]any
					_ = json.Unmarshal(encoded, &fields)
					if b.Attribution != "SOURCE" || b.URI != a.URI || fields["anchor_status"] != "INVALID_COORDINATES" {
						t.Fatal("invalid range has SOURCE binding without unavailable-range disposition")
					}
				}
			}
		}
	}
}
