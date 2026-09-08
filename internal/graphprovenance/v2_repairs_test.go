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
	"lsp-trace/sessionruntime"
)

func TestV2RepairSelectorPointers(t *testing.T) {
	for _, state := range []string{"zero_requests", "unadmitted", "alias", "missing"} {
		t.Run(state, func(t *testing.T) {
			r, root := coordinatorV2Fixture(t, func(r *acquisition.Request) {
				switch state {
				case "zero_requests":
					r.Limits.MaxRequests = 0
				case "unadmitted":
					r.Limits.MaxNodes = 0
				case "missing":
					r.Root.Locator.Line = nil
					r.Root.Locator.Character = nil
					r.Root.Locator.Symbol = "Missing"
				}
			})
			raw, err := CaptureV2(context.Background(), r, root)
			if err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			if err = json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			expected := []string{"/acquisition/request/required_targets/0/locator", "/acquisition/targets/1/requested/locator"}
			if state == "missing" {
				expected = append(expected, "/acquisition/request/root/locator/symbol", "/acquisition/targets/0/requested/locator/symbol")
			} else {
				expected = append(expected, "/acquisition/request/root/locator", "/acquisition/targets/0/requested/locator")
			}
			for _, pointer := range expected {
				found := -1
				for i, b := range e.Bindings {
					if b.Pointer == pointer {
						found = i
						if b.URI != r.Request.Root.Locator.URI || b.Attribution != "SOURCE" {
							t.Fatal("selector attribution", b)
						}
						if !strings.HasSuffix(pointer, "/symbol") && b.AnchorStatus != "VALID_COORDINATES" {
							t.Fatal("selector coordinate status", b)
						}
					}
				}
				if found < 0 {
					t.Fatal("missing independently expected selector", pointer)
				}
				copy := e
				copy.Bindings = append(append([]BindingV2{}, e.Bindings[:found]...), e.Bindings[found+1:]...)
				if err := ValidateV2(copy); err == nil {
					t.Fatal("omitted selector accepted", pointer)
				}
			}
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal("source-free selector admission", err)
			}
		})
	}
}

func TestV2RepairIndexedReferences(t *testing.T) {
	raw, e := reviewEnvelope(t)
	expected := []string{"/acquisition/targets/1/connection/path/group_ids/0", "/acquisition/targets/1/connection/path/occurrence_ids/0/0", "/graph/seeds/0/reached_relation_ids/0"}
	for _, p := range expected {
		found := false
		for i, b := range e.Bindings {
			if b.Pointer == p {
				found = true
				if b.Attribution != "NON_SOURCE" || b.AnchorStatus != "NON_SOURCE" || b.URI != "" || len(b.ReceiptIDs) != 0 {
					t.Fatal("relation is not a coordinate", b)
				}
				var copy EvidenceV2
				_ = json.Unmarshal(raw, &copy)
				copy.Bindings = append(copy.Bindings[:i], copy.Bindings[i+1:]...)
				bad, err := json.Marshal(copy)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = ValidateFor(bad, Family, "v2"); err == nil {
					t.Fatal("omitted independently expected reference accepted", p)
				}
			}
		}
		if !found {
			t.Fatal("missing independently expected reference", p)
		}
	}
}

func TestV2RepairCoordinateCarriers(t *testing.T) {
	good := json.RawMessage(`{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}`)
	empty := json.RawMessage(`{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}`)
	for _, method := range []string{"textDocument/prepareCallHierarchy", "textDocument/documentSymbol", "callHierarchy/outgoingCalls"} {
		for _, variant := range []string{"valid", "empty", "backwards", "missing", "wrong_type", "selection_outside"} {
			if method == "callHierarchy/outgoingCalls" && variant == "selection_outside" {
				continue
			}
			t.Run(method+"/"+variant, func(t *testing.T) {
				base, root := coordinatorV2Fixture(t, nil)
				req := base.Request
				req.RequiredTargets = nil
				if method == "textDocument/documentSymbol" {
					req.Root.Locator.Symbol = "A"
					req.Root.Locator.Line = nil
					req.Root.Locator.Character = nil
				}
				span := good
				selection := good
				switch variant {
				case "empty":
					span = empty
					selection = empty
				case "backwards":
					span = json.RawMessage(`{"start":{"line":2,"character":0},"end":{"line":1,"character":0}}`)
				case "missing":
					span = json.RawMessage(`{"start":{"line":0},"end":{"line":0,"character":1}}`)
				case "wrong_type":
					span = json.RawMessage(`{"start":{"line":"0","character":0},"end":{"line":0,"character":1}}`)
				case "selection_outside":
					selection = json.RawMessage(`{"start":{"line":2,"character":0},"end":{"line":2,"character":1}}`)
				}
				item := *base.Targets[0].Resolution.Prepared
				wire := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
					if q.Method == method {
						if method == "callHierarchy/outgoingCalls" {
							return json.Marshal([]any{map[string]any{"to": item, "fromRanges": []any{span}}})
						}
						row := map[string]any{"name": "A", "kind": 12, "range": span, "selectionRange": selection}
						if method != "textDocument/documentSymbol" {
							row["uri"] = req.Root.Locator.URI
							row["data"] = map[string]any{"range": span, "uri": "file:///opaque.go"}
						}
						return json.Marshal([]any{row})
					}
					if q.Method == "textDocument/prepareCallHierarchy" {
						return json.Marshal([]any{item})
					}
					return json.RawMessage(`[]`), nil
				})
				r, err := acquisition.Acquire(context.Background(), wire, req)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := CaptureV2(context.Background(), r, root)
				if err != nil {
					t.Fatal("legitimate acquisition rejected", err)
				}
				var e EvidenceV2
				if err = json.Unmarshal(raw, &e); err != nil {
					t.Fatal(err)
				}
				queried := false
				for i, q := range r.Requests {
					if q.Method == method {
						queried = true
						prefix := fmt.Sprintf("/acquisition/requests/%d/response/", i)
						if variant == "missing" || variant == "wrong_type" {
							if q.Outcome != "FAILED" {
								t.Fatal("wire shape was not rejected", q.Outcome)
							}
							for _, b := range e.Bindings {
								if strings.HasPrefix(b.Pointer, prefix) {
									t.Fatal("fabricated malformed wire anchor", b)
								}
							}
							continue
						}
						pointer := prefix + "0/range"
						if method == "callHierarchy/outgoingCalls" {
							pointer = prefix + "0/fromRanges/0"
						}
						if variant == "selection_outside" {
							pointer = prefix + "0/selectionRange"
						}
						want := "VALID_COORDINATES"
						if variant == "backwards" || variant == "selection_outside" {
							want = "INVALID_COORDINATES"
						}
						found := -1
						for j, b := range e.Bindings {
							if b.Pointer == pointer {
								found = j
								if b.Attribution != "SOURCE" || b.URI != req.Root.Locator.URI || b.AnchorStatus != want || len(b.ReceiptIDs) == 0 {
									t.Fatalf("coordinate disposition %s: %+v", want, b)
								}
							}
						}
						if found < 0 {
							t.Fatal("missing exact response range", pointer)
						}
						if want == "INVALID_COORDINATES" {
							altered := e
							altered.Bindings = append([]BindingV2{}, e.Bindings...)
							altered.Bindings[found].AnchorStatus = "VALID_COORDINATES"
							bad, err := json.Marshal(altered)
							if err != nil {
								t.Fatal(err)
							}
							if _, err = ValidateFor(bad, Family, "v2"); err == nil {
								t.Fatal("invalid anchor status upgrade accepted")
							}
						}
					}
				}
				if !queried {
					t.Fatal("fixture did not issue tested method")
				}
				for _, b := range e.Bindings {
					if strings.Contains(b.Pointer, "/data/") || b.URI == "file:///opaque.go" {
						t.Fatal("opaque data interpreted", b)
					}
				}
				original, _ := json.Marshal(r)
				restored, _ := json.Marshal(e.Acquisition)
				if !bytes.Equal(original, restored) {
					t.Fatal("complete acquisition changed")
				}
				if err = os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				if _, err = ValidateFor(raw, Family, "v2"); err != nil {
					t.Fatal("source-free admission", err)
				}
			})
		}
	}
}

func TestV2RepairCoherentlyResealedLanguage(t *testing.T) {
	_, e := reviewEnvelope(t)
	for i := range e.Acquisition.Supplies {
		e.Acquisition.Supplies[i].LanguageID = "typescript"
	}
	for i := range e.Acquisition.Requests {
		q := &e.Acquisition.Requests[i]
		if q.Method == "source/prepareDocument" {
			var s acquisition.Supply
			_ = json.Unmarshal(q.Response, &s)
			s.LanguageID = "typescript"
			q.Response, _ = json.Marshal(s)
		}
	}
	reviewAccounting(&e)
	// Rebuild the attacker-controlled projection without calling the validator
	// being tested: otherwise early rejection bypasses the composed entry point.
	for i := range e.Supplies {
		e.Supplies[i].Observation.LanguageID = "typescript"
		if e.Supplies[i].Receipt != nil {
			*e.Supplies[i].Receipt = sealV2(*e.Supplies[i].Receipt)
		}
	}
	if err := bindV2(&e); err != nil {
		t.Fatal(err)
	}
	if err := acquisition.ValidateResult(e.Acquisition); err != nil {
		t.Fatal("attack must pass coordinator accounting", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(raw, Family, "v2"); err == nil || !strings.Contains(err.Error(), "didOpen language") {
		t.Fatal("fully resealed language attack must fail exact join", err)
	}
}

func TestV2RepairRuntimeLanguageControls(t *testing.T) {
	cases := []struct {
		name, explicit, first, second string
		notifications                 bool
		want                          bool
	}{
		{"inferred", "", "go", "go", true, true},
		{"configured_default", "", "custom", "custom", true, true},
		{"explicit_override_extension", "typescript", "typescript", "typescript", true, true},
		{"explicit_verbatim", " Go ", " Go ", " Go ", true, true},
		{"explicit_mismatch", "go", "typescript", "typescript", true, false},
		{"didchange_contradiction", "", "go", "typescript", true, false},
		{"cached", "", "custom", "custom", false, true},
		{"cached_contradiction", "", "go", "typescript", false, false},
		{"unavailable_language", "", "", "", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, root := coordinatorV2Fixture(t, nil)
			req := base.Request
			req.Root.Locator.LanguageID = tc.explicit
			req.RequiredTargets[0].Locator.LanguageID = tc.explicit
			one := uint32(1)
			req.RequiredTargets[0].Locator.Line = &one
			item := *base.Targets[0].Resolution.Prepared
			wire := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
				if q.Method == "textDocument/prepareCallHierarchy" {
					return json.Marshal([]any{item})
				}
				return json.RawMessage(`[]`), nil
			})
			count := 0
			client := acquisition.WithDocumentSupply(wire, func(_ context.Context, a acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
				count++
				language := tc.first
				if count > 1 {
					language = tc.second
				}
				s := acquisition.Supply{URI: l.URI, LanguageID: language}
				if tc.notifications {
					text := fmt.Sprintf("independent version %d", count)
					method := "textDocument/didOpen"
					params := map[string]any{"textDocument": map[string]any{"uri": l.URI, "languageId": language, "version": count, "text": text}}
					if count > 1 {
						method = "textDocument/didChange"
						params = map[string]any{"textDocument": map[string]any{"uri": l.URI, "version": count}, "contentChanges": []any{map[string]any{"text": text}}}
					}
					p, _ := json.Marshal(params)
					s.Observation, _ = json.Marshal(sessionruntime.DocumentSupply{Classification: Supplied, SessionID: a.SessionID, Generation: a.Generation, URI: l.URI, DocumentVersion: count, Method: method, Content: []byte(text), Params: p})
				}
				return s, nil
			})
			r, err := acquisition.Acquire(context.Background(), client, req)
			if err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatal("expected two supplies", count)
			}
			raw, err := CaptureV2(context.Background(), r, root)
			if (err == nil) != tc.want {
				t.Fatalf("language acceptance want %v: %v", tc.want, err)
			}
			if err == nil {
				if err = os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				if _, err = ValidateFor(raw, Family, "v2"); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
